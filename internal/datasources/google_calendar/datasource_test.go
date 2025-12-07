package google_calendar

import (
	"errors"
	"testing"
	"time"

	"github.com/dakaneye/kora/internal/datasources"
	"github.com/dakaneye/kora/internal/models"
)

// TestGoogleCalendarDataSource_Name verifies the datasource returns the correct name.
func TestGoogleCalendarDataSource_Name(t *testing.T) {
	ds := &GoogleCalendarDataSource{}

	if got := ds.Name(); got != "google-calendar" {
		t.Errorf("Name() = %q, want %q", got, "google-calendar")
	}
}

// TestGoogleCalendarDataSource_Service verifies the datasource returns the correct service.
func TestGoogleCalendarDataSource_Service(t *testing.T) {
	ds := &GoogleCalendarDataSource{}

	if got := ds.Service(); got != models.SourceGoogleCalendar {
		t.Errorf("Service() = %q, want %q", got, models.SourceGoogleCalendar)
	}
}

// TestNewGoogleCalendarDataSource_NilAuthProvider verifies constructor rejects nil auth provider.
func TestNewGoogleCalendarDataSource_NilAuthProvider(t *testing.T) {
	ds, err := NewGoogleCalendarDataSource(nil)
	if err == nil {
		t.Error("expected error for nil auth provider, got nil")
	}
	if ds != nil {
		t.Error("expected nil datasource on error")
	}
	if !errors.Is(err, errors.New("google calendar: auth provider required")) {
		wantMsg := "google calendar: auth provider required"
		if err.Error() != wantMsg {
			t.Errorf("error message = %q, want %q", err.Error(), wantMsg)
		}
	}
}

// TestFetch_InvalidOptions verifies invalid options are rejected.
func TestFetch_InvalidOptions(t *testing.T) {
	// Test only validation errors by checking FetchOptions.Validate directly
	tests := []struct {
		name    string
		opts    datasources.FetchOptions
		wantErr bool
	}{
		{
			name: "zero Since time",
			opts: datasources.FetchOptions{
				Since: time.Time{},
			},
			wantErr: true,
		},
		{
			name: "valid Since time",
			opts: datasources.FetchOptions{
				Since: time.Date(2025, 12, 7, 0, 0, 0, 0, time.UTC),
			},
			wantErr: false,
		},
		{
			name: "negative Limit",
			opts: datasources.FetchOptions{
				Since: time.Date(2025, 12, 7, 0, 0, 0, 0, time.UTC),
				Limit: -1,
			},
			wantErr: true,
		},
		{
			name: "valid Limit",
			opts: datasources.FetchOptions{
				Since: time.Date(2025, 12, 7, 0, 0, 0, 0, time.UTC),
				Limit: 100,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.opts.Validate()

			if tt.wantErr {
				if err == nil {
					t.Error("expected error for invalid options, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("expected no error, got %v", err)
				}
			}
		})
	}
}

// TestFilterCalendarEvents verifies filtering logic.
func TestFilterCalendarEvents(t *testing.T) {
	ds := &GoogleCalendarDataSource{
		email: "test@example.com",
	}

	tests := []struct {
		name      string
		events    []CalendarEvent
		wantCount int
		desc      string
	}{
		{
			name: "filter declined events",
			events: []CalendarEvent{
				createTestEventWithResponse("e1", "Meeting 1", time.Now(), "accepted"),
				createTestEventWithResponse("e2", "Meeting 2", time.Now(), "declined"),
				createTestEventWithResponse("e3", "Meeting 3", time.Now(), "tentative"),
			},
			wantCount: 2,
			desc:      "declined event should be filtered",
		},
		{
			name: "filter cancelled events",
			events: []CalendarEvent{
				{
					ID:      "e1",
					Summary: "Confirmed",
					Status:  "confirmed",
					Start:   EventTime{DateTime: time.Now()},
					End:     EventTime{DateTime: time.Now().Add(time.Hour)},
				},
				{
					ID:      "e2",
					Summary: "Cancelled",
					Status:  "cancelled",
					Start:   EventTime{DateTime: time.Now()},
					End:     EventTime{DateTime: time.Now().Add(time.Hour)},
				},
			},
			wantCount: 1,
			desc:      "cancelled event should be filtered",
		},
		{
			name: "keep all valid statuses",
			events: []CalendarEvent{
				createTestEventWithResponse("e1", "Accepted", time.Now(), "accepted"),
				createTestEventWithResponse("e2", "Tentative", time.Now(), "tentative"),
				createTestEventWithResponse("e3", "Needs Action", time.Now(), "needsAction"),
				{
					ID:      "e4",
					Summary: "No response (organizer)",
					Status:  "confirmed",
					Start:   EventTime{DateTime: time.Now()},
					End:     EventTime{DateTime: time.Now().Add(time.Hour)},
					Organizer: Person{
						Email: "test@example.com",
						Self:  true,
					},
				},
			},
			wantCount: 4,
			desc:      "all valid statuses should be kept",
		},
		{
			name:      "empty input",
			events:    []CalendarEvent{},
			wantCount: 0,
			desc:      "empty input returns empty output",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filtered := ds.filterCalendarEvents(tt.events)

			if len(filtered) != tt.wantCount {
				t.Errorf("filterCalendarEvents() returned %d events, want %d: %s",
					len(filtered), tt.wantCount, tt.desc)
			}

			// Verify no declined events in result
			for _, event := range filtered {
				if event.GetUserResponseStatus() == "declined" {
					t.Errorf("found declined event in filtered results: %s", event.Summary)
				}
				if event.Status == "cancelled" {
					t.Errorf("found cancelled event in filtered results: %s", event.Summary)
				}
			}
		})
	}
}

