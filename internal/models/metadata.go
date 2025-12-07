// Package models provides typed accessors for rich event metadata.
// Ground truth defined in specs/efas/0001-event-model.md
package models

import (
	"fmt"
)

// ReviewRequest represents a review request on a PR.
type ReviewRequest struct {
	Login    string `json:"login"`
	Type     string `json:"type"`      // "user" or "team"
	TeamSlug string `json:"team_slug"` // only for team requests
}

// Review represents a PR review.
type Review struct {
	Author string `json:"author"`
	State  string `json:"state"` // "approved", "changes_requested", "commented", "pending"
}

// CICheck represents a CI check status.
type CICheck struct {
	Name       string `json:"name"`
	Status     string `json:"status"`     // "queued", "in_progress", "completed"
	Conclusion string `json:"conclusion"` // "success", "failure", "neutral", etc.
}

// FileChange represents a changed file in a PR.
type FileChange struct {
	Path      string `json:"path"`
	Additions int    `json:"additions"`
	Deletions int    `json:"deletions"`
}

// Comment represents a comment on an issue or PR.
type Comment struct {
	Author    string `json:"author"`
	Body      string `json:"body"`
	CreatedAt string `json:"created_at"`
}

// TimelineEvent represents an activity on an issue.
type TimelineEvent struct {
	Type      string `json:"type"`
	Actor     string `json:"actor"`
	CreatedAt string `json:"created_at"`
}

// PRMetadata provides typed access to PR-specific metadata.
// EFA 0001: Fields match the PR metadata table.
//
//nolint:govet // Field order matches EFA 0001 specification, not optimized for alignment
type PRMetadata struct {
	Repo                string          `json:"repo"`
	Number              int             `json:"number"`
	State               string          `json:"state"`
	AuthorLogin         string          `json:"author_login"`
	Assignees           []string        `json:"assignees"`
	UserRelationships   []string        `json:"user_relationships"`
	ReviewRequests      []ReviewRequest `json:"review_requests"`
	Reviews             []Review        `json:"reviews"`
	CIChecks            []CICheck       `json:"ci_checks"`
	CIRollup            string          `json:"ci_rollup"`
	FilesChanged        []FileChange    `json:"files_changed"`
	FilesChangedCount   int             `json:"files_changed_count"`
	Additions           int             `json:"additions"`
	Deletions           int             `json:"deletions"`
	Labels              []string        `json:"labels"`
	Milestone           string          `json:"milestone"`
	LinkedIssues        []string        `json:"linked_issues"`
	Body                string          `json:"body"`
	CommentsCount       int             `json:"comments_count"`
	ReviewCommentsCount int             `json:"review_comments_count"`
	UnresolvedThreads   int             `json:"unresolved_threads"`
	IsDraft             bool            `json:"is_draft"`
	Mergeable           string          `json:"mergeable"`
	HeadRef             string          `json:"head_ref"`
	BaseRef             string          `json:"base_ref"`
	CreatedAt           string          `json:"created_at"`
	UpdatedAt           string          `json:"updated_at"`
}

// IssueMetadata provides typed access to Issue-specific metadata.
// EFA 0001: Fields match the Issue metadata table.
//
//nolint:govet // Field order matches EFA 0001 specification, not optimized for alignment
type IssueMetadata struct {
	Repo              string          `json:"repo"`
	Number            int             `json:"number"`
	State             string          `json:"state"`
	AuthorLogin       string          `json:"author_login"`
	Assignees         []string        `json:"assignees"`
	UserRelationships []string        `json:"user_relationships"`
	Labels            []string        `json:"labels"`
	Milestone         string          `json:"milestone"`
	Body              string          `json:"body"`
	Comments          []Comment       `json:"comments"`
	CommentsCount     int             `json:"comments_count"`
	LinkedPRs         []string        `json:"linked_prs"`
	Reactions         map[string]int  `json:"reactions"`
	TimelineSummary   []TimelineEvent `json:"timeline_summary"`
	CreatedAt         string          `json:"created_at"`
	UpdatedAt         string          `json:"updated_at"`
}

