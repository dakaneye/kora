package output

import (
	"strings"
	"testing"
	"time"

	"github.com/dakaneye/kora/internal/models"
)

func TestMarkdownFormatter_Format(t *testing.T) {
	events := testEvents()
	stats := testStats()

	f := NewMarkdownFormatter()
	out, err := f.Format(events, stats)
	if err != nil {
		t.Fatalf("Format() error: %v", err)
	}

	// Verify header
	if !strings.Contains(out, "# Digest: 4 items") {
		t.Error("Output should contain digest header with count")
	}
	if !strings.Contains(out, "(2 require action)") {
		t.Error("Output should contain require action count")
	}

	// Verify sections
	if !strings.Contains(out, "## Requires Action (2)") {
		t.Error("Output should contain 'Requires Action' section")
	}
	if !strings.Contains(out, "## For Awareness (2)") {
		t.Error("Output should contain 'For Awareness' section")
	}
}

func TestMarkdownFormatter_FormatEventDetails(t *testing.T) {
	events := testEvents()
	stats := testStats()

	f := NewMarkdownFormatter()
	out, err := f.Format(events, stats)
	if err != nil {
		t.Fatalf("Format() error: %v", err)
	}

	// Verify event headers
	if !strings.Contains(out, "### [PR Review] Review requested: Add feature X") {
		t.Error("Output should contain PR review event header")
	}
	if !strings.Contains(out, "### [Slack DM] Question about deployment") {
		t.Error("Output should contain Slack DM event header")
	}

	// Verify bullet point details
	if !strings.Contains(out, "- URL:") {
		t.Error("Output should contain URL bullet")
	}
	if !strings.Contains(out, "- Author: @") {
		t.Error("Output should contain Author bullet with @ prefix")
	}
	if !strings.Contains(out, "- Time:") {
		t.Error("Output should contain Time bullet")
	}
	if !strings.Contains(out, "- Priority:") {
		t.Error("Output should contain Priority bullet")
	}
}

func TestMarkdownFormatter_FormatMetadata(t *testing.T) {
	events := testEvents()
	stats := testStats()

	f := NewMarkdownFormatter()
	out, err := f.Format(events, stats)
	if err != nil {
		t.Fatalf("Format() error: %v", err)
	}

	// GitHub metadata
	if !strings.Contains(out, "- Repo: org/repo") {
		t.Error("Output should contain repo metadata")
	}
	if !strings.Contains(out, "- State: open") {
		t.Error("Output should contain state metadata")
	}

	// Slack metadata
	if !strings.Contains(out, "- Workspace: company") {
		t.Error("Output should contain workspace metadata")
	}
	if !strings.Contains(out, "- Channel: #general") {
		t.Error("Output should contain channel metadata")
	}
}

func TestMarkdownFormatter_FormatWithSourceErrors(t *testing.T) {
	events := testEvents()
	stats := testStatsWithErrors()

	f := NewMarkdownFormatter()
	out, err := f.Format(events, stats)
	if err != nil {
		t.Fatalf("Format() error: %v", err)
	}

	if !strings.Contains(out, "**Warning: Partial results due to source errors**") {
		t.Error("Output should contain partial results warning")
	}
	if !strings.Contains(out, "github: rate limited") {
		t.Error("Output should contain specific error message")
	}
}

func TestMarkdownFormatter_FormatEmpty(t *testing.T) {
	stats := NewFormatStats(nil, 0, nil)

	f := NewMarkdownFormatter()
	out, err := f.Format(nil, stats)
	if err != nil {
		t.Fatalf("Format() error: %v", err)
	}

	if !strings.Contains(out, "# Digest: 0 items") {
		t.Error("Output should contain zero items header")
	}
	if !strings.Contains(out, "No events found.") {
		t.Error("Output should contain empty state message")
	}
}

func TestMarkdownFormatter_NoFancyFormatting(t *testing.T) {
	events := testEvents()
	stats := testStats()

	f := NewMarkdownFormatter()
	out, err := f.Format(events, stats)
	if err != nil {
		t.Fatalf("Format() error: %v", err)
	}

	// No checkboxes
	if strings.Contains(out, "- [ ]") || strings.Contains(out, "- [x]") {
		t.Error("Output should not contain checkboxes")
	}

	// No box-drawing characters (Unicode box drawing block U+2500-U+257F)
	for r := rune(0x2500); r <= rune(0x257F); r++ {
		if strings.ContainsRune(out, r) {
			t.Errorf("Output should not contain box-drawing character: %c (U+%04X)", r, r)
		}
	}

	// No ANSI codes
	if strings.Contains(out, "\x1b[") || strings.Contains(out, "\033[") {
		t.Error("Output should not contain ANSI escape codes")
	}
}

func TestMarkdownFormatter_EventOrdering(t *testing.T) {
	events := testEvents()
	stats := testStats()

	f := NewMarkdownFormatter()
	out, err := f.Format(events, stats)
	if err != nil {
		t.Fatalf("Format() error: %v", err)
	}

	// RequiresAction section should come before ForAwareness
	requiresIdx := strings.Index(out, "## Requires Action")
	awarenessIdx := strings.Index(out, "## For Awareness")

	if requiresIdx == -1 || awarenessIdx == -1 {
		t.Fatal("Could not find both sections")
	}
	if requiresIdx > awarenessIdx {
		t.Error("Requires Action section should come before For Awareness")
	}
}

