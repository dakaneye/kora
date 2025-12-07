// Package slack implements the Slack datasource using the Slack Web API.
// Ground truth defined in specs/efas/0003-datasource-interface.md
//
// IT IS FORBIDDEN TO CHANGE the core fetch logic without updating EFA 0003.
// Claude MUST stop and ask before modifying interface implementations.
//
// SECURITY: The Slack token is retrieved from auth.AuthProvider and used
// only in the Authorization header. It is never logged or exposed.
// See EFA 0002 for credential security requirements.
package slack

// slackSearchResponse represents the response from search.messages API.
//
//nolint:govet // Field order matches Slack API response for readability
type slackSearchResponse struct {
	OK       bool   `json:"ok"`
	Error    string `json:"error"`
	Messages struct {
		Matches []slackMessage `json:"matches"`
	} `json:"messages"`
}

// slackConversationsResponse represents the response from users.conversations API.
//
//nolint:govet // Field order matches Slack API response for readability
type slackConversationsResponse struct {
	OK       bool           `json:"ok"`
	Error    string         `json:"error"`
	Channels []slackChannel `json:"channels"`
}

// slackHistoryResponse represents the response from conversations.history API.
//
//nolint:govet // Field order matches Slack API response for readability
type slackHistoryResponse struct {
	OK       bool           `json:"ok"`
	Error    string         `json:"error"`
	Messages []slackMessage `json:"messages"`
}

// slackAuthTestResponse represents the response from auth.test API.
//
//nolint:govet // Field order matches Slack API response for readability
type slackAuthTestResponse struct {
	OK     bool   `json:"ok"`
	Error  string `json:"error"`
	UserID string `json:"user_id"`
	User   string `json:"user"`
	TeamID string `json:"team_id"`
	Team   string `json:"team"`
}

// slackMessage represents a message from Slack API.
// Used by both search.messages and conversations.history responses.
type slackMessage struct {
	TS        string `json:"ts"`
	ThreadTS  string `json:"thread_ts"`
	Text      string `json:"text"`
	User      string `json:"user"`
	Username  string `json:"username"`
	Permalink string `json:"permalink"`
	Channel   struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"channel"`
}

// slackChannel represents a Slack channel/conversation.
type slackChannel struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	User string `json:"user"` // For IM channels, this is the other user's ID
}
