package codeowners

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"
)

// makeTeamMembersResponse creates a JSON response for the GitHub team members API.
func makeTeamMembersResponse(logins ...string) []byte {
	members := make([]teamMemberResponse, len(logins))
	for i, login := range logins {
		members[i] = teamMemberResponse{Login: login}
	}
	b, _ := json.Marshal(members)
	return b
}

func TestNewTeamResolver(t *testing.T) {
	//nolint:govet // test struct field order prioritizes readability
	tests := []struct {
		name string
		cred githubCredential
		want bool // true if non-nil expected
	}{
		{
			name: "nil credential returns nil",
			cred: nil,
			want: false,
		},
		{
			name: "valid credential returns resolver",
			cred: newMockCredential(),
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewTeamResolver(tt.cred)
			if (got != nil) != tt.want {
				t.Errorf("NewTeamResolver() = %v, want non-nil=%v", got, tt.want)
			}
		})
	}
}

func TestTeamResolver_IsMember_True(t *testing.T) {
	cred := newMockCredential()
	cred.setResponse("orgs/acme/teams/backend/members",
		makeTeamMembersResponse("alice", "bob", "charlie"), nil)

	resolver := NewTeamResolver(cred)
	ctx := context.Background()

	// Check that alice is a member
	isMember, err := resolver.IsMember(ctx, "acme/backend", "alice")
	if err != nil {
		t.Fatalf("IsMember() error = %v", err)
	}
	if !isMember {
		t.Error("IsMember() = false, want true for team member")
	}
}

func TestTeamResolver_IsMember_False(t *testing.T) {
	cred := newMockCredential()
	cred.setResponse("orgs/acme/teams/backend/members",
		makeTeamMembersResponse("alice", "bob", "charlie"), nil)

	resolver := NewTeamResolver(cred)
	ctx := context.Background()

	// Check that dave is NOT a member
	isMember, err := resolver.IsMember(ctx, "acme/backend", "dave")
	if err != nil {
		t.Fatalf("IsMember() error = %v", err)
	}
	if isMember {
		t.Error("IsMember() = true, want false for non-member")
	}
}

func TestTeamResolver_IsMember_CaseInsensitive(t *testing.T) {
	cred := newMockCredential()
	cred.setResponse("orgs/acme/teams/backend/members",
		makeTeamMembersResponse("Alice", "Bob"), nil)

	resolver := NewTeamResolver(cred)
	ctx := context.Background()

	// GitHub logins are case-insensitive
	tests := []struct {
		login    string
		expected bool
	}{
		{"alice", true},
		{"Alice", true},
		{"ALICE", true},
		{"AlIcE", true},
		{"bob", true},
		{"BOB", true},
		{"charlie", false},
	}

	for _, tt := range tests {
		t.Run(tt.login, func(t *testing.T) {
			isMember, err := resolver.IsMember(ctx, "acme/backend", tt.login)
			if err != nil {
				t.Fatalf("IsMember() error = %v", err)
			}
			if isMember != tt.expected {
				t.Errorf("IsMember(%q) = %v, want %v", tt.login, isMember, tt.expected)
			}
		})
	}
}

func TestTeamResolver_IsMember_EmptyLogin(t *testing.T) {
	cred := newMockCredential()
	cred.setResponse("orgs/acme/teams/backend/members",
		makeTeamMembersResponse("alice"), nil)

	resolver := NewTeamResolver(cred)
	ctx := context.Background()

	isMember, err := resolver.IsMember(ctx, "acme/backend", "")
	if err != nil {
		t.Fatalf("IsMember() error = %v", err)
	}
	if isMember {
		t.Error("IsMember() = true for empty login, want false")
	}
}

func TestTeamResolver_GetMembers_CacheHit(t *testing.T) {
	cred := newMockCredential()
	cred.setResponse("orgs/acme/teams/backend/members",
		makeTeamMembersResponse("alice", "bob"), nil)

	resolver := NewTeamResolver(cred)
	ctx := context.Background()

	// First call - should hit API
	members1, err := resolver.GetMembers(ctx, "acme/backend")
	if err != nil {
		t.Fatalf("first GetMembers() error = %v", err)
	}
	if len(members1) != 2 {
		t.Fatalf("first GetMembers() returned %d members, want 2", len(members1))
	}
	if cred.getCallCount() != 1 {
		t.Errorf("first call: API calls = %d, want 1", cred.getCallCount())
	}

	// Second call - should use cache (no API call)
	members2, err := resolver.GetMembers(ctx, "acme/backend")
	if err != nil {
		t.Fatalf("second GetMembers() error = %v", err)
	}
	if len(members2) != 2 {
		t.Fatalf("second GetMembers() returned %d members, want 2", len(members2))
	}
	if cred.getCallCount() != 1 {
		t.Errorf("after second call: API calls = %d, want 1 (should use cache)", cred.getCallCount())
	}
}

