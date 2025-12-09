package github

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/dakaneye/kora/internal/auth"
	ghauth "github.com/dakaneye/kora/internal/auth/github"
)

// mockAuthProvider implements auth.AuthProvider for testing.
type mockAuthProvider struct {
	credential      *mockGitHubDelegatedCredential
	authenticateErr error
	getCredErr      error
	service         auth.Service
	authenticated   bool
}

func newMockAuthProvider(service auth.Service) *mockAuthProvider {
	return &mockAuthProvider{
		service:       service,
		authenticated: true,
		credential:    newMockGitHubDelegatedCredential(),
	}
}

func (m *mockAuthProvider) Service() auth.Service {
	return m.service
}

func (m *mockAuthProvider) Authenticate(ctx context.Context) error {
	if m.authenticateErr != nil {
		return m.authenticateErr
	}
	if !m.authenticated {
		return auth.ErrNotAuthenticated
	}
	return nil
}

func (m *mockAuthProvider) GetCredential(ctx context.Context) (auth.Credential, error) {
	if m.getCredErr != nil {
		return nil, m.getCredErr
	}
	if !m.authenticated {
		return nil, auth.ErrNotAuthenticated
	}
	return m.credential, nil
}

func (m *mockAuthProvider) IsAuthenticated(ctx context.Context) bool {
	return m.authenticated
}

// mockGitHubDelegatedCredential embeds the real GitHubDelegatedCredential
// and overrides ExecuteAPI to return test data.
type mockGitHubDelegatedCredential struct {
	*ghauth.GitHubDelegatedCredential
	// responses maps query patterns to response data
	// For REST: "review-requested", "mentions:pr", "mentions:issue", "assignee"
	// For GraphQL search: "graphql:search:pr-review", "graphql:search:pr-mention", etc.
	// For GraphQL context: "graphql:pr:owner/repo/123", "graphql:issue:owner/repo/456"
	responses map[string]mockAPIResponse
	callCount int
	mu        sync.RWMutex // protects responses and callCount for concurrent access
}

type mockAPIResponse struct {
	err  error
	data []byte
}

func newMockGitHubDelegatedCredential() *mockGitHubDelegatedCredential {
	// Create a real GitHubDelegatedCredential with test username
	realCred, _ := ghauth.NewGitHubDelegatedCredential("testuser", "gh")
	return &mockGitHubDelegatedCredential{
		GitHubDelegatedCredential: realCred,
		responses:                 make(map[string]mockAPIResponse),
	}
}

// ExecuteAPI overrides the real implementation to return test data.
// This allows us to test without calling the actual gh CLI.
// Supports both REST and GraphQL endpoints.
// Thread-safe for concurrent access from parallel goroutines.
func (m *mockGitHubDelegatedCredential) ExecuteAPI(ctx context.Context, endpoint string, args ...string) ([]byte, error) {
	m.mu.Lock()
	m.callCount++
	m.mu.Unlock()

	// Handle GraphQL endpoint
	if endpoint == "graphql" {
		return m.handleGraphQL(args)
	}

	// Handle "user" endpoint for current user lookup (used by authored PRs)
	if endpoint == "user" {
		m.mu.RLock()
		resp, ok := m.responses["rest:user"]
		m.mu.RUnlock()
		if ok {
			if resp.err != nil {
				return nil, resp.err
			}
			return resp.data, nil
		}
		// Default: return a mock user response
		return []byte(`{"login": "testuser"}`), nil
	}

	// Handle direct REST endpoint (e.g., "repos/owner/repo/contents/path")
	// Check for direct endpoint match first
	m.mu.RLock()
	resp, ok := m.responses["rest:"+endpoint]
	m.mu.RUnlock()
	if ok {
		if resp.err != nil {
			return nil, resp.err
		}
		return resp.data, nil
	}

	// Handle REST search endpoint (legacy)
	return m.handleREST(args)
}

