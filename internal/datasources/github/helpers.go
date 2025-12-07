package github

import (
	"fmt"
	"strings"

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

// deduplicateEvents removes duplicate events by URL.
// Same PR/issue can appear in multiple searches (e.g., mentioned AND assigned).
// Keeps the first occurrence (which is typically higher priority).
func deduplicateEvents(events []models.Event) []models.Event {
	seen := make(map[string]bool)
	result := make([]models.Event, 0, len(events))
	for i := range events {
		if !seen[events[i].URL] {
			seen[events[i].URL] = true
			result = append(result, events[i])
		}
	}
	return result
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
// Adds "..." suffix if truncated.
func truncateTitle(title string) string {
	if len(title) <= 100 {
		return title
	}
	return title[:97] + "..."
}
