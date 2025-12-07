package output

import (
	"strings"
	"testing"
	"time"

	"github.com/dakaneye/kora/internal/models"
)

func TestTextFormatter_Format(t *testing.T) {
	events := testEvents()
	stats := testStats()

	f := NewTextFormatter()
	out, err := f.Format(events, stats)
	if err != nil {
		t.Fatalf("Format() error: %v", err)
	}

	// Verify header
	if !strings.Contains(out, "DIGEST: 4 items") {
		t.Error("Output should contain digest header with count")
	}
	if !strings.Contains(out, "2 require action") {
		t.Error("Output should contain require action count")
	}

	// Verify sections
	if !strings.Contains(out, "REQUIRES ACTION:") {
		t.Error("Output should contain 'REQUIRES ACTION' section")
	}
	if !strings.Contains(out, "FOR AWARENESS:") {
		t.Error("Output should contain 'FOR AWARENESS' section")
	}
}

func TestTextFormatter_FormatEventLines(t *testing.T) {
	events := testEvents()
	stats := testStats()

	f := NewTextFormatter()
	out, err := f.Format(events, stats)
	if err != nil {
		t.Fatalf("Format() error: %v", err)
	}

	// Verify event format: [PRIORITY] Type: Title
	if !strings.Contains(out, "[HIGH] PR Review: Review requested: Add feature X") {
		t.Error("Output should contain formatted PR review event")
	}

	// Verify URL line (indented, without protocol)
	if !strings.Contains(out, "github.com/org/repo/pull/123") {
		t.Error("Output should contain shortened URL")
	}
	// Should not have https://
	if strings.Contains(out, "https://github.com") {
		t.Error("Output should not contain https:// prefix in URL")
	}

	// Verify author in parentheses
	if !strings.Contains(out, "(@janedev,") {
		t.Error("Output should contain author with @ prefix")
	}
}

func TestTextFormatter_FormatWithSourceErrors(t *testing.T) {
	events := testEvents()
	stats := testStatsWithErrors()

	f := NewTextFormatter()
	out, err := f.Format(events, stats)
	if err != nil {
		t.Fatalf("Format() error: %v", err)
	}

	if !strings.Contains(out, "WARNING: Partial results") {
		t.Error("Output should contain partial results warning")
	}
	if !strings.Contains(out, "github: rate limited") {
		t.Error("Output should contain specific error message")
	}
}

func TestTextFormatter_FormatEmpty(t *testing.T) {
	stats := NewFormatStats(nil, 0, nil)

	f := NewTextFormatter()
	out, err := f.Format(nil, stats)
	if err != nil {
		t.Fatalf("Format() error: %v", err)
	}

	if !strings.Contains(out, "DIGEST: 0 items") {
		t.Error("Output should contain zero items header")
	}
	if !strings.Contains(out, "No events found.") {
		t.Error("Output should contain empty state message")
	}
}

