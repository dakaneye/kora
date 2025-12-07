// Package gmail provides Gmail message filtering for the Gmail datasource.
// Ground truth defined in specs/efas/0003-datasource-interface.md
//
// EFA 0003 Rule 12: Gmail must filter mailing lists (List-Unsubscribe, List-Id headers).
// EFA 0003 Rule 13: Gmail must detect automated senders (noreply@, no-reply@, etc).
package gmail

import "strings"

// automatedPatterns are common email prefixes for automated senders.
// These patterns indicate system-generated emails that should be excluded
// from the morning digest to focus on emails from real people.
//
// EFA 0003 Rule 13: Gmail datasource MUST filter automated sender patterns.
var automatedPatterns = []string{
	"noreply@",
	"no-reply@",
	"donotreply@",
	"do-not-reply@",
	"notifications@",
	"notification@",
	"automated@",
	"bot@",
	"mailer-daemon@",
	"postmaster@",
	"support@",    // Often automated ticketing systems
	"info@",       // Often newsletters
	"newsletter@", // Newsletters
	"updates@",    // Update notifications
	"alert@",      // System alerts
	"alerts@",     // System alerts
}

// IsMailingList checks if a message is from a mailing list.
// Detection is based on the presence of List-Unsubscribe, List-Id, or
// Precedence headers that indicate bulk or list mail.
//
// EFA 0003 Rule 12: Gmail datasource MUST check List-Unsubscribe header
// to exclude mailing lists.
//
// Parameters:
//   - msg: The Gmail message to check
//
// Returns true if the message appears to be from a mailing list.
func IsMailingList(msg *Message) bool {
	if msg == nil {
		return false
	}

	// Check for List-Unsubscribe header (most common indicator)
	if msg.GetHeader("List-Unsubscribe") != "" {
		return true
	}

	// Check for List-Id header (RFC 2919)
	if msg.GetHeader("List-Id") != "" {
		return true
	}

	// Check for Precedence: list or bulk (RFC 2076)
	precedence := strings.ToLower(msg.GetHeader("Precedence"))
	if precedence == "list" || precedence == "bulk" {
		return true
	}

	return false
}

// IsAutomated checks if a message is from an automated sender.
// Detection is based on common sender patterns like noreply@, no-reply@,
// donotreply@, notifications@, automated@, bot@, and similar prefixes.
//
// EFA 0003 Rule 13: Gmail datasource MUST filter automated sender patterns.
//
// Parameters:
//   - msg: The Gmail message to check
//
// Returns true if the message appears to be from an automated sender.
func IsAutomated(msg *Message) bool {
	if msg == nil {
		return false
	}

	// Check sender email against known automated patterns
	from := strings.ToLower(msg.FromEmail())
	for _, pattern := range automatedPatterns {
		if strings.HasPrefix(from, pattern) {
			return true
		}
	}

	// Check for "via" in the from name (automated forwards, delegated sending)
	fromName := strings.ToLower(msg.FromName())
	return strings.Contains(fromName, " via ")
}

// IsImportant checks if a message should be treated as important.
// A message is considered important if:
//   - Gmail has marked it with the IMPORTANT label
//   - The sender is in the importantSenders list (exact match or domain match)
//
// Domain matching allows patterns like "@company.com" to match any sender
// from that domain (e.g., "anyone@company.com").
//
// Parameters:
//   - msg: The Gmail message to check
//   - importantSenders: List of important email addresses or domain patterns
//
// Returns true if the message should be treated as important.
func IsImportant(msg *Message, importantSenders []string) bool {
	if msg == nil {
		return false
	}

	// Check Gmail's IMPORTANT label
	if msg.IsImportant() {
		return true
	}

	// Check against configured important senders
	from := strings.ToLower(msg.FromEmail())
	if from == "" {
		return false
	}

	for _, sender := range importantSenders {
		senderLower := strings.ToLower(sender)

		// Exact email match
		if from == senderLower {
			return true
		}

		// Domain match (e.g., "@company.com" matches "anyone@company.com")
		if strings.HasPrefix(senderLower, "@") && strings.HasSuffix(from, senderLower) {
			return true
		}
	}

	return false
}

