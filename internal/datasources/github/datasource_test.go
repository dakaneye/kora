package github

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/dakaneye/kora/internal/auth"
	"github.com/dakaneye/kora/internal/datasources"
	"github.com/dakaneye/kora/internal/models"
)

// TestNewDataSource verifies datasource construction validation.
func TestNewDataSource(t *testing.T) {
	tests := []struct {
		name        string
		authService auth.Service
		wantErr     bool
	}{
		{
			name:        "valid github auth provider",
			authService: auth.ServiceGitHub,
			wantErr:     false,
		},
		{
			name:        "invalid slack auth provider",
			authService: auth.ServiceSlack,
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			authProvider := newMockAuthProvider(tt.authService)
			ds, err := NewDataSource(authProvider)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				if ds != nil {
					t.Error("expected nil datasource on error")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if ds == nil {
					t.Error("expected non-nil datasource")
				}
				if ds.Name() != "github" {
					t.Errorf("expected name 'github', got %q", ds.Name())
				}
				if ds.Service() != models.SourceGitHub {
					t.Errorf("expected service %q, got %q", models.SourceGitHub, ds.Service())
				}
			}
		})
	}
}

// TestFetch_AllSearchesSucceed verifies that all events are returned and sorted when all searches succeed.
func TestFetch_AllSearchesSucceed(t *testing.T) {
	ctx := context.Background()
	authProvider := newMockAuthProvider(auth.ServiceGitHub)

	// Load test data
	prReviews := loadTestData(t, "pr_reviews.json")
	prMentions := loadTestData(t, "pr_mentions.json")
	issueMentions := loadTestData(t, "issue_mentions.json")
	assignedIssues := loadTestData(t, "assigned_issues.json")

	// Configure mock responses for each search query
	cred := authProvider.credential
	cred.setResponseForQuery("review-requested", prReviews)
	cred.setResponseForQuery("mentions:pr", prMentions)
	cred.setResponseForQuery("mentions:issue", issueMentions)
	cred.setResponseForQuery("assignee", assignedIssues)

	ds, err := NewDataSource(authProvider)
	if err != nil {
		t.Fatalf("NewDataSource failed: %v", err)
	}

	// Use a fixed Since time that's before all test data timestamps.
	// Test data has timestamps on 2025-12-06 starting at 03:00 UTC.
	opts := datasources.FetchOptions{
		Since: time.Date(2025, 12, 5, 0, 0, 0, 0, time.UTC),
	}

	result, err := ds.Fetch(ctx, opts)
	if err != nil {
		t.Fatalf("Fetch failed: %v", err)
	}

	// Should have 6 total events (2 PR reviews, 1 PR mention, 1 issue mention, 2 assigned issues)
	expectedCount := 6
	if len(result.Events) != expectedCount {
		t.Errorf("expected %d events, got %d", expectedCount, len(result.Events))
	}

	// Verify all events pass validation (EFA 0001 requirement)
	for i, event := range result.Events {
		if err := event.Validate(); err != nil {
			t.Errorf("event %d failed validation: %v", i, err)
		}
	}

	// Verify events are sorted by timestamp ascending
	for i := 1; i < len(result.Events); i++ {
		if result.Events[i].Timestamp.Before(result.Events[i-1].Timestamp) {
			t.Errorf("events not sorted: event[%d]=%v before event[%d]=%v",
				i, result.Events[i].Timestamp, i-1, result.Events[i-1].Timestamp)
		}
	}

	// Verify statistics
	if result.Stats.APICallCount != 4 {
		t.Errorf("expected 4 API calls, got %d", result.Stats.APICallCount)
	}
	if result.Stats.EventsFetched != expectedCount {
		t.Errorf("expected %d events fetched, got %d", expectedCount, result.Stats.EventsFetched)
	}
	if result.Stats.EventsReturned != expectedCount {
		t.Errorf("expected %d events returned, got %d", expectedCount, result.Stats.EventsReturned)
	}
	if result.Partial {
		t.Error("expected Partial=false on full success")
	}
	if len(result.Errors) > 0 {
		t.Errorf("expected no errors, got %v", result.Errors)
	}
}

