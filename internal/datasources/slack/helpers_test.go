package slack

import (
	"testing"
	"time"

	"github.com/dakaneye/kora/internal/datasources"
	"github.com/dakaneye/kora/internal/models"
)

func TestGetWorkspaceFromPermalink(t *testing.T) {
	tests := []struct {
		name      string
		permalink string
		want      string
	}{
		{
			name:      "standard workspace permalink",
			permalink: "https://myworkspace.slack.com/archives/C12345/p1234567890",
			want:      "myworkspace",
		},
		{
			name:      "app.slack.com permalink",
			permalink: "https://app.slack.com/archives/C12345/p1234567890",
			want:      "",
		},
		{
			name:      "empty permalink",
			permalink: "",
			want:      "",
		},
		{
			name:      "invalid URL",
			permalink: "not-a-url",
			want:      "",
		},
		{
			name:      "slack URL without subdomain",
			permalink: "https://slack.com/archives/C12345",
			want:      "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getWorkspaceFromPermalink(tt.permalink)
			if got != tt.want {
				t.Errorf("getWorkspaceFromPermalink() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestBuildDMPermalink(t *testing.T) {
	tests := []struct {
		name      string
		channelID string
		ts        string
		want      string
	}{
		{
			name:      "valid channel and timestamp - returns empty without team ID",
			channelID: "D12345678",
			ts:        "1234567890.123456",
			want:      "", // Empty because we don't have team ID
		},
		{
			name:      "empty channel ID",
			channelID: "",
			ts:        "1234567890.123456",
			want:      "",
		},
		{
			name:      "empty timestamp",
			channelID: "D12345678",
			ts:        "",
			want:      "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildDMPermalink(tt.channelID, tt.ts)
			if got != tt.want {
				t.Errorf("buildDMPermalink() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDeduplicateEvents(t *testing.T) {
	now := time.Now().UTC()

	tests := []struct {
		name   string
		events []models.Event
		want   int // expected count after dedup
	}{
		{
			name:   "empty list",
			events: []models.Event{},
			want:   0,
		},
		{
			name: "no duplicates",
			events: []models.Event{
				{URL: "https://example.com/1", Timestamp: now},
				{URL: "https://example.com/2", Timestamp: now},
			},
			want: 2,
		},
		{
			name: "duplicate URLs removed",
			events: []models.Event{
				{URL: "https://example.com/1", Timestamp: now},
				{URL: "https://example.com/1", Timestamp: now},
				{URL: "https://example.com/2", Timestamp: now},
			},
			want: 2,
		},
		{
			name: "empty URLs use composite key",
			events: []models.Event{
				{URL: "", Source: models.SourceSlack, Type: models.EventTypeSlackDM, Timestamp: now},
				{URL: "", Source: models.SourceSlack, Type: models.EventTypeSlackMention, Timestamp: now},
			},
			want: 2, // Different types = different keys
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := deduplicateEvents(tt.events)
			if len(got) != tt.want {
				t.Errorf("deduplicateEvents() returned %d events, want %d", len(got), tt.want)
			}
		})
	}
}

func TestFilterEvents(t *testing.T) {
	now := time.Now().UTC()
	events := []models.Event{
		{Type: models.EventTypeSlackDM, Priority: models.PriorityHigh, RequiresAction: true, Timestamp: now},
		{Type: models.EventTypeSlackMention, Priority: models.PriorityMedium, RequiresAction: false, Timestamp: now},
		{Type: models.EventTypeSlackDM, Priority: models.PriorityLow, RequiresAction: true, Timestamp: now},
	}

	//nolint:govet // test struct alignment not relevant
	tests := []struct {
		name string
		opts datasources.FetchOptions
		want int
	}{
		{
			name: "no filter",
			opts: datasources.FetchOptions{Since: now.Add(-time.Hour)},
			want: 3,
		},
		{
			name: "filter by event type - DM only",
			opts: datasources.FetchOptions{
				Since: now.Add(-time.Hour),
				Filter: &datasources.FetchFilter{
					EventTypes: []models.EventType{models.EventTypeSlackDM},
				},
			},
			want: 2,
		},
		{
			name: "filter by event type - mention only",
			opts: datasources.FetchOptions{
				Since: now.Add(-time.Hour),
				Filter: &datasources.FetchFilter{
					EventTypes: []models.EventType{models.EventTypeSlackMention},
				},
			},
			want: 1,
		},
		{
			name: "filter by priority - high and medium",
			opts: datasources.FetchOptions{
				Since: now.Add(-time.Hour),
				Filter: &datasources.FetchFilter{
					MinPriority: models.PriorityMedium,
				},
			},
			want: 2,
		},
		{
			name: "filter by requires action",
			opts: datasources.FetchOptions{
				Since: now.Add(-time.Hour),
				Filter: &datasources.FetchFilter{
					RequiresAction: true,
				},
			},
			want: 2,
		},
		{
			name: "combined filters",
			opts: datasources.FetchOptions{
				Since: now.Add(-time.Hour),
				Filter: &datasources.FetchFilter{
					EventTypes:     []models.EventType{models.EventTypeSlackDM},
					MinPriority:    models.PriorityMedium,
					RequiresAction: true,
				},
			},
			want: 1, // Only the high priority DM
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := filterEvents(events, tt.opts)
			if len(got) != tt.want {
				t.Errorf("filterEvents() returned %d events, want %d", len(got), tt.want)
			}
		})
	}
}