// handleGraphQL processes GraphQL API calls.
// Thread-safe: uses RLock to read from responses map.
func (m *mockGitHubDelegatedCredential) handleGraphQL(args []string) ([]byte, error) {
	// Extract query and variables from args
	// Variables are now passed individually as -f key=value or -F key=value
	// The FIRST "query=" is the GraphQL query, subsequent "query=" are variables
	var queryStr string
	queryFound := false
	vars := make(map[string]string)

	for i := 0; i < len(args); i++ {
		if (args[i] == "-f" || args[i] == "-F") && i+1 < len(args) {
			val := args[i+1]
			idx := strings.Index(val, "=")
			if idx > 0 {
				key := val[:idx]
				value := val[idx+1:]
				if key == "query" && !queryFound {
					// First "query=" is the GraphQL query itself
					queryStr = value
					queryFound = true
				} else {
					// All other fields are GraphQL variables
					vars[key] = value
				}
			}
			i++ // Skip the value argument
		}
	}

	// Determine the type of GraphQL query based on content
	var key string
	switch {
	case contains(queryStr, "SearchPRs"):
		// It's a PR search query, determine type from the searchQuery variable
		key = determineSearchKey(vars["searchQuery"])
	case contains(queryStr, "SearchIssues"):
		// It's an issue search query, determine type from the searchQuery variable
		key = determineSearchKey(vars["searchQuery"])
	case contains(queryStr, "PRContext"):
		// It's a PR context query
		key = "graphql:pr:context"
	case contains(queryStr, "IssueContext"):
		// It's an issue context query
		key = "graphql:issue:context"
	default:
		return nil, fmt.Errorf("mock: unknown graphql query type, queryStr=%q, vars=%v", queryStr[:min(100, len(queryStr))], vars)
	}

	m.mu.RLock()
	resp, ok := m.responses[key]
	m.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("mock: no response configured for graphql key %s", key)
	}
	if resp.err != nil {
		return nil, resp.err
	}
	return resp.data, nil
}

// handleREST processes REST API calls (legacy support for old tests).
// Thread-safe: uses RLock to read from responses map.
func (m *mockGitHubDelegatedCredential) handleREST(args []string) ([]byte, error) {
	// Extract query from args to determine which response to return
	var query string
	for i, arg := range args {
		if arg == "-f" && i+1 < len(args) {
			if len(args[i+1]) > 2 && args[i+1][:2] == "q=" {
				query = args[i+1][2:]
				break
			}
		}
	}

	// Match query to response key
	var key string
	switch {
	case query != "" && contains(query, "review-requested"):
		key = "review-requested"
	case query != "" && contains(query, "mentions") && contains(query, "type:pr"):
		key = "mentions:pr"
	case query != "" && contains(query, "mentions") && contains(query, "type:issue"):
		key = "mentions:issue"
	case query != "" && contains(query, "assignee"):
		key = "assignee"
	default:
		return nil, fmt.Errorf("mock: no response configured for query %s", query)
	}

	m.mu.RLock()
	resp, ok := m.responses[key]
	m.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("mock: no response configured for key %s", key)
	}
	if resp.err != nil {
		return nil, resp.err
	}
	return resp.data, nil
}

// setGraphQLResponse configures the mock to return data for a GraphQL query type.
// key should be one of:
// - "graphql:search:pr-review", "graphql:search:pr-mention", "graphql:search:issue-mention", "graphql:search:issue-assigned"
// - "graphql:pr:context", "graphql:issue:context"
// Thread-safe: uses Lock to write to responses map.
func (m *mockGitHubDelegatedCredential) setGraphQLResponse(key string, data []byte) {
	m.mu.Lock()
	m.responses[key] = mockAPIResponse{data: data}
	m.mu.Unlock()
}

// setGraphQLError configures the mock to return an error for a GraphQL query type.
// Thread-safe: uses Lock to write to responses map.
func (m *mockGitHubDelegatedCredential) setGraphQLError(key string, err error) {
	m.mu.Lock()
	m.responses[key] = mockAPIResponse{err: err}
	m.mu.Unlock()
}

// contains is a simple substring check helper
func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}

// determineSearchKey maps GraphQL search query variables to a mock response key.
// queryVar is the value of the $query GraphQL variable (the GitHub search query string).
func determineSearchKey(queryVar string) string {
	switch {
	case contains(queryVar, "review-requested"):
		return "graphql:search:pr-review"
	case contains(queryVar, "mentions") && contains(queryVar, "type:pr"):
		return "graphql:search:pr-mention"
	case contains(queryVar, "mentions") && contains(queryVar, "type:issue"):
		return "graphql:search:issue-mention"
	case contains(queryVar, "assignee"):
		return "graphql:search:issue-assigned"
	case contains(queryVar, "commenter:"):
		return "graphql:search:issue-commenter"
	case contains(queryVar, "author:"):
		return "graphql:search:pr-author"
	case contains(queryVar, "involves:"):
		return "graphql:search:pr-involves"
	case contains(queryVar, "repo:") && contains(queryVar, "is:merged"):
		return "graphql:search:pr-watched"
	default:
		return "graphql:search:default"
	}
}
