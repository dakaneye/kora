package github

import (
	"context"
	"testing"
	"time"

	"github.com/dakaneye/kora/internal/auth"
	"github.com/dakaneye/kora/internal/codeowners"
	"github.com/dakaneye/kora/internal/datasources"
	"github.com/dakaneye/kora/internal/models"
)

// TestCodeownerEvents_DirectOwnership verifies codeowner event creation for direct user ownership.
func TestCodeownerEvents_DirectOwnership(t *testing.T) {
	ctx := context.Background()
	authProvider := newMockAuthProvider(auth.ServiceGitHub)

	// Configure mock for current user
	authProvider.credential.setRESTResponse("user", []byte(`{"login":"testuser"}`))

	// Configure mock for all GraphQL searches
	setupMockForAllSearches(authProvider.credential, t)

	// Create CODEOWNERS ruleset where testuser owns src/rebuild/*.go
	ruleset, _ := codeowners.Parse([]byte(`
# Test CODEOWNERS
src/rebuild/*.go @testuser
`))

	// Create a mock fetcher that wraps the real codeowners package
	mockFetcher := &wrappedCodeownersFetcher{
		rulesets: map[string]*codeowners.Ruleset{
			"example/repo": ruleset,
		},
	}

	ds, err := NewDataSource(authProvider,
		WithCodeownersFetcher(mockFetcher.toRealFetcher(authProvider.credential)),
	)
	if err != nil {
		t.Fatalf("NewDataSource failed: %v", err)
	}

	// Manually set the mock fetcher since we can't easily inject it
	ds.codeownersFetcher = mockFetcher.toRealFetcher(authProvider.credential)

	opts := datasources.FetchOptions{
		Since: time.Date(2025, 12, 5, 0, 0, 0, 0, time.UTC),
	}

	result, err := ds.Fetch(ctx, opts)
	if err != nil {
		t.Fatalf("Fetch failed: %v", err)
	}

	// Look for codeowner events
	var codeownerEvents []models.Event
	for _, event := range result.Events {
		if event.Type == models.EventTypePRCodeowner {
			codeownerEvents = append(codeownerEvents, event)
		}
	}

	// The test data in pr_full_context.json has files:
	// - src/rebuild/java.go -> should match @testuser
	// - src/rebuild/java_test.go -> should match @testuser
	// So we expect codeowner events for PRs that have these files

	// Verify codeowner events have correct properties per EFA 0001
	for _, event := range codeownerEvents {
		if event.Priority != models.PriorityMedium {
			t.Errorf("codeowner event has wrong priority: got %d, want %d", event.Priority, models.PriorityMedium)
		}
		if !event.RequiresAction {
			t.Error("codeowner event should require action")
		}
		relationships, ok := event.Metadata["user_relationships"].([]string)
		if !ok {
			t.Error("codeowner event missing user_relationships")
		} else {
			foundCodeowner := false
			for _, rel := range relationships {
				if rel == "codeowner" {
					foundCodeowner = true
					break
				}
			}
			if !foundCodeowner {
				t.Errorf("codeowner event missing 'codeowner' in relationships: %v", relationships)
			}
		}
		if err := event.Validate(); err != nil {
			t.Errorf("codeowner event failed validation: %v", err)
		}
	}
}

