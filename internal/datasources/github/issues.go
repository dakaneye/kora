package github

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/dakaneye/kora/internal/models"
)

// fetchIssueMentions fetches issues where user is mentioned.
// Query: mentions:@me type:issue updated:>=DATE
//
// Per EFA 0001:
//   - EventType: models.EventTypeIssueMention
//   - Priority: models.PriorityMedium (3)
//   - RequiresAction: false
//
// Per EFA 0002:
//
//	All API calls use GitHubDelegatedCredential.ExecuteAPI()
func (d *DataSource) fetchIssueMentions(ctx context.Context, cred githubCredential, since time.Time) ([]models.Event, error) {
	query := fmt.Sprintf("mentions:@me type:issue updated:>=%s", since.Format("2006-01-02"))

	if len(d.orgs) > 0 {
		query += " " + buildOrgFilter(d.orgs)
	}

	out, err := cred.ExecuteAPI(ctx, "search/issues",
		"-X", "GET",
		"-f", fmt.Sprintf("q=%s", query),
		"-f", "per_page=100",
	)
	if err != nil {
		return nil, fmt.Errorf("search issue mentions: %w", err)
	}

	var searchResult ghSearchResult
	if err := json.Unmarshal(out, &searchResult); err != nil {
		return nil, fmt.Errorf("parse issue mentions: %w", err)
	}

	events := make([]models.Event, 0, len(searchResult.Items))
	for i := range searchResult.Items {
		item := &searchResult.Items[i]
		// Skip PRs in issue search (the type:issue filter should handle this,
		// but we check to be safe)
		if item.PullRequest.IsSet() {
			continue
		}

		// Skip items that are before the since time
		if !item.UpdatedAt.After(since) {
			continue
		}

		event := models.Event{
			Type:   models.EventTypeIssueMention,
			Title:  truncateTitle(fmt.Sprintf("Mentioned in issue: %s", item.Title)),
			Source: models.SourceGitHub,
			URL:    item.HTMLURL,
			Author: models.Person{
				Name:     item.User.Login,
				Username: item.User.Login,
			},
			Timestamp:      item.UpdatedAt,
			Priority:       models.PriorityMedium, // Mentions are medium priority per EFA 0001
			RequiresAction: false,
			Metadata: map[string]any{
				"repo":               extractRepoName(item.RepositoryURL),
				"number":             item.Number,
				"state":              item.State,
				"author_login":       item.User.Login,
				"user_relationships": []string{"mentioned"},
				"labels":             extractLabels(item.Labels),
			},
		}
		events = append(events, event)
	}

	return events, nil
}

// fetchAssignedIssues fetches issues assigned to the user.
// Query: assignee:@me is:open type:issue updated:>=DATE
//
// Per EFA 0001:
//   - EventType: models.EventTypeIssueAssigned
//   - Priority: models.PriorityMedium (3)
//   - RequiresAction: true
//
// Per EFA 0002:
//
//	All API calls use GitHubDelegatedCredential.ExecuteAPI()
func (d *DataSource) fetchAssignedIssues(ctx context.Context, cred githubCredential, since time.Time) ([]models.Event, error) {
	query := fmt.Sprintf("assignee:@me is:open type:issue updated:>=%s", since.Format("2006-01-02"))

	if len(d.orgs) > 0 {
		query += " " + buildOrgFilter(d.orgs)
	}

	out, err := cred.ExecuteAPI(ctx, "search/issues",
		"-X", "GET",
		"-f", fmt.Sprintf("q=%s", query),
		"-f", "per_page=100",
	)
	if err != nil {
		return nil, fmt.Errorf("search assigned issues: %w", err)
	}

	var searchResult ghSearchResult
	if err := json.Unmarshal(out, &searchResult); err != nil {
		return nil, fmt.Errorf("parse assigned issues: %w", err)
	}

	events := make([]models.Event, 0, len(searchResult.Items))
	for i := range searchResult.Items {
		item := &searchResult.Items[i]
		// Skip items that are before the since time
		if !item.UpdatedAt.After(since) {
			continue
		}

		event := models.Event{
			Type:   models.EventTypeIssueAssigned,
			Title:  truncateTitle(fmt.Sprintf("Assigned: %s", item.Title)),
			Source: models.SourceGitHub,
			URL:    item.HTMLURL,
			Author: models.Person{
				Name:     item.User.Login,
				Username: item.User.Login,
			},
			Timestamp:      item.UpdatedAt,
			Priority:       models.PriorityMedium, // Assigned issues are medium priority per EFA 0001
			RequiresAction: true,
			Metadata: map[string]any{
				"repo":               extractRepoName(item.RepositoryURL),
				"number":             item.Number,
				"state":              item.State,
				"author_login":       item.User.Login,
				"user_relationships": []string{"assignee"},
				"labels":             extractLabels(item.Labels),
			},
		}
		events = append(events, event)
	}

	return events, nil
}
