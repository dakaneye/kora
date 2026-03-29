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
	teams []string
}

// NewLinear returns a Linear source. If runner is nil, a real subprocess runner is used.
// Teams is an optional list of team keys (e.g. ["ECO"]) to scope queries.
func NewLinear(runner exec.Runner, teams []string) *Linear {
	if runner == nil {
		runner = &exec.DefaultRunner{}
	}
	return &Linear{
		cliSource: cliSource{
			name:        "linear",
			runner:      runner,
			cli:         "linear",
			checkArgs:   []string{"auth", "whoami"},
			refreshArgs: []string{"auth", "login"},
		},
		teams: teams,
	}
}

// Fetch retrieves assigned issues, cycles, and recent activity from Linear.
func (l *Linear) Fetch(ctx context.Context, since time.Duration) (json.RawMessage, error) {
	cutoff := time.Now().Add(-since).Format(time.RFC3339)

	// Build team filter fragment for GraphQL queries
	teamFilter := ""
	if len(l.teams) > 0 {
		keys := ""
		for i, t := range l.teams {
			if i > 0 {
				keys += ", "
			}
			keys += fmt.Sprintf(`"%s"`, t) //nolint:gocritic // GraphQL requires literal double quotes
		}
		teamFilter = fmt.Sprintf(`, team: { key: { in: [%s] } }`, keys)
	}

	issueFields := "identifier title state { name type } priority priorityLabel url updatedAt team { name key }"

	queries := []parallelQuery{
		{
			key:  "assigned_issues",
			args: []string{"api", fmt.Sprintf(`{ viewer { assignedIssues(first: 100, orderBy: updatedAt, filter: { updatedAt: { gte: "%s" }%s }) { nodes { %s createdAt completedAt project { name } labels { nodes { name } } } } } }`, cutoff, teamFilter, issueFields)}, //nolint:gocritic // GraphQL requires literal double quotes
		},
		{
			key:  "cycles",
			args: []string{"api", fmt.Sprintf(`{ teams%s { nodes { name key cycles(first: 5, orderBy: createdAt) { nodes { number name startsAt endsAt progress } } } } }`, l.teamsGraphQLFilter())},
		},
		{
			key:  "commented_issues",
			args: []string{"api", fmt.Sprintf(`{ issues(filter: { comments: { user: { isMe: { eq: true } }, updatedAt: { gte: "%s" } }%s }, first: 100) { nodes { %s } } }`, cutoff, teamFilter, issueFields)}, //nolint:gocritic // GraphQL requires literal double quotes
		},
		{
			key:  "completed_issues",
			args: []string{"api", fmt.Sprintf(`{ issues(filter: { completedAt: { gte: "%s" }, assignee: { isMe: { eq: true } }%s }, first: 100) { nodes { %s completedAt project { name } } } }`, cutoff, teamFilter, issueFields)}, //nolint:gocritic // GraphQL requires literal double quotes
		},
	}

	results, err := fetchParallel(ctx, l.runner, "linear", queries)
	if err != nil {
		return nil, fmt.Errorf("linear fetch: %w", err)
	}

	data, err := json.Marshal(results)
	if err != nil {
		return nil, fmt.Errorf("linear marshal: %w", err)
	}
	return data, nil
}

// teamsGraphQLFilter returns a GraphQL filter argument for the top-level teams query.
// Returns empty string if no team filter is configured (fetches all teams).
func (l *Linear) teamsGraphQLFilter() string {
	if len(l.teams) == 0 {
		return ""
	}
	keys := ""
	for i, t := range l.teams {
		if i > 0 {
			keys += ", "
		}
		keys += fmt.Sprintf(`"%s"`, t) //nolint:gocritic // GraphQL requires literal double quotes
	}
	return fmt.Sprintf(`(filter: { key: { in: [%s] } })`, keys)
}
