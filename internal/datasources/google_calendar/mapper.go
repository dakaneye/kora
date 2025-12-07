// Package google_calendar provides Google Calendar API client and datasource.
// Ground truth defined in specs/efas/0001-event-model.md and specs/efas/0003-datasource-interface.md
//
// This file converts Google Calendar events to Kora Events.
// IT IS FORBIDDEN TO ADD new EventTypes or metadata keys without updating EFA 0001.
//
//nolint:revive // Package name matches directory structure convention
package google_calendar

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/dakaneye/kora/internal/models"
)

// TimeProvider allows testing with deterministic time.
// In production, use time.Now.
type TimeProvider func() time.Time

// defaultTimeProvider returns the current time.
func defaultTimeProvider() time.Time {
	return time.Now()
}

// mapperConfig holds configuration for the event mapper.
type mapperConfig struct {
	timeProvider TimeProvider
}

// MapperOption configures the event mapper.
type MapperOption func(*mapperConfig)

// WithTimeProvider sets a custom time provider for testing.
func WithTimeProvider(tp TimeProvider) MapperOption {
	return func(c *mapperConfig) {
		c.timeProvider = tp
	}
}

// ToEvent converts a CalendarEvent to a Kora Event.
//
// Parameters:
//   - calEvent: The Google Calendar event
//   - email: The authenticated user's email
//   - calendarID: The calendar ID this event belongs to
//   - opts: Optional configuration (e.g., custom time provider for testing)
//
// Returns error if the resulting event fails validation.
//
// Event type determination follows EFA 0001 priority order:
//  1. Check if starts within 1 hour first (calendar_upcoming)
//  2. Check user response status (needsAction, tentative)
//  3. Check if user is organizer with pending RSVPs
//  4. Check if all-day event
//  5. Default to accepted meeting
func ToEvent(calEvent *CalendarEvent, email, calendarID string, opts ...MapperOption) (models.Event, error) {
	cfg := &mapperConfig{
		timeProvider: defaultTimeProvider,
	}
	for _, opt := range opts {
		opt(cfg)
	}

	now := cfg.timeProvider()

	// Determine event type and whether action is required
	eventType, requiresAction := determineEventType(calEvent, email, now)

	// Determine priority based on event type per EFA 0001
	priority := determinePriority(eventType)

	// Build title based on event type
	title := determineTitle(calEvent, eventType)

	// Build metadata
	metadata, err := buildMetadata(calEvent, email, calendarID)
	if err != nil {
		return models.Event{}, fmt.Errorf("building metadata: %w", err)
	}

	// Create the event
	event := models.Event{
		Type:           eventType,
		Title:          truncateTitle(title),
		Source:         models.SourceGoogleCalendar,
		URL:            calEvent.HTMLLink,
		Author:         buildAuthor(calEvent),
		Timestamp:      calEvent.Start.Time(),
		Priority:       priority,
		RequiresAction: requiresAction,
		Metadata:       metadata,
	}

	// Validate before returning
	if err := event.Validate(); err != nil {
		return models.Event{}, fmt.Errorf("invalid event: %w", err)
	}

	return event, nil
}

// determineEventType returns the event type and whether action is required.
// Follows EFA 0001 priority order for type selection.
func determineEventType(calEvent *CalendarEvent, email string, now time.Time) (models.EventType, bool) {
	startTime := calEvent.Start.Time()

	// 1. Check if event starts within 1 hour (highest priority)
	if !startTime.IsZero() && startTime.After(now) {
		timeUntilStart := startTime.Sub(now)
		if timeUntilStart <= time.Hour {
			// Upcoming event - no action required, just awareness
			return models.EventTypeCalendarUpcoming, false
		}
	}

	// 2. Check user's response status
	responseStatus := calEvent.GetUserResponseStatus()
	switch responseStatus {
	case "needsAction":
		return models.EventTypeCalendarNeedsRSVP, true
	case "tentative":
		return models.EventTypeCalendarTentative, false
	}

	// 3. Check if user is organizer with pending RSVPs
	if calEvent.IsUserOrganizer() {
		pendingRSVPs := calEvent.GetPendingRSVPs()
		if len(pendingRSVPs) > 0 {
			return models.EventTypeCalendarOrganizerPending, true
		}
	}

	// 4. Check if all-day event
	if calEvent.IsAllDay() {
		return models.EventTypeCalendarAllDay, false
	}

	// 5. Default to accepted meeting
	return models.EventTypeCalendarMeeting, false
}

// determinePriority returns the priority based on event type per EFA 0001.
func determinePriority(eventType models.EventType) models.Priority {
	switch eventType {
	case models.EventTypeCalendarUpcoming:
		// Event starts within 1 hour = High priority
		return models.PriorityHigh
	case models.EventTypeCalendarNeedsRSVP,
		models.EventTypeCalendarOrganizerPending,
		models.EventTypeCalendarTentative,
		models.EventTypeCalendarMeeting:
		// All medium priority per EFA 0001
		return models.PriorityMedium
	case models.EventTypeCalendarAllDay:
		// All-day events are informational
		return models.PriorityInfo
	default:
		return models.PriorityMedium
	}
}

