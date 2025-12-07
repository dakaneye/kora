package output

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/dakaneye/kora/internal/models"
)

func TestJSONFormatter_Format(t *testing.T) {
	events := testEvents()
	stats := testStats()

	f := NewJSONFormatter(false)
	out, err := f.Format(events, stats)
	if err != nil {
		t.Fatalf("Format() error: %v", err)
	}

	// Verify it's valid JSON
	var parsed jsonOutput
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("Output is not valid JSON: %v\nOutput: %s", err, out)
	}

	// Verify stats
	if parsed.Stats.Total != 4 {
		t.Errorf("Stats.Total = %d, want 4", parsed.Stats.Total)
	}
	if parsed.Stats.RequiresAction != 2 {
		t.Errorf("Stats.RequiresAction = %d, want 2", parsed.Stats.RequiresAction)
	}
	if parsed.Stats.ByPriority["high"] != 2 {
		t.Errorf("Stats.ByPriority[high] = %d, want 2", parsed.Stats.ByPriority["high"])
	}
	if parsed.Stats.ByPriority["medium"] != 1 {
		t.Errorf("Stats.ByPriority[medium] = %d, want 1", parsed.Stats.ByPriority["medium"])
	}
	if parsed.Stats.PartialSuccess {
		t.Error("Stats.PartialSuccess should be false")
	}

	// Verify events are sorted (RequiresAction first, then by priority)
	if len(parsed.Events) != 4 {
		t.Fatalf("len(Events) = %d, want 4", len(parsed.Events))
	}
	// First two should be RequiresAction=true
	if !parsed.Events[0].RequiresAction {
		t.Error("Events[0] should require action")
	}
	if !parsed.Events[1].RequiresAction {
		t.Error("Events[1] should require action")
	}
	// Last two should be RequiresAction=false
	if parsed.Events[2].RequiresAction {
		t.Error("Events[2] should not require action")
	}
	if parsed.Events[3].RequiresAction {
		t.Error("Events[3] should not require action")
	}
}

func TestJSONFormatter_FormatPretty(t *testing.T) {
	events := testEvents()
	stats := testStats()

	f := NewJSONFormatter(true)
	out, err := f.Format(events, stats)
	if err != nil {
		t.Fatalf("Format() error: %v", err)
	}

	// Pretty format should have newlines and indentation
	if !strings.Contains(out, "\n") {
		t.Error("Pretty JSON should contain newlines")
	}
	if !strings.Contains(out, "  ") {
		t.Error("Pretty JSON should contain indentation")
	}
}

func TestJSONFormatter_FormatCompact(t *testing.T) {
	events := testEvents()
	stats := testStats()

	f := NewJSONFormatter(false)
	out, err := f.Format(events, stats)
	if err != nil {
		t.Fatalf("Format() error: %v", err)
	}

	// Compact format should be a single line
	lines := strings.Split(out, "\n")
	if len(lines) != 1 {
		t.Errorf("Compact JSON should be single line, got %d lines", len(lines))
	}
}

func TestJSONFormatter_FormatWithSourceErrors(t *testing.T) {
	events := testEvents()
	stats := testStatsWithErrors()

	f := NewJSONFormatter(false)
	out, err := f.Format(events, stats)
	if err != nil {
		t.Fatalf("Format() error: %v", err)
	}

	var parsed jsonOutput
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("Output is not valid JSON: %v", err)
	}

	if !parsed.Stats.PartialSuccess {
		t.Error("Stats.PartialSuccess should be true when there are source errors")
	}
	if parsed.Stats.SourceErrors["github"] != "rate limited" {
		t.Errorf("Stats.SourceErrors[github] = %q, want 'rate limited'", parsed.Stats.SourceErrors["github"])
	}
}

func TestJSONFormatter_FormatEmpty(t *testing.T) {
	stats := NewFormatStats(nil, 0, nil)

	f := NewJSONFormatter(false)
	out, err := f.Format(nil, stats)
	if err != nil {
		t.Fatalf("Format() error: %v", err)
	}

	var parsed jsonOutput
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("Output is not valid JSON: %v", err)
	}

	if parsed.Stats.Total != 0 {
		t.Errorf("Stats.Total = %d, want 0", parsed.Stats.Total)
	}
	if len(parsed.Events) != 0 {
		t.Errorf("len(Events) = %d, want 0", len(parsed.Events))
	}
}

func TestJSONFormatter_NoANSICodes(t *testing.T) {
	events := testEvents()
	stats := testStats()

	f := NewJSONFormatter(true) // Use pretty for more output
	out, err := f.Format(events, stats)
	if err != nil {
		t.Fatalf("Format() error: %v", err)
	}

	// Check for ANSI escape sequences
	if strings.Contains(out, "\x1b[") || strings.Contains(out, "\033[") {
		t.Error("JSON output should not contain ANSI escape codes")
	}
}

