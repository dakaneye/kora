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