// AsPRMetadata extracts typed PR metadata from an event.
// Returns an error if the event is not a GitHub PR event or if parsing fails.
func (e *Event) AsPRMetadata() (*PRMetadata, error) {
	if e.Source != SourceGitHub {
		return nil, fmt.Errorf("AsPRMetadata: event source is %s, not github", e.Source)
	}
	if !e.IsPREvent() {
		return nil, fmt.Errorf("AsPRMetadata: event type %s is not a PR event", e.Type)
	}

	m := &PRMetadata{}

	// Extract simple fields
	m.Repo = getString(e.Metadata, "repo")
	m.Number = getInt(e.Metadata, "number")
	m.State = getString(e.Metadata, "state")
	m.AuthorLogin = getString(e.Metadata, "author_login")
	m.CIRollup = getString(e.Metadata, "ci_rollup")
	m.FilesChangedCount = getInt(e.Metadata, "files_changed_count")
	m.Additions = getInt(e.Metadata, "additions")
	m.Deletions = getInt(e.Metadata, "deletions")
	m.Milestone = getString(e.Metadata, "milestone")
	m.Body = getString(e.Metadata, "body")
	m.CommentsCount = getInt(e.Metadata, "comments_count")
	m.ReviewCommentsCount = getInt(e.Metadata, "review_comments_count")
	m.UnresolvedThreads = getInt(e.Metadata, "unresolved_threads")
	m.IsDraft = getBool(e.Metadata, "is_draft")
	m.Mergeable = getString(e.Metadata, "mergeable")
	m.HeadRef = getString(e.Metadata, "head_ref")
	m.BaseRef = getString(e.Metadata, "base_ref")
	m.CreatedAt = getString(e.Metadata, "created_at")
	m.UpdatedAt = getString(e.Metadata, "updated_at")

	// Extract string slices
	m.Assignees = getStringSlice(e.Metadata, "assignees")
	m.UserRelationships = getStringSlice(e.Metadata, "user_relationships")
	m.Labels = getStringSlice(e.Metadata, "labels")
	m.LinkedIssues = getStringSlice(e.Metadata, "linked_issues")

	// Extract complex types
	m.ReviewRequests = parseReviewRequests(e.Metadata["review_requests"])
	m.Reviews = parseReviews(e.Metadata["reviews"])
	m.CIChecks = parseCIChecks(e.Metadata["ci_checks"])
	m.FilesChanged = parseFilesChanged(e.Metadata["files_changed"])

	return m, nil
}

// AsIssueMetadata extracts typed Issue metadata from an event.
// Returns an error if the event is not a GitHub Issue event or if parsing fails.
func (e *Event) AsIssueMetadata() (*IssueMetadata, error) {
	if e.Source != SourceGitHub {
		return nil, fmt.Errorf("AsIssueMetadata: event source is %s, not github", e.Source)
	}
	if !e.IsIssueEvent() {
		return nil, fmt.Errorf("AsIssueMetadata: event type %s is not an issue event", e.Type)
	}

	m := &IssueMetadata{}

	// Extract simple fields
	m.Repo = getString(e.Metadata, "repo")
	m.Number = getInt(e.Metadata, "number")
	m.State = getString(e.Metadata, "state")
	m.AuthorLogin = getString(e.Metadata, "author_login")
	m.Milestone = getString(e.Metadata, "milestone")
	m.Body = getString(e.Metadata, "body")
	m.CommentsCount = getInt(e.Metadata, "comments_count")
	m.CreatedAt = getString(e.Metadata, "created_at")
	m.UpdatedAt = getString(e.Metadata, "updated_at")

	// Extract string slices
	m.Assignees = getStringSlice(e.Metadata, "assignees")
	m.UserRelationships = getStringSlice(e.Metadata, "user_relationships")
	m.Labels = getStringSlice(e.Metadata, "labels")
	m.LinkedPRs = getStringSlice(e.Metadata, "linked_prs")

	// Extract complex types
	m.Comments = parseComments(e.Metadata["comments"])
	m.Reactions = parseReactions(e.Metadata["reactions"])
	m.TimelineSummary = parseTimelineSummary(e.Metadata["timeline_summary"])

	return m, nil
}

// IsPREvent returns true if the event type is a PR-related type.
func (e *Event) IsPREvent() bool {
	switch e.Type {
	case EventTypePRReview, EventTypePRMention, EventTypePRAuthor, EventTypePRCodeowner:
		return true
	}
	return false
}

// IsIssueEvent returns true if the event type is an Issue-related type.
func (e *Event) IsIssueEvent() bool {
	switch e.Type {
	case EventTypeIssueMention, EventTypeIssueAssigned:
		return true
	}
	return false
}

// HasUserRelationship checks if the event has a specific user relationship.
func (e *Event) HasUserRelationship(rel string) bool {
	rels := getStringSlice(e.Metadata, "user_relationships")
	for _, r := range rels {
		if r == rel {
			return true
		}
	}
	return false
}

// HasDirectReviewRequest returns true if the user has a direct (non-team) review request.
func (m *PRMetadata) HasDirectReviewRequest(username string) bool {
	for _, rr := range m.ReviewRequests {
		if rr.Type == "user" && rr.Login == username {
			return true
		}
	}
	return false
}

// HasTeamReviewRequest returns true if the user has a team-based review request.
func (m *PRMetadata) HasTeamReviewRequest() bool {
	for _, rr := range m.ReviewRequests {
		if rr.Type == "team" {
			return true
		}
	}
	return false
}