func TestJSONFormatter_EventFields(t *testing.T) {
	events := testEvents()
	stats := testStats()

	f := NewJSONFormatter(false)
	out, err := f.Format(events, stats)
	if err != nil {
		t.Fatalf("Format() error: %v", err)
	}

	var parsed jsonOutput
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("Output is not valid JSON: %v", err)
	}

	// Find the PR review event and verify fields
	var prEvent *jsonEvent
	for i := range parsed.Events {
		if parsed.Events[i].Type == string(models.EventTypePRReview) {
			prEvent = &parsed.Events[i]
			break
		}
	}
	if prEvent == nil {
		t.Fatal("Could not find PR review event")
	}

	if prEvent.Title != "Review requested: Add feature X" {
		t.Errorf("Title = %q, want 'Review requested: Add feature X'", prEvent.Title)
	}
	if prEvent.Source != "github" {
		t.Errorf("Source = %q, want 'github'", prEvent.Source)
	}
	if prEvent.URL != "https://github.com/org/repo/pull/123" {
		t.Errorf("URL = %q, want 'https://github.com/org/repo/pull/123'", prEvent.URL)
	}
	if prEvent.Author != "janedev" {
		t.Errorf("Author = %q, want 'janedev'", prEvent.Author)
	}
	if prEvent.Priority != "high" {
		t.Errorf("Priority = %q, want 'high'", prEvent.Priority)
	}
	if !prEvent.RequiresAction {
		t.Error("RequiresAction should be true")
	}
	if prEvent.Metadata["repo"] != "org/repo" {
		t.Errorf("Metadata[repo] = %v, want 'org/repo'", prEvent.Metadata["repo"])
	}
}

func TestJSONFormatter_GeneratedAtRFC3339(t *testing.T) {
	events := testEvents()
	stats := testStats()

	f := NewJSONFormatter(false)
	out, err := f.Format(events, stats)
	if err != nil {
		t.Fatalf("Format() error: %v", err)
	}

	var parsed jsonOutput
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("Output is not valid JSON: %v", err)
	}

	// Verify generated_at is present and valid RFC3339
	if parsed.GeneratedAt.IsZero() {
		t.Error("GeneratedAt should not be zero")
	}

	// Verify it can be marshaled and parsed as RFC3339
	jsonBytes, _ := json.Marshal(parsed.GeneratedAt)
	var parsedTime time.Time
	if err := json.Unmarshal(jsonBytes, &parsedTime); err != nil {
		t.Errorf("GeneratedAt is not valid RFC3339: %v", err)
	}
}

func TestJSONFormatter_AllEventTypes(t *testing.T) {
	events := comprehensiveTestEvents()
	stats := NewFormatStats(events, 1*time.Second, nil)

	f := NewJSONFormatter(false)
	out, err := f.Format(events, stats)
	if err != nil {
		t.Fatalf("Format() error: %v", err)
	}

	var parsed jsonOutput
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("Output is not valid JSON: %v", err)
	}

	// Verify all event types are present
	eventTypes := make(map[string]bool)
	for _, e := range parsed.Events {
		eventTypes[e.Type] = true
	}

	expectedTypes := []string{
		string(models.EventTypePRReview),
		string(models.EventTypePRMention),
		string(models.EventTypeIssueMention),
		string(models.EventTypeIssueAssigned),
		string(models.EventTypeSlackDM),
		string(models.EventTypeSlackMention),
	}

	for _, expected := range expectedTypes {
		if !eventTypes[expected] {
			t.Errorf("Missing event type: %s", expected)
		}
	}
}

func TestJSONFormatter_AllPriorities(t *testing.T) {
	events := comprehensiveTestEvents()
	stats := NewFormatStats(events, 1*time.Second, nil)

	f := NewJSONFormatter(false)
	out, err := f.Format(events, stats)
	if err != nil {
		t.Fatalf("Format() error: %v", err)
	}

	var parsed jsonOutput
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("Output is not valid JSON: %v", err)
	}

	// Verify all priority counts are in stats
	if parsed.Stats.ByPriority["critical"] != 1 {
		t.Errorf("ByPriority[critical] = %d, want 1", parsed.Stats.ByPriority["critical"])
	}
	if parsed.Stats.ByPriority["high"] != 2 {
		t.Errorf("ByPriority[high] = %d, want 2", parsed.Stats.ByPriority["high"])
	}
	if parsed.Stats.ByPriority["medium"] != 2 {
		t.Errorf("ByPriority[medium] = %d, want 2", parsed.Stats.ByPriority["medium"])
	}
	if parsed.Stats.ByPriority["low"] != 1 {
		t.Errorf("ByPriority[low] = %d, want 1", parsed.Stats.ByPriority["low"])
	}
	if parsed.Stats.ByPriority["info"] != 2 {
		t.Errorf("ByPriority[info] = %d, want 2", parsed.Stats.ByPriority["info"])
	}
}

func TestJSONFormatter_LongTitles(t *testing.T) {
	events := comprehensiveTestEvents()
	stats := NewFormatStats(events, 1*time.Second, nil)

	f := NewJSONFormatter(false)
	out, err := f.Format(events, stats)
	if err != nil {
		t.Fatalf("Format() error: %v", err)
	}

	// Verify long title is preserved
	if !strings.Contains(out, "approaches the maximum allowed length") {
		t.Error("Long title should be preserved in JSON output")
	}
}

func TestJSONFormatter_SortingCorrectness(t *testing.T) {
	events := comprehensiveTestEvents()
	stats := NewFormatStats(events, 1*time.Second, nil)

	f := NewJSONFormatter(false)
	out, err := f.Format(events, stats)
	if err != nil {
		t.Fatalf("Format() error: %v", err)
	}

	var parsed jsonOutput
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("Output is not valid JSON: %v", err)
	}

	// First group: RequiresAction=true, sorted by priority
	actionCount := 0
	for i, e := range parsed.Events {
		if !e.RequiresAction {
			// Found first non-action event
			actionCount = i
			break
		}
	}

	if actionCount == 0 {
		t.Fatal("Should have some RequiresAction events first")
	}

	// Verify first event is Critical priority (1)
	if parsed.Events[0].Priority != "critical" {
		t.Errorf("First event should be critical, got %s", parsed.Events[0].Priority)
	}
}
