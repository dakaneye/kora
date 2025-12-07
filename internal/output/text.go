package output

import (
	"fmt"
	"strings"

	"github.com/dakaneye/kora/internal/models"
)

// TextFormatter renders events as plain text.
// This is the simplest, most compact format - no markup, highly scannable.
type TextFormatter struct{}

// NewTextFormatter creates a TextFormatter.
func NewTextFormatter() *TextFormatter {
	return &TextFormatter{}
}

// Format renders events as plain text.
func (f *TextFormatter) Format(events []models.Event, stats *FormatStats) (string, error) {
	var b strings.Builder

	// Header
	b.WriteString(fmt.Sprintf("DIGEST: %d items", stats.TotalEvents))
	if stats.RequiresAction > 0 {
		b.WriteString(fmt.Sprintf(", %d require action", stats.RequiresAction))
	}
	b.WriteString("\n")

	// Source errors warning
	if len(stats.SourceErrors) > 0 {
		b.WriteString("\nWARNING: Partial results\n")
		for source, errMsg := range stats.SourceErrors {
			b.WriteString(fmt.Sprintf("  %s: %s\n", source, errMsg))
		}
	}

	// Group events
	action, awareness := groupByRequiresAction(events)

	// Requires Action section
	if len(action) > 0 {
		b.WriteString("\nREQUIRES ACTION:\n")
		f.writeEventGroup(&b, action)
	}

	// For Awareness section
	if len(awareness) > 0 {
		b.WriteString("\nFOR AWARENESS:\n")
		f.writeEventGroup(&b, awareness)
	}

	// Empty state
	if len(events) == 0 {
		b.WriteString("\nNo events found.\n")
	}

	return b.String(), nil
}

// writeEventGroup writes a group of events in compact text format.
func (f *TextFormatter) writeEventGroup(b *strings.Builder, events []models.Event) {
	sorted := sortByPriorityThenTime(events)

	for i := range sorted {
		e := &sorted[i]
		// Priority tag and event summary on one line
		priority := strings.ToUpper(priorityLabel(e.Priority))
		typeLabel := f.shortTypeLabel(e.Type)
		fmt.Fprintf(b, "[%s] %s: %s\n", priority, typeLabel, e.Title)

		// URL and metadata on second line, indented
		urlDisplay := f.shortenURL(e.URL)
		fmt.Fprintf(b, "       %s (@%s, %s)\n", urlDisplay, e.Author.Username, relativeTime(e.Timestamp))
	}
}

// shortTypeLabel returns a compact label for event types.
func (f *TextFormatter) shortTypeLabel(t models.EventType) string {
	switch t {
	case models.EventTypePRReview:
		return "PR Review"
	case models.EventTypePRMention:
		return "PR Mention"
	case models.EventTypeIssueMention:
		return "Issue Mention"
	case models.EventTypeIssueAssigned:
		return "Issue"
	default:
		return string(t)
	}
}

// shortenURL removes the protocol prefix for display.
func (f *TextFormatter) shortenURL(u string) string {
	u = strings.TrimPrefix(u, "https://")
	u = strings.TrimPrefix(u, "http://")
	return u
}
