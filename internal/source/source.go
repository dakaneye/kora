// Package source defines the Source interface and orchestrates parallel data fetching.
package source

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"
)

// Source gathers activity data from a single external service.
type Source interface {
	Name() string
	CheckAuth(ctx context.Context) error
	RefreshAuth(ctx context.Context) error
	Fetch(ctx context.Context, since time.Duration) (json.RawMessage, error)
}

// Result is the top-level output envelope.
type Result struct {
	Sources   map[string]json.RawMessage `json:"sources"`
	FetchedAt string                     `json:"fetched_at"`
	Since     string                     `json:"since"`
}

// RunError collects per-source errors.
type RunError struct {
	Errors []FetchError
}

// FetchError describes a failure in a single source.
type FetchError struct {
	Source string `json:"source"`
	Phase  string `json:"phase"`
	Err    string `json:"error"`
}

func (e *RunError) Error() string {
	msgs := make([]string, len(e.Errors))
	for i, se := range e.Errors {
		msgs[i] = fmt.Sprintf("%s (%s): %s", se.Source, se.Phase, se.Err)
	}
	return strings.Join(msgs, "; ")
}

// Run orchestrates auth checking and data fetching across all sources.
//
//  1. Check auth for all sources in parallel.
//  2. For any that fail, run RefreshAuth sequentially (may open browser).
//  3. Re-check those sources.
//  4. If any still fail, return error with details.
//  5. Fetch all sources in parallel.
//  6. If any fetch fails, return error with details.
func Run(ctx context.Context, sources []Source, since time.Duration) (Result, error) {
	result := Result{
		FetchedAt: time.Now().UTC().Format(time.RFC3339),
		Since:     since.String(),
		Sources:   make(map[string]json.RawMessage),
	}

	if len(sources) == 0 {
		return result, nil
	}

	// Phase 1: Check auth in parallel
	fmt.Fprintf(os.Stderr, "checking auth for %d sources...\n", len(sources))
	authFailed := checkAuthParallel(ctx, sources)

	// Phase 2: Refresh failed sources sequentially
	if len(authFailed) > 0 {
		names := make([]string, len(authFailed))
		for i, s := range authFailed {
			names[i] = s.Name()
		}
		fmt.Fprintf(os.Stderr, "auth failed for: %s — attempting refresh\n", strings.Join(names, ", "))
		for _, s := range authFailed {
			if err := s.RefreshAuth(ctx); err != nil {
				fmt.Fprintf(os.Stderr, "%s: refresh failed: %v\n", s.Name(), err)
				continue
			}
		}
		// Phase 3: Re-check previously failed sources
		stillFailed := checkAuthParallel(ctx, authFailed)
		if len(stillFailed) > 0 {
			runErr := &RunError{}
			for _, s := range stillFailed {
				runErr.Errors = append(runErr.Errors, FetchError{
					Source: s.Name(),
					Phase:  "auth",
					Err:    "authentication failed after refresh attempt",
				})
			}
			return result, runErr
		}
	}

	// Phase 4: Fetch all in parallel
	fmt.Fprintf(os.Stderr, "fetching activity from %d sources...\n", len(sources))
	var mu sync.Mutex
	var fetchErrors []FetchError

	var wg sync.WaitGroup
	for _, s := range sources {
		wg.Add(1)
		go func() {
			defer wg.Done()
			data, err := s.Fetch(ctx, since)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				fetchErrors = append(fetchErrors, FetchError{
					Source: s.Name(),
					Phase:  "fetch",
					Err:    err.Error(),
				})
				return
			}
			result.Sources[s.Name()] = data
		}()
	}
	wg.Wait()

	if len(fetchErrors) > 0 {
		return result, &RunError{Errors: fetchErrors}
	}

	return result, nil
}

func checkAuthParallel(ctx context.Context, sources []Source) []Source {
	type authResult struct {
		source Source
		err    error
	}

	results := make([]authResult, len(sources))
	var wg sync.WaitGroup

	for i, s := range sources {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results[i] = authResult{source: s, err: s.CheckAuth(ctx)}
		}()
	}
	wg.Wait()

	var failed []Source
	for _, r := range results {
		if r.err != nil {
			failed = append(failed, r.source)
		}
	}
	return failed
}
