package github

import (
	"encoding/json"
	"os"
	"testing"
)

// wrapGraphQLResponse wraps raw data in a GraphQL response structure.
// This converts { "search": {...} } to { "data": { "search": {...} } }
func wrapGraphQLResponse(data []byte) []byte {
	wrapped := GraphQLResponse{
		Data: json.RawMessage(data),
	}
	result, _ := json.Marshal(wrapped)
	return result
}

// loadGraphQLTestData loads GraphQL test data from testdata/graphql/ and wraps it.
func loadGraphQLTestData(t *testing.T, filename string) []byte {
	t.Helper()
	data, err := os.ReadFile("testdata/graphql/" + filename)
	if err != nil {
		t.Fatalf("failed to load graphql test data %s: %v", filename, err)
	}
	return wrapGraphQLResponse(data)
}

// setupMockForAllSearches configures the mock for all 5 GraphQL searches.
// This is the common setup for most tests.
func setupMockForAllSearches(cred *mockGitHubDelegatedCredential, t *testing.T) {
	// Load search responses
	prSearchResp := loadGraphQLTestData(t, "search_prs.json")
	issueSearchResp := loadGraphQLTestData(t, "search_issues.json")
	emptySearchResp := loadGraphQLTestData(t, "empty_search.json")

	// Load context responses
	prContextResp := loadGraphQLTestData(t, "pr_full_context.json")
	issueContextResp := loadGraphQLTestData(t, "issue_full_context.json")

	// Configure search responses
	cred.setGraphQLResponse("graphql:search:pr-review", prSearchResp)
	cred.setGraphQLResponse("graphql:search:pr-mention", emptySearchResp)
	cred.setGraphQLResponse("graphql:search:issue-mention", issueSearchResp)
	cred.setGraphQLResponse("graphql:search:issue-assigned", emptySearchResp)
	cred.setGraphQLResponse("graphql:search:pr-author", emptySearchResp) // Authored PRs

	// Configure context responses
	cred.setGraphQLResponse("graphql:pr:context", prContextResp)
	cred.setGraphQLResponse("graphql:issue:context", issueContextResp)
}

// setupMockWithPartialFailure configures the mock so some searches succeed and others fail.
func setupMockWithPartialFailure(cred *mockGitHubDelegatedCredential, t *testing.T, failKeys ...string) {
	// Load all test data
	prSearchResp := loadGraphQLTestData(t, "search_prs.json")
	issueSearchResp := loadGraphQLTestData(t, "search_issues.json")
	emptySearchResp := loadGraphQLTestData(t, "empty_search.json")
	prContextResp := loadGraphQLTestData(t, "pr_full_context.json")
	issueContextResp := loadGraphQLTestData(t, "issue_full_context.json")

	// Map of all responses
	responses := map[string][]byte{
		"graphql:search:pr-review":      prSearchResp,
		"graphql:search:pr-mention":     emptySearchResp,
		"graphql:search:issue-mention":  issueSearchResp,
		"graphql:search:issue-assigned": emptySearchResp,
		"graphql:search:pr-author":      emptySearchResp,
		"graphql:pr:context":            prContextResp,
		"graphql:issue:context":         issueContextResp,
	}

	// Create a set of keys to fail
	failSet := make(map[string]bool)
	for _, key := range failKeys {
		failSet[key] = true
	}

	// Configure responses
	for key, data := range responses {
		if failSet[key] {
			cred.setGraphQLError(key, errMockFailed)
		} else {
			cred.setGraphQLResponse(key, data)
		}
	}
}

// errMockFailed is a sentinel error for mock failures.
var errMockFailed = &mockError{"simulated failure"}

type mockError struct {
	msg string
}

func (e *mockError) Error() string {
	return e.msg
}