// HasChangesRequested returns true if any review has requested changes.
func (m *PRMetadata) HasChangesRequested() bool {
	for _, r := range m.Reviews {
		if r.State == "changes_requested" {
			return true
		}
	}
	return false
}

// IsApproved returns true if the PR has been approved without pending change requests.
func (m *PRMetadata) IsApproved() bool {
	hasApproval := false
	for _, r := range m.Reviews {
		if r.State == "changes_requested" {
			return false
		}
		if r.State == "approved" {
			hasApproval = true
		}
	}
	return hasApproval
}

// Helper functions for type-safe extraction from map[string]any

func getString(m map[string]any, key string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func getInt(m map[string]any, key string) int {
	if v, ok := m[key]; ok {
		switch n := v.(type) {
		case int:
			return n
		case int64:
			return int(n)
		case float64:
			return int(n)
		}
	}
	return 0
}

func getBool(m map[string]any, key string) bool {
	if v, ok := m[key]; ok {
		if b, ok := v.(bool); ok {
			return b
		}
	}
	return false
}

func getStringSlice(m map[string]any, key string) []string {
	if v, ok := m[key]; ok {
		switch slice := v.(type) {
		case []string:
			return slice
		case []any:
			result := make([]string, 0, len(slice))
			for _, item := range slice {
				if s, ok := item.(string); ok {
					result = append(result, s)
				}
			}
			return result
		}
	}
	return nil
}

func parseReviewRequests(v any) []ReviewRequest {
	if v == nil {
		return nil
	}
	slice, ok := v.([]any)
	if !ok {
		return nil
	}
	result := make([]ReviewRequest, 0, len(slice))
	for _, item := range slice {
		if m, ok := item.(map[string]any); ok {
			rr := ReviewRequest{
				Login:    getString(m, "login"),
				Type:     getString(m, "type"),
				TeamSlug: getString(m, "team_slug"),
			}
			result = append(result, rr)
		}
	}
	return result
}

func parseReviews(v any) []Review {
	if v == nil {
		return nil
	}
	slice, ok := v.([]any)
	if !ok {
		return nil
	}
	result := make([]Review, 0, len(slice))
	for _, item := range slice {
		if m, ok := item.(map[string]any); ok {
			r := Review{
				Author: getString(m, "author"),
				State:  getString(m, "state"),
			}
			result = append(result, r)
		}
	}
	return result
}

func parseCIChecks(v any) []CICheck {
	if v == nil {
		return nil
	}
	slice, ok := v.([]any)
	if !ok {
		return nil
	}
	result := make([]CICheck, 0, len(slice))
	for _, item := range slice {
		if m, ok := item.(map[string]any); ok {
			c := CICheck{
				Name:       getString(m, "name"),
				Status:     getString(m, "status"),
				Conclusion: getString(m, "conclusion"),
			}
			result = append(result, c)
		}
	}
	return result
}

func parseFilesChanged(v any) []FileChange {
	if v == nil {
		return nil
	}
	slice, ok := v.([]any)
	if !ok {
		return nil
	}
	result := make([]FileChange, 0, len(slice))
	for _, item := range slice {
		if m, ok := item.(map[string]any); ok {
			f := FileChange{
				Path:      getString(m, "path"),
				Additions: getInt(m, "additions"),
				Deletions: getInt(m, "deletions"),
			}
			result = append(result, f)
		}
	}
	return result
}

func parseComments(v any) []Comment {
	if v == nil {
		return nil
	}
	slice, ok := v.([]any)
	if !ok {
		return nil
	}
	result := make([]Comment, 0, len(slice))
	for _, item := range slice {
		if m, ok := item.(map[string]any); ok {
			c := Comment{
				Author:    getString(m, "author"),
				Body:      getString(m, "body"),
				CreatedAt: getString(m, "created_at"),
			}
			result = append(result, c)
		}
	}
	return result
}

func parseReactions(v any) map[string]int {
	if v == nil {
		return nil
	}
	m, ok := v.(map[string]any)
	if !ok {
		return nil
	}
	result := make(map[string]int, len(m))
	for k, val := range m {
		switch n := val.(type) {
		case int:
			result[k] = n
		case int64:
			result[k] = int(n)
		case float64:
			result[k] = int(n)
		}
	}
	return result
}

func parseTimelineSummary(v any) []TimelineEvent {
	if v == nil {
		return nil
	}
	slice, ok := v.([]any)
	if !ok {
		return nil
	}
	result := make([]TimelineEvent, 0, len(slice))
	for _, item := range slice {
		if m, ok := item.(map[string]any); ok {
			te := TimelineEvent{
				Type:      getString(m, "type"),
				Actor:     getString(m, "actor"),
				CreatedAt: getString(m, "created_at"),
			}
			result = append(result, te)
		}
	}
	return result
}
