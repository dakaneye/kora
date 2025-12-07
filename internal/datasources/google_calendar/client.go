// Package google_calendar provides Google Calendar API client and datasource.
// Ground truth defined in specs/efas/0003-datasource-interface.md
//
// SECURITY: This client uses GoogleOAuthCredential from the auth provider.
// Access tokens are used only in Authorization headers and MUST NEVER be logged.
// See EFA 0002 for credential security requirements.
//
//nolint:revive // Package name matches directory structure convention
package google_calendar

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/dakaneye/kora/internal/auth/google"
	"github.com/dakaneye/kora/internal/datasources"
)

const (
	// calendarAPIBase is the base URL for Google Calendar API v3.
	calendarAPIBase = "https://www.googleapis.com/calendar/v3"

	// defaultMaxResults is the API limit for events.list.
	defaultMaxResults = 250

	// maxRetries is the maximum number of retries for rate-limited requests.
	maxRetries = 3

	// initialBackoff is the initial backoff duration for exponential backoff.
	initialBackoff = 1 * time.Second
)

// CalendarClient wraps the Google Calendar API v3.
//
// SECURITY: Uses GoogleOAuthCredential for authentication.
// Access tokens are used only in Authorization headers and MUST NEVER be logged.
type CalendarClient struct {
	httpClient *http.Client
	credential *google.GoogleOAuthCredential
}

// NewCalendarClient creates a new Google Calendar API client.
// The credential must be valid and non-expired.
//
// The httpClient parameter is optional; if nil, http.DefaultClient is used.
// For production use, provide a client with appropriate timeouts.
func NewCalendarClient(credential *google.GoogleOAuthCredential, httpClient *http.Client) *CalendarClient {
	if httpClient == nil {
		httpClient = &http.Client{
			Timeout: 30 * time.Second,
		}
	}
	return &CalendarClient{
		httpClient: httpClient,
		credential: credential,
	}
}

// CalendarEntry represents a calendar in the user's calendar list.
//
//nolint:govet // Field order matches API response for clarity
type CalendarEntry struct {
	ID         string `json:"id"`         // Calendar ID
	Summary    string `json:"summary"`    // Display name
	AccessRole string `json:"accessRole"` // "owner", "reader", "writer", "freeBusyReader"
	Primary    bool   `json:"primary"`    // Is this the user's primary calendar
	Selected   bool   `json:"selected"`   // Is this calendar selected in the UI
}

// CalendarEvent represents an event from the Google Calendar API.
//
//nolint:govet // Field order matches API response for clarity
type CalendarEvent struct {
	ID             string          `json:"id"`
	Summary        string          `json:"summary"` // Title
	Description    string          `json:"description"`
	Location       string          `json:"location"`
	Start          EventTime       `json:"start"`
	End            EventTime       `json:"end"`
	Creator        Person          `json:"creator"`
	Organizer      Person          `json:"organizer"`
	Attendees      []Attendee      `json:"attendees"`
	Status         string          `json:"status"` // "confirmed", "tentative", "canceled"
	HTMLLink       string          `json:"htmlLink"`
	ConferenceData *ConferenceData `json:"conferenceData,omitempty"`
}

// EventTime represents when an event starts or ends.
// For timed events, DateTime is set. For all-day events, Date is set.
type EventTime struct {
	DateTime time.Time `json:"-"`    // Parsed from dateTime field
	Date     string    `json:"date"` // For all-day events (YYYY-MM-DD format)
	TimeZone string    `json:"timeZone"`

	// Raw dateTime string for JSON unmarshaling
	DateTimeRaw string `json:"dateTime"`
}

// IsAllDay returns true if this is an all-day event.
func (e *EventTime) IsAllDay() bool {
	return e.Date != "" && e.DateTimeRaw == ""
}

// Time returns the event time. For all-day events, parses the Date field.
func (e *EventTime) Time() time.Time {
	if !e.DateTime.IsZero() {
		return e.DateTime
	}
	if e.Date != "" {
		t, err := time.Parse("2006-01-02", e.Date)
		if err == nil {
			return t
		}
	}
	return time.Time{}
}

// UnmarshalJSON implements custom JSON unmarshaling to handle dateTime parsing.
func (e *EventTime) UnmarshalJSON(data []byte) error {
	type Alias EventTime
	aux := &struct {
		*Alias
	}{
		Alias: (*Alias)(e),
	}
	if err := json.Unmarshal(data, aux); err != nil {
		return err
	}

	// Parse dateTime if present
	if e.DateTimeRaw != "" {
		t, err := time.Parse(time.RFC3339, e.DateTimeRaw)
		if err != nil {
			return fmt.Errorf("parsing dateTime %q: %w", e.DateTimeRaw, err)
		}
		e.DateTime = t
	}

	return nil
}

