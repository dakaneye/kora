package slack

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"time"

	"github.com/dakaneye/kora/internal/models"
)

// fetchDMs retrieves direct messages since the given time.
// Strategy:
//  1. List IM conversations via users.conversations
//  2. Fetch history for each IM channel via conversations.history
//
// Per EFA 0001:
//   - EventType: models.EventTypeSlackDM
//   - Priority: models.PriorityHigh (2)
//   - RequiresAction: true
//
// Per EFA 0002:
//   - Token is passed to apiRequest which handles it securely
//
// Returns:
//   - events: the fetched DM events
//   - apiCalls: number of API calls made
//   - err: any error encountered (only returns error if listing conversations fails)
func (d *DataSource) fetchDMs(ctx context.Context, token string, since time.Time) ([]models.Event, int, error) {
	// 1. List IM (direct message) conversations
	params := url.Values{
		"types":            {"im"},
		"exclude_archived": {"true"},
		"limit":            {"100"},
	}

	resp, err := d.apiRequest(ctx, token, "users.conversations", params)
	apiCalls := 1
	if err != nil {
		return nil, apiCalls, err
	}

	var convResp slackConversationsResponse
	if err := json.Unmarshal(resp, &convResp); err != nil {
		return nil, apiCalls, fmt.Errorf("parse users.conversations: %w", err)
	}
	if !convResp.OK {
		return nil, apiCalls, fmt.Errorf("users.conversations failed: %s", convResp.Error)
	}

	// 2. Fetch history for each DM channel
	var allEvents []models.Event
	// Convert since to Slack timestamp format (Unix seconds with microseconds)
	sinceTS := fmt.Sprintf("%d.000000", since.Unix())

	for _, channel := range convResp.Channels {
		// Check for context cancellation between channel fetches
		if ctx.Err() != nil {
			break
		}

		histParams := url.Values{
			"channel": {channel.ID},
			"oldest":  {sinceTS},
			"limit":   {"50"},
		}

		histResp, err := d.apiRequest(ctx, token, "conversations.history", histParams)
		apiCalls++
		if err != nil {
			// Continue with other channels on error (partial success per EFA 0003)
			continue
		}

		var history slackHistoryResponse
		if err := json.Unmarshal(histResp, &history); err != nil {
			continue
		}
		if !history.OK {
			continue
		}

		for i := range history.Messages {
			msg := &history.Messages[i]

			// Skip own messages (compare to cached user ID)
			if msg.User == d.userID {
				continue
			}

			ts, err := parseSlackTimestamp(msg.TS)
			if err != nil {
				continue
			}

			// Skip messages from before the since time (EFA 0003: Timestamp > Since)
			if !ts.After(since) {
				continue
			}

			// Determine if this is a thread reply
			isThreadReply := msg.ThreadTS != "" && msg.TS != msg.ThreadTS

			// Get author - in DMs, User field contains the sender's ID
			authorUsername := msg.User
			if authorUsername == "" {
				authorUsername = "unknown"
			}

			event := models.Event{
				Type:           models.EventTypeSlackDM,
				Title:          truncateTitle(stripMrkdwn(msg.Text)),
				Source:         models.SourceSlack,
				URL:            buildDMPermalink(channel.ID, msg.TS),
				Author:         models.Person{Username: authorUsername},
				Timestamp:      ts.UTC(),            // Ensure UTC per EFA 0001
				Priority:       models.PriorityHigh, // DMs are high priority per EFA 0001
				RequiresAction: true,
				Metadata:       make(map[string]any),
			}

			// Add metadata per EFA 0001 allowed keys for Slack
			if msg.ThreadTS != "" {
				event.Metadata["thread_ts"] = msg.ThreadTS
			}
			event.Metadata["is_thread_reply"] = isThreadReply

			allEvents = append(allEvents, event)
		}
	}

	return allEvents, apiCalls, nil
}
