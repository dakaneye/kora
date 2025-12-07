//nolint:revive // Package name matches directory structure convention
package google_calendar

import (
	"testing"
	"time"

	"github.com/dakaneye/kora/internal/models"
)

// testTime is a fixed time for deterministic tests.
var testTime = time.Date(2025, 12, 7, 10, 0, 0, 0, time.UTC)

// testTimeProvider returns a TimeProvider that always returns testTime.
func testTimeProvider(t time.Time) TimeProvider {
	return func() time.Time {
		return t
	}
}

func TestToEvent_EventTypeUpcoming(t *testing.T) {
	// Event starts in 30 minutes - should be "upcoming"
	startTime := testTime.Add(30 * time.Minute)
	endTime := startTime.Add(1 * time.Hour)

	calEvent := &CalendarEvent{
		ID:       "event-123",
		Summary:  "Team Standup",
		HTMLLink: "https://calendar.google.com/event?eid=abc123",
		Start: EventTime{
			DateTime: startTime,
		},
		End: EventTime{
			DateTime: endTime,
		},
		Organizer: Person{
			Email:       "manager@example.com",
			DisplayName: "Manager",
		},
		Attendees: []Attendee{
			{Email: "user@example.com", ResponseStatus: "accepted", Self: true},
			{Email: "manager@example.com", ResponseStatus: "accepted"},
		},
		Status: "confirmed",
	}

	event, err := ToEvent(calEvent, "user@example.com", "primary",
		WithTimeProvider(testTimeProvider(testTime)))
	if err != nil {
		t.Fatalf("ToEvent failed: %v", err)
	}

	// Assertions
	if event.Type != models.EventTypeCalendarUpcoming {
		t.Errorf("expected type %v, got %v", models.EventTypeCalendarUpcoming, event.Type)
	}
	if event.Priority != models.PriorityHigh {
		t.Errorf("expected priority %v, got %v", models.PriorityHigh, event.Priority)
	}
	if event.RequiresAction {
		t.Error("expected RequiresAction=false for upcoming event")
	}
	expectedTitle := "Starting soon: Team Standup"
	if event.Title != expectedTitle {
		t.Errorf("expected title %q, got %q", expectedTitle, event.Title)
	}
	if event.Source != models.SourceGoogleCalendar {
		t.Errorf("expected source %v, got %v", models.SourceGoogleCalendar, event.Source)
	}
}

func TestToEvent_EventTypeNeedsRSVP(t *testing.T) {
	// Event in 2 hours, user hasn't responded
	startTime := testTime.Add(2 * time.Hour)
	endTime := startTime.Add(1 * time.Hour)

	calEvent := &CalendarEvent{
		ID:       "event-456",
		Summary:  "Product Review",
		HTMLLink: "https://calendar.google.com/event?eid=def456",
		Start: EventTime{
			DateTime: startTime,
		},
		End: EventTime{
			DateTime: endTime,
		},
		Organizer: Person{
			Email:       "pm@example.com",
			DisplayName: "PM",
		},
		Attendees: []Attendee{
			{Email: "user@example.com", ResponseStatus: "needsAction", Self: true},
			{Email: "pm@example.com", ResponseStatus: "accepted"},
		},
		Status: "confirmed",
	}

	event, err := ToEvent(calEvent, "user@example.com", "primary",
		WithTimeProvider(testTimeProvider(testTime)))
	if err != nil {
		t.Fatalf("ToEvent failed: %v", err)
	}

	if event.Type != models.EventTypeCalendarNeedsRSVP {
		t.Errorf("expected type %v, got %v", models.EventTypeCalendarNeedsRSVP, event.Type)
	}
	if event.Priority != models.PriorityMedium {
		t.Errorf("expected priority %v, got %v", models.PriorityMedium, event.Priority)
	}
	if !event.RequiresAction {
		t.Error("expected RequiresAction=true for needs RSVP event")
	}
	expectedTitle := "Needs RSVP: Product Review"
	if event.Title != expectedTitle {
		t.Errorf("expected title %q, got %q", expectedTitle, event.Title)
	}
}