func TestTeamResolver_GetMembers_CacheMiss(t *testing.T) {
	cred := newMockCredential()
	cred.setResponse("orgs/acme/teams/backend/members",
		makeTeamMembersResponse("alice"), nil)
	cred.setResponse("orgs/acme/teams/frontend/members",
		makeTeamMembersResponse("bob"), nil)

	resolver := NewTeamResolver(cred)
	ctx := context.Background()

	// First team
	members1, err := resolver.GetMembers(ctx, "acme/backend")
	if err != nil {
		t.Fatalf("backend GetMembers() error = %v", err)
	}
	if len(members1) != 1 || members1[0] != "alice" {
		t.Fatalf("backend GetMembers() = %v, want [alice]", members1)
	}

	// Different team - should hit API again
	members2, err := resolver.GetMembers(ctx, "acme/frontend")
	if err != nil {
		t.Fatalf("frontend GetMembers() error = %v", err)
	}
	if len(members2) != 1 || members2[0] != "bob" {
		t.Fatalf("frontend GetMembers() = %v, want [bob]", members2)
	}

	if cred.getCallCount() != 2 {
		t.Errorf("API calls = %d, want 2", cred.getCallCount())
	}
}

func TestTeamResolver_GetMembers_EmptyTeam(t *testing.T) {
	cred := newMockCredential()
	// Return empty array for team with no members
	cred.setResponse("orgs/acme/teams/empty-team/members",
		makeTeamMembersResponse(), nil)

	resolver := NewTeamResolver(cred)
	ctx := context.Background()

	members, err := resolver.GetMembers(ctx, "acme/empty-team")
	if err != nil {
		t.Fatalf("GetMembers() error = %v", err)
	}
	if members == nil {
		t.Error("GetMembers() returned nil, want empty slice")
	}
	if len(members) != 0 {
		t.Errorf("GetMembers() returned %d members, want 0", len(members))
	}
}

func TestTeamResolver_GetMembers_InvalidTeamFormat(t *testing.T) {
	cred := newMockCredential()
	resolver := NewTeamResolver(cred)
	ctx := context.Background()

	tests := []struct {
		name string
		team string
	}{
		{"empty string", ""},
		{"no slash", "acme-backend"},
		{"only slash", "/"},
		{"empty org", "/backend"},
		{"empty team", "acme/"},
		{"whitespace org", "  /backend"},
		{"whitespace team", "acme/  "},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := resolver.GetMembers(ctx, tt.team)
			if err == nil {
				t.Errorf("GetMembers(%q) error = nil, want error for invalid format", tt.team)
			}
			if !containsString(err.Error(), "invalid team format") {
				t.Errorf("error = %v, want to contain 'invalid team format'", err)
			}
		})
	}
}

func TestTeamResolver_GetMembers_APIError(t *testing.T) {
	cred := newMockCredential()
	cred.setResponse("orgs/acme/teams/backend/members",
		nil, errors.New("404 Not Found"))

	resolver := NewTeamResolver(cred)
	ctx := context.Background()

	_, err := resolver.GetMembers(ctx, "acme/backend")
	if err == nil {
		t.Fatal("GetMembers() error = nil, want error")
	}
	if !containsString(err.Error(), "fetching team") {
		t.Errorf("error = %v, want to contain 'fetching team'", err)
	}
}

func TestTeamResolver_GetMembers_InvalidJSON(t *testing.T) {
	cred := newMockCredential()
	cred.setResponse("orgs/acme/teams/backend/members",
		[]byte("not valid json"), nil)

	resolver := NewTeamResolver(cred)
	ctx := context.Background()

	_, err := resolver.GetMembers(ctx, "acme/backend")
	if err == nil {
		t.Fatal("GetMembers() error = nil, want error for invalid JSON")
	}
	if !containsString(err.Error(), "parsing team members") {
		t.Errorf("error = %v, want to contain 'parsing team members'", err)
	}
}

