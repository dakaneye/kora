package output

import (
	"encoding/json"
	"time"

	"github.com/dakaneye/kora/internal/models"
)

// JSONFormatter renders events as JSON for MCP tool responses.
// This is the primary format for Claude to parse and reason about.
type JSONFormatter struct {
	pretty bool
}

// NewJSONFormatter creates a JSONFormatter.
// If pretty is true, output is indented for human debugging.
// If pretty is false, output is compact for MCP responses.
func NewJSONFormatter(pretty bool) *JSONFormatter {
	return &JSONFormatter{pretty: pretty}
}

// jsonOutput is the top-level structure of JSON output.
type jsonOutput struct {
	GeneratedAt time.Time   `json:"generated_at"`
	Stats       jsonStats   `json:"stats"`
	Events      []jsonEvent `json:"events"`
}

// jsonStats contains summary statistics.
//
//nolint:govet // Field order matches JSON output for readability
type jsonStats struct {
	Total          int               `json:"total"`
	RequiresAction int               `json:"requires_action"`
	ByPriority     map[string]int    `json:"by_priority"`
	PartialSuccess bool              `json:"partial_success"`
	SourceErrors   map[string]string `json:"source_errors,omitempty"`
}

// jsonEvent is the JSON representation of an event.
//
//nolint:govet // Field order matches JSON output for readability
type jsonEvent struct {
	Type           string         `json:"type"`
	Title          string         `json:"title"`
	Source         string         `json:"source"`
	URL            string         `json:"url"`
	Author         string         `json:"author"`
	Timestamp      time.Time      `json:"timestamp"`
	Priority       string         `json:"priority"`
	RequiresAction bool           `json:"requires_action"`
	Metadata       map[string]any `json:"metadata,omitempty"`
}

// Format renders events as JSON.
func (f *JSONFormatter) Format(events []models.Event, stats *FormatStats) (string, error) {
	// Sort events: RequiresAction first, then by Priority, then by Timestamp
	sorted := sortByPriorityThenTime(events)

	// Build output structure
	out := jsonOutput{
		GeneratedAt: time.Now().UTC(),
		Stats: jsonStats{
			Total:          stats.TotalEvents,
			RequiresAction: stats.RequiresAction,
			ByPriority:     make(map[string]int),
			PartialSuccess: len(stats.SourceErrors) > 0,
			SourceErrors:   stats.SourceErrors,
		},
		Events: make([]jsonEvent, 0, len(sorted)),
	}

	// Convert priority counts to string keys
	out.Stats.ByPriority["critical"] = stats.ByPriority[models.PriorityCritical]
	out.Stats.ByPriority["high"] = stats.ByPriority[models.PriorityHigh]
	out.Stats.ByPriority["medium"] = stats.ByPriority[models.PriorityMedium]
	out.Stats.ByPriority["low"] = stats.ByPriority[models.PriorityLow]
	out.Stats.ByPriority["info"] = stats.ByPriority[models.PriorityInfo]

	// Convert events
	for i := range sorted {
		e := &sorted[i]
		je := jsonEvent{
			Type:           string(e.Type),
			Title:          e.Title,
			Source:         string(e.Source),
			URL:            e.URL,
			Author:         formatAuthor(e.Author),
			Timestamp:      e.Timestamp,
			Priority:       priorityLabel(e.Priority),
			RequiresAction: e.RequiresAction,
			Metadata:       e.Metadata,
		}
		out.Events = append(out.Events, je)
	}

	// Marshal to JSON
	var data []byte
	var err error
	if f.pretty {
		data, err = json.MarshalIndent(out, "", "  ")
	} else {
		data, err = json.Marshal(out)
	}
	if err != nil {
		return "", err
	}

	return string(data), nil
}
