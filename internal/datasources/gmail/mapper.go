// Package gmail provides Gmail message to Kora Event mapping.
// Ground truth defined in specs/efas/0001-event-model.md and specs/efas/0003-datasource-interface.md
//
// This file converts Gmail messages to Kora Events.
// IT IS FORBIDDEN TO ADD new EventTypes or metadata keys without updating EFA 0001.
//
//nolint:revive // Package name matches directory structure convention
package gmail

import (
	"fmt"
	"time"

	"github.com/dakaneye/kora/internal/models"
)

// ToEvent converts a Gmail message to a Kora Event.
//
// Parameters:
//   - msg: The Gmail message
//   - userEmail: The authenticated user's email (for To/CC detection)
//   - importantSenders: List of important sender emails/domains
//
// Returns error if the resulting event fails validation.
//
// Event type determination follows EFA 0001 priority order:
//  1. Check if important (sender list or IMPORTANT label) -> email_important
//  2. Check if direct recipient (To: field) -> email_direct
//  3. Check if CC recipient (CC: field) -> email_cc
func ToEvent(msg *Message, userEmail string, importantSenders []string) (models.Event, error) {
	if msg == nil {
		return models.Event{}, fmt.Errorf("message is nil")
	}

	// Determine event type and whether action is required
	eventType, requiresAction := determineEventType(msg, userEmail, importantSenders)

	// Determine priority based on event type per EFA 0001
	priority := determinePriority(eventType)

	// Build title based on event type
	title := determineTitle(msg, eventType)

	// Build metadata
	metadata := buildMetadata(msg)

	// Build Gmail URL from message ID
	// Gmail deep links use the format: https://mail.google.com/mail/u/0/#inbox/{message_id}
	url := fmt.Sprintf("https://mail.google.com/mail/u/0/#inbox/%s", msg.ID)

	// Create the event
	event := models.Event{
		Type:           eventType,
		Title:          truncateTitle(title),
		Source:         models.SourceGmail,
		URL:            url,
		Author:         buildAuthor(msg),
		Timestamp:      msg.Date(),
		Priority:       priority,
		RequiresAction: requiresAction,
		Metadata:       metadata,
	}

	// Validate before returning
	if err := event.Validate(); err != nil {
		return models.Event{}, fmt.Errorf("invalid event: %w", err)
	}

	return event, nil
}

// ToEvents batch converts messages to events.
// Returns (events, errors) - errors for individual message conversions.
// This allows partial success per EFA 0003 requirements.
func ToEvents(messages []*Message, userEmail string, importantSenders []string) ([]models.Event, []error) {
	if len(messages) == 0 {
		return nil, nil
	}

	events := make([]models.Event, 0, len(messages))
	var errs []error

	for _, msg := range messages {
		if msg == nil {
			continue
		}

		event, err := ToEvent(msg, userEmail, importantSenders)
		if err != nil {
			errs = append(errs, fmt.Errorf("message %s: %w", msg.ID, err))
			continue
		}
		events = append(events, event)
	}

	return events, errs
}

// determineEventType returns the event type and whether action is required.
// Follows EFA 0001 priority order for type selection:
//  1. Important sender OR Gmail-marked important -> email_important (High, requires action)
//  2. User in To: field, unread -> email_direct (Medium, requires action)
//  3. User in CC: field -> email_cc (Low, no action)
func determineEventType(msg *Message, userEmail string, importantSenders []string) (models.EventType, bool) {
	// 1. Check if important (highest priority for type determination)
	if IsImportant(msg, importantSenders) {
		return models.EventTypeEmailImportant, true
	}

	// 2. Check if direct recipient (To: field)
	if IsDirectRecipient(msg, userEmail) {
		// Direct emails require action when unread
		return models.EventTypeEmailDirect, msg.IsUnread()
	}

	// 3. Check if CC recipient (CC: field)
	if IsCCRecipient(msg, userEmail) {
		// CC emails don't require action
		return models.EventTypeEmailCC, false
	}

	// Default to direct if user is neither To nor CC but message was fetched
	// This handles edge cases like BCC or mailing list remnants
	return models.EventTypeEmailDirect, msg.IsUnread()
}

// determinePriority returns the priority based on event type per EFA 0001.
//
// | Event Type       | Priority      |
// |------------------|---------------|
// | email_important  | High (2)      |
// | email_direct     | Medium (3)    |
// | email_cc         | Low (4)       |
func determinePriority(eventType models.EventType) models.Priority {
	switch eventType {
	case models.EventTypeEmailImportant:
		return models.PriorityHigh
	case models.EventTypeEmailDirect:
		return models.PriorityMedium
	case models.EventTypeEmailCC:
		return models.PriorityLow
	default:
		return models.PriorityMedium
	}
}

// determineTitle creates an appropriate title based on event type.
//
// Title Format by Event Type:
//   - email_important: "Important: {subject}"
//   - email_direct: "{subject}"
//   - email_cc: "CC'd: {subject}"
func determineTitle(msg *Message, eventType models.EventType) string {
	subject := msg.Subject()
	if subject == "" {
		subject = "(No subject)"
	}

	switch eventType {
	case models.EventTypeEmailImportant:
		return fmt.Sprintf("Important: %s", subject)
	case models.EventTypeEmailCC:
		return fmt.Sprintf("CC'd: %s", subject)
	case models.EventTypeEmailDirect:
		return subject
	default:
		return subject
	}
}

// truncateTitle ensures the title is 1-100 characters per EFA 0001.
func truncateTitle(title string) string {
	if len(title) <= 100 {
		return title
	}
	return title[:97] + "..."
}

// buildAuthor creates the Author field from the message sender.
func buildAuthor(msg *Message) models.Person {
	name := msg.FromName()
	email := msg.FromEmail()

	// Use email as fallback for display name
	if name == "" {
		name = email
	}

	// Username is required - use email address
	username := email
	if username == "" {
		username = "unknown"
	}

	return models.Person{
		Name:     name,
		Username: username,
	}
}

// buildMetadata constructs the metadata map per EFA 0001 allowed keys.
// Required metadata keys for SourceGmail:
//   - message_id, thread_id, from_email, from_name
//   - to_addresses, cc_addresses, subject, snippet
//   - labels, is_unread, is_important, is_starred
//   - has_attachments, received_at
func buildMetadata(msg *Message) map[string]any {
	metadata := make(map[string]any)

	// Core identifiers
	metadata["message_id"] = msg.ID
	metadata["thread_id"] = msg.ThreadID

	// Sender info
	metadata["from_email"] = msg.FromEmail()
	metadata["from_name"] = msg.FromName()

	// Recipient addresses as slices (validation expects slices, not JSON strings)
	metadata["to_addresses"] = msg.To()
	metadata["cc_addresses"] = msg.CC()

	// Email content
	metadata["subject"] = msg.Subject()
	metadata["snippet"] = msg.Snippet

	// Labels as slice
	metadata["labels"] = msg.LabelIDs

	// Boolean flags
	metadata["is_unread"] = msg.IsUnread()
	metadata["is_important"] = msg.IsImportant()
	metadata["is_starred"] = msg.IsStarred()
	metadata["has_attachments"] = msg.HasAttachments()

	// Timestamp in RFC3339 format
	receivedAt := msg.Date()
	if !receivedAt.IsZero() {
		metadata["received_at"] = receivedAt.Format(time.RFC3339)
	} else {
		metadata["received_at"] = ""
	}

	return metadata
}
