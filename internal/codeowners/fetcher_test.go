package codeowners

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// mockCredential implements githubCredential for testing.
type mockCredential struct {
	// responses maps endpoint to response data
	responses map[string]mockResponse
	// callCount tracks API calls for verification
	callCount atomic.Int32
	// mu protects responses for concurrent access
	mu sync.RWMutex
}

//nolint:govet // field alignment not critical for test structs
type mockResponse struct {
	data []byte
	err  error
}

func newMockCredential() *mockCredential {
	return &mockCredential{
		responses: make(map[string]mockResponse),
	}
}

func (m *mockCredential) setResponse(endpoint string, data []byte, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.responses[endpoint] = mockResponse{data: data, err: err}
}

func (m *mockCredential) ExecuteAPI(ctx context.Context, endpoint string, args ...string) ([]byte, error) {
	m.callCount.Add(1)

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

func (m *mockCredential) getCallCount() int {
	return int(m.callCount.Load())
}

// makeContentResponse creates a JSON response for the GitHub contents API.
func makeContentResponse(content string) []byte {
	resp := contentResponse{
		Content:  base64.StdEncoding.EncodeToString([]byte(content)),
		Encoding: "base64",
		Type:     "file",
	}
	b, _ := json.Marshal(resp)
	return b
}

// makeDirectoryResponse creates a JSON response for a directory.
func makeDirectoryResponse() []byte {
	resp := contentResponse{
		Type: "dir",
	}
	b, _ := json.Marshal(resp)
	return b
}

func TestNewFetcher(t *testing.T) {
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
			name: "valid credential returns fetcher",
			cred: newMockCredential(),
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewFetcher(tt.cred)
			if (got != nil) != tt.want {
				t.Errorf("NewFetcher() = %v, want non-nil=%v", got, tt.want)
			}
		})
	}
}

func TestFetcher_GetRuleset_CacheHit(t *testing.T) {
	cred := newMockCredential()
	cred.setResponse("repos/owner/repo/contents/.github/CODEOWNERS",
		makeContentResponse("* @global-owner"), nil)

	f := NewFetcher(cred)
	ctx := context.Background()

	// First call - should hit API
	rs1, err := f.GetRuleset(ctx, "owner/repo")
	if err != nil {
		t.Fatalf("first GetRuleset() error = %v", err)
	}
	if rs1 == nil {
		t.Fatal("first GetRuleset() returned nil")
	}
	if cred.getCallCount() != 1 {
		t.Errorf("first call: API calls = %d, want 1", cred.getCallCount())
	}

	// Second call - should use cache (no API call)
	rs2, err := f.GetRuleset(ctx, "owner/repo")
	if err != nil {
		t.Fatalf("second GetRuleset() error = %v", err)
	}
	if rs2 != rs1 {
		t.Error("second GetRuleset() returned different ruleset (cache miss)")
	}
	if cred.getCallCount() != 1 {
		t.Errorf("after second call: API calls = %d, want 1 (should use cache)", cred.getCallCount())
	}
}

func TestFetcher_GetRuleset_CacheMiss(t *testing.T) {
	cred := newMockCredential()
	cred.setResponse("repos/owner/repo1/contents/.github/CODEOWNERS",
		makeContentResponse("* @team1"), nil)
	cred.setResponse("repos/owner/repo2/contents/.github/CODEOWNERS",
		makeContentResponse("* @team2"), nil)

	f := NewFetcher(cred)
	ctx := context.Background()

	// First repo
	rs1, err := f.GetRuleset(ctx, "owner/repo1")
	if err != nil {
		t.Fatalf("repo1 GetRuleset() error = %v", err)
	}
	if rs1 == nil || len(rs1.Rules) != 1 {
		t.Fatal("repo1 GetRuleset() unexpected result")
	}

	// Different repo - should hit API again
	rs2, err := f.GetRuleset(ctx, "owner/repo2")
	if err != nil {
		t.Fatalf("repo2 GetRuleset() error = %v", err)
	}
	if rs2 == nil || len(rs2.Rules) != 1 {
		t.Fatal("repo2 GetRuleset() unexpected result")
	}

	if cred.getCallCount() != 2 {
		t.Errorf("API calls = %d, want 2", cred.getCallCount())
	}
}

