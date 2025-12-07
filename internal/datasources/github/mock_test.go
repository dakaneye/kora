package github

import (
	"context"
	"fmt"

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
func (m *mockGitHubDelegatedCredential) ExecuteAPI(ctx context.Context, endpoint string, args ...string) ([]byte, error) {
	m.callCount++

	// Handle GraphQL endpoint
	if endpoint == "graphql" {
		return m.handleGraphQL(args)
	}

	// Handle REST endpoint (legacy)
	return m.handleREST(args)
}

// handleGraphQL processes GraphQL API calls.
func (m *mockGitHubDelegatedCredential) handleGraphQL(args []string) ([]byte, error) {
	// Extract query and variables from args
	var queryStr, varsStr string
	for i, arg := range args {
		if arg == "-f" && i+1 < len(args) {
			val := args[i+1]
			if len(val) > 6 && val[:6] == "query=" {
				queryStr = val[6:]
			} else if len(val) > 10 && val[:10] == "variables=" {
				varsStr = val[10:]
			}
		}
	}

	// Determine the type of GraphQL query based on content
	var key string
	switch {
	case contains(queryStr, "SearchPRs") || contains(queryStr, "search(query"):
		// It's a search query, determine type from variables
		key = determineSearchKey(varsStr)
	case contains(queryStr, "PRContext") || contains(queryStr, "pullRequest(number"):
		// It's a PR context query
		key = "graphql:pr:context"
	case contains(queryStr, "IssueContext") || contains(queryStr, "issue(number"):
		// It's an issue context query
		key = "graphql:issue:context"
	default:
		return nil, fmt.Errorf("mock: unknown graphql query type")
	}

	resp, ok := m.responses[key]
	if !ok {
		return nil, fmt.Errorf("mock: no response configured for graphql key %s", key)
	}
	if resp.err != nil {
		return nil, resp.err
	}
	return resp.data, nil
}

// handleREST processes REST API calls (legacy support for old tests).
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

	resp, ok := m.responses[key]
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
func (m *mockGitHubDelegatedCredential) setGraphQLResponse(key string, data []byte) {
	m.responses[key] = mockAPIResponse{data: data}
}

// setGraphQLError configures the mock to return an error for a GraphQL query type.
func (m *mockGitHubDelegatedCredential) setGraphQLError(key string, err error) {
	m.responses[key] = mockAPIResponse{err: err}
}

// contains is a simple substring check helper
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && findSubstring(s, substr))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// determineSearchKey maps GraphQL search query variables to a mock response key.
func determineSearchKey(varsStr string) string {
	switch {
	case contains(varsStr, "review-requested"):
		return "graphql:search:pr-review"
	case contains(varsStr, "mentions") && contains(varsStr, "type:pr"):
		return "graphql:search:pr-mention"
	case contains(varsStr, "mentions") && contains(varsStr, "type:issue"):
		return "graphql:search:issue-mention"
	case contains(varsStr, "assignee"):
		return "graphql:search:issue-assigned"
	default:
		return "graphql:search:default"
	}
}
