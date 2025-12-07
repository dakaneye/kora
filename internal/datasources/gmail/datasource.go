// Package gmail provides Gmail API client and datasource.
// Ground truth defined in specs/efas/0003-datasource-interface.md
//
// IT IS FORBIDDEN TO CHANGE without updating EFA 0003.
// Claude MUST stop and ask before modifying this file.
//
//nolint:revive // Package name matches directory structure convention
package gmail

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/dakaneye/kora/internal/auth"
	"github.com/dakaneye/kora/internal/auth/google"
	"github.com/dakaneye/kora/internal/datasources"
	"github.com/dakaneye/kora/internal/models"
)

// TimeProvider is a function that returns the current time.
// Used for testing to inject a fixed time.
type TimeProvider func() time.Time

// defaultTimeProvider returns the current time.
func defaultTimeProvider() time.Time {
	return time.Now()
}

// GmailDataSource fetches email events from Gmail API.
// Ground truth defined in specs/efas/0003-datasource-interface.md
//
// IT IS FORBIDDEN TO CHANGE without updating EFA 0003.
// Claude MUST stop and ask before modifying this file.
//
// Security:
//   - Uses GoogleAuthProvider for OAuth credentials (EFA 0002)
//   - Never logs or exposes access tokens
//   - Delegates all credential handling to auth provider
//
//nolint:govet // Field order matches logical grouping, not memory alignment
type GmailDataSource struct {
	authProvider     *google.GoogleAuthProvider
	email            string
	importantSenders []string
	timeProvider     TimeProvider
}

// Option configures a GmailDataSource.
type Option func(*GmailDataSource)

// WithImportantSenders configures important sender addresses.
// Emails from these senders get higher priority (email_important event type).
// Supports exact email matches and domain patterns (e.g., "@company.com").
func WithImportantSenders(senders []string) Option {
	return func(d *GmailDataSource) {
		d.importantSenders = senders
	}
}

// WithTimeProviderOption sets a custom time provider for testing.
func WithTimeProviderOption(tp TimeProvider) Option {
	return func(d *GmailDataSource) {
		d.timeProvider = tp
	}
}

// NewGmailDataSource creates a datasource for a specific Gmail account.
//
// Parameters:
//   - authProvider: GoogleAuthProvider for the target account (required)
//   - opts: Optional configuration (important senders, time provider)
//
// Returns error if authProvider is nil.
func NewGmailDataSource(authProvider *google.GoogleAuthProvider, opts ...Option) (*GmailDataSource, error) {
	if authProvider == nil {
		return nil, errors.New("gmail: auth provider required")
	}

	d := &GmailDataSource{
		authProvider: authProvider,
		email:        authProvider.Email(),
		timeProvider: defaultTimeProvider,
	}

	for _, opt := range opts {
		opt(d)
	}

	return d, nil
}

// Name returns a human-readable identifier for logging.
// Format: "gmail" per EFA 0003.
func (d *GmailDataSource) Name() string {
	return "gmail"
}

// Service returns models.SourceGmail.
// Used for grouping events and associating with AuthProviders.
func (d *GmailDataSource) Service() models.Source {
	return models.SourceGmail
}

