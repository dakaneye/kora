package slack

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/dakaneye/kora/internal/datasources"
	"github.com/dakaneye/kora/internal/models"
)

// getWorkspaceFromPermalink extracts the workspace name from a Slack permalink.
// Input: "https://workspace.slack.com/archives/C12345/p1234567890"
// Output: "workspace"
//
// Returns empty string if the permalink is invalid or doesn't contain a workspace.
func getWorkspaceFromPermalink(permalink string) string {
	if permalink == "" {
		return ""
	}

	u, err := url.Parse(permalink)
	if err != nil {
		return ""
	}

	// Extract subdomain from host: "workspace.slack.com" -> "workspace"
	// Also handle "app.slack.com" which doesn't have a workspace subdomain
	parts := strings.Split(u.Host, ".")
	if len(parts) >= 3 && parts[0] != "app" {
		return parts[0]
	}

	return ""
}

// buildDMPermalink constructs a Slack app deep link for a DM message.
// Since DMs don't always have permalinks from the API, we construct one.
//
// Format: https://app.slack.com/client/TEAM/CHANNEL/p<ts>
// The timestamp is converted from "1234567890.123456" to "p1234567890123456"
//
// Note: This URL format requires the team ID, which we may not have.
// When team ID is not available, we return an empty string (valid per EFA 0001).
func buildDMPermalink(channelID, ts string) string {
	if channelID == "" || ts == "" {
		return ""
	}

	// For DMs without a team ID, return empty string (valid per EFA 0001).
	// The Slack API's conversations.history doesn't include team info,
	// and building an incorrect URL would be worse than no URL.
	// Users can still identify the message by timestamp and author.
	return ""
}

// deduplicateEvents removes duplicate events by URL.
// Same message can appear in multiple sources (e.g., a mention in a DM).
// Keeps the first occurrence (which may have higher priority).
func deduplicateEvents(events []models.Event) []models.Event {
	seen := make(map[string]bool)
	result := make([]models.Event, 0, len(events))

	for i := range events {
		// Use URL as primary dedup key
		key := events[i].URL
		if key == "" {
			// If no URL, use a composite key
			key = fmt.Sprintf("%s-%s-%d", events[i].Source, events[i].Type, events[i].Timestamp.UnixNano())
		}

		if !seen[key] {
			seen[key] = true
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