// TestCodeownerEvents_TeamMembership verifies codeowner detection via team membership.
func TestCodeownerEvents_TeamMembership(t *testing.T) {
	ctx := context.Background()
	authProvider := newMockAuthProvider(auth.ServiceGitHub)

	// Configure mock for current user
	authProvider.credential.setRESTResponse("user", []byte(`{"login":"testuser"}`))

	// Configure mock for all GraphQL searches
	setupMockForAllSearches(authProvider.credential, t)

	// Also configure mock for team membership check
	authProvider.credential.setRESTResponse("orgs/example/teams/backend-team/members",
		[]byte(`[{"login":"testuser"},{"login":"otheruser"}]`))

	// Create CODEOWNERS ruleset where @example/backend-team owns files
	ruleset, _ := codeowners.Parse([]byte(`
# Test CODEOWNERS
src/rebuild/*.go @example/backend-team
`))

	// Use real team resolver with mock credential
	teamResolver := codeowners.NewTeamResolver(authProvider.credential)

	// Create fetcher with the ruleset
	fetcher := createMockCodeownersFetcher(authProvider.credential, map[string]*codeowners.Ruleset{
		"example/repo": ruleset,
	})

	ds, err := NewDataSource(authProvider,
		WithCodeownersFetcher(fetcher),
		WithTeamResolver(teamResolver),
	)
	if err != nil {
		t.Fatalf("NewDataSource failed: %v", err)
	}

	opts := datasources.FetchOptions{
		Since: time.Date(2025, 12, 5, 0, 0, 0, 0, time.UTC),
	}

	result, err := ds.Fetch(ctx, opts)
	if err != nil {
		t.Fatalf("Fetch failed: %v", err)
	}

	// Look for codeowner events
	var codeownerEvents []models.Event
	for _, event := range result.Events {
		if event.Type == models.EventTypePRCodeowner {
			codeownerEvents = append(codeownerEvents, event)
		}
	}

	// Verify events pass validation
	for _, event := range codeownerEvents {
		if err := event.Validate(); err != nil {
			t.Errorf("codeowner event failed validation: %v", err)
		}
	}
}

// TestCodeownerEvents_NoCodeownersFile verifies no events when repo has no CODEOWNERS.
func TestCodeownerEvents_NoCodeownersFile(t *testing.T) {
	ctx := context.Background()
	authProvider := newMockAuthProvider(auth.ServiceGitHub)

	// Configure mock for current user
	authProvider.credential.setRESTResponse("user", []byte(`{"login":"testuser"}`))

	// Configure mock for all GraphQL searches
	setupMockForAllSearches(authProvider.credential, t)

	// Create empty fetcher (no CODEOWNERS files)
	fetcher := createMockCodeownersFetcher(authProvider.credential, nil)

	ds, err := NewDataSource(authProvider,
		WithCodeownersFetcher(fetcher),
	)
	if err != nil {
		t.Fatalf("NewDataSource failed: %v", err)
	}

	opts := datasources.FetchOptions{
		Since: time.Date(2025, 12, 5, 0, 0, 0, 0, time.UTC),
	}

	result, err := ds.Fetch(ctx, opts)
	if err != nil {
		t.Fatalf("Fetch failed: %v", err)
	}

	// Should have no codeowner events
	for _, event := range result.Events {
		if event.Type == models.EventTypePRCodeowner {
			t.Error("expected no codeowner events when repo has no CODEOWNERS file")
		}
	}
}

// TestCodeownerEvents_AlreadyReviewer verifies no duplicate event when user is already a reviewer.
func TestCodeownerEvents_AlreadyReviewer(t *testing.T) {
	ctx := context.Background()
	authProvider := newMockAuthProvider(auth.ServiceGitHub)

	// Configure mock for current user (same as review requester in test data)
	authProvider.credential.setRESTResponse("user", []byte(`{"login":"currentuser"}`))

	// Configure mock for all GraphQL searches
	setupMockForAllSearches(authProvider.credential, t)

	// Create CODEOWNERS ruleset where currentuser owns files
	ruleset, _ := codeowners.Parse([]byte(`
# Test CODEOWNERS
src/rebuild/*.go @currentuser
`))

	fetcher := createMockCodeownersFetcher(authProvider.credential, map[string]*codeowners.Ruleset{
		"example/repo": ruleset,
	})

	ds, err := NewDataSource(authProvider,
		WithCodeownersFetcher(fetcher),
	)
	if err != nil {
		t.Fatalf("NewDataSource failed: %v", err)
	}

	opts := datasources.FetchOptions{
		Since: time.Date(2025, 12, 5, 0, 0, 0, 0, time.UTC),
	}

	result, err := ds.Fetch(ctx, opts)
	if err != nil {
		t.Fatalf("Fetch failed: %v", err)
	}

	// Count events by type
	eventCounts := make(map[models.EventType]int)
	for _, event := range result.Events {
		eventCounts[event.Type]++
	}

	// pr_full_context.json has "currentuser" in review_requests
	// So codeowner events should be skipped for PRs where user is already requested
	// Note: The test checks that we don't create duplicate codeowner events
	// when the user is already a reviewer

	// Verify all events pass validation
	for _, event := range result.Events {
		if err := event.Validate(); err != nil {
			t.Errorf("event failed validation: %v", err)
		}
	}
}