// TestApplyFilters verifies FetchOptions filters are applied correctly.
func TestApplyFilters(t *testing.T) {
	ds := &GoogleCalendarDataSource{
		email: "test@example.com",
	}

	// Create test events
	event1 := models.Event{
		Type:           models.EventTypeCalendarMeeting,
		Title:          "Regular Meeting",
		Priority:       models.PriorityMedium,
		RequiresAction: false,
		Source:         models.SourceGoogleCalendar,
		URL:            "https://calendar.google.com/event?eid=1",
		Timestamp:      time.Now(),
		Author:         models.Person{Name: "Organizer", Username: "org@example.com"},
		Metadata:       map[string]any{},
	}

	event2 := models.Event{
		Type:           models.EventTypeCalendarNeedsRSVP,
		Title:          "Needs RSVP",
		Priority:       models.PriorityMedium,
		RequiresAction: true,
		Source:         models.SourceGoogleCalendar,
		URL:            "https://calendar.google.com/event?eid=2",
		Timestamp:      time.Now(),
		Author:         models.Person{Name: "Organizer", Username: "org@example.com"},
		Metadata:       map[string]any{},
	}

	event3 := models.Event{
		Type:           models.EventTypeCalendarUpcoming,
		Title:          "Starting Soon",
		Priority:       models.PriorityHigh,
		RequiresAction: false,
		Source:         models.SourceGoogleCalendar,
		URL:            "https://calendar.google.com/event?eid=3",
		Timestamp:      time.Now(),
		Author:         models.Person{Name: "Organizer", Username: "org@example.com"},
		Metadata:       map[string]any{},
	}

	allEvents := []models.Event{event1, event2, event3}

	tests := []struct {
		name      string
		events    []models.Event
		opts      datasources.FetchOptions
		wantCount int
		desc      string
	}{
		{
			name:      "no filter - return all",
			events:    allEvents,
			opts:      datasources.FetchOptions{},
			wantCount: 3,
			desc:      "no filter should return all events",
		},
		{
			name:   "filter by event type",
			events: allEvents,
			opts: datasources.FetchOptions{
				Filter: &datasources.FetchFilter{
					EventTypes: []models.EventType{models.EventTypeCalendarNeedsRSVP},
				},
			},
			wantCount: 1,
			desc:      "should return only needs_rsvp events",
		},
		{
			name:   "filter by min priority",
			events: allEvents,
			opts: datasources.FetchOptions{
				Filter: &datasources.FetchFilter{
					MinPriority: models.PriorityHigh, // Only high priority (lower number)
				},
			},
			wantCount: 1,
			desc:      "should return only high priority events",
		},
		{
			name:   "filter by requires action",
			events: allEvents,
			opts: datasources.FetchOptions{
				Filter: &datasources.FetchFilter{
					RequiresAction: true,
				},
			},
			wantCount: 1,
			desc:      "should return only events requiring action",
		},
		{
			name:   "multiple event types",
			events: allEvents,
			opts: datasources.FetchOptions{
				Filter: &datasources.FetchFilter{
					EventTypes: []models.EventType{
						models.EventTypeCalendarMeeting,
						models.EventTypeCalendarUpcoming,
					},
				},
			},
			wantCount: 2,
			desc:      "should return events matching either type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filtered := ds.applyFilters(tt.events, tt.opts)

			if len(filtered) != tt.wantCount {
				t.Errorf("applyFilters() returned %d events, want %d: %s",
					len(filtered), tt.wantCount, tt.desc)
			}
		})
	}
}

