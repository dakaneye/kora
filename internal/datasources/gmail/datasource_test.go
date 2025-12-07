package gmail

import (
	"testing"
	"time"

	"github.com/dakaneye/kora/internal/datasources"
	"github.com/dakaneye/kora/internal/models"
)

// TestGmailDataSource_Name verifies the datasource returns the correct name.
func TestGmailDataSource_Name(t *testing.T) {
	ds := &GmailDataSource{}

	if got := ds.Name(); got != "gmail" {
		t.Errorf("Name() = %q, want %q", got, "gmail")
	}
}

// TestGmailDataSource_Service verifies the datasource returns the correct service.
func TestGmailDataSource_Service(t *testing.T) {
	ds := &GmailDataSource{}

	if got := ds.Service(); got != models.SourceGmail {
		t.Errorf("Service() = %q, want %q", got, models.SourceGmail)
	}
}

// TestNewGmailDataSource_NilAuthProvider verifies constructor rejects nil auth provider.
func TestNewGmailDataSource_NilAuthProvider(t *testing.T) {
	ds, err := NewGmailDataSource(nil)
	if err == nil {
		t.Error("expected error for nil auth provider, got nil")
	}
	if ds != nil {
		t.Error("expected nil datasource on error")
	}
	expectedMsg := "gmail: auth provider required"
	if err.Error() != expectedMsg {
		t.Errorf("error message = %q, want %q", err.Error(), expectedMsg)
	}
}

// TestNewGmailDataSource_Success verifies successful construction.
// Note: This test cannot run without a valid keychain and OAuth config.
// It serves as documentation of the expected behavior.
// Integration tests cover actual auth provider usage.

// TestWithImportantSenders verifies the important senders option.
func TestWithImportantSenders(t *testing.T) {
	senders := []string{"vip@company.com", "@importantdomain.com"}

	ds := &GmailDataSource{}
	opt := WithImportantSenders(senders)
	opt(ds)

	if len(ds.importantSenders) != len(senders) {
		t.Errorf("importantSenders length = %d, want %d", len(ds.importantSenders), len(senders))
	}

	for i, sender := range senders {
		if ds.importantSenders[i] != sender {
			t.Errorf("importantSenders[%d] = %q, want %q", i, ds.importantSenders[i], sender)
		}
	}
}