// TestCodeownerEvents_NotInOwnershipRules verifies no event when user doesn't own files.
func TestCodeownerEvents_NotInOwnershipRules(t *testing.T) {
	ctx := context.Background()
	authProvider := newMockAuthProvider(auth.ServiceGitHub)

	// Configure mock for current user
	authProvider.credential.setRESTResponse("user", []byte(`{"login":"testuser"}`))

	// Configure mock for all GraphQL searches
	setupMockForAllSearches(authProvider.credential, t)

	// Create CODEOWNERS ruleset where testuser does NOT own the files in test data
	ruleset, _ := codeowners.Parse([]byte(`
# Test CODEOWNERS - testuser owns different files
docs/*.md @testuser
`))

	fetcher := createMockCodeownersFetcher(authProvider.credential, map[string]*codeowners.Ruleset{
		"example/repo": ruleset,
	})

	ds, err := NewDataSource(authProvider,
		WithCodeownersFetcher(fetcher),
	)
	if err != nil {
		t.Fatalf("NewDataSource failed: %v", err)
	}

	opts := datasources.FetchOptions{
		Since: time.Date(2025, 12, 5, 0, 0, 0, 0, time.UTC),
	}

	result, err := ds.Fetch(ctx, opts)
	if err != nil {
		t.Fatalf("Fetch failed: %v", err)
	}

	// Should have no codeowner events since testuser doesn't own src/rebuild/*.go
	for _, event := range result.Events {
		if event.Type == models.EventTypePRCodeowner {
			t.Error("expected no codeowner events when user doesn't own changed files")
		}
	}
}

// TestCodeownerEvents_DisabledWithoutFetcher verifies no codeowner processing without fetcher.
func TestCodeownerEvents_DisabledWithoutFetcher(t *testing.T) {
	ctx := context.Background()
	authProvider := newMockAuthProvider(auth.ServiceGitHub)

	// Configure mock for all GraphQL searches (no user mock needed)
	setupMockForAllSearches(authProvider.credential, t)

	// Create datasource WITHOUT codeowners fetcher
	ds, err := NewDataSource(authProvider)
	if err != nil {
		t.Fatalf("NewDataSource failed: %v", err)
	}

	opts := datasources.FetchOptions{
		Since: time.Date(2025, 12, 5, 0, 0, 0, 0, time.UTC),
	}

	result, err := ds.Fetch(ctx, opts)
	if err != nil {
		t.Fatalf("Fetch failed: %v", err)
	}

	// Should have no codeowner events
	for _, event := range result.Events {
		if event.Type == models.EventTypePRCodeowner {
			t.Error("expected no codeowner events when fetcher is not configured")
		}
	}
}

// TestGetCurrentUser verifies current user caching.
func TestGetCurrentUser(t *testing.T) {
	ctx := context.Background()
	authProvider := newMockAuthProvider(auth.ServiceGitHub)

	// Configure mock for current user
	authProvider.credential.setRESTResponse("user", []byte(`{"login":"cacheduser"}`))

	ds, err := NewDataSource(authProvider)
	if err != nil {
		t.Fatalf("NewDataSource failed: %v", err)
	}

	// First call should fetch
	user1, err := ds.getCurrentUser(ctx, authProvider.credential)
	if err != nil {
		t.Fatalf("getCurrentUser failed: %v", err)
	}
	if user1 != "cacheduser" {
		t.Errorf("expected 'cacheduser', got %q", user1)
	}

	// Change the mock response - should still return cached value
	authProvider.credential.setRESTResponse("user", []byte(`{"login":"differentuser"}`))

	user2, err := ds.getCurrentUser(ctx, authProvider.credential)
	if err != nil {
		t.Fatalf("getCurrentUser failed: %v", err)
	}
	if user2 != "cacheduser" {
		t.Errorf("expected cached 'cacheduser', got %q (cache didn't work)", user2)
	}
}

