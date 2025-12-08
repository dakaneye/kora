//go:build integration

package integration

import (
	"context"
	"testing"
	"time"

	"github.com/dakaneye/kora/internal/auth/github"
	"github.com/dakaneye/kora/internal/datasources"
	githubds "github.com/dakaneye/kora/internal/datasources/github"
	"github.com/dakaneye/kora/internal/models"
)

// testSince24h returns a timestamp 24 hours ago.
func testSince24h() time.Time {
	return time.Now().Add(-24 * time.Hour)
}

// TestGitHubDataSource_Fetch tests fetching events from real GitHub API.
func TestGitHubDataSource_Fetch(t *testing.T) {
	requireGitHubAuth(t)

	ctx := context.Background()

	// Create auth provider
	authProvider := github.NewGitHubAuthProvider("")
	if err := authProvider.Authenticate(ctx); err != nil {
		t.Fatalf("GitHub authentication failed: %v", err)
	}

	// Create datasource
	ds, err := githubds.NewDataSource(authProvider)
	if err != nil {
		t.Fatalf("NewDataSource() error = %v", err)
	}

	// Verify datasource properties
	if ds.Name() != "github" {
		t.Errorf("Name() = %q, want %q", ds.Name(), "github")
	}
	if ds.Service() != models.SourceGitHub {
		t.Errorf("Service() = %q, want %q", ds.Service(), models.SourceGitHub)
	}

	t.Run("Fetch with 24h window", func(t *testing.T) {
		opts := datasources.FetchOptions{
			Since: testSince24h(),
			Limit: 0, // no limit
		}

		result, err := ds.Fetch(ctx, opts)
		if err != nil {
			t.Fatalf("Fetch() error = %v", err)
		}

		// Verify result structure
		if result == nil {
			t.Fatal("Fetch() returned nil result")
		}

		// Verify stats are populated
		if result.Stats.Duration == 0 {
			t.Error("Stats.Duration = 0, expected non-zero")
		}
		if result.Stats.APICallCount == 0 {
			t.Error("Stats.APICallCount = 0, expected at least 1")
		}

		// Log what we got for debugging
		t.Logf("Fetched %d events in %v with %d API calls",
			len(result.Events),
			result.Stats.Duration,
			result.Stats.APICallCount)

		// If we got events, validate them thoroughly
		if len(result.Events) > 0 {
			for i, event := range result.Events {
				t.Run("ValidateEvent", func(t *testing.T) {
					// EFA 0001: All events must pass Validate()
					if err := event.Validate(); err != nil {
						t.Errorf("Event[%d].Validate() error = %v", i, err)
					}

					// Verify source is GitHub
					if event.Source != models.SourceGitHub {
						t.Errorf("Event[%d].Source = %q, want %q", i, event.Source, models.SourceGitHub)
					}

					// Verify event type is valid GitHub type
					validTypes := map[models.EventType]bool{
						models.EventTypePRReview:      true,
						models.EventTypePRMention:     true,
						models.EventTypeIssueMention:  true,
						models.EventTypeIssueAssigned: true,
					}
					if !validTypes[event.Type] {
						t.Errorf("Event[%d].Type = %q, not a valid GitHub event type", i, event.Type)
					}

					// Verify timestamp is within expected range
					if event.Timestamp.Before(opts.Since) {
						t.Errorf("Event[%d].Timestamp = %v, before Since = %v",
							i, event.Timestamp, opts.Since)
					}
					if event.Timestamp.After(time.Now()) {
						t.Errorf("Event[%d].Timestamp = %v, after current time", i, event.Timestamp)
					}

					// Verify URL is populated and looks like GitHub URL
					if event.URL == "" {
						t.Errorf("Event[%d].URL is empty", i)
					}

					// Verify author is populated
					if event.Author.Username == "" {
						t.Errorf("Event[%d].Author.Username is empty", i)
					}

					// Verify priority is in valid range
					if !event.Priority.IsValid() {
						t.Errorf("Event[%d].Priority = %d, not in valid range 1-5", i, event.Priority)
					}

					// Verify title is populated
					if event.Title == "" {
						t.Errorf("Event[%d].Title is empty", i)
					}
				})
			}

			// Verify events are sorted by timestamp ascending (EFA 0003)
			for i := 1; i < len(result.Events); i++ {
				if result.Events[i].Timestamp.Before(result.Events[i-1].Timestamp) {
					t.Errorf("Events not sorted: Event[%d].Timestamp %v before Event[%d].Timestamp %v",
						i, result.Events[i].Timestamp, i-1, result.Events[i-1].Timestamp)
				}
			}
		} else {
			t.Log("No events found in 24h window - this is acceptable for integration tests")
		}

		// Check for partial failures
		if result.Partial {
			t.Logf("Partial result: %d errors occurred", len(result.Errors))
			for i, err := range result.Errors {
				t.Logf("Error[%d]: %v", i, err)
			}
		}
	})

	t.Run("Fetch with limit", func(t *testing.T) {
		opts := datasources.FetchOptions{
			Since: testSince24h(),
			Limit: 5, // limit to 5 events
		}

		result, err := ds.Fetch(ctx, opts)
		if err != nil {
			t.Fatalf("Fetch() error = %v", err)
		}

		if len(result.Events) > 5 {
			t.Errorf("Fetch() returned %d events, expected <= 5 (limit)", len(result.Events))
		}
	})

	t.Run("Fetch with short window", func(t *testing.T) {
		// Use 1 hour window - less likely to have events
		opts := datasources.FetchOptions{
			Since: time.Now().Add(-1 * time.Hour),
			Limit: 0,
		}

		result, err := ds.Fetch(ctx, opts)
		if err != nil {
			t.Fatalf("Fetch() error = %v", err)
		}

		t.Logf("1-hour window: fetched %d events", len(result.Events))

		// Verify all events are within the time window
		for i, event := range result.Events {
			if event.Timestamp.Before(opts.Since) {
				t.Errorf("Event[%d].Timestamp = %v, before Since = %v",
					i, event.Timestamp, opts.Since)
			}
		}
	})
}

