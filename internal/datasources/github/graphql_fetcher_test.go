package github

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/dakaneye/kora/internal/auth"
	"github.com/dakaneye/kora/internal/models"
)

// TestCalculateReviewRequestPriority verifies priority and user_relationships based on review_requests.
// Per EFA 0001:
//   - Direct user request: Priority 2 (High), relationship "direct_reviewer"
//   - Team-only request: Priority 3 (Medium), relationship "team_reviewer"
//   - Mixed (user + team): Priority 2 (High), relationship "direct_reviewer"
//   - No review requests: Priority 2 (High), relationship "reviewer" (default)
func TestCalculateReviewRequestPriority(t *testing.T) {
	//nolint:govet // test struct field order prioritizes readability
	tests := []struct {
		name             string
		metadata         map[string]any
		expectedPriority models.Priority
		expectedRelation []string
	}{
		{
			name: "user review request only",
			metadata: map[string]any{
				"review_requests": []map[string]any{
					{"login": "currentuser", "type": "user"},
				},
			},
			expectedPriority: models.PriorityHigh,
			expectedRelation: []string{"direct_reviewer"},
		},
		{
			name: "team review request only",
			metadata: map[string]any{
				"review_requests": []map[string]any{
					{"login": "core-team", "type": "team", "team_slug": "org/core-team"},
				},
			},
			expectedPriority: models.PriorityMedium,
			expectedRelation: []string{"team_reviewer"},
		},
		{
			name: "mixed user and team requests - user takes priority",
			metadata: map[string]any{
				"review_requests": []map[string]any{
					{"login": "currentuser", "type": "user"},
					{"login": "core-team", "type": "team", "team_slug": "org/core-team"},
				},
			},
			expectedPriority: models.PriorityHigh,
			expectedRelation: []string{"direct_reviewer"},
		},
		{
			name: "multiple team requests",
			metadata: map[string]any{
				"review_requests": []map[string]any{
					{"login": "team-a", "type": "team", "team_slug": "org/team-a"},
					{"login": "team-b", "type": "team", "team_slug": "org/team-b"},
				},
			},
			expectedPriority: models.PriorityMedium,
			expectedRelation: []string{"team_reviewer"},
		},
		{
			name: "multiple user requests",
			metadata: map[string]any{
				"review_requests": []map[string]any{
					{"login": "user1", "type": "user"},
					{"login": "user2", "type": "user"},
				},
			},
			expectedPriority: models.PriorityHigh,
			expectedRelation: []string{"direct_reviewer"},
		},
		{
			name: "team before user in list - still user priority",
			metadata: map[string]any{
				"review_requests": []map[string]any{
					{"login": "core-team", "type": "team", "team_slug": "org/core-team"},
					{"login": "currentuser", "type": "user"},
				},
			},
			expectedPriority: models.PriorityHigh,
			expectedRelation: []string{"direct_reviewer"},
		},
		{
			name:             "empty review_requests array",
			metadata:         map[string]any{"review_requests": []map[string]any{}},
			expectedPriority: models.PriorityHigh,
			expectedRelation: []string{"reviewer"},
		},
		{
			name:             "no review_requests key",
			metadata:         map[string]any{"repo": "org/repo"},
			expectedPriority: models.PriorityHigh,
			expectedRelation: []string{"reviewer"},
		},
		{
			name:             "nil metadata",
			metadata:         nil,
			expectedPriority: models.PriorityHigh,
			expectedRelation: []string{"reviewer"},
		},
		{
			name:             "review_requests wrong type",
			metadata:         map[string]any{"review_requests": "invalid"},
			expectedPriority: models.PriorityHigh,
			expectedRelation: []string{"reviewer"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			priority, relationships := calculateReviewRequestPriority(tt.metadata)

			if priority != tt.expectedPriority {
				t.Errorf("priority = %d, want %d", priority, tt.expectedPriority)
			}

			if len(relationships) != len(tt.expectedRelation) {
				t.Errorf("relationships = %v, want %v", relationships, tt.expectedRelation)
			} else {
				for i, rel := range relationships {
					if rel != tt.expectedRelation[i] {
						t.Errorf("relationships[%d] = %q, want %q", i, rel, tt.expectedRelation[i])
					}
				}
			}
		})
	}
}

// TestCalculateReviewRequestPriority_MetadataNotModified ensures original metadata is not modified.
func TestCalculateReviewRequestPriority_MetadataNotModified(t *testing.T) {
	metadata := map[string]any{
		"review_requests": []map[string]any{
			{"login": "currentuser", "type": "user"},
		},
		"repo":   "org/repo",
		"number": 123,
	}

	// Get original length
	originalReviewRequests := metadata["review_requests"].([]map[string]any)
	originalLen := len(originalReviewRequests)
	originalType := originalReviewRequests[0]["type"]

	// Call function
	_, _ = calculateReviewRequestPriority(metadata)

	// Verify metadata was not modified
	newReviewRequests := metadata["review_requests"].([]map[string]any)
	if len(newReviewRequests) != originalLen {
		t.Error("calculateReviewRequestPriority modified the review_requests length")
	}
	if newReviewRequests[0]["type"] != originalType {
		t.Error("calculateReviewRequestPriority modified the review_requests content")
	}
}

