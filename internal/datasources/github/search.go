// Package github provides GraphQL search operations for PR and Issue discovery.
// Ground truth defined in specs/efas/0001-event-model.md (metadata fields)
// and specs/efas/0003-datasource-interface.md (fetch semantics).
package github

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// SearchPRsResponse represents the response from SearchPRsQuery.
type SearchPRsResponse struct {
	Search struct {
		Nodes      []SearchPRNode
		PageInfo   PageInfo `json:"pageInfo"`
		IssueCount int      `json:"issueCount"`
	} `json:"search"`
}

// SearchPRNode represents a PR node from search results.
type SearchPRNode struct {
	Title      string    `json:"title"`
	URL        string    `json:"url"`
	UpdatedAt  time.Time `json:"updatedAt"`
	Repository struct {
		NameWithOwner string `json:"nameWithOwner"`
	} `json:"repository"`
	Author struct {
		Login string `json:"login"`
	} `json:"author"`
	Number int `json:"number"`
}

// SearchIssuesResponse represents the response from SearchIssuesQuery.
type SearchIssuesResponse struct {
	Search struct {
		Nodes      []SearchIssueNode
		PageInfo   PageInfo `json:"pageInfo"`
		IssueCount int      `json:"issueCount"`
	} `json:"search"`
}

// SearchIssueNode represents an issue node from search results.
type SearchIssueNode struct {
	Title      string    `json:"title"`
	URL        string    `json:"url"`
	UpdatedAt  time.Time `json:"updatedAt"`
	Repository struct {
		NameWithOwner string `json:"nameWithOwner"`
	} `json:"repository"`
	Author struct {
		Login string `json:"login"`
	} `json:"author"`
	Number int `json:"number"`
}

// PageInfo contains pagination information from GraphQL responses.
type PageInfo struct {
	EndCursor   string `json:"endCursor"`
	HasNextPage bool   `json:"hasNextPage"`
}

// SearchItem represents a unified search result item (PR or Issue).
// Used internally for fetching full context.
type SearchItem struct {
	Owner     string
	Repo      string
	Title     string
	URL       string
	UpdatedAt time.Time
	Author    string
	Number    int
	IsPR      bool
}

// ParseSearchPRsResponse parses the raw GraphQL response from SearchPRsQuery.
func ParseSearchPRsResponse(data json.RawMessage) (*SearchPRsResponse, error) {
	var resp SearchPRsResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("parse search PRs response: %w", err)
	}
	return &resp, nil
}

// ParseSearchIssuesResponse parses the raw GraphQL response from SearchIssuesQuery.
func ParseSearchIssuesResponse(data json.RawMessage) (*SearchIssuesResponse, error) {
	var resp SearchIssuesResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("parse search issues response: %w", err)
	}
	return &resp, nil
}

// ToSearchItems converts SearchPRsResponse nodes to SearchItems.
func (r *SearchPRsResponse) ToSearchItems() []SearchItem {
	items := make([]SearchItem, 0, len(r.Search.Nodes))
	for _, node := range r.Search.Nodes {
		owner, repo := splitOwnerRepo(node.Repository.NameWithOwner)
		items = append(items, SearchItem{
			Owner:     owner,
			Repo:      repo,
			Number:    node.Number,
			Title:     node.Title,
			URL:       node.URL,
			UpdatedAt: node.UpdatedAt,
			Author:    node.Author.Login,
			IsPR:      true,
		})
	}
	return items
}

// ToSearchItems converts SearchIssuesResponse nodes to SearchItems.
func (r *SearchIssuesResponse) ToSearchItems() []SearchItem {
	items := make([]SearchItem, 0, len(r.Search.Nodes))
	for _, node := range r.Search.Nodes {
		owner, repo := splitOwnerRepo(node.Repository.NameWithOwner)
		items = append(items, SearchItem{
			Owner:     owner,
			Repo:      repo,
			Number:    node.Number,
			Title:     node.Title,
			URL:       node.URL,
			UpdatedAt: node.UpdatedAt,
			Author:    node.Author.Login,
			IsPR:      false,
		})
	}
	return items
}

// splitOwnerRepo splits "owner/repo" into owner and repo parts.
func splitOwnerRepo(nameWithOwner string) (owner, repo string) {
	parts := strings.SplitN(nameWithOwner, "/", 2)
	if len(parts) != 2 {
		return nameWithOwner, ""
	}
	return parts[0], parts[1]
}

// searchPRs executes a GraphQL PR search and returns search items.
// The query string should be a GitHub search query (e.g., "review-requested:@me is:open").
func searchPRs(ctx context.Context, client *GraphQLClient, query string, limit int) ([]SearchItem, error) {
	if limit <= 0 {
		limit = 100 // Default limit
	}

	variables := map[string]any{
		"query": query,
		"first": limit,
	}

	data, err := client.Execute(ctx, SearchPRsQuery, variables)
	if err != nil {
		return nil, fmt.Errorf("search PRs: %w", err)
	}

	resp, err := ParseSearchPRsResponse(data)
	if err != nil {
		return nil, err
	}

	return resp.ToSearchItems(), nil
}

// searchIssues executes a GraphQL issue search and returns search items.
// The query string should be a GitHub search query (e.g., "mentions:@me type:issue").
func searchIssues(ctx context.Context, client *GraphQLClient, query string, limit int) ([]SearchItem, error) {
	if limit <= 0 {
		limit = 100 // Default limit
	}

	variables := map[string]any{
		"query": query,
		"first": limit,
	}

	data, err := client.Execute(ctx, SearchIssuesQuery, variables)
	if err != nil {
		return nil, fmt.Errorf("search issues: %w", err)
	}

	resp, err := ParseSearchIssuesResponse(data)
	if err != nil {
		return nil, err
	}

	return resp.ToSearchItems(), nil
}
