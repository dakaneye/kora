package source

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/dakaneye/kora/internal/exec"
)

// GitHub fetches activity from GitHub via the gh CLI.
type GitHub struct {
	cliSource
}

// NewGitHub returns a GitHub source. If runner is nil, a real subprocess runner is used.
func NewGitHub(runner exec.Runner) *GitHub {
	if runner == nil {
		runner = &exec.DefaultRunner{}
	}
	return &GitHub{cliSource: cliSource{
		name:        "github",
		runner:      runner,
		cli:         "gh",
		checkArgs:   []string{"auth", "status"},
		refreshArgs: []string{"auth", "refresh"},
	}}
}

// Fetch retrieves PRs, issues, and review requests from GitHub.
func (g *GitHub) Fetch(ctx context.Context, since time.Duration) (json.RawMessage, error) {
	cutoff := time.Now().Add(-since).Format("2006-01-02")
	jsonFields := "number,title,url,state,updatedAt,repository,author,labels,createdAt"

	queries := []parallelQuery{
		{"review_requests", []string{"search", "prs", "--review-requested=@me", "--updated=>=" + cutoff, "--json", jsonFields, "--limit", "100"}},
		{"authored_prs", []string{"search", "prs", "--author=@me", "--updated=>=" + cutoff, "--json", jsonFields, "--limit", "100"}},
		{"assigned_issues", []string{"search", "issues", "--assignee=@me", "--updated=>=" + cutoff, "--json", jsonFields, "--limit", "100"}},
		{"commented_prs", []string{"search", "prs", "--commenter=@me", "--updated=>=" + cutoff, "--json", jsonFields, "--limit", "100"}},
	}

	results, err := fetchParallel(ctx, g.runner, g.cli, queries)
	if err != nil {
		return nil, fmt.Errorf("github fetch: %w", err)
	}

	data, err := json.Marshal(results)
	if err != nil {
		return nil, fmt.Errorf("github marshal: %w", err)
	}
	return data, nil
}