func TestToEvent_EventTypeOrganizerPending(t *testing.T) {
	// User is organizer with pending RSVPs from others
	startTime := testTime.Add(3 * time.Hour)
	endTime := startTime.Add(1 * time.Hour)

	calEvent := &CalendarEvent{
		ID:       "event-789",
		Summary:  "Engineering Sync",
		HTMLLink: "https://calendar.google.com/event?eid=ghi789",
		Start: EventTime{
			DateTime: startTime,
		},
		End: EventTime{
			DateTime: endTime,
		},
		Organizer: Person{
			Email:       "user@example.com",
			DisplayName: "User",
			Self:        true,
		},
		Attendees: []Attendee{
			{Email: "user@example.com", ResponseStatus: "accepted", Self: true, Organizer: true},
			{Email: "dev1@example.com", ResponseStatus: "needsAction"},
			{Email: "dev2@example.com", ResponseStatus: "accepted"},
		},
		Status: "confirmed",
	}

	event, err := ToEvent(calEvent, "user@example.com", "primary",
		WithTimeProvider(testTimeProvider(testTime)))
	if err != nil {
		t.Fatalf("ToEvent failed: %v", err)
	}

	if event.Type != models.EventTypeCalendarOrganizerPending {
		t.Errorf("expected type %v, got %v", models.EventTypeCalendarOrganizerPending, event.Type)
	}
	if event.Priority != models.PriorityMedium {
		t.Errorf("expected priority %v, got %v", models.PriorityMedium, event.Priority)
	}
	if !event.RequiresAction {
		t.Error("expected RequiresAction=true for organizer pending event")
	}
	expectedTitle := "Awaiting RSVPs: Engineering Sync"
	if event.Title != expectedTitle {
		t.Errorf("expected title %q, got %q", expectedTitle, event.Title)
	}

	// Check pending_rsvps metadata
	pendingRSVPs, ok := event.Metadata["pending_rsvps"].([]string)
	if !ok {
		t.Fatal("expected pending_rsvps to be []string")
	}
	if len(pendingRSVPs) != 1 || pendingRSVPs[0] != "dev1@example.com" {
		t.Errorf("expected pending_rsvps [dev1@example.com], got %v", pendingRSVPs)
	}
}

func TestToEvent_EventTypeTentative(t *testing.T) {
	startTime := testTime.Add(2 * time.Hour)
	endTime := startTime.Add(1 * time.Hour)

	calEvent := &CalendarEvent{
		ID:       "event-101",
		Summary:  "Optional Meeting",
		HTMLLink: "https://calendar.google.com/event?eid=jkl101",
		Start: EventTime{
			DateTime: startTime,
		},
		End: EventTime{
			DateTime: endTime,
		},
		Organizer: Person{
			Email:       "organizer@example.com",
			DisplayName: "Organizer",
		},
		Attendees: []Attendee{
			{Email: "user@example.com", ResponseStatus: "tentative", Self: true},
			{Email: "organizer@example.com", ResponseStatus: "accepted"},
		},
		Status: "confirmed",
	}

	event, err := ToEvent(calEvent, "user@example.com", "primary",
		WithTimeProvider(testTimeProvider(testTime)))
	if err != nil {
		t.Fatalf("ToEvent failed: %v", err)
	}

	if event.Type != models.EventTypeCalendarTentative {
		t.Errorf("expected type %v, got %v", models.EventTypeCalendarTentative, event.Type)
	}
	if event.Priority != models.PriorityMedium {
		t.Errorf("expected priority %v, got %v", models.PriorityMedium, event.Priority)
	}
	if event.RequiresAction {
		t.Error("expected RequiresAction=false for tentative event")
	}
	expectedTitle := "Tentative: Optional Meeting"
	if event.Title != expectedTitle {
		t.Errorf("expected title %q, got %q", expectedTitle, event.Title)
	}
}

