// Package datasources provides abstractions for fetching events from external services.
// Ground truth defined in specs/efas/0003-datasource-interface.md
package datasources

import (
	"errors"
	"time"

	"github.com/dakaneye/kora/internal/models"
)

// FetchResult contains the results of a Fetch operation.
//
//nolint:govet // Field order matches EFA 0003 specification, not optimized for alignment
type FetchResult struct {
	// Events contains the fetched events, sorted by Timestamp ascending.
	Events []models.Event

	// Partial indicates some events may be missing due to errors.
	// When true, Errors contains details about what failed.
	Partial bool

	// Errors contains non-fatal errors encountered during fetch.
	// These did not prevent returning partial results.
	Errors []error

	// RateLimited indicates the fetch was cut short due to rate limiting.
	// The caller may retry after RateLimitReset.
	RateLimited bool

	// RateLimitReset is when rate limiting expires (zero if not rate limited).
	RateLimitReset time.Time

	// Stats contains fetch statistics for observability.
	Stats FetchStats
}

// FetchStats provides observability data about a fetch operation.
type FetchStats struct {
	// Duration is how long the fetch took.
	Duration time.Duration

	// APICallCount is the number of API calls made.
	APICallCount int

	// EventsFetched is the total events before filtering.
	EventsFetched int

	// EventsReturned is the count after filtering.
	EventsReturned int
}

// HasEvents returns true if any events were fetched.
func (r *FetchResult) HasEvents() bool {
	return len(r.Events) > 0
}

// HasErrors returns true if any errors occurred.
func (r *FetchResult) HasErrors() bool {
	return len(r.Errors) > 0
}

// CombinedError returns all errors as a single error, or nil if none.
func (r *FetchResult) CombinedError() error {
	if !r.HasErrors() {
		return nil
	}
	return errors.Join(r.Errors...)
}
