package gmail

import (
	"testing"
	"time"
)

// TestMessage_GetHeader verifies header retrieval is case-insensitive.
func TestMessage_GetHeader(t *testing.T) {
	msg := &Message{
		Payload: MessagePayload{
			Headers: []Header{
				{Name: "From", Value: "sender@example.com"},
				{Name: "Subject", Value: "Test Subject"},
				{Name: "X-Custom-Header", Value: "custom-value"},
			},
		},
	}

	tests := []struct {
		name       string
		headerName string
		want       string
	}{
		{
			name:       "exact case match",
			headerName: "From",
			want:       "sender@example.com",
		},
		{
			name:       "different case",
			headerName: "from",
			want:       "sender@example.com",
		},
		{
			name:       "uppercase",
			headerName: "SUBJECT",
			want:       "Test Subject",
		},
		{
			name:       "mixed case custom header",
			headerName: "x-CuStOm-HeAdEr",
			want:       "custom-value",
		},
		{
			name:       "not found",
			headerName: "NonExistent",
			want:       "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := msg.GetHeader(tt.headerName)
			if got != tt.want {
				t.Errorf("GetHeader(%q) = %q, want %q", tt.headerName, got, tt.want)
			}
		})
	}
}

// TestMessage_FromEmail verifies email extraction from From header.
func TestMessage_FromEmail(t *testing.T) {
	tests := []struct {
		name string
		from string
		want string
	}{
		{
			name: "email with display name",
			from: "John Doe <john@example.com>",
			want: "john@example.com",
		},
		{
			name: "email only",
			from: "john@example.com",
			want: "john@example.com",
		},
		{
			name: "complex format",
			from: "\"Doe, John\" <john.doe@company.com>",
			want: "john.doe@company.com",
		},
		{
			name: "empty from",
			from: "",
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := &Message{
				Payload: MessagePayload{
					Headers: []Header{
						{Name: "From", Value: tt.from},
					},
				},
			}

			got := msg.FromEmail()
			if got != tt.want {
				t.Errorf("FromEmail() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestMessage_FromName verifies display name extraction.
func TestMessage_FromName(t *testing.T) {
	tests := []struct {
		name string
		from string
		want string
	}{
		{
			name: "name and email",
			from: "John Doe <john@example.com>",
			want: "John Doe",
		},
		{
			name: "quoted name",
			from: "\"Doe, John\" <john@example.com>",
			want: "Doe, John",
		},
		{
			name: "email only - no name",
			from: "john@example.com",
			want: "",
		},
		{
			name: "empty",
			from: "",
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := &Message{
				Payload: MessagePayload{
					Headers: []Header{
						{Name: "From", Value: tt.from},
					},
				},
			}

			got := msg.FromName()
			if got != tt.want {
				t.Errorf("FromName() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestMessage_Date verifies date parsing.
func TestMessage_Date(t *testing.T) {
	tests := []struct {
		name         string
		dateHeader   string
		internalDate string
		wantZero     bool
	}{
		{
			name:       "valid date header",
			dateHeader: "Tue, 14 Nov 2023 12:00:00 +0000",
			wantZero:   false,
		},
		{
			name:         "fallback to internal date",
			internalDate: "1700000000000", // Unix ms
			wantZero:     false,
		},
		{
			name:     "no date headers",
			wantZero: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := &Message{
				InternalDate: tt.internalDate,
				Payload: MessagePayload{
					Headers: []Header{},
				},
			}
			if tt.dateHeader != "" {
				msg.Payload.Headers = append(msg.Payload.Headers,
					Header{Name: "Date", Value: tt.dateHeader})
			}

			got := msg.Date()
			if tt.wantZero && !got.IsZero() {
				t.Error("expected zero time")
			}
			if !tt.wantZero && got.IsZero() {
				t.Error("expected non-zero time")
			}
		})
	}
}

// TestMessage_LabelChecks verifies label checking methods.
func TestMessage_LabelChecks(t *testing.T) {
	msg := &Message{
		LabelIDs: []string{"INBOX", "UNREAD", "IMPORTANT", "STARRED"},
	}

	if !msg.IsUnread() {
		t.Error("IsUnread() should be true")
	}
	if !msg.IsImportant() {
		t.Error("IsImportant() should be true")
	}
	if !msg.IsStarred() {
		t.Error("IsStarred() should be true")
	}
	if !msg.IsInInbox() {
		t.Error("IsInInbox() should be true")
	}

	// Test message without labels
	msgNoLabels := &Message{
		LabelIDs: []string{},
	}

	if msgNoLabels.IsUnread() {
		t.Error("IsUnread() should be false for message without labels")
	}
	if msgNoLabels.IsImportant() {
		t.Error("IsImportant() should be false for message without labels")
	}
}

// TestMessage_HasAttachments verifies attachment detection.
func TestMessage_HasAttachments(t *testing.T) {
	tests := []struct {
		name string
		msg  *Message
		want bool
	}{
		{
			name: "no parts",
			msg: &Message{
				Payload: MessagePayload{
					Parts: []MessagePart{},
				},
			},
			want: false,
		},
		{
			name: "parts with no filenames",
			msg: &Message{
				Payload: MessagePayload{
					Parts: []MessagePart{
						{MimeType: "text/plain"},
						{MimeType: "text/html"},
					},
				},
			},
			want: false,
		},
		{
			name: "part with filename",
			msg: &Message{
				Payload: MessagePayload{
					Parts: []MessagePart{
						{MimeType: "text/plain"},
						{MimeType: "application/pdf", Filename: "document.pdf"},
					},
				},
			},
			want: true,
		},
		{
			name: "nested part with filename",
			msg: &Message{
				Payload: MessagePayload{
					Parts: []MessagePart{
						{
							MimeType: "multipart/mixed",
							Parts: []MessagePart{
								{MimeType: "application/pdf", Filename: "doc.pdf"},
							},
						},
					},
				},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.msg.HasAttachments()
			if got != tt.want {
				t.Errorf("HasAttachments() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestMessage_To verifies To header parsing.
func TestMessage_To(t *testing.T) {
	tests := []struct {
		name    string
		toValue string
		want    []string
	}{
		{
			name:    "single recipient",
			toValue: "user@example.com",
			want:    []string{"user@example.com"},
		},
		{
			name:    "multiple recipients",
			toValue: "user1@example.com, user2@example.com",
			want:    []string{"user1@example.com", "user2@example.com"},
		},
		{
			name:    "recipients with names",
			toValue: "John Doe <john@example.com>, Jane Smith <jane@example.com>",
			want:    []string{"john@example.com", "jane@example.com"},
		},
		{
			name:    "empty",
			toValue: "",
			want:    nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := &Message{
				Payload: MessagePayload{
					Headers: []Header{
						{Name: "To", Value: tt.toValue},
					},
				},
			}

			got := msg.To()
			if len(got) != len(tt.want) {
				t.Errorf("To() returned %d addresses, want %d", len(got), len(tt.want))
			}
			for i, addr := range tt.want {
				if i >= len(got) {
					continue
				}
				if got[i] != addr {
					t.Errorf("To()[%d] = %q, want %q", i, got[i], addr)
				}
			}
		})
	}
}

// TestMessage_CC verifies CC header parsing.
func TestMessage_CC(t *testing.T) {
	msg := &Message{
		Payload: MessagePayload{
			Headers: []Header{
				{Name: "Cc", Value: "cc1@example.com, cc2@example.com"},
			},
		},
	}

	got := msg.CC()
	if len(got) != 2 {
		t.Errorf("CC() returned %d addresses, want 2", len(got))
	}
	if len(got) > 0 && got[0] != "cc1@example.com" {
		t.Errorf("CC()[0] = %q, want cc1@example.com", got[0])
	}
}

// TestMessage_Subject verifies subject retrieval.
func TestMessage_Subject(t *testing.T) {
	msg := &Message{
		Payload: MessagePayload{
			Headers: []Header{
				{Name: "Subject", Value: "Test Subject Line"},
			},
		},
	}

	got := msg.Subject()
	want := "Test Subject Line"
	if got != want {
		t.Errorf("Subject() = %q, want %q", got, want)
	}
}

// TestDecodeBase64URL verifies base64 URL decoding.
func TestDecodeBase64URL(t *testing.T) {
	tests := []struct {
		name    string
		encoded string
		want    string
		wantErr bool
	}{
		{
			name:    "valid base64 URL encoded",
			encoded: "SGVsbG8gV29ybGQ=",
			want:    "Hello World",
			wantErr: false,
		},
		{
			name:    "empty string",
			encoded: "",
			want:    "",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := decodeBase64URL(tt.encoded)
			if (err != nil) != tt.wantErr {
				t.Errorf("decodeBase64URL() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("decodeBase64URL() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestConstants verifies expected constant values.
func TestConstants(t *testing.T) {
	if gmailAPIBase != "https://www.googleapis.com/gmail/v1" {
		t.Errorf("gmailAPIBase = %q, want https://www.googleapis.com/gmail/v1", gmailAPIBase)
	}
	if defaultMaxResults != 100 {
		t.Errorf("defaultMaxResults = %d, want 100", defaultMaxResults)
	}
	if maxBatchSize != 10 {
		t.Errorf("maxBatchSize = %d, want 10", maxBatchSize)
	}
	if maxRetries != 3 {
		t.Errorf("maxRetries = %d, want 3", maxRetries)
	}
	if initialBackoff != 1*time.Second {
		t.Errorf("initialBackoff = %v, want 1s", initialBackoff)
	}
}

// Note: Full API client testing (ListMessages, GetMessage, BatchGetMessages, doRequestWithRetry)
// requires mocking HTTP responses and is better suited for integration tests.
// These tests cover the message parsing and helper methods that can be tested in isolation.
