package gmail

import (
	"strings"
	"testing"
	"time"

	"github.com/dakaneye/kora/internal/models"
)

// testMessage creates a basic test message with reasonable defaults.
func testMessage(id string) *Message {
	return &Message{
		ID:           id,
		ThreadID:     "thread-" + id,
		LabelIDs:     []string{"INBOX", "UNREAD"},
		Snippet:      "This is a test email snippet...",
		InternalDate: "1700000000000", // Unix ms timestamp
		Payload: MessagePayload{
			Headers: []Header{
				{Name: "From", Value: "sender@example.com"},
				{Name: "To", Value: "user@example.com"},
				{Name: "Subject", Value: "Test Subject"},
				{Name: "Date", Value: "Tue, 14 Nov 2023 12:00:00 +0000"},
			},
			MimeType: "text/plain",
		},
	}
}

func TestToEvent_BasicConversion(t *testing.T) {
	msg := testMessage("msg-001")
	userEmail := "user@example.com"

	event, err := ToEvent(msg, userEmail, nil)
	if err != nil {
		t.Fatalf("ToEvent failed: %v", err)
	}

	// Verify basic fields
	if event.Type != models.EventTypeEmailDirect {
		t.Errorf("expected type %s, got %s", models.EventTypeEmailDirect, event.Type)
	}
	if event.Title != "Test Subject" {
		t.Errorf("expected title 'Test Subject', got %q", event.Title)
	}
	if event.Source != models.SourceGmail {
		t.Errorf("expected source %s, got %s", models.SourceGmail, event.Source)
	}
	if event.Priority != models.PriorityMedium {
		t.Errorf("expected priority %d, got %d", models.PriorityMedium, event.Priority)
	}
	if !event.RequiresAction {
		t.Error("expected RequiresAction=true for unread direct email")
	}

	// Verify author
	if event.Author.Username != "sender@example.com" {
		t.Errorf("expected author username 'sender@example.com', got %q", event.Author.Username)
	}

	// Verify URL format
	expectedURL := "https://mail.google.com/mail/u/0/#inbox/msg-001"
	if event.URL != expectedURL {
		t.Errorf("expected URL %q, got %q", expectedURL, event.URL)
	}

	// Verify metadata
	if event.Metadata["message_id"] != "msg-001" {
		t.Errorf("expected metadata message_id='msg-001', got %v", event.Metadata["message_id"])
	}
	if event.Metadata["thread_id"] != "thread-msg-001" {
		t.Errorf("expected metadata thread_id='thread-msg-001', got %v", event.Metadata["thread_id"])
	}
}

