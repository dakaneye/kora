// Package models defines the core Event type for Kora.
// Ground truth defined in specs/efas/0001-event-model.md
package models

// DeduplicateEvents merges events with the same URL by combining user_relationships.
// When duplicates exist:
//   - Pick the event with highest priority (lowest number) as primary
//   - Merge user_relationships from all duplicates
//   - Keep metadata from primary event
//
// Ground truth: EFA 0001 defines user_relationships as []string in metadata.
func DeduplicateEvents(events []Event) []Event {
	if len(events) == 0 {
		return events
	}

	// Group by URL while preserving first occurrence order
	groups := make(map[string][]int) // URL -> indices into events slice
	var order []string
	for i := range events {
		if _, exists := groups[events[i].URL]; !exists {
			order = append(order, events[i].URL)
		}
		groups[events[i].URL] = append(groups[events[i].URL], i)
	}

	result := make([]Event, 0, len(order))
	for _, url := range order {
		indices := groups[url]
		if len(indices) == 1 {
			result = append(result, events[indices[0]])
			continue
		}

		merged := mergeEventGroup(events, indices)
		result = append(result, merged)
	}

	return result
}

// mergeEventGroup merges a group of events with the same URL.
// Selects the highest priority event as primary and combines all user_relationships.
func mergeEventGroup(events []Event, indices []int) Event {
	// Find primary event (highest priority = lowest number)
	primaryIdx := indices[0]
	for _, idx := range indices[1:] {
		if events[idx].Priority < events[primaryIdx].Priority {
			primaryIdx = idx
		}
	}

	// Collect all unique user_relationships from all events
	seen := make(map[string]struct{})
	var allRelationships []string
	for _, idx := range indices {
		rels := extractUserRelationships(&events[idx])
		for _, r := range rels {
			if _, exists := seen[r]; !exists {
				seen[r] = struct{}{}
				allRelationships = append(allRelationships, r)
			}
		}
	}

	// Create merged event with updated metadata
	merged := events[primaryIdx]
	if len(allRelationships) > 0 {
		if merged.Metadata == nil {
			merged.Metadata = make(map[string]any)
		}
		merged.Metadata["user_relationships"] = allRelationships
	}

	return merged
}

// extractUserRelationships extracts the user_relationships slice from event metadata.
// Handles nil metadata, missing key, and type conversion from []any to []string.
func extractUserRelationships(e *Event) []string {
	if e.Metadata == nil {
		return nil
	}

	rels, ok := e.Metadata["user_relationships"]
	if !ok {
		return nil
	}

	switch v := rels.(type) {
	case []string:
		return v
	case []any:
		// Handle JSON unmarshaling which produces []any
		result := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				result = append(result, s)
			}
		}
		return result
	default:
		return nil
	}
}
