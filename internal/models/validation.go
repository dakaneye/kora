package models

// Ground truth defined in specs/efas/0001-event-model.md
// IT IS FORBIDDEN TO CHANGE validation rules without updating EFA 0001.

import (
	"fmt"
	"net/url"
	"strings"
)

// allowedMetadataKeys defines the permitted metadata keys per source.
// EFA 0001: Do NOT add keys here without updating the EFA table.
// IT IS FORBIDDEN TO ADD KEYS without updating this map AND the EFA table above.
var allowedMetadataKeys = map[Source]map[string]struct{}{
	SourceGitHub: {
		// Common fields (PR and Issue)
		"repo":               {},
		"number":             {},
		"state":              {},
		"author_login":       {},
		"assignees":          {},
		"user_relationships": {},
		"labels":             {},
		"milestone":          {},
		"body":               {},
		"comments_count":     {},
		"created_at":         {},
		"updated_at":         {},
		// PR-specific fields
		"review_requests":       {},
		"reviews":               {},
		"ci_checks":             {},
		"ci_rollup":             {},
		"files_changed":         {},
		"files_changed_count":   {},
		"additions":             {},
		"deletions":             {},
		"linked_issues":         {},
		"review_comments_count": {},
		"unresolved_threads":    {},
		"is_draft":              {},
		"mergeable":             {},
		"head_ref":              {},
		"base_ref":              {},
		// Issue-specific fields
		"comments":         {},
		"linked_prs":       {},
		"reactions":        {},
		"timeline_summary": {},
	},
	SourceSlack: {
		"workspace":       {},
		"channel":         {},
		"thread_ts":       {},
		"is_thread_reply": {},
	},
}

// Validate checks that the event satisfies all invariants defined in EFA 0001.
// All 8 validation rules are enforced:
//  1. Type must be a defined EventType constant
//  2. Title must be non-empty and <=100 characters
//  3. Source must be a defined Source constant
//  4. URL must be a valid URL or empty string
//  5. Author.Username must be non-empty
//  6. Timestamp must not be zero
//  7. Priority must be 1-5 inclusive
//  8. Metadata keys must be from the allowed set for the Source
func (e *Event) Validate() error {
	var errs []string

	if !e.Type.IsValid() {
		errs = append(errs, "invalid event type")
	}
	if e.Title == "" || len(e.Title) > 100 {
		errs = append(errs, "title must be 1-100 characters")
	}
	if !e.Source.IsValid() {
		errs = append(errs, "invalid source")
	}
	if err := e.validateURL(); err != nil {
		errs = append(errs, err.Error())
	}
	if e.Author.Username == "" {
		errs = append(errs, "author username required")
	}
	if e.Timestamp.IsZero() {
		errs = append(errs, "timestamp required")
	}
	if !e.Priority.IsValid() {
		errs = append(errs, "priority must be 1-5")
	}
	if err := e.validateMetadataKeys(); err != nil {
		errs = append(errs, err.Error())
	}

	if len(errs) > 0 {
		return fmt.Errorf("invalid event: %s", strings.Join(errs, "; "))
	}
	return nil
}

// validateURL checks that the URL is empty or a valid absolute URL.
func (e *Event) validateURL() error {
	if e.URL == "" {
		return nil
	}
	u, err := url.Parse(e.URL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}
	if u.Scheme == "" || u.Host == "" {
		return fmt.Errorf("URL must be absolute with scheme and host")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("URL scheme must be http or https")
	}
	return nil
}

// validateMetadataKeys ensures all metadata keys are in the allowed set for the source.
func (e *Event) validateMetadataKeys() error {
	if len(e.Metadata) == 0 {
		return nil
	}
	allowed, ok := allowedMetadataKeys[e.Source]
	if !ok {
		// Source validation handles unknown sources; skip metadata check
		return nil
	}
	var invalid []string
	for k := range e.Metadata {
		if _, ok := allowed[k]; !ok {
			invalid = append(invalid, k)
		}
	}
	if len(invalid) > 0 {
		return fmt.Errorf("invalid metadata keys for %s: %v", e.Source, invalid)
	}
	return nil
}