func TestToEvent_EventTypes(t *testing.T) {
	//nolint:govet // Field order for readability in test cases
	tests := []struct {
		name             string
		setupMsg         func(*Message)
		userEmail        string
		importantSenders []string
		expectedType     models.EventType
		expectedPriority models.Priority
		expectedAction   bool
		titlePrefix      string
	}{
		{
			name: "important_via_label",
			setupMsg: func(m *Message) {
				m.LabelIDs = []string{"INBOX", "IMPORTANT", "UNREAD"}
			},
			userEmail:        "user@example.com",
			importantSenders: nil,
			expectedType:     models.EventTypeEmailImportant,
			expectedPriority: models.PriorityHigh,
			expectedAction:   true,
			titlePrefix:      "Important: ",
		},
		{
			name: "important_via_sender_exact",
			setupMsg: func(m *Message) {
				m.Payload.Headers = []Header{
					{Name: "From", Value: "vip@company.com"},
					{Name: "To", Value: "user@example.com"},
					{Name: "Subject", Value: "VIP Message"},
				}
			},
			userEmail:        "user@example.com",
			importantSenders: []string{"vip@company.com"},
			expectedType:     models.EventTypeEmailImportant,
			expectedPriority: models.PriorityHigh,
			expectedAction:   true,
			titlePrefix:      "Important: ",
		},
		{
			name: "important_via_sender_domain",
			setupMsg: func(m *Message) {
				m.Payload.Headers = []Header{
					{Name: "From", Value: "anyone@important.org"},
					{Name: "To", Value: "user@example.com"},
					{Name: "Subject", Value: "Domain Match"},
				}
			},
			userEmail:        "user@example.com",
			importantSenders: []string{"@important.org"},
			expectedType:     models.EventTypeEmailImportant,
			expectedPriority: models.PriorityHigh,
			expectedAction:   true,
			titlePrefix:      "Important: ",
		},
		{
			name: "direct_unread",
			setupMsg: func(m *Message) {
				m.LabelIDs = []string{"INBOX", "UNREAD"}
				m.Payload.Headers = []Header{
					{Name: "From", Value: "sender@example.com"},
					{Name: "To", Value: "user@example.com"},
					{Name: "Subject", Value: "Direct Email"},
				}
			},
			userEmail:        "user@example.com",
			importantSenders: nil,
			expectedType:     models.EventTypeEmailDirect,
			expectedPriority: models.PriorityMedium,
			expectedAction:   true,
			titlePrefix:      "",
		},
		{
			name: "direct_read",
			setupMsg: func(m *Message) {
				m.LabelIDs = []string{"INBOX"} // No UNREAD
				m.Payload.Headers = []Header{
					{Name: "From", Value: "sender@example.com"},
					{Name: "To", Value: "user@example.com"},
					{Name: "Subject", Value: "Read Email"},
				}
			},
			userEmail:        "user@example.com",
			importantSenders: nil,
			expectedType:     models.EventTypeEmailDirect,
			expectedPriority: models.PriorityMedium,
			expectedAction:   false, // Read = no action required
			titlePrefix:      "",
		},
		{
			name: "cc_recipient",
			setupMsg: func(m *Message) {
				m.LabelIDs = []string{"INBOX", "UNREAD"}
				m.Payload.Headers = []Header{
					{Name: "From", Value: "sender@example.com"},
					{Name: "To", Value: "someone@example.com"},
					{Name: "Cc", Value: "user@example.com"},
					{Name: "Subject", Value: "CC Email"},
				}
			},
			userEmail:        "user@example.com",
			importantSenders: nil,
			expectedType:     models.EventTypeEmailCC,
			expectedPriority: models.PriorityLow,
			expectedAction:   false,
			titlePrefix:      "CC'd: ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := testMessage("test-" + tt.name)
			tt.setupMsg(msg)

			event, err := ToEvent(msg, tt.userEmail, tt.importantSenders)
			if err != nil {
				t.Fatalf("ToEvent failed: %v", err)
			}

			if event.Type != tt.expectedType {
				t.Errorf("expected type %s, got %s", tt.expectedType, event.Type)
			}
			if event.Priority != tt.expectedPriority {
				t.Errorf("expected priority %d, got %d", tt.expectedPriority, event.Priority)
			}
			if event.RequiresAction != tt.expectedAction {
				t.Errorf("expected RequiresAction=%v, got %v", tt.expectedAction, event.RequiresAction)
			}
			if tt.titlePrefix != "" && !strings.HasPrefix(event.Title, tt.titlePrefix) {
				t.Errorf("expected title to start with %q, got %q", tt.titlePrefix, event.Title)
			}
		})
	}
}

func TestToEvent_TitleTruncation(t *testing.T) {
	msg := testMessage("msg-long")
	// Create a very long subject
	longSubject := strings.Repeat("x", 150)
	msg.Payload.Headers = []Header{
		{Name: "From", Value: "sender@example.com"},
		{Name: "To", Value: "user@example.com"},
		{Name: "Subject", Value: longSubject},
	}

	event, err := ToEvent(msg, "user@example.com", nil)
	if err != nil {
		t.Fatalf("ToEvent failed: %v", err)
	}

	if len(event.Title) > 100 {
		t.Errorf("title should be truncated to 100 chars, got %d", len(event.Title))
	}
	if !strings.HasSuffix(event.Title, "...") {
		t.Error("truncated title should end with '...'")
	}
}

func TestToEvent_NoSubject(t *testing.T) {
	msg := testMessage("msg-no-subject")
	msg.Payload.Headers = []Header{
		{Name: "From", Value: "sender@example.com"},
		{Name: "To", Value: "user@example.com"},
		// No Subject header
	}

	event, err := ToEvent(msg, "user@example.com", nil)
	if err != nil {
		t.Fatalf("ToEvent failed: %v", err)
	}

	if event.Title != "(No subject)" {
		t.Errorf("expected title '(No subject)', got %q", event.Title)
	}
}

