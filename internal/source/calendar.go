package source

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/dakaneye/kora/internal/exec"
)

// Calendar fetches calendar events via the gws CLI.
type Calendar struct {
	cliSource
}

// NewCalendar returns a Calendar source. If runner is nil, a real subprocess runner is used.
func NewCalendar(runner exec.Runner) *Calendar {
	if runner == nil {
		runner = &exec.DefaultRunner{}
	}
	return &Calendar{cliSource: cliSource{
		name:        "calendar",
		runner:      runner,
		cli:         "gws",
		checkArgs:   []string{"auth", "status"},
		refreshArgs: []string{"auth", "login"},
	}}
}

// Fetch retrieves calendar events within the given time window.
func (c *Calendar) Fetch(ctx context.Context, since time.Duration) (json.RawMessage, error) {
	now := time.Now().UTC()
	timeMin := now.Add(-since).Format(time.RFC3339)
	timeMax := now.Format(time.RFC3339)
	params, err := json.Marshal(map[string]any{
		"calendarId":   "primary",
		"timeMin":      timeMin,
		"timeMax":      timeMax,
		"singleEvents": true,
		"orderBy":      "startTime",
	})
	if err != nil {
		return nil, fmt.Errorf("calendar params marshal: %w", err)
	}

	result, err := c.runner.Run(ctx, "gws", "calendar", "events", "list", "--params", string(params))
	if err != nil {
		return nil, fmt.Errorf("calendar fetch: %w", err)
	}

	// Wrap raw response under "events" key
	var raw json.RawMessage
	if unmarshalErr := json.Unmarshal([]byte(result.Stdout), &raw); unmarshalErr != nil {
		return nil, fmt.Errorf("calendar parse: %w", unmarshalErr)
	}

	data, err := json.Marshal(map[string]json.RawMessage{"events": raw})
	if err != nil {
		return nil, fmt.Errorf("calendar marshal: %w", err)
	}
	return data, nil
}
