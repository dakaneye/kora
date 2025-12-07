package github

import (
	"testing"

	"github.com/dakaneye/kora/internal/datasources"
	"github.com/dakaneye/kora/internal/models"
)

func TestBuildOrgFilter(t *testing.T) {
	//nolint:govet // test struct field order prioritizes readability
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

// Note: TestDeduplicateEvents is in internal/models/dedup_test.go
// The function models.DeduplicateEvents is used in datasource.go

func TestFilterEvents(t *testing.T) {
	events := []models.Event{
		{URL: "1", Type: models.EventTypePRReview, Priority: models.PriorityHigh, RequiresAction: true},
		{URL: "2", Type: models.EventTypePRMention, Priority: models.PriorityMedium, RequiresAction: false},
		{URL: "3", Type: models.EventTypeIssueMention, Priority: models.PriorityMedium, RequiresAction: false},
		{URL: "4", Type: models.EventTypeIssueAssigned, Priority: models.PriorityLow, RequiresAction: true},
	}

	//nolint:govet // test struct field order prioritizes readability
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
	t.Run("short title unchanged", func(t *testing.T) {
		got := truncateTitle("Short title")
		if got != "Short title" {
			t.Errorf("truncateTitle() = %q, want %q", got, "Short title")
		}
	})

	t.Run("long title truncated to 100 runes", func(t *testing.T) {
		// This title has 122 characters/runes
		input := "This is a very long title that exceeds the hundred character limit and should be truncated by the datasource implementation"
		got := truncateTitle(input)
		gotRunes := len([]rune(got))
		if gotRunes != 100 {
			t.Errorf("truncateTitle() returned %d runes, want 100", gotRunes)
		}
		// Should end with "..."
		if got[len(got)-3:] != "..." {
			t.Errorf("truncateTitle() should end with '...', got %q", got[len(got)-3:])
		}
	})

	t.Run("UTF-8 safe truncation", func(t *testing.T) {
		// Build a string with exactly 110 Japanese characters (each is 1 rune)
		// to ensure truncation happens
		input := "日本語のテキストがとても長くなることがあります。これは百文字を超える長いタイトルです。さらにテキストを追加します。これは更に長いテキストです。もっと追加します。終わりの文字です。ここまで来たら十分長い。"
		inputRunes := len([]rune(input))
		if inputRunes <= 100 {
			t.Fatalf("test setup error: input has only %d runes, need >100", inputRunes)
		}

		got := truncateTitle(input)
		gotRunes := len([]rune(got))
		if gotRunes != 100 {
			t.Errorf("truncateTitle() returned %d runes, want 100", gotRunes)
		}
		// Verify it ends with "..." (3 ASCII chars)
		if got[len(got)-3:] != "..." {
			t.Errorf("truncateTitle() should end with '...', got %q", got[len(got)-3:])
		}
	})

	t.Run("emoji handling", func(t *testing.T) {
		// Emojis can be multiple code points; test that we don't crash
		input := "Fix bug 🔥🔥🔥🔥🔥🔥🔥🔥🔥🔥🔥🔥🔥🔥🔥🔥🔥🔥🔥🔥🔥🔥🔥🔥🔥🔥🔥🔥🔥🔥🔥🔥🔥🔥🔥🔥🔥🔥🔥🔥🔥🔥🔥🔥🔥🔥🔥🔥🔥🔥🔥🔥🔥🔥🔥🔥🔥🔥🔥🔥🔥🔥🔥🔥🔥🔥🔥🔥🔥🔥🔥🔥🔥🔥🔥🔥🔥🔥🔥🔥🔥🔥🔥🔥🔥🔥🔥🔥🔥🔥🔥🔥🔥🔥🔥🔥🔥🔥🔥🔥"
		got := truncateTitle(input)
		gotRunes := len([]rune(got))
		if gotRunes > 100 {
			t.Errorf("truncateTitle() returned %d runes, max is 100", gotRunes)
		}
	})

	t.Run("newline removed", func(t *testing.T) {
		input := "Title with\nnewline"
		got := truncateTitle(input)
		want := "Title with newline"
		if got != want {
			t.Errorf("truncateTitle() = %q, want %q", got, want)
		}
	})

	t.Run("whitespace trimmed", func(t *testing.T) {
		input := "  Title with leading and trailing spaces  "
		got := truncateTitle(input)
		want := "Title with leading and trailing spaces"
		if got != want {
			t.Errorf("truncateTitle() = %q, want %q", got, want)
		}
	})

	t.Run("short title with UTF-8 unchanged", func(t *testing.T) {
		input := "日本語"
		got := truncateTitle(input)
		if got != input {
			t.Errorf("truncateTitle() = %q, want %q", got, input)
		}
	})
}
