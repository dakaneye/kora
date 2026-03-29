package source

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/dakaneye/kora/internal/exec"
)

// Linear fetches activity from Linear via the linear CLI's GraphQL API passthrough.
type Linear struct {
	cliSource
}

// NewLinear returns a Linear source. If runner is nil, a real subprocess runner is used.
func NewLinear(runner exec.Runner) *Linear {
	if runner == nil {
		runner = &exec.DefaultRunner{}
	}
	return &Linear{cliSource: cliSource{
		name:        "linear",
		runner:      runner,
		cli:         "linear",
		checkArgs:   []string{"auth", "whoami"},
		refreshArgs: []string{"auth", "login"},
	}}
}

// Fetch retrieves assigned issues, cycles, and recent activity from Linear.
func (l *Linear) Fetch(ctx context.Context, since time.Duration) (json.RawMessage, error) {
	cutoff := time.Now().Add(-since).Format(time.RFC3339)

	queries := []parallelQuery{
		{
			key:  "assigned_issues",
			args: []string{"api", `{ viewer { assignedIssues(first: 100, orderBy: updatedAt) { nodes { identifier title state { name type } priority priorityLabel url updatedAt createdAt completedAt team { name key } project { name } labels { nodes { name } } } } } }`},
		},
		{
			key:  "cycles",
			args: []string{"api", `{ teams { nodes { name key cycles(first: 5, orderBy: createdAt) { nodes { number name startsAt endsAt progress } } } } }`},
		},
		{
			key:  "commented_issues",
			args: []string{"api", fmt.Sprintf(`{ issues(filter: { comments: { user: { isMe: { eq: true } }, updatedAt: { gte: "%s" } } }, first: 100) { nodes { identifier title state { name type } priority priorityLabel url updatedAt team { name key } } } }`, cutoff)}, //nolint:gocritic // GraphQL requires literal double quotes in query string
		},
		{
			key:  "completed_issues",
			args: []string{"api", fmt.Sprintf(`{ issues(filter: { completedAt: { gte: "%s" }, assignee: { isMe: { eq: true } } }, first: 100) { nodes { identifier title state { name type } priority priorityLabel url updatedAt completedAt team { name key } project { name } } } }`, cutoff)}, //nolint:gocritic // GraphQL requires literal double quotes in query string
		},
	}

	results, err := fetchParallel(ctx, l.runner, l.cli, queries)
	if err != nil {
		return nil, fmt.Errorf("linear fetch: %w", err)
	}

	data, err := json.Marshal(results)
	if err != nil {
		return nil, fmt.Errorf("linear marshal: %w", err)
	}
	return data, nil
}
