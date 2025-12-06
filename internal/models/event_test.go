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
			name: "valid Slack DM event",
			event: Event{
				Type:           EventTypeSlackDM,
				Title:          "Question about deployment",
				Source:         SourceSlack,
				URL:            "https://slack.com/archives/D123/p123",
				Author:         Person{Username: "U12345678"},
				Timestamp:      time.Now(),
				Priority:       PriorityHigh,
				RequiresAction: true,
				Metadata: map[string]any{
					"workspace": "company",
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
			name: "invalid Slack metadata key fails",
			event: Event{
				Type:      EventTypeSlackDM,
				Title:     "Test",
				Source:    SourceSlack,
				URL:       "https://slack.com/archives/D123/p123",
				Author:    Person{Username: "testuser"},
				Timestamp: time.Now(),
				Priority:  PriorityHigh,
				Metadata: map[string]any{
					"unknown_key": "value",
				},
			},
			wantErr: true,
			errMsg:  "invalid metadata keys for slack",
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
					"repo":         "owner/repo",
					"number":       1,
					"state":        "open",
					"review_state": "pending",
					"labels":       []string{"bug"},
				},
			},
			wantErr: false,
		},
		{
			name: "valid Slack metadata keys pass",
			event: Event{
				Type:      EventTypeSlackDM,
				Title:     "Test",
				Source:    SourceSlack,
				URL:       "https://slack.com/archives/D123/p123",
				Author:    Person{Username: "testuser"},
				Timestamp: time.Now(),
				Priority:  PriorityHigh,
				Metadata: map[string]any{
					"workspace":       "company",
					"channel":         "general",
					"thread_ts":       "123.456",
					"is_thread_reply": true,
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
		{"PR review is valid", EventTypePRReview, true},
		{"PR mention is valid", EventTypePRMention, true},
		{"Issue mention is valid", EventTypeIssueMention, true},
		{"Issue assigned is valid", EventTypeIssueAssigned, true},
		{"Slack DM is valid", EventTypeSlackDM, true},
		{"Slack mention is valid", EventTypeSlackMention, true},
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
		{EventTypePRReview, "pr_review"},
		{EventTypePRMention, "pr_mention"},
		{EventTypeIssueMention, "issue_mention"},
		{EventTypeIssueAssigned, "issue_assigned"},
		{EventTypeSlackDM, "slack_dm"},
		{EventTypeSlackMention, "slack_mention"},
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
		{"GitHub is valid", SourceGitHub, true},
		{"Slack is valid", SourceSlack, true},
		{"empty string is invalid", Source(""), false},
		{"random string is invalid", Source("random"), false},
		{"jira is invalid", Source("jira"), false},
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
		{SourceSlack, "slack"},
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