// TestWithTimeProviderOption verifies the time provider option.
func TestWithTimeProviderOption(t *testing.T) {
	fixedTime := time.Date(2025, 12, 7, 10, 0, 0, 0, time.UTC)
	mockProvider := func() time.Time {
		return fixedTime
	}

	ds := &GmailDataSource{
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

// TestDeduplicateMessageIDs verifies deduplication logic.
func TestDeduplicateMessageIDs(t *testing.T) {
	ds := &GmailDataSource{}

	tests := []struct {
		name      string
		unreadIDs []MessageID
		inboxIDs  []MessageID
		wantLen   int
		wantIDs   []string
	}{
		{
			name:      "no duplicates",
			unreadIDs: []MessageID{{ID: "msg1"}, {ID: "msg2"}},
			inboxIDs:  []MessageID{{ID: "msg3"}, {ID: "msg4"}},
			wantLen:   4,
			wantIDs:   []string{"msg1", "msg2", "msg3", "msg4"},
		},
		{
			name:      "all duplicates",
			unreadIDs: []MessageID{{ID: "msg1"}, {ID: "msg2"}},
			inboxIDs:  []MessageID{{ID: "msg1"}, {ID: "msg2"}},
			wantLen:   2,
			wantIDs:   []string{"msg1", "msg2"},
		},
		{
			name:      "partial duplicates",
			unreadIDs: []MessageID{{ID: "msg1"}, {ID: "msg2"}},
			inboxIDs:  []MessageID{{ID: "msg2"}, {ID: "msg3"}},
			wantLen:   3,
			wantIDs:   []string{"msg1", "msg2", "msg3"},
		},
		{
			name:      "empty unread",
			unreadIDs: []MessageID{},
			inboxIDs:  []MessageID{{ID: "msg1"}, {ID: "msg2"}},
			wantLen:   2,
			wantIDs:   []string{"msg1", "msg2"},
		},
		{
			name:      "empty inbox",
			unreadIDs: []MessageID{{ID: "msg1"}, {ID: "msg2"}},
			inboxIDs:  []MessageID{},
			wantLen:   2,
			wantIDs:   []string{"msg1", "msg2"},
		},
		{
			name:      "both empty",
			unreadIDs: []MessageID{},
			inboxIDs:  []MessageID{},
			wantLen:   0,
			wantIDs:   []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ds.deduplicateMessageIDs(tt.unreadIDs, tt.inboxIDs)

			if len(got) != tt.wantLen {
				t.Errorf("deduplicateMessageIDs() returned %d IDs, want %d", len(got), tt.wantLen)
			}

			// Verify order and content
			for i, wantID := range tt.wantIDs {
				if i >= len(got) {
					t.Errorf("missing ID at index %d: want %q", i, wantID)
					continue
				}
				if got[i] != wantID {
					t.Errorf("ID at index %d = %q, want %q", i, got[i], wantID)
				}
			}
		})
	}
}

// TestDeduplicateMessageIDs_PreservesUnreadFirst verifies unread order is preserved.
func TestDeduplicateMessageIDs_PreservesUnreadFirst(t *testing.T) {
	ds := &GmailDataSource{}

	// Same message in both, unread should come first
	unreadIDs := []MessageID{{ID: "msg-dup"}, {ID: "msg-unread"}}
	inboxIDs := []MessageID{{ID: "msg-inbox"}, {ID: "msg-dup"}}

	got := ds.deduplicateMessageIDs(unreadIDs, inboxIDs)

	// Should be: msg-dup (from unread), msg-unread, msg-inbox
	expected := []string{"msg-dup", "msg-unread", "msg-inbox"}

	if len(got) != len(expected) {
		t.Fatalf("expected %d IDs, got %d", len(expected), len(got))
	}

	for i, wantID := range expected {
		if got[i] != wantID {
			t.Errorf("ID at index %d = %q, want %q", i, got[i], wantID)
		}
	}
}

// TestApplyFilters verifies FetchOptions filters are applied correctly.
func TestApplyFilters(t *testing.T) {
	ds := &GmailDataSource{
		email: "test@example.com",
	}

	// Create test events
	event1 := models.Event{
		Type:           models.EventTypeEmailDirect,
		Title:          "Direct Email",
		Priority:       models.PriorityMedium,
		RequiresAction: true,
		Source:         models.SourceGmail,
		URL:            "https://mail.google.com/mail/u/0/#inbox/1",
		Timestamp:      time.Now(),
		Author:         models.Person{Name: "Sender", Username: "sender@example.com"},
		Metadata:       map[string]any{},
	}

	event2 := models.Event{
		Type:           models.EventTypeEmailImportant,
		Title:          "Important Email",
		Priority:       models.PriorityHigh,
		RequiresAction: true,
		Source:         models.SourceGmail,
		URL:            "https://mail.google.com/mail/u/0/#inbox/2",
		Timestamp:      time.Now(),
		Author:         models.Person{Name: "VIP", Username: "vip@example.com"},
		Metadata:       map[string]any{},
	}

	event3 := models.Event{
		Type:           models.EventTypeEmailCC,
		Title:          "CC'd Email",
		Priority:       models.PriorityLow,
		RequiresAction: false,
		Source:         models.SourceGmail,
		URL:            "https://mail.google.com/mail/u/0/#inbox/3",
		Timestamp:      time.Now(),
		Author:         models.Person{Name: "Sender", Username: "sender@example.com"},
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
					EventTypes: []models.EventType{models.EventTypeEmailImportant},
				},
			},
			wantCount: 1,
			desc:      "should return only important events",
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
			wantCount: 2,
			desc:      "should return only events requiring action",
		},
		{
			name:   "multiple event types",
			events: allEvents,
			opts: datasources.FetchOptions{
				Filter: &datasources.FetchFilter{
					EventTypes: []models.EventType{
						models.EventTypeEmailDirect,
						models.EventTypeEmailCC,
					},
				},
			},
			wantCount: 2,
			desc:      "should return events matching either type",
		},
		{
			name:   "combined filters - type and priority",
			events: allEvents,
			opts: datasources.FetchOptions{
				Filter: &datasources.FetchFilter{
					EventTypes:  []models.EventType{models.EventTypeEmailImportant},
					MinPriority: models.PriorityHigh,
				},
			},
			wantCount: 1,
			desc:      "should match both filters",
		},
		{
			name:   "filter excludes all",
			events: allEvents,
			opts: datasources.FetchOptions{
				Filter: &datasources.FetchFilter{
					EventTypes: []models.EventType{models.EventTypeCalendarMeeting},
				},
			},
			wantCount: 0,
			desc:      "non-matching filter excludes all",
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

// TestFilterMessages_Integration verifies filtering of Gmail messages.
func TestFilterMessages_Integration(t *testing.T) {
	tests := []struct {
		name     string
		messages []*Message
		wantLen  int
		desc     string
	}{
		{
			name: "filter mailing lists",
			messages: []*Message{
				createTestMessage(map[string]string{"From": "person@example.com"}, nil),
				createTestMessage(map[string]string{
					"From":             "newsletter@company.com",
					"List-Unsubscribe": "<mailto:unsub@company.com>",
				}, nil),
			},
			wantLen: 1,
			desc:    "mailing list should be filtered out",
		},
		{
			name: "filter automated senders",
			messages: []*Message{
				createTestMessage(map[string]string{"From": "person@example.com"}, nil),
				createTestMessage(map[string]string{"From": "noreply@github.com"}, nil),
				createTestMessage(map[string]string{"From": "notifications@slack.com"}, nil),
			},
			wantLen: 1,
			desc:    "automated senders should be filtered out",
		},
		{
			name: "filter both mailing lists and automated",
			messages: []*Message{
				createTestMessage(map[string]string{"From": "person@example.com"}, nil),
				createTestMessage(map[string]string{
					"From":    "updates@company.com",
					"List-Id": "<updates.company.com>",
				}, nil),
				createTestMessage(map[string]string{"From": "bot@service.com"}, nil),
			},
			wantLen: 1,
			desc:    "both types should be filtered out",
		},
		{
			name: "keep all real people",
			messages: []*Message{
				createTestMessage(map[string]string{"From": "person1@example.com"}, nil),
				createTestMessage(map[string]string{"From": "person2@example.com"}, nil),
				createTestMessage(map[string]string{"From": "person3@example.com"}, nil),
			},
			wantLen: 3,
			desc:    "all messages from real people should be kept",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filtered := FilterMessages(tt.messages)

			if len(filtered) != tt.wantLen {
				t.Errorf("FilterMessages() returned %d messages, want %d: %s",
					len(filtered), tt.wantLen, tt.desc)
			}

			// Verify no filtered types in result
			for _, msg := range filtered {
				if IsMailingList(msg) {
					t.Errorf("found mailing list in filtered results: %s", msg.From())
				}
				if IsAutomated(msg) {
					t.Errorf("found automated sender in filtered results: %s", msg.From())
				}
			}
		})
	}
}

