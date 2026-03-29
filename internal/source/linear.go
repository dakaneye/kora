package source

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/dakaneye/kora/internal/exec"
)

// Linear fetches activity from Linear via the linear CLI's GraphQL API passthrough.
type Linear struct {
	runner exec.Runner
}

// NewLinear returns a Linear source. If runner is nil, a real subprocess runner is used.
func NewLinear(runner exec.Runner) *Linear {
	if runner == nil {
		runner = &exec.DefaultRunner{}
	}
	return &Linear{runner: runner}
}

func (l *Linear) Name() string { return "linear" }

func (l *Linear) CheckAuth(ctx context.Context) error {
	_, err := l.runner.Run(ctx, "linear", "auth", "whoami")
	if err != nil {
		return fmt.Errorf("linear auth check: %w", err)
	}
	return nil
}

func (l *Linear) RefreshAuth(ctx context.Context) error {
	_, err := l.runner.Run(ctx, "linear", "auth", "login")
	if err != nil {
		return fmt.Errorf("linear auth refresh: %w", err)
	}
	return nil
}

func (l *Linear) Fetch(ctx context.Context, since time.Duration) (json.RawMessage, error) {
	cutoff := time.Now().Add(-since).Format(time.RFC3339)

	type subQuery struct {
		key   string
		query string
	}

	queries := []subQuery{
		{
			key:   "assigned_issues",
			query: `{ viewer { assignedIssues(first: 100, orderBy: updatedAt) { nodes { identifier title state { name type } priority priorityLabel url updatedAt createdAt completedAt team { name key } project { name } labels { nodes { name } } } } } }`,
		},
		{
			key:   "cycles",
			query: `{ teams { nodes { name key cycles(first: 5, orderBy: createdAt) { nodes { number name startsAt endsAt progress completedScopeCount scopeCount } } } } }`,
		},
		{
			key:   "commented_issues",
			query: fmt.Sprintf(`{ issueSearch(filter: { comments: { user: { isMe: { eq: true } }, updatedAt: { gte: "%s" } } }, first: 100) { nodes { identifier title state { name type } priority priorityLabel url updatedAt team { name key } } } }`, cutoff),
		},
		{
			key:   "completed_issues",
			query: fmt.Sprintf(`{ issueSearch(filter: { completedAt: { gte: "%s" }, assignee: { isMe: { eq: true } } }, first: 100) { nodes { identifier title state { name type } priority priorityLabel url updatedAt completedAt team { name key } project { name } } } }`, cutoff),
		},
	}

	var mu sync.Mutex
	results := make(map[string]json.RawMessage)
	var fetchErr error

	var wg sync.WaitGroup
	for _, q := range queries {
		wg.Add(1)
		go func() {
			defer wg.Done()
			result, err := l.runner.Run(ctx, "linear", "api", q.query)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				if fetchErr == nil {
					fetchErr = fmt.Errorf("linear %s: %w", q.key, err)
				}
				return
			}
			results[q.key] = json.RawMessage(result.Stdout)
		}()
	}
	wg.Wait()

	if fetchErr != nil {
		return nil, fetchErr
	}

	data, err := json.Marshal(results)
	if err != nil {
		return nil, fmt.Errorf("linear marshal: %w", err)
	}
	return data, nil
}