// TestFetch_PartialSuccess verifies that partial results are returned when some searches fail.
func TestFetch_PartialSuccess(t *testing.T) {
	ctx := context.Background()
	authProvider := newMockAuthProvider(auth.ServiceGitHub)

	prReviews := loadTestData(t, "pr_reviews.json")
	issueMentions := loadTestData(t, "issue_mentions.json")

	// Configure mock responses: PR reviews and issue mentions succeed, others fail
	cred := authProvider.credential
	cred.setResponseForQuery("review-requested", prReviews)
	cred.setErrorForQuery("mentions:pr", errors.New("rate limited"))
	cred.setResponseForQuery("mentions:issue", issueMentions)
	cred.setErrorForQuery("assignee", errors.New("service unavailable"))

	ds, err := NewDataSource(authProvider)
	if err != nil {
		t.Fatalf("NewDataSource failed: %v", err)
	}

	// Use fixed time before test data timestamps (2025-12-06 03:00 UTC)
	opts := datasources.FetchOptions{
		Since: time.Date(2025, 12, 5, 0, 0, 0, 0, time.UTC),
	}

	result, err := ds.Fetch(ctx, opts)
	if err != nil {
		t.Fatalf("Fetch failed: %v", err)
	}

	// Should have 3 events (2 PR reviews, 1 issue mention)
	expectedCount := 3
	if len(result.Events) != expectedCount {
		t.Errorf("expected %d events, got %d", expectedCount, len(result.Events))
	}

	// Verify partial flag is set
	if !result.Partial {
		t.Error("expected Partial=true when some searches fail")
	}

	// Verify errors are captured
	if len(result.Errors) != 2 {
		t.Errorf("expected 2 errors, got %d: %v", len(result.Errors), result.Errors)
	}

	// Verify all returned events are valid
	for i, event := range result.Events {
		if err := event.Validate(); err != nil {
			t.Errorf("event %d failed validation: %v", i, err)
		}
	}
}

// TestFetch_SinceFiltering verifies that FetchOptions.Since correctly filters events.
func TestFetch_SinceFiltering(t *testing.T) {
	ctx := context.Background()
	authProvider := newMockAuthProvider(auth.ServiceGitHub)

	prReviews := loadTestData(t, "pr_reviews.json")
	emptyResults := loadTestData(t, "empty_results.json")

	cred := authProvider.credential
	cred.setResponseForQuery("review-requested", prReviews)
	cred.setResponseForQuery("mentions:pr", emptyResults)
	cred.setResponseForQuery("mentions:issue", emptyResults)
	cred.setResponseForQuery("assignee", emptyResults)

	ds, err := NewDataSource(authProvider)
	if err != nil {
		t.Fatalf("NewDataSource failed: %v", err)
	}

	// Set Since to a time after the first PR review but before the second
	// pr_reviews.json has events at 08:00:00 and 07:30:00
	since := time.Date(2025, 12, 6, 7, 45, 0, 0, time.UTC)

	opts := datasources.FetchOptions{
		Since: since,
	}

	result, err := ds.Fetch(ctx, opts)
	if err != nil {
		t.Fatalf("Fetch failed: %v", err)
	}

	// Should have 1 event (only the 08:00:00 PR review)
	expectedCount := 1
	if len(result.Events) != expectedCount {
		t.Errorf("expected %d events after filtering, got %d", expectedCount, len(result.Events))
	}

	// Verify the returned event is after 'since'
	if len(result.Events) > 0 {
		event := result.Events[0]
		if !event.Timestamp.After(since) {
			t.Errorf("event timestamp %v should be after since %v", event.Timestamp, since)
		}
	}
}

