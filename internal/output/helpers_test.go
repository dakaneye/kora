package output

import (
	"testing"
	"time"

	"github.com/dakaneye/kora/internal/models"
)

func TestRelativeTime(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name string
		t    time.Time
		want string
	}{
		{"just now", now.Add(-30 * time.Second), "just now"},
		{"1 minute ago", now.Add(-1 * time.Minute), "1m ago"},
		{"5 minutes ago", now.Add(-5 * time.Minute), "5m ago"},
		{"59 minutes ago", now.Add(-59 * time.Minute), "59m ago"},
		{"1 hour ago", now.Add(-1 * time.Hour), "1h ago"},
		{"2 hours ago", now.Add(-2 * time.Hour), "2h ago"},
		{"23 hours ago", now.Add(-23 * time.Hour), "23h ago"},
		{"1 day ago", now.Add(-24 * time.Hour), "1d ago"},
		{"2 days ago", now.Add(-48 * time.Hour), "2d ago"},
		{"6 days ago", now.Add(-6 * 24 * time.Hour), "6d ago"},
		{"1 week ago", now.Add(-7 * 24 * time.Hour), "1w ago"},
		{"2 weeks ago", now.Add(-14 * 24 * time.Hour), "2w ago"},
		{"future time", now.Add(1 * time.Hour), "in the future"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := relativeTime(tt.t)
			if got != tt.want {
				t.Errorf("relativeTime() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestPriorityLabel(t *testing.T) {
	//nolint:govet // Field order prioritizes readability over memory alignment
	tests := []struct {
		priority models.Priority
		want     string
	}{
		{models.PriorityCritical, "critical"},
		{models.PriorityHigh, "high"},
		{models.PriorityMedium, "medium"},
		{models.PriorityLow, "low"},
		{models.PriorityInfo, "info"},
		{models.Priority(0), "unknown(0)"},
		{models.Priority(99), "unknown(99)"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := priorityLabel(tt.priority)
			if got != tt.want {
				t.Errorf("priorityLabel(%d) = %q, want %q", tt.priority, got, tt.want)
			}
		})
	}
}

func TestGroupByRequiresAction(t *testing.T) {
	events := []models.Event{
		{Title: "A", RequiresAction: true},
		{Title: "B", RequiresAction: false},
		{Title: "C", RequiresAction: true},
		{Title: "D", RequiresAction: false},
		{Title: "E", RequiresAction: true},
	}

	action, awareness := groupByRequiresAction(events)

	if len(action) != 3 {
		t.Errorf("len(action) = %d, want 3", len(action))
	}
	if len(awareness) != 2 {
		t.Errorf("len(awareness) = %d, want 2", len(awareness))
	}

	// Verify all action items have RequiresAction=true
	for _, e := range action {
		if !e.RequiresAction {
			t.Errorf("Action event %q should have RequiresAction=true", e.Title)
		}
	}

	// Verify all awareness items have RequiresAction=false
	for _, e := range awareness {
		if e.RequiresAction {
			t.Errorf("Awareness event %q should have RequiresAction=false", e.Title)
		}
	}
}

func TestGroupByRequiresActionEmpty(t *testing.T) {
	action, awareness := groupByRequiresAction(nil)

	if len(action) != 0 {
		t.Errorf("len(action) = %d, want 0", len(action))
	}
	if len(awareness) != 0 {
		t.Errorf("len(awareness) = %d, want 0", len(awareness))
	}
}

func TestSortByPriorityThenTime(t *testing.T) {
	now := time.Now()

	events := []models.Event{
		{Title: "Low priority old", Priority: models.PriorityLow, Timestamp: now.Add(-3 * time.Hour), RequiresAction: false},
		{Title: "High priority new", Priority: models.PriorityHigh, Timestamp: now.Add(-1 * time.Hour), RequiresAction: false},
		{Title: "High priority old", Priority: models.PriorityHigh, Timestamp: now.Add(-2 * time.Hour), RequiresAction: false},
		{Title: "Critical requires action", Priority: models.PriorityCritical, Timestamp: now, RequiresAction: true},
		{Title: "Low requires action", Priority: models.PriorityLow, Timestamp: now, RequiresAction: true},
	}

	sorted := sortByPriorityThenTime(events)

	// Verify RequiresAction comes first
	if !sorted[0].RequiresAction || !sorted[1].RequiresAction {
		t.Error("RequiresAction events should come first")
	}
	if sorted[2].RequiresAction || sorted[3].RequiresAction || sorted[4].RequiresAction {
		t.Error("Non-RequiresAction events should come after")
	}

	// Within RequiresAction, Critical (1) should come before Low (4)
	if sorted[0].Priority != models.PriorityCritical {
		t.Errorf("First event should be Critical, got priority %d", sorted[0].Priority)
	}
	if sorted[1].Priority != models.PriorityLow {
		t.Errorf("Second event should be Low, got priority %d", sorted[1].Priority)
	}

	// Within non-RequiresAction, High (2) should come before Low (4)
	if sorted[2].Priority != models.PriorityHigh {
		t.Errorf("Third event should be High, got priority %d", sorted[2].Priority)
	}
	if sorted[3].Priority != models.PriorityHigh {
		t.Errorf("Fourth event should be High, got priority %d", sorted[3].Priority)
	}

	// For same priority, newer should come first (sorted[2] should be newer than sorted[3])
	if !sorted[2].Timestamp.After(sorted[3].Timestamp) {
		t.Error("Among same priority events, newer should come first")
	}

	if sorted[4].Priority != models.PriorityLow {
		t.Errorf("Fifth event should be Low, got priority %d", sorted[4].Priority)
	}
}

func TestSortByPriorityThenTimeDoesNotModifyInput(t *testing.T) {
	now := time.Now()

	original := []models.Event{
		{Title: "B", Priority: models.PriorityHigh, Timestamp: now},
		{Title: "A", Priority: models.PriorityCritical, Timestamp: now},
	}

	originalFirstTitle := original[0].Title
	_ = sortByPriorityThenTime(original)

	if original[0].Title != originalFirstTitle {
		t.Error("sortByPriorityThenTime should not modify the input slice")
	}
}

func TestFormatAuthor(t *testing.T) {
	tests := []struct {
		name   string
		person models.Person
		want   string
	}{
		{
			name:   "username only",
			person: models.Person{Username: "alice"},
			want:   "alice",
		},
		{
			name:   "with display name",
			person: models.Person{Name: "Alice Smith", Username: "alice"},
			want:   "alice",
		},
		{
			name:   "slack user id",
			person: models.Person{Username: "U12345678"},
			want:   "U12345678",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatAuthor(tt.person)
			if got != tt.want {
				t.Errorf("formatAuthor() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCapitalizeFirst(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"hello", "Hello"},
		{"world", "World"},
		{"a", "A"},
		{"", ""},
		{"Hello", "Hello"},       // Already capitalized
		{"1abc", "1abc"},         // Starts with number
		{"UPPER", "UPPER"},       // All caps
		{"critical", "Critical"}, // Priority label
		{"high", "High"},         // Priority label
		{"médium", "Médium"},     // Non-ASCII first char
		{"123", "123"},           // Only numbers
		{" space", " space"},     // Starts with space
		{"!exclaim", "!exclaim"}, // Special char
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := capitalizeFirst(tt.input)
			if got != tt.want {
				t.Errorf("capitalizeFirst(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
