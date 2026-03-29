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
	runner exec.Runner
}

// NewCalendar returns a Calendar source. If runner is nil, a real subprocess runner is used.
func NewCalendar(runner exec.Runner) *Calendar {
	if runner == nil {
		runner = &exec.DefaultRunner{}
	}
	return &Calendar{runner: runner}
}

func (c *Calendar) Name() string { return "calendar" }

func (c *Calendar) CheckAuth(ctx context.Context) error {
	_, err := c.runner.Run(ctx, "gws", "auth", "status")
	if err != nil {
		return fmt.Errorf("calendar auth check: %w", err)
	}
	return nil
}

func (c *Calendar) RefreshAuth(ctx context.Context) error {
	_, err := c.runner.Run(ctx, "gws", "auth", "login")
	if err != nil {
		return fmt.Errorf("calendar auth refresh: %w", err)
	}
	return nil
}

func (c *Calendar) Fetch(ctx context.Context, since time.Duration) (json.RawMessage, error) {
	now := time.Now().UTC()
	timeMin := now.Add(-since).Format(time.RFC3339)
	timeMax := now.Format(time.RFC3339)
	params := fmt.Sprintf(`{"calendarId":"primary","timeMin":"%s","timeMax":"%s","singleEvents":true,"orderBy":"startTime"}`, timeMin, timeMax)

	result, err := c.runner.Run(ctx, "gws", "calendar", "events", "list", "--params", params)
	if err != nil {
		return nil, fmt.Errorf("calendar fetch: %w", err)
	}

	// Wrap raw response under "events" key
	var raw json.RawMessage
	if err := json.Unmarshal([]byte(result.Stdout), &raw); err != nil {
		return nil, fmt.Errorf("calendar parse: %w", err)
	}

	data, err := json.Marshal(map[string]json.RawMessage{"events": raw})
	if err != nil {
		return nil, fmt.Errorf("calendar marshal: %w", err)
	}
	return data, nil
}
