// Package github provides GraphQL response parsing for rich PR context.
// Ground truth defined in specs/efas/0001-event-model.md (metadata fields).
package github

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// PRGraphQLResponse is the top-level response from PRQuery.
type PRGraphQLResponse struct {
	Repository struct {
		PullRequest *PRData `json:"pullRequest"`
	} `json:"repository"`
}

// PRData represents the pull request data from GraphQL.
//
//nolint:govet // struct field order prioritizes readability over memory layout
type PRData struct {
	Number    int       `json:"number"`
	Title     string    `json:"title"`
	State     string    `json:"state"`
	IsDraft   bool      `json:"isDraft"`
	Mergeable string    `json:"mergeable"`
	URL       string    `json:"url"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
	Body      string    `json:"body"`

	HeadRefName string `json:"headRefName"`
	BaseRefName string `json:"baseRefName"`

	Additions    int `json:"additions"`
	Deletions    int `json:"deletions"`
	ChangedFiles int `json:"changedFiles"`

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

	Files struct {
		Nodes []struct {
			Path      string `json:"path"`
			Additions int    `json:"additions"`
			Deletions int    `json:"deletions"`
		} `json:"nodes"`
	} `json:"files"`

	ReviewRequests struct {
		Nodes []struct {
			RequestedReviewer ReviewerNode `json:"requestedReviewer"`
		} `json:"nodes"`
	} `json:"reviewRequests"`

	Reviews struct {
		Nodes []struct {
			Author struct {
				Login string `json:"login"`
			} `json:"author"`
			State string `json:"state"`
		} `json:"nodes"`
	} `json:"reviews"`

	ReviewThreads struct {
		Nodes []struct {
			IsResolved bool `json:"isResolved"`
		} `json:"nodes"`
	} `json:"reviewThreads"`

	Comments struct {
		TotalCount int `json:"totalCount"`
	} `json:"comments"`

	Commits struct {
		Nodes []struct {
			Commit struct {
				StatusCheckRollup *struct {
					State    string `json:"state"`
					Contexts struct {
						Nodes []CheckContext `json:"nodes"`
					} `json:"contexts"`
				} `json:"statusCheckRollup"`
			} `json:"commit"`
		} `json:"nodes"`
	} `json:"commits"`

	ClosingIssuesReferences struct {
		Nodes []struct {
			URL string `json:"url"`
		} `json:"nodes"`
	} `json:"closingIssuesReferences"`
}

// ReviewerNode represents a reviewer which can be User or Team.
//
//nolint:govet // struct field order prioritizes readability
type ReviewerNode struct {
	Typename     string `json:"__typename"`
	Login        string `json:"login,omitempty"`
	Slug         string `json:"slug,omitempty"`
	Organization *struct {
		Login string `json:"login"`
	} `json:"organization,omitempty"`
}

// CheckContext represents a CI check from statusCheckRollup.
type CheckContext struct {
	Typename   string `json:"__typename"`
	Name       string `json:"name,omitempty"`
	Status     string `json:"status,omitempty"`
	Conclusion string `json:"conclusion,omitempty"`
	Context    string `json:"context,omitempty"`
	State      string `json:"state,omitempty"`
}

// ParsePRResponse parses a GraphQL response into PR metadata map.
func ParsePRResponse(data json.RawMessage, repo string) (map[string]any, error) {
	var resp PRGraphQLResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("parse PR response: %w", err)
	}

	pr := resp.Repository.PullRequest
	if pr == nil {
		return nil, fmt.Errorf("pull request not found")
	}

	return buildPRMetadata(pr, repo), nil
}

// buildPRMetadata converts PRData to the metadata map per EFA 0001.
func buildPRMetadata(pr *PRData, repo string) map[string]any {
	metadata := map[string]any{
		"repo":                repo,
		"number":              pr.Number,
		"state":               strings.ToLower(pr.State),
		"author_login":        pr.Author.Login,
		"is_draft":            pr.IsDraft,
		"mergeable":           strings.ToLower(pr.Mergeable),
		"head_ref":            pr.HeadRefName,
		"base_ref":            pr.BaseRefName,
		"additions":           pr.Additions,
		"deletions":           pr.Deletions,
		"files_changed_count": pr.ChangedFiles,
		"comments_count":      pr.Comments.TotalCount,
	}

	// Truncate body to 500 chars per EFA 0001
	body := pr.Body
	if len(body) > 500 {
		body = body[:500]
	}
	metadata["body"] = body

	// Milestone
	if pr.Milestone != nil {
		metadata["milestone"] = pr.Milestone.Title
	}

	// Assignees
	assignees := make([]string, 0, len(pr.Assignees.Nodes))
	for _, a := range pr.Assignees.Nodes {
		assignees = append(assignees, a.Login)
	}
	metadata["assignees"] = assignees

	// Labels
	labels := make([]string, 0, len(pr.Labels.Nodes))
	for _, l := range pr.Labels.Nodes {
		labels = append(labels, l.Name)
	}
	metadata["labels"] = labels

	// Files changed
	files := make([]map[string]any, 0, len(pr.Files.Nodes))
	for _, f := range pr.Files.Nodes {
		files = append(files, map[string]any{
			"path":      f.Path,
			"additions": f.Additions,
			"deletions": f.Deletions,
		})
	}
	metadata["files_changed"] = files

	// Review requests with type differentiation
	reviewRequests := make([]map[string]any, 0, len(pr.ReviewRequests.Nodes))
	for _, rr := range pr.ReviewRequests.Nodes {
		req := map[string]any{}
		switch rr.RequestedReviewer.Typename {
		case "User":
			req["login"] = rr.RequestedReviewer.Login
			req["type"] = "user"
		case "Team":
			req["login"] = rr.RequestedReviewer.Slug
			req["type"] = "team"
			if rr.RequestedReviewer.Organization != nil {
				req["team_slug"] = rr.RequestedReviewer.Organization.Login + "/" + rr.RequestedReviewer.Slug
			}
		}
		if len(req) > 0 {
			reviewRequests = append(reviewRequests, req)
		}
	}
	metadata["review_requests"] = reviewRequests

	// Reviews
	reviews := make([]map[string]any, 0, len(pr.Reviews.Nodes))
	for _, r := range pr.Reviews.Nodes {
		reviews = append(reviews, map[string]any{
			"author": r.Author.Login,
			"state":  strings.ToLower(r.State),
		})
	}
	metadata["reviews"] = reviews

	// Unresolved threads count
	unresolvedCount := 0
	for _, t := range pr.ReviewThreads.Nodes {
		if !t.IsResolved {
			unresolvedCount++
		}
	}
	metadata["unresolved_threads"] = unresolvedCount

	// CI checks and rollup
	if len(pr.Commits.Nodes) > 0 {
		commit := pr.Commits.Nodes[0].Commit
		if commit.StatusCheckRollup != nil {
			metadata["ci_rollup"] = strings.ToLower(commit.StatusCheckRollup.State)

			checks := make([]map[string]any, 0, len(commit.StatusCheckRollup.Contexts.Nodes))
			for _, c := range commit.StatusCheckRollup.Contexts.Nodes {
				check := map[string]any{}
				switch c.Typename {
				case "CheckRun":
					check["name"] = c.Name
					check["status"] = strings.ToLower(c.Status)
					check["conclusion"] = strings.ToLower(c.Conclusion)
				case "StatusContext":
					check["name"] = c.Context
					check["status"] = "completed"
					check["conclusion"] = strings.ToLower(c.State)
				}
				if len(check) > 0 {
					checks = append(checks, check)
				}
			}
			metadata["ci_checks"] = checks
		}
	}

	// Linked issues
	linkedIssues := make([]string, 0, len(pr.ClosingIssuesReferences.Nodes))
	for _, i := range pr.ClosingIssuesReferences.Nodes {
		linkedIssues = append(linkedIssues, i.URL)
	}
	metadata["linked_issues"] = linkedIssues

	return metadata
}