// TestCalculatePRAuthorPriority verifies priority, requiresAction, and title prefix for authored PRs.
// Per EFA 0001 Priority Assignment Rules:
//   - CI failing: Priority 1 (Critical), RequiresAction=true, "CI failing: "
//   - Changes requested: Priority 2 (High), RequiresAction=true, "Changes requested: "
//   - Has approval(s): Priority 3 (Medium), RequiresAction=false, "Ready to merge: "
//   - Awaiting review: Priority 3 (Medium), RequiresAction=false, "Awaiting review: "
//   - Default: Priority 3 (Medium), RequiresAction=false, "Your PR: "
func TestCalculatePRAuthorPriority(t *testing.T) {
	//nolint:govet // test struct field order prioritizes readability
	tests := []struct {
		name                   string
		metadata               map[string]any
		expectedPriority       models.Priority
		expectedRequiresAction bool
		expectedTitlePrefix    string
	}{
		{
			name: "CI failure - critical priority",
			metadata: map[string]any{
				"ci_rollup": "failure",
				"reviews":   []map[string]any{},
			},
			expectedPriority:       models.PriorityCritical,
			expectedRequiresAction: true,
			expectedTitlePrefix:    "CI failing: ",
		},
		{
			name: "CI error - critical priority",
			metadata: map[string]any{
				"ci_rollup": "error",
			},
			expectedPriority:       models.PriorityCritical,
			expectedRequiresAction: true,
			expectedTitlePrefix:    "CI failing: ",
		},
		{
			name: "CI failure with approval - CI takes priority",
			metadata: map[string]any{
				"ci_rollup": "failure",
				"reviews": []map[string]any{
					{"author": "reviewer1", "state": "approved"},
				},
			},
			expectedPriority:       models.PriorityCritical,
			expectedRequiresAction: true,
			expectedTitlePrefix:    "CI failing: ",
		},
		{
			name: "changes requested - high priority",
			metadata: map[string]any{
				"ci_rollup": "success",
				"reviews": []map[string]any{
					{"author": "reviewer1", "state": "changes_requested"},
				},
			},
			expectedPriority:       models.PriorityHigh,
			expectedRequiresAction: true,
			expectedTitlePrefix:    "Changes requested: ",
		},
		{
			name: "changes requested with partial approval",
			metadata: map[string]any{
				"ci_rollup": "success",
				"reviews": []map[string]any{
					{"author": "reviewer1", "state": "approved"},
					{"author": "reviewer2", "state": "changes_requested"},
				},
			},
			expectedPriority:       models.PriorityHigh,
			expectedRequiresAction: true,
			expectedTitlePrefix:    "Changes requested: ",
		},
		{
			name: "approved - ready to merge",
			metadata: map[string]any{
				"ci_rollup": "success",
				"reviews": []map[string]any{
					{"author": "reviewer1", "state": "approved"},
				},
			},
			expectedPriority:       models.PriorityMedium,
			expectedRequiresAction: false,
			expectedTitlePrefix:    "Ready to merge: ",
		},
		{
			name: "multiple approvals - ready to merge",
			metadata: map[string]any{
				"ci_rollup": "success",
				"reviews": []map[string]any{
					{"author": "reviewer1", "state": "approved"},
					{"author": "reviewer2", "state": "approved"},
				},
			},
			expectedPriority:       models.PriorityMedium,
			expectedRequiresAction: false,
			expectedTitlePrefix:    "Ready to merge: ",
		},
		{
			name: "awaiting review - review requested but no reviews yet",
			metadata: map[string]any{
				"ci_rollup": "success",
				"reviews":   []map[string]any{},
				"review_requests": []map[string]any{
					{"login": "reviewer1", "type": "user"},
				},
			},
			expectedPriority:       models.PriorityMedium,
			expectedRequiresAction: false,
			expectedTitlePrefix:    "Awaiting review: ",
		},
		{
			name: "no reviews - default state",
			metadata: map[string]any{
				"ci_rollup": "success",
			},
			expectedPriority:       models.PriorityMedium,
			expectedRequiresAction: false,
			expectedTitlePrefix:    "Your PR: ",
		},
		{
			name: "commented review only - default state",
			metadata: map[string]any{
				"ci_rollup": "success",
				"reviews": []map[string]any{
					{"author": "reviewer1", "state": "commented"},
				},
			},
			expectedPriority:       models.PriorityMedium,
			expectedRequiresAction: false,
			expectedTitlePrefix:    "Your PR: ",
		},
		{
			name: "pending CI - default state",
			metadata: map[string]any{
				"ci_rollup": "pending",
			},
			expectedPriority:       models.PriorityMedium,
			expectedRequiresAction: false,
			expectedTitlePrefix:    "Your PR: ",
		},
		{
			name:                   "nil metadata - default state",
			metadata:               nil,
			expectedPriority:       models.PriorityMedium,
			expectedRequiresAction: false,
			expectedTitlePrefix:    "Your PR: ",
		},
		{
			name:                   "empty metadata - default state",
			metadata:               map[string]any{},
			expectedPriority:       models.PriorityMedium,
			expectedRequiresAction: false,
			expectedTitlePrefix:    "Your PR: ",
		},
		{
			name: "reviews wrong type - default state",
			metadata: map[string]any{
				"ci_rollup": "success",
				"reviews":   "invalid",
			},
			expectedPriority:       models.PriorityMedium,
			expectedRequiresAction: false,
			expectedTitlePrefix:    "Your PR: ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			priority, requiresAction, titlePrefix := calculatePRAuthorPriority(tt.metadata)

			if priority != tt.expectedPriority {
				t.Errorf("priority = %d, want %d", priority, tt.expectedPriority)
			}

			if requiresAction != tt.expectedRequiresAction {
				t.Errorf("requiresAction = %v, want %v", requiresAction, tt.expectedRequiresAction)
			}

			if titlePrefix != tt.expectedTitlePrefix {
				t.Errorf("titlePrefix = %q, want %q", titlePrefix, tt.expectedTitlePrefix)
			}
		})
	}
}

// TestCalculatePRAuthorPriority_MetadataNotModified ensures original metadata is not modified.
func TestCalculatePRAuthorPriority_MetadataNotModified(t *testing.T) {
	metadata := map[string]any{
		"ci_rollup": "failure",
		"reviews": []map[string]any{
			{"author": "reviewer1", "state": "approved"},
		},
		"repo":   "org/repo",
		"number": 123,
	}

	// Get original values
	originalCIRollup := metadata["ci_rollup"]
	originalReviews := metadata["reviews"].([]map[string]any)
	originalLen := len(originalReviews)

	// Call function
	_, _, _ = calculatePRAuthorPriority(metadata)

	// Verify metadata was not modified
	if metadata["ci_rollup"] != originalCIRollup {
		t.Error("calculatePRAuthorPriority modified ci_rollup")
	}

	newReviews := metadata["reviews"].([]map[string]any)
	if len(newReviews) != originalLen {
		t.Error("calculatePRAuthorPriority modified reviews length")
	}
}