// TestDefaultTimeProvider verifies the default time provider works.
func TestDefaultTimeProvider(t *testing.T) {
	before := time.Now()
	got := defaultTimeProvider()
	after := time.Now()

	if got.Before(before) || got.After(after) {
		t.Errorf("defaultTimeProvider() = %v, expected between %v and %v", got, before, after)
	}
}

// TestNewGmailDataSource_WithOptions verifies options are applied correctly.
// Note: This test cannot run without a valid keychain and OAuth config.
// It serves as documentation of the expected behavior.
// The option functions themselves are tested individually above.

// TestApplyFilters_PriorityLogic verifies priority filtering logic.
func TestApplyFilters_PriorityLogic(t *testing.T) {
	ds := &GmailDataSource{}

	// Create events with different priorities
	highPriorityEvent := models.Event{
		Type:     models.EventTypeEmailImportant,
		Title:    "High Priority",
		Priority: models.PriorityHigh, // Priority 2
		Source:   models.SourceGmail,
		URL:      "https://mail.google.com/mail/u/0/#inbox/1",
		Author:   models.Person{Username: "test"},
		Metadata: map[string]any{},
	}

	mediumPriorityEvent := models.Event{
		Type:     models.EventTypeEmailDirect,
		Title:    "Medium Priority",
		Priority: models.PriorityMedium, // Priority 3
		Source:   models.SourceGmail,
		URL:      "https://mail.google.com/mail/u/0/#inbox/2",
		Author:   models.Person{Username: "test"},
		Metadata: map[string]any{},
	}

	lowPriorityEvent := models.Event{
		Type:     models.EventTypeEmailCC,
		Title:    "Low Priority",
		Priority: models.PriorityLow, // Priority 4
		Source:   models.SourceGmail,
		URL:      "https://mail.google.com/mail/u/0/#inbox/3",
		Author:   models.Person{Username: "test"},
		Metadata: map[string]any{},
	}

	allEvents := []models.Event{highPriorityEvent, mediumPriorityEvent, lowPriorityEvent}

	// Filter for MinPriority=High should return only high priority
	opts := datasources.FetchOptions{
		Filter: &datasources.FetchFilter{
			MinPriority: models.PriorityHigh,
		},
	}

	filtered := ds.applyFilters(allEvents, opts)
	if len(filtered) != 1 {
		t.Errorf("expected 1 high priority event, got %d", len(filtered))
	}
	if len(filtered) > 0 && filtered[0].Priority != models.PriorityHigh {
		t.Errorf("expected high priority, got %d", filtered[0].Priority)
	}

	// Filter for MinPriority=Medium should return high and medium
	opts.Filter.MinPriority = models.PriorityMedium
	filtered = ds.applyFilters(allEvents, opts)
	if len(filtered) != 2 {
		t.Errorf("expected 2 events (high and medium), got %d", len(filtered))
	}
}

