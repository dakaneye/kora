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
	// key format: "review-requested", "mentions:pr", "mentions:issue", "assignee"
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
// Matches based on query content to return the right test data.
func (m *mockGitHubDelegatedCredential) ExecuteAPI(ctx context.Context, endpoint string, args ...string) ([]byte, error) {
	m.callCount++

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

// setResponseForQuery configures the mock to return data for a query pattern.
// key should be one of: "review-requested", "mentions:pr", "mentions:issue", "assignee"
func (m *mockGitHubDelegatedCredential) setResponseForQuery(key string, data []byte) {
	m.responses[key] = mockAPIResponse{data: data}
}

// setErrorForQuery configures the mock to return an error for a query pattern.
func (m *mockGitHubDelegatedCredential) setErrorForQuery(key string, err error) {
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
