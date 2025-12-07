package gmail

import (
	"testing"
)

// createTestMessage creates a test Message with the given headers.
func createTestMessage(headers map[string]string, labels []string) *Message {
	hdrs := make([]Header, 0, len(headers))
	for name, value := range headers {
		hdrs = append(hdrs, Header{Name: name, Value: value})
	}
	return &Message{
		ID:       "test-id",
		ThreadID: "test-thread",
		LabelIDs: labels,
		Payload: MessagePayload{
			Headers: hdrs,
		},
	}
}

func TestIsMailingList(t *testing.T) {
	tests := []struct {
		msg  *Message
		name string
		want bool
	}{
		{
			name: "nil message",
			msg:  nil,
			want: false,
		},
		{
			name: "regular email",
			msg: createTestMessage(map[string]string{
				"From":    "person@example.com",
				"Subject": "Hello",
			}, nil),
			want: false,
		},
		{
			name: "has List-Unsubscribe header",
			msg: createTestMessage(map[string]string{
				"From":             "newsletter@company.com",
				"List-Unsubscribe": "<mailto:unsubscribe@company.com>",
			}, nil),
			want: true,
		},
		{
			name: "has List-Id header",
			msg: createTestMessage(map[string]string{
				"From":    "team@company.com",
				"List-Id": "<team.company.com>",
			}, nil),
			want: true,
		},
		{
			name: "has Precedence list",
			msg: createTestMessage(map[string]string{
				"From":       "announce@company.com",
				"Precedence": "list",
			}, nil),
			want: true,
		},
		{
			name: "has Precedence bulk",
			msg: createTestMessage(map[string]string{
				"From":       "marketing@company.com",
				"Precedence": "bulk",
			}, nil),
			want: true,
		},
		{
			name: "has Precedence bulk uppercase",
			msg: createTestMessage(map[string]string{
				"From":       "marketing@company.com",
				"Precedence": "BULK",
			}, nil),
			want: true,
		},
		{
			name: "has Precedence normal",
			msg: createTestMessage(map[string]string{
				"From":       "person@company.com",
				"Precedence": "normal",
			}, nil),
			want: false,
		},
		{
			name: "has both List-Unsubscribe and List-Id",
			msg: createTestMessage(map[string]string{
				"From":             "newsletter@company.com",
				"List-Unsubscribe": "<mailto:unsubscribe@company.com>",
				"List-Id":          "<newsletter.company.com>",
			}, nil),
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsMailingList(tt.msg)
			if got != tt.want {
				t.Errorf("IsMailingList() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsAutomated(t *testing.T) {
	tests := []struct {
		msg  *Message
		name string
		want bool
	}{
		{
			name: "nil message",
			msg:  nil,
			want: false,
		},
		{
			name: "regular email from person",
			msg: createTestMessage(map[string]string{
				"From": "John Doe <john@example.com>",
			}, nil),
			want: false,
		},
		{
			name: "noreply sender",
			msg: createTestMessage(map[string]string{
				"From": "noreply@github.com",
			}, nil),
			want: true,
		},
		{
			name: "no-reply sender with hyphen",
			msg: createTestMessage(map[string]string{
				"From": "no-reply@company.com",
			}, nil),
			want: true,
		},
		{
			name: "donotreply sender",
			msg: createTestMessage(map[string]string{
				"From": "donotreply@bank.com",
			}, nil),
			want: true,
		},
		{
			name: "do-not-reply sender with hyphens",
			msg: createTestMessage(map[string]string{
				"From": "do-not-reply@service.com",
			}, nil),
			want: true,
		},
		{
			name: "notifications sender",
			msg: createTestMessage(map[string]string{
				"From": "notifications@slack.com",
			}, nil),
			want: true,
		},
		{
			name: "notification singular",
			msg: createTestMessage(map[string]string{
				"From": "notification@app.com",
			}, nil),
			want: true,
		},
		{
			name: "automated sender",
			msg: createTestMessage(map[string]string{
				"From": "automated@system.com",
			}, nil),
			want: true,
		},
		{
			name: "bot sender",
			msg: createTestMessage(map[string]string{
				"From": "bot@service.com",
			}, nil),
			want: true,
		},
		{
			name: "mailer-daemon",
			msg: createTestMessage(map[string]string{
				"From": "MAILER-DAEMON@mail.google.com",
			}, nil),
			want: true,
		},
		{
			name: "postmaster",
			msg: createTestMessage(map[string]string{
				"From": "postmaster@mail.example.com",
			}, nil),
			want: true,
		},
		{
			name: "support address",
			msg: createTestMessage(map[string]string{
				"From": "support@company.com",
			}, nil),
			want: true,
		},
		{
			name: "info address",
			msg: createTestMessage(map[string]string{
				"From": "info@company.com",
			}, nil),
			want: true,
		},
		{
			name: "newsletter address",
			msg: createTestMessage(map[string]string{
				"From": "newsletter@company.com",
			}, nil),
			want: true,
		},
		{
			name: "updates address",
			msg: createTestMessage(map[string]string{
				"From": "updates@service.com",
			}, nil),
			want: true,
		},
		{
			name: "alert address",
			msg: createTestMessage(map[string]string{
				"From": "alert@monitoring.com",
			}, nil),
			want: true,
		},
		{
			name: "alerts address plural",
			msg: createTestMessage(map[string]string{
				"From": "alerts@monitoring.com",
			}, nil),
			want: true,
		},
		{
			name: "via in from name",
			msg: createTestMessage(map[string]string{
				"From": "John Doe via Google <john@example.com>",
			}, nil),
			want: true,
		},
		{
			name: "noreply uppercase",
			msg: createTestMessage(map[string]string{
				"From": "NOREPLY@company.com",
			}, nil),
			want: true,
		},
		{
			name: "email in display name",
			msg: createTestMessage(map[string]string{
				"From": "GitHub <noreply@github.com>",
			}, nil),
			want: true,
		},
		{
			name: "email with via but not space delimited",
			msg: createTestMessage(map[string]string{
				"From": "services@company.com",
			}, nil),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsAutomated(tt.msg)
			if got != tt.want {
				t.Errorf("IsAutomated() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsImportant(t *testing.T) {
	tests := []struct {
		name             string
		msg              *Message
		importantSenders []string
		want             bool
	}{
		{
			name: "nil message",
			msg:  nil,
			want: false,
		},
		{
			name: "regular email no important senders",
			msg: createTestMessage(map[string]string{
				"From": "person@example.com",
			}, nil),
			importantSenders: nil,
			want:             false,
		},
		{
			name: "has IMPORTANT label",
			msg: createTestMessage(map[string]string{
				"From": "person@example.com",
			}, []string{"INBOX", "IMPORTANT"}),
			importantSenders: nil,
			want:             true,
		},
		{
			name: "exact match in important senders",
			msg: createTestMessage(map[string]string{
				"From": "manager@company.com",
			}, nil),
			importantSenders: []string{"manager@company.com", "ceo@company.com"},
			want:             true,
		},
		{
			name: "exact match case insensitive",
			msg: createTestMessage(map[string]string{
				"From": "Manager@Company.COM",
			}, nil),
			importantSenders: []string{"manager@company.com"},
			want:             true,
		},
		{
			name: "domain match",
			msg: createTestMessage(map[string]string{
				"From": "anyone@importantclient.com",
			}, nil),
			importantSenders: []string{"@importantclient.com"},
			want:             true,
		},
		{
			name: "domain match case insensitive",
			msg: createTestMessage(map[string]string{
				"From": "EXEC@ImportantClient.COM",
			}, nil),
			importantSenders: []string{"@importantclient.com"},
			want:             true,
		},
		{
			name: "no match in important senders",
			msg: createTestMessage(map[string]string{
				"From": "stranger@unknown.com",
			}, nil),
			importantSenders: []string{"manager@company.com", "@importantclient.com"},
			want:             false,
		},
		{
			name: "empty from email",
			msg: createTestMessage(map[string]string{
				"Subject": "No From Header",
			}, nil),
			importantSenders: []string{"anyone@company.com"},
			want:             false,
		},
		{
			name: "domain pattern without @ prefix does not match",
			msg: createTestMessage(map[string]string{
				"From": "user@company.com",
			}, nil),
			importantSenders: []string{"company.com"}, // Missing @ prefix
			want:             false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsImportant(tt.msg, tt.importantSenders)
			if got != tt.want {
				t.Errorf("IsImportant() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsDirectRecipient(t *testing.T) {
	tests := []struct {
		name      string
		msg       *Message
		userEmail string
		want      bool
	}{
		{
			name:      "nil message",
			msg:       nil,
			userEmail: "user@example.com",
			want:      false,
		},
		{
			name: "empty user email",
			msg: createTestMessage(map[string]string{
				"To": "recipient@example.com",
			}, nil),
			userEmail: "",
			want:      false,
		},
		{
			name: "user is direct recipient",
			msg: createTestMessage(map[string]string{
				"To": "user@example.com",
			}, nil),
			userEmail: "user@example.com",
			want:      true,
		},
		{
			name: "user is one of multiple recipients",
			msg: createTestMessage(map[string]string{
				"To": "first@example.com, user@example.com, third@example.com",
			}, nil),
			userEmail: "user@example.com",
			want:      true,
		},
		{
			name: "user is not a recipient",
			msg: createTestMessage(map[string]string{
				"To": "other@example.com",
			}, nil),
			userEmail: "user@example.com",
			want:      false,
		},
		{
			name: "case insensitive match",
			msg: createTestMessage(map[string]string{
				"To": "USER@EXAMPLE.COM",
			}, nil),
			userEmail: "user@example.com",
			want:      true,
		},
		{
			name: "user is in CC but not To",
			msg: createTestMessage(map[string]string{
				"To": "other@example.com",
				"Cc": "user@example.com",
			}, nil),
			userEmail: "user@example.com",
			want:      false,
		},
		{
			name: "recipient with display name",
			msg: createTestMessage(map[string]string{
				"To": "John Doe <user@example.com>",
			}, nil),
			userEmail: "user@example.com",
			want:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsDirectRecipient(tt.msg, tt.userEmail)
			if got != tt.want {
				t.Errorf("IsDirectRecipient() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsCCRecipient(t *testing.T) {
	tests := []struct {
		name      string
		msg       *Message
		userEmail string
		want      bool
	}{
		{
			name:      "nil message",
			msg:       nil,
			userEmail: "user@example.com",
			want:      false,
		},
		{
			name: "empty user email",
			msg: createTestMessage(map[string]string{
				"Cc": "recipient@example.com",
			}, nil),
			userEmail: "",
			want:      false,
		},
		{
			name: "user is CC recipient",
			msg: createTestMessage(map[string]string{
				"Cc": "user@example.com",
			}, nil),
			userEmail: "user@example.com",
			want:      true,
		},
		{
			name: "user is one of multiple CC recipients",
			msg: createTestMessage(map[string]string{
				"Cc": "first@example.com, user@example.com, third@example.com",
			}, nil),
			userEmail: "user@example.com",
			want:      true,
		},
		{
			name: "user is not a CC recipient",
			msg: createTestMessage(map[string]string{
				"Cc": "other@example.com",
			}, nil),
			userEmail: "user@example.com",
			want:      false,
		},
		{
			name: "case insensitive match",
			msg: createTestMessage(map[string]string{
				"Cc": "USER@EXAMPLE.COM",
			}, nil),
			userEmail: "user@example.com",
			want:      true,
		},
		{
			name: "user is in To but not CC",
			msg: createTestMessage(map[string]string{
				"To": "user@example.com",
				"Cc": "other@example.com",
			}, nil),
			userEmail: "user@example.com",
			want:      false,
		},
		{
			name: "CC recipient with display name",
			msg: createTestMessage(map[string]string{
				"Cc": "John Doe <user@example.com>",
			}, nil),
			userEmail: "user@example.com",
			want:      true,
		},
		{
			name: "no CC header",
			msg: createTestMessage(map[string]string{
				"To": "other@example.com",
			}, nil),
			userEmail: "user@example.com",
			want:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsCCRecipient(tt.msg, tt.userEmail)
			if got != tt.want {
				t.Errorf("IsCCRecipient() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFilterMessages(t *testing.T) {
	tests := []struct {
		name     string
		messages []*Message
		wantLen  int
	}{
		{
			name:     "nil messages",
			messages: nil,
			wantLen:  0,
		},
		{
			name:     "empty messages",
			messages: []*Message{},
			wantLen:  0,
		},
		{
			name: "all messages from real people",
			messages: []*Message{
				createTestMessage(map[string]string{"From": "person1@example.com"}, nil),
				createTestMessage(map[string]string{"From": "person2@example.com"}, nil),
			},
			wantLen: 2,
		},
		{
			name: "filters mailing lists",
			messages: []*Message{
				createTestMessage(map[string]string{"From": "person@example.com"}, nil),
				createTestMessage(map[string]string{
					"From":             "newsletter@company.com",
					"List-Unsubscribe": "<mailto:unsub@company.com>",
				}, nil),
			},
			wantLen: 1,
		},
		{
			name: "filters automated senders",
			messages: []*Message{
				createTestMessage(map[string]string{"From": "person@example.com"}, nil),
				createTestMessage(map[string]string{"From": "noreply@github.com"}, nil),
				createTestMessage(map[string]string{"From": "notifications@slack.com"}, nil),
			},
			wantLen: 1,
		},
		{
			name: "filters both mailing lists and automated",
			messages: []*Message{
				createTestMessage(map[string]string{"From": "person@example.com"}, nil),
				createTestMessage(map[string]string{
					"From":    "updates@company.com",
					"List-Id": "<updates.company.com>",
				}, nil),
				createTestMessage(map[string]string{"From": "bot@service.com"}, nil),
			},
			wantLen: 1,
		},
		{
			name: "skips nil messages in slice",
			messages: []*Message{
				createTestMessage(map[string]string{"From": "person1@example.com"}, nil),
				nil,
				createTestMessage(map[string]string{"From": "person2@example.com"}, nil),
			},
			wantLen: 2,
		},
		{
			name: "all filtered out",
			messages: []*Message{
				createTestMessage(map[string]string{"From": "noreply@company.com"}, nil),
				createTestMessage(map[string]string{
					"From":             "list@group.com",
					"List-Unsubscribe": "<mailto:unsub@group.com>",
				}, nil),
			},
			wantLen: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FilterMessages(tt.messages)
			if len(got) != tt.wantLen {
				t.Errorf("FilterMessages() returned %d messages, want %d", len(got), tt.wantLen)
			}
		})
	}
}

func TestFilterMessagesWithOptions(t *testing.T) {
	tests := []struct {
		name     string
		messages []*Message
		opts     FilterOptions
		wantLen  int
	}{
		{
			name:     "nil messages",
			messages: nil,
			opts:     FilterOptions{},
			wantLen:  0,
		},
		{
			name: "default options filters mailing lists and automated",
			messages: []*Message{
				createTestMessage(map[string]string{"From": "person@example.com"}, nil),
				createTestMessage(map[string]string{"From": "noreply@company.com"}, nil),
				createTestMessage(map[string]string{
					"From":             "list@group.com",
					"List-Unsubscribe": "<mailto:unsub@group.com>",
				}, nil),
			},
			opts:    FilterOptions{},
			wantLen: 1,
		},
		{
			name: "include mailing lists",
			messages: []*Message{
				createTestMessage(map[string]string{"From": "person@example.com"}, nil),
				createTestMessage(map[string]string{
					"From":             "list@group.com",
					"List-Unsubscribe": "<mailto:unsub@group.com>",
				}, nil),
			},
			opts:    FilterOptions{IncludeMailingLists: true},
			wantLen: 2,
		},
		{
			name: "include automated",
			messages: []*Message{
				createTestMessage(map[string]string{"From": "person@example.com"}, nil),
				createTestMessage(map[string]string{"From": "noreply@company.com"}, nil),
			},
			opts:    FilterOptions{IncludeAutomated: true},
			wantLen: 2,
		},
		{
			name: "filter by user email as recipient",
			messages: []*Message{
				createTestMessage(map[string]string{
					"From": "person1@example.com",
					"To":   "user@example.com",
				}, nil),
				createTestMessage(map[string]string{
					"From": "person2@example.com",
					"To":   "other@example.com",
					"Cc":   "user@example.com",
				}, nil),
				createTestMessage(map[string]string{
					"From": "person3@example.com",
					"To":   "other@example.com",
				}, nil),
			},
			opts:    FilterOptions{UserEmail: "user@example.com"},
			wantLen: 2, // First two messages include user
		},
		{
			name: "direct only filters CC recipients",
			messages: []*Message{
				createTestMessage(map[string]string{
					"From": "person1@example.com",
					"To":   "user@example.com",
				}, nil),
				createTestMessage(map[string]string{
					"From": "person2@example.com",
					"To":   "other@example.com",
					"Cc":   "user@example.com",
				}, nil),
			},
			opts: FilterOptions{
				UserEmail:  "user@example.com",
				DirectOnly: true,
			},
			wantLen: 1, // Only first message where user is in To
		},
		{
			name: "all options combined",
			messages: []*Message{
				createTestMessage(map[string]string{
					"From": "person@example.com",
					"To":   "user@example.com",
				}, nil),
				createTestMessage(map[string]string{
					"From":             "noreply@company.com",
					"To":               "user@example.com",
					"List-Unsubscribe": "<mailto:unsub@company.com>",
				}, nil),
			},
			opts: FilterOptions{
				UserEmail:           "user@example.com",
				IncludeMailingLists: true,
				IncludeAutomated:    true,
			},
			wantLen: 2, // Both pass because includes are enabled
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FilterMessagesWithOptions(tt.messages, tt.opts)
			if len(got) != tt.wantLen {
				t.Errorf("FilterMessagesWithOptions() returned %d messages, want %d", len(got), tt.wantLen)
			}
		})
	}
}

func TestAutomatedPatterns(t *testing.T) {
	// Verify that all documented patterns are present
	expectedPatterns := []string{
		"noreply@",
		"no-reply@",
		"donotreply@",
		"do-not-reply@",
		"notifications@",
		"notification@",
		"automated@",
		"bot@",
		"mailer-daemon@",
		"postmaster@",
		"support@",
		"info@",
		"newsletter@",
		"updates@",
		"alert@",
		"alerts@",
	}

	for _, expected := range expectedPatterns {
		found := false
		for _, actual := range automatedPatterns {
			if expected == actual {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected automated pattern %q not found", expected)
		}
	}

	// Verify count matches
	if len(automatedPatterns) != len(expectedPatterns) {
		t.Errorf("automatedPatterns has %d entries, expected %d",
			len(automatedPatterns), len(expectedPatterns))
	}
}
