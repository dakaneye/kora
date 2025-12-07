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

// CalendarAttendee represents an attendee of a calendar event.
// EFA 0001: Fields match the Google Calendar attendees array structure.
type CalendarAttendee struct {
	Email  string `json:"email"`
	Name   string `json:"name"`
	Status string `json:"status"` // "accepted", "tentative", "needsAction", "declined"
}

// CalendarMetadata provides typed access to Google Calendar event metadata.
// EFA 0001: Fields match the Google Calendar metadata table.
//
//nolint:govet // Field order matches EFA 0001 specification, not optimized for alignment
type CalendarMetadata struct {
	CalendarID     string             `json:"calendar_id"`
	EventID        string             `json:"event_id"`
	StartTime      string             `json:"start_time"`
	EndTime        string             `json:"end_time"`
	IsAllDay       bool               `json:"is_all_day"`
	OrganizerEmail string             `json:"organizer_email"`
	OrganizerName  string             `json:"organizer_name"`
	Attendees      []CalendarAttendee `json:"attendees"`
	AttendeeCount  int                `json:"attendee_count"`
	ResponseStatus string             `json:"response_status"`
	PendingRSVPs   []string           `json:"pending_rsvps"`
	Location       string             `json:"location"`
	ConferenceURL  string             `json:"conference_url"`
	Description    string             `json:"description"`
}

// EmailMetadata provides typed access to Gmail event metadata.
// EFA 0001: Fields match the Gmail metadata table.
//
//nolint:govet // Field order matches EFA 0001 specification, not optimized for alignment
type EmailMetadata struct {
	MessageID      string   `json:"message_id"`
	ThreadID       string   `json:"thread_id"`
	FromEmail      string   `json:"from_email"`
	FromName       string   `json:"from_name"`
	ToAddresses    []string `json:"to_addresses"`
	CCAddresses    []string `json:"cc_addresses"`
	Subject        string   `json:"subject"`
	Snippet        string   `json:"snippet"`
	Labels         []string `json:"labels"`
	IsUnread       bool     `json:"is_unread"`
	IsImportant    bool     `json:"is_important"`
	IsStarred      bool     `json:"is_starred"`
	HasAttachments bool     `json:"has_attachments"`
	ReceivedAt     string   `json:"received_at"`
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

// AsCalendarMetadata extracts typed Calendar metadata from an event.
// Returns an error if the event is not a Google Calendar event or if parsing fails.
func (e *Event) AsCalendarMetadata() (*CalendarMetadata, error) {
	if e.Source != SourceGoogleCalendar {
		return nil, fmt.Errorf("AsCalendarMetadata: event source is %s, not google_calendar", e.Source)
	}
	if !e.IsCalendarEvent() {
		return nil, fmt.Errorf("AsCalendarMetadata: event type %s is not a calendar event", e.Type)
	}

	m := &CalendarMetadata{}

	// Extract simple fields
	m.CalendarID = getString(e.Metadata, "calendar_id")
	m.EventID = getString(e.Metadata, "event_id")
	m.StartTime = getString(e.Metadata, "start_time")
	m.EndTime = getString(e.Metadata, "end_time")
	m.IsAllDay = getBool(e.Metadata, "is_all_day")
	m.OrganizerEmail = getString(e.Metadata, "organizer_email")
	m.OrganizerName = getString(e.Metadata, "organizer_name")
	m.AttendeeCount = getInt(e.Metadata, "attendee_count")
	m.ResponseStatus = getString(e.Metadata, "response_status")
	m.Location = getString(e.Metadata, "location")
	m.ConferenceURL = getString(e.Metadata, "conference_url")
	m.Description = getString(e.Metadata, "description")

	// Extract string slices
	m.PendingRSVPs = getStringSlice(e.Metadata, "pending_rsvps")

	// Extract complex types
	m.Attendees = parseCalendarAttendees(e.Metadata["attendees"])

	return m, nil
}

// AsEmailMetadata extracts typed Email metadata from an event.
// Returns an error if the event is not a Gmail event or if parsing fails.
func (e *Event) AsEmailMetadata() (*EmailMetadata, error) {
	if e.Source != SourceGmail {
		return nil, fmt.Errorf("AsEmailMetadata: event source is %s, not gmail", e.Source)
	}
	if !e.IsEmailEvent() {
		return nil, fmt.Errorf("AsEmailMetadata: event type %s is not an email event", e.Type)
	}

	m := &EmailMetadata{}

	// Extract simple fields
	m.MessageID = getString(e.Metadata, "message_id")
	m.ThreadID = getString(e.Metadata, "thread_id")
	m.FromEmail = getString(e.Metadata, "from_email")
	m.FromName = getString(e.Metadata, "from_name")
	m.Subject = getString(e.Metadata, "subject")
	m.Snippet = getString(e.Metadata, "snippet")
	m.IsUnread = getBool(e.Metadata, "is_unread")
	m.IsImportant = getBool(e.Metadata, "is_important")
	m.IsStarred = getBool(e.Metadata, "is_starred")
	m.HasAttachments = getBool(e.Metadata, "has_attachments")
	m.ReceivedAt = getString(e.Metadata, "received_at")

	// Extract string slices
	m.ToAddresses = getStringSlice(e.Metadata, "to_addresses")
	m.CCAddresses = getStringSlice(e.Metadata, "cc_addresses")
	m.Labels = getStringSlice(e.Metadata, "labels")

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

// IsCalendarEvent returns true if the event type is a Calendar event.
func (e *Event) IsCalendarEvent() bool {
	switch e.Type {
	case EventTypeCalendarUpcoming, EventTypeCalendarNeedsRSVP, EventTypeCalendarOrganizerPending,
		EventTypeCalendarTentative, EventTypeCalendarMeeting, EventTypeCalendarAllDay:
		return true
	}
	return false
}

// IsEmailEvent returns true if the event type is an Email event.
func (e *Event) IsEmailEvent() bool {
	switch e.Type {
	case EventTypeEmailImportant, EventTypeEmailDirect, EventTypeEmailCC:
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

func parseCalendarAttendees(v any) []CalendarAttendee {
	if v == nil {
		return nil
	}
	slice, ok := v.([]any)
	if !ok {
		return nil
	}
	result := make([]CalendarAttendee, 0, len(slice))
	for _, item := range slice {
		if m, ok := item.(map[string]any); ok {
			a := CalendarAttendee{
				Email:  getString(m, "email"),
				Name:   getString(m, "name"),
				Status: getString(m, "status"),
			}
			result = append(result, a)
		}
	}
	return result
}
