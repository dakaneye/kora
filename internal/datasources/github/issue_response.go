// Package github provides GraphQL response parsing for rich Issue context.
// Ground truth defined in specs/efas/0001-event-model.md (metadata fields).
package github

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// IssueGraphQLResponse is the top-level response from IssueQuery.
type IssueGraphQLResponse struct {
	Repository struct {
		Issue *IssueData `json:"issue"`
	} `json:"repository"`
}

// IssueData represents the issue data from GraphQL.
//
//nolint:govet // struct field order prioritizes readability over memory layout
type IssueData struct {
	Number    int       `json:"number"`
	Title     string    `json:"title"`
	State     string    `json:"state"`
	URL       string    `json:"url"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
	Body      string    `json:"body"`

	Author struct {
		Login string `json:"login"`
	} `json:"author"`

	Assignees struct {
		Nodes []struct {
			Login string `json:"login"`
		} `json:"nodes"`
	} `json:"assignees"`

	Labels struct {
		Nodes []struct {
			Name string `json:"name"`
		} `json:"nodes"`
	} `json:"labels"`

	Milestone *struct {
		Title string `json:"title"`
	} `json:"milestone"`

	Comments struct {
		TotalCount int `json:"totalCount"`
		Nodes      []struct {
			Author struct {
				Login string `json:"login"`
			} `json:"author"`
			Body      string    `json:"body"`
			CreatedAt time.Time `json:"createdAt"`
		} `json:"nodes"`
	} `json:"comments"`

	Reactions struct {
		TotalCount int `json:"totalCount"`
	} `json:"reactions"`

	ReactionGroups []struct {
		Content string `json:"content"`
		Users   struct {
			TotalCount int `json:"totalCount"`
		} `json:"users"`
	} `json:"reactionGroups"`

	TimelineItems struct {
		Nodes []TimelineNode `json:"nodes"`
	} `json:"timelineItems"`
}

// TimelineNode represents a timeline event which can be various types.
//
//nolint:govet // struct field order prioritizes readability
type TimelineNode struct {
	Typename  string    `json:"__typename"`
	CreatedAt time.Time `json:"createdAt,omitempty"`
	Actor     *struct {
		Login string `json:"login"`
	} `json:"actor,omitempty"`
	Label *struct {
		Name string `json:"name"`
	} `json:"label,omitempty"`
	Source *struct {
		URL string `json:"url,omitempty"`
	} `json:"source,omitempty"`
}

// ParseIssueResponse parses a GraphQL response into Issue metadata map.
func ParseIssueResponse(data json.RawMessage, repo string) (map[string]any, error) {
	var resp IssueGraphQLResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("parse Issue response: %w", err)
	}

	issue := resp.Repository.Issue
	if issue == nil {
		return nil, fmt.Errorf("issue not found")
	}

	return buildIssueMetadata(issue, repo), nil
}

// buildIssueMetadata converts IssueData to the metadata map per EFA 0001.
func buildIssueMetadata(issue *IssueData, repo string) map[string]any {
	metadata := map[string]any{
		"repo":           repo,
		"number":         issue.Number,
		"state":          strings.ToLower(issue.State),
		"author_login":   issue.Author.Login,
		"comments_count": issue.Comments.TotalCount,
		"created_at":     issue.CreatedAt.Format(time.RFC3339),
		"updated_at":     issue.UpdatedAt.Format(time.RFC3339),
	}

	// Truncate body to 500 chars per EFA 0001 (UTF-8 safe)
	metadata["body"] = truncateString(issue.Body, 500)

	// Milestone
	if issue.Milestone != nil {
		metadata["milestone"] = issue.Milestone.Title
	}

	// Assignees
	assignees := make([]string, 0, len(issue.Assignees.Nodes))
	for _, a := range issue.Assignees.Nodes {
		assignees = append(assignees, a.Login)
	}
	metadata["assignees"] = assignees

	// Labels
	labels := make([]string, 0, len(issue.Labels.Nodes))
	for _, l := range issue.Labels.Nodes {
		labels = append(labels, l.Name)
	}
	metadata["labels"] = labels

	// Comments (recent 10, handle ghost users with nil Author)
	comments := make([]map[string]any, 0, len(issue.Comments.Nodes))
	for _, c := range issue.Comments.Nodes {
		authorLogin := ""
		if c.Author.Login != "" {
			authorLogin = c.Author.Login
		}
		comments = append(comments, map[string]any{
			"author":     authorLogin,
			"body":       truncateString(c.Body, 200),
			"created_at": c.CreatedAt.Format(time.RFC3339),
		})
	}
	metadata["comments"] = comments

	// Reactions map
	reactions := make(map[string]int)
	for _, rg := range issue.ReactionGroups {
		if rg.Users.TotalCount > 0 {
			// Convert GitHub reaction names to emoji-style keys
			key := convertReactionContent(rg.Content)
			reactions[key] = rg.Users.TotalCount
		}
	}
	metadata["reactions"] = reactions

	// Timeline summary
	timeline := make([]map[string]any, 0, len(issue.TimelineItems.Nodes))
	for _, t := range issue.TimelineItems.Nodes {
		event := map[string]any{
			"type":       strings.ToLower(strings.TrimSuffix(t.Typename, "Event")),
			"created_at": t.CreatedAt.Format(time.RFC3339),
		}
		if t.Actor != nil {
			event["actor"] = t.Actor.Login
		}
		if t.Label != nil {
			event["label"] = t.Label.Name
		}
		if t.Source != nil && t.Source.URL != "" {
			event["source_url"] = t.Source.URL
		}
		timeline = append(timeline, event)
	}
	metadata["timeline_summary"] = timeline

	return metadata
}

// convertReactionContent converts GitHub reaction content to emoji-style keys.
func convertReactionContent(content string) string {
	// GitHub GraphQL returns uppercase names like "THUMBS_UP"
	switch content {
	case "THUMBS_UP":
		return "+1"
	case "THUMBS_DOWN":
		return "-1"
	case "LAUGH":
		return "laugh"
	case "HOORAY":
		return "hooray"
	case "CONFUSED":
		return "confused"
	case "HEART":
		return "heart"
	case "ROCKET":
		return "rocket"
	case "EYES":
		return "eyes"
	default:
		return strings.ToLower(content)
	}
}