func TestTeamResolver_InvalidateCache(t *testing.T) {
	cred := newMockCredential()
	cred.setResponse("orgs/acme/teams/team1/members",
		makeTeamMembersResponse("alice"), nil)
	cred.setResponse("orgs/acme/teams/team2/members",
		makeTeamMembersResponse("bob"), nil)

	resolver := NewTeamResolver(cred)
	ctx := context.Background()

	// Populate cache
	_, _ = resolver.GetMembers(ctx, "acme/team1")
	_, _ = resolver.GetMembers(ctx, "acme/team2")

	if resolver.CacheSize() != 2 {
		t.Fatalf("cache size = %d, want 2", resolver.CacheSize())
	}

	// Invalidate specific team
	resolver.InvalidateCache("acme/team1")
	if resolver.CacheSize() != 1 {
		t.Errorf("after invalidate team1: cache size = %d, want 1", resolver.CacheSize())
	}

	// Invalidate all
	resolver.InvalidateCache("")
	if resolver.CacheSize() != 0 {
		t.Errorf("after invalidate all: cache size = %d, want 0", resolver.CacheSize())
	}
}

func TestTeamResolver_InvalidateCache_RefetchAfterInvalidation(t *testing.T) {
	cred := newMockCredential()
	cred.setResponse("orgs/acme/teams/backend/members",
		makeTeamMembersResponse("alice"), nil)

	resolver := NewTeamResolver(cred)
	ctx := context.Background()

	// First fetch
	members1, _ := resolver.GetMembers(ctx, "acme/backend")
	initialCallCount := cred.getCallCount()

	if len(members1) != 1 || members1[0] != "alice" {
		t.Fatalf("first fetch: members = %v, want [alice]", members1)
	}

	// Update the response (simulating membership change)
	cred.setResponse("orgs/acme/teams/backend/members",
		makeTeamMembersResponse("alice", "bob"), nil)

	// Second fetch - should use cache
	members2, _ := resolver.GetMembers(ctx, "acme/backend")
	if cred.getCallCount() != initialCallCount {
		t.Error("cache was not used after membership change")
	}
	if len(members2) != 1 {
		t.Error("cache returned updated members unexpectedly")
	}

	// Invalidate and refetch
	resolver.InvalidateCache("acme/backend")
	members3, _ := resolver.GetMembers(ctx, "acme/backend")

	if cred.getCallCount() <= initialCallCount {
		t.Error("API was not called after cache invalidation")
	}
	if len(members3) != 2 {
		t.Errorf("after invalidation: members count = %d, want 2", len(members3))
	}
}

func TestTeamResolver_ConcurrentAccess(t *testing.T) {
	cred := newMockCredential()
	cred.setResponse("orgs/acme/teams/backend/members",
		makeTeamMembersResponse("alice", "bob", "charlie"), nil)

	resolver := NewTeamResolver(cred)
	ctx := context.Background()

	const numGoroutines = 50
	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	errChan := make(chan error, numGoroutines)
	resultChan := make(chan bool, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			isMember, err := resolver.IsMember(ctx, "acme/backend", "alice")
			if err != nil {
				errChan <- err
				return
			}
			resultChan <- isMember
		}()
	}

	wg.Wait()
	close(errChan)
	close(resultChan)

	// Check for errors
	for err := range errChan {
		t.Errorf("concurrent IsMember() error = %v", err)
	}

	// All results should be true
	for result := range resultChan {
		if !result {
			t.Error("concurrent IsMember() returned false, want true")
		}
	}

	// Should have made very few API calls due to caching
	if cred.getCallCount() > 5 {
		t.Errorf("API calls = %d, want <= 5 (most should use cache)", cred.getCallCount())
	}
}

func TestTeamResolver_ConcurrentDifferentTeams(t *testing.T) {
	cred := newMockCredential()
	teams := []string{"acme/team1", "acme/team2", "acme/team3"}
	for _, team := range teams {
		parts := splitTeam(team)
		cred.setResponse("orgs/"+parts[0]+"/teams/"+parts[1]+"/members",
			makeTeamMembersResponse("member-"+parts[1]), nil)
	}

	resolver := NewTeamResolver(cred)
	ctx := context.Background()

	var wg sync.WaitGroup
	wg.Add(len(teams) * 10)

	for i := 0; i < 10; i++ {
		for _, team := range teams {
			team := team
			go func() {
				defer wg.Done()
				members, err := resolver.GetMembers(ctx, team)
				if err != nil {
					t.Errorf("GetMembers(%s) error = %v", team, err)
					return
				}
				if len(members) != 1 {
					t.Errorf("GetMembers(%s) returned %d members, want 1", team, len(members))
				}
			}()
		}
	}

	wg.Wait()

	// Should have 3 cached entries
	if resolver.CacheSize() != 3 {
		t.Errorf("cache size = %d, want 3", resolver.CacheSize())
	}
}

