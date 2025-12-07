//go:build integration

package integration

import (
	"context"
	"testing"

	"github.com/dakaneye/kora/internal/auth/github"
	"github.com/dakaneye/kora/internal/auth/keychain"
	"github.com/dakaneye/kora/internal/auth/slack"
	"github.com/dakaneye/kora/internal/datasources"
	githubds "github.com/dakaneye/kora/internal/datasources/github"
	slackds "github.com/dakaneye/kora/internal/datasources/slack"
)

// TestPartialFailure_SlackDisabled tests that GitHub events are returned
// even when Slack is disabled or unavailable.
func TestPartialFailure_SlackDisabled(t *testing.T) {
	requireGitHubAuth(t)

	ctx := context.Background()

	// Initialize only GitHub datasource (Slack intentionally disabled)
	ghAuth := github.NewGitHubAuthProvider("")
	if err := ghAuth.Authenticate(ctx); err != nil {
		t.Fatalf("GitHub authentication failed: %v", err)
	}

	ghDS, err := githubds.NewDataSource(ghAuth)
	if err != nil {
		t.Fatalf("NewDataSource(github) error = %v", err)
	}

	// Create runner with only GitHub
	runner := datasources.NewRunner([]datasources.DataSource{ghDS})

	opts := defaultFetchOptions()
	result, err := runner.Run(ctx, opts)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	// Verify we got GitHub results
	t.Run("GitHub events returned", func(t *testing.T) {
		if ghResult, ok := result.SourceResults["github"]; ok {
			t.Logf("GitHub returned %d events", len(ghResult.Events))
		} else {
			t.Error("No GitHub results in SourceResults map")
		}
	})

	// Verify Slack is not in results (since we didn't include it)
	t.Run("Slack not in results", func(t *testing.T) {
		if _, ok := result.SourceResults["slack"]; ok {
			t.Error("Slack should not be in results when not included in runner")
		}
		if _, ok := result.SourceErrors["slack"]; ok {
			t.Error("Slack should not be in errors when not included in runner")
		}
	})

	// Verify success status (no errors since Slack wasn't attempted)
	t.Run("Run succeeds with only GitHub", func(t *testing.T) {
		if !result.Success() && !result.Partial() {
			t.Error("Run() should succeed when only GitHub is enabled")
		}
		if result.Partial() {
			t.Logf("Partial result due to: %v", result.SourceErrors)
		}
	})
}

// TestPartialFailure_GitHubDisabled tests that Slack events are returned
// even when GitHub is disabled or unavailable.
func TestPartialFailure_GitHubDisabled(t *testing.T) {
	requireSlackAuth(t)

	ctx := context.Background()

	// Initialize only Slack datasource (GitHub intentionally disabled)
	kc := keychain.NewMacOSKeychain("")
	slackAuth := slack.NewSlackAuthProvider(kc, nil)
	if err := slackAuth.Authenticate(ctx); err != nil {
		t.Fatalf("Slack authentication failed: %v", err)
	}

	slackDS, err := slackds.NewDataSource(slackAuth)
	if err != nil {
		t.Fatalf("NewDataSource(slack) error = %v", err)
	}

	// Create runner with only Slack
	runner := datasources.NewRunner([]datasources.DataSource{slackDS})

	opts := defaultFetchOptions()
	result, err := runner.Run(ctx, opts)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	// Verify we got Slack results
	t.Run("Slack events returned", func(t *testing.T) {
		if slackResult, ok := result.SourceResults["slack"]; ok {
			t.Logf("Slack returned %d events", len(slackResult.Events))
		} else {
			t.Error("No Slack results in SourceResults map")
		}
	})

	// Verify GitHub is not in results
	t.Run("GitHub not in results", func(t *testing.T) {
		if _, ok := result.SourceResults["github"]; ok {
			t.Error("GitHub should not be in results when not included in runner")
		}
		if _, ok := result.SourceErrors["github"]; ok {
			t.Error("GitHub should not be in errors when not included in runner")
		}
	})

	// Verify success status
	t.Run("Run succeeds with only Slack", func(t *testing.T) {
		if !result.Success() && !result.Partial() {
			t.Error("Run() should succeed when only Slack is enabled")
		}
	})
}

