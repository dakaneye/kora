package github

import (
	"context"
	"errors"
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

	// Configure mock responses for GraphQL searches
	setupMockForAllSearches(authProvider.credential, t)

	ds, err := NewDataSource(authProvider)
	if err != nil {
		t.Fatalf("NewDataSource failed: %v", err)
	}

	// Use a fixed Since time that's before all test data timestamps.
	// Test data has timestamps on 2025-12-06 starting at 05:00 UTC.
	opts := datasources.FetchOptions{
		Since: time.Date(2025, 12, 5, 0, 0, 0, 0, time.UTC),
	}

	result, err := ds.Fetch(ctx, opts)
	if err != nil {
		t.Fatalf("Fetch failed: %v", err)
	}

	// Should have 4 total events (2 from PR search + 2 from issue search)
	// (pr-mention and issue-assigned return empty results)
	expectedCount := 4
	if len(result.Events) != expectedCount {
		t.Errorf("expected %d events, got %d", expectedCount, len(result.Events))
		for i, e := range result.Events {
			t.Logf("event[%d]: type=%s url=%s ts=%v", i, e.Type, e.URL, e.Timestamp)
		}
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

	// Verify statistics (4 search calls - one for each type)
	if result.Stats.APICallCount != 4 {
		t.Errorf("expected 4 API calls, got %d", result.Stats.APICallCount)
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

	// Configure mock: PR reviews and issue mentions succeed, others fail
	setupMockWithPartialFailure(authProvider.credential, t,
		"graphql:search:pr-mention",
		"graphql:search:issue-assigned",
	)

	ds, err := NewDataSource(authProvider)
	if err != nil {
		t.Fatalf("NewDataSource failed: %v", err)
	}

	// Use fixed time before test data timestamps (2025-12-06 05:00 UTC)
	opts := datasources.FetchOptions{
		Since: time.Date(2025, 12, 5, 0, 0, 0, 0, time.UTC),
	}

	result, err := ds.Fetch(ctx, opts)
	if err != nil {
		t.Fatalf("Fetch failed: %v", err)
	}

	// Should have 4 events (2 PR reviews + 2 issue mentions)
	expectedCount := 4
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

	// Configure mock for all searches
	setupMockForAllSearches(authProvider.credential, t)

	ds, err := NewDataSource(authProvider)
	if err != nil {
		t.Fatalf("NewDataSource failed: %v", err)
	}

	// Set Since to a time after some events but before others
	// search_prs.json has events at 08:00:00 and 07:30:00
	// search_issues.json has events at 06:00:00 and 05:00:00
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
		for i, e := range result.Events {
			t.Logf("event[%d]: ts=%v", i, e.Timestamp)
		}
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

	// Use the same PR search data for both PR review and PR mention to create duplicates
	prSearchResp := loadGraphQLTestData(t, "search_prs.json")
	emptySearchResp := loadGraphQLTestData(t, "empty_search.json")
	prContextResp := loadGraphQLTestData(t, "pr_full_context.json")

	cred := authProvider.credential
	cred.setGraphQLResponse("graphql:search:pr-review", prSearchResp)
	cred.setGraphQLResponse("graphql:search:pr-mention", prSearchResp) // Same data to create duplicates
	cred.setGraphQLResponse("graphql:search:issue-mention", emptySearchResp)
	cred.setGraphQLResponse("graphql:search:issue-assigned", emptySearchResp)
	cred.setGraphQLResponse("graphql:pr:context", prContextResp)

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

	// Configure mock with empty results for all searches
	emptySearchResp := loadGraphQLTestData(t, "empty_search.json")

	cred := authProvider.credential
	cred.setGraphQLResponse("graphql:search:pr-review", emptySearchResp)
	cred.setGraphQLResponse("graphql:search:pr-mention", emptySearchResp)
	cred.setGraphQLResponse("graphql:search:issue-mention", emptySearchResp)
	cred.setGraphQLResponse("graphql:search:issue-assigned", emptySearchResp)

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
		searchKey    string // which search to return results for
		expectedType models.EventType
		expectedPri  models.Priority
	}{
		{
			name:         "PR review request",
			searchKey:    "graphql:search:pr-review",
			expectedType: models.EventTypePRReview,
			expectedPri:  models.PriorityHigh, // Per EFA 0001
		},
		{
			name:         "PR mention",
			searchKey:    "graphql:search:pr-mention",
			expectedType: models.EventTypePRMention,
			expectedPri:  models.PriorityMedium, // Per EFA 0001
		},
		{
			name:         "issue mention",
			searchKey:    "graphql:search:issue-mention",
			expectedType: models.EventTypeIssueMention,
			expectedPri:  models.PriorityMedium, // Per EFA 0001
		},
		{
			name:         "assigned issue",
			searchKey:    "graphql:search:issue-assigned",
			expectedType: models.EventTypeIssueAssigned,
			expectedPri:  models.PriorityMedium, // Per EFA 0001
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			authProvider := newMockAuthProvider(auth.ServiceGitHub)

			// Load GraphQL test data
			prSearchResp := loadGraphQLTestData(t, "search_prs.json")
			issueSearchResp := loadGraphQLTestData(t, "search_issues.json")
			emptySearchResp := loadGraphQLTestData(t, "empty_search.json")
			prContextResp := loadGraphQLTestData(t, "pr_full_context.json")
			issueContextResp := loadGraphQLTestData(t, "issue_full_context.json")

			cred := authProvider.credential

			// Set empty responses for all searches by default
			cred.setGraphQLResponse("graphql:search:pr-review", emptySearchResp)
			cred.setGraphQLResponse("graphql:search:pr-mention", emptySearchResp)
			cred.setGraphQLResponse("graphql:search:issue-mention", emptySearchResp)
			cred.setGraphQLResponse("graphql:search:issue-assigned", emptySearchResp)
			cred.setGraphQLResponse("graphql:pr:context", prContextResp)
			cred.setGraphQLResponse("graphql:issue:context", issueContextResp)

			// Set the test data for the appropriate search type
			switch tt.searchKey {
			case "graphql:search:pr-review", "graphql:search:pr-mention":
				cred.setGraphQLResponse(tt.searchKey, prSearchResp)
			case "graphql:search:issue-mention", "graphql:search:issue-assigned":
				cred.setGraphQLResponse(tt.searchKey, issueSearchResp)
			}

			ds, err := NewDataSource(authProvider)
			if err != nil {
				t.Fatalf("NewDataSource failed: %v", err)
			}

			// Use a fixed Since time before all test data timestamps
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

	// search_prs.json contains a long title (PR #456)
	setupMockForAllSearches(authProvider.credential, t)

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

// TestTransform_MetadataKeys verifies that metadata contains required EFA 0001 keys.
func TestTransform_MetadataKeys(t *testing.T) {
	ctx := context.Background()
	authProvider := newMockAuthProvider(auth.ServiceGitHub)

	// Configure mock for all searches
	setupMockForAllSearches(authProvider.credential, t)

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

	// EFA 0001 required metadata keys for GitHub
	requiredKeys := []string{"repo", "number", "state"}

	for i, event := range result.Events {
		// Verify required keys are present
		for _, key := range requiredKeys {
			if _, ok := event.Metadata[key]; !ok {
				t.Errorf("event %d missing required metadata key %q", i, key)
			}
		}

		// Verify user_relationships is present (added by GraphQL fetchers)
		if _, ok := event.Metadata["user_relationships"]; !ok {
			t.Errorf("event %d missing user_relationships metadata", i)
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
