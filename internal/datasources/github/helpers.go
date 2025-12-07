package github

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/dakaneye/kora/internal/datasources"
	"github.com/dakaneye/kora/internal/models"
)

// buildOrgFilter creates a GitHub search org filter string.
// Example: ["chainguard-dev", "sigstore"] -> "org:chainguard-dev org:sigstore"
func buildOrgFilter(orgs []string) string {
	if len(orgs) == 0 {
		return ""
	}
	parts := make([]string, len(orgs))
	for i, org := range orgs {
		parts[i] = fmt.Sprintf("org:%s", org)
	}
	return strings.Join(parts, " ")
}

// filterEvents applies FetchOptions filters to the event list.
// Filters supported:
//   - EventTypes: include only specified event types
//   - MinPriority: include only events with priority <= MinPriority
//   - RequiresAction: include only events that require action
func filterEvents(events []models.Event, opts datasources.FetchOptions) []models.Event {
	if opts.Filter == nil {
		return events
	}

	result := make([]models.Event, 0, len(events))
	for i := range events {
		// Filter by event types
		if len(opts.Filter.EventTypes) > 0 {
			found := false
			for _, t := range opts.Filter.EventTypes {
				if events[i].Type == t {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		// Filter by priority (MinPriority is the lowest priority to include)
		// Remember: priority 1 is highest, 5 is lowest
		if opts.Filter.MinPriority > 0 && events[i].Priority > opts.Filter.MinPriority {
			continue
		}

		// Filter by requires action
		if opts.Filter.RequiresAction && !events[i].RequiresAction {
			continue
		}

		result = append(result, events[i])
	}
	return result
}

// truncateTitle truncates a title to 100 characters per EFA 0001.
// Adds "..." suffix if truncated. Handles UTF-8 safely by counting runes.
// Also removes newlines for single-line display.
func truncateTitle(title string) string {
	// Remove newlines for single-line title
	title = strings.ReplaceAll(title, "\n", " ")
	title = strings.TrimSpace(title)

	if utf8.RuneCountInString(title) <= 100 {
		return title
	}
	runes := []rune(title)
	return string(runes[:97]) + "..."
}