// TestPriorityAssignmentRulesEFA0001 is a comprehensive test that verifies all priority
// assignment rules match EFA 0001 specifications.
//
// Per EFA 0001 Priority Assignment Rules:
//
//	| Condition                           | Priority          | EventType      | RequiresAction |
//	|-------------------------------------|-------------------|----------------|----------------|
//	| PR author + CI failing              | 1 (Critical)      | pr_author      | true           |
//	| PR review requested (direct user)   | 2 (High)          | pr_review      | true           |
//	| PR author + changes requested       | 2 (High)          | pr_author      | true           |
//	| PR review requested (team)          | 3 (Medium)        | pr_review      | true           |
//	| PR author (pending/approved)        | 3 (Medium)        | pr_author      | false          |
//	| @mention in issue/PR                | 3 (Medium)        | *_mention      | false          |
//	| Issue assigned                      | 3 (Medium)        | issue_assigned | true           |
func TestPriorityAssignmentRulesEFA0001(t *testing.T) {
	t.Run("PR Author Priority Rules", func(t *testing.T) {
		// Test 1: CI failing = Critical (1), RequiresAction=true
		t.Run("CI failing is Critical priority with RequiresAction", func(t *testing.T) {
			metadata := map[string]any{"ci_rollup": "failure"}
			priority, requiresAction, _ := calculatePRAuthorPriority(metadata)
			if priority != models.PriorityCritical {
				t.Errorf("CI failing: got priority %d, want %d (Critical)", priority, models.PriorityCritical)
			}
			if !requiresAction {
				t.Error("CI failing: RequiresAction should be true")
			}
		})

		// Test 2: Changes requested = High (2), RequiresAction=true
		t.Run("Changes requested is High priority with RequiresAction", func(t *testing.T) {
			metadata := map[string]any{
				"ci_rollup": "success",
				"reviews": []map[string]any{
					{"author": "reviewer", "state": "changes_requested"},
				},
			}
			priority, requiresAction, _ := calculatePRAuthorPriority(metadata)
			if priority != models.PriorityHigh {
				t.Errorf("Changes requested: got priority %d, want %d (High)", priority, models.PriorityHigh)
			}
			if !requiresAction {
				t.Error("Changes requested: RequiresAction should be true")
			}
		})

		// Test 3: Has approval = Medium (3), RequiresAction=false
		t.Run("Approved is Medium priority without RequiresAction", func(t *testing.T) {
			metadata := map[string]any{
				"ci_rollup": "success",
				"reviews": []map[string]any{
					{"author": "reviewer", "state": "approved"},
				},
			}
			priority, requiresAction, _ := calculatePRAuthorPriority(metadata)
			if priority != models.PriorityMedium {
				t.Errorf("Approved: got priority %d, want %d (Medium)", priority, models.PriorityMedium)
			}
			if requiresAction {
				t.Error("Approved: RequiresAction should be false")
			}
		})

		// Test 4: Pending (no reviews) = Medium (3), RequiresAction=false
		t.Run("Pending is Medium priority without RequiresAction", func(t *testing.T) {
			metadata := map[string]any{
				"ci_rollup": "pending",
			}
			priority, requiresAction, _ := calculatePRAuthorPriority(metadata)
			if priority != models.PriorityMedium {
				t.Errorf("Pending: got priority %d, want %d (Medium)", priority, models.PriorityMedium)
			}
			if requiresAction {
				t.Error("Pending: RequiresAction should be false")
			}
		})
	})

	t.Run("PR Review Request Priority Rules", func(t *testing.T) {
		// Test 1: Direct user request = High (2)
		t.Run("Direct user request is High priority", func(t *testing.T) {
			metadata := map[string]any{
				"review_requests": []map[string]any{
					{"login": "user", "type": "user"},
				},
			}
			priority, _ := calculateReviewRequestPriority(metadata)
			if priority != models.PriorityHigh {
				t.Errorf("Direct user request: got priority %d, want %d (High)", priority, models.PriorityHigh)
			}
		})

		// Test 2: Team request = Medium (3)
		t.Run("Team request is Medium priority", func(t *testing.T) {
			metadata := map[string]any{
				"review_requests": []map[string]any{
					{"login": "team", "type": "team"},
				},
			}
			priority, _ := calculateReviewRequestPriority(metadata)
			if priority != models.PriorityMedium {
				t.Errorf("Team request: got priority %d, want %d (Medium)", priority, models.PriorityMedium)
			}
		})

		// Test 3: Mixed user and team = High (2) - user takes precedence
		t.Run("Mixed request prioritizes user (High priority)", func(t *testing.T) {
			metadata := map[string]any{
				"review_requests": []map[string]any{
					{"login": "team", "type": "team"},
					{"login": "user", "type": "user"},
				},
			}
			priority, _ := calculateReviewRequestPriority(metadata)
			if priority != models.PriorityHigh {
				t.Errorf("Mixed request: got priority %d, want %d (High)", priority, models.PriorityHigh)
			}
		})
	})

	t.Run("Priority Hierarchy", func(t *testing.T) {
		// Verify CI failure takes priority over changes requested
		t.Run("CI failure trumps changes requested", func(t *testing.T) {
			metadata := map[string]any{
				"ci_rollup": "failure",
				"reviews": []map[string]any{
					{"author": "reviewer", "state": "changes_requested"},
				},
			}
			priority, _, _ := calculatePRAuthorPriority(metadata)
			if priority != models.PriorityCritical {
				t.Errorf("CI failure with changes requested: got priority %d, want %d (Critical)", priority, models.PriorityCritical)
			}
		})

		// Verify CI failure takes priority over approval
		t.Run("CI failure trumps approval", func(t *testing.T) {
			metadata := map[string]any{
				"ci_rollup": "failure",
				"reviews": []map[string]any{
					{"author": "reviewer", "state": "approved"},
				},
			}
			priority, _, _ := calculatePRAuthorPriority(metadata)
			if priority != models.PriorityCritical {
				t.Errorf("CI failure with approval: got priority %d, want %d (Critical)", priority, models.PriorityCritical)
			}
		})

		// Verify changes requested takes priority over approval
		t.Run("Changes requested trumps approval", func(t *testing.T) {
			metadata := map[string]any{
				"ci_rollup": "success",
				"reviews": []map[string]any{
					{"author": "reviewer1", "state": "approved"},
					{"author": "reviewer2", "state": "changes_requested"},
				},
			}
			priority, _, _ := calculatePRAuthorPriority(metadata)
			if priority != models.PriorityHigh {
				t.Errorf("Mixed reviews: got priority %d, want %d (High)", priority, models.PriorityHigh)
			}
		})
	})
}

// TestDeduplicationPreservesHigherPriority verifies that when events are deduplicated,
// the higher priority (lower number) event is kept per EFA 0001.
func TestDeduplicationPreservesHigherPriority(t *testing.T) {
	tests := []struct {
		name             string
		firstPriority    models.Priority
		secondPriority   models.Priority
		expectedPriority models.Priority
	}{
		{
			name:             "Critical beats High",
			firstPriority:    models.PriorityCritical,
			secondPriority:   models.PriorityHigh,
			expectedPriority: models.PriorityCritical,
		},
		{
			name:             "High beats Medium",
			firstPriority:    models.PriorityHigh,
			secondPriority:   models.PriorityMedium,
			expectedPriority: models.PriorityHigh,
		},
		{
			name:             "Medium beats Low",
			firstPriority:    models.PriorityMedium,
			secondPriority:   models.PriorityLow,
			expectedPriority: models.PriorityMedium,
		},
		{
			name:             "Later Critical replaces earlier Medium",
			firstPriority:    models.PriorityMedium,
			secondPriority:   models.PriorityCritical,
			expectedPriority: models.PriorityCritical,
		},
		{
			name:             "Same priority keeps first",
			firstPriority:    models.PriorityMedium,
			secondPriority:   models.PriorityMedium,
			expectedPriority: models.PriorityMedium,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			events := []models.Event{
				{
					URL:      "https://github.com/test/repo/1",
					Priority: tt.firstPriority,
					Metadata: map[string]any{"user_relationships": []string{"first"}},
				},
				{
					URL:      "https://github.com/test/repo/1",
					Priority: tt.secondPriority,
					Metadata: map[string]any{"user_relationships": []string{"second"}},
				},
			}

			result := models.DeduplicateEvents(events)

			if len(result) != 1 {
				t.Fatalf("expected 1 event, got %d", len(result))
			}

			if result[0].Priority != tt.expectedPriority {
				t.Errorf("priority = %d, want %d", result[0].Priority, tt.expectedPriority)
			}
		})
	}
}