func TestTextFormatter_NoFancyFormatting(t *testing.T) {
	events := testEvents()
	stats := testStats()

	f := NewTextFormatter()
	out, err := f.Format(events, stats)
	if err != nil {
		t.Fatalf("Format() error: %v", err)
	}

	// No markdown
	if strings.Contains(out, "###") || strings.Contains(out, "**") || strings.Contains(out, "- [") {
		t.Error("Output should not contain markdown formatting")
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

func TestTextFormatter_CompactFormat(t *testing.T) {
	events := testEvents()
	stats := testStats()

	f := NewTextFormatter()
	out, err := f.Format(events, stats)
	if err != nil {
		t.Fatalf("Format() error: %v", err)
	}

	// Count lines - should be reasonably compact
	lines := strings.Split(strings.TrimSpace(out), "\n")

	// Header (1) + blank + REQUIRES ACTION (1) + events*2 + blank + FOR AWARENESS (1) + events*2
	// = 1 + 1 + 1 + 4 + 1 + 1 + 4 = 13 lines approximately
	if len(lines) > 20 {
		t.Errorf("Output has too many lines (%d), should be compact", len(lines))
	}
}

func TestTextFormatter_ShortenURL(t *testing.T) {
	f := NewTextFormatter()

	tests := []struct {
		input string
		want  string
	}{
		{"https://github.com/org/repo", "github.com/org/repo"},
		{"http://example.com/path", "example.com/path"},
		{"ftp://server.com/file", "ftp://server.com/file"}, // Non-http not stripped
		{"github.com/org/repo", "github.com/org/repo"},     // Already stripped
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := f.shortenURL(tt.input)
			if got != tt.want {
				t.Errorf("shortenURL(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestTextFormatter_ShortTypeLabel(t *testing.T) {
	f := NewTextFormatter()

	tests := []struct {
		eventType models.EventType
		want      string
	}{
		{models.EventTypePRReview, "PR Review"},
		{models.EventTypePRMention, "PR Mention"},
		{models.EventTypeIssueMention, "Issue Mention"},
		{models.EventTypeIssueAssigned, "Issue"},
		{models.EventType("unknown"), "unknown"},
	}

	for _, tt := range tests {
		t.Run(string(tt.eventType), func(t *testing.T) {
			got := f.shortTypeLabel(tt.eventType)
			if got != tt.want {
				t.Errorf("shortTypeLabel(%s) = %q, want %q", tt.eventType, got, tt.want)
			}
		})
	}
}

func TestTextFormatter_AllEventTypes(t *testing.T) {
	events := comprehensiveTestEvents()
	stats := NewFormatStats(events, 1*time.Second, nil)

	f := NewTextFormatter()
	out, err := f.Format(events, stats)
	if err != nil {
		t.Fatalf("Format() error: %v", err)
	}

	// Verify all event type labels are present
	expectedLabels := []string{
		"PR Review:",
		"PR Mention:",
		"Issue Mention:",
		"Issue:",
	}

	for _, label := range expectedLabels {
		if !strings.Contains(out, label) {
			t.Errorf("Output should contain event type label: %s", label)
		}
	}
}

func TestTextFormatter_PriorityLabels(t *testing.T) {
	events := comprehensiveTestEvents()
	stats := NewFormatStats(events, 1*time.Second, nil)

	f := NewTextFormatter()
	out, err := f.Format(events, stats)
	if err != nil {
		t.Fatalf("Format() error: %v", err)
	}

	// Verify priority labels are in brackets and uppercase
	priorityLabels := []string{
		"[CRITICAL]",
		"[HIGH]",
		"[MEDIUM]",
		"[LOW]",
		"[INFO]",
	}

	for _, label := range priorityLabels {
		if !strings.Contains(out, label) {
			t.Errorf("Output should contain priority label: %s", label)
		}
	}
}

func TestTextFormatter_URLShortening(t *testing.T) {
	events := comprehensiveTestEvents()
	stats := NewFormatStats(events, 1*time.Second, nil)

	f := NewTextFormatter()
	out, err := f.Format(events, stats)
	if err != nil {
		t.Fatalf("Format() error: %v", err)
	}

	// Verify URLs are shortened (no https://)
	if strings.Contains(out, "https://github.com") {
		t.Error("Output should not contain https:// prefix")
	}

	// Verify shortened URLs are present
	if !strings.Contains(out, "github.com/org/repo") {
		t.Error("Output should contain shortened GitHub URLs")
	}
}

func TestTextFormatter_LongTitle(t *testing.T) {
	events := comprehensiveTestEvents()
	stats := NewFormatStats(events, 1*time.Second, nil)

	f := NewTextFormatter()
	out, err := f.Format(events, stats)
	if err != nil {
		t.Fatalf("Format() error: %v", err)
	}

	// Verify long title is preserved
	if !strings.Contains(out, "approaches the maximum allowed length") {
		t.Error("Long title should be preserved in text output")
	}
}

func TestTextFormatter_NoEmojis(t *testing.T) {
	events := comprehensiveTestEvents()
	stats := NewFormatStats(events, 1*time.Second, nil)

	f := NewTextFormatter()
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

func TestTextFormatter_RelativeTime(t *testing.T) {
	events := comprehensiveTestEvents()
	stats := NewFormatStats(events, 1*time.Second, nil)

	f := NewTextFormatter()
	out, err := f.Format(events, stats)
	if err != nil {
		t.Fatalf("Format() error: %v", err)
	}

	// Verify relative time strings are present
	relativeTimePatterns := []string{
		"ago",
	}

	hasRelativeTime := false
	for _, pattern := range relativeTimePatterns {
		if strings.Contains(out, pattern) {
			hasRelativeTime = true
			break
		}
	}

	if !hasRelativeTime {
		t.Error("Output should contain relative time information")
	}
}
