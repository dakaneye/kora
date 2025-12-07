package slack

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"time"

	"github.com/dakaneye/kora/internal/auth"
	"github.com/dakaneye/kora/internal/datasources"
	"github.com/dakaneye/kora/internal/models"
)

// DataSource fetches events from Slack (DMs and mentions).
//
// SECURITY per EFA 0002:
//   - The Slack token is retrieved from auth.AuthProvider
//   - Token is used ONLY in the Authorization header (see apiRequest)
//   - Token is NEVER logged or included in error messages
type DataSource struct {
	authProvider auth.AuthProvider
	httpClient   *http.Client
	baseURL      string
	userID       string // cached user ID for mention searches
}

// Option configures the Slack DataSource.
type Option func(*DataSource)

// WithHTTPClient sets a custom HTTP client.
// Use this for testing or to configure custom timeouts/transports.
func WithHTTPClient(client *http.Client) Option {
	return func(d *DataSource) {
		d.httpClient = client
	}
}

// WithBaseURL sets a custom Slack API base URL.
// Use this for testing with a mock server.
func WithBaseURL(baseURL string) Option {
	return func(d *DataSource) {
		d.baseURL = baseURL
	}
}

// NewDataSource creates a Slack datasource.
// The authProvider must be a Slack auth provider (Service() == auth.ServiceSlack).
//
// Returns an error if the authProvider is not for Slack service.
func NewDataSource(authProvider auth.AuthProvider, opts ...Option) (*DataSource, error) {
	if authProvider.Service() != auth.ServiceSlack {
		return nil, fmt.Errorf("slack datasource requires slack auth provider, got %s", authProvider.Service())
	}

	d := &DataSource{
		authProvider: authProvider,
		httpClient: &http.Client{
			Timeout: defaultTimeout,
		},
		baseURL: defaultBaseURL,
	}
	for _, opt := range opts {
		opt(d)
	}
	return d, nil
}

// Name returns the datasource identifier for logging.
func (d *DataSource) Name() string {
	return "slack"
}

// Service returns the service this datasource connects to.
func (d *DataSource) Service() models.Source {
	return models.SourceSlack
}

// Fetch retrieves Slack events (DMs and mentions).
//
// Search strategy:
//  1. Get user ID via auth.test (cached after first call)
//  2. Search for @mentions using search.messages with <@USER_ID>
//  3. List DM conversations and fetch unread messages
//
// EFA 0003: Context must be used for all network operations.
// EFA 0003: Partial success must be supported.
// EFA 0001: All returned events must pass Validate().
//
// SECURITY per EFA 0002:
//   - Token is obtained from AuthProvider and used ONLY in Authorization header
//   - Token is NEVER logged or included in error messages
func (d *DataSource) Fetch(ctx context.Context, opts datasources.FetchOptions) (*datasources.FetchResult, error) {
	if err := opts.Validate(); err != nil {
		return nil, fmt.Errorf("slack fetch: %w", err)
	}

	// Get credential
	cred, err := d.authProvider.GetCredential(ctx)
	if err != nil {
		return nil, fmt.Errorf("slack fetch: %w", datasources.ErrNotAuthenticated)
	}

	// Get token value - SECURITY: This must NEVER be logged
	token := cred.Value()
	if token == "" {
		return nil, fmt.Errorf("slack fetch: %w", datasources.ErrNotAuthenticated)
	}

	result := &datasources.FetchResult{
		Stats: datasources.FetchStats{},
	}
	startTime := time.Now()

	// Get user ID if not cached (needed for mention searches)
	if d.userID == "" {
		userID, userErr := d.getUserID(ctx, token)
		if userErr != nil {
			return nil, fmt.Errorf("slack fetch: get user id: %w", userErr)
		}
		d.userID = userID
		result.Stats.APICallCount++
	}

	// Execute fetches with partial success support (EFA 0003)
	var allEvents []models.Event
	var fetchErrors []error

	// 1. Fetch mentions (search.messages)
	mentions, mentionCalls, err := d.fetchMentions(ctx, token, opts.Since)
	result.Stats.APICallCount += mentionCalls
	if err != nil {
		fetchErrors = append(fetchErrors, fmt.Errorf("mentions: %w", err))
	} else {
		allEvents = append(allEvents, mentions...)
	}

	// Check context cancellation between calls
	if ctx.Err() != nil {
		result.Events = allEvents
		result.Errors = fetchErrors
		result.Partial = len(allEvents) > 0
		result.Stats.Duration = time.Since(startTime)
		return result, ctx.Err()
	}

	// 2. Fetch DMs (users.conversations + conversations.history)
	dms, dmCalls, err := d.fetchDMs(ctx, token, opts.Since)
	result.Stats.APICallCount += dmCalls
	if err != nil {
		fetchErrors = append(fetchErrors, fmt.Errorf("dms: %w", err))
	} else {
		allEvents = append(allEvents, dms...)
	}

	// Deduplicate by URL (same message can appear in multiple sources)
	allEvents = deduplicateEvents(allEvents)

	// Record fetched count before filtering
	result.Stats.EventsFetched = len(allEvents)

	// Filter by options
	allEvents = filterEvents(allEvents, opts)
	result.Stats.EventsReturned = len(allEvents)

	// Apply limit
	if opts.Limit > 0 && len(allEvents) > opts.Limit {
		allEvents = allEvents[:opts.Limit]
	}

	// Sort by timestamp ascending (EFA 0003 requirement)
	sort.Slice(allEvents, func(i, j int) bool {
		return allEvents[i].Timestamp.Before(allEvents[j].Timestamp)
	})

	// Validate all events (EFA 0001 requirement)
	validEvents := make([]models.Event, 0, len(allEvents))
	for i := range allEvents {
		if err := allEvents[i].Validate(); err != nil {
			fetchErrors = append(fetchErrors, fmt.Errorf("event validation: %w", err))
			continue
		}
		validEvents = append(validEvents, allEvents[i])
	}

	result.Events = validEvents
	result.Errors = fetchErrors
	result.Partial = len(fetchErrors) > 0 && len(validEvents) > 0
	result.Stats.Duration = time.Since(startTime)

	return result, nil
}

// getUserID retrieves the authenticated user's ID via auth.test API.
// The result is cached in d.userID for subsequent calls.
//
// SECURITY: Token is passed to apiRequest which handles it securely.
func (d *DataSource) getUserID(ctx context.Context, token string) (string, error) {
	resp, err := d.apiRequest(ctx, token, "auth.test", nil)
	if err != nil {
		return "", err
	}

	var authResp slackAuthTestResponse
	if err := json.Unmarshal(resp, &authResp); err != nil {
		return "", fmt.Errorf("parse auth.test: %w", err)
	}
	if !authResp.OK {
		return "", fmt.Errorf("auth.test failed: %s", authResp.Error)
	}

	return authResp.UserID, nil
}