// TestFetch_Deduplication verifies that duplicate URLs are removed.
func TestFetch_Deduplication(t *testing.T) {
	ctx := context.Background()
	authProvider := newMockAuthProvider(auth.ServiceGitHub)

	// Use the same PR reviews data for both searches to create duplicates
	prReviews := loadTestData(t, "pr_reviews.json")
	emptyResults := loadTestData(t, "empty_results.json")

	cred := authProvider.credential
	cred.setResponseForQuery("review-requested", prReviews)
	cred.setResponseForQuery("mentions:pr", prReviews) // Same data
	cred.setResponseForQuery("mentions:pr", emptyResults)
	cred.setResponseForQuery("mentions:issue", emptyResults)
	cred.setResponseForQuery("assignee", emptyResults)

	ds, err := NewDataSource(authProvider)
	if err != nil {
		t.Fatalf("NewDataSource failed: %v", err)
	}

	// Use fixed time before test data timestamps
	opts := datasources.FetchOptions{
		Since: time.Date(2025, 12, 5, 0, 0, 0, 0, time.UTC),
	}

	result, err := ds.Fetch(ctx, opts)
	if err != nil {
		t.Fatalf("Fetch failed: %v", err)
	}

	// Should have 2 unique events even though returned in 2 searches
	expectedCount := 2
	if len(result.Events) != expectedCount {
		t.Errorf("expected %d events after deduplication, got %d", expectedCount, len(result.Events))
	}

	// Verify no duplicate URLs
	seen := make(map[string]bool)
	for _, event := range result.Events {
		if seen[event.URL] {
			t.Errorf("duplicate URL found: %s", event.URL)
		}
		seen[event.URL] = true
	}
}

// TestFetch_OrgFilter verifies that org filter is applied to search queries.
func TestFetch_OrgFilter(t *testing.T) {
	ctx := context.Background()
	authProvider := newMockAuthProvider(auth.ServiceGitHub)

	emptyResults := loadTestData(t, "empty_results.json")

	// Configure mock to expect org filter in queries
	cred := authProvider.credential
	cred.setResponseForQuery("review-requested", emptyResults)
	cred.setResponseForQuery("mentions:pr", emptyResults)
	cred.setResponseForQuery("mentions:issue", emptyResults)
	cred.setResponseForQuery("mentions:pr", emptyResults)
	cred.setResponseForQuery("mentions:issue", emptyResults)
	cred.setResponseForQuery("assignee", emptyResults)

	ds, err := NewDataSource(authProvider, WithOrgs([]string{"example"}))
	if err != nil {
		t.Fatalf("NewDataSource failed: %v", err)
	}

	// Use fixed time before test data timestamps
	opts := datasources.FetchOptions{
		Since: time.Date(2025, 12, 5, 0, 0, 0, 0, time.UTC),
	}

	_, err = ds.Fetch(ctx, opts)
	if err != nil {
		t.Fatalf("Fetch failed: %v", err)
	}

	// If we get here, the mock received the expected queries with org filters
}

