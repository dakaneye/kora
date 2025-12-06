// Package datasources provides abstractions for fetching events from external services.
// Ground truth defined in specs/efas/0003-datasource-interface.md
//
// IT IS FORBIDDEN TO CHANGE the DataSourceRunner type without updating EFA 0003.
package datasources

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/dakaneye/kora/internal/models"
	"golang.org/x/sync/errgroup"
)

// DataSourceRunner executes multiple datasources concurrently.
// IT IS FORBIDDEN TO CHANGE THIS TYPE without updating EFA 0003.
type DataSourceRunner struct {
	sources []DataSource
	timeout time.Duration
}

// RunnerOption configures a DataSourceRunner.
type RunnerOption func(*DataSourceRunner)

// WithTimeout sets the per-datasource timeout.
// Default is 30 seconds.
func WithTimeout(d time.Duration) RunnerOption {
	return func(r *DataSourceRunner) {
		r.timeout = d
	}
}

// NewRunner creates a DataSourceRunner with the given datasources.
func NewRunner(sources []DataSource, opts ...RunnerOption) *DataSourceRunner {
	r := &DataSourceRunner{
		sources: sources,
		timeout: 30 * time.Second,
	}
	for _, opt := range opts {
		opt(r)
	}
	return r
}

// sourceResult holds the result from a single datasource fetch.
//
//nolint:govet // Field order matches logical grouping, not optimized for alignment
type sourceResult struct {
	name   string
	result *FetchResult
	err    error
}

// Run executes all datasources concurrently and aggregates results.
//
// Execution model:
//   - Each datasource runs in its own goroutine
//   - Per-datasource timeout is applied via context
//   - Failures in one datasource do not affect others
//   - Results are aggregated and sorted by timestamp
//
// The returned RunResult contains:
//   - All events from successful datasources
//   - Per-datasource errors for failed datasources
//   - Statistics for observability
//
// EFA 0003: One datasource failure MUST NOT block others.
// EFA 0003: All returned events MUST pass Event.Validate().
// EFA 0003: Events MUST be sorted by Timestamp ascending.
func (r *DataSourceRunner) Run(ctx context.Context, opts FetchOptions) (*RunResult, error) {
	if err := opts.Validate(); err != nil {
		return nil, fmt.Errorf("invalid fetch options: %w", err)
	}

	if len(r.sources) == 0 {
		return &RunResult{
			SourceResults: make(map[string]*FetchResult),
			SourceErrors:  make(map[string]error),
		}, nil
	}

	results := make(chan sourceResult, len(r.sources))

	g, gctx := errgroup.WithContext(ctx)

	for _, src := range r.sources {
		src := src // capture loop variable
		g.Go(func() error {
			// Apply per-datasource timeout
			srcCtx, cancel := context.WithTimeout(gctx, r.timeout)
			defer cancel()

			result, err := src.Fetch(srcCtx, opts)
			results <- sourceResult{
				name:   src.Name(),
				result: result,
				err:    err,
			}
			// Don't return error - we want all datasources to run
			// Returning an error would cancel other goroutines via errgroup
			return nil
		})
	}

	// Wait for all goroutines to complete.
	// We ignore the error because each goroutine returns nil (errors are sent via channel).
	//nolint:errcheck // Intentional: goroutines always return nil, errors sent via channel
	g.Wait() // #nosec G104 -- goroutines always return nil, errors sent via channel
	close(results)

	// Aggregate results
	runResult := &RunResult{
		SourceResults: make(map[string]*FetchResult),
		SourceErrors:  make(map[string]error),
	}

	for sr := range results {
		if sr.err != nil {
			runResult.SourceErrors[sr.name] = sr.err
			// Still include partial results if available
			if sr.result != nil && sr.result.HasEvents() {
				runResult.SourceResults[sr.name] = sr.result
				runResult.Events = append(runResult.Events, sr.result.Events...)
			}
		} else if sr.result != nil {
			runResult.SourceResults[sr.name] = sr.result
			runResult.Events = append(runResult.Events, sr.result.Events...)
		}
	}

	// Sort all events by timestamp ascending
	sort.Slice(runResult.Events, func(i, j int) bool {
		return runResult.Events[i].Timestamp.Before(runResult.Events[j].Timestamp)
	})

	return runResult, nil
}

// RunResult contains aggregated results from all datasources.
//
//nolint:govet // Field order matches EFA 0003 specification, not optimized for alignment
type RunResult struct {
	// Events contains all events from all datasources, sorted by Timestamp.
	Events []models.Event

	// SourceResults contains per-datasource results.
	SourceResults map[string]*FetchResult

	// SourceErrors contains per-datasource errors for failed datasources.
	SourceErrors map[string]error
}

// Success returns true if all datasources succeeded.
func (r *RunResult) Success() bool {
	return len(r.SourceErrors) == 0
}

// Partial returns true if some datasources succeeded but others failed.
func (r *RunResult) Partial() bool {
	return len(r.SourceErrors) > 0 && len(r.SourceResults) > 0
}

// TotalEvents returns the count of all fetched events.
func (r *RunResult) TotalEvents() int {
	return len(r.Events)
}
