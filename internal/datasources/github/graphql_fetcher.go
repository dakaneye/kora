// Package github provides GraphQL-based fetching for rich PR and Issue context.
// Ground truth defined in specs/efas/0001-event-model.md (metadata fields)
// and specs/efas/0003-datasource-interface.md (fetch semantics).
//
// This file implements the two-phase fetch approach:
// 1. Search for items matching criteria (SearchPRsQuery/SearchIssuesQuery)
// 2. Fetch full context for each item (PRQuery/IssueQuery)
package github

import (
	"context"
	"fmt"
	"time"

	"github.com/dakaneye/kora/internal/models"
)

// fetchPRReviewRequestsGraphQL fetches PRs where user's review is requested using GraphQL.
// Query: review-requested:@me is:open -draft:true updated:>=DATE
//
// Per EFA 0001:
//   - EventType: models.EventTypePRReview
//   - Priority: models.PriorityHigh (2)
//   - RequiresAction: true
//
// Per EFA 0003: Context must be used for all network operations.
func (d *DataSource) fetchPRReviewRequestsGraphQL(ctx context.Context, client *GraphQLClient, since time.Time, orgs []string) ([]models.Event, error) {
	query := fmt.Sprintf("review-requested:@me is:open -draft:true type:pr updated:>=%s", since.Format("2006-01-02"))

	if len(orgs) > 0 {
		query += " " + buildOrgFilter(orgs)
	}

	// Search for matching PRs
	items, err := searchPRs(ctx, client, query, 100)
	if err != nil {
		return nil, fmt.Errorf("search pr reviews: %w", err)
	}

	// Fetch full context for each PR
	events := make([]models.Event, 0, len(items))
	for _, item := range items {
		// Check context cancellation between items
		select {
		case <-ctx.Done():
			return events, ctx.Err()
		default:
		}

		// Skip items that are before the since time
		if !item.UpdatedAt.After(since) {
			continue
		}

		// Fetch full PR context
		metadata, err := fetchPRFullContext(ctx, client, item.Owner, item.Repo, item.Number)
		if err != nil {
			// Log but continue with partial data
			// Use basic metadata from search results
			metadata = map[string]any{
				"repo":               item.Owner + "/" + item.Repo,
				"number":             item.Number,
				"state":              "open",
				"author_login":       item.Author,
				"user_relationships": []string{"reviewer"},
			}
		} else {
			// Add user_relationships to metadata
			metadata["user_relationships"] = []string{"reviewer"}
		}

		event := models.Event{
			Type:   models.EventTypePRReview,
			Title:  truncateTitle(fmt.Sprintf("Review requested: %s", item.Title)),
			Source: models.SourceGitHub,
			URL:    item.URL,
			Author: models.Person{
				Name:     item.Author,
				Username: item.Author,
			},
			Timestamp:      item.UpdatedAt,
			Priority:       models.PriorityHigh, // PR reviews are high priority per EFA 0001
			RequiresAction: true,
			Metadata:       metadata,
		}
		events = append(events, event)
	}

	return events, nil
}

// fetchPRMentionsGraphQL fetches PRs where user is mentioned using GraphQL.
// Query: mentions:@me type:pr updated:>=DATE
//
// Per EFA 0001:
//   - EventType: models.EventTypePRMention
//   - Priority: models.PriorityMedium (3)
//   - RequiresAction: false
func (d *DataSource) fetchPRMentionsGraphQL(ctx context.Context, client *GraphQLClient, since time.Time, orgs []string) ([]models.Event, error) {
	query := fmt.Sprintf("mentions:@me type:pr updated:>=%s", since.Format("2006-01-02"))

	if len(orgs) > 0 {
		query += " " + buildOrgFilter(orgs)
	}

	// Search for matching PRs
	items, err := searchPRs(ctx, client, query, 100)
	if err != nil {
		return nil, fmt.Errorf("search pr mentions: %w", err)
	}

	// Fetch full context for each PR
	events := make([]models.Event, 0, len(items))
	for _, item := range items {
		select {
		case <-ctx.Done():
			return events, ctx.Err()
		default:
		}

		if !item.UpdatedAt.After(since) {
			continue
		}

		metadata, err := fetchPRFullContext(ctx, client, item.Owner, item.Repo, item.Number)
		if err != nil {
			metadata = map[string]any{
				"repo":               item.Owner + "/" + item.Repo,
				"number":             item.Number,
				"state":              "open",
				"author_login":       item.Author,
				"user_relationships": []string{"mentioned"},
			}
		} else {
			metadata["user_relationships"] = []string{"mentioned"}
		}

		event := models.Event{
			Type:   models.EventTypePRMention,
			Title:  truncateTitle(fmt.Sprintf("Mentioned in PR: %s", item.Title)),
			Source: models.SourceGitHub,
			URL:    item.URL,
			Author: models.Person{
				Name:     item.Author,
				Username: item.Author,
			},
			Timestamp:      item.UpdatedAt,
			Priority:       models.PriorityMedium, // Mentions are medium priority per EFA 0001
			RequiresAction: false,
			Metadata:       metadata,
		}
		events = append(events, event)
	}

	return events, nil
}

