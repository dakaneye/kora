package output

import (
	"testing"
	"time"

	"github.com/dakaneye/kora/internal/models"
)

func TestNewFormatter(t *testing.T) {
	tests := []struct {
		name    string
		format  string
		wantErr bool
	}{
		{"json format", "json", false},
		{"json-pretty format", "json-pretty", false},
		{"markdown format", "markdown", false},
		{"md alias", "md", false},
		{"text format", "text", false},
		{"txt alias", "txt", false},
		{"unknown format", "xml", true},
		{"empty format", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f, err := NewFormatter(tt.format)
			if tt.wantErr {
				if err == nil {
					t.Errorf("NewFormatter(%q) expected error, got nil", tt.format)
				}
				return
			}
			if err != nil {
				t.Errorf("NewFormatter(%q) unexpected error: %v", tt.format, err)
			}
			if f == nil {
				t.Errorf("NewFormatter(%q) returned nil formatter", tt.format)
			}
		})
	}
}

func TestNewFormatStats(t *testing.T) {
	now := time.Now()

	events := []models.Event{
		{
			Type:           models.EventTypePRReview,
			Title:          "Review PR 1",
			Source:         models.SourceGitHub,
			Author:         models.Person{Username: "alice"},
			Timestamp:      now,
			Priority:       models.PriorityHigh,
			RequiresAction: true,
		},
		{
			Type:           models.EventTypePRReview,
			Title:          "Review PR 2",
			Source:         models.SourceGitHub,
			Author:         models.Person{Username: "bob"},
			Timestamp:      now,
			Priority:       models.PriorityCritical,
			RequiresAction: true,
		},
		{
			Type:           models.EventTypeSlackMention,
			Title:          "Mention in general",
			Source:         models.SourceSlack,
			Author:         models.Person{Username: "charlie"},
			Timestamp:      now,
			Priority:       models.PriorityMedium,
			RequiresAction: false,
		},
	}

	sourceErrors := map[string]string{
		"calendar": "auth failed",
	}

	stats := NewFormatStats(events, 5*time.Second, sourceErrors)

	if stats.TotalEvents != 3 {
		t.Errorf("TotalEvents = %d, want 3", stats.TotalEvents)
	}
	if stats.RequiresAction != 2 {
		t.Errorf("RequiresAction = %d, want 2", stats.RequiresAction)
	}
	if stats.ByPriority[models.PriorityCritical] != 1 {
		t.Errorf("ByPriority[Critical] = %d, want 1", stats.ByPriority[models.PriorityCritical])
	}
	if stats.ByPriority[models.PriorityHigh] != 1 {
		t.Errorf("ByPriority[High] = %d, want 1", stats.ByPriority[models.PriorityHigh])
	}
	if stats.ByPriority[models.PriorityMedium] != 1 {
		t.Errorf("ByPriority[Medium] = %d, want 1", stats.ByPriority[models.PriorityMedium])
	}
	if stats.ByPriority[models.PriorityLow] != 0 {
		t.Errorf("ByPriority[Low] = %d, want 0", stats.ByPriority[models.PriorityLow])
	}
	if stats.ByPriority[models.PriorityInfo] != 0 {
		t.Errorf("ByPriority[Info] = %d, want 0", stats.ByPriority[models.PriorityInfo])
	}
	if stats.Duration != 5*time.Second {
		t.Errorf("Duration = %v, want 5s", stats.Duration)
	}
	if stats.SourceErrors["calendar"] != "auth failed" {
		t.Errorf("SourceErrors[calendar] = %q, want 'auth failed'", stats.SourceErrors["calendar"])
	}
}

func TestNewFormatStatsEmpty(t *testing.T) {
	stats := NewFormatStats(nil, 0, nil)

	if stats.TotalEvents != 0 {
		t.Errorf("TotalEvents = %d, want 0", stats.TotalEvents)
	}
	if stats.RequiresAction != 0 {
		t.Errorf("RequiresAction = %d, want 0", stats.RequiresAction)
	}
	if stats.SourceErrors == nil {
		t.Error("SourceErrors should not be nil")
	}
}

// testEvents returns a slice of test events for formatter tests.
func testEvents() []models.Event {
	now := time.Now()
	return []models.Event{
		{
			Type:           models.EventTypePRReview,
			Title:          "Review requested: Add feature X",
			Source:         models.SourceGitHub,
			URL:            "https://github.com/org/repo/pull/123",
			Author:         models.Person{Name: "Jane Developer", Username: "janedev"},
			Timestamp:      now.Add(-2 * time.Hour),
			Priority:       models.PriorityHigh,
			RequiresAction: true,
			Metadata: map[string]any{
				"repo":   "org/repo",
				"number": 123,
				"state":  "open",
			},
		},
		{
			Type:           models.EventTypeSlackDM,
			Title:          "Question about deployment",
			Source:         models.SourceSlack,
			URL:            "https://workspace.slack.com/archives/D123/p1234567890",
			Author:         models.Person{Name: "Bob Manager", Username: "bobm"},
			Timestamp:      now.Add(-30 * time.Minute),
			Priority:       models.PriorityHigh,
			RequiresAction: true,
			Metadata: map[string]any{
				"workspace": "company",
			},
		},
		{
			Type:           models.EventTypeSlackMention,
			Title:          "Mentioned in #general",
			Source:         models.SourceSlack,
			URL:            "https://workspace.slack.com/archives/C123/p9876543210",
			Author:         models.Person{Username: "colleague"},
			Timestamp:      now.Add(-1 * time.Hour),
			Priority:       models.PriorityMedium,
			RequiresAction: false,
			Metadata: map[string]any{
				"workspace": "company",
				"channel":   "general",
			},
		},
		{
			Type:           models.EventTypeIssueMention,
			Title:          "Bug report needs triage",
			Source:         models.SourceGitHub,
			URL:            "https://github.com/org/repo/issues/456",
			Author:         models.Person{Username: "triager"},
			Timestamp:      now.Add(-3 * time.Hour),
			Priority:       models.PriorityLow,
			RequiresAction: false,
			Metadata: map[string]any{
				"repo":   "org/repo",
				"number": 456,
				"state":  "open",
			},
		},
	}
}