func TestToEvent_NilMessage(t *testing.T) {
	_, err := ToEvent(nil, "user@example.com", nil)
	if err == nil {
		t.Error("expected error for nil message")
	}
}

func TestToEvent_MetadataKeys(t *testing.T) {
	msg := testMessage("msg-meta")
	msg.LabelIDs = []string{"INBOX", "UNREAD", "STARRED", "IMPORTANT"}
	msg.Payload.Headers = []Header{
		{Name: "From", Value: "John Doe <john@example.com>"},
		{Name: "To", Value: "user@example.com, other@example.com"},
		{Name: "Cc", Value: "cc1@example.com"},
		{Name: "Subject", Value: "Metadata Test"},
	}
	msg.Payload.Parts = []MessagePart{
		{Filename: "attachment.pdf"}, // Has attachment
	}

	event, err := ToEvent(msg, "user@example.com", nil)
	if err != nil {
		t.Fatalf("ToEvent failed: %v", err)
	}

	// Check all required metadata keys exist
	requiredKeys := []string{
		"message_id", "thread_id", "from_email", "from_name",
		"to_addresses", "cc_addresses", "subject", "snippet",
		"labels", "is_unread", "is_important", "is_starred",
		"has_attachments", "received_at",
	}

	for _, key := range requiredKeys {
		if _, exists := event.Metadata[key]; !exists {
			t.Errorf("missing required metadata key: %s", key)
		}
	}

	// Verify specific metadata values
	if event.Metadata["from_email"] != "john@example.com" {
		t.Errorf("expected from_email='john@example.com', got %v", event.Metadata["from_email"])
	}
	if event.Metadata["from_name"] != "John Doe" {
		t.Errorf("expected from_name='John Doe', got %v", event.Metadata["from_name"])
	}
	if event.Metadata["is_unread"] != true {
		t.Errorf("expected is_unread=true, got %v", event.Metadata["is_unread"])
	}
	if event.Metadata["is_important"] != true {
		t.Errorf("expected is_important=true, got %v", event.Metadata["is_important"])
	}
	if event.Metadata["is_starred"] != true {
		t.Errorf("expected is_starred=true, got %v", event.Metadata["is_starred"])
	}
	if event.Metadata["has_attachments"] != true {
		t.Errorf("expected has_attachments=true, got %v", event.Metadata["has_attachments"])
	}
}

func TestToEvents_BatchConversion(t *testing.T) {
	messages := []*Message{
		testMessage("msg-1"),
		testMessage("msg-2"),
		testMessage("msg-3"),
	}
	// Set up proper headers for all messages
	for _, m := range messages {
		m.Payload.Headers = []Header{
			{Name: "From", Value: "sender@example.com"},
			{Name: "To", Value: "user@example.com"},
			{Name: "Subject", Value: "Test " + m.ID},
		}
	}

	events, errs := ToEvents(messages, "user@example.com", nil)
	if len(errs) > 0 {
		t.Errorf("unexpected errors: %v", errs)
	}
	if len(events) != 3 {
		t.Errorf("expected 3 events, got %d", len(events))
	}
}

func TestToEvents_PartialSuccess(t *testing.T) {
	messages := []*Message{
		testMessage("msg-valid"),
		nil, // Invalid - will be skipped
		testMessage("msg-valid-2"),
	}
	// Set up proper headers
	messages[0].Payload.Headers = []Header{
		{Name: "From", Value: "sender@example.com"},
		{Name: "To", Value: "user@example.com"},
		{Name: "Subject", Value: "Valid 1"},
	}
	messages[2].Payload.Headers = []Header{
		{Name: "From", Value: "sender@example.com"},
		{Name: "To", Value: "user@example.com"},
		{Name: "Subject", Value: "Valid 2"},
	}

	events, errs := ToEvents(messages, "user@example.com", nil)
	if len(errs) != 0 {
		t.Errorf("expected 0 errors (nil messages skipped), got %d: %v", len(errs), errs)
	}
	if len(events) != 2 {
		t.Errorf("expected 2 events, got %d", len(events))
	}
}

func TestToEvents_EmptySlice(t *testing.T) {
	events, errs := ToEvents(nil, "user@example.com", nil)
	if events != nil {
		t.Errorf("expected nil events, got %v", events)
	}
	if errs != nil {
		t.Errorf("expected nil errs, got %v", errs)
	}
}