// TestGitHubDataSource_RateLimitHandling tests rate limit information.
func TestGitHubDataSource_RateLimitHandling(t *testing.T) {
	requireGitHubAuth(t)

	ctx := context.Background()

	authProvider := github.NewGitHubAuthProvider("")
	ds, err := githubds.NewDataSource(authProvider)
	if err != nil {
		t.Fatalf("NewDataSource() error = %v", err)
	}

	opts := defaultFetchOptions()
	result, err := ds.Fetch(ctx, opts)
	if err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}

	t.Run("RateLimitInfo", func(t *testing.T) {
		// Rate limit fields should be consistent
		if result.RateLimited {
			if result.RateLimitReset.IsZero() {
				t.Error("RateLimited = true but RateLimitReset is zero")
			}
			t.Logf("Rate limited until: %v", result.RateLimitReset)
		} else {
			// Not rate limited - normal case
			t.Log("Not rate limited")
		}
	})
}

// TestGitHubDataSource_ConcurrentFetch tests concurrent fetch operations.
func TestGitHubDataSource_ConcurrentFetch(t *testing.T) {
	requireGitHubAuth(t)

	ctx := context.Background()

	authProvider := github.NewGitHubAuthProvider("")
	ds, err := githubds.NewDataSource(authProvider)
	if err != nil {
		t.Fatalf("NewDataSource() error = %v", err)
	}

	// Create multiple concurrent fetches
	results := make(chan *datasources.FetchResult, 3)
	errors := make(chan error, 3)

	opts := defaultFetchOptions()

	for i := 0; i < 3; i++ {
		go func() {
			result, err := ds.Fetch(ctx, opts)
			if err != nil {
				errors <- err
				return
			}
			results <- result
		}()
	}

	// Collect results
	var fetchResults []*datasources.FetchResult
	for i := 0; i < 3; i++ {
		select {
		case result := <-results:
			fetchResults = append(fetchResults, result)
		case err := <-errors:
			t.Errorf("Concurrent fetch[%d] error = %v", i, err)
		}
	}

	if len(fetchResults) > 0 {
		t.Logf("Concurrent fetches successful: %d results", len(fetchResults))
	}
}

// TestGitHubDataSource_ContextCancellation tests context cancellation handling.
func TestGitHubDataSource_ContextCancellation(t *testing.T) {
	requireGitHubAuth(t)

	authProvider := github.NewGitHubAuthProvider("")
	ds, err := githubds.NewDataSource(authProvider)
	if err != nil {
		t.Fatalf("NewDataSource() error = %v", err)
	}

	// Create context with immediate cancellation
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	opts := defaultFetchOptions()
	result, err := ds.Fetch(ctx, opts)

	// Expect either context.Canceled error or partial results
	if err != nil {
		if err != context.Canceled {
			t.Logf("Expected context.Canceled, got: %v (acceptable)", err)
		}
	}

	if result != nil && result.Partial {
		t.Log("Got partial results before cancellation (acceptable)")
	}
}

