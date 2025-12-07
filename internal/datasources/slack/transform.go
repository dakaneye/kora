package slack

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// parseSlackTimestamp converts a Slack timestamp to time.Time.
// Slack timestamps are in the format "1234567890.123456" (Unix seconds.microseconds).
//
// Returns an error if the timestamp format is invalid.
func parseSlackTimestamp(ts string) (time.Time, error) {
	if ts == "" {
		return time.Time{}, fmt.Errorf("empty timestamp")
	}

	parts := strings.Split(ts, ".")
	if len(parts) != 2 {
		return time.Time{}, fmt.Errorf("invalid slack timestamp format: %s", ts)
	}

	sec, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid seconds in timestamp: %w", err)
	}

	// Parse microseconds - pad to 6 digits if needed
	usecStr := parts[1]
	for len(usecStr) < 6 {
		usecStr += "0"
	}
	if len(usecStr) > 6 {
		usecStr = usecStr[:6]
	}

	usec, err := strconv.ParseInt(usecStr, 10, 64)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid microseconds in timestamp: %w", err)
	}

	// Convert to time.Time - microseconds to nanoseconds
	return time.Unix(sec, usec*1000).UTC(), nil
}

// truncateTitle truncates a title to 100 characters per EFA 0001.
// Also removes newlines and trims whitespace for single-line display.
// Adds "..." suffix if truncated.
func truncateTitle(title string) string {
	// Remove newlines for single-line title
	title = strings.ReplaceAll(title, "\n", " ")
	title = strings.ReplaceAll(title, "\r", " ")
	// Collapse multiple spaces
	for strings.Contains(title, "  ") {
		title = strings.ReplaceAll(title, "  ", " ")
	}
	title = strings.TrimSpace(title)

	// Handle empty title
	if title == "" {
		return "(empty message)"
	}

	// Truncate if needed
	if len(title) <= 100 {
		return title
	}
	return title[:97] + "..."
}

// stripMrkdwn removes Slack mrkdwn formatting for plain text display.
// Handles:
//   - User mentions: <@U12345|name> -> @name or <@U12345> -> @user
//   - Channel mentions: <#C12345|channel> -> #channel
//   - Links: <https://url|text> -> text or <https://url> -> url
func stripMrkdwn(text string) string {
	result := text

	// Process all <...> patterns
	for {
		start := strings.Index(result, "<")
		if start == -1 {
			break
		}
		end := strings.Index(result[start:], ">")
		if end == -1 {
			break
		}
		end += start // Adjust to absolute position

		mention := result[start : end+1]
		inner := mention[1 : len(mention)-1] // Remove < and >

		var replacement string
		switch {
		case strings.HasPrefix(inner, "@"):
			// User mention: <@U12345|name> or <@U12345>
			if idx := strings.Index(inner, "|"); idx != -1 {
				replacement = "@" + inner[idx+1:]
			} else {
				replacement = "@user"
			}
		case strings.HasPrefix(inner, "#"):
			// Channel mention: <#C12345|channel> or <#C12345>
			if idx := strings.Index(inner, "|"); idx != -1 {
				replacement = "#" + inner[idx+1:]
			} else {
				replacement = "#channel"
			}
		case strings.HasPrefix(inner, "!"):
			// Special mentions: <!here>, <!channel>, <!everyone>
			if idx := strings.Index(inner, "|"); idx != -1 {
				replacement = "@" + inner[idx+1:]
			} else {
				replacement = "@" + strings.TrimPrefix(inner, "!")
			}
		case strings.Contains(inner, "://"):
			// URL: <https://url|text> or <https://url>
			if idx := strings.Index(inner, "|"); idx != -1 {
				replacement = inner[idx+1:]
			} else {
				replacement = inner
			}
		default:
			// Unknown format, just use the inner content
			replacement = inner
		}

		result = result[:start] + replacement + result[end+1:]
	}

	return result
}