// Person represents a person associated with a calendar event.
type Person struct {
	Email       string `json:"email"`
	DisplayName string `json:"displayName"`
	Self        bool   `json:"self"` // Is this the authenticated user
}

// Attendee represents a calendar event attendee.
type Attendee struct {
	Email          string `json:"email"`
	DisplayName    string `json:"displayName"`
	ResponseStatus string `json:"responseStatus"` // "needsAction", "declined", "tentative", "accepted"
	Self           bool   `json:"self"`
	Organizer      bool   `json:"organizer"`
	Optional       bool   `json:"optional"`
}

// ConferenceData contains video meeting information.
type ConferenceData struct {
	EntryPoints []EntryPoint `json:"entryPoints"`
}

// EntryPoint represents a way to join a conference.
type EntryPoint struct {
	EntryPointType string `json:"entryPointType"` // "video", "phone", "sip", "more"
	URI            string `json:"uri"`
	Label          string `json:"label"`
}

// calendarListResponse is the API response for calendarList.list.
type calendarListResponse struct {
	NextPageToken string          `json:"nextPageToken"`
	Items         []CalendarEntry `json:"items"`
}

// eventsListResponse is the API response for events.list.
type eventsListResponse struct {
	NextPageToken string          `json:"nextPageToken"`
	Items         []CalendarEvent `json:"items"`
}

// ListCalendars returns all calendars the user has access to.
//
// Implements pagination to fetch all calendars.
// Respects context cancellation and handles rate limiting with exponential backoff.
func (c *CalendarClient) ListCalendars(ctx context.Context) ([]CalendarEntry, error) {
	var allCalendars []CalendarEntry
	pageToken := ""

	for {
		// Check context cancellation
		select {
		case <-ctx.Done():
			if len(allCalendars) > 0 {
				// Return partial results
				return allCalendars, ctx.Err()
			}
			return nil, ctx.Err()
		default:
		}

		// Build request URL
		reqURL := fmt.Sprintf("%s/users/me/calendarList", calendarAPIBase)
		if pageToken != "" {
			reqURL = fmt.Sprintf("%s?pageToken=%s", reqURL, url.QueryEscape(pageToken))
		}

		// Execute request with retry
		body, err := c.doRequestWithRetry(ctx, reqURL)
		if err != nil {
			if len(allCalendars) > 0 {
				// Partial success
				return allCalendars, fmt.Errorf("listing calendars: %w", err)
			}
			return nil, fmt.Errorf("listing calendars: %w", err)
		}

		// Parse response
		var resp calendarListResponse
		if err := json.Unmarshal(body, &resp); err != nil {
			return allCalendars, fmt.Errorf("parsing calendar list response: %w", err)
		}

		allCalendars = append(allCalendars, resp.Items...)

		// Check for more pages
		if resp.NextPageToken == "" {
			break
		}
		pageToken = resp.NextPageToken
	}

	return allCalendars, nil
}

// ListEvents returns events in a time range for a specific calendar.
//
// Parameters:
//   - calendarID: The calendar identifier (use "primary" for the user's primary calendar)
//   - since: Exclusive lower bound for event start time
//   - until: Exclusive upper bound for event start time
//
// Events are returned sorted by start time.
// Recurring events are expanded into individual occurrences.
// Respects context cancellation and handles rate limiting with exponential backoff.
func (c *CalendarClient) ListEvents(ctx context.Context, calendarID string, since, until time.Time) ([]CalendarEvent, error) {
	var allEvents []CalendarEvent
	pageToken := ""

	for {
		// Check context cancellation
		select {
		case <-ctx.Done():
			if len(allEvents) > 0 {
				// Return partial results
				return allEvents, ctx.Err()
			}
			return nil, ctx.Err()
		default:
		}

		// Build request URL with parameters
		params := url.Values{}
		params.Set("timeMin", since.Format(time.RFC3339))
		params.Set("timeMax", until.Format(time.RFC3339))
		params.Set("singleEvents", "true") // Expand recurring events
		params.Set("orderBy", "startTime")
		params.Set("maxResults", strconv.Itoa(defaultMaxResults))
		if pageToken != "" {
			params.Set("pageToken", pageToken)
		}

		reqURL := fmt.Sprintf("%s/calendars/%s/events?%s",
			calendarAPIBase,
			url.PathEscape(calendarID),
			params.Encode(),
		)

		// Execute request with retry
		body, err := c.doRequestWithRetry(ctx, reqURL)
		if err != nil {
			if len(allEvents) > 0 {
				// Partial success
				return allEvents, fmt.Errorf("listing events for %s: %w", calendarID, err)
			}
			return nil, fmt.Errorf("listing events for %s: %w", calendarID, err)
		}

		// Parse response
		var resp eventsListResponse
		if err := json.Unmarshal(body, &resp); err != nil {
			return allEvents, fmt.Errorf("parsing events response: %w", err)
		}

		allEvents = append(allEvents, resp.Items...)

		// Check for more pages
		if resp.NextPageToken == "" {
			break
		}
		pageToken = resp.NextPageToken
	}

	return allEvents, nil
}

