// Package github implements the GitHub datasource using gh CLI delegation.
// Ground truth defined in specs/efas/0003-datasource-interface.md
package github

import "time"

// ghSearchResult represents the GitHub search API response.
type ghSearchResult struct {
	Items      []ghItem `json:"items"`
	TotalCount int      `json:"total_count"`
}

// ghItem represents a single issue or PR from GitHub search results.
// Field order optimized for memory alignment.
type ghItem struct {
	UpdatedAt     time.Time `json:"updated_at"`
	Title         string    `json:"title"`
	State         string    `json:"state"`
	HTMLURL       string    `json:"html_url"`
	RepositoryURL string    `json:"repository_url"`
	User          ghUser    `json:"user"`
	PullRequest   ghPR      `json:"pull_request"`
	Labels        []ghLabel `json:"labels"`
	Number        int       `json:"number"`
}

// ghUser represents a GitHub user in API responses.
type ghUser struct {
	Login string `json:"login"`
}

// ghLabel represents a label on an issue or PR.
type ghLabel struct {
	Name string `json:"name"`
}

// ghPR contains pull request metadata. Present only when the item is a PR.
type ghPR struct {
	URL string `json:"url"`
}

// IsSet returns true if this item has pull request data (meaning it's a PR, not an issue).
func (pr ghPR) IsSet() bool {
	return pr.URL != ""
}