// testStats returns stats for the test events.
func testStats() *FormatStats {
	events := testEvents()
	return NewFormatStats(events, 2*time.Second, nil)
}

// testStatsWithErrors returns stats with partial success.
func testStatsWithErrors() *FormatStats {
	events := testEvents()
	return NewFormatStats(events, 2*time.Second, map[string]string{
		"github": "rate limited",
	})
}

// comprehensiveTestEvents returns events covering all EventTypes and edge cases.
func comprehensiveTestEvents() []models.Event {
	now := time.Now()
	return []models.Event{
		// All EventType values
		{
			Type:           models.EventTypePRReview,
			Title:          "PR review requested",
			Source:         models.SourceGitHub,
			URL:            "https://github.com/org/repo/pull/1",
			Author:         models.Person{Username: "user1"},
			Timestamp:      now.Add(-1 * time.Hour),
			Priority:       models.PriorityCritical,
			RequiresAction: true,
			Metadata: map[string]any{
				"repo":   "org/repo",
				"number": 1,
				"state":  "open",
			},
		},
		{
			Type:           models.EventTypePRMention,
			Title:          "Mentioned in PR comment",
			Source:         models.SourceGitHub,
			URL:            "https://github.com/org/repo/pull/2",
			Author:         models.Person{Username: "user2"},
			Timestamp:      now.Add(-2 * time.Hour),
			Priority:       models.PriorityHigh,
			RequiresAction: true,
			Metadata: map[string]any{
				"repo":   "org/repo",
				"number": 2,
			},
		},
		{
			Type:           models.EventTypeIssueMention,
			Title:          "Mentioned in issue",
			Source:         models.SourceGitHub,
			URL:            "https://github.com/org/repo/issues/3",
			Author:         models.Person{Username: "user3"},
			Timestamp:      now.Add(-3 * time.Hour),
			Priority:       models.PriorityMedium,
			RequiresAction: false,
			Metadata: map[string]any{
				"repo":   "org/repo",
				"number": 3,
				"labels": []string{"bug", "help-wanted"},
			},
		},
		{
			Type:           models.EventTypeIssueAssigned,
			Title:          "Issue assigned to you",
			Source:         models.SourceGitHub,
			URL:            "https://github.com/org/repo/issues/4",
			Author:         models.Person{Name: "Assigner", Username: "user4"},
			Timestamp:      now.Add(-4 * time.Hour),
			Priority:       models.PriorityLow,
			RequiresAction: true,
			Metadata: map[string]any{
				"repo":   "org/repo",
				"number": 4,
				"state":  "open",
				"labels": []string{"enhancement"},
			},
		},
		{
			Type:           models.EventTypeSlackDM,
			Title:          "Direct message",
			Source:         models.SourceSlack,
			URL:            "https://workspace.slack.com/archives/D123/p1234",
			Author:         models.Person{Username: "U12345"},
			Timestamp:      now.Add(-5 * time.Hour),
			Priority:       models.PriorityInfo,
			RequiresAction: false,
			Metadata: map[string]any{
				"workspace": "company",
			},
		},
		{
			Type:           models.EventTypeSlackMention,
			Title:          "Mentioned in #engineering",
			Source:         models.SourceSlack,
			URL:            "https://workspace.slack.com/archives/C456/p5678",
			Author:         models.Person{Username: "U67890"},
			Timestamp:      now.Add(-6 * time.Hour),
			Priority:       models.PriorityHigh,
			RequiresAction: false,
			Metadata: map[string]any{
				"workspace": "company",
				"channel":   "engineering",
			},
		},
		// Edge case: 100-char title (max length)
		{
			Type:           models.EventTypePRReview,
			Title:          "This is a very long title that approaches the maximum allowed length for event titles in our system",
			Source:         models.SourceGitHub,
			URL:            "https://github.com/org/repo/pull/99",
			Author:         models.Person{Username: "longuser"},
			Timestamp:      now.Add(-7 * time.Hour),
			Priority:       models.PriorityMedium,
			RequiresAction: false,
		},
		// Edge case: Minimal event (no metadata)
		{
			Type:           models.EventTypeIssueMention,
			Title:          "Minimal event",
			Source:         models.SourceGitHub,
			URL:            "https://github.com/org/repo/issues/100",
			Author:         models.Person{Username: "minimal"},
			Timestamp:      now.Add(-8 * time.Hour),
			Priority:       models.PriorityInfo,
			RequiresAction: false,
			Metadata:       nil,
		},
	}
}
