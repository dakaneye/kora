package slack

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"time"

	"github.com/dakaneye/kora/internal/models"
)

// fetchMentions searches for @mentions of the user.
// Uses search.messages API with query: <@USER_ID>
//
// Per EFA 0001:
//   - EventType: models.EventTypeSlackMention
//   - Priority: models.PriorityMedium (3)
//   - RequiresAction: false
//
// Per EFA 0002:
//   - Token is passed to apiRequest which handles it securely
//
// Returns:
//   - events: the fetched mention events
//   - apiCalls: number of API calls made
//   - err: any error encountered
func (d *DataSource) fetchMentions(ctx context.Context, token string, since time.Time) ([]models.Event, int, error) {
	// Build search query for user mentions
	// Format: <@USER_ID> to find messages mentioning the user
	query := fmt.Sprintf("<@%s>", d.userID)

	params := url.Values{
		"query": {query},
		"count": {"100"},
		"sort":  {"timestamp"},
	}

	resp, err := d.apiRequest(ctx, token, "search.messages", params)
	if err != nil {
		return nil, 1, err
	}

	var searchResp slackSearchResponse
	if err := json.Unmarshal(resp, &searchResp); err != nil {
		return nil, 1, fmt.Errorf("parse search.messages: %w", err)
	}
	if !searchResp.OK {
		return nil, 1, fmt.Errorf("search.messages failed: %s", searchResp.Error)
	}

	events := make([]models.Event, 0, len(searchResp.Messages.Matches))
	for i := range searchResp.Messages.Matches {
		msg := &searchResp.Messages.Matches[i]

		// Parse Slack timestamp to time.Time
		ts, err := parseSlackTimestamp(msg.TS)
		if err != nil {
			continue // Skip malformed timestamps
		}

		// Skip messages from before the since time (EFA 0003: Timestamp > Since)
		if !ts.After(since) {
			continue
		}

		// Determine if this is a thread reply
		isThreadReply := msg.ThreadTS != "" && msg.TS != msg.ThreadTS

		// Get the author username - prefer User ID over Username
		authorUsername := msg.User
		if authorUsername == "" {
			authorUsername = msg.Username
		}
		// If still empty, use a placeholder (will fail validation if truly empty)
		if authorUsername == "" {
			authorUsername = "unknown"
		}

		event := models.Event{
			Type:           models.EventTypeSlackMention,
			Title:          truncateTitle(stripMrkdwn(msg.Text)),
			Source:         models.SourceSlack,
			URL:            msg.Permalink,
			Author:         models.Person{Username: authorUsername},
			Timestamp:      ts.UTC(),              // Ensure UTC per EFA 0001
			Priority:       models.PriorityMedium, // Mentions are medium priority per EFA 0001
			RequiresAction: false,
			Metadata:       make(map[string]any),
		}

		// Add metadata per EFA 0001 allowed keys for Slack
		if workspace := getWorkspaceFromPermalink(msg.Permalink); workspace != "" {
			event.Metadata["workspace"] = workspace
		}
		if msg.Channel.Name != "" {
			event.Metadata["channel"] = msg.Channel.Name
		}
		if msg.ThreadTS != "" {
			event.Metadata["thread_ts"] = msg.ThreadTS
		}
		event.Metadata["is_thread_reply"] = isThreadReply

		events = append(events, event)
	}

	return events, 1, nil
}