func TestTeamResolver_RaceCondition(t *testing.T) {
	cred := newMockCredential()
	cred.setResponse("orgs/acme/teams/backend/members",
		makeTeamMembersResponse("alice", "bob"), nil)

	resolver := NewTeamResolver(cred)
	ctx := context.Background()

	var wg sync.WaitGroup
	wg.Add(100)

	// Mix of operations to trigger race conditions
	for i := 0; i < 100; i++ {
		go func(n int) {
			defer wg.Done()
			switch n % 5 {
			case 0:
				_, _ = resolver.GetMembers(ctx, "acme/backend")
			case 1:
				_, _ = resolver.IsMember(ctx, "acme/backend", "alice")
			case 2:
				resolver.InvalidateCache("acme/backend")
			case 3:
				resolver.InvalidateCache("")
			case 4:
				_ = resolver.CacheSize()
			}
		}(i)
	}

	wg.Wait()
}

func TestTeamResolver_NilReceiver(t *testing.T) {
	var resolver *TeamResolver

	ctx := context.Background()

	// All methods should handle nil receiver gracefully
	members, err := resolver.GetMembers(ctx, "acme/backend")
	if err != nil {
		t.Errorf("nil.GetMembers() error = %v, want nil", err)
	}
	if members != nil {
		t.Errorf("nil.GetMembers() = %v, want nil", members)
	}

	isMember, err := resolver.IsMember(ctx, "acme/backend", "alice")
	if err != nil {
		t.Errorf("nil.IsMember() error = %v, want nil", err)
	}
	if isMember {
		t.Error("nil.IsMember() = true, want false")
	}

	// InvalidateCache should not panic
	resolver.InvalidateCache("")
	resolver.InvalidateCache("acme/backend")

	// CacheSize should return 0
	if resolver.CacheSize() != 0 {
		t.Errorf("nil.CacheSize() = %d, want 0", resolver.CacheSize())
	}
}

func TestTeamResolver_ContextCancellation(t *testing.T) {
	cred := newMockCredential()
	cred.setResponse("orgs/acme/teams/backend/members",
		makeTeamMembersResponse("alice"), nil)

	resolver := NewTeamResolver(cred)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := resolver.GetMembers(ctx, "acme/backend")
	if err == nil {
		t.Fatal("GetMembers() error = nil, want context cancellation error")
	}
}

func TestTeamResolver_ContextTimeout(t *testing.T) {
	// Create a mock that delays response
	cred := &slowMockCredential{
		delay: 100 * time.Millisecond,
	}

	resolver := NewTeamResolver(cred)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	_, err := resolver.GetMembers(ctx, "acme/backend")
	if err == nil {
		t.Error("GetMembers() error = nil, want timeout error")
	}
}

func TestParseTeamRef(t *testing.T) {
	tests := []struct {
		name      string
		team      string
		wantOrg   string
		wantSlug  string
		wantError bool
	}{
		{
			name:      "valid team",
			team:      "acme/backend",
			wantOrg:   "acme",
			wantSlug:  "backend",
			wantError: false,
		},
		{
			name:      "team with hyphen",
			team:      "my-org/my-team-name",
			wantOrg:   "my-org",
			wantSlug:  "my-team-name",
			wantError: false,
		},
		{
			name:      "org with numbers",
			team:      "acme123/team456",
			wantOrg:   "acme123",
			wantSlug:  "team456",
			wantError: false,
		},
		{
			name:      "empty string",
			team:      "",
			wantError: true,
		},
		{
			name:      "no slash",
			team:      "acme-backend",
			wantError: true,
		},
		{
			name:      "only slash",
			team:      "/",
			wantError: true,
		},
		{
			name:      "empty org",
			team:      "/backend",
			wantError: true,
		},
		{
			name:      "empty team slug",
			team:      "acme/",
			wantError: true,
		},
		{
			name:      "multiple slashes - only first split",
			team:      "acme/team/subteam",
			wantOrg:   "acme",
			wantSlug:  "team/subteam",
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			org, slug, err := parseTeamRef(tt.team)
			if (err != nil) != tt.wantError {
				t.Errorf("parseTeamRef(%q) error = %v, wantError %v", tt.team, err, tt.wantError)
				return
			}
			if !tt.wantError {
				if org != tt.wantOrg {
					t.Errorf("parseTeamRef(%q) org = %q, want %q", tt.team, org, tt.wantOrg)
				}
				if slug != tt.wantSlug {
					t.Errorf("parseTeamRef(%q) slug = %q, want %q", tt.team, slug, tt.wantSlug)
				}
			}
		})
	}
}