// TestApplyFilters_EmptyFilter verifies nil filter returns all events.
func TestApplyFilters_EmptyFilter(t *testing.T) {
	ds := &GmailDataSource{}

	events := []models.Event{
		{Type: models.EventTypeEmailDirect, Source: models.SourceGmail, URL: "test", Author: models.Person{Username: "test"}, Metadata: map[string]any{}},
		{Type: models.EventTypeEmailImportant, Source: models.SourceGmail, URL: "test", Author: models.Person{Username: "test"}, Metadata: map[string]any{}},
		{Type: models.EventTypeEmailCC, Source: models.SourceGmail, URL: "test", Author: models.Person{Username: "test"}, Metadata: map[string]any{}},
	}

	// Nil filter should return all
	opts := datasources.FetchOptions{}
	filtered := ds.applyFilters(events, opts)
	if len(filtered) != len(events) {
		t.Errorf("nil filter should return all %d events, got %d", len(events), len(filtered))
	}
}

// TestGmailDataSource_Implements_DataSource verifies interface compliance.
func TestGmailDataSource_Implements_DataSource(t *testing.T) {
	// This test verifies at compile time that GmailDataSource implements DataSource.
	// If it doesn't, this will fail to compile.
	var _ datasources.DataSource = (*GmailDataSource)(nil)
}

// TestFetch_ContextCancellation would test context cancellation.
// This requires mocking the client, which is beyond the scope of basic unit tests.
// Integration tests with a mock HTTP server would cover this scenario.

// TestFetch_PartialSuccess would test partial success with one query failing.
// This also requires mocking the client for controlled failure scenarios.
// These scenarios are better covered in integration tests.
