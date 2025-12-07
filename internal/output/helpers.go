package output

import (
	"fmt"
	"sort"
	"time"

	"github.com/dakaneye/kora/internal/models"
)

// relativeTime formats a timestamp as a human-readable relative time.
// Examples: "2h ago", "1d ago", "3d ago", "just now".
func relativeTime(t time.Time) string {
	now := time.Now()
	d := now.Sub(t)

	if d < 0 {
		return "in the future"
	}

	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		mins := int(d.Minutes())
		if mins == 1 {
			return "1m ago"
		}
		return fmt.Sprintf("%dm ago", mins)
	case d < 24*time.Hour:
		hours := int(d.Hours())
		if hours == 1 {
			return "1h ago"
		}
		return fmt.Sprintf("%dh ago", hours)
	case d < 7*24*time.Hour:
		days := int(d.Hours() / 24)
		if days == 1 {
			return "1d ago"
		}
		return fmt.Sprintf("%dd ago", days)
	default:
		weeks := int(d.Hours() / (24 * 7))
		if weeks == 1 {
			return "1w ago"
		}
		return fmt.Sprintf("%dw ago", weeks)
	}
}

// priorityLabel returns a lowercase string label for a priority.
func priorityLabel(p models.Priority) string {
	switch p {
	case models.PriorityCritical:
		return "critical"
	case models.PriorityHigh:
		return "high"
	case models.PriorityMedium:
		return "medium"
	case models.PriorityLow:
		return "low"
	case models.PriorityInfo:
		return "info"
	default:
		return fmt.Sprintf("unknown(%d)", p)
	}
}

// groupByRequiresAction splits events into two slices:
// action (RequiresAction=true) and awareness (RequiresAction=false).
// Each group is sorted by priority then timestamp.
func groupByRequiresAction(events []models.Event) (action, awareness []models.Event) {
	for i := range events {
		if events[i].RequiresAction {
			action = append(action, events[i])
		} else {
			awareness = append(awareness, events[i])
		}
	}
	return action, awareness
}

// sortByPriorityThenTime sorts events by:
// 1. RequiresAction (true first)
// 2. Priority (1=Critical first, ascending)
// 3. Timestamp (most recent first)
//
// Returns a new slice; does not modify the input.
func sortByPriorityThenTime(events []models.Event) []models.Event {
	sorted := make([]models.Event, len(events))
	copy(sorted, events)

	sort.Slice(sorted, func(i, j int) bool {
		ei, ej := sorted[i], sorted[j]

		// RequiresAction first
		if ei.RequiresAction != ej.RequiresAction {
			return ei.RequiresAction
		}

		// Then by priority (lower number = higher priority)
		if ei.Priority != ej.Priority {
			return ei.Priority < ej.Priority
		}

		// Then by timestamp (most recent first)
		return ei.Timestamp.After(ej.Timestamp)
	})

	return sorted
}

// formatAuthor formats a Person for display.
// Uses Username, prefixed with @ for consistency.
func formatAuthor(p models.Person) string {
	return p.Username
}

// capitalizeFirst returns a string with the first letter capitalized.
// This is a simple replacement for strings.Title which is deprecated.
func capitalizeFirst(s string) string {
	if s == "" {
		return s
	}
	// For ASCII lowercase letters, convert to uppercase
	first := s[0]
	if first >= 'a' && first <= 'z' {
		return string(first-32) + s[1:]
	}
	return s
}
