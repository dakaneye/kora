// Package output provides formatters for rendering event digests.
// These formatters produce output optimized for consumption by Claude (AI assistant)
// as MCP tool responses.
package output

import (
	"fmt"
	"time"

	"github.com/dakaneye/kora/internal/models"
)

// Formatter renders a list of events into a formatted string.
// Implementations must produce output suitable for AI consumption:
// - No ANSI color codes
// - No box-drawing characters
// - No emojis
type Formatter interface {
	// Format renders events with optional statistics into a formatted string.
	// Events are sorted by RequiresAction (true first), then by Priority (1=highest),
	// then by Timestamp (most recent first).
	Format(events []models.Event, stats *FormatStats) (string, error)
}

// FormatStats contains summary statistics about the digest.
//
//nolint:govet // Field order prioritizes readability over memory alignment
type FormatStats struct {
	// TotalEvents is the count of all events in the digest.
	TotalEvents int

	// RequiresAction is the count of events where RequiresAction is true.
	RequiresAction int

	// ByPriority maps each priority level to its event count.
	ByPriority map[models.Priority]int

	// Duration is how long the fetch operation took.
	Duration time.Duration

	// SourceErrors maps source names to their error messages.
	// Non-empty when partial success occurred.
	SourceErrors map[string]string
}

// NewFormatStats creates a FormatStats from a slice of events.
func NewFormatStats(events []models.Event, duration time.Duration, sourceErrors map[string]string) *FormatStats {
	stats := &FormatStats{
		TotalEvents:  len(events),
		ByPriority:   make(map[models.Priority]int),
		Duration:     duration,
		SourceErrors: sourceErrors,
	}

	// Initialize all priority levels to 0
	stats.ByPriority[models.PriorityCritical] = 0
	stats.ByPriority[models.PriorityHigh] = 0
	stats.ByPriority[models.PriorityMedium] = 0
	stats.ByPriority[models.PriorityLow] = 0
	stats.ByPriority[models.PriorityInfo] = 0

	for i := range events {
		if events[i].RequiresAction {
			stats.RequiresAction++
		}
		stats.ByPriority[events[i].Priority]++
	}

	if stats.SourceErrors == nil {
		stats.SourceErrors = make(map[string]string)
	}

	return stats
}

// NewFormatter creates a formatter for the specified format type.
// Supported formats: "json", "markdown", "text".
func NewFormatter(format string) (Formatter, error) {
	switch format {
	case "json":
		return NewJSONFormatter(false), nil
	case "json-pretty":
		return NewJSONFormatter(true), nil
	case "markdown", "md":
		return NewMarkdownFormatter(), nil
	case "text", "txt":
		return NewTextFormatter(), nil
	default:
		return nil, fmt.Errorf("unsupported format: %q (supported: json, json-pretty, markdown, text)", format)
	}
}