// TestPartialFailure_BothSources tests runner with both sources when both are available.
func TestPartialFailure_BothSources(t *testing.T) {
	// This test only runs if both auth methods are available
	requireGitHubAuth(t)
	requireSlackAuth(t)

	ctx := context.Background()

	// Initialize GitHub
	ghAuth := github.NewGitHubAuthProvider("")
	if err := ghAuth.Authenticate(ctx); err != nil {
		t.Fatalf("GitHub authentication failed: %v", err)
	}
	ghDS, err := githubds.NewDataSource(ghAuth)
	if err != nil {
		t.Fatalf("NewDataSource(github) error = %v", err)
	}

	// Initialize Slack
	kc := keychain.NewMacOSKeychain("")
	slackAuth := slack.NewSlackAuthProvider(kc, nil)
	if err := slackAuth.Authenticate(ctx); err != nil {
		t.Fatalf("Slack authentication failed: %v", err)
	}
	slackDS, err := slackds.NewDataSource(slackAuth)
	if err != nil {
		t.Fatalf("NewDataSource(slack) error = %v", err)
	}

	// Create runner with both sources
	runner := datasources.NewRunner([]datasources.DataSource{ghDS, slackDS})

	opts := defaultFetchOptions()
	result, err := runner.Run(ctx, opts)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	t.Run("Both sources attempted", func(t *testing.T) {
		// Should have results or errors for both
		ghAttempted := false
		slackAttempted := false

		if _, ok := result.SourceResults["github"]; ok {
			ghAttempted = true
		}
		if _, ok := result.SourceErrors["github"]; ok {
			ghAttempted = true
		}

		if _, ok := result.SourceResults["slack"]; ok {
			slackAttempted = true
		}
		if _, ok := result.SourceErrors["slack"]; ok {
			slackAttempted = true
		}

		if !ghAttempted {
			t.Error("GitHub was not attempted")
		}
		if !slackAttempted {
			t.Error("Slack was not attempted")
		}

		t.Logf("GitHub: %d events, Slack: %d events",
			getEventCount(result.SourceResults["github"]),
			getEventCount(result.SourceResults["slack"]))
	})

	t.Run("Combined events sorted", func(t *testing.T) {
		// Verify combined events are sorted by timestamp
		for i := 1; i < len(result.Events); i++ {
			if result.Events[i].Timestamp.Before(result.Events[i-1].Timestamp) {
				t.Errorf("Combined events not sorted: Event[%d].Timestamp %v before Event[%d].Timestamp %v",
					i, result.Events[i].Timestamp, i-1, result.Events[i-1].Timestamp)
			}
		}
	})

	t.Run("Events from both sources", func(t *testing.T) {
		// Count events by source
		ghCount := 0
		slackCount := 0

		for _, event := range result.Events {
			switch event.Source {
			case "github":
				ghCount++
			case "slack":
				slackCount++
			default:
				t.Errorf("Unknown event source: %s", event.Source)
			}
		}

		t.Logf("Combined result: %d GitHub events, %d Slack events", ghCount, slackCount)
	})
}

// TestPartialFailure_OneSourceFails tests behavior when one source fails completely.
// This test simulates a failure by using an invalid auth configuration.
func TestPartialFailure_OneSourceFails(t *testing.T) {
	requireGitHubAuth(t)

	ctx := context.Background()

	// Initialize GitHub (should succeed)
	ghAuth := github.NewGitHubAuthProvider("")
	if err := ghAuth.Authenticate(ctx); err != nil {
		t.Fatalf("GitHub authentication failed: %v", err)
	}
	ghDS, err := githubds.NewDataSource(ghAuth)
	if err != nil {
		t.Fatalf("NewDataSource(github) error = %v", err)
	}

	// Initialize Slack with invalid keychain (should fail)
	// Use a keychain that will fail to authenticate
	kc := keychain.NewMacOSKeychain("")
	slackAuth := slack.NewSlackAuthProvider(kc, nil)
	// Don't authenticate - let it fail during fetch

	// Create Slack datasource anyway (auth failure will happen during fetch)
	slackDS, err := slackds.NewDataSource(slackAuth)
	if err != nil {
		// If we can't even create it, just test with GitHub only
		t.Log("Cannot create Slack datasource (expected if not authenticated)")
		runner := datasources.NewRunner([]datasources.DataSource{ghDS})
		opts := defaultFetchOptions()
		result, err := runner.Run(ctx, opts)
		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}

		if !result.Success() {
			t.Error("Should succeed with only GitHub when Slack fails to initialize")
		}
		return
	}

	// Create runner with both sources
	runner := datasources.NewRunner([]datasources.DataSource{ghDS, slackDS})

	opts := defaultFetchOptions()
	result, err := runner.Run(ctx, opts)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	t.Run("Partial success when one fails", func(t *testing.T) {
		// Should have GitHub results
		if _, ok := result.SourceResults["github"]; !ok {
			t.Error("Expected GitHub results even if Slack fails")
		}

		// Should have Slack error if it failed
		hasSlackError := false
		if _, ok := result.SourceErrors["slack"]; ok {
			hasSlackError = true
			t.Logf("Slack error (expected): %v", result.SourceErrors["slack"])
		}

		// If Slack has results, that's fine too
		if _, ok := result.SourceResults["slack"]; ok {
			t.Log("Slack succeeded (auth was available)")
		}

		// Should be partial if Slack failed but GitHub succeeded
		if hasSlackError && result.Partial() {
			t.Log("✓ Correctly reports partial success")
		}
	})

	t.Run("GitHub events still returned", func(t *testing.T) {
		ghCount := 0
		for _, event := range result.Events {
			if event.Source == "github" {
				ghCount++
			}
		}
		t.Logf("Got %d GitHub events despite Slack failure", ghCount)
	})
}

// getEventCount safely gets event count from a FetchResult.
func getEventCount(result *datasources.FetchResult) int {
	if result == nil {
		return 0
	}
	return len(result.Events)
}

// TestPartialFailure_EmptyResults tests behavior when datasources return no events.
func TestPartialFailure_EmptyResults(t *testing.T) {
	requireGitHubAuth(t)

	ctx := context.Background()

	ghAuth := github.NewGitHubAuthProvider("")
	ghDS, err := githubds.NewDataSource(ghAuth)
	if err != nil {
		t.Fatalf("NewDataSource(github) error = %v", err)
	}

	runner := datasources.NewRunner([]datasources.DataSource{ghDS})

	// Use a very short time window - likely to return no events
	opts := datasources.FetchOptions{
		Since: testSince24h().Add(23*3600*1e9 + 59*60*1e9), // 1 minute ago
		Limit: 0,
	}

	result, err := runner.Run(ctx, opts)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	t.Run("Empty results are success", func(t *testing.T) {
		if len(result.Events) == 0 {
			t.Log("No events in short window (expected)")
		}

		// Empty results should still be considered success if no errors
		if len(result.SourceErrors) == 0 {
			if !result.Success() {
				t.Error("Empty results without errors should be Success = true")
			}
		}
	})
}
