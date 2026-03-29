package source

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/dakaneye/kora/internal/exec"
)

// GitHub fetches activity from GitHub via the gh CLI.
type GitHub struct {
	runner exec.Runner
}

// NewGitHub returns a GitHub source. If runner is nil, a real subprocess runner is used.
func NewGitHub(runner exec.Runner) *GitHub {
	if runner == nil {
		runner = &exec.DefaultRunner{}
	}
	return &GitHub{runner: runner}
}

func (g *GitHub) Name() string { return "github" }

func (g *GitHub) CheckAuth(ctx context.Context) error {
	_, err := g.runner.Run(ctx, "gh", "auth", "status")
	if err != nil {
		return fmt.Errorf("github auth check: %w", err)
	}
	return nil
}

func (g *GitHub) RefreshAuth(ctx context.Context) error {
	_, err := g.runner.Run(ctx, "gh", "auth", "refresh")
	if err != nil {
		return fmt.Errorf("github auth refresh: %w", err)
	}
	return nil
}

func (g *GitHub) Fetch(ctx context.Context, since time.Duration) (json.RawMessage, error) {
	cutoff := time.Now().Add(-since).Format("2006-01-02")
	jsonFields := "number,title,url,state,updatedAt,repository,author,labels,createdAt"

	type subQuery struct {
		key  string
		args []string
	}

	queries := []subQuery{
		{"review_requests", []string{"search", "prs", "--review-requested=@me", "--updated=>=" + cutoff, "--json", jsonFields, "--limit", "100"}},
		{"authored_prs", []string{"search", "prs", "--author=@me", "--updated=>=" + cutoff, "--json", jsonFields, "--limit", "100"}},
		{"assigned_issues", []string{"search", "issues", "--assignee=@me", "--updated=>=" + cutoff, "--json", jsonFields, "--limit", "100"}},
		{"commented_prs", []string{"search", "prs", "--commenter=@me", "--updated=>=" + cutoff, "--json", jsonFields, "--limit", "100"}},
	}

	var mu sync.Mutex
	results := make(map[string]json.RawMessage)
	var fetchErr error

	var wg sync.WaitGroup
	for _, q := range queries {
		wg.Add(1)
		go func() {
			defer wg.Done()
			result, err := g.runner.Run(ctx, "gh", q.args...)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				if fetchErr == nil {
					fetchErr = fmt.Errorf("github %s: %w", q.key, err)
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
		return nil, fmt.Errorf("github marshal: %w", err)
	}
	return data, nil
}
