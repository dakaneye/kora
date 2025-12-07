// Package testutil provides test helpers for the models package.
package testutil

import (
	"testing"
	"time"

	"github.com/dakaneye/kora/internal/models"
)

// AssertValidEvent verifies that the given event passes validation.
// It calls t.Helper() to report errors at the caller's location.
func AssertValidEvent(t testing.TB, e *models.Event) {
	t.Helper()
	if err := e.Validate(); err != nil {
		t.Errorf("invalid event: %v", err)
	}
}

// AssertMetadataKeys checks that all metadata keys in the event are in the allowed set.
// It calls t.Helper() to report errors at the caller's location.
func AssertMetadataKeys(t testing.TB, e *models.Event, allowed []string) {
	t.Helper()
	allowedSet := make(map[string]bool)
	for _, k := range allowed {
		allowedSet[k] = true
	}
	for k := range e.Metadata {
		if !allowedSet[k] {
			t.Errorf("unexpected metadata key %q for source %s", k, e.Source)
		}
	}
}

// NewTestEvent creates a valid GitHub PR review event for testing.
// All required fields are populated with sensible defaults.
func NewTestEvent() models.Event {
	return models.Event{
		Type:           models.EventTypePRReview,
		Title:          "Test PR Review",
		Source:         models.SourceGitHub,
		URL:            "https://github.com/owner/repo/pull/1",
		Author:         models.Person{Username: "testuser"},
		Timestamp:      time.Now().UTC(),
		Priority:       models.PriorityHigh,
		RequiresAction: true,
		Metadata: map[string]any{
			"repo":   "owner/repo",
			"number": 1,
		},
	}
}

// NewTestEventWithOptions creates a test event with optional modifications.
// The base event is a valid GitHub PR review, which can be customized via options.
func NewTestEventWithOptions(opts ...func(*models.Event)) models.Event {
	e := NewTestEvent()
	for _, opt := range opts {
		opt(&e)
	}
	return e
}

// WithType sets the event type.
func WithType(t models.EventType) func(*models.Event) {
	return func(e *models.Event) {
		e.Type = t
	}
}

// WithTitle sets the event title.
func WithTitle(title string) func(*models.Event) {
	return func(e *models.Event) {
		e.Title = title
	}
}

// WithSource sets the event source.
func WithSource(source models.Source) func(*models.Event) {
	return func(e *models.Event) {
		e.Source = source
	}
}

// WithURL sets the event URL.
func WithURL(url string) func(*models.Event) {
	return func(e *models.Event) {
		e.URL = url
	}
}

// WithAuthor sets the event author.
func WithAuthor(username, name string) func(*models.Event) {
	return func(e *models.Event) {
		e.Author = models.Person{Username: username, Name: name}
	}
}

// WithTimestamp sets the event timestamp.
func WithTimestamp(ts time.Time) func(*models.Event) {
	return func(e *models.Event) {
		e.Timestamp = ts
	}
}

// WithPriority sets the event priority.
func WithPriority(p models.Priority) func(*models.Event) {
	return func(e *models.Event) {
		e.Priority = p
	}
}

// WithRequiresAction sets whether the event requires action.
func WithRequiresAction(requiresAction bool) func(*models.Event) {
	return func(e *models.Event) {
		e.RequiresAction = requiresAction
	}
}

// WithMetadata sets the event metadata.
func WithMetadata(metadata map[string]any) func(*models.Event) {
	return func(e *models.Event) {
		e.Metadata = metadata
	}
}