func TestToEvent_EventTypeMeeting(t *testing.T) {
	// User accepted, not starting within 1 hour
	startTime := testTime.Add(2 * time.Hour)
	endTime := startTime.Add(1 * time.Hour)

	calEvent := &CalendarEvent{
		ID:       "event-202",
		Summary:  "Team Meeting",
		HTMLLink: "https://calendar.google.com/event?eid=mno202",
		Start: EventTime{
			DateTime: startTime,
		},
		End: EventTime{
			DateTime: endTime,
		},
		Organizer: Person{
			Email:       "lead@example.com",
			DisplayName: "Lead",
		},
		Attendees: []Attendee{
			{Email: "user@example.com", ResponseStatus: "accepted", Self: true},
			{Email: "lead@example.com", ResponseStatus: "accepted"},
		},
		Status: "confirmed",
	}

	event, err := ToEvent(calEvent, "user@example.com", "primary",
		WithTimeProvider(testTimeProvider(testTime)))
	if err != nil {
		t.Fatalf("ToEvent failed: %v", err)
	}

	if event.Type != models.EventTypeCalendarMeeting {
		t.Errorf("expected type %v, got %v", models.EventTypeCalendarMeeting, event.Type)
	}
	if event.Priority != models.PriorityMedium {
		t.Errorf("expected priority %v, got %v", models.PriorityMedium, event.Priority)
	}
	if event.RequiresAction {
		t.Error("expected RequiresAction=false for accepted meeting")
	}
	// Meeting type uses plain summary
	expectedTitle := "Team Meeting"
	if event.Title != expectedTitle {
		t.Errorf("expected title %q, got %q", expectedTitle, event.Title)
	}
}

func TestToEvent_EventTypeAllDay(t *testing.T) {
	calEvent := &CalendarEvent{
		ID:       "event-303",
		Summary:  "Company Holiday",
		HTMLLink: "https://calendar.google.com/event?eid=pqr303",
		Start: EventTime{
			Date: "2025-12-25",
		},
		End: EventTime{
			Date: "2025-12-26",
		},
		Organizer: Person{
			Email:       "hr@example.com",
			DisplayName: "HR",
		},
		Attendees: []Attendee{
			{Email: "user@example.com", ResponseStatus: "accepted", Self: true},
		},
		Status: "confirmed",
	}

	event, err := ToEvent(calEvent, "user@example.com", "primary",
		WithTimeProvider(testTimeProvider(testTime)))
	if err != nil {
		t.Fatalf("ToEvent failed: %v", err)
	}

	if event.Type != models.EventTypeCalendarAllDay {
		t.Errorf("expected type %v, got %v", models.EventTypeCalendarAllDay, event.Type)
	}
	if event.Priority != models.PriorityInfo {
		t.Errorf("expected priority %v, got %v", models.PriorityInfo, event.Priority)
	}
	if event.RequiresAction {
		t.Error("expected RequiresAction=false for all-day event")
	}
	expectedTitle := "Company Holiday (all day)"
	if event.Title != expectedTitle {
		t.Errorf("expected title %q, got %q", expectedTitle, event.Title)
	}

	// Check is_all_day metadata
	isAllDay, ok := event.Metadata["is_all_day"].(bool)
	if !ok || !isAllDay {
		t.Error("expected is_all_day=true in metadata")
	}
}

func TestToEvent_UpcomingOverridesNeedsRSVP(t *testing.T) {
	// Event starts in 30 minutes AND user hasn't responded
	// Upcoming should take priority
	startTime := testTime.Add(30 * time.Minute)
	endTime := startTime.Add(1 * time.Hour)

	calEvent := &CalendarEvent{
		ID:       "event-404",
		Summary:  "Urgent Meeting",
		HTMLLink: "https://calendar.google.com/event?eid=stu404",
		Start: EventTime{
			DateTime: startTime,
		},
		End: EventTime{
			DateTime: endTime,
		},
		Organizer: Person{
			Email:       "boss@example.com",
			DisplayName: "Boss",
		},
		Attendees: []Attendee{
			{Email: "user@example.com", ResponseStatus: "needsAction", Self: true},
			{Email: "boss@example.com", ResponseStatus: "accepted"},
		},
		Status: "confirmed",
	}

	event, err := ToEvent(calEvent, "user@example.com", "primary",
		WithTimeProvider(testTimeProvider(testTime)))
	if err != nil {
		t.Fatalf("ToEvent failed: %v", err)
	}

	// Upcoming should override needs RSVP
	if event.Type != models.EventTypeCalendarUpcoming {
		t.Errorf("expected type %v, got %v", models.EventTypeCalendarUpcoming, event.Type)
	}
	if event.Priority != models.PriorityHigh {
		t.Errorf("expected priority %v, got %v", models.PriorityHigh, event.Priority)
	}
}