func TestFetcher_GetRuleset_FallbackLocations(t *testing.T) {
	//nolint:govet // test struct field order prioritizes readability
	tests := []struct {
		name           string
		foundAt        string // which location has the file
		expectedCalls  int
		expectedOwners []string
	}{
		{
			name:           "found at .github/CODEOWNERS",
			foundAt:        ".github/CODEOWNERS",
			expectedCalls:  1,
			expectedOwners: []string{"@github-team"},
		},
		{
			name:           "found at root CODEOWNERS",
			foundAt:        "CODEOWNERS",
			expectedCalls:  2, // tries .github first
			expectedOwners: []string{"@root-team"},
		},
		{
			name:           "found at docs/CODEOWNERS",
			foundAt:        "docs/CODEOWNERS",
			expectedCalls:  3, // tries .github, root first
			expectedOwners: []string{"@docs-team"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cred := newMockCredential()

			// Set up responses based on where file should be found
			switch tt.foundAt {
			case ".github/CODEOWNERS":
				cred.setResponse("repos/owner/repo/contents/.github/CODEOWNERS",
					makeContentResponse("* @github-team"), nil)
			case "CODEOWNERS":
				cred.setResponse("repos/owner/repo/contents/CODEOWNERS",
					makeContentResponse("* @root-team"), nil)
			case "docs/CODEOWNERS":
				cred.setResponse("repos/owner/repo/contents/docs/CODEOWNERS",
					makeContentResponse("* @docs-team"), nil)
			}

			f := NewFetcher(cred)
			ctx := context.Background()

			rs, err := f.GetRuleset(ctx, "owner/repo")
			if err != nil {
				t.Fatalf("GetRuleset() error = %v", err)
			}
			if rs == nil {
				t.Fatal("GetRuleset() returned nil")
			}
			if len(rs.Rules) != 1 {
				t.Fatalf("GetRuleset() rules count = %d, want 1", len(rs.Rules))
			}
			if rs.Rules[0].Owners[0] != tt.expectedOwners[0] {
				t.Errorf("owner = %s, want %s", rs.Rules[0].Owners[0], tt.expectedOwners[0])
			}
			if cred.getCallCount() != tt.expectedCalls {
				t.Errorf("API calls = %d, want %d", cred.getCallCount(), tt.expectedCalls)
			}
		})
	}
}

func TestFetcher_GetRuleset_NoCODEOWNERS(t *testing.T) {
	cred := newMockCredential()
	// No responses set - all locations return 404

	f := NewFetcher(cred)
	ctx := context.Background()

	rs, err := f.GetRuleset(ctx, "owner/repo")
	if err != nil {
		t.Fatalf("GetRuleset() error = %v, want nil", err)
	}
	if rs != nil {
		t.Errorf("GetRuleset() = %v, want nil (no CODEOWNERS)", rs)
	}
	// Should have tried all 3 locations
	if cred.getCallCount() != 3 {
		t.Errorf("API calls = %d, want 3", cred.getCallCount())
	}
}

func TestFetcher_GetRuleset_APIError(t *testing.T) {
	cred := newMockCredential()
	cred.setResponse("repos/owner/repo/contents/.github/CODEOWNERS",
		nil, errors.New("500 Internal Server Error"))

	f := NewFetcher(cred)
	ctx := context.Background()

	_, err := f.GetRuleset(ctx, "owner/repo")
	if err == nil {
		t.Fatal("GetRuleset() error = nil, want error")
	}
	if !containsString(err.Error(), "api call failed") {
		t.Errorf("error = %v, want to contain 'api call failed'", err)
	}
}

func TestFetcher_GetRuleset_InvalidRepoFormat(t *testing.T) {
	cred := newMockCredential()
	f := NewFetcher(cred)
	ctx := context.Background()

	tests := []string{
		"noslash",
		"",
		" ",
	}

	for _, repo := range tests {
		t.Run(repo, func(t *testing.T) {
			_, err := f.GetRuleset(ctx, repo)
			if err == nil {
				t.Error("GetRuleset() error = nil, want error for invalid format")
			}
			if !containsString(err.Error(), "invalid repo format") {
				t.Errorf("error = %v, want to contain 'invalid repo format'", err)
			}
		})
	}
}

func TestFetcher_GetRuleset_DirectoryAtLocation(t *testing.T) {
	cred := newMockCredential()
	// .github/CODEOWNERS is a directory (weird but possible)
	cred.setResponse("repos/owner/repo/contents/.github/CODEOWNERS",
		makeDirectoryResponse(), nil)
	// Root CODEOWNERS has the actual file
	cred.setResponse("repos/owner/repo/contents/CODEOWNERS",
		makeContentResponse("* @root-owner"), nil)

	f := NewFetcher(cred)
	ctx := context.Background()

	rs, err := f.GetRuleset(ctx, "owner/repo")
	if err != nil {
		t.Fatalf("GetRuleset() error = %v", err)
	}
	if rs == nil {
		t.Fatal("GetRuleset() returned nil")
	}
	if rs.Rules[0].Owners[0] != "@root-owner" {
		t.Errorf("owner = %s, want @root-owner", rs.Rules[0].Owners[0])
	}
}

