// Package google_calendar provides Google Calendar API client and datasource.
// Ground truth defined in specs/efas/0003-datasource-interface.md
//
// IT IS FORBIDDEN TO CHANGE without updating EFA 0003.
// Claude MUST stop and ask before modifying this file.
//
//nolint:revive // Package name matches directory structure convention
package google_calendar

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

// lookAheadDuration is how far into the future to fetch calendar events.
// Meetings within this window are fetched for the digest.
const lookAheadDuration = 7 * 24 * time.Hour // 7 days

// GoogleCalendarDataSource fetches calendar events from Google Calendar API.
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
type GoogleCalendarDataSource struct {
	authProvider *google.GoogleAuthProvider
	calendarIDs  []string // Which calendars to fetch (empty = all)
	email        string   // User's email for mapper
	timeProvider TimeProvider
}

// Option configures a GoogleCalendarDataSource.
type Option func(*GoogleCalendarDataSource)

// WithCalendarIDs configures specific calendars to fetch.
// If not specified, all calendars the user has access to are fetched.
func WithCalendarIDs(ids []string) Option {
	return func(d *GoogleCalendarDataSource) {
		d.calendarIDs = ids
	}
}

// WithTimeProviderOption sets a custom time provider for testing.
func WithTimeProviderOption(tp TimeProvider) Option {
	return func(d *GoogleCalendarDataSource) {
		d.timeProvider = tp
	}
}

