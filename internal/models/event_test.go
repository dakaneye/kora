package models

import (
	"strings"
	"testing"
	"time"
)

func TestEvent_Validate(t *testing.T) {
	//nolint:govet // test struct field order prioritizes readability
	tests := []struct {
		name    string
		event   Event
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid GitHub PR review event",
			event: Event{
				Type:           EventTypePRReview,
				Title:          "Review requested: Add feature",
				Source:         SourceGitHub,
				URL:            "https://github.com/owner/repo/pull/1",
				Author:         Person{Username: "testuser"},
				Timestamp:      time.Now(),
				Priority:       PriorityHigh,
				RequiresAction: true,
				Metadata: map[string]any{
					"repo":   "owner/repo",
					"number": 1,
				},
			},
			wantErr: false,
		},
		{
			name: "empty title fails",
			event: Event{
				Type:      EventTypePRReview,
				Title:     "",
				Source:    SourceGitHub,
				URL:       "https://github.com/owner/repo/pull/1",
				Author:    Person{Username: "testuser"},
				Timestamp: time.Now(),
				Priority:  PriorityHigh,
			},
			wantErr: true,
			errMsg:  "title must be 1-100 characters",
		},
		{
			name: "title exceeds 100 chars fails",
			event: Event{
				Type:      EventTypePRReview,
				Title:     strings.Repeat("a", 101),
				Source:    SourceGitHub,
				URL:       "https://github.com/owner/repo/pull/1",
				Author:    Person{Username: "testuser"},
				Timestamp: time.Now(),
				Priority:  PriorityHigh,
			},
			wantErr: true,
			errMsg:  "title must be 1-100 characters",
		},
		{
			name: "invalid event type fails",
			event: Event{
				Type:      EventType("invalid"),
				Title:     "Test",
				Source:    SourceGitHub,
				URL:       "https://github.com/owner/repo/pull/1",
				Author:    Person{Username: "testuser"},
				Timestamp: time.Now(),
				Priority:  PriorityHigh,
			},
			wantErr: true,
			errMsg:  "invalid event type",
		},
		{
			name: "invalid source fails",
			event: Event{
				Type:      EventTypePRReview,
				Title:     "Test",
				Source:    Source("invalid"),
				URL:       "https://github.com/owner/repo/pull/1",
				Author:    Person{Username: "testuser"},
				Timestamp: time.Now(),
				Priority:  PriorityHigh,
			},
			wantErr: true,
			errMsg:  "invalid source",
		},
		{
			name: "non-absolute URL fails",
			event: Event{
				Type:      EventTypePRReview,
				Title:     "Test",
				Source:    SourceGitHub,
				URL:       "/relative/path",
				Author:    Person{Username: "testuser"},
				Timestamp: time.Now(),
				Priority:  PriorityHigh,
			},
			wantErr: true,
			errMsg:  "URL must be absolute with scheme and host",
		},
		{
			name: "non-http/https scheme fails",
			event: Event{
				Type:      EventTypePRReview,
				Title:     "Test",
				Source:    SourceGitHub,
				URL:       "ftp://github.com/owner/repo",
				Author:    Person{Username: "testuser"},
				Timestamp: time.Now(),
				Priority:  PriorityHigh,
			},
			wantErr: true,
			errMsg:  "URL scheme must be http or https",
		},
		{
			name: "empty URL is allowed",
			event: Event{
				Type:      EventTypePRReview,
				Title:     "Test",
				Source:    SourceGitHub,
				URL:       "",
				Author:    Person{Username: "testuser"},
				Timestamp: time.Now(),
				Priority:  PriorityHigh,
			},
			wantErr: false,
		},
		{
			name: "empty author username fails",
			event: Event{
				Type:      EventTypePRReview,
				Title:     "Test",
				Source:    SourceGitHub,
				URL:       "https://github.com/owner/repo/pull/1",
				Author:    Person{Username: ""},
				Timestamp: time.Now(),
				Priority:  PriorityHigh,
			},
			wantErr: true,
			errMsg:  "author username required",
		},
		{
			name: "zero timestamp fails",
			event: Event{
				Type:      EventTypePRReview,
				Title:     "Test",
				Source:    SourceGitHub,
				URL:       "https://github.com/owner/repo/pull/1",
				Author:    Person{Username: "testuser"},
				Timestamp: time.Time{},
				Priority:  PriorityHigh,
			},
			wantErr: true,
			errMsg:  "timestamp required",
		},
		{
			name: "priority 0 fails",
			event: Event{
				Type:      EventTypePRReview,
				Title:     "Test",
				Source:    SourceGitHub,
				URL:       "https://github.com/owner/repo/pull/1",
				Author:    Person{Username: "testuser"},
				Timestamp: time.Now(),
				Priority:  Priority(0),
			},
			wantErr: true,
			errMsg:  "priority must be 1-5",
		},
		{
			name: "priority 6 fails",
			event: Event{
				Type:      EventTypePRReview,
				Title:     "Test",
				Source:    SourceGitHub,
				URL:       "https://github.com/owner/repo/pull/1",
				Author:    Person{Username: "testuser"},
				Timestamp: time.Now(),
				Priority:  Priority(6),
			},
			wantErr: true,
			errMsg:  "priority must be 1-5",
		},
		{
			name: "priority -1 fails",
			event: Event{
				Type:      EventTypePRReview,
				Title:     "Test",
				Source:    SourceGitHub,
				URL:       "https://github.com/owner/repo/pull/1",
				Author:    Person{Username: "testuser"},
				Timestamp: time.Now(),
				Priority:  Priority(-1),
			},
			wantErr: true,
			errMsg:  "priority must be 1-5",
		},
		{
			name: "invalid GitHub metadata key fails",
			event: Event{
				Type:      EventTypePRReview,
				Title:     "Test",
				Source:    SourceGitHub,
				URL:       "https://github.com/owner/repo/pull/1",
				Author:    Person{Username: "testuser"},
				Timestamp: time.Now(),
				Priority:  PriorityHigh,
				Metadata: map[string]any{
					"invalid_key": "value",
				},
			},
			wantErr: true,
			errMsg:  "invalid metadata keys for github",
		},
		{
			name: "valid GitHub metadata keys pass",
			event: Event{
				Type:      EventTypePRReview,
				Title:     "Test",
				Source:    SourceGitHub,
				URL:       "https://github.com/owner/repo/pull/1",
				Author:    Person{Username: "testuser"},
				Timestamp: time.Now(),
				Priority:  PriorityHigh,
				Metadata: map[string]any{
					"repo":               "owner/repo",
					"number":             1,
					"state":              "open",
					"author_login":       "testuser",
					"user_relationships": []string{"reviewer"},
					"labels":             []string{"bug"},
				},
			},
			wantErr: false,
		},
		{
			name: "multiple validation errors combined",
			event: Event{
				Type:      EventType("invalid"),
				Title:     "",
				Source:    Source("invalid"),
				URL:       "not-a-url",
				Author:    Person{Username: ""},
				Timestamp: time.Time{},
				Priority:  Priority(0),
			},
			wantErr: true,
			errMsg:  "invalid event",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.event.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Event.Validate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && err != nil && tt.errMsg != "" {
				if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("Event.Validate() error = %v, want error containing %q", err, tt.errMsg)
				}
			}
		})
	}
}

