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
	orgs  []string
	repos []string
}

// NewGitHub returns a GitHub source. If runner is nil, a real subprocess runner is used.
// Orgs and repos are optional filters — if empty, no org/repo scoping is applied.
func NewGitHub(runner exec.Runner, orgs, repos []string) *GitHub {
	if runner == nil {
		runner = &exec.DefaultRunner{}
	}
	return &GitHub{
		cliSource: cliSource{
			name:        "github",
			runner:      runner,
			cli:         "gh",
			checkArgs:   []string{"auth", "status"},
			refreshArgs: []string{"auth", "refresh"},
		},
		orgs:  orgs,
		repos: repos,
	}
}

// Fetch retrieves PRs, issues, and review requests from GitHub.
func (g *GitHub) Fetch(ctx context.Context, since time.Duration) (json.RawMessage, error) {
	cutoff := time.Now().Add(-since).Format("2006-01-02")
	jsonFields := "number,title,url,state,updatedAt,repository,author,labels,createdAt"

	// Build qualifier args for org/repo filtering
	qualifiers := make([]string, 0, len(g.orgs)+len(g.repos))
	for _, org := range g.orgs {
		qualifiers = append(qualifiers, "--owner="+org)
	}
	for _, repo := range g.repos {
		qualifiers = append(qualifiers, "--repo="+repo)
	}

	buildArgs := func(base []string) []string {
		return append(append(base, qualifiers...), "--updated=>="+cutoff, "--json", jsonFields, "--limit", "100")
	}

	queries := []parallelQuery{
		{"review_requests", buildArgs([]string{"search", "prs", "--review-requested=@me"})},
		{"authored_prs", buildArgs([]string{"search", "prs", "--author=@me"})},
		{"assigned_issues", buildArgs([]string{"search", "issues", "--assignee=@me"})},
		{"commented_prs", buildArgs([]string{"search", "prs", "--commenter=@me"})},
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