func TestToEvent_Metadata(t *testing.T) {
	startTime := testTime.Add(2 * time.Hour)
	endTime := startTime.Add(1 * time.Hour)

	calEvent := &CalendarEvent{
		ID:          "event-meta",
		Summary:     "Metadata Test",
		Description: "Test description for metadata",
		Location:    "Conference Room A",
		HTMLLink:    "https://calendar.google.com/event?eid=meta123",
		Start: EventTime{
			DateTime: startTime,
		},
		End: EventTime{
			DateTime: endTime,
		},
		Organizer: Person{
			Email:       "organizer@example.com",
			DisplayName: "Organizer Name",
		},
		Attendees: []Attendee{
			{Email: "user@example.com", DisplayName: "User", ResponseStatus: "accepted", Self: true},
			{Email: "other@example.com", DisplayName: "Other", ResponseStatus: "accepted"},
		},
		ConferenceData: &ConferenceData{
			EntryPoints: []EntryPoint{
				{EntryPointType: "video", URI: "https://meet.google.com/abc-def-ghi"},
			},
		},
		Status: "confirmed",
	}

	event, err := ToEvent(calEvent, "user@example.com", "work-calendar",
		WithTimeProvider(testTimeProvider(testTime)))
	if err != nil {
		t.Fatalf("ToEvent failed: %v", err)
	}

	// Check required metadata fields
	//nolint:govet // Field alignment is not important in test code
	tests := []struct {
		key      string
		expected any
	}{
		{"calendar_id", "work-calendar"},
		{"event_id", "event-meta"},
		{"is_all_day", false},
		{"organizer_email", "organizer@example.com"},
		{"organizer_name", "Organizer Name"},
		{"attendee_count", 2},
		{"response_status", "accepted"},
		{"location", "Conference Room A"},
		{"conference_url", "https://meet.google.com/abc-def-ghi"},
		{"description", "Test description for metadata"},
	}

	for _, tc := range tests {
		t.Run(tc.key, func(t *testing.T) {
			val, ok := event.Metadata[tc.key]
			if !ok {
				t.Errorf("metadata key %q not found", tc.key)
				return
			}
			if val != tc.expected {
				t.Errorf("metadata[%q] = %v, want %v", tc.key, val, tc.expected)
			}
		})
	}

	// Check attendees structure
	attendees, ok := event.Metadata["attendees"].([]map[string]string)
	if !ok {
		t.Fatal("attendees should be []map[string]string")
	}
	if len(attendees) != 2 {
		t.Errorf("expected 2 attendees, got %d", len(attendees))
	}
}

func TestToEvent_NoTitle(t *testing.T) {
	startTime := testTime.Add(2 * time.Hour)
	endTime := startTime.Add(1 * time.Hour)

	calEvent := &CalendarEvent{
		ID:      "event-notitle",
		Summary: "", // Empty summary
		Start: EventTime{
			DateTime: startTime,
		},
		End: EventTime{
			DateTime: endTime,
		},
		Organizer: Person{
			Email: "test@example.com",
		},
		Attendees: []Attendee{
			{Email: "user@example.com", ResponseStatus: "accepted", Self: true},
		},
		HTMLLink: "https://calendar.google.com/event?eid=notitle",
		Status:   "confirmed",
	}

	event, err := ToEvent(calEvent, "user@example.com", "primary",
		WithTimeProvider(testTimeProvider(testTime)))
	if err != nil {
		t.Fatalf("ToEvent failed: %v", err)
	}

	expectedTitle := "(No title)"
	if event.Title != expectedTitle {
		t.Errorf("expected title %q, got %q", expectedTitle, event.Title)
	}
}