// fetchIssueMentionsGraphQL fetches issues where user is mentioned using GraphQL.
// Query: mentions:@me type:issue updated:>=DATE
//
// Per EFA 0001:
//   - EventType: models.EventTypeIssueMention
//   - Priority: models.PriorityMedium (3)
//   - RequiresAction: false
func (d *DataSource) fetchIssueMentionsGraphQL(ctx context.Context, client *GraphQLClient, since time.Time, orgs []string) ([]models.Event, error) {
	query := fmt.Sprintf("mentions:@me type:issue updated:>=%s", since.Format("2006-01-02"))

	if len(orgs) > 0 {
		query += " " + buildOrgFilter(orgs)
	}

	// Search for matching issues
	items, err := searchIssues(ctx, client, query, 100)
	if err != nil {
		return nil, fmt.Errorf("search issue mentions: %w", err)
	}

	events := make([]models.Event, 0, len(items))
	for _, item := range items {
		select {
		case <-ctx.Done():
			return events, ctx.Err()
		default:
		}

		if !item.UpdatedAt.After(since) {
			continue
		}

		metadata, err := fetchIssueFullContext(ctx, client, item.Owner, item.Repo, item.Number)
		if err != nil {
			metadata = map[string]any{
				"repo":               item.Owner + "/" + item.Repo,
				"number":             item.Number,
				"state":              "open",
				"author_login":       item.Author,
				"user_relationships": []string{"mentioned"},
			}
		} else {
			metadata["user_relationships"] = []string{"mentioned"}
		}

		event := models.Event{
			Type:   models.EventTypeIssueMention,
			Title:  truncateTitle(fmt.Sprintf("Mentioned in issue: %s", item.Title)),
			Source: models.SourceGitHub,
			URL:    item.URL,
			Author: models.Person{
				Name:     item.Author,
				Username: item.Author,
			},
			Timestamp:      item.UpdatedAt,
			Priority:       models.PriorityMedium, // Mentions are medium priority per EFA 0001
			RequiresAction: false,
			Metadata:       metadata,
		}
		events = append(events, event)
	}

	return events, nil
}

// fetchAssignedIssuesGraphQL fetches issues assigned to the user using GraphQL.
// Query: assignee:@me is:open type:issue updated:>=DATE
//
// Per EFA 0001:
//   - EventType: models.EventTypeIssueAssigned
//   - Priority: models.PriorityMedium (3)
//   - RequiresAction: true
func (d *DataSource) fetchAssignedIssuesGraphQL(ctx context.Context, client *GraphQLClient, since time.Time, orgs []string) ([]models.Event, error) {
	query := fmt.Sprintf("assignee:@me is:open type:issue updated:>=%s", since.Format("2006-01-02"))

	if len(orgs) > 0 {
		query += " " + buildOrgFilter(orgs)
	}

	// Search for matching issues
	items, err := searchIssues(ctx, client, query, 100)
	if err != nil {
		return nil, fmt.Errorf("search assigned issues: %w", err)
	}

	events := make([]models.Event, 0, len(items))
	for _, item := range items {
		select {
		case <-ctx.Done():
			return events, ctx.Err()
		default:
		}

		if !item.UpdatedAt.After(since) {
			continue
		}

		metadata, err := fetchIssueFullContext(ctx, client, item.Owner, item.Repo, item.Number)
		if err != nil {
			metadata = map[string]any{
				"repo":               item.Owner + "/" + item.Repo,
				"number":             item.Number,
				"state":              "open",
				"author_login":       item.Author,
				"user_relationships": []string{"assignee"},
			}
		} else {
			metadata["user_relationships"] = []string{"assignee"}
		}

		event := models.Event{
			Type:   models.EventTypeIssueAssigned,
			Title:  truncateTitle(fmt.Sprintf("Assigned: %s", item.Title)),
			Source: models.SourceGitHub,
			URL:    item.URL,
			Author: models.Person{
				Name:     item.Author,
				Username: item.Author,
			},
			Timestamp:      item.UpdatedAt,
			Priority:       models.PriorityMedium, // Assigned issues are medium priority per EFA 0001
			RequiresAction: true,
			Metadata:       metadata,
		}
		events = append(events, event)
	}

	return events, nil
}

// fetchPRFullContext fetches rich PR metadata using PRQuery.
// Returns the metadata map per EFA 0001.
func fetchPRFullContext(ctx context.Context, client *GraphQLClient, owner, repo string, number int) (map[string]any, error) {
	variables := map[string]any{
		"owner":  owner,
		"repo":   repo,
		"number": number,
	}

	data, err := client.Execute(ctx, PRQuery, variables)
	if err != nil {
		return nil, fmt.Errorf("fetch PR context: %w", err)
	}

	return ParsePRResponse(data, owner+"/"+repo)
}

// fetchIssueFullContext fetches rich Issue metadata using IssueQuery.
// Returns the metadata map per EFA 0001.
func fetchIssueFullContext(ctx context.Context, client *GraphQLClient, owner, repo string, number int) (map[string]any, error) {
	variables := map[string]any{
		"owner":  owner,
		"repo":   repo,
		"number": number,
	}

	data, err := client.Execute(ctx, IssueQuery, variables)
	if err != nil {
		return nil, fmt.Errorf("fetch issue context: %w", err)
	}

	return ParseIssueResponse(data, owner+"/"+repo)
}
