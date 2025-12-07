package github

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/dakaneye/kora/internal/models"
)

// fetchPRReviewRequests fetches PRs where user's review is requested.
// Query: review-requested:@me is:open -draft:true updated:>=DATE
//
// Per EFA 0001:
//   - EventType: models.EventTypePRReview
//   - Priority: models.PriorityHigh (2)
//   - RequiresAction: true
//
// Per EFA 0002:
//
//	All API calls use GitHubDelegatedCredential.ExecuteAPI()
func (d *DataSource) fetchPRReviewRequests(ctx context.Context, cred githubCredential, since time.Time) ([]models.Event, error) {
	query := fmt.Sprintf("review-requested:@me is:open -draft:true updated:>=%s", since.Format("2006-01-02"))

	// Add org filter if configured
	if len(d.orgs) > 0 {
		query += " " + buildOrgFilter(d.orgs)
	}

	out, err := cred.ExecuteAPI(ctx, "search/issues",
		"-X", "GET",
		"-f", fmt.Sprintf("q=%s", query),
		"-f", "per_page=100",
	)
	if err != nil {
		return nil, fmt.Errorf("search pr reviews: %w", err)
	}

	var searchResult ghSearchResult
	if err := json.Unmarshal(out, &searchResult); err != nil {
		return nil, fmt.Errorf("parse pr reviews: %w", err)
	}

	events := make([]models.Event, 0, len(searchResult.Items))
	for i := range searchResult.Items {
		item := &searchResult.Items[i]
		// Skip items that aren't PRs
		if !item.PullRequest.IsSet() {
			continue
		}

		// Skip items that are before the since time
		if !item.UpdatedAt.After(since) {
			continue
		}

		event := models.Event{
			Type:   models.EventTypePRReview,
			Title:  truncateTitle(fmt.Sprintf("Review requested: %s", item.Title)),
			Source: models.SourceGitHub,
			URL:    item.HTMLURL,
			Author: models.Person{
				Name:     item.User.Login,
				Username: item.User.Login,
			},
			Timestamp:      item.UpdatedAt,
			Priority:       models.PriorityHigh, // PR reviews are high priority per EFA 0001
			RequiresAction: true,
			Metadata: map[string]any{
				"repo":         extractRepoName(item.RepositoryURL),
				"number":       item.Number,
				"state":        item.State,
				"review_state": "pending",
				"labels":       extractLabels(item.Labels),
			},
		}
		events = append(events, event)
	}

	return events, nil
}

// fetchPRMentions fetches PRs where user is mentioned.
// Query: mentions:@me type:pr updated:>=DATE
//
// Per EFA 0001:
//   - EventType: models.EventTypePRMention
//   - Priority: models.PriorityMedium (3)
//   - RequiresAction: false
//
// Per EFA 0002:
//
//	All API calls use GitHubDelegatedCredential.ExecuteAPI()
func (d *DataSource) fetchPRMentions(ctx context.Context, cred githubCredential, since time.Time) ([]models.Event, error) {
	query := fmt.Sprintf("mentions:@me type:pr updated:>=%s", since.Format("2006-01-02"))

	if len(d.orgs) > 0 {
		query += " " + buildOrgFilter(d.orgs)
	}

	out, err := cred.ExecuteAPI(ctx, "search/issues",
		"-X", "GET",
		"-f", fmt.Sprintf("q=%s", query),
		"-f", "per_page=100",
	)
	if err != nil {
		return nil, fmt.Errorf("search pr mentions: %w", err)
	}

	var searchResult ghSearchResult
	if err := json.Unmarshal(out, &searchResult); err != nil {
		return nil, fmt.Errorf("parse pr mentions: %w", err)
	}

	events := make([]models.Event, 0, len(searchResult.Items))
	for i := range searchResult.Items {
		item := &searchResult.Items[i]
		// Skip items that are before the since time
		if !item.UpdatedAt.After(since) {
			continue
		}

		event := models.Event{
			Type:   models.EventTypePRMention,
			Title:  truncateTitle(fmt.Sprintf("Mentioned in PR: %s", item.Title)),
			Source: models.SourceGitHub,
			URL:    item.HTMLURL,
			Author: models.Person{
				Name:     item.User.Login,
				Username: item.User.Login,
			},
			Timestamp:      item.UpdatedAt,
			Priority:       models.PriorityMedium, // Mentions are medium priority per EFA 0001
			RequiresAction: false,                 // May or may not need action
			Metadata: map[string]any{
				"repo":   extractRepoName(item.RepositoryURL),
				"number": item.Number,
				"state":  item.State,
				"labels": extractLabels(item.Labels),
			},
		}
		events = append(events, event)
	}

	return events, nil
}
