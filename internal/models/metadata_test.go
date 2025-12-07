package models

import (
	"testing"
	"time"
)

func TestAsPRMetadata(t *testing.T) {
	//nolint:govet // test struct field order prioritizes readability
	tests := []struct {
		name    string
		event   Event
		wantErr bool
		check   func(*testing.T, *PRMetadata)
	}{
		{
			name: "valid PR event with rich metadata",
			event: Event{
				Type:      EventTypePRReview,
				Title:     "Test PR",
				Source:    SourceGitHub,
				URL:       "https://github.com/org/repo/pull/123",
				Author:    Person{Username: "author"},
				Timestamp: time.Now(),
				Priority:  PriorityHigh,
				Metadata: map[string]any{
					"repo":               "org/repo",
					"number":             123,
					"state":              "open",
					"author_login":       "janedev",
					"assignees":          []string{"janedev"},
					"user_relationships": []string{"reviewer"},
					"review_requests": []any{
						map[string]any{"login": "currentuser", "type": "user"},
						map[string]any{"login": "team-name", "type": "team", "team_slug": "org/team-name"},
					},
					"reviews": []any{
						map[string]any{"author": "reviewer1", "state": "approved"},
						map[string]any{"author": "reviewer2", "state": "changes_requested"},
					},
					"ci_checks": []any{
						map[string]any{"name": "build", "status": "completed", "conclusion": "success"},
						map[string]any{"name": "test", "status": "completed", "conclusion": "failure"},
					},
					"ci_rollup":           "failure",
					"files_changed_count": 5,
					"additions":           100,
					"deletions":           50,
					"is_draft":            false,
					"unresolved_threads":  2,
					"mergeable":           "mergeable",
					"labels":              []string{"bug", "priority"},
				},
			},
			check: func(t *testing.T, m *PRMetadata) {
				if m.Repo != "org/repo" {
					t.Errorf("Repo = %q, want %q", m.Repo, "org/repo")
				}
				if m.Number != 123 {
					t.Errorf("Number = %d, want %d", m.Number, 123)
				}
				if m.AuthorLogin != "janedev" {
					t.Errorf("AuthorLogin = %q, want %q", m.AuthorLogin, "janedev")
				}
				if len(m.ReviewRequests) != 2 {
					t.Errorf("ReviewRequests len = %d, want 2", len(m.ReviewRequests))
				}
				if m.ReviewRequests[0].Type != "user" {
					t.Errorf("ReviewRequests[0].Type = %q, want %q", m.ReviewRequests[0].Type, "user")
				}
				if m.ReviewRequests[1].TeamSlug != "org/team-name" {
					t.Errorf("ReviewRequests[1].TeamSlug = %q, want %q", m.ReviewRequests[1].TeamSlug, "org/team-name")
				}
				if len(m.Reviews) != 2 {
					t.Errorf("Reviews len = %d, want 2", len(m.Reviews))
				}
				if m.CIRollup != "failure" {
					t.Errorf("CIRollup = %q, want %q", m.CIRollup, "failure")
				}
				if len(m.CIChecks) != 2 {
					t.Errorf("CIChecks len = %d, want 2", len(m.CIChecks))
				}
				if m.FilesChangedCount != 5 {
					t.Errorf("FilesChangedCount = %d, want 5", m.FilesChangedCount)
				}
				if m.UnresolvedThreads != 2 {
					t.Errorf("UnresolvedThreads = %d, want 2", m.UnresolvedThreads)
				}
				if len(m.Labels) != 2 {
					t.Errorf("Labels len = %d, want 2", len(m.Labels))
				}
			},
		},
		{
			name: "PR author event",
			event: Event{
				Type:      EventTypePRAuthor,
				Title:     "My PR",
				Source:    SourceGitHub,
				URL:       "https://github.com/org/repo/pull/456",
				Author:    Person{Username: "me"},
				Timestamp: time.Now(),
				Priority:  PriorityCritical,
				Metadata: map[string]any{
					"repo":               "org/repo",
					"number":             456,
					"user_relationships": []any{"author"},
					"ci_rollup":          "failure",
				},
			},
			check: func(t *testing.T, m *PRMetadata) {
				if m.Number != 456 {
					t.Errorf("Number = %d, want 456", m.Number)
				}
				if m.CIRollup != "failure" {
					t.Errorf("CIRollup = %q, want %q", m.CIRollup, "failure")
				}
				if len(m.UserRelationships) != 1 || m.UserRelationships[0] != "author" {
					t.Errorf("UserRelationships = %v, want [author]", m.UserRelationships)
				}
			},
		},
		{
			name: "non-GitHub event returns error",
			event: Event{
				Type:      EventTypeSlackDM,
				Source:    SourceSlack,
				Author:    Person{Username: "user"},
				Timestamp: time.Now(),
			},
			wantErr: true,
		},
		{
			name: "issue event returns error",
			event: Event{
				Type:      EventTypeIssueAssigned,
				Source:    SourceGitHub,
				Author:    Person{Username: "user"},
				Timestamp: time.Now(),
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, err := tt.event.AsPRMetadata()
			if (err != nil) != tt.wantErr {
				t.Errorf("AsPRMetadata() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err == nil && tt.check != nil {
				tt.check(t, m)
			}
		})
	}
}

func TestAsIssueMetadata(t *testing.T) {
	//nolint:govet // test struct field order prioritizes readability
	tests := []struct {
		name    string
		event   Event
		wantErr bool
		check   func(*testing.T, *IssueMetadata)
	}{
		{
			name: "valid issue assigned event with rich metadata",
			event: Event{
				Type:      EventTypeIssueAssigned,
				Title:     "Test Issue",
				Source:    SourceGitHub,
				URL:       "https://github.com/org/repo/issues/789",
				Author:    Person{Username: "author"},
				Timestamp: time.Now(),
				Priority:  PriorityMedium,
				Metadata: map[string]any{
					"repo":               "org/repo",
					"number":             789,
					"state":              "open",
					"author_login":       "salesrep",
					"assignees":          []string{"currentuser"},
					"user_relationships": []any{"assignee"},
					"labels":             []string{"bug", "customer"},
					"milestone":          "Q4-2025",
					"body":               "Issue description here...",
					"comments": []any{
						map[string]any{"author": "pm", "body": "Please prioritize", "created_at": "2025-12-06T05:00:00Z"},
					},
					"comments_count": 1,
					"reactions":      map[string]any{"+1": 3, "heart": 1},
					"timeline_summary": []any{
						map[string]any{"type": "assigned", "actor": "pm", "created_at": "2025-12-06T04:00:00Z"},
					},
				},
			},
			check: func(t *testing.T, m *IssueMetadata) {
				if m.Repo != "org/repo" {
					t.Errorf("Repo = %q, want %q", m.Repo, "org/repo")
				}
				if m.Number != 789 {
					t.Errorf("Number = %d, want 789", m.Number)
				}
				if m.Milestone != "Q4-2025" {
					t.Errorf("Milestone = %q, want %q", m.Milestone, "Q4-2025")
				}
				if len(m.Comments) != 1 {
					t.Errorf("Comments len = %d, want 1", len(m.Comments))
				}
				if m.Comments[0].Author != "pm" {
					t.Errorf("Comments[0].Author = %q, want %q", m.Comments[0].Author, "pm")
				}
				if m.Reactions["+1"] != 3 {
					t.Errorf("Reactions[+1] = %d, want 3", m.Reactions["+1"])
				}
				if len(m.TimelineSummary) != 1 {
					t.Errorf("TimelineSummary len = %d, want 1", len(m.TimelineSummary))
				}
			},
		},
		{
			name: "issue mention event",
			event: Event{
				Type:      EventTypeIssueMention,
				Title:     "Mentioned in issue",
				Source:    SourceGitHub,
				URL:       "https://github.com/org/repo/issues/100",
				Author:    Person{Username: "author"},
				Timestamp: time.Now(),
				Priority:  PriorityMedium,
				Metadata: map[string]any{
					"repo":               "org/repo",
					"number":             100,
					"user_relationships": []any{"mentioned"},
				},
			},
			check: func(t *testing.T, m *IssueMetadata) {
				if m.Number != 100 {
					t.Errorf("Number = %d, want 100", m.Number)
				}
				if len(m.UserRelationships) != 1 || m.UserRelationships[0] != "mentioned" {
					t.Errorf("UserRelationships = %v, want [mentioned]", m.UserRelationships)
				}
			},
		},
		{
			name: "non-GitHub event returns error",
			event: Event{
				Type:      EventTypeSlackDM,
				Source:    SourceSlack,
				Author:    Person{Username: "user"},
				Timestamp: time.Now(),
			},
			wantErr: true,
		},
		{
			name: "PR event returns error",
			event: Event{
				Type:      EventTypePRReview,
				Source:    SourceGitHub,
				Author:    Person{Username: "user"},
				Timestamp: time.Now(),
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, err := tt.event.AsIssueMetadata()
			if (err != nil) != tt.wantErr {
				t.Errorf("AsIssueMetadata() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err == nil && tt.check != nil {
				tt.check(t, m)
			}
		})
	}
}

func TestIsPREvent(t *testing.T) {
	tests := []struct {
		eventType EventType
		want      bool
	}{
		{EventTypePRReview, true},
		{EventTypePRMention, true},
		{EventTypePRAuthor, true},
		{EventTypePRCodeowner, true},
		{EventTypeIssueMention, false},
		{EventTypeIssueAssigned, false},
		{EventTypeSlackDM, false},
		{EventTypeSlackMention, false},
	}

	for _, tt := range tests {
		t.Run(string(tt.eventType), func(t *testing.T) {
			e := Event{Type: tt.eventType}
			if got := e.IsPREvent(); got != tt.want {
				t.Errorf("IsPREvent() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsIssueEvent(t *testing.T) {
	tests := []struct {
		eventType EventType
		want      bool
	}{
		{EventTypePRReview, false},
		{EventTypePRMention, false},
		{EventTypePRAuthor, false},
		{EventTypePRCodeowner, false},
		{EventTypeIssueMention, true},
		{EventTypeIssueAssigned, true},
		{EventTypeSlackDM, false},
		{EventTypeSlackMention, false},
	}

	for _, tt := range tests {
		t.Run(string(tt.eventType), func(t *testing.T) {
			e := Event{Type: tt.eventType}
			if got := e.IsIssueEvent(); got != tt.want {
				t.Errorf("IsIssueEvent() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPRMetadataHelpers(t *testing.T) {
	t.Run("HasDirectReviewRequest", func(t *testing.T) {
		m := &PRMetadata{
			ReviewRequests: []ReviewRequest{
				{Login: "user1", Type: "user"},
				{Login: "team1", Type: "team"},
			},
		}
		if !m.HasDirectReviewRequest("user1") {
			t.Error("HasDirectReviewRequest(user1) = false, want true")
		}
		if m.HasDirectReviewRequest("user2") {
			t.Error("HasDirectReviewRequest(user2) = true, want false")
		}
		if m.HasDirectReviewRequest("team1") {
			t.Error("HasDirectReviewRequest(team1) = true, want false (team request)")
		}
	})

	t.Run("HasTeamReviewRequest", func(t *testing.T) {
		m1 := &PRMetadata{
			ReviewRequests: []ReviewRequest{
				{Login: "user1", Type: "user"},
			},
		}
		if m1.HasTeamReviewRequest() {
			t.Error("HasTeamReviewRequest() = true, want false")
		}

		m2 := &PRMetadata{
			ReviewRequests: []ReviewRequest{
				{Login: "team1", Type: "team"},
			},
		}
		if !m2.HasTeamReviewRequest() {
			t.Error("HasTeamReviewRequest() = false, want true")
		}
	})

	t.Run("HasChangesRequested", func(t *testing.T) {
		m1 := &PRMetadata{
			Reviews: []Review{
				{Author: "r1", State: "approved"},
			},
		}
		if m1.HasChangesRequested() {
			t.Error("HasChangesRequested() = true, want false")
		}

		m2 := &PRMetadata{
			Reviews: []Review{
				{Author: "r1", State: "approved"},
				{Author: "r2", State: "changes_requested"},
			},
		}
		if !m2.HasChangesRequested() {
			t.Error("HasChangesRequested() = false, want true")
		}
	})

	t.Run("IsApproved", func(t *testing.T) {
		tests := []struct {
			name    string
			reviews []Review
			want    bool
		}{
			{
				name:    "no reviews",
				reviews: nil,
				want:    false,
			},
			{
				name: "only comments",
				reviews: []Review{
					{Author: "r1", State: "commented"},
				},
				want: false,
			},
			{
				name: "approved",
				reviews: []Review{
					{Author: "r1", State: "approved"},
				},
				want: true,
			},
			{
				name: "approved then changes requested",
				reviews: []Review{
					{Author: "r1", State: "approved"},
					{Author: "r2", State: "changes_requested"},
				},
				want: false,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				m := &PRMetadata{Reviews: tt.reviews}
				if got := m.IsApproved(); got != tt.want {
					t.Errorf("IsApproved() = %v, want %v", got, tt.want)
				}
			})
		}
	})
}

func TestHasUserRelationship(t *testing.T) {
	e := Event{
		Metadata: map[string]any{
			"user_relationships": []any{"author", "reviewer"},
		},
	}

	if !e.HasUserRelationship("author") {
		t.Error("HasUserRelationship(author) = false, want true")
	}
	if !e.HasUserRelationship("reviewer") {
		t.Error("HasUserRelationship(reviewer) = false, want true")
	}
	if e.HasUserRelationship("mentioned") {
		t.Error("HasUserRelationship(mentioned) = true, want false")
	}
}
