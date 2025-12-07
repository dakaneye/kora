package output

import (
	"fmt"
	"strings"
	"time"

	"github.com/dakaneye/kora/internal/models"
)

// MarkdownFormatter renders events as clean, simple markdown.
// Output is readable without rendering and suitable for AI consumption.
// NO fancy formatting, NO emojis, NO checkboxes - plain headers and bullet points only.
type MarkdownFormatter struct{}

// NewMarkdownFormatter creates a MarkdownFormatter.
func NewMarkdownFormatter() *MarkdownFormatter {
	return &MarkdownFormatter{}
}

// Format renders events as markdown.
func (f *MarkdownFormatter) Format(events []models.Event, stats *FormatStats) (string, error) {
	var b strings.Builder

	// Header
	b.WriteString(fmt.Sprintf("# Digest: %d items", stats.TotalEvents))
	if stats.RequiresAction > 0 {
		b.WriteString(fmt.Sprintf(" (%d require action)", stats.RequiresAction))
	}
	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("Generated: %s\n", time.Now().UTC().Format("2006-01-02 15:04 UTC")))

	// Source errors warning
	if len(stats.SourceErrors) > 0 {
		b.WriteString("\n**Warning: Partial results due to source errors**\n")
		for source, errMsg := range stats.SourceErrors {
			b.WriteString(fmt.Sprintf("- %s: %s\n", source, errMsg))
		}
	}

	// Group events
	action, awareness := groupByRequiresAction(events)

	// Requires Action section
	if len(action) > 0 {
		b.WriteString(fmt.Sprintf("\n## Requires Action (%d)\n", len(action)))
		f.writeEventGroup(&b, action)
	}

	// For Awareness section
	if len(awareness) > 0 {
		b.WriteString(fmt.Sprintf("\n## For Awareness (%d)\n", len(awareness)))
		f.writeEventGroup(&b, awareness)
	}

	// Empty state
	if len(events) == 0 {
		b.WriteString("\nNo events found.\n")
	}

	return b.String(), nil
}

// writeEventGroup writes a group of events sorted by priority then time.
func (f *MarkdownFormatter) writeEventGroup(b *strings.Builder, events []models.Event) {
	sorted := sortByPriorityThenTime(events)

	for i := range sorted {
		e := &sorted[i]
		// Event header with type and title
		fmt.Fprintf(b, "\n### [%s] %s\n", eventTypeLabel(e.Type), e.Title)

		// Details as bullet points
		if e.URL != "" {
			fmt.Fprintf(b, "- URL: %s\n", e.URL)
		}
		fmt.Fprintf(b, "- Author: @%s\n", e.Author.Username)
		fmt.Fprintf(b, "- Time: %s\n", relativeTime(e.Timestamp))
		fmt.Fprintf(b, "- Priority: %s\n", capitalizeFirst(priorityLabel(e.Priority)))

		// Source-specific metadata
		f.writeMetadata(b, e)
	}
}

// writeMetadata writes relevant metadata as bullet points.
func (f *MarkdownFormatter) writeMetadata(b *strings.Builder, e *models.Event) {
	if e.Metadata == nil {
		return
	}

	switch e.Source {
	case models.SourceGitHub:
		if repo, ok := e.Metadata["repo"].(string); ok {
			fmt.Fprintf(b, "- Repo: %s\n", repo)
		}
		if state, ok := e.Metadata["state"].(string); ok {
			fmt.Fprintf(b, "- State: %s\n", state)
		}
		if labels, ok := e.Metadata["labels"].([]string); ok && len(labels) > 0 {
			fmt.Fprintf(b, "- Labels: %s\n", strings.Join(labels, ", "))
		}
	case models.SourceSlack:
		if workspace, ok := e.Metadata["workspace"].(string); ok {
			fmt.Fprintf(b, "- Workspace: %s\n", workspace)
		}
		if channel, ok := e.Metadata["channel"].(string); ok {
			fmt.Fprintf(b, "- Channel: #%s\n", channel)
		}
	}
}

// eventTypeLabel returns a human-readable label for event types.
func eventTypeLabel(t models.EventType) string {
	switch t {
	case models.EventTypePRReview:
		return "PR Review"
	case models.EventTypePRMention:
		return "PR Mention"
	case models.EventTypeIssueMention:
		return "Issue Mention"
	case models.EventTypeIssueAssigned:
		return "Issue Assigned"
	case models.EventTypeSlackDM:
		return "Slack DM"
	case models.EventTypeSlackMention:
		return "Slack Mention"
	default:
		return string(t)
	}
}