// IsDirectRecipient checks if the user is in the To: field of the message.
// This indicates the message was directly addressed to the user, not just CC'd.
//
// Parameters:
//   - msg: The Gmail message to check
//   - userEmail: The user's email address to match
//
// Returns true if the user is a direct recipient (in To: field).
func IsDirectRecipient(msg *Message, userEmail string) bool {
	if msg == nil || userEmail == "" {
		return false
	}

	for _, to := range msg.To() {
		if strings.EqualFold(to, userEmail) {
			return true
		}
	}

	return false
}

// IsCCRecipient checks if the user is in the CC: field of the message.
// This indicates the message was copied to the user, not directly addressed.
//
// Parameters:
//   - msg: The Gmail message to check
//   - userEmail: The user's email address to match
//
// Returns true if the user is a CC recipient (in CC: field).
func IsCCRecipient(msg *Message, userEmail string) bool {
	if msg == nil || userEmail == "" {
		return false
	}

	for _, cc := range msg.CC() {
		if strings.EqualFold(cc, userEmail) {
			return true
		}
	}

	return false
}

// FilterMessages filters messages for the Gmail datasource.
// Returns only messages that:
//   - Are NOT from mailing lists (no List-Unsubscribe, List-Id headers)
//   - Are NOT from automated senders (no noreply@, etc. patterns)
//   - Are from real people
//
// This function applies EFA 0003 Rules 12 and 13 to filter out noise
// and return only messages that likely require human attention.
//
// Parameters:
//   - messages: The list of messages to filter
//
// Returns a new slice containing only messages from real people.
func FilterMessages(messages []*Message) []*Message {
	if len(messages) == 0 {
		return nil
	}

	filtered := make([]*Message, 0, len(messages))
	for _, msg := range messages {
		if msg == nil {
			continue
		}

		// Skip mailing lists (EFA 0003 Rule 12)
		if IsMailingList(msg) {
			continue
		}

		// Skip automated senders (EFA 0003 Rule 13)
		if IsAutomated(msg) {
			continue
		}

		// Message passes all filters - from a real person
		filtered = append(filtered, msg)
	}

	return filtered
}

// FilterMessagesWithOptions filters messages with additional configuration.
// This provides more control over filtering behavior compared to FilterMessages.
//
// Parameters:
//   - messages: The list of messages to filter
//   - opts: Configuration options for filtering
//
// Returns a new slice of filtered messages.
func FilterMessagesWithOptions(messages []*Message, opts FilterOptions) []*Message {
	if len(messages) == 0 {
		return nil
	}

	filtered := make([]*Message, 0, len(messages))
	for _, msg := range messages {
		if msg == nil {
			continue
		}

		// Skip mailing lists unless configured to include
		if !opts.IncludeMailingLists && IsMailingList(msg) {
			continue
		}

		// Skip automated senders unless configured to include
		if !opts.IncludeAutomated && IsAutomated(msg) {
			continue
		}

		// Apply user email filter if specified
		if opts.UserEmail != "" {
			isRecipient := IsDirectRecipient(msg, opts.UserEmail) ||
				IsCCRecipient(msg, opts.UserEmail)
			if opts.DirectOnly && !IsDirectRecipient(msg, opts.UserEmail) {
				continue
			}
			if !isRecipient {
				continue
			}
		}

		filtered = append(filtered, msg)
	}

	return filtered
}

// FilterOptions configures message filtering behavior.
type FilterOptions struct {
	// UserEmail is the user's email address for recipient filtering.
	// If empty, recipient filtering is skipped.
	UserEmail string

	// DirectOnly filters to only messages where user is in To: field.
	// Requires UserEmail to be set. Default false includes CC recipients.
	DirectOnly bool

	// IncludeMailingLists allows mailing list messages to pass through.
	// Default false filters them out per EFA 0003 Rule 12.
	IncludeMailingLists bool

	// IncludeAutomated allows automated sender messages to pass through.
	// Default false filters them out per EFA 0003 Rule 13.
	IncludeAutomated bool
}