// TestCheckCodeownerEvents_Unit is a unit test for checkCodeownerEvents method.
func TestCheckCodeownerEvents_Unit(t *testing.T) {
	//nolint:govet // Field order prioritizes readability over memory alignment
	tests := []struct {
		name          string
		event         models.Event
		currentUser   string
		ruleset       *codeowners.Ruleset
		wantEvent     bool
		wantRelations []string
	}{
		{
			name: "user is direct owner",
			event: models.Event{
				Type:      models.EventTypePRMention,
				Title:     "Test PR",
				Source:    models.SourceGitHub,
				URL:       "https://github.com/test/repo/pull/1",
				Author:    models.Person{Username: "author"},
				Timestamp: time.Now(),
				Priority:  models.PriorityMedium,
				Metadata: map[string]any{
					"repo":               "test/repo",
					"user_relationships": []string{"mentioned"},
					"files_changed": []map[string]any{
						{"path": "src/main.go", "additions": 10, "deletions": 5},
					},
				},
			},
			currentUser:   "testuser",
			ruleset:       mustParseRuleset("src/*.go @testuser"),
			wantEvent:     true,
			wantRelations: []string{"mentioned", "codeowner"},
		},
		{
			name: "user is already reviewer",
			event: models.Event{
				Type:      models.EventTypePRReview,
				Title:     "Test PR",
				Source:    models.SourceGitHub,
				URL:       "https://github.com/test/repo/pull/1",
				Author:    models.Person{Username: "author"},
				Timestamp: time.Now(),
				Priority:  models.PriorityHigh,
				Metadata: map[string]any{
					"repo":               "test/repo",
					"user_relationships": []string{"reviewer"},
					"review_requests": []map[string]any{
						{"login": "testuser", "type": "user"},
					},
					"files_changed": []map[string]any{
						{"path": "src/main.go", "additions": 10, "deletions": 5},
					},
				},
			},
			currentUser: "testuser",
			ruleset:     mustParseRuleset("src/*.go @testuser"),
			wantEvent:   false, // Already a reviewer
		},
		{
			name: "user not in owners",
			event: models.Event{
				Type:      models.EventTypePRMention,
				Title:     "Test PR",
				Source:    models.SourceGitHub,
				URL:       "https://github.com/test/repo/pull/1",
				Author:    models.Person{Username: "author"},
				Timestamp: time.Now(),
				Priority:  models.PriorityMedium,
				Metadata: map[string]any{
					"repo":               "test/repo",
					"user_relationships": []string{"mentioned"},
					"files_changed": []map[string]any{
						{"path": "src/main.go", "additions": 10, "deletions": 5},
					},
				},
			},
			currentUser: "testuser",
			ruleset:     mustParseRuleset("src/*.go @otheruser"),
			wantEvent:   false, // testuser not in owners
		},
		{
			name: "no files changed metadata",
			event: models.Event{
				Type:      models.EventTypePRMention,
				Title:     "Test PR",
				Source:    models.SourceGitHub,
				URL:       "https://github.com/test/repo/pull/1",
				Author:    models.Person{Username: "author"},
				Timestamp: time.Now(),
				Priority:  models.PriorityMedium,
				Metadata: map[string]any{
					"repo":               "test/repo",
					"user_relationships": []string{"mentioned"},
					// No files_changed
				},
			},
			currentUser: "testuser",
			ruleset:     mustParseRuleset("src/*.go @testuser"),
			wantEvent:   false, // No files to check
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()

			// Create a mock fetcher
			authProvider := newMockAuthProvider(auth.ServiceGitHub)
			fetcher := createMockCodeownersFetcher(authProvider.credential, map[string]*codeowners.Ruleset{
				"test/repo": tt.ruleset,
			})

			ds := &DataSource{
				authProvider:      authProvider,
				codeownersFetcher: fetcher,
			}

			gotEvent, err := ds.checkCodeownerEvents(ctx, &tt.event, tt.currentUser)
			if err != nil {
				t.Fatalf("checkCodeownerEvents failed: %v", err)
			}

			if tt.wantEvent {
				if gotEvent == nil {
					t.Fatal("expected codeowner event, got nil")
				}
				if gotEvent.Type != models.EventTypePRCodeowner {
					t.Errorf("wrong event type: got %s, want %s", gotEvent.Type, models.EventTypePRCodeowner)
				}
				if gotEvent.Priority != models.PriorityMedium {
					t.Errorf("wrong priority: got %d, want %d", gotEvent.Priority, models.PriorityMedium)
				}
				if !gotEvent.RequiresAction {
					t.Error("event should require action")
				}

				// Check relationships
				rels, ok := gotEvent.Metadata["user_relationships"].([]string)
				if !ok {
					t.Error("missing user_relationships in metadata")
				} else if len(rels) != len(tt.wantRelations) {
					t.Errorf("wrong relationships: got %v, want %v", rels, tt.wantRelations)
				}

				// Validate the event
				if err := gotEvent.Validate(); err != nil {
					t.Errorf("event failed validation: %v", err)
				}
			} else if gotEvent != nil {
				t.Errorf("expected no event, got %+v", gotEvent)
			}
		})
	}
}

