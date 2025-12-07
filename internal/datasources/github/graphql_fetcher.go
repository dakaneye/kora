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
//   - Priority: models.PriorityHigh (2) for direct user requests
//   - Priority: models.PriorityMedium (3) for team-only requests
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
		}

		// Determine priority and user_relationships based on review request type
		// Per EFA 0001: user requests = PriorityHigh (2), team-only = PriorityMedium (3)
		priority, relationships := calculateReviewRequestPriority(metadata)
		metadata["user_relationships"] = relationships

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
			Priority:       priority,
			RequiresAction: true,
			Metadata:       metadata,
		}
		events = append(events, event)
	}

	return events, nil
}

// calculateReviewRequestPriority determines priority and user_relationships based on review_requests.
// Per EFA 0001:
//   - Direct user request: Priority 2 (High), relationship "direct_reviewer"
//   - Team-only request: Priority 3 (Medium), relationship "team_reviewer"
//   - Mixed (user + team): Priority 2 (High), relationship "direct_reviewer"
func calculateReviewRequestPriority(metadata map[string]any) (priority models.Priority, relationships []string) {
	reviewRequests, ok := metadata["review_requests"].([]map[string]any)
	if !ok || len(reviewRequests) == 0 {
		// No review_requests in metadata, default to reviewer relationship with High priority
		return models.PriorityHigh, []string{"reviewer"}
	}

	// Check if any review request is a direct user request
	hasUserRequest := false
	for _, rr := range reviewRequests {
		if rr["type"] == "user" {
			hasUserRequest = true
			break
		}
	}

	if hasUserRequest {
		return models.PriorityHigh, []string{"direct_reviewer"}
	}
	return models.PriorityMedium, []string{"team_reviewer"}
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

// fetchAuthoredPRsGraphQL fetches PRs authored by the current user using GraphQL.
// Query: author:LOGIN is:open type:pr updated:>=DATE
//
// Per EFA 0001:
//   - EventType: models.EventTypePRAuthor
//   - Priority: PriorityCritical (1) for CI failing
//   - Priority: PriorityHigh (2) for changes requested
//   - Priority: PriorityMedium (3) for pending/approved
//   - RequiresAction: true when CI failing or changes requested
//
// Per EFA 0003: Context must be used for all network operations.
func (d *DataSource) fetchAuthoredPRsGraphQL(ctx context.Context, client *GraphQLClient, cred githubCredential, since time.Time, orgs []string) ([]models.Event, error) {
	// Get current user login
	currentUser, err := d.getCurrentUser(ctx, cred)
	if err != nil {
		return nil, fmt.Errorf("get current user: %w", err)
	}

	// Build search query for authored PRs
	query := fmt.Sprintf("author:%s is:open type:pr updated:>=%s", currentUser, since.Format("2006-01-02"))

	if len(orgs) > 0 {
		query += " " + buildOrgFilter(orgs)
	}

	// Search for matching PRs
	items, err := searchPRs(ctx, client, query, 100)
	if err != nil {
		return nil, fmt.Errorf("search authored prs: %w", err)
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
			metadata = map[string]any{
				"repo":               item.Owner + "/" + item.Repo,
				"number":             item.Number,
				"state":              "open",
				"author_login":       item.Author,
				"user_relationships": []string{"author"},
			}
		} else {
			metadata["user_relationships"] = []string{"author"}
		}

		// Calculate priority based on CI status and reviews per EFA 0001
		priority, requiresAction, titlePrefix := calculatePRAuthorPriority(metadata)

		event := models.Event{
			Type:   models.EventTypePRAuthor,
			Title:  truncateTitle(fmt.Sprintf("%s%s", titlePrefix, item.Title)),
			Source: models.SourceGitHub,
			URL:    item.URL,
			Author: models.Person{
				Name:     item.Author,
				Username: item.Author,
			},
			Timestamp:      item.UpdatedAt,
			Priority:       priority,
			RequiresAction: requiresAction,
			Metadata:       metadata,
		}
		events = append(events, event)
	}

	return events, nil
}

// calculatePRAuthorPriority determines priority and title prefix for authored PRs.
// Per EFA 0001 Priority Assignment Rules:
//   - CI failing: Priority 1 (Critical), RequiresAction=true, "CI failing: "
//   - Changes requested: Priority 2 (High), RequiresAction=true, "Changes requested: "
//   - No reviews (awaiting): Priority 3 (Medium), RequiresAction=false, "Awaiting review: "
//   - Has approvals: Priority 3 (Medium), RequiresAction=false, "Ready to merge: "
//   - Default (pending): Priority 3 (Medium), RequiresAction=false, "Your PR: "
//
//nolint:errcheck // type assertions intentionally ignore ok bool for graceful handling
func calculatePRAuthorPriority(metadata map[string]any) (priority models.Priority, requiresAction bool, titlePrefix string) {
	// Check CI status first (highest priority factor)
	ciRollup, _ := metadata["ci_rollup"].(string)
	if ciRollup == "failure" || ciRollup == "error" {
		return models.PriorityCritical, true, "CI failing: "
	}

	// Check reviews for changes_requested or approvals
	reviews, reviewsOK := metadata["reviews"].([]map[string]any)
	if reviewsOK && len(reviews) > 0 {
		hasApproval := false
		hasChangesRequested := false

		for _, review := range reviews {
			state, _ := review["state"].(string)
			switch state {
			case "approved":
				hasApproval = true
			case "changes_requested":
				hasChangesRequested = true
			}
		}

		// Changes requested takes priority over approvals
		if hasChangesRequested {
			return models.PriorityHigh, true, "Changes requested: "
		}

		if hasApproval {
			return models.PriorityMedium, false, "Ready to merge: "
		}
	}

	// Check if any reviews exist at all
	reviewRequests, reqOK := metadata["review_requests"].([]map[string]any)
	if !reviewsOK || len(reviews) == 0 {
		// No reviews yet - check if review is requested
		if reqOK && len(reviewRequests) > 0 {
			return models.PriorityMedium, false, "Awaiting review: "
		}
	}

	// Default: pending state
	return models.PriorityMedium, false, "Your PR: "
}