// TestTransform_EventTypeAndPriority verifies correct event type and priority assignment.
func TestTransform_EventTypeAndPriority(t *testing.T) {
	tests := []struct {
		name         string
		testDataFile string
		expectedType models.EventType
		expectedPri  models.Priority
	}{
		{
			name:         "PR review request",
			testDataFile: "pr_reviews.json",
			expectedType: models.EventTypePRReview,
			expectedPri:  models.PriorityHigh, // Per EFA 0001
		},
		{
			name:         "PR mention",
			testDataFile: "pr_mentions.json",
			expectedType: models.EventTypePRMention,
			expectedPri:  models.PriorityMedium, // Per EFA 0001
		},
		{
			name:         "issue mention",
			testDataFile: "issue_mentions.json",
			expectedType: models.EventTypeIssueMention,
			expectedPri:  models.PriorityMedium, // Per EFA 0001
		},
		{
			name:         "assigned issue",
			testDataFile: "assigned_issues.json",
			expectedType: models.EventTypeIssueAssigned,
			expectedPri:  models.PriorityMedium, // Per EFA 0001
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			authProvider := newMockAuthProvider(auth.ServiceGitHub)

			testData := loadTestData(t, tt.testDataFile)
			emptyResults := loadTestData(t, "empty_results.json")

			cred := authProvider.credential
			// Set the test data for the appropriate search
			switch tt.expectedType {
			case models.EventTypePRReview:
				cred.setResponseForQuery("review-requested", testData)
				cred.setResponseForQuery("mentions:pr", emptyResults)
				cred.setResponseForQuery("mentions:issue", emptyResults)
				cred.setResponseForQuery("assignee", emptyResults)
			case models.EventTypePRMention:
				cred.setResponseForQuery("review-requested", emptyResults)
				cred.setResponseForQuery("mentions:pr", testData)
				cred.setResponseForQuery("mentions:issue", emptyResults)
				cred.setResponseForQuery("assignee", emptyResults)
			case models.EventTypeIssueMention:
				cred.setResponseForQuery("review-requested", emptyResults)
				cred.setResponseForQuery("mentions:pr", emptyResults)
				cred.setResponseForQuery("mentions:issue", testData)
				cred.setResponseForQuery("assignee", emptyResults)
			case models.EventTypeIssueAssigned:
				cred.setResponseForQuery("review-requested", emptyResults)
				cred.setResponseForQuery("mentions:pr", emptyResults)
				cred.setResponseForQuery("mentions:issue", emptyResults)
				cred.setResponseForQuery("assignee", testData)
			}

			ds, err := NewDataSource(authProvider)
			if err != nil {
				t.Fatalf("NewDataSource failed: %v", err)
			}

			// Use a fixed Since time before all test data timestamps
			// Test data has timestamps starting at 2025-12-06 03:00 UTC
			opts := datasources.FetchOptions{
				Since: time.Date(2025, 12, 5, 0, 0, 0, 0, time.UTC),
			}

			result, err := ds.Fetch(ctx, opts)
			if err != nil {
				t.Fatalf("Fetch failed: %v", err)
			}

			if len(result.Events) == 0 {
				t.Fatal("expected at least one event")
			}

			event := result.Events[0]
			if event.Type != tt.expectedType {
				t.Errorf("expected type %q, got %q", tt.expectedType, event.Type)
			}
			if event.Priority != tt.expectedPri {
				t.Errorf("expected priority %d, got %d", tt.expectedPri, event.Priority)
			}
		})
	}
}

// TestTransform_TitleTruncation verifies that titles exceeding 100 chars are truncated.
func TestTransform_TitleTruncation(t *testing.T) {
	ctx := context.Background()
	authProvider := newMockAuthProvider(auth.ServiceGitHub)

	// pr_reviews.json contains a long title
	prReviews := loadTestData(t, "pr_reviews.json")
	emptyResults := loadTestData(t, "empty_results.json")

	cred := authProvider.credential
	cred.setResponseForQuery("review-requested", prReviews)
	cred.setResponseForQuery("mentions:pr", emptyResults)
	cred.setResponseForQuery("mentions:issue", emptyResults)
	cred.setResponseForQuery("assignee", emptyResults)

	ds, err := NewDataSource(authProvider)
	if err != nil {
		t.Fatalf("NewDataSource failed: %v", err)
	}

	// Use fixed time before test data timestamps
	opts := datasources.FetchOptions{
		Since: time.Date(2025, 12, 5, 0, 0, 0, 0, time.UTC),
	}

	result, err := ds.Fetch(ctx, opts)
	if err != nil {
		t.Fatalf("Fetch failed: %v", err)
	}

	// Find the event with the long title (PR #456)
	var longEvent *models.Event
	for i := range result.Events {
		if result.Events[i].URL == "https://github.com/example/repo2/pull/456" {
			longEvent = &result.Events[i]
			break
		}
	}

	if longEvent == nil {
		t.Fatal("expected to find event with long title")
	}

	// Verify title is truncated to 100 chars per EFA 0001
	if len(longEvent.Title) > 100 {
		t.Errorf("title length %d exceeds 100 characters", len(longEvent.Title))
	}

	// Verify it ends with "..." when truncated
	if len(longEvent.Title) == 100 && longEvent.Title[97:] != "..." {
		t.Error("truncated title should end with '...'")
	}
}

