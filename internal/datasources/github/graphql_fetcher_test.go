package github

import (
	"testing"

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

			result := deduplicateEvents(events)

			if len(result) != 1 {
				t.Fatalf("expected 1 event, got %d", len(result))
			}

			if result[0].Priority != tt.expectedPriority {
				t.Errorf("priority = %d, want %d", result[0].Priority, tt.expectedPriority)
			}
		})
	}
}