func TestFetcher_GetRuleset_ParseError(t *testing.T) {
	cred := newMockCredential()
	// Return invalid base64
	cred.setResponse("repos/owner/repo/contents/.github/CODEOWNERS",
		[]byte(`{"content": "not-valid-base64!!!", "encoding": "base64", "type": "file"}`), nil)

	f := NewFetcher(cred)
	ctx := context.Background()

	_, err := f.GetRuleset(ctx, "owner/repo")
	if err == nil {
		t.Fatal("GetRuleset() error = nil, want error for invalid base64")
	}
}

func TestFetcher_GetRuleset_ContextCancellation(t *testing.T) {
	cred := newMockCredential()
	cred.setResponse("repos/owner/repo/contents/.github/CODEOWNERS",
		makeContentResponse("* @owner"), nil)

	f := NewFetcher(cred)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := f.GetRuleset(ctx, "owner/repo")
	if err == nil {
		t.Fatal("GetRuleset() error = nil, want context cancellation error")
	}
}

func TestFetcher_InvalidateCache(t *testing.T) {
	cred := newMockCredential()
	cred.setResponse("repos/owner/repo1/contents/.github/CODEOWNERS",
		makeContentResponse("* @team1"), nil)
	cred.setResponse("repos/owner/repo2/contents/.github/CODEOWNERS",
		makeContentResponse("* @team2"), nil)

	f := NewFetcher(cred)
	ctx := context.Background()

	// Populate cache
	_, _ = f.GetRuleset(ctx, "owner/repo1")
	_, _ = f.GetRuleset(ctx, "owner/repo2")

	if f.CacheSize() != 2 {
		t.Fatalf("cache size = %d, want 2", f.CacheSize())
	}

	// Invalidate specific repo
	f.InvalidateCache("owner/repo1")
	if f.CacheSize() != 1 {
		t.Errorf("after invalidate repo1: cache size = %d, want 1", f.CacheSize())
	}

	// Invalidate all
	f.InvalidateCache("")
	if f.CacheSize() != 0 {
		t.Errorf("after invalidate all: cache size = %d, want 0", f.CacheSize())
	}
}

func TestFetcher_InvalidateCache_RefetchAfterInvalidation(t *testing.T) {
	cred := newMockCredential()
	cred.setResponse("repos/owner/repo/contents/.github/CODEOWNERS",
		makeContentResponse("* @original-owner"), nil)

	f := NewFetcher(cred)
	ctx := context.Background()

	// First fetch
	rs1, _ := f.GetRuleset(ctx, "owner/repo")
	initialCallCount := cred.getCallCount()

	// Update the response (simulating file change)
	cred.setResponse("repos/owner/repo/contents/.github/CODEOWNERS",
		makeContentResponse("* @new-owner"), nil)

	// Second fetch - should use cache
	rs2, _ := f.GetRuleset(ctx, "owner/repo")
	if cred.getCallCount() != initialCallCount {
		t.Error("cache was not used after file change")
	}
	if rs1.Rules[0].Owners[0] != rs2.Rules[0].Owners[0] {
		t.Error("cache returned different ruleset unexpectedly")
	}

	// Invalidate and refetch
	f.InvalidateCache("owner/repo")
	rs3, _ := f.GetRuleset(ctx, "owner/repo")

	if cred.getCallCount() <= initialCallCount {
		t.Error("API was not called after cache invalidation")
	}
	if rs3.Rules[0].Owners[0] != "@new-owner" {
		t.Errorf("after invalidation, owner = %s, want @new-owner", rs3.Rules[0].Owners[0])
	}
}