// Fetch retrieves email events per EFA 0003 requirements.
//
// Implementation:
//  1. Gets credential from auth provider
//  2. Creates Gmail client
//  3. Executes both queries concurrently (unread, inbox)
//  4. Fetches full message details (BatchGetMessages)
//  5. Filters out mailing lists and automated senders
//  6. Deduplicates by message_id (same message can appear in both queries)
//  7. Converts to models.Event via mapper
//  8. Sorts by timestamp ascending
//
// Gmail Queries:
//   - Query 1: "is:unread after:{timestamp}" - Unread emails
//   - Query 2: "in:inbox -in:sent after:{timestamp}" - Inbox emails (for unreplied detection)
//
// EFA 0003 compliance:
//   - Context used for all network operations
//   - Partial success supported (one query fails, other succeeds)
//   - All returned events pass Validate()
//   - Events sorted by Timestamp ascending
func (d *GmailDataSource) Fetch(ctx context.Context, opts datasources.FetchOptions) (*datasources.FetchResult, error) {
	startTime := d.timeProvider()

	// Validate options per EFA 0003
	if err := opts.Validate(); err != nil {
		return nil, fmt.Errorf("gmail: %w", err)
	}

	// 1. Get credential from auth provider
	cred, err := d.authProvider.GetCredential(ctx)
	if err != nil {
		if errors.Is(err, auth.ErrNotAuthenticated) {
			return nil, fmt.Errorf("%w: %v", datasources.ErrNotAuthenticated, err)
		}
		return nil, fmt.Errorf("gmail auth: %w", err)
	}

	// Type assert to GoogleOAuthCredential
	oauthCred, ok := cred.(*google.GoogleOAuthCredential)
	if !ok {
		return nil, fmt.Errorf("%w: unexpected credential type", datasources.ErrNotAuthenticated)
	}

	// 2. Create Gmail client
	client := NewGmailClient(oauthCred, nil)

	// 3. Build queries with Unix timestamp
	sinceUnix := opts.Since.Unix()
	unreadQuery := fmt.Sprintf("is:unread after:%d", sinceUnix)
	inboxQuery := fmt.Sprintf("in:inbox -in:sent after:%d", sinceUnix)

	// Determine max results per query
	maxResults := 100
	if opts.Limit > 0 && opts.Limit < maxResults {
		maxResults = opts.Limit
	}

	// 4. Execute queries concurrently
	g, gctx := errgroup.WithContext(ctx)

	var mu sync.Mutex
	var unreadIDs, inboxIDs []MessageID
	var unreadErr, inboxErr error
	var apiCallCount int

	g.Go(func() error {
		ids, err := client.ListMessages(gctx, unreadQuery, maxResults)
		mu.Lock()
		defer mu.Unlock()
		apiCallCount++
		unreadIDs = ids
		unreadErr = err
		return nil // Don't fail group on single query error
	})

	g.Go(func() error {
		ids, err := client.ListMessages(gctx, inboxQuery, maxResults)
		mu.Lock()
		defer mu.Unlock()
		apiCallCount++
		inboxIDs = ids
		inboxErr = err
		return nil // Don't fail group on single query error
	})

	// Wait for both queries.
	// Error is intentionally ignored because goroutines return nil and track
	// errors via unreadErr/inboxErr for partial success support per EFA 0003.
	_ = g.Wait() //nolint:errcheck // Individual errors tracked in unreadErr/inboxErr

	// 5. Handle partial success
	// Pre-allocate for potential errors from both queries
	fetchErrors := make([]error, 0, 2)
	if unreadErr != nil {
		fetchErrors = append(fetchErrors, fmt.Errorf("unread query: %w", unreadErr))
	}
	if inboxErr != nil {
		fetchErrors = append(fetchErrors, fmt.Errorf("inbox query: %w", inboxErr))
	}

	// If both failed, return error
	if unreadErr != nil && inboxErr != nil {
		return &datasources.FetchResult{
			Events:  []models.Event{},
			Partial: false,
			Errors:  fetchErrors,
			Stats: datasources.FetchStats{
				Duration:     d.timeProvider().Sub(startTime),
				APICallCount: apiCallCount,
			},
		}, errors.Join(fetchErrors...)
	}

	// 6. Deduplicate message IDs
	uniqueIDs := d.deduplicateMessageIDs(unreadIDs, inboxIDs)

	// Handle no messages case
	if len(uniqueIDs) == 0 {
		return &datasources.FetchResult{
			Events:  []models.Event{},
			Partial: len(fetchErrors) > 0,
			Errors:  fetchErrors,
			Stats: datasources.FetchStats{
				Duration:     d.timeProvider().Sub(startTime),
				APICallCount: apiCallCount,
			},
		}, nil
	}

	// 7. Batch fetch messages
	messages, batchErrors := client.BatchGetMessages(ctx, uniqueIDs)
	apiCallCount += len(uniqueIDs) // Each message is an API call

	// Collect batch errors
	for i, err := range batchErrors {
		if err != nil {
			fetchErrors = append(fetchErrors, fmt.Errorf("message %s: %w", uniqueIDs[i], err))
		}
	}

	// Collect non-nil messages
	var validMessages []*Message
	for _, msg := range messages {
		if msg != nil {
			validMessages = append(validMessages, msg)
		}
	}

	totalFetched := len(validMessages)

	// 8. Filter out mailing lists and automated senders
	filteredMessages := FilterMessages(validMessages)

	// 9. Convert to events with mapper
	events, conversionErrors := ToEvents(filteredMessages, d.email, d.importantSenders)

	// Collect conversion errors (non-fatal)
	fetchErrors = append(fetchErrors, conversionErrors...)

	// 10. Apply additional filters from FetchOptions
	events = d.applyFilters(events, opts)

	// 11. Sort by timestamp ascending
	sort.Slice(events, func(i, j int) bool {
		return events[i].Timestamp.Before(events[j].Timestamp)
	})

	// Build result
	result := &datasources.FetchResult{
		Events:  events,
		Partial: len(fetchErrors) > 0 && len(events) > 0,
		Errors:  fetchErrors,
		Stats: datasources.FetchStats{
			Duration:       d.timeProvider().Sub(startTime),
			APICallCount:   apiCallCount,
			EventsFetched:  totalFetched,
			EventsReturned: len(events),
		},
	}

	// If all messages failed, return error
	if len(fetchErrors) > 0 && len(events) == 0 {
		combinedErr := result.CombinedError()
		return result, combinedErr
	}

	return result, nil
}

// deduplicateMessageIDs combines and deduplicates message IDs from multiple queries.
// Returns unique IDs preserving order from unread (higher priority) first.
func (d *GmailDataSource) deduplicateMessageIDs(unreadIDs, inboxIDs []MessageID) []string {
	seen := make(map[string]bool)
	var unique []string

	// Process unread first (higher priority)
	for _, id := range unreadIDs {
		if !seen[id.ID] {
			seen[id.ID] = true
			unique = append(unique, id.ID)
		}
	}

	// Add inbox IDs that weren't in unread
	for _, id := range inboxIDs {
		if !seen[id.ID] {
			seen[id.ID] = true
			unique = append(unique, id.ID)
		}
	}

	return unique
}

// applyFilters applies FetchOptions filters to the converted events.
func (d *GmailDataSource) applyFilters(events []models.Event, opts datasources.FetchOptions) []models.Event {
	if opts.Filter == nil {
		return events
	}

	filtered := make([]models.Event, 0, len(events))

	for i := range events {
		event := &events[i]

		// Filter by event types
		if len(opts.Filter.EventTypes) > 0 {
			found := false
			for _, t := range opts.Filter.EventTypes {
				if event.Type == t {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		// Filter by minimum priority (remember: lower number = higher priority)
		if opts.Filter.MinPriority > 0 && event.Priority > opts.Filter.MinPriority {
			continue
		}

		// Filter by requires action
		if opts.Filter.RequiresAction && !event.RequiresAction {
			continue
		}

		filtered = append(filtered, *event)
	}

	return filtered
}

// Ensure GmailDataSource implements datasources.DataSource.
var _ datasources.DataSource = (*GmailDataSource)(nil)
