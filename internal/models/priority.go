package models

// Ground truth defined in specs/efas/0001-event-model.md
// IT IS FORBIDDEN TO CHANGE without updating EFA 0001.

// Priority represents the urgency level of an event (1-5, where 1 is highest).
type Priority int

// Priority constants representing urgency levels.
const (
	// PriorityCritical means blocking others, needs immediate action.
	PriorityCritical Priority = 1
	// PriorityHigh means should address today.
	PriorityHigh Priority = 2
	// PriorityMedium means should address this week.
	PriorityMedium Priority = 3
	// PriorityLow means nice to know.
	PriorityLow Priority = 4
	// PriorityInfo means FYI only.
	PriorityInfo Priority = 5
)

// IsValid reports whether p is within the valid priority range (1-5).
func (p Priority) IsValid() bool {
	return p >= PriorityCritical && p <= PriorityInfo
}

// String returns a human-readable representation of the priority level.
func (p Priority) String() string {
	switch p {
	case PriorityCritical:
		return "Critical"
	case PriorityHigh:
		return "High"
	case PriorityMedium:
		return "Medium"
	case PriorityLow:
		return "Low"
	case PriorityInfo:
		return "Info"
	default:
		return "Unknown"
	}
}

// Priority Assignment Rules (EFA 0001)
//
// Datasources MUST assign priority according to these rules:
//
//	| Condition                           | Priority          | EventType      | RequiresAction |
//	|-------------------------------------|-------------------|----------------|----------------|
//	| PR author + CI failing              | 1 (Critical)      | pr_author      | true           |
//	| PR review requested (direct user)   | 2 (High)          | pr_review      | true           |
//	| PR author + changes requested       | 2 (High)          | pr_author      | true           |
//	| Direct message / DM                 | 2 (High)          | slack_dm       | true           |
//	| PR review requested (team)          | 3 (Medium)        | pr_review      | true           |
//	| PR author (pending/approved)        | 3 (Medium)        | pr_author      | false          |
//	| PR codeowner (not explicit reviewer)| 3 (Medium)        | pr_codeowner   | true           |
//	| @mention in issue/PR/channel        | 3 (Medium)        | *_mention      | false          |
//	| Issue assigned                      | 3 (Medium)        | issue_assigned | true           |
//	| Thread reply                        | 4 (Low)           | slack_mention  | false          |
//	| FYI / informational                 | 5 (Info)          | -              | -              |
//
// Priority Calculation for PR Author:
//
//	func calculatePRAuthorPriority(ciRollup string, hasChangesRequested bool) Priority {
//	    if ciRollup == "failure" {
//	        return PriorityCritical // 1 - CI broken, blocks merge
//	    }
//	    if hasChangesRequested {
//	        return PriorityHigh // 2 - Reviewer waiting
//	    }
//	    return PriorityMedium // 3 - PR in progress
//	}
//
// Priority Calculation for PR Review:
//
//	func calculatePRReviewPriority(reviewRequestType string) Priority {
//	    if reviewRequestType == "user" {
//	        return PriorityHigh // 2 - Direct request
//	    }
//	    return PriorityMedium // 3 - Team request
//	}
