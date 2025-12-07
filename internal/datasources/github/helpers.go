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

// deduplicateEvents removes duplicate events by URL, merging user_relationships.
// Same PR/issue can appear in multiple searches (e.g., author AND reviewer).
// Keeps the event with highest priority (lowest number) and merges user_relationships.
//
// Per EFA 0001, user_relationships can contain multiple values when user has
// multiple relationships to the same item (e.g., ["author", "reviewer"]).
func deduplicateEvents(events []models.Event) []models.Event {
	// Map URL to index in result slice and the event
	seen := make(map[string]int)
	result := make([]models.Event, 0, len(events))

	for i := range events {
		url := events[i].URL
		if idx, exists := seen[url]; exists {
			// Merge user_relationships from both events
			result[idx] = mergeEventRelationships(result[idx], events[i])
		} else {
			seen[url] = len(result)
			result = append(result, events[i])
		}
	}
	return result
}

// mergeEventRelationships merges user_relationships from two events.
// Keeps the event with higher priority (lower number) and combines relationships.
//
//nolint:gocritic // hugeParam: intentionally passing by value to avoid mutation
func mergeEventRelationships(existing, incoming models.Event) models.Event {
	// Determine which event to keep based on priority
	// Lower priority number = higher priority
	keepExisting := existing.Priority <= incoming.Priority

	var primary, secondary models.Event
	if keepExisting {
		primary = existing
		secondary = incoming
	} else {
		primary = incoming
		secondary = existing
	}

	// Extract relationships from both events
	primaryRels := extractRelationships(primary.Metadata)
	secondaryRels := extractRelationships(secondary.Metadata)

	// Merge relationships (deduplicate)
	relSet := make(map[string]struct{})
	for _, r := range primaryRels {
		relSet[r] = struct{}{}
	}
	for _, r := range secondaryRels {
		relSet[r] = struct{}{}
	}

	// Convert back to slice
	merged := make([]string, 0, len(relSet))
	for r := range relSet {
		merged = append(merged, r)
	}

	// Sort for deterministic output
	sortStrings(merged)

	// Update metadata with merged relationships
	if primary.Metadata == nil {
		primary.Metadata = make(map[string]any)
	}
	primary.Metadata["user_relationships"] = merged

	return primary
}

// extractRelationships extracts user_relationships from event metadata.
func extractRelationships(metadata map[string]any) []string {
	if metadata == nil {
		return nil
	}
	rels, ok := metadata["user_relationships"].([]string)
	if ok {
		return rels
	}
	return nil
}

// sortStrings sorts a slice of strings in place (simple bubble sort for small slices).
func sortStrings(s []string) {
	for i := 0; i < len(s); i++ {
		for j := i + 1; j < len(s); j++ {
			if s[i] > s[j] {
				s[i], s[j] = s[j], s[i]
			}
		}
	}
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