func TestEventTypeLabel(t *testing.T) {
	tests := []struct {
		eventType models.EventType
		want      string
	}{
		{models.EventTypePRReview, "PR Review"},
		{models.EventTypePRMention, "PR Mention"},
		{models.EventTypeIssueMention, "Issue Mention"},
		{models.EventTypeIssueAssigned, "Issue Assigned"},
		{models.EventTypeSlackDM, "Slack DM"},
		{models.EventTypeSlackMention, "Slack Mention"},
		{models.EventType("unknown"), "unknown"},
	}

	for _, tt := range tests {
		t.Run(string(tt.eventType), func(t *testing.T) {
			got := eventTypeLabel(tt.eventType)
			if got != tt.want {
				t.Errorf("eventTypeLabel(%s) = %q, want %q", tt.eventType, got, tt.want)
			}
		})
	}
}

func TestMarkdownFormatter_AllEventTypes(t *testing.T) {
	events := comprehensiveTestEvents()
	stats := NewFormatStats(events, 1*time.Second, nil)

	f := NewMarkdownFormatter()
	out, err := f.Format(events, stats)
	if err != nil {
		t.Fatalf("Format() error: %v", err)
	}

	// Verify all event type labels are present
	expectedLabels := []string{
		"[PR Review]",
		"[PR Mention]",
		"[Issue Mention]",
		"[Issue Assigned]",
		"[Slack DM]",
		"[Slack Mention]",
	}

	for _, label := range expectedLabels {
		if !strings.Contains(out, label) {
			t.Errorf("Output should contain event type label: %s", label)
		}
	}
}

func TestMarkdownFormatter_GitHubLabelsMetadata(t *testing.T) {
	events := comprehensiveTestEvents()
	stats := NewFormatStats(events, 1*time.Second, nil)

	f := NewMarkdownFormatter()
	out, err := f.Format(events, stats)
	if err != nil {
		t.Fatalf("Format() error: %v", err)
	}

	// Verify labels metadata is rendered
	if !strings.Contains(out, "- Labels: bug, help-wanted") {
		t.Error("Output should contain labels metadata")
	}
	if !strings.Contains(out, "- Labels: enhancement") {
		t.Error("Output should contain single label")
	}
}

func TestMarkdownFormatter_NoMetadata(t *testing.T) {
	now := time.Now()
	events := []models.Event{
		{
			Type:           models.EventTypeIssueMention,
			Title:          "Event with nil metadata",
			Source:         models.SourceGitHub,
			URL:            "https://github.com/org/repo/issues/1",
			Author:         models.Person{Username: "user"},
			Timestamp:      now,
			Priority:       models.PriorityHigh,
			RequiresAction: false,
			Metadata:       nil,
		},
		{
			Type:           models.EventTypeSlackDM,
			Title:          "Event with empty metadata",
			Source:         models.SourceSlack,
			URL:            "https://slack.com/archives/D123/p456",
			Author:         models.Person{Username: "user"},
			Timestamp:      now,
			Priority:       models.PriorityHigh,
			RequiresAction: false,
			Metadata:       map[string]any{},
		},
	}
	stats := NewFormatStats(events, 1*time.Second, nil)

	f := NewMarkdownFormatter()
	out, err := f.Format(events, stats)
	if err != nil {
		t.Fatalf("Format() error: %v", err)
	}

	// Should not crash with nil or empty metadata
	if !strings.Contains(out, "Event with nil metadata") {
		t.Error("Output should contain event with nil metadata")
	}
	if !strings.Contains(out, "Event with empty metadata") {
		t.Error("Output should contain event with empty metadata")
	}
}

func TestMarkdownFormatter_NoEmojis(t *testing.T) {
	events := comprehensiveTestEvents()
	stats := NewFormatStats(events, 1*time.Second, nil)

	f := NewMarkdownFormatter()
	out, err := f.Format(events, stats)
	if err != nil {
		t.Fatalf("Format() error: %v", err)
	}

	// Check for common emoji patterns
	emojiPatterns := []string{
		"🔥", "✅", "❌", "⚠️", "📝", "🚀",
		":fire:", ":check:", ":x:", ":warning:",
	}

	for _, emoji := range emojiPatterns {
		if strings.Contains(out, emoji) {
			t.Errorf("Output should not contain emoji: %s", emoji)
		}
	}
}

func TestMarkdownFormatter_LongTitle(t *testing.T) {
	events := comprehensiveTestEvents()
	stats := NewFormatStats(events, 1*time.Second, nil)

	f := NewMarkdownFormatter()
	out, err := f.Format(events, stats)
	if err != nil {
		t.Fatalf("Format() error: %v", err)
	}

	// Verify long title is preserved
	if !strings.Contains(out, "approaches the maximum allowed length") {
		t.Error("Long title should be preserved in markdown output")
	}
}