// TestFetchClosedPRsGraphQL verifies the fetchClosedPRsGraphQL method.
// Per EFA 0001:
//   - EventType: models.EventTypePRClosed
//   - Priority: models.PriorityInfo (5) - informational only
//   - RequiresAction: false
//   - Title prefix: "Merged: " for merged PRs, "Closed: " for closed PRs
//   - user_relationships: ["author"]
func TestFetchClosedPRsGraphQL(t *testing.T) {
	//nolint:govet // test struct field order prioritizes readability
	tests := []struct {
		name           string
		mockSetup      func(*mockGitHubDelegatedCredential)
		since          time.Time
		orgs           []string
		expectedEvents int
		checkEvents    func(*testing.T, []models.Event)
		expectedError  bool
	}{
		{
			name: "merged PR - success",
			mockSetup: func(cred *mockGitHubDelegatedCredential) {
				// Search response: one merged PR
				searchData := []byte(`{
					"data": {
						"search": {
							"issueCount": 1,
							"nodes": [
								{
									"number": 123,
									"title": "Add feature",
									"url": "https://github.com/owner/repo/pull/123",
									"updatedAt": "2025-12-06T10:00:00Z",
									"repository": {"nameWithOwner": "owner/repo"},
									"author": {"login": "testuser"}
								}
							],
							"pageInfo": {"hasNextPage": false, "endCursor": ""}
						}
					}
				}`)
				cred.setGraphQLResponse("graphql:search:pr-author", searchData)

				// Full PR context response with state="merged"
				contextData := []byte(`{
					"data": {
						"repository": {
							"pullRequest": {
								"number": 123,
								"title": "Add feature",
								"state": "MERGED",
								"isDraft": false,
								"mergeable": "UNKNOWN",
								"url": "https://github.com/owner/repo/pull/123",
								"createdAt": "2025-12-05T10:00:00Z",
								"updatedAt": "2025-12-06T10:00:00Z",
								"author": {"login": "testuser"},
								"assignees": {"nodes": []},
								"labels": {"nodes": []},
								"milestone": null,
								"body": "",
								"headRefName": "feature",
								"baseRefName": "main",
								"additions": 10,
								"deletions": 5,
								"changedFiles": 2,
								"files": {"nodes": []},
								"reviewRequests": {"nodes": []},
								"reviews": {"nodes": []},
								"reviewThreads": {"nodes": []},
								"comments": {"totalCount": 0},
								"commits": {"nodes": [{"commit": {"statusCheckRollup": null}}]},
								"closingIssuesReferences": {"nodes": []}
							}
						}
					}
				}`)
				cred.setGraphQLResponse("graphql:pr:context", contextData)
			},
			since:          time.Date(2025, 12, 6, 0, 0, 0, 0, time.UTC),
			orgs:           []string{},
			expectedEvents: 1,
			checkEvents: func(t *testing.T, events []models.Event) {
				if len(events) == 0 {
					t.Fatal("expected events, got none")
				}

				event := events[0]

				// Verify event type
				if event.Type != models.EventTypePRClosed {
					t.Errorf("Type = %q, want %q", event.Type, models.EventTypePRClosed)
				}

				// Verify title has "Merged: " prefix
				if !strings.HasPrefix(event.Title, "Merged: ") {
					t.Errorf("Title = %q, want prefix %q", event.Title, "Merged: ")
				}
				if !contains(event.Title, "Add feature") {
					t.Errorf("Title = %q, want to contain %q", event.Title, "Add feature")
				}

				// Verify priority is Info (5)
				if event.Priority != models.PriorityInfo {
					t.Errorf("Priority = %d, want %d (Info)", event.Priority, models.PriorityInfo)
				}

				// Verify RequiresAction is false
				if event.RequiresAction {
					t.Error("RequiresAction = true, want false")
				}

				// Verify user_relationships includes "author"
				relationships, ok := event.Metadata["user_relationships"].([]string)
				if !ok {
					t.Fatal("user_relationships not found or wrong type")
				}
				if len(relationships) != 1 || relationships[0] != "author" {
					t.Errorf("user_relationships = %v, want [author]", relationships)
				}

				// Verify metadata includes state="merged"
				state, ok := event.Metadata["state"].(string)
				if !ok {
					t.Fatal("state not found in metadata")
				}
				if state != "merged" {
					t.Errorf("state = %q, want %q", state, "merged")
				}

				// Verify event passes validation
				if err := event.Validate(); err != nil {
					t.Errorf("event validation failed: %v", err)
				}
			},
		},
		{
			name: "closed (not merged) PR - success",
			mockSetup: func(cred *mockGitHubDelegatedCredential) {
				// Search response: one closed PR
				searchData := []byte(`{
					"data": {
						"search": {
							"issueCount": 1,
							"nodes": [
								{
									"number": 456,
									"title": "Fix bug",
									"url": "https://github.com/owner/repo/pull/456",
									"updatedAt": "2025-12-06T11:00:00Z",
									"repository": {"nameWithOwner": "owner/repo"},
									"author": {"login": "testuser"}
								}
							],
							"pageInfo": {"hasNextPage": false, "endCursor": ""}
						}
					}
				}`)
				cred.setGraphQLResponse("graphql:search:pr-author", searchData)

				// Full PR context response with state="closed" (not merged)
				contextData := []byte(`{
					"data": {
						"repository": {
							"pullRequest": {
								"number": 456,
								"title": "Fix bug",
								"state": "CLOSED",
								"isDraft": false,
								"mergeable": "UNKNOWN",
								"url": "https://github.com/owner/repo/pull/456",
								"createdAt": "2025-12-05T11:00:00Z",
								"updatedAt": "2025-12-06T11:00:00Z",
								"author": {"login": "testuser"},
								"assignees": {"nodes": []},
								"labels": {"nodes": []},
								"milestone": null,
								"body": "",
								"headRefName": "bugfix",
								"baseRefName": "main",
								"additions": 5,
								"deletions": 2,
								"changedFiles": 1,
								"files": {"nodes": []},
								"reviewRequests": {"nodes": []},
								"reviews": {"nodes": []},
								"reviewThreads": {"nodes": []},
								"comments": {"totalCount": 0},
								"commits": {"nodes": [{"commit": {"statusCheckRollup": null}}]},
								"closingIssuesReferences": {"nodes": []}
							}
						}
					}
				}`)
				cred.setGraphQLResponse("graphql:pr:context", contextData)
			},
			since:          time.Date(2025, 12, 6, 0, 0, 0, 0, time.UTC),
			orgs:           []string{},
			expectedEvents: 1,
			checkEvents: func(t *testing.T, events []models.Event) {
				if len(events) == 0 {
					t.Fatal("expected events, got none")
				}

				event := events[0]

				// Verify event type
				if event.Type != models.EventTypePRClosed {
					t.Errorf("Type = %q, want %q", event.Type, models.EventTypePRClosed)
				}

				// Verify title has "Closed: " prefix (not "Merged: ")
				if !strings.HasPrefix(event.Title, "Closed: ") {
					t.Errorf("Title = %q, want prefix %q", event.Title, "Closed: ")
				}
				if !contains(event.Title, "Fix bug") {
					t.Errorf("Title = %q, want to contain %q", event.Title, "Fix bug")
				}

				// Verify priority is Info (5)
				if event.Priority != models.PriorityInfo {
					t.Errorf("Priority = %d, want %d (Info)", event.Priority, models.PriorityInfo)
				}

				// Verify RequiresAction is false
				if event.RequiresAction {
					t.Error("RequiresAction = true, want false")
				}

				// Verify metadata includes state="closed"
				state, ok := event.Metadata["state"].(string)
				if !ok {
					t.Fatal("state not found in metadata")
				}
				if state != "closed" {
					t.Errorf("state = %q, want %q", state, "closed")
				}

				// Verify event passes validation
				if err := event.Validate(); err != nil {
					t.Errorf("event validation failed: %v", err)
				}
			},
		},
		{
			name: "empty results - no closed PRs",
			mockSetup: func(cred *mockGitHubDelegatedCredential) {
				// Search response: empty results
				searchData := []byte(`{
					"data": {
						"search": {
							"issueCount": 0,
							"nodes": [],
							"pageInfo": {"hasNextPage": false, "endCursor": ""}
						}
					}
				}`)
				cred.setGraphQLResponse("graphql:search:pr-author", searchData)
			},
			since:          time.Date(2025, 12, 6, 0, 0, 0, 0, time.UTC),
			orgs:           []string{},
			expectedEvents: 0,
			checkEvents: func(t *testing.T, events []models.Event) {
				if len(events) != 0 {
					t.Errorf("expected no events, got %d", len(events))
				}
			},
		},
		{
			name: "multiple merged PRs",
			mockSetup: func(cred *mockGitHubDelegatedCredential) {
				// Search response: two merged PRs
				searchData := []byte(`{
					"data": {
						"search": {
							"issueCount": 2,
							"nodes": [
								{
									"number": 100,
									"title": "First PR",
									"url": "https://github.com/owner/repo/pull/100",
									"updatedAt": "2025-12-06T10:00:00Z",
									"repository": {"nameWithOwner": "owner/repo"},
									"author": {"login": "testuser"}
								},
								{
									"number": 101,
									"title": "Second PR",
									"url": "https://github.com/owner/repo/pull/101",
									"updatedAt": "2025-12-06T11:00:00Z",
									"repository": {"nameWithOwner": "owner/repo"},
									"author": {"login": "testuser"}
								}
							],
							"pageInfo": {"hasNextPage": false, "endCursor": ""}
						}
					}
				}`)
				cred.setGraphQLResponse("graphql:search:pr-author", searchData)

				// Full PR context response - merged state
				contextData := []byte(`{
					"data": {
						"repository": {
							"pullRequest": {
								"number": 100,
								"title": "First PR",
								"state": "MERGED",
								"isDraft": false,
								"mergeable": "UNKNOWN",
								"url": "https://github.com/owner/repo/pull/100",
								"createdAt": "2025-12-05T10:00:00Z",
								"updatedAt": "2025-12-06T10:00:00Z",
								"author": {"login": "testuser"},
								"assignees": {"nodes": []},
								"labels": {"nodes": []},
								"milestone": null,
								"body": "",
								"headRefName": "feature",
								"baseRefName": "main",
								"additions": 10,
								"deletions": 5,
								"changedFiles": 2,
								"files": {"nodes": []},
								"reviewRequests": {"nodes": []},
								"reviews": {"nodes": []},
								"reviewThreads": {"nodes": []},
								"comments": {"totalCount": 0},
								"commits": {"nodes": [{"commit": {"statusCheckRollup": null}}]},
								"closingIssuesReferences": {"nodes": []}
							}
						}
					}
				}`)
				cred.setGraphQLResponse("graphql:pr:context", contextData)
			},
			since:          time.Date(2025, 12, 6, 0, 0, 0, 0, time.UTC),
			orgs:           []string{},
			expectedEvents: 2,
			checkEvents: func(t *testing.T, events []models.Event) {
				if len(events) != 2 {
					t.Fatalf("expected 2 events, got %d", len(events))
				}

				// Verify all events are EventTypePRClosed with correct priority
				for i, event := range events {
					if event.Type != models.EventTypePRClosed {
						t.Errorf("event[%d].Type = %q, want %q", i, event.Type, models.EventTypePRClosed)
					}
					if event.Priority != models.PriorityInfo {
						t.Errorf("event[%d].Priority = %d, want %d", i, event.Priority, models.PriorityInfo)
					}
					if event.RequiresAction {
						t.Errorf("event[%d].RequiresAction = true, want false", i)
					}
					if !strings.HasPrefix(event.Title, "Merged: ") {
						t.Errorf("event[%d].Title = %q, want 'Merged: ' prefix", i, event.Title)
					}
				}
			},
		},
		{
			name: "with org filter",
			mockSetup: func(cred *mockGitHubDelegatedCredential) {
				// Search response: one PR from specified org
				searchData := []byte(`{
					"data": {
						"search": {
							"issueCount": 1,
							"nodes": [
								{
									"number": 200,
									"title": "Org-specific PR",
									"url": "https://github.com/myorg/repo/pull/200",
									"updatedAt": "2025-12-06T12:00:00Z",
									"repository": {"nameWithOwner": "myorg/repo"},
									"author": {"login": "testuser"}
								}
							],
							"pageInfo": {"hasNextPage": false, "endCursor": ""}
						}
					}
				}`)
				cred.setGraphQLResponse("graphql:search:pr-author", searchData)

				// Full PR context response
				contextData := []byte(`{
					"data": {
						"repository": {
							"pullRequest": {
								"number": 200,
								"title": "Org-specific PR",
								"state": "MERGED",
								"isDraft": false,
								"mergeable": "UNKNOWN",
								"url": "https://github.com/myorg/repo/pull/200",
								"createdAt": "2025-12-05T12:00:00Z",
								"updatedAt": "2025-12-06T12:00:00Z",
								"author": {"login": "testuser"},
								"assignees": {"nodes": []},
								"labels": {"nodes": []},
								"milestone": null,
								"body": "",
								"headRefName": "feature",
								"baseRefName": "main",
								"additions": 10,
								"deletions": 5,
								"changedFiles": 2,
								"files": {"nodes": []},
								"reviewRequests": {"nodes": []},
								"reviews": {"nodes": []},
								"reviewThreads": {"nodes": []},
								"comments": {"totalCount": 0},
								"commits": {"nodes": [{"commit": {"statusCheckRollup": null}}]},
								"closingIssuesReferences": {"nodes": []}
							}
						}
					}
				}`)
				cred.setGraphQLResponse("graphql:pr:context", contextData)
			},
			since:          time.Date(2025, 12, 6, 0, 0, 0, 0, time.UTC),
			orgs:           []string{"myorg"},
			expectedEvents: 1,
			checkEvents: func(t *testing.T, events []models.Event) {
				if len(events) != 1 {
					t.Fatalf("expected 1 event, got %d", len(events))
				}

				event := events[0]
				if !contains(event.URL, "myorg") {
					t.Errorf("URL = %q, want to contain 'myorg'", event.URL)
				}
			},
		},
		{
			name: "context fetch fails - uses partial metadata",
			mockSetup: func(cred *mockGitHubDelegatedCredential) {
				// Search response: one PR
				searchData := []byte(`{
					"data": {
						"search": {
							"issueCount": 1,
							"nodes": [
								{
									"number": 300,
									"title": "PR with context error",
									"url": "https://github.com/owner/repo/pull/300",
									"updatedAt": "2025-12-06T13:00:00Z",
									"repository": {"nameWithOwner": "owner/repo"},
									"author": {"login": "testuser"}
								}
							],
							"pageInfo": {"hasNextPage": false, "endCursor": ""}
						}
					}
				}`)
				cred.setGraphQLResponse("graphql:search:pr-author", searchData)

				// Context fetch returns error
				cred.setGraphQLError("graphql:pr:context", fmt.Errorf("context fetch failed"))
			},
			since:          time.Date(2025, 12, 6, 0, 0, 0, 0, time.UTC),
			orgs:           []string{},
			expectedEvents: 1,
			checkEvents: func(t *testing.T, events []models.Event) {
				if len(events) != 1 {
					t.Fatalf("expected 1 event, got %d", len(events))
				}

				event := events[0]

				// Should still create event with partial metadata
				if event.Type != models.EventTypePRClosed {
					t.Errorf("Type = %q, want %q", event.Type, models.EventTypePRClosed)
				}

				// Should have "Closed: " prefix (default when state unknown)
				if !strings.HasPrefix(event.Title, "Closed: ") {
					t.Errorf("Title = %q, want 'Closed: ' prefix", event.Title)
				}

				// Verify basic metadata from search results
				repo, ok := event.Metadata["repo"].(string)
				if !ok || repo != "owner/repo" {
					t.Errorf("repo = %v, want 'owner/repo'", event.Metadata["repo"])
				}

				number, ok := event.Metadata["number"].(int)
				if !ok || number != 300 {
					t.Errorf("number = %v, want 300", event.Metadata["number"])
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup mock auth provider
			authProvider := newMockAuthProvider(auth.ServiceGitHub)
			tt.mockSetup(authProvider.credential)

			// Create datasource
			ds, err := NewDataSource(authProvider)
			if err != nil {
				t.Fatalf("NewDataSource failed: %v", err)
			}

			// Create GraphQL client
			gqlClient := NewGraphQLClient(authProvider.credential)

			// Call method
			events, err := ds.fetchClosedPRsGraphQL(
				context.Background(),
				gqlClient,
				authProvider.credential,
				tt.since,
				tt.orgs,
			)

			// Check error expectation
			if tt.expectedError && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.expectedError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			// Check event count
			if len(events) != tt.expectedEvents {
				t.Errorf("expected %d events, got %d", tt.expectedEvents, len(events))
			}

			// Run custom checks
			if tt.checkEvents != nil {
				tt.checkEvents(t, events)
			}
		})
	}
}

// TestCheckCommentMentions verifies the checkCommentMentions helper function.
// This helper scans PR comments and reviews for @mentions of the specified username.
// Mention detection is case-insensitive and requires @ prefix.
func TestCheckCommentMentions(t *testing.T) {
	//nolint:govet // test struct field order prioritizes readability
	tests := []struct {
		name     string
		metadata map[string]any
		username string
		want     bool
	}{
		{
			name: "mention in review body",
			metadata: map[string]any{
				"reviews": []map[string]any{
					{"author": "reviewer", "body": "Hey @testuser can you check this?"},
				},
			},
			username: "testuser",
			want:     true,
		},
		{
			name: "mention in issue comment",
			metadata: map[string]any{
				"comments": []map[string]any{
					{"author": "commenter", "body": "Thanks @testuser for looking!"},
				},
			},
			username: "testuser",
			want:     true,
		},
		{
			name: "case insensitive - lowercase",
			metadata: map[string]any{
				"reviews": []map[string]any{
					{"author": "reviewer", "body": "Please review @USER"},
				},
			},
			username: "user",
			want:     true,
		},
		{
			name: "case insensitive - uppercase",
			metadata: map[string]any{
				"comments": []map[string]any{
					{"author": "commenter", "body": "CC @USER for review"},
				},
			},
			username: "USER",
			want:     true,
		},
		{
			name: "no mentions - username without @",
			metadata: map[string]any{
				"reviews": []map[string]any{
					{"author": "reviewer", "body": "testuser should look at this"},
				},
			},
			username: "testuser",
			want:     false,
		},
		{
			name:     "empty metadata",
			metadata: map[string]any{},
			username: "testuser",
			want:     false,
		},
		{
			name:     "nil metadata",
			metadata: nil,
			username: "testuser",
			want:     false,
		},
		{
			name: "missing reviews key",
			metadata: map[string]any{
				"repo": "owner/repo",
			},
			username: "testuser",
			want:     false,
		},
		{
			name: "missing comments key",
			metadata: map[string]any{
				"reviews": []map[string]any{},
			},
			username: "testuser",
			want:     false,
		},
		{
			name: "empty username",
			metadata: map[string]any{
				"reviews": []map[string]any{
					{"author": "reviewer", "body": "@someone please review"},
				},
			},
			username: "",
			want:     false,
		},
		{
			name: "mention in both reviews and comments",
			metadata: map[string]any{
				"reviews": []map[string]any{
					{"author": "reviewer", "body": "@testuser what do you think?"},
				},
				"comments": []map[string]any{
					{"author": "commenter", "body": "I agree with @testuser"},
				},
			},
			username: "testuser",
			want:     true,
		},
		{
			name: "no mention in multiple reviews",
			metadata: map[string]any{
				"reviews": []map[string]any{
					{"author": "reviewer1", "body": "LGTM"},
					{"author": "reviewer2", "body": "Approved"},
				},
			},
			username: "testuser",
			want:     false,
		},
		{
			name: "wrong type for reviews - not []map[string]any",
			metadata: map[string]any{
				"reviews": "invalid",
			},
			username: "testuser",
			want:     false,
		},
		{
			name: "wrong type for comments - not []map[string]any",
			metadata: map[string]any{
				"comments": 123,
			},
			username: "testuser",
			want:     false,
		},
		{
			name: "body not a string in review",
			metadata: map[string]any{
				"reviews": []map[string]any{
					{"author": "reviewer", "body": 123},
				},
			},
			username: "testuser",
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := checkCommentMentions(tt.metadata, tt.username)
			if got != tt.want {
				t.Errorf("checkCommentMentions() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestFetchPRCommentMentionsGraphQL verifies the fetchPRCommentMentionsGraphQL method.
// Per EFA 0001:
//   - EventType: models.EventTypePRCommentMention
//   - Priority: models.PriorityMedium (3)
//   - RequiresAction: false
//   - user_relationships: ["mentioned"]
//   - Title prefix: "Mentioned in comment: "
func TestFetchPRCommentMentionsGraphQL(t *testing.T) {
	//nolint:govet // test struct field order prioritizes readability
	tests := []struct {
		name           string
		mockSetup      func(*mockGitHubDelegatedCredential)
		since          time.Time
		orgs           []string
		expectedEvents int
		checkEvents    func(*testing.T, []models.Event)
		expectedError  bool
	}{
		{
			name: "PR with review comment mention",
			mockSetup: func(cred *mockGitHubDelegatedCredential) {
				// Search response: one PR involving the user
				searchData := []byte(`{
					"data": {
						"search": {
							"issueCount": 1,
							"nodes": [
								{
									"number": 123,
									"title": "Add feature",
									"url": "https://github.com/owner/repo/pull/123",
									"updatedAt": "2025-12-06T10:00:00Z",
									"repository": {"nameWithOwner": "owner/repo"},
									"author": {"login": "otheruser"}
								}
							],
							"pageInfo": {"hasNextPage": false, "endCursor": ""}
						}
					}
				}`)
				cred.setGraphQLResponse("graphql:search:pr-involves", searchData)

				// Full PR context response with mention in review
				contextData := []byte(`{
					"data": {
						"repository": {
							"pullRequest": {
								"number": 123,
								"title": "Add feature",
								"state": "OPEN",
								"isDraft": false,
								"mergeable": "MERGEABLE",
								"url": "https://github.com/owner/repo/pull/123",
								"createdAt": "2025-12-05T10:00:00Z",
								"updatedAt": "2025-12-06T10:00:00Z",
								"author": {"login": "otheruser"},
								"assignees": {"nodes": []},
								"labels": {"nodes": []},
								"milestone": null,
								"body": "PR description",
								"headRefName": "feature",
								"baseRefName": "main",
								"additions": 10,
								"deletions": 5,
								"changedFiles": 2,
								"files": {"nodes": []},
								"reviewRequests": {"nodes": []},
								"reviews": {
									"nodes": [
										{"author": {"login": "reviewer"}, "state": "COMMENTED", "body": "Hey @testuser can you check this?", "createdAt": "2025-12-06T09:00:00Z"}
									]
								},
								"reviewThreads": {"nodes": []},
								"comments": {
									"totalCount": 1,
									"nodes": [
										{"author": {"login": "otheruser"}, "body": "Thanks for looking!", "createdAt": "2025-12-06T08:00:00Z", "updatedAt": "2025-12-06T08:00:00Z"}
									]
								},
								"commits": {"nodes": [{"commit": {"statusCheckRollup": null}}]},
								"closingIssuesReferences": {"nodes": []}
							}
						}
					}
				}`)
				cred.setGraphQLResponse("graphql:pr:context", contextData)
			},
			since:          time.Date(2025, 12, 6, 0, 0, 0, 0, time.UTC),
			orgs:           []string{},
			expectedEvents: 1,
			checkEvents: func(t *testing.T, events []models.Event) {
				if len(events) == 0 {
					t.Fatal("expected events, got none")
				}

				event := events[0]

				// Verify event type
				if event.Type != models.EventTypePRCommentMention {
					t.Errorf("Type = %q, want %q", event.Type, models.EventTypePRCommentMention)
				}

				// Verify title has "Mentioned in comment: " prefix
				if !strings.HasPrefix(event.Title, "Mentioned in comment: ") {
					t.Errorf("Title = %q, want prefix %q", event.Title, "Mentioned in comment: ")
				}
				if !contains(event.Title, "Add feature") {
					t.Errorf("Title = %q, want to contain %q", event.Title, "Add feature")
				}

				// Verify priority is Medium (3)
				if event.Priority != models.PriorityMedium {
					t.Errorf("Priority = %d, want %d (Medium)", event.Priority, models.PriorityMedium)
				}

				// Verify RequiresAction is false
				if event.RequiresAction {
					t.Error("RequiresAction = true, want false")
				}

				// Verify user_relationships includes "mentioned"
				relationships, ok := event.Metadata["user_relationships"].([]string)
				if !ok {
					t.Fatal("user_relationships not found or wrong type")
				}
				if len(relationships) != 1 || relationships[0] != "mentioned" {
					t.Errorf("user_relationships = %v, want [mentioned]", relationships)
				}

				// Verify event passes validation
				if err := event.Validate(); err != nil {
					t.Errorf("event validation failed: %v", err)
				}
			},
		},
		{
			name: "PR with issue comment mention",
			mockSetup: func(cred *mockGitHubDelegatedCredential) {
				// Search response: one PR with comment mention
				searchData := []byte(`{
					"data": {
						"search": {
							"issueCount": 1,
							"nodes": [
								{
									"number": 456,
									"title": "Fix bug",
									"url": "https://github.com/owner/repo/pull/456",
									"updatedAt": "2025-12-06T11:00:00Z",
									"repository": {"nameWithOwner": "owner/repo"},
									"author": {"login": "author"}
								}
							],
							"pageInfo": {"hasNextPage": false, "endCursor": ""}
						}
					}
				}`)
				cred.setGraphQLResponse("graphql:search:pr-involves", searchData)

				// Full PR context response with mention in comment
				contextData := []byte(`{
					"data": {
						"repository": {
							"pullRequest": {
								"number": 456,
								"title": "Fix bug",
								"state": "OPEN",
								"isDraft": false,
								"mergeable": "MERGEABLE",
								"url": "https://github.com/owner/repo/pull/456",
								"createdAt": "2025-12-05T11:00:00Z",
								"updatedAt": "2025-12-06T11:00:00Z",
								"author": {"login": "author"},
								"assignees": {"nodes": []},
								"labels": {"nodes": []},
								"milestone": null,
								"body": "",
								"headRefName": "bugfix",
								"baseRefName": "main",
								"additions": 5,
								"deletions": 2,
								"changedFiles": 1,
								"files": {"nodes": []},
								"reviewRequests": {"nodes": []},
								"reviews": {"nodes": []},
								"reviewThreads": {"nodes": []},
								"comments": {
									"totalCount": 1,
									"nodes": [
										{"author": {"login": "commenter"}, "body": "@testuser please take a look at this", "createdAt": "2025-12-06T10:00:00Z", "updatedAt": "2025-12-06T10:00:00Z"}
									]
								},
								"commits": {"nodes": [{"commit": {"statusCheckRollup": null}}]},
								"closingIssuesReferences": {"nodes": []}
							}
						}
					}
				}`)
				cred.setGraphQLResponse("graphql:pr:context", contextData)
			},
			since:          time.Date(2025, 12, 6, 0, 0, 0, 0, time.UTC),
			orgs:           []string{},
			expectedEvents: 1,
			checkEvents: func(t *testing.T, events []models.Event) {
				if len(events) != 1 {
					t.Fatalf("expected 1 event, got %d", len(events))
				}

				event := events[0]

				// Same verifications as review mention
				if event.Type != models.EventTypePRCommentMention {
					t.Errorf("Type = %q, want %q", event.Type, models.EventTypePRCommentMention)
				}
				if event.Priority != models.PriorityMedium {
					t.Errorf("Priority = %d, want %d", event.Priority, models.PriorityMedium)
				}
				if event.RequiresAction {
					t.Error("RequiresAction should be false")
				}

				relationships := event.Metadata["user_relationships"].([]string)
				if len(relationships) != 1 || relationships[0] != "mentioned" {
					t.Errorf("user_relationships = %v, want [mentioned]", relationships)
				}
			},
		},
		{
			name: "PR without mentions - no events",
			mockSetup: func(cred *mockGitHubDelegatedCredential) {
				// Search response: one PR but no mentions
				searchData := []byte(`{
					"data": {
						"search": {
							"issueCount": 1,
							"nodes": [
								{
									"number": 789,
									"title": "Update docs",
									"url": "https://github.com/owner/repo/pull/789",
									"updatedAt": "2025-12-06T12:00:00Z",
									"repository": {"nameWithOwner": "owner/repo"},
									"author": {"login": "author"}
								}
							],
							"pageInfo": {"hasNextPage": false, "endCursor": ""}
						}
					}
				}`)
				cred.setGraphQLResponse("graphql:search:pr-involves", searchData)

				// Full PR context response without mentions
				contextData := []byte(`{
					"data": {
						"repository": {
							"pullRequest": {
								"number": 789,
								"title": "Update docs",
								"state": "OPEN",
								"isDraft": false,
								"mergeable": "MERGEABLE",
								"url": "https://github.com/owner/repo/pull/789",
								"createdAt": "2025-12-05T12:00:00Z",
								"updatedAt": "2025-12-06T12:00:00Z",
								"author": {"login": "author"},
								"assignees": {"nodes": []},
								"labels": {"nodes": []},
								"milestone": null,
								"body": "",
								"headRefName": "docs",
								"baseRefName": "main",
								"additions": 3,
								"deletions": 1,
								"changedFiles": 1,
								"files": {"nodes": []},
								"reviewRequests": {"nodes": []},
								"reviews": {
									"nodes": [
										{"author": {"login": "reviewer"}, "state": "APPROVED", "body": "LGTM", "createdAt": "2025-12-06T11:00:00Z"}
									]
								},
								"reviewThreads": {"nodes": []},
								"comments": {
									"totalCount": 1,
									"nodes": [
										{"author": {"login": "commenter"}, "body": "Looks good!", "createdAt": "2025-12-06T10:00:00Z", "updatedAt": "2025-12-06T10:00:00Z"}
									]
								},
								"commits": {"nodes": [{"commit": {"statusCheckRollup": null}}]},
								"closingIssuesReferences": {"nodes": []}
							}
						}
					}
				}`)
				cred.setGraphQLResponse("graphql:pr:context", contextData)
			},
			since:          time.Date(2025, 12, 6, 0, 0, 0, 0, time.UTC),
			orgs:           []string{},
			expectedEvents: 0,
			checkEvents: func(t *testing.T, events []models.Event) {
				if len(events) != 0 {
					t.Errorf("expected no events, got %d", len(events))
				}
			},
		},
		{
			name: "empty results - no PRs found",
			mockSetup: func(cred *mockGitHubDelegatedCredential) {
				// Search response: empty results
				searchData := []byte(`{
					"data": {
						"search": {
							"issueCount": 0,
							"nodes": [],
							"pageInfo": {"hasNextPage": false, "endCursor": ""}
						}
					}
				}`)
				cred.setGraphQLResponse("graphql:search:pr-involves", searchData)
			},
			since:          time.Date(2025, 12, 6, 0, 0, 0, 0, time.UTC),
			orgs:           []string{},
			expectedEvents: 0,
			checkEvents: func(t *testing.T, events []models.Event) {
				if len(events) != 0 {
					t.Errorf("expected no events, got %d", len(events))
				}
			},
		},
		{
			name: "context fetch fails - PR skipped",
			mockSetup: func(cred *mockGitHubDelegatedCredential) {
				// Search response: one PR
				searchData := []byte(`{
					"data": {
						"search": {
							"issueCount": 1,
							"nodes": [
								{
									"number": 999,
									"title": "PR with context error",
									"url": "https://github.com/owner/repo/pull/999",
									"updatedAt": "2025-12-06T13:00:00Z",
									"repository": {"nameWithOwner": "owner/repo"},
									"author": {"login": "author"}
								}
							],
							"pageInfo": {"hasNextPage": false, "endCursor": ""}
						}
					}
				}`)
				cred.setGraphQLResponse("graphql:search:pr-involves", searchData)

				// Context fetch returns error
				cred.setGraphQLError("graphql:pr:context", fmt.Errorf("context fetch failed"))
			},
			since:          time.Date(2025, 12, 6, 0, 0, 0, 0, time.UTC),
			orgs:           []string{},
			expectedEvents: 0,
			checkEvents: func(t *testing.T, events []models.Event) {
				if len(events) != 0 {
					t.Errorf("expected no events (PR skipped on error), got %d", len(events))
				}
			},
		},
		{
			name: "multiple PRs - both with mentions",
			mockSetup: func(cred *mockGitHubDelegatedCredential) {
				// Search response: two PRs
				searchData := []byte(`{
					"data": {
						"search": {
							"issueCount": 2,
							"nodes": [
								{
									"number": 100,
									"title": "First PR",
									"url": "https://github.com/owner/repo/pull/100",
									"updatedAt": "2025-12-06T10:00:00Z",
									"repository": {"nameWithOwner": "owner/repo"},
									"author": {"login": "author1"}
								},
								{
									"number": 101,
									"title": "Second PR",
									"url": "https://github.com/owner/repo/pull/101",
									"updatedAt": "2025-12-06T11:00:00Z",
									"repository": {"nameWithOwner": "owner/repo"},
									"author": {"login": "author2"}
								}
							],
							"pageInfo": {"hasNextPage": false, "endCursor": ""}
						}
					}
				}`)
				cred.setGraphQLResponse("graphql:search:pr-involves", searchData)

				// Full PR context response - with mention
				// Note: Mock returns same context for all PRs (limitation of mock system)
				contextData := []byte(`{
					"data": {
						"repository": {
							"pullRequest": {
								"number": 100,
								"title": "First PR",
								"state": "OPEN",
								"isDraft": false,
								"mergeable": "MERGEABLE",
								"url": "https://github.com/owner/repo/pull/100",
								"createdAt": "2025-12-05T10:00:00Z",
								"updatedAt": "2025-12-06T10:00:00Z",
								"author": {"login": "author1"},
								"assignees": {"nodes": []},
								"labels": {"nodes": []},
								"milestone": null,
								"body": "",
								"headRefName": "feature",
								"baseRefName": "main",
								"additions": 10,
								"deletions": 5,
								"changedFiles": 2,
								"files": {"nodes": []},
								"reviewRequests": {"nodes": []},
								"reviews": {
									"nodes": [
										{"author": {"login": "reviewer"}, "state": "COMMENTED", "body": "@testuser please review", "createdAt": "2025-12-06T09:00:00Z"}
									]
								},
								"reviewThreads": {"nodes": []},
								"comments": {"totalCount": 0, "nodes": []},
								"commits": {"nodes": [{"commit": {"statusCheckRollup": null}}]},
								"closingIssuesReferences": {"nodes": []}
							}
						}
					}
				}`)
				cred.setGraphQLResponse("graphql:pr:context", contextData)
			},
			since:          time.Date(2025, 12, 6, 0, 0, 0, 0, time.UTC),
			orgs:           []string{},
			expectedEvents: 2, // Both PRs have mentions (mock limitation: same context for all)
			checkEvents: func(t *testing.T, events []models.Event) {
				if len(events) != 2 {
					t.Fatalf("expected 2 events, got %d", len(events))
				}

				// Verify all events are EventTypePRCommentMention with correct priority
				for i, event := range events {
					if event.Type != models.EventTypePRCommentMention {
						t.Errorf("event[%d].Type = %q, want %q", i, event.Type, models.EventTypePRCommentMention)
					}
					if event.Priority != models.PriorityMedium {
						t.Errorf("event[%d].Priority = %d, want %d", i, event.Priority, models.PriorityMedium)
					}
					if event.RequiresAction {
						t.Errorf("event[%d].RequiresAction = true, want false", i)
					}
				}
			},
		},
		{
			name: "with org filter",
			mockSetup: func(cred *mockGitHubDelegatedCredential) {
				// Search response: one PR from specified org
				searchData := []byte(`{
					"data": {
						"search": {
							"issueCount": 1,
							"nodes": [
								{
									"number": 200,
									"title": "Org-specific PR",
									"url": "https://github.com/myorg/repo/pull/200",
									"updatedAt": "2025-12-06T14:00:00Z",
									"repository": {"nameWithOwner": "myorg/repo"},
									"author": {"login": "orguser"}
								}
							],
							"pageInfo": {"hasNextPage": false, "endCursor": ""}
						}
					}
				}`)
				cred.setGraphQLResponse("graphql:search:pr-involves", searchData)

				// Full PR context response with mention
				contextData := []byte(`{
					"data": {
						"repository": {
							"pullRequest": {
								"number": 200,
								"title": "Org-specific PR",
								"state": "OPEN",
								"isDraft": false,
								"mergeable": "MERGEABLE",
								"url": "https://github.com/myorg/repo/pull/200",
								"createdAt": "2025-12-05T14:00:00Z",
								"updatedAt": "2025-12-06T14:00:00Z",
								"author": {"login": "orguser"},
								"assignees": {"nodes": []},
								"labels": {"nodes": []},
								"milestone": null,
								"body": "",
								"headRefName": "feature",
								"baseRefName": "main",
								"additions": 10,
								"deletions": 5,
								"changedFiles": 2,
								"files": {"nodes": []},
								"reviewRequests": {"nodes": []},
								"reviews": {
									"nodes": [
										{"author": {"login": "reviewer"}, "state": "COMMENTED", "body": "@testuser what do you think?", "createdAt": "2025-12-06T13:00:00Z"}
									]
								},
								"reviewThreads": {"nodes": []},
								"comments": {"totalCount": 0, "nodes": []},
								"commits": {"nodes": [{"commit": {"statusCheckRollup": null}}]},
								"closingIssuesReferences": {"nodes": []}
							}
						}
					}
				}`)
				cred.setGraphQLResponse("graphql:pr:context", contextData)
			},
			since:          time.Date(2025, 12, 6, 0, 0, 0, 0, time.UTC),
			orgs:           []string{"myorg"},
			expectedEvents: 1,
			checkEvents: func(t *testing.T, events []models.Event) {
				if len(events) != 1 {
					t.Fatalf("expected 1 event, got %d", len(events))
				}

				event := events[0]
				if !contains(event.URL, "myorg") {
					t.Errorf("URL = %q, want to contain 'myorg'", event.URL)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup mock auth provider
			authProvider := newMockAuthProvider(auth.ServiceGitHub)
			tt.mockSetup(authProvider.credential)

			// Create datasource
			ds, err := NewDataSource(authProvider)
			if err != nil {
				t.Fatalf("NewDataSource failed: %v", err)
			}

			// Create GraphQL client
			gqlClient := NewGraphQLClient(authProvider.credential)

			// Call method
			events, err := ds.fetchPRCommentMentionsGraphQL(
				context.Background(),
				gqlClient,
				authProvider.credential,
				tt.since,
				tt.orgs,
			)

			// Check error expectation
			if tt.expectedError && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.expectedError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			// Check event count
			if len(events) != tt.expectedEvents {
				t.Errorf("expected %d events, got %d", tt.expectedEvents, len(events))
			}

			// Run custom checks
			if tt.checkEvents != nil {
				tt.checkEvents(t, events)
			}
		})
	}
}