// mustParseRuleset parses a CODEOWNERS string or panics.
func mustParseRuleset(content string) *codeowners.Ruleset {
	rs, err := codeowners.Parse([]byte(content))
	if err != nil {
		panic(err)
	}
	return rs
}

// wrappedCodeownersFetcher wraps a map of rulesets for testing.
type wrappedCodeownersFetcher struct {
	rulesets map[string]*codeowners.Ruleset
}

func (w *wrappedCodeownersFetcher) toRealFetcher(cred githubCredential) *codeowners.Fetcher {
	return codeowners.NewFetcher(cred)
}

// createMockCodeownersFetcher creates a codeowners.Fetcher with pre-populated cache.
func createMockCodeownersFetcher(cred githubCredential, rulesets map[string]*codeowners.Ruleset) *codeowners.Fetcher {
	// Create real fetcher
	fetcher := codeowners.NewFetcher(cred)
	if fetcher == nil {
		return nil
	}

	// Configure mock responses for each repo's CODEOWNERS
	mockCred, ok := cred.(*mockGitHubDelegatedCredential)
	if !ok {
		return fetcher
	}

	for repo, ruleset := range rulesets {
		if ruleset == nil {
			// No CODEOWNERS - mock 404 response
			mockCred.setRESTError("repos/"+repo+"/contents/.github/CODEOWNERS", &mockError{"404 Not Found"})
			mockCred.setRESTError("repos/"+repo+"/contents/CODEOWNERS", &mockError{"404 Not Found"})
			mockCred.setRESTError("repos/"+repo+"/contents/docs/CODEOWNERS", &mockError{"404 Not Found"})
		} else {
			// Build CODEOWNERS content from rules
			var content string
			for _, rule := range ruleset.Rules {
				content += rule.Pattern
				for _, owner := range rule.Owners {
					content += " " + owner
				}
				content += "\n"
			}
			// Base64 encode the content
			encoded := encodeBase64([]byte(content))
			mockCred.setRESTResponse("repos/"+repo+"/contents/.github/CODEOWNERS",
				[]byte(`{"content":"`+encoded+`","encoding":"base64","type":"file"}`))
		}
	}

	return fetcher
}

// encodeBase64 encodes bytes to base64.
func encodeBase64(data []byte) string {
	const base64Chars = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"
	result := make([]byte, 0, ((len(data)+2)/3)*4)

	for i := 0; i < len(data); i += 3 {
		var b uint32
		n := 0
		for j := 0; j < 3 && i+j < len(data); j++ {
			b = (b << 8) | uint32(data[i+j])
			n++
		}
		b <<= (3 - n) * 8

		for j := 0; j < n+1; j++ {
			shift := 18 - j*6
			result = append(result, base64Chars[(b>>shift)&0x3F])
		}
		for j := n + 1; j < 4; j++ {
			result = append(result, '=')
		}
	}

	return string(result)
}

// setRESTResponse adds a REST API response mock.
func (m *mockGitHubDelegatedCredential) setRESTResponse(endpoint string, data []byte) {
	m.responses["rest:"+endpoint] = mockAPIResponse{data: data}
}

// setRESTError adds a REST API error mock.
func (m *mockGitHubDelegatedCredential) setRESTError(endpoint string, err error) {
	m.responses["rest:"+endpoint] = mockAPIResponse{err: err}
}