func TestEventType_IsValid(t *testing.T) {
	tests := []struct {
		name      string
		eventType EventType
		want      bool
	}{
		// GitHub event types
		{"PR review is valid", EventTypePRReview, true},
		{"PR mention is valid", EventTypePRMention, true},
		{"PR author is valid", EventTypePRAuthor, true},
		{"PR codeowner is valid", EventTypePRCodeowner, true},
		{"PR closed is valid", EventTypePRClosed, true},
		{"PR comment mention is valid", EventTypePRCommentMention, true},
		{"Issue mention is valid", EventTypeIssueMention, true},
		{"Issue assigned is valid", EventTypeIssueAssigned, true},
		// Google Calendar event types
		{"Calendar upcoming is valid", EventTypeCalendarUpcoming, true},
		{"Calendar needs RSVP is valid", EventTypeCalendarNeedsRSVP, true},
		{"Calendar organizer pending is valid", EventTypeCalendarOrganizerPending, true},
		{"Calendar tentative is valid", EventTypeCalendarTentative, true},
		{"Calendar meeting is valid", EventTypeCalendarMeeting, true},
		{"Calendar all day is valid", EventTypeCalendarAllDay, true},
		// Gmail event types
		{"Email important is valid", EventTypeEmailImportant, true},
		{"Email direct is valid", EventTypeEmailDirect, true},
		{"Email CC is valid", EventTypeEmailCC, true},
		// Invalid event types
		{"empty string is invalid", EventType(""), false},
		{"random string is invalid", EventType("random"), false},
		{"pr_comment is invalid", EventType("pr_comment"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.eventType.IsValid(); got != tt.want {
				t.Errorf("EventType.IsValid() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEventType_Constants(t *testing.T) {
	// Verify each constant exists and has expected value
	tests := []struct {
		constant EventType
		expected string
	}{
		// GitHub event types
		{EventTypePRReview, "pr_review"},
		{EventTypePRMention, "pr_mention"},
		{EventTypePRAuthor, "pr_author"},
		{EventTypePRCodeowner, "pr_codeowner"},
		{EventTypePRClosed, "pr_closed"},
		{EventTypePRCommentMention, "pr_comment_mention"},
		{EventTypeIssueMention, "issue_mention"},
		{EventTypeIssueAssigned, "issue_assigned"},
		// Google Calendar event types
		{EventTypeCalendarUpcoming, "calendar_upcoming"},
		{EventTypeCalendarNeedsRSVP, "calendar_needs_rsvp"},
		{EventTypeCalendarOrganizerPending, "calendar_organizer_pending"},
		{EventTypeCalendarTentative, "calendar_tentative"},
		{EventTypeCalendarMeeting, "calendar_meeting"},
		{EventTypeCalendarAllDay, "calendar_all_day"},
		// Gmail event types
		{EventTypeEmailImportant, "email_important"},
		{EventTypeEmailDirect, "email_direct"},
		{EventTypeEmailCC, "email_cc"},
	}

	for _, tt := range tests {
		t.Run(string(tt.constant), func(t *testing.T) {
			if string(tt.constant) != tt.expected {
				t.Errorf("EventType constant = %q, want %q", tt.constant, tt.expected)
			}
			if !tt.constant.IsValid() {
				t.Errorf("EventType constant %q should be valid", tt.constant)
			}
		})
	}
}

func TestSource_IsValid(t *testing.T) {
	tests := []struct {
		name   string
		source Source
		want   bool
	}{
		// Valid sources
		{"GitHub is valid", SourceGitHub, true},
		{"Google Calendar is valid", SourceGoogleCalendar, true},
		{"Gmail is valid", SourceGmail, true},
		// Invalid sources
		{"empty string is invalid", Source(""), false},
		{"random string is invalid", Source("random"), false},
		{"jira is invalid", Source("jira"), false},
		{"google is invalid", Source("google"), false},
		{"calendar is invalid", Source("calendar"), false},
		{"email is invalid", Source("email"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.source.IsValid(); got != tt.want {
				t.Errorf("Source.IsValid() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSource_Constants(t *testing.T) {
	// Verify each constant exists and has expected value
	tests := []struct {
		constant Source
		expected string
	}{
		{SourceGitHub, "github"},
		{SourceGoogleCalendar, "google_calendar"},
		{SourceGmail, "gmail"},
	}

	for _, tt := range tests {
		t.Run(string(tt.constant), func(t *testing.T) {
			if string(tt.constant) != tt.expected {
				t.Errorf("Source constant = %q, want %q", tt.constant, tt.expected)
			}
			if !tt.constant.IsValid() {
				t.Errorf("Source constant %q should be valid", tt.constant)
			}
		})
	}
}

// TestEvent_Validate_GoogleCalendar tests validation of Google Calendar events.
func TestEvent_Validate_GoogleCalendar(t *testing.T) {
	now := time.Now()

	//nolint:govet // test struct field order prioritizes readability
	tests := []struct {
		name    string
		event   Event
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid calendar upcoming event",
			event: Event{
				Type:           EventTypeCalendarUpcoming,
				Title:          "Team standup in 30 min",
				Source:         SourceGoogleCalendar,
				URL:            "https://calendar.google.com/event?eid=abc123",
				Author:         Person{Username: "john.doe@example.com"},
				Timestamp:      now,
				Priority:       PriorityHigh,
				RequiresAction: false,
				Metadata: map[string]any{
					"calendar_id":     "primary",
					"event_id":        "abc123",
					"start_time":      now.Add(30 * time.Minute).Format(time.RFC3339),
					"end_time":        now.Add(60 * time.Minute).Format(time.RFC3339),
					"organizer_email": "jane@example.com",
					"attendees":       []string{"john@example.com", "jane@example.com"},
				},
			},
			wantErr: false,
		},
		{
			name: "valid calendar needs RSVP event",
			event: Event{
				Type:           EventTypeCalendarNeedsRSVP,
				Title:          "Project review meeting",
				Source:         SourceGoogleCalendar,
				URL:            "https://calendar.google.com/event?eid=xyz789",
				Author:         Person{Username: "organizer@example.com"},
				Timestamp:      now,
				Priority:       PriorityMedium,
				RequiresAction: true,
				Metadata: map[string]any{
					"calendar_id":     "work",
					"event_id":        "xyz789",
					"response_status": "needsAction",
					"organizer_email": "organizer@example.com",
				},
			},
			wantErr: false,
		},
		{
			name: "valid calendar organizer pending event",
			event: Event{
				Type:           EventTypeCalendarOrganizerPending,
				Title:          "Quarterly planning",
				Source:         SourceGoogleCalendar,
				URL:            "https://calendar.google.com/event?eid=plan456",
				Author:         Person{Username: "me@example.com"},
				Timestamp:      now,
				Priority:       PriorityMedium,
				RequiresAction: true,
				Metadata: map[string]any{
					"calendar_id":     "primary",
					"event_id":        "plan456",
					"organizer_email": "me@example.com",
					"pending_rsvps":   []string{"alice@example.com", "bob@example.com"},
					"attendee_count":  5,
				},
			},
			wantErr: false,
		},
		{
			name: "valid calendar tentative event",
			event: Event{
				Type:           EventTypeCalendarTentative,
				Title:          "Optional workshop",
				Source:         SourceGoogleCalendar,
				URL:            "https://calendar.google.com/event?eid=ws999",
				Author:         Person{Username: "trainer@example.com"},
				Timestamp:      now,
				Priority:       PriorityLow,
				RequiresAction: false,
				Metadata: map[string]any{
					"calendar_id":     "primary",
					"event_id":        "ws999",
					"response_status": "tentative",
				},
			},
			wantErr: false,
		},
		{
			name: "valid calendar meeting event",
			event: Event{
				Type:           EventTypeCalendarMeeting,
				Title:          "1:1 with manager",
				Source:         SourceGoogleCalendar,
				URL:            "https://calendar.google.com/event?eid=one2one",
				Author:         Person{Username: "manager@example.com"},
				Timestamp:      now,
				Priority:       PriorityMedium,
				RequiresAction: false,
				Metadata: map[string]any{
					"calendar_id":     "primary",
					"event_id":        "one2one",
					"conference_url":  "https://meet.google.com/xyz-abcd-efg",
					"location":        "Conference Room A",
					"response_status": "accepted",
				},
			},
			wantErr: false,
		},
		{
			name: "valid calendar all day event",
			event: Event{
				Type:           EventTypeCalendarAllDay,
				Title:          "Company holiday",
				Source:         SourceGoogleCalendar,
				URL:            "https://calendar.google.com/event?eid=holiday",
				Author:         Person{Username: "hr@example.com"},
				Timestamp:      now,
				Priority:       PriorityLow,
				RequiresAction: false,
				Metadata: map[string]any{
					"calendar_id": "company",
					"event_id":    "holiday",
					"is_all_day":  true,
					"description": "Memorial Day - Office closed",
				},
			},
			wantErr: false,
		},
		{
			name: "invalid calendar metadata key fails",
			event: Event{
				Type:           EventTypeCalendarMeeting,
				Title:          "Meeting",
				Source:         SourceGoogleCalendar,
				URL:            "https://calendar.google.com/event?eid=test",
				Author:         Person{Username: "test@example.com"},
				Timestamp:      now,
				Priority:       PriorityMedium,
				RequiresAction: false,
				Metadata: map[string]any{
					"calendar_id":  "primary",
					"invalid_key":  "value",
					"another_bad":  123,
				},
			},
			wantErr: true,
			errMsg:  "invalid metadata keys for google_calendar",
		},
		{
			name: "calendar event with all allowed metadata keys",
			event: Event{
				Type:           EventTypeCalendarMeeting,
				Title:          "Comprehensive meeting",
				Source:         SourceGoogleCalendar,
				URL:            "https://calendar.google.com/event?eid=full",
				Author:         Person{Username: "test@example.com"},
				Timestamp:      now,
				Priority:       PriorityMedium,
				RequiresAction: false,
				Metadata: map[string]any{
					"calendar_id":     "primary",
					"event_id":        "full",
					"start_time":      now.Format(time.RFC3339),
					"end_time":        now.Add(time.Hour).Format(time.RFC3339),
					"is_all_day":      false,
					"organizer_email": "org@example.com",
					"organizer_name":  "Organizer Name",
					"attendees":       []string{"a@example.com"},
					"attendee_count":  1,
					"response_status": "accepted",
					"pending_rsvps":   []string{},
					"location":        "Room 1",
					"conference_url":  "https://meet.google.com/abc",
					"description":     "Meeting description",
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.event.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Event.Validate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && err != nil && tt.errMsg != "" {
				if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("Event.Validate() error = %v, want error containing %q", err, tt.errMsg)
				}
			}
		})
	}
}

// TestEvent_Validate_Gmail tests validation of Gmail events.
func TestEvent_Validate_Gmail(t *testing.T) {
	now := time.Now()

	//nolint:govet // test struct field order prioritizes readability
	tests := []struct {
		name    string
		event   Event
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid email important event",
			event: Event{
				Type:           EventTypeEmailImportant,
				Title:          "Important: Q4 Budget Review",
				Source:         SourceGmail,
				URL:            "https://mail.google.com/mail/u/0/#inbox/abc123",
				Author:         Person{Username: "cfo@example.com", Name: "CFO Name"},
				Timestamp:      now,
				Priority:       PriorityHigh,
				RequiresAction: true,
				Metadata: map[string]any{
					"message_id":      "abc123",
					"thread_id":       "thread_abc",
					"from_email":      "cfo@example.com",
					"from_name":       "CFO Name",
					"to_addresses":    []string{"team@example.com"},
					"subject":         "Important: Q4 Budget Review",
					"snippet":         "Please review the attached Q4 budget...",
					"labels":          []string{"INBOX", "IMPORTANT"},
					"is_unread":       true,
					"is_important":    true,
					"is_starred":      false,
					"has_attachments": true,
					"received_at":     now.Format(time.RFC3339),
				},
			},
			wantErr: false,
		},
		{
			name: "valid email direct event",
			event: Event{
				Type:           EventTypeEmailDirect,
				Title:          "Project update needed",
				Source:         SourceGmail,
				URL:            "https://mail.google.com/mail/u/0/#inbox/xyz789",
				Author:         Person{Username: "pm@example.com"},
				Timestamp:      now,
				Priority:       PriorityMedium,
				RequiresAction: true,
				Metadata: map[string]any{
					"message_id":   "xyz789",
					"thread_id":    "thread_xyz",
					"from_email":   "pm@example.com",
					"to_addresses": []string{"me@example.com"},
					"subject":      "Project update needed",
					"is_unread":    true,
				},
			},
			wantErr: false,
		},
		{
			name: "valid email CC event",
			event: Event{
				Type:           EventTypeEmailCC,
				Title:          "FYI: Team announcement",
				Source:         SourceGmail,
				URL:            "https://mail.google.com/mail/u/0/#inbox/fyi456",
				Author:         Person{Username: "lead@example.com"},
				Timestamp:      now,
				Priority:       PriorityLow,
				RequiresAction: false,
				Metadata: map[string]any{
					"message_id":   "fyi456",
					"thread_id":    "thread_fyi",
					"from_email":   "lead@example.com",
					"to_addresses": []string{"team@example.com"},
					"cc_addresses": []string{"me@example.com", "others@example.com"},
					"subject":      "FYI: Team announcement",
					"is_unread":    true,
				},
			},
			wantErr: false,
		},
		{
			name: "invalid gmail metadata key fails",
			event: Event{
				Type:           EventTypeEmailDirect,
				Title:          "Test email",
				Source:         SourceGmail,
				URL:            "https://mail.google.com/mail/u/0/#inbox/test",
				Author:         Person{Username: "test@example.com"},
				Timestamp:      now,
				Priority:       PriorityMedium,
				RequiresAction: false,
				Metadata: map[string]any{
					"message_id":    "test",
					"invalid_field": "should fail",
					"another_bad":   true,
				},
			},
			wantErr: true,
			errMsg:  "invalid metadata keys for gmail",
		},
		{
			name: "gmail event with all allowed metadata keys",
			event: Event{
				Type:           EventTypeEmailImportant,
				Title:          "Comprehensive email",
				Source:         SourceGmail,
				URL:            "https://mail.google.com/mail/u/0/#inbox/full",
				Author:         Person{Username: "sender@example.com"},
				Timestamp:      now,
				Priority:       PriorityHigh,
				RequiresAction: true,
				Metadata: map[string]any{
					"message_id":      "full",
					"thread_id":       "thread_full",
					"from_email":      "sender@example.com",
					"from_name":       "Sender Name",
					"to_addresses":    []string{"to1@example.com", "to2@example.com"},
					"cc_addresses":    []string{"cc@example.com"},
					"subject":         "Comprehensive email",
					"snippet":         "Email preview text...",
					"labels":          []string{"INBOX", "IMPORTANT", "STARRED"},
					"is_unread":       true,
					"is_important":    true,
					"is_starred":      true,
					"has_attachments": true,
					"received_at":     now.Format(time.RFC3339),
				},
			},
			wantErr: false,
		},
		{
			name: "gmail event with empty URL is allowed",
			event: Event{
				Type:           EventTypeEmailDirect,
				Title:          "Email without URL",
				Source:         SourceGmail,
				URL:            "",
				Author:         Person{Username: "test@example.com"},
				Timestamp:      now,
				Priority:       PriorityMedium,
				RequiresAction: false,
				Metadata: map[string]any{
					"message_id": "nourl",
					"thread_id":  "thread_nourl",
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.event.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Event.Validate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && err != nil && tt.errMsg != "" {
				if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("Event.Validate() error = %v, want error containing %q", err, tt.errMsg)
				}
			}
		})
	}
}

// TestEvent_IsCalendarEvent tests the IsCalendarEvent helper method.
func TestEvent_IsCalendarEvent(t *testing.T) {
	tests := []struct {
		name      string
		eventType EventType
		want      bool
	}{
		// Calendar event types
		{"calendar_upcoming", EventTypeCalendarUpcoming, true},
		{"calendar_needs_rsvp", EventTypeCalendarNeedsRSVP, true},
		{"calendar_organizer_pending", EventTypeCalendarOrganizerPending, true},
		{"calendar_tentative", EventTypeCalendarTentative, true},
		{"calendar_meeting", EventTypeCalendarMeeting, true},
		{"calendar_all_day", EventTypeCalendarAllDay, true},
		// Non-calendar event types
		{"pr_review", EventTypePRReview, false},
		{"pr_mention", EventTypePRMention, false},
		{"issue_mention", EventTypeIssueMention, false},
		{"email_important", EventTypeEmailImportant, false},
		{"email_direct", EventTypeEmailDirect, false},
		{"email_cc", EventTypeEmailCC, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := Event{Type: tt.eventType}
			if got := event.IsCalendarEvent(); got != tt.want {
				t.Errorf("Event.IsCalendarEvent() = %v, want %v for type %s", got, tt.want, tt.eventType)
			}
		})
	}
}

// TestEvent_IsEmailEvent tests the IsEmailEvent helper method.
func TestEvent_IsEmailEvent(t *testing.T) {
	tests := []struct {
		name      string
		eventType EventType
		want      bool
	}{
		// Email event types
		{"email_important", EventTypeEmailImportant, true},
		{"email_direct", EventTypeEmailDirect, true},
		{"email_cc", EventTypeEmailCC, true},
		// Non-email event types
		{"pr_review", EventTypePRReview, false},
		{"pr_mention", EventTypePRMention, false},
		{"issue_mention", EventTypeIssueMention, false},
		{"calendar_upcoming", EventTypeCalendarUpcoming, false},
		{"calendar_needs_rsvp", EventTypeCalendarNeedsRSVP, false},
		{"calendar_meeting", EventTypeCalendarMeeting, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := Event{Type: tt.eventType}
			if got := event.IsEmailEvent(); got != tt.want {
				t.Errorf("Event.IsEmailEvent() = %v, want %v for type %s", got, tt.want, tt.eventType)
			}
		})
	}
}

// TestAsCalendarMetadata tests the AsCalendarMetadata method.
func TestAsCalendarMetadata(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name    string
		event   Event
		wantErr bool
		errMsg  string
		check   func(*testing.T, *CalendarMetadata)
	}{
		{
			name: "valid calendar event with full metadata",
			event: Event{
				Type:      EventTypeCalendarMeeting,
				Source:    SourceGoogleCalendar,
				Title:     "Test meeting",
				Author:    Person{Username: "test@example.com"},
				Timestamp: now,
				Priority:  PriorityMedium,
				Metadata: map[string]any{
					"calendar_id":     "primary",
					"event_id":        "evt123",
					"start_time":      "2025-01-15T10:00:00Z",
					"end_time":        "2025-01-15T11:00:00Z",
					"is_all_day":      false,
					"organizer_email": "organizer@example.com",
					"organizer_name":  "Organizer Name",
					"attendees": []any{
						map[string]any{"email": "a@example.com", "name": "A", "status": "accepted"},
						map[string]any{"email": "b@example.com", "name": "B", "status": "tentative"},
					},
					"attendee_count":  2,
					"response_status": "accepted",
					"pending_rsvps":   []string{"c@example.com"},
					"location":        "Room 1",
					"conference_url":  "https://meet.google.com/abc",
					"description":     "Meeting description",
				},
			},
			wantErr: false,
			check: func(t *testing.T, m *CalendarMetadata) {
				if m.CalendarID != "primary" {
					t.Errorf("CalendarID = %q, want %q", m.CalendarID, "primary")
				}
				if m.EventID != "evt123" {
					t.Errorf("EventID = %q, want %q", m.EventID, "evt123")
				}
				if m.StartTime != "2025-01-15T10:00:00Z" {
					t.Errorf("StartTime = %q, want %q", m.StartTime, "2025-01-15T10:00:00Z")
				}
				if m.IsAllDay != false {
					t.Errorf("IsAllDay = %v, want false", m.IsAllDay)
				}
				if len(m.Attendees) != 2 {
					t.Fatalf("Attendees length = %d, want 2", len(m.Attendees))
				}
				if m.Attendees[0].Email != "a@example.com" {
					t.Errorf("Attendees[0].Email = %q, want %q", m.Attendees[0].Email, "a@example.com")
				}
				if m.Attendees[1].Status != "tentative" {
					t.Errorf("Attendees[1].Status = %q, want %q", m.Attendees[1].Status, "tentative")
				}
				if len(m.PendingRSVPs) != 1 || m.PendingRSVPs[0] != "c@example.com" {
					t.Errorf("PendingRSVPs = %v, want [c@example.com]", m.PendingRSVPs)
				}
			},
		},
		{
			name: "calendar event with minimal metadata",
			event: Event{
				Type:      EventTypeCalendarUpcoming,
				Source:    SourceGoogleCalendar,
				Title:     "Test",
				Author:    Person{Username: "test@example.com"},
				Timestamp: now,
				Priority:  PriorityHigh,
				Metadata: map[string]any{
					"calendar_id": "primary",
					"event_id":    "evt456",
				},
			},
			wantErr: false,
			check: func(t *testing.T, m *CalendarMetadata) {
				if m.CalendarID != "primary" {
					t.Errorf("CalendarID = %q, want %q", m.CalendarID, "primary")
				}
				if m.EventID != "evt456" {
					t.Errorf("EventID = %q, want %q", m.EventID, "evt456")
				}
				if len(m.Attendees) != 0 {
					t.Errorf("Attendees length = %d, want 0", len(m.Attendees))
				}
			},
		},
		{
			name: "wrong source returns error",
			event: Event{
				Type:      EventTypeCalendarMeeting,
				Source:    SourceGitHub,
				Title:     "Test",
				Author:    Person{Username: "test@example.com"},
				Timestamp: now,
				Priority:  PriorityMedium,
			},
			wantErr: true,
			errMsg:  "event source is github, not google_calendar",
		},
		{
			name: "wrong event type returns error",
			event: Event{
				Type:      EventTypePRReview,
				Source:    SourceGoogleCalendar,
				Title:     "Test",
				Author:    Person{Username: "test@example.com"},
				Timestamp: now,
				Priority:  PriorityMedium,
			},
			wantErr: true,
			errMsg:  "is not a calendar event",
		},
		{
			name: "nil attendees handled gracefully",
			event: Event{
				Type:      EventTypeCalendarMeeting,
				Source:    SourceGoogleCalendar,
				Title:     "Test",
				Author:    Person{Username: "test@example.com"},
				Timestamp: now,
				Priority:  PriorityMedium,
				Metadata: map[string]any{
					"calendar_id": "primary",
					"event_id":    "evt789",
					"attendees":   nil,
				},
			},
			wantErr: false,
			check: func(t *testing.T, m *CalendarMetadata) {
				if m.Attendees != nil {
					t.Errorf("Attendees = %v, want nil", m.Attendees)
				}
			},
		},
		{
			name: "wrong type attendees handled gracefully",
			event: Event{
				Type:      EventTypeCalendarMeeting,
				Source:    SourceGoogleCalendar,
				Title:     "Test",
				Author:    Person{Username: "test@example.com"},
				Timestamp: now,
				Priority:  PriorityMedium,
				Metadata: map[string]any{
					"calendar_id": "primary",
					"event_id":    "evt999",
					"attendees":   "not a slice",
				},
			},
			wantErr: false,
			check: func(t *testing.T, m *CalendarMetadata) {
				if m.Attendees != nil {
					t.Errorf("Attendees = %v, want nil", m.Attendees)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, err := tt.event.AsCalendarMetadata()
			if (err != nil) != tt.wantErr {
				t.Errorf("AsCalendarMetadata() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && err != nil && tt.errMsg != "" {
				if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("AsCalendarMetadata() error = %v, want error containing %q", err, tt.errMsg)
				}
			}
			if !tt.wantErr && tt.check != nil {
				tt.check(t, m)
			}
		})
	}
}

// TestAsEmailMetadata tests the AsEmailMetadata method.
func TestAsEmailMetadata(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name    string
		event   Event
		wantErr bool
		errMsg  string
		check   func(*testing.T, *EmailMetadata)
	}{
		{
			name: "valid email event with full metadata",
			event: Event{
				Type:      EventTypeEmailImportant,
				Source:    SourceGmail,
				Title:     "Important email",
				Author:    Person{Username: "sender@example.com"},
				Timestamp: now,
				Priority:  PriorityHigh,
				Metadata: map[string]any{
					"message_id":      "msg123",
					"thread_id":       "thread123",
					"from_email":      "sender@example.com",
					"from_name":       "Sender Name",
					"to_addresses":    []string{"to1@example.com", "to2@example.com"},
					"cc_addresses":    []string{"cc@example.com"},
					"subject":         "Test subject",
					"snippet":         "Email preview...",
					"labels":          []string{"INBOX", "IMPORTANT"},
					"is_unread":       true,
					"is_important":    true,
					"is_starred":      false,
					"has_attachments": true,
					"received_at":     "2025-01-15T09:00:00Z",
				},
			},
			wantErr: false,
			check: func(t *testing.T, m *EmailMetadata) {
				if m.MessageID != "msg123" {
					t.Errorf("MessageID = %q, want %q", m.MessageID, "msg123")
				}
				if m.ThreadID != "thread123" {
					t.Errorf("ThreadID = %q, want %q", m.ThreadID, "thread123")
				}
				if m.FromEmail != "sender@example.com" {
					t.Errorf("FromEmail = %q, want %q", m.FromEmail, "sender@example.com")
				}
				if len(m.ToAddresses) != 2 {
					t.Fatalf("ToAddresses length = %d, want 2", len(m.ToAddresses))
				}
				if m.ToAddresses[0] != "to1@example.com" {
					t.Errorf("ToAddresses[0] = %q, want %q", m.ToAddresses[0], "to1@example.com")
				}
				if len(m.CCAddresses) != 1 || m.CCAddresses[0] != "cc@example.com" {
					t.Errorf("CCAddresses = %v, want [cc@example.com]", m.CCAddresses)
				}
				if len(m.Labels) != 2 {
					t.Errorf("Labels length = %d, want 2", len(m.Labels))
				}
				if !m.IsUnread {
					t.Errorf("IsUnread = false, want true")
				}
				if !m.IsImportant {
					t.Errorf("IsImportant = false, want true")
				}
				if m.IsStarred {
					t.Errorf("IsStarred = true, want false")
				}
				if !m.HasAttachments {
					t.Errorf("HasAttachments = false, want true")
				}
			},
		},
		{
			name: "email event with minimal metadata",
			event: Event{
				Type:      EventTypeEmailDirect,
				Source:    SourceGmail,
				Title:     "Test",
				Author:    Person{Username: "sender@example.com"},
				Timestamp: now,
				Priority:  PriorityMedium,
				Metadata: map[string]any{
					"message_id": "msg456",
					"thread_id":  "thread456",
					"from_email": "sender@example.com",
				},
			},
			wantErr: false,
			check: func(t *testing.T, m *EmailMetadata) {
				if m.MessageID != "msg456" {
					t.Errorf("MessageID = %q, want %q", m.MessageID, "msg456")
				}
				if m.ThreadID != "thread456" {
					t.Errorf("ThreadID = %q, want %q", m.ThreadID, "thread456")
				}
				if len(m.ToAddresses) != 0 {
					t.Errorf("ToAddresses length = %d, want 0", len(m.ToAddresses))
				}
			},
		},
		{
			name: "wrong source returns error",
			event: Event{
				Type:      EventTypeEmailDirect,
				Source:    SourceGitHub,
				Title:     "Test",
				Author:    Person{Username: "test@example.com"},
				Timestamp: now,
				Priority:  PriorityMedium,
			},
			wantErr: true,
			errMsg:  "event source is github, not gmail",
		},
		{
			name: "wrong event type returns error",
			event: Event{
				Type:      EventTypePRReview,
				Source:    SourceGmail,
				Title:     "Test",
				Author:    Person{Username: "test@example.com"},
				Timestamp: now,
				Priority:  PriorityMedium,
			},
			wantErr: true,
			errMsg:  "is not an email event",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, err := tt.event.AsEmailMetadata()
			if (err != nil) != tt.wantErr {
				t.Errorf("AsEmailMetadata() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && err != nil && tt.errMsg != "" {
				if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("AsEmailMetadata() error = %v, want error containing %q", err, tt.errMsg)
				}
			}
			if !tt.wantErr && tt.check != nil {
				tt.check(t, m)
			}
		})
	}
}