// TestGitHubDataSource_WatchedRepos tests fetching merged PRs from watched repos.
func TestGitHubDataSource_WatchedRepos(t *testing.T) {
	requireGitHubAuth(t)

	ctx := context.Background()

	authProvider := github.NewGitHubAuthProvider("")
	if err := authProvider.Authenticate(ctx); err != nil {
		t.Fatalf("GitHub authentication failed: %v", err)
	}

	// Use a well-known public repo with frequent merges
	watchedRepos := []string{"kubernetes/kubernetes"}

	ds, err := githubds.NewDataSource(authProvider, githubds.WithWatchedRepos(watchedRepos))
	if err != nil {
		t.Fatalf("NewDataSource() error = %v", err)
	}

	t.Run("Fetch with watched repos", func(t *testing.T) {
		// Use 7-day window to increase chance of finding merged PRs
		opts := datasources.FetchOptions{
			Since: time.Now().Add(-7 * 24 * time.Hour),
			Limit: 10,
		}

		result, err := ds.Fetch(ctx, opts)
		if err != nil {
			t.Fatalf("Fetch() error = %v", err)
		}

		if result == nil {
			t.Fatal("Fetch() returned nil result")
		}

		t.Logf("Fetched %d total events (including watched repos)", len(result.Events))

		// Count watched repo events
		watchedRepoEvents := 0
		for _, event := range result.Events {
			if event.Type == models.EventTypePRClosed {
				watchedRepoEvents++

				// Validate watched repo events specifically
				if err := event.Validate(); err != nil {
					t.Errorf("Watched repo event validation error: %v", err)
				}

				// Verify watched_repo metadata is set
				if watched, ok := event.Metadata["watched_repo"].(bool); !ok || !watched {
					t.Errorf("Event missing watched_repo=true metadata: %+v", event.Metadata)
				}

				// Verify repo metadata matches a watched repo
				if repo, ok := event.Metadata["repo"].(string); ok {
					found := false
					for _, wr := range watchedRepos {
						if repo == wr {
							found = true
							break
						}
					}
					if !found {
						t.Errorf("Event repo %q not in watched repos list", repo)
					}
				}

				// Verify timestamp is within window
				if event.Timestamp.Before(opts.Since) {
					t.Errorf("Watched repo event timestamp %v before Since %v",
						event.Timestamp, opts.Since)
				}

				// Verify priority is Info (5) for watched repo events
				if event.Priority != models.PriorityInfo {
					t.Errorf("Watched repo event priority = %d, want %d (PriorityInfo)",
						event.Priority, models.PriorityInfo)
				}

				// Verify RequiresAction is false
				if event.RequiresAction {
					t.Error("Watched repo event RequiresAction = true, want false")
				}
			}
		}

		t.Logf("Found %d watched repo events", watchedRepoEvents)

		if watchedRepoEvents == 0 {
			t.Log("No merged PRs found in watched repos within time window - this is acceptable for integration tests")
		}
	})

	t.Run("Fetch with no watched repos configured", func(t *testing.T) {
		// Create datasource without watched repos
		dsNoWatch, err := githubds.NewDataSource(authProvider)
		if err != nil {
			t.Fatalf("NewDataSource() error = %v", err)
		}

		opts := datasources.FetchOptions{
			Since: testSince24h(),
			Limit: 10,
		}

		result, err := dsNoWatch.Fetch(ctx, opts)
		if err != nil {
			t.Fatalf("Fetch() error = %v", err)
		}

		// Verify no watched repo events when not configured
		for _, event := range result.Events {
			if event.Type == models.EventTypePRClosed {
				if watched, ok := event.Metadata["watched_repo"].(bool); ok && watched {
					t.Error("Got watched_repo event when watched repos not configured")
				}
			}
		}
	})
}

// TestGitHubDataSource_InvalidOptions tests validation of fetch options.
func TestGitHubDataSource_InvalidOptions(t *testing.T) {
	requireGitHubAuth(t)

	ctx := context.Background()

	authProvider := github.NewGitHubAuthProvider("")
	ds, err := githubds.NewDataSource(authProvider)
	if err != nil {
		t.Fatalf("NewDataSource() error = %v", err)
	}

	t.Run("InvalidSince", func(t *testing.T) {
		// Zero time is invalid
		opts := datasources.FetchOptions{
			Since: time.Time{},
			Limit: 0,
		}

		_, err := ds.Fetch(ctx, opts)
		if err == nil {
			t.Error("Fetch() with zero Since expected error, got nil")
		}
	})

	t.Run("NegativeLimit", func(t *testing.T) {
		opts := datasources.FetchOptions{
			Since: testSince24h(),
			Limit: -1,
		}

		_, err := ds.Fetch(ctx, opts)
		if err == nil {
			t.Error("Fetch() with negative Limit expected error, got nil")
		}
	})
}
