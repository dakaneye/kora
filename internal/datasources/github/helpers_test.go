package github

import (
	"testing"

	"github.com/dakaneye/kora/internal/datasources"
	"github.com/dakaneye/kora/internal/models"
)

func TestBuildOrgFilter(t *testing.T) {
	tests := []struct {
		name string
		orgs []string
		want string
	}{
		{
			name: "empty",
			orgs: nil,
			want: "",
		},
		{
			name: "single org",
			orgs: []string{"example"},
			want: "org:example",
		},
		{
			name: "multiple orgs",
			orgs: []string{"chainguard-dev", "sigstore"},
			want: "org:chainguard-dev org:sigstore",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildOrgFilter(tt.orgs)
			if got != tt.want {
				t.Errorf("buildOrgFilter(%v) = %q, want %q", tt.orgs, got, tt.want)
			}
		})
	}
}

func TestDeduplicateEvents(t *testing.T) {
	events := []models.Event{
		{URL: "https://github.com/a/b/1", Type: models.EventTypePRReview},
		{URL: "https://github.com/a/b/2", Type: models.EventTypePRMention},
		{URL: "https://github.com/a/b/1", Type: models.EventTypePRMention}, // duplicate
		{URL: "https://github.com/a/b/3", Type: models.EventTypeIssueMention},
	}

	result := deduplicateEvents(events)

	if len(result) != 3 {
		t.Errorf("expected 3 events after dedup, got %d", len(result))
	}

	// Verify first occurrence wins (PRReview, not PRMention)
	for _, e := range result {
		if e.URL == "https://github.com/a/b/1" && e.Type != models.EventTypePRReview {
			t.Errorf("expected first occurrence to win, got type %s", e.Type)
		}
	}
}

func TestFilterEvents(t *testing.T) {
	events := []models.Event{
		{URL: "1", Type: models.EventTypePRReview, Priority: models.PriorityHigh, RequiresAction: true},
		{URL: "2", Type: models.EventTypePRMention, Priority: models.PriorityMedium, RequiresAction: false},
		{URL: "3", Type: models.EventTypeIssueMention, Priority: models.PriorityMedium, RequiresAction: false},
		{URL: "4", Type: models.EventTypeIssueAssigned, Priority: models.PriorityLow, RequiresAction: true},
	}

	tests := []struct {
		name      string
		filter    *datasources.FetchFilter
		wantCount int
		wantURLs  []string
	}{
		{
			name:      "nil filter returns all",
			filter:    nil,
			wantCount: 4,
		},
		{
			name: "filter by event type",
			filter: &datasources.FetchFilter{
				EventTypes: []models.EventType{models.EventTypePRReview, models.EventTypePRMention},
			},
			wantCount: 2,
			wantURLs:  []string{"1", "2"},
		},
		{
			name: "filter by min priority",
			filter: &datasources.FetchFilter{
				MinPriority: models.PriorityMedium,
			},
			wantCount: 3, // High and Medium priorities (1, 2, 3)
			wantURLs:  []string{"1", "2", "3"},
		},
		{
			name: "filter by requires action",
			filter: &datasources.FetchFilter{
				RequiresAction: true,
			},
			wantCount: 2, // 1 and 4
			wantURLs:  []string{"1", "4"},
		},
		{
			name: "combined filters",
			filter: &datasources.FetchFilter{
				EventTypes:     []models.EventType{models.EventTypePRReview, models.EventTypeIssueAssigned},
				RequiresAction: true,
			},
			wantCount: 2,
			wantURLs:  []string{"1", "4"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := datasources.FetchOptions{Filter: tt.filter}
			result := filterEvents(events, opts)

			if len(result) != tt.wantCount {
				t.Errorf("expected %d events, got %d", tt.wantCount, len(result))
			}

			if tt.wantURLs != nil {
				for i, wantURL := range tt.wantURLs {
					if i >= len(result) {
						t.Errorf("missing expected URL %s", wantURL)
						continue
					}
					if result[i].URL != wantURL {
						t.Errorf("result[%d].URL = %s, want %s", i, result[i].URL, wantURL)
					}
				}
			}
		})
	}
}

func TestTruncateTitle(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "short title unchanged",
			input: "Short title",
			want:  "Short title",
		},
		{
			name:  "exactly 100 chars unchanged",
			input: "This is exactly one hundred characters long xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
			want:  "This is exactly one hundred characters long xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
		},
		{
			name:  "long title truncated",
			input: "This is a very long title that exceeds the hundred character limit and should be truncated by the datasource implementation",
			want:  "This is a very long title that exceeds the hundred character limit and should be truncated by the...",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncateTitle(tt.input)
			if got != tt.want {
				t.Errorf("truncateTitle() = %q (len=%d), want %q (len=%d)", got, len(got), tt.want, len(tt.want))
			}
			if len(got) > 100 {
				t.Errorf("truncateTitle() returned %d chars, max is 100", len(got))
			}
		})
	}
}