// NewGoogleCalendarDataSource creates a datasource for a specific Google account.
//
// Parameters:
//   - authProvider: GoogleAuthProvider for the target account (required)
//   - opts: Optional configuration (calendar IDs, time provider)
//
// Returns error if authProvider is nil.
func NewGoogleCalendarDataSource(authProvider *google.GoogleAuthProvider, opts ...Option) (*GoogleCalendarDataSource, error) {
	if authProvider == nil {
		return nil, errors.New("google calendar: auth provider required")
	}

	d := &GoogleCalendarDataSource{
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
// Format: "google-calendar" per EFA 0003.
func (d *GoogleCalendarDataSource) Name() string {
	return "google-calendar"
}

// Service returns models.SourceGoogleCalendar.
// Used for grouping events and associating with AuthProviders.
func (d *GoogleCalendarDataSource) Service() models.Source {
	return models.SourceGoogleCalendar
}

// Fetch retrieves calendar events per EFA 0003 requirements.
//
// Implementation:
//  1. Gets credential from auth provider
//  2. Lists calendars if not specified
//  3. Fetches events from each calendar concurrently via errgroup
//  4. Filters out declined/canceled events
//  5. Converts to models.Event via mapper
//  6. Aggregates results with partial success support
//
// Time window:
//   - Since: opts.Since (start of digest window)
//   - Until: Since + 7 days (look ahead for upcoming meetings)
//
// EFA 0003 compliance:
//   - Context used for all network operations
//   - Partial success supported (some calendars fail, others succeed)
//   - All returned events pass Validate()
//   - Events sorted by Timestamp ascending
func (d *GoogleCalendarDataSource) Fetch(ctx context.Context, opts datasources.FetchOptions) (*datasources.FetchResult, error) {
	startTime := d.timeProvider()

	// Validate options per EFA 0003
	if err := opts.Validate(); err != nil {
		return nil, fmt.Errorf("google calendar: %w", err)
	}

	// 1. Get credential from auth provider
	cred, err := d.authProvider.GetCredential(ctx)
	if err != nil {
		if errors.Is(err, auth.ErrNotAuthenticated) {
			return nil, fmt.Errorf("%w: %v", datasources.ErrNotAuthenticated, err)
		}
		return nil, fmt.Errorf("google calendar auth: %w", err)
	}

	// Type assert to GoogleOAuthCredential
	oauthCred, ok := cred.(*google.GoogleOAuthCredential)
	if !ok {
		return nil, fmt.Errorf("%w: unexpected credential type", datasources.ErrNotAuthenticated)
	}

	// 2. Create calendar client
	client := NewCalendarClient(oauthCred, nil)

	// 3. Get calendar list if not specified
	calendars := d.calendarIDs
	var apiCallCount int

	if len(calendars) == 0 {
		calList, err := client.ListCalendars(ctx)
		apiCallCount++

		if err != nil {
			// Check for specific error types
			if errors.Is(err, datasources.ErrNotAuthenticated) ||
				errors.Is(err, datasources.ErrRateLimited) ||
				errors.Is(err, datasources.ErrServiceUnavailable) {
				return nil, err
			}
			// Context cancellation
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			return nil, fmt.Errorf("listing calendars: %w", err)
		}

		// Extract calendar IDs
		calendars = make([]string, 0, len(calList))
		for _, cal := range calList {
			calendars = append(calendars, cal.ID)
		}
	}

	// Handle no calendars case
	if len(calendars) == 0 {
		return &datasources.FetchResult{
			Events: []models.Event{},
			Stats: datasources.FetchStats{
				Duration:     d.timeProvider().Sub(startTime),
				APICallCount: apiCallCount,
			},
		}, nil
	}

	// 4. Calculate time window
	// Since is the lower bound (exclusive)
	// Until is Since + 7 days for look-ahead
	since := opts.Since
	until := since.Add(lookAheadDuration)

	// 5. Fetch events from each calendar concurrently
	g, gctx := errgroup.WithContext(ctx)

	// Use mutex to safely aggregate results
	var mu sync.Mutex
	var allEvents []models.Event
	var fetchErrors []error
	var totalEventsFetched int

	for _, calID := range calendars {
		calID := calID // capture loop variable
		g.Go(func() error {
			events, fetched, err := d.fetchCalendarEvents(gctx, client, calID, since, until, opts)

			mu.Lock()
			defer mu.Unlock()

			apiCallCount++ // One API call per calendar
			totalEventsFetched += fetched

			if err != nil {
				// Don't fail entire fetch on single calendar error
				// This enables partial success per EFA 0003
				fetchErrors = append(fetchErrors, fmt.Errorf("calendar %s: %w", calID, err))
				return nil
			}

			allEvents = append(allEvents, events...)
			return nil
		})
	}

	// Wait for all goroutines to complete
	if err := g.Wait(); err != nil {
		// errgroup returns first non-nil error, but we return nil from goroutines
		// This shouldn't happen, but handle it just in case
		fetchErrors = append(fetchErrors, err)
	}

	// 6. Sort all events by timestamp ascending
	sort.Slice(allEvents, func(i, j int) bool {
		return allEvents[i].Timestamp.Before(allEvents[j].Timestamp)
	})

	// 7. Build result
	result := &datasources.FetchResult{
		Events:  allEvents,
		Partial: len(fetchErrors) > 0 && len(allEvents) > 0,
		Errors:  fetchErrors,
		Stats: datasources.FetchStats{
			Duration:       d.timeProvider().Sub(startTime),
			APICallCount:   apiCallCount,
			EventsFetched:  totalEventsFetched,
			EventsReturned: len(allEvents),
		},
	}

	// If all calendars failed, return error
	if len(fetchErrors) > 0 && len(allEvents) == 0 {
		combinedErr := result.CombinedError()
		return result, combinedErr
	}

	return result, nil
}

// fetchCalendarEvents fetches and filters events from a single calendar.
// Returns:
//   - events: Filtered and converted models.Event slice
//   - fetched: Total raw events fetched (before filtering)
//   - err: Any error encountered
//
// Filtering rules per EFA 0003:
//   - Exclude declined events (user responded "declined")
//   - Exclude canceled events (API status value uses British spelling)
//   - Include: accepted, tentative, needsAction, organizer
func (d *GoogleCalendarDataSource) fetchCalendarEvents(
	ctx context.Context,
	client *CalendarClient,
	calendarID string,
	since, until time.Time,
	opts datasources.FetchOptions,
) ([]models.Event, int, error) {
	// Check context before making API call
	select {
	case <-ctx.Done():
		return nil, 0, ctx.Err()
	default:
	}

	// Fetch raw calendar events
	calEvents, err := client.ListEvents(ctx, calendarID, since, until)
	if err != nil {
		// Propagate sentinel errors
		if errors.Is(err, datasources.ErrNotAuthenticated) ||
			errors.Is(err, datasources.ErrRateLimited) ||
			errors.Is(err, datasources.ErrServiceUnavailable) {
			return nil, 0, err
		}
		// Context errors
		if ctx.Err() != nil {
			return nil, 0, ctx.Err()
		}
		return nil, 0, fmt.Errorf("listing events: %w", err)
	}

	totalFetched := len(calEvents)

	// Filter events before conversion
	filteredEvents := d.filterCalendarEvents(calEvents)

	// Convert to models.Event using mapper
	// Pass time provider for consistent "upcoming" detection
	events, conversionErrors := ToEvents(filteredEvents, d.email, calendarID, WithTimeProvider(d.timeProvider))

	// Log conversion errors but don't fail the entire calendar
	// Invalid events are skipped, valid events are returned
	// Per EFA, we skip invalid events - the mapper handles this gracefully
	_ = conversionErrors // Explicitly ignore conversion errors

	// Apply additional filters from FetchOptions if specified
	events = d.applyFilters(events, opts)

	return events, totalFetched, nil
}

// filterCalendarEvents applies calendar-specific filtering rules.
// Per EFA 0003:
//   - Exclude declined events (responseStatus = "declined")
//   - Exclude canceled events (API status value uses British spelling)
//   - Include: accepted, tentative, needsAction, no response (organizer)
func (d *GoogleCalendarDataSource) filterCalendarEvents(events []CalendarEvent) []CalendarEvent {
	filtered := make([]CalendarEvent, 0, len(events))

	for i := range events {
		event := &events[i]

		// Exclude cancelled events (Google Calendar API uses British English spelling)
		//nolint:misspell // Google Calendar API spelling
		if event.Status == "cancelled" {
			continue
		}

		// Exclude declined events
		responseStatus := event.GetUserResponseStatus()
		if responseStatus == "declined" {
			continue
		}

		// Include all other events:
		// - accepted: User confirmed attendance
		// - tentative: User marked tentative
		// - needsAction: User hasn't responded yet
		// - empty: User is organizer or no attendees list
		filtered = append(filtered, *event)
	}

	return filtered
}

// applyFilters applies FetchOptions filters to the converted events.
func (d *GoogleCalendarDataSource) applyFilters(events []models.Event, opts datasources.FetchOptions) []models.Event {
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

// Ensure GoogleCalendarDataSource implements datasources.DataSource.
var _ datasources.DataSource = (*GoogleCalendarDataSource)(nil)