func TestFetcher_ConcurrentAccess(t *testing.T) {
	cred := newMockCredential()
	cred.setResponse("repos/owner/repo/contents/.github/CODEOWNERS",
		makeContentResponse("* @concurrent-owner"), nil)

	f := NewFetcher(cred)
	ctx := context.Background()

	const numGoroutines = 50
	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	errChan := make(chan error, numGoroutines)
	rsChan := make(chan *Ruleset, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			rs, err := f.GetRuleset(ctx, "owner/repo")
			if err != nil {
				errChan <- err
				return
			}
			rsChan <- rs
		}()
	}

	wg.Wait()
	close(errChan)
	close(rsChan)

	// Check for errors
	for err := range errChan {
		t.Errorf("concurrent GetRuleset() error = %v", err)
	}

	// All results should be the same cached ruleset
	var firstRs *Ruleset
	for rs := range rsChan {
		if firstRs == nil {
			firstRs = rs
		} else if rs != firstRs {
			// In a properly cached system, all goroutines should get the same pointer
			// However, due to race conditions in cache population, different instances
			// with the same content are acceptable
			if len(rs.Rules) != len(firstRs.Rules) {
				t.Error("concurrent access returned rulesets with different rules")
			}
		}
	}

	// Should have made very few API calls due to caching
	// The first goroutine to acquire the write lock will fetch,
	// others should use the cache
	if cred.getCallCount() > 5 {
		t.Errorf("API calls = %d, want <= 5 (most should use cache)", cred.getCallCount())
	}
}

func TestFetcher_ConcurrentDifferentRepos(t *testing.T) {
	cred := newMockCredential()
	repos := []string{"owner/repo1", "owner/repo2", "owner/repo3"}
	for _, repo := range repos {
		cred.setResponse("repos/"+repo+"/contents/.github/CODEOWNERS",
			makeContentResponse("* @team-"+repo), nil)
	}

	f := NewFetcher(cred)
	ctx := context.Background()

	var wg sync.WaitGroup
	wg.Add(len(repos) * 10)

	for i := 0; i < 10; i++ {
		for _, repo := range repos {
			repo := repo
			go func() {
				defer wg.Done()
				rs, err := f.GetRuleset(ctx, repo)
				if err != nil {
					t.Errorf("GetRuleset(%s) error = %v", repo, err)
					return
				}
				if rs == nil {
					t.Errorf("GetRuleset(%s) returned nil", repo)
				}
			}()
		}
	}

	wg.Wait()

	// Should have 3 cached entries
	if f.CacheSize() != 3 {
		t.Errorf("cache size = %d, want 3", f.CacheSize())
	}
}

func TestFetcher_NilReceiver(t *testing.T) {
	var f *Fetcher

	ctx := context.Background()

	// All methods should handle nil receiver gracefully
	rs, err := f.GetRuleset(ctx, "owner/repo")
	if err != nil {
		t.Errorf("nil.GetRuleset() error = %v, want nil", err)
	}
	if rs != nil {
		t.Errorf("nil.GetRuleset() = %v, want nil", rs)
	}

	// InvalidateCache should not panic
	f.InvalidateCache("")
	f.InvalidateCache("owner/repo")

	// CacheSize should return 0
	if f.CacheSize() != 0 {
		t.Errorf("nil.CacheSize() = %d, want 0", f.CacheSize())
	}
}

func TestDecodeContent(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		encoding string
		want     string
		wantErr  bool
	}{
		{
			name:     "valid base64",
			content:  base64.StdEncoding.EncodeToString([]byte("* @owner")),
			encoding: "base64",
			want:     "* @owner",
			wantErr:  false,
		},
		{
			name:     "base64 with newlines",
			content:  "KiBA\nb3du\nZXI=",
			encoding: "base64",
			want:     "* @owner",
			wantErr:  false,
		},
		{
			name:     "empty encoding defaults to base64",
			content:  base64.StdEncoding.EncodeToString([]byte("content")),
			encoding: "",
			want:     "content",
			wantErr:  false,
		},
		{
			name:     "unsupported encoding",
			content:  "content",
			encoding: "utf-8",
			want:     "",
			wantErr:  true,
		},
		{
			name:     "invalid base64",
			content:  "not-valid-base64!!!",
			encoding: "base64",
			want:     "",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := decodeContent(tt.content, tt.encoding)
			if (err != nil) != tt.wantErr {
				t.Errorf("decodeContent() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if string(got) != tt.want {
				t.Errorf("decodeContent() = %q, want %q", string(got), tt.want)
			}
		})
	}
}