func TestDetermineEventType_PriorityOrder(t *testing.T) {
	// Test that important takes precedence over direct
	msg := testMessage("msg-priority")
	msg.LabelIDs = []string{"INBOX", "IMPORTANT", "UNREAD"}
	msg.Payload.Headers = []Header{
		{Name: "From", Value: "vip@company.com"},
		{Name: "To", Value: "user@example.com"}, // Direct recipient
		{Name: "Subject", Value: "Priority Test"},
	}

	eventType, _ := determineEventType(msg, "user@example.com", []string{"vip@company.com"})
	if eventType != models.EventTypeEmailImportant {
		t.Errorf("important should take precedence over direct, got %s", eventType)
	}

	// Test that direct takes precedence over CC when user is in both
	msg2 := testMessage("msg-priority-2")
	msg2.LabelIDs = []string{"INBOX", "UNREAD"}
	msg2.Payload.Headers = []Header{
		{Name: "From", Value: "sender@example.com"},
		{Name: "To", Value: "user@example.com"},
		{Name: "Cc", Value: "user@example.com"}, // Also CC'd
		{Name: "Subject", Value: "Both To and CC"},
	}

	eventType2, _ := determineEventType(msg2, "user@example.com", nil)
	if eventType2 != models.EventTypeEmailDirect {
		t.Errorf("direct should take precedence over CC, got %s", eventType2)
	}
}

func TestBuildMetadata_ReceivedAt(t *testing.T) {
	msg := testMessage("msg-time")
	msg.InternalDate = "1700000000000" // 2023-11-14 22:13:20 UTC

	metadata := buildMetadata(msg)

	receivedAt, ok := metadata["received_at"].(string)
	if !ok {
		t.Fatal("received_at should be a string")
	}

	// Verify it's RFC3339 format
	_, err := time.Parse(time.RFC3339, receivedAt)
	if err != nil {
		t.Errorf("received_at should be RFC3339 format: %v", err)
	}
}

func TestBuildAuthor_WithDisplayName(t *testing.T) {
	msg := testMessage("msg-author")
	msg.Payload.Headers = []Header{
		{Name: "From", Value: "John Doe <john@example.com>"},
	}

	author := buildAuthor(msg)
	if author.Name != "John Doe" {
		t.Errorf("expected Name='John Doe', got %q", author.Name)
	}
	if author.Username != "john@example.com" {
		t.Errorf("expected Username='john@example.com', got %q", author.Username)
	}
}

func TestBuildAuthor_EmailOnly(t *testing.T) {
	msg := testMessage("msg-author-email")
	msg.Payload.Headers = []Header{
		{Name: "From", Value: "john@example.com"},
	}

	author := buildAuthor(msg)
	// Name should fall back to email
	if author.Name != "john@example.com" {
		t.Errorf("expected Name='john@example.com', got %q", author.Name)
	}
	if author.Username != "john@example.com" {
		t.Errorf("expected Username='john@example.com', got %q", author.Username)
	}
}

func TestBuildAuthor_MissingFrom(t *testing.T) {
	msg := testMessage("msg-no-from")
	msg.Payload.Headers = []Header{
		{Name: "To", Value: "user@example.com"},
		// No From header
	}

	author := buildAuthor(msg)
	if author.Username != "unknown" {
		t.Errorf("expected Username='unknown', got %q", author.Username)
	}
}

func TestToEvent_ValidationPasses(t *testing.T) {
	// Test that a well-formed message produces a valid event
	msg := testMessage("msg-valid")
	msg.Payload.Headers = []Header{
		{Name: "From", Value: "sender@example.com"},
		{Name: "To", Value: "user@example.com"},
		{Name: "Subject", Value: "Valid Email"},
		{Name: "Date", Value: "Tue, 14 Nov 2023 12:00:00 +0000"},
	}

	event, err := ToEvent(msg, "user@example.com", nil)
	if err != nil {
		t.Fatalf("ToEvent should not fail for valid message: %v", err)
	}

	// Double-check validation passes
	if err := event.Validate(); err != nil {
		t.Errorf("event.Validate() should pass: %v", err)
	}
}