// TestLookAheadDuration verifies the look-ahead window constant.
func TestLookAheadDuration(t *testing.T) {
	expected := 7 * 24 * time.Hour // 7 days

	if lookAheadDuration != expected {
		t.Errorf("lookAheadDuration = %v, want %v", lookAheadDuration, expected)
	}
}

// TestWithCalendarIDs verifies the calendar IDs option.
func TestWithCalendarIDs(t *testing.T) {
	ids := []string{"cal1", "cal2", "cal3"}

	ds := &GoogleCalendarDataSource{}
	opt := WithCalendarIDs(ids)
	opt(ds)

	if len(ds.calendarIDs) != len(ids) {
		t.Errorf("calendarIDs length = %d, want %d", len(ds.calendarIDs), len(ids))
	}

	for i, id := range ids {
		if ds.calendarIDs[i] != id {
			t.Errorf("calendarIDs[%d] = %q, want %q", i, ds.calendarIDs[i], id)
		}
	}
}

// TestWithTimeProviderOption verifies the time provider option.
func TestWithTimeProviderOption(t *testing.T) {
	fixedTime := time.Date(2025, 12, 7, 10, 0, 0, 0, time.UTC)
	mockProvider := func() time.Time {
		return fixedTime
	}

	ds := &GoogleCalendarDataSource{
		timeProvider: defaultTimeProvider, // Start with default
	}

	opt := WithTimeProviderOption(mockProvider)
	opt(ds)

	// Test that the time provider was set
	got := ds.timeProvider()
	if !got.Equal(fixedTime) {
		t.Errorf("timeProvider() = %v, want %v", got, fixedTime)
	}
}

// Helper functions

func createTestEventWithResponse(id, summary string, start time.Time, responseStatus string) CalendarEvent {
	return CalendarEvent{
		ID:      id,
		Summary: summary,
		Start: EventTime{
			DateTime:    start,
			DateTimeRaw: start.Format(time.RFC3339),
		},
		End: EventTime{
			DateTime:    start.Add(time.Hour),
			DateTimeRaw: start.Add(time.Hour).Format(time.RFC3339),
		},
		Status: "confirmed",
		Attendees: []Attendee{
			{
				Email:          "test@example.com",
				ResponseStatus: responseStatus,
				Self:           true,
			},
		},
		Organizer: Person{
			Email:       "organizer@example.com",
			DisplayName: "Organizer",
		},
		HTMLLink: "https://calendar.google.com/event?eid=" + id,
	}
}