func TestFetcher_GetRuleset_ComplexCODEOWNERS(t *testing.T) {
	content := `# Global owners
* @global-owner

# Frontend
*.js @frontend-team
*.ts @frontend-team
src/components/** @ui-team @frontend-team

# Backend
*.go @backend-team
/internal/** @internal-team

# Documentation
*.md @docs-team
`

	cred := newMockCredential()
	cred.setResponse("repos/owner/repo/contents/.github/CODEOWNERS",
		makeContentResponse(content), nil)

	f := NewFetcher(cred)
	ctx := context.Background()

	rs, err := f.GetRuleset(ctx, "owner/repo")
	if err != nil {
		t.Fatalf("GetRuleset() error = %v", err)
	}
	if rs == nil {
		t.Fatal("GetRuleset() returned nil")
	}

	// Verify rules were parsed correctly
	expectedRules := 7
	if len(rs.Rules) != expectedRules {
		t.Errorf("rules count = %d, want %d", len(rs.Rules), expectedRules)
	}

	// Test matching
	tests := []struct {
		path string
		want []string
	}{
		{"README.md", []string{"@docs-team"}},
		{"main.go", []string{"@backend-team"}},
		{"app.js", []string{"@frontend-team"}},
		{"src/components/Button.tsx", []string{"@ui-team", "@frontend-team"}},
		{"internal/auth/auth.go", []string{"@internal-team"}},
		{"random.txt", []string{"@global-owner"}},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := rs.Match(tt.path)
			if !stringSliceEqual(got, tt.want) {
				t.Errorf("Match(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

// TestFetcher_RaceCondition runs the race detector on concurrent operations.
func TestFetcher_RaceCondition(t *testing.T) {
	cred := newMockCredential()
	cred.setResponse("repos/owner/repo/contents/.github/CODEOWNERS",
		makeContentResponse("* @owner"), nil)

	f := NewFetcher(cred)
	ctx := context.Background()

	var wg sync.WaitGroup
	wg.Add(100)

	// Mix of operations to trigger race conditions
	for i := 0; i < 100; i++ {
		go func(n int) {
			defer wg.Done()
			switch n % 5 {
			case 0:
				_, _ = f.GetRuleset(ctx, "owner/repo")
			case 1:
				f.InvalidateCache("owner/repo")
			case 2:
				f.InvalidateCache("")
			case 3:
				_ = f.CacheSize()
			case 4:
				_, _ = f.GetRuleset(ctx, "owner/repo2")
			}
		}(i)
	}

	wg.Wait()
}

func TestFetcher_ContextTimeout(t *testing.T) {
	// Create a mock that delays response
	cred := &slowMockCredential{
		delay: 100 * time.Millisecond,
	}

	f := NewFetcher(cred)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	_, err := f.GetRuleset(ctx, "owner/repo")
	if err == nil {
		t.Error("GetRuleset() error = nil, want timeout error")
	}
}

// slowMockCredential simulates a slow API response.
type slowMockCredential struct {
	delay time.Duration
}

func (s *slowMockCredential) ExecuteAPI(ctx context.Context, endpoint string, args ...string) ([]byte, error) {
	select {
	case <-time.After(s.delay):
		return makeContentResponse("* @owner"), nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// Helper functions

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr ||
		len(s) > len(substr) && (s[:len(substr)] == substr ||
			s[len(s)-len(substr):] == substr ||
			containsSubstr(s, substr)))
}

func containsSubstr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func stringSliceEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// Benchmarks

func BenchmarkFetcher_GetRuleset_CacheHit(b *testing.B) {
	cred := newMockCredential()
	cred.setResponse("repos/owner/repo/contents/.github/CODEOWNERS",
		makeContentResponse("* @global-owner\n*.go @backend"), nil)

	f := NewFetcher(cred)
	ctx := context.Background()

	// Populate cache
	_, _ = f.GetRuleset(ctx, "owner/repo")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = f.GetRuleset(ctx, "owner/repo")
	}
}

func BenchmarkFetcher_GetRuleset_CacheMiss(b *testing.B) {
	cred := newMockCredential()
	f := NewFetcher(cred)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Use unique repo names to force cache miss
		repo := "owner/repo" + string(rune(i%26+'a'))
		cred.setResponse("repos/"+repo+"/contents/.github/CODEOWNERS",
			makeContentResponse("* @owner"), nil)
		f.InvalidateCache(repo)
		_, _ = f.GetRuleset(ctx, repo)
	}
}

func BenchmarkFetcher_ConcurrentAccess(b *testing.B) {
	cred := newMockCredential()
	cred.setResponse("repos/owner/repo/contents/.github/CODEOWNERS",
		makeContentResponse("* @global-owner"), nil)

	f := NewFetcher(cred)
	ctx := context.Background()

	// Populate cache
	_, _ = f.GetRuleset(ctx, "owner/repo")

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, _ = f.GetRuleset(ctx, "owner/repo")
		}
	})
}