// doRequestWithRetry executes an HTTP GET request with exponential backoff on 429 responses.
//
// Returns the response body on success, or an error with appropriate sentinel type.
// SECURITY: Access token is used only in Authorization header.
func (c *CalendarClient) doRequestWithRetry(ctx context.Context, reqURL string) ([]byte, error) {
	backoff := initialBackoff

	for attempt := 0; attempt <= maxRetries; attempt++ {
		// Check context before making request
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		// Create request
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, http.NoBody)
		if err != nil {
			return nil, fmt.Errorf("creating request: %w", err)
		}

		// Set authorization header
		// SECURITY: Token used only here, never logged
		req.Header.Set("Authorization", "Bearer "+c.credential.Value())
		req.Header.Set("Accept", "application/json")

		// Execute request
		resp, err := c.httpClient.Do(req)
		if err != nil {
			// Check for context cancellation
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			return nil, fmt.Errorf("executing request: %w", err)
		}

		// Read body
		body, readErr := io.ReadAll(resp.Body)
		resp.Body.Close() //nolint:errcheck // Ignore close error after read

		if readErr != nil {
			return nil, fmt.Errorf("reading response body: %w", readErr)
		}

		// Handle response status
		switch resp.StatusCode {
		case http.StatusOK:
			return body, nil

		case http.StatusUnauthorized:
			// 401: Re-auth needed
			return nil, fmt.Errorf("%w: access token invalid or expired", datasources.ErrNotAuthenticated)

		case http.StatusTooManyRequests:
			// 429: Rate limited - retry with backoff
			if attempt < maxRetries {
				// Check for Retry-After header
				if retryAfter := resp.Header.Get("Retry-After"); retryAfter != "" {
					if seconds, parseErr := strconv.Atoi(retryAfter); parseErr == nil {
						backoff = time.Duration(seconds) * time.Second
					}
				}

				// Wait before retry
				select {
				case <-ctx.Done():
					return nil, ctx.Err()
				case <-time.After(backoff):
					// Double backoff for next attempt
					backoff *= 2
					continue
				}
			}
			return nil, fmt.Errorf("%w: exceeded retry attempts", datasources.ErrRateLimited)

		case http.StatusServiceUnavailable, http.StatusBadGateway, http.StatusGatewayTimeout:
			// 5xx: Service unavailable - retry with backoff
			if attempt < maxRetries {
				select {
				case <-ctx.Done():
					return nil, ctx.Err()
				case <-time.After(backoff):
					backoff *= 2
					continue
				}
			}
			return nil, fmt.Errorf("%w: status %d", datasources.ErrServiceUnavailable, resp.StatusCode)

		case http.StatusForbidden:
			// 403: Permission denied
			return nil, fmt.Errorf("permission denied: %s", string(body))

		case http.StatusNotFound:
			// 404: Calendar not found
			return nil, fmt.Errorf("calendar not found: %s", string(body))

		default:
			// Other error
			return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
		}
	}

	// Should not reach here
	return nil, fmt.Errorf("%w: request failed", datasources.ErrServiceUnavailable)
}

// GetVideoMeetingURL extracts the video meeting URL from conference data.
// Returns empty string if no video meeting is attached.
func (e *CalendarEvent) GetVideoMeetingURL() string {
	if e.ConferenceData == nil {
		return ""
	}
	for _, ep := range e.ConferenceData.EntryPoints {
		if ep.EntryPointType == "video" {
			return ep.URI
		}
	}
	return ""
}

// GetUserResponseStatus returns the response status of the authenticated user.
// Returns empty string if the user is not an attendee.
func (e *CalendarEvent) GetUserResponseStatus() string {
	for _, attendee := range e.Attendees {
		if attendee.Self {
			return attendee.ResponseStatus
		}
	}
	return ""
}

// GetPendingRSVPs returns attendees who have not yet responded.
func (e *CalendarEvent) GetPendingRSVPs() []Attendee {
	var pending []Attendee
	for _, attendee := range e.Attendees {
		if attendee.ResponseStatus == "needsAction" {
			pending = append(pending, attendee)
		}
	}
	return pending
}

// IsUserOrganizer returns true if the authenticated user is the event organizer.
func (e *CalendarEvent) IsUserOrganizer() bool {
	return e.Organizer.Self
}

// IsAllDay returns true if this is an all-day event.
func (e *CalendarEvent) IsAllDay() bool {
	return e.Start.IsAllDay()
}