// determineTitle creates an appropriate title based on event type.
func determineTitle(calEvent *CalendarEvent, eventType models.EventType) string {
	summary := calEvent.Summary
	if summary == "" {
		summary = "(No title)"
	}

	switch eventType {
	case models.EventTypeCalendarUpcoming:
		return fmt.Sprintf("Starting soon: %s", summary)
	case models.EventTypeCalendarNeedsRSVP:
		return fmt.Sprintf("Needs RSVP: %s", summary)
	case models.EventTypeCalendarOrganizerPending:
		return fmt.Sprintf("Awaiting RSVPs: %s", summary)
	case models.EventTypeCalendarTentative:
		return fmt.Sprintf("Tentative: %s", summary)
	case models.EventTypeCalendarAllDay:
		return fmt.Sprintf("%s (all day)", summary)
	case models.EventTypeCalendarMeeting:
		return summary
	default:
		return summary
	}
}

// truncateTitle ensures the title is 1-100 characters per EFA 0001.
func truncateTitle(title string) string {
	if len(title) <= 100 {
		return title
	}
	return title[:97] + "..."
}

// buildAuthor creates the Author field from the event organizer.
func buildAuthor(calEvent *CalendarEvent) models.Person {
	name := calEvent.Organizer.DisplayName
	if name == "" {
		name = calEvent.Organizer.Email
	}

	username := calEvent.Organizer.Email
	if username == "" {
		username = "unknown"
	}

	return models.Person{
		Name:     name,
		Username: username,
	}
}

// buildMetadata constructs the metadata map per EFA 0001 allowed keys.
func buildMetadata(calEvent *CalendarEvent, email, calendarID string) (map[string]any, error) {
	metadata := make(map[string]any)

	// Required fields per EFA 0001
	metadata["calendar_id"] = calendarID
	metadata["event_id"] = calEvent.ID
	metadata["start_time"] = formatTime(calEvent.Start)
	metadata["end_time"] = formatTime(calEvent.End)
	metadata["is_all_day"] = calEvent.IsAllDay()
	metadata["organizer_email"] = calEvent.Organizer.Email
	metadata["organizer_name"] = calEvent.Organizer.DisplayName
	metadata["attendee_count"] = len(calEvent.Attendees)
	metadata["response_status"] = calEvent.GetUserResponseStatus()

	// Attendees as JSON array per EFA 0001: [{email, name, status}]
	attendees, err := buildAttendeesMetadata(calEvent.Attendees)
	if err != nil {
		return nil, fmt.Errorf("building attendees metadata: %w", err)
	}
	metadata["attendees"] = attendees

	// Pending RSVPs - emails of attendees who haven't responded
	pendingRSVPs := calEvent.GetPendingRSVPs()
	pendingEmails := make([]string, 0, len(pendingRSVPs))
	for _, attendee := range pendingRSVPs {
		pendingEmails = append(pendingEmails, attendee.Email)
	}
	metadata["pending_rsvps"] = pendingEmails

	// Optional fields - only add if non-empty
	if calEvent.Location != "" {
		metadata["location"] = calEvent.Location
	}

	if url := calEvent.GetVideoMeetingURL(); url != "" {
		metadata["conference_url"] = url
	}

	if calEvent.Description != "" {
		metadata["description"] = truncateDescription(calEvent.Description)
	}

	return metadata, nil
}

// buildAttendeesMetadata converts attendees to the EFA 0001 format.
// Returns a slice of maps with keys: email, name, status
func buildAttendeesMetadata(attendees []Attendee) ([]map[string]string, error) {
	result := make([]map[string]string, 0, len(attendees))
	for _, a := range attendees {
		attendeeMap := map[string]string{
			"email":  a.Email,
			"name":   a.DisplayName,
			"status": a.ResponseStatus,
		}
		result = append(result, attendeeMap)
	}
	return result, nil
}

// formatTime returns an RFC3339 string for the event time.
// For all-day events, uses the Date field.
func formatTime(et EventTime) string {
	if et.IsAllDay() {
		// All-day events store date as YYYY-MM-DD
		// Parse and format as RFC3339 at midnight UTC
		t, err := time.Parse("2006-01-02", et.Date)
		if err != nil {
			return et.Date
		}
		return t.Format(time.RFC3339)
	}
	return et.DateTime.Format(time.RFC3339)
}

// truncateDescription limits description to 500 chars per EFA 0001.
func truncateDescription(desc string) string {
	if len(desc) <= 500 {
		return desc
	}
	return desc[:497] + "..."
}

// ToEvents converts multiple CalendarEvents to Kora Events.
// Invalid events are skipped with a warning (logged via the provided logger).
// Returns all successfully converted events and any errors encountered.
func ToEvents(calEvents []CalendarEvent, email, calendarID string, opts ...MapperOption) ([]models.Event, []error) {
	events := make([]models.Event, 0, len(calEvents))
	var errs []error

	for i := range calEvents {
		calEvent := &calEvents[i]
		// Skip canceled events (Google Calendar API uses British English spelling)
		if calEvent.Status == "cancelled" { //nolint:misspell // Google Calendar API spelling
			continue
		}

		event, err := ToEvent(calEvent, email, calendarID, opts...)
		if err != nil {
			errs = append(errs, fmt.Errorf("event %s: %w", calEvent.ID, err))
			continue
		}
		events = append(events, event)
	}

	return events, errs
}

// ToEventsJSON converts events and returns them as JSON for debugging.
// This is a helper for development and should not be used in production.
func ToEventsJSON(events []models.Event) (string, error) {
	data, err := json.MarshalIndent(events, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshaling events: %w", err)
	}
	return string(data), nil
}