// TestTransform_MetadataKeys verifies that metadata contains only EFA 0001 allowed keys.
func TestTransform_MetadataKeys(t *testing.T) {
	ctx := context.Background()
	authProvider := newMockAuthProvider(auth.ServiceGitHub)

	prReviews := loadTestData(t, "pr_reviews.json")
	emptyResults := loadTestData(t, "empty_results.json")

	cred := authProvider.credential
	cred.setResponseForQuery("review-requested", prReviews)
	cred.setResponseForQuery("mentions:pr", emptyResults)
	cred.setResponseForQuery("mentions:issue", emptyResults)
	cred.setResponseForQuery("assignee", emptyResults)

	ds, err := NewDataSource(authProvider)
	if err != nil {
		t.Fatalf("NewDataSource failed: %v", err)
	}

	// Use fixed time before test data timestamps
	opts := datasources.FetchOptions{
		Since: time.Date(2025, 12, 5, 0, 0, 0, 0, time.UTC),
	}

	result, err := ds.Fetch(ctx, opts)
	if err != nil {
		t.Fatalf("Fetch failed: %v", err)
	}

	// EFA 0001 allowed metadata keys for GitHub (subset used by current implementation)
	allowedKeys := map[string]bool{
		"repo":               true,
		"number":             true,
		"state":              true,
		"author_login":       true,
		"user_relationships": true,
		"labels":             true,
	}

	for i, event := range result.Events {
		for key := range event.Metadata {
			if !allowedKeys[key] {
				t.Errorf("event %d has disallowed metadata key: %q", i, key)
			}
		}

		// Verify required keys are present
		if _, ok := event.Metadata["repo"]; !ok {
			t.Errorf("event %d missing required metadata key 'repo'", i)
		}
		if _, ok := event.Metadata["number"]; !ok {
			t.Errorf("event %d missing required metadata key 'number'", i)
		}
		if _, ok := event.Metadata["state"]; !ok {
			t.Errorf("event %d missing required metadata key 'state'", i)
		}
	}
}

// TestFetch_AuthFailure verifies proper error handling when auth fails.
func TestFetch_AuthFailure(t *testing.T) {
	ctx := context.Background()
	authProvider := newMockAuthProvider(auth.ServiceGitHub)
	authProvider.authenticated = false

	ds, err := NewDataSource(authProvider)
	if err != nil {
		t.Fatalf("NewDataSource failed: %v", err)
	}

	// Use fixed time (doesn't matter for auth test, but keeping consistent)
	opts := datasources.FetchOptions{
		Since: time.Date(2025, 12, 5, 0, 0, 0, 0, time.UTC),
	}

	_, err = ds.Fetch(ctx, opts)

	// Should return ErrNotAuthenticated per EFA 0003
	if !errors.Is(err, datasources.ErrNotAuthenticated) {
		t.Errorf("expected ErrNotAuthenticated, got %v", err)
	}
}

// TestFetch_WrongAuthProvider verifies error when non-GitHub auth provider is used.
func TestFetch_WrongAuthProvider(t *testing.T) {
	authProvider := newMockAuthProvider(auth.ServiceSlack)

	_, err := NewDataSource(authProvider)
	if err == nil {
		t.Error("expected error when using wrong auth provider, got nil")
	}
}

// TestFetch_InvalidOptions verifies that invalid FetchOptions are rejected.
func TestFetch_InvalidOptions(t *testing.T) {
	ctx := context.Background()
	authProvider := newMockAuthProvider(auth.ServiceGitHub)

	ds, err := NewDataSource(authProvider)
	if err != nil {
		t.Fatalf("NewDataSource failed: %v", err)
	}

	// FetchOptions with zero Since is invalid per datasources.FetchOptions.Validate()
	opts := datasources.FetchOptions{
		Since: time.Time{},
	}

	_, err = ds.Fetch(ctx, opts)
	if err == nil {
		t.Error("expected error for invalid FetchOptions, got nil")
	}
}

// loadTestData loads test JSON data from testdata directory.
func loadTestData(t *testing.T, filename string) []byte {
	t.Helper()
	data, err := os.ReadFile("testdata/" + filename)
	if err != nil {
		t.Fatalf("failed to load test data %s: %v", filename, err)
	}
	return data
}