// Helper to split team for test setup
func splitTeam(team string) []string {
	parts := make([]string, 2)
	for i, p := range splitOnce(team, '/') {
		if i < 2 {
			parts[i] = p
		}
	}
	return parts
}

func splitOnce(s string, sep rune) []string {
	for i, c := range s {
		if c == sep {
			return []string{s[:i], s[i+1:]}
		}
	}
	return []string{s}
}

// Benchmarks

func BenchmarkTeamResolver_GetMembers_CacheHit(b *testing.B) {
	cred := newMockCredential()
	cred.setResponse("orgs/acme/teams/backend/members",
		makeTeamMembersResponse("alice", "bob", "charlie", "dave", "eve"), nil)

	resolver := NewTeamResolver(cred)
	ctx := context.Background()

	// Populate cache
	_, _ = resolver.GetMembers(ctx, "acme/backend")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = resolver.GetMembers(ctx, "acme/backend")
	}
}

func BenchmarkTeamResolver_IsMember_CacheHit(b *testing.B) {
	cred := newMockCredential()
	cred.setResponse("orgs/acme/teams/backend/members",
		makeTeamMembersResponse("alice", "bob", "charlie", "dave", "eve"), nil)

	resolver := NewTeamResolver(cred)
	ctx := context.Background()

	// Populate cache
	_, _ = resolver.GetMembers(ctx, "acme/backend")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = resolver.IsMember(ctx, "acme/backend", "charlie")
	}
}

func BenchmarkTeamResolver_ConcurrentAccess(b *testing.B) {
	cred := newMockCredential()
	cred.setResponse("orgs/acme/teams/backend/members",
		makeTeamMembersResponse("alice", "bob", "charlie", "dave", "eve"), nil)

	resolver := NewTeamResolver(cred)
	ctx := context.Background()

	// Populate cache
	_, _ = resolver.GetMembers(ctx, "acme/backend")

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, _ = resolver.IsMember(ctx, "acme/backend", "alice")
		}
	})
}

// trackingMockCredential extends mockCredential to track call patterns.
type trackingMockCredential struct {
	responses map[string]mockResponse
	calls     []string
	mu        sync.RWMutex
}

func newTrackingMockCredential() *trackingMockCredential {
	return &trackingMockCredential{
		responses: make(map[string]mockResponse),
		calls:     make([]string, 0),
	}
}

func (m *trackingMockCredential) setResponse(endpoint string, data []byte, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.responses[endpoint] = mockResponse{data: data, err: err}
}

func (m *trackingMockCredential) ExecuteAPI(ctx context.Context, endpoint string, args ...string) ([]byte, error) {
	m.mu.Lock()
	m.calls = append(m.calls, endpoint)
	m.mu.Unlock()

	// Check context cancellation
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	m.mu.RLock()
	defer m.mu.RUnlock()
	if resp, ok := m.responses[endpoint]; ok {
		return resp.data, resp.err
	}
	return nil, errors.New("404 Not Found")
}

func (m *trackingMockCredential) getCalls() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]string, len(m.calls))
	copy(result, m.calls)
	return result
}

func TestTeamResolver_APIEndpoint(t *testing.T) {
	cred := newTrackingMockCredential()
	cred.setResponse("orgs/my-org/teams/my-team/members",
		makeTeamMembersResponse("alice"), nil)

	resolver := NewTeamResolver(cred)
	ctx := context.Background()

	_, err := resolver.GetMembers(ctx, "my-org/my-team")
	if err != nil {
		t.Fatalf("GetMembers() error = %v", err)
	}

	calls := cred.getCalls()
	if len(calls) != 1 {
		t.Fatalf("expected 1 API call, got %d", len(calls))
	}

	expectedEndpoint := "orgs/my-org/teams/my-team/members"
	if calls[0] != expectedEndpoint {
		t.Errorf("API endpoint = %q, want %q", calls[0], expectedEndpoint)
	}
}
