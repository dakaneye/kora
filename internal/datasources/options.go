// Package datasources provides abstractions for fetching events from external services.
// Ground truth defined in specs/efas/0003-datasource-interface.md
package datasources

import (
	"errors"
	"time"

	"github.com/dakaneye/kora/internal/models"
)

// FetchOptions configures a Fetch operation.
// IT IS FORBIDDEN TO ADD FIELDS without updating EFA 0003.
//
//nolint:govet // Field order matches EFA 0003 specification, not optimized for alignment
type FetchOptions struct {
	// Since is the exclusive lower bound for event timestamps.
	// Events with Timestamp <= Since are excluded.
	// Required: must not be zero.
	Since time.Time

	// Limit is the maximum number of events to return.
	// 0 means no limit (use service default, typically 100).
	// Implementations should respect rate limits over this limit.
	Limit int

	// Filter contains optional filter criteria.
	// Interpretation is datasource-specific.
	Filter *FetchFilter
}

// FetchFilter provides optional filtering criteria.
// Not all datasources support all filters; unsupported filters are ignored.
type FetchFilter struct {
	// EventTypes limits results to specific event types.
	// Empty slice means all types supported by the datasource.
	EventTypes []models.EventType

	// MinPriority filters to events with priority <= this value.
	// 0 means no priority filter.
	// Remember: priority 1 is highest, 5 is lowest.
	MinPriority models.Priority

	// RequiresAction filters to only actionable events.
	// false means all events, true means only RequiresAction=true.
	RequiresAction bool
}

// Validate checks that FetchOptions are valid.
func (o FetchOptions) Validate() error {
	if o.Since.IsZero() {
		return errors.New("FetchOptions.Since is required")
	}
	if o.Limit < 0 {
		return errors.New("FetchOptions.Limit must be non-negative")
	}
	return nil
}