func TestToEvent_LongTitle(t *testing.T) {
	startTime := testTime.Add(2 * time.Hour)
	endTime := startTime.Add(1 * time.Hour)

	// Create a summary longer than 100 chars
	longSummary := "This is a very long meeting title that exceeds the one hundred character limit imposed by the EFA 0001 specification"

	calEvent := &CalendarEvent{
		ID:      "event-long",
		Summary: longSummary,
		Start: EventTime{
			DateTime: startTime,
		},
		End: EventTime{
			DateTime: endTime,
		},
		Organizer: Person{
			Email: "test@example.com",
		},
		Attendees: []Attendee{
			{Email: "user@example.com", ResponseStatus: "accepted", Self: true},
		},
		HTMLLink: "https://calendar.google.com/event?eid=long",
		Status:   "confirmed",
	}

	event, err := ToEvent(calEvent, "user@example.com", "primary",
		WithTimeProvider(testTimeProvider(testTime)))
	if err != nil {
		t.Fatalf("ToEvent failed: %v", err)
	}

	if len(event.Title) > 100 {
		t.Errorf("title should be truncated to 100 chars, got %d", len(event.Title))
	}
	if event.Title[len(event.Title)-3:] != "..." {
		t.Error("truncated title should end with ...")
	}
}

func TestToEvent_LongDescription(t *testing.T) {
	startTime := testTime.Add(2 * time.Hour)
	endTime := startTime.Add(1 * time.Hour)

	// Create a description longer than 500 chars
	longDesc := ""
	for i := 0; i < 60; i++ {
		longDesc += "Description "
	}

	calEvent := &CalendarEvent{
		ID:          "event-longdesc",
		Summary:     "Test Event",
		Description: longDesc,
		Start: EventTime{
			DateTime: startTime,
		},
		End: EventTime{
			DateTime: endTime,
		},
		Organizer: Person{
			Email: "test@example.com",
		},
		Attendees: []Attendee{
			{Email: "user@example.com", ResponseStatus: "accepted", Self: true},
		},
		HTMLLink: "https://calendar.google.com/event?eid=longdesc",
		Status:   "confirmed",
	}

	event, err := ToEvent(calEvent, "user@example.com", "primary",
		WithTimeProvider(testTimeProvider(testTime)))
	if err != nil {
		t.Fatalf("ToEvent failed: %v", err)
	}

	desc, ok := event.Metadata["description"].(string)
	if !ok {
		t.Fatal("description should be a string")
	}
	if len(desc) > 500 {
		t.Errorf("description should be truncated to 500 chars, got %d", len(desc))
	}
	if desc[len(desc)-3:] != "..." {
		t.Error("truncated description should end with ...")
	}
}

func TestToEvent_Validation(t *testing.T) {
	startTime := testTime.Add(2 * time.Hour)
	endTime := startTime.Add(1 * time.Hour)

	calEvent := &CalendarEvent{
		ID:      "event-valid",
		Summary: "Valid Event",
		Start: EventTime{
			DateTime: startTime,
		},
		End: EventTime{
			DateTime: endTime,
		},
		Organizer: Person{
			Email: "organizer@example.com",
		},
		Attendees: []Attendee{
			{Email: "user@example.com", ResponseStatus: "accepted", Self: true},
		},
		HTMLLink: "https://calendar.google.com/event?eid=valid",
		Status:   "confirmed",
	}

	event, err := ToEvent(calEvent, "user@example.com", "primary",
		WithTimeProvider(testTimeProvider(testTime)))
	if err != nil {
		t.Fatalf("ToEvent failed: %v", err)
	}

	// Validate should pass
	if err := event.Validate(); err != nil {
		t.Errorf("event should be valid: %v", err)
	}

	// Check all required fields are set
	if event.Type == "" {
		t.Error("Type should not be empty")
	}
	if event.Title == "" {
		t.Error("Title should not be empty")
	}
	if event.Source != models.SourceGoogleCalendar {
		t.Error("Source should be google_calendar")
	}
	if event.Author.Username == "" {
		t.Error("Author.Username should not be empty")
	}
	if event.Timestamp.IsZero() {
		t.Error("Timestamp should not be zero")
	}
	if !event.Priority.IsValid() {
		t.Error("Priority should be valid (1-5)")
	}
}

