package models

// Ground truth defined in specs/efas/0001-event-model.md
// IT IS FORBIDDEN TO CHANGE without updating EFA 0001.

// Person represents a user across any datasource.
type Person struct {
	// Name is the display name (may be empty).
	Name string `json:"name,omitempty"`

	// Username is the platform-specific handle (required).
	// GitHub: "octocat", Slack: "U12345678" or display name
	Username string `json:"username"`
}
