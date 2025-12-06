// Package models defines the core Event type for Kora.
// Ground truth defined in specs/efas/0001-event-model.md
//
// IT IS FORBIDDEN TO CHANGE the Event struct without updating EFA 0001.
// Claude MUST stop and ask before modifying this file.
package models

import "time"

// Event represents a single work item from any datasource.
// EFA 0001: All fields are protected. Do not add/remove without EFA update.
// IT IS FORBIDDEN TO CHANGE THIS STRUCTURE without updating EFA 0001.
//
//nolint:govet // Field order matches EFA 0001 specification, not optimized for alignment
type Event struct {
	// Type classifies the event. Must be one of the EventType constants.
	Type EventType `json:"type"`

	// Title is a brief human-readable summary (1 line, <100 chars).
	Title string `json:"title"`

	// Source identifies which datasource produced this event.
	Source Source `json:"source"`

	// URL is a direct link to the item (PR, message, etc).
	URL string `json:"url"`

	// Author is who created/sent the item.
	Author Person `json:"author"`

	// Timestamp is when the event occurred (UTC).
	Timestamp time.Time `json:"timestamp"`

	// Priority is 1-5 where 1 is highest priority.
	Priority Priority `json:"priority"`

	// RequiresAction indicates if user must respond/act.
	RequiresAction bool `json:"requires_action"`

	// Metadata contains source-specific data.
	// Keys are defined per-source in EFA 0001.
	Metadata map[string]any `json:"metadata,omitempty"`
}

// EventType classifies events from datasources.
// EFA 0001: Do NOT add types here without updating the EFA.
type EventType string

// EventType constants for GitHub events.
const (
	// EventTypePRReview indicates a review was requested on a PR.
	EventTypePRReview EventType = "pr_review"
	// EventTypePRMention indicates a mention in a PR.
	EventTypePRMention EventType = "pr_mention"
	// EventTypeIssueMention indicates a mention in an issue.
	EventTypeIssueMention EventType = "issue_mention"
	// EventTypeIssueAssigned indicates assignment to an issue.
	EventTypeIssueAssigned EventType = "issue_assigned"
)

// EventType constants for Slack events.
const (
	// EventTypeSlackDM indicates a direct message.
	EventTypeSlackDM EventType = "slack_dm"
	// EventTypeSlackMention indicates an @mention in a channel.
	EventTypeSlackMention EventType = "slack_mention"
)

// validEventTypes is the authoritative set of valid event types.
// EFA 0001: Do NOT add types here without updating the EFA.
var validEventTypes = map[EventType]struct{}{
	EventTypePRReview:      {},
	EventTypePRMention:     {},
	EventTypeIssueMention:  {},
	EventTypeIssueAssigned: {},
	EventTypeSlackDM:       {},
	EventTypeSlackMention:  {},
}

// IsValid reports whether t is a defined EventType constant.
func (t EventType) IsValid() bool {
	_, ok := validEventTypes[t]
	return ok
}

// Source identifies which datasource produced an event.
// EFA 0001: Do NOT add sources here without updating the EFA.
type Source string

// Source constants for supported datasources.
const (
	// SourceGitHub indicates the event came from GitHub.
	SourceGitHub Source = "github"
	// SourceSlack indicates the event came from Slack.
	SourceSlack Source = "slack"
)

// validSources is the authoritative set of valid sources.
// EFA 0001: Do NOT add sources here without updating the EFA.
var validSources = map[Source]struct{}{
	SourceGitHub: {},
	SourceSlack:  {},
}

// IsValid reports whether s is a defined Source constant.
func (s Source) IsValid() bool {
	_, ok := validSources[s]
	return ok
}
