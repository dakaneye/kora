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

// searchPRsResponse represents the response from SearchPRsQuery.
type searchPRsResponse struct {
	Search struct {
		Nodes      []searchPRNode
		PageInfo   pageInfo `json:"pageInfo"`
		IssueCount int      `json:"issueCount"`
	} `json:"search"`
}

// searchPRNode represents a PR node from search results.
type searchPRNode struct {
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

// searchIssuesResponse represents the response from SearchIssuesQuery.
type searchIssuesResponse struct {
	Search struct {
		Nodes      []searchIssueNode
		PageInfo   pageInfo `json:"pageInfo"`
		IssueCount int      `json:"issueCount"`
	} `json:"search"`
}

// searchIssueNode represents an issue node from search results.
type searchIssueNode struct {
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

// pageInfo contains pagination information from GraphQL responses.
type pageInfo struct {
	EndCursor   string `json:"endCursor"`
	HasNextPage bool   `json:"hasNextPage"`
}

// searchItem represents a unified search result item (PR or Issue).
// Used internally for fetching full context.
type searchItem struct {
	Owner     string
	Repo      string
	Title     string
	URL       string
	UpdatedAt time.Time
	Author    string
	Number    int
	IsPR      bool
}

// parseSearchPRsResponse parses the raw GraphQL response from SearchPRsQuery.
func parseSearchPRsResponse(data json.RawMessage) (*searchPRsResponse, error) {
	var resp searchPRsResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("parse search PRs response: %w", err)
	}
	return &resp, nil
}

// parseSearchIssuesResponse parses the raw GraphQL response from SearchIssuesQuery.
func parseSearchIssuesResponse(data json.RawMessage) (*searchIssuesResponse, error) {
	var resp searchIssuesResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("parse search issues response: %w", err)
	}
	return &resp, nil
}

// toSearchItems converts searchPRsResponse nodes to searchItems.
func (r *searchPRsResponse) toSearchItems() []searchItem {
	items := make([]searchItem, 0, len(r.Search.Nodes))
	for _, node := range r.Search.Nodes {
		owner, repo := splitOwnerRepo(node.Repository.NameWithOwner)
		items = append(items, searchItem{
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

// toSearchItems converts searchIssuesResponse nodes to searchItems.
func (r *searchIssuesResponse) toSearchItems() []searchItem {
	items := make([]searchItem, 0, len(r.Search.Nodes))
	for _, node := range r.Search.Nodes {
		owner, repo := splitOwnerRepo(node.Repository.NameWithOwner)
		items = append(items, searchItem{
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
// Handles pagination automatically to fetch all results up to limit.
func searchPRs(ctx context.Context, client *GraphQLClient, query string, limit int) ([]searchItem, error) {
	if limit <= 0 {
		limit = 100 // Default limit
	}

	var allItems []searchItem
	var cursor string

	// Pagination loop - fetch pages until we have enough results or no more pages
	for {
		// Check context cancellation between pages
		select {
		case <-ctx.Done():
			return allItems, ctx.Err()
		default:
		}

		// Calculate page size: min(remaining, 100) - GitHub's max is 100 per page
		pageSize := limit - len(allItems)
		if pageSize > 100 {
			pageSize = 100
		}

		// Variable named "searchQuery" to avoid conflict with gh's reserved "query" field
		variables := map[string]any{
			"searchQuery": query,
			"first":       pageSize,
		}
		if cursor != "" {
			variables["after"] = cursor
		}

		data, err := client.Execute(ctx, SearchPRsQuery, variables)
		if err != nil {
			return nil, fmt.Errorf("search PRs: %w", err)
		}

		resp, err := parseSearchPRsResponse(data)
		if err != nil {
			return nil, err
		}

		allItems = append(allItems, resp.toSearchItems()...)

		// Stop if we've reached our limit or no more pages
		if len(allItems) >= limit || !resp.Search.PageInfo.HasNextPage {
			break
		}

		cursor = resp.Search.PageInfo.EndCursor
	}

	return allItems, nil
}

// searchIssues executes a GraphQL issue search and returns search items.
// The query string should be a GitHub search query (e.g., "mentions:@me type:issue").
// Handles pagination automatically to fetch all results up to limit.
func searchIssues(ctx context.Context, client *GraphQLClient, query string, limit int) ([]searchItem, error) {
	if limit <= 0 {
		limit = 100 // Default limit
	}

	var allItems []searchItem
	var cursor string

	// Pagination loop - fetch pages until we have enough results or no more pages
	for {
		// Check context cancellation between pages
		select {
		case <-ctx.Done():
			return allItems, ctx.Err()
		default:
		}

		// Calculate page size: min(remaining, 100) - GitHub's max is 100 per page
		pageSize := limit - len(allItems)
		if pageSize > 100 {
			pageSize = 100
		}

		// Variable named "searchQuery" to avoid conflict with gh's reserved "query" field
		variables := map[string]any{
			"searchQuery": query,
			"first":       pageSize,
		}
		if cursor != "" {
			variables["after"] = cursor
		}

		data, err := client.Execute(ctx, SearchIssuesQuery, variables)
		if err != nil {
			return nil, fmt.Errorf("search issues: %w", err)
		}

		resp, err := parseSearchIssuesResponse(data)
		if err != nil {
			return nil, err
		}

		allItems = append(allItems, resp.toSearchItems()...)

		// Stop if we've reached our limit or no more pages
		if len(allItems) >= limit || !resp.Search.PageInfo.HasNextPage {
			break
		}

		cursor = resp.Search.PageInfo.EndCursor
	}

	return allItems, nil
}