func TestToEvents_SkipsCanceled(t *testing.T) {
	startTime := testTime.Add(2 * time.Hour)
	endTime := startTime.Add(1 * time.Hour)

	calEvents := []CalendarEvent{
		{
			ID:      "event-1",
			Summary: "Active Event",
			Start:   EventTime{DateTime: startTime},
			End:     EventTime{DateTime: endTime},
			Organizer: Person{
				Email: "test@example.com",
			},
			Attendees: []Attendee{
				{Email: "user@example.com", ResponseStatus: "accepted", Self: true},
			},
			HTMLLink: "https://calendar.google.com/event?eid=1",
			Status:   "confirmed",
		},
		{
			ID:      "event-2",
			Summary: "Canceled Event",
			Start:   EventTime{DateTime: startTime},
			End:     EventTime{DateTime: endTime},
			Organizer: Person{
				Email: "test@example.com",
			},
			Attendees: []Attendee{
				{Email: "user@example.com", ResponseStatus: "accepted", Self: true},
			},
			HTMLLink: "https://calendar.google.com/event?eid=2",
			Status:   "cancelled", //nolint:misspell // Google Calendar API uses British English
		},
	}

	events, errs := ToEvents(calEvents, "user@example.com", "primary",
		WithTimeProvider(testTimeProvider(testTime)))

	if len(errs) > 0 {
		t.Errorf("unexpected errors: %v", errs)
	}
	if len(events) != 1 {
		t.Errorf("expected 1 event (canceled should be skipped), got %d", len(events))
	}
	if events[0].Metadata["event_id"] != "event-1" {
		t.Error("wrong event returned")
	}
}

func TestDetermineEventType_Priority(t *testing.T) {
	// Test that upcoming events within 1 hour take priority
	now := testTime
	startSoon := now.Add(45 * time.Minute)

	// Event starting soon but user needs RSVP - upcoming should win
	calEvent := &CalendarEvent{
		Start: EventTime{DateTime: startSoon},
		Organizer: Person{
			Email: "other@example.com",
		},
		Attendees: []Attendee{
			{Email: "user@example.com", ResponseStatus: "needsAction", Self: true},
		},
	}

	eventType, _ := determineEventType(calEvent, "user@example.com", now)
	if eventType != models.EventTypeCalendarUpcoming {
		t.Errorf("expected %v for event starting within 1hr, got %v",
			models.EventTypeCalendarUpcoming, eventType)
	}

	// Event NOT starting soon but user needs RSVP - needs RSVP should apply
	startLater := now.Add(2 * time.Hour)
	calEvent.Start.DateTime = startLater
	eventType, _ = determineEventType(calEvent, "user@example.com", now)
	if eventType != models.EventTypeCalendarNeedsRSVP {
		t.Errorf("expected %v for event not starting soon with needsAction, got %v",
			models.EventTypeCalendarNeedsRSVP, eventType)
	}
}

func TestBuildAuthor_FallbackToEmail(t *testing.T) {
	calEvent := &CalendarEvent{
		Organizer: Person{
			Email:       "nondisplayname@example.com",
			DisplayName: "",
		},
	}

	author := buildAuthor(calEvent)

	if author.Name != "nondisplayname@example.com" {
		t.Errorf("expected Name to fallback to email, got %q", author.Name)
	}
	if author.Username != "nondisplayname@example.com" {
		t.Errorf("expected Username to be email, got %q", author.Username)
	}
}
