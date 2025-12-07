package github

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

// mockGraphQLCredential implements githubCredential for testing GraphQL client.
//
//nolint:govet // test struct field order prioritizes readability
type mockGraphQLCredential struct {
	response []byte
	err      error
}

func (m *mockGraphQLCredential) ExecuteAPI(_ context.Context, _ string, _ ...string) ([]byte, error) {
	return m.response, m.err
}

func TestGraphQLClient_Execute_Success(t *testing.T) {
	expectedData := json.RawMessage(`{"repository":{"pullRequest":{"title":"Test PR"}}}`)
	response := GraphQLResponse{
		Data: expectedData,
	}
	responseBytes, _ := json.Marshal(response)

	cred := &mockGraphQLCredential{response: responseBytes}
	client := NewGraphQLClient(cred)

	data, err := client.Execute(context.Background(), "query { test }", nil)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if string(data) != string(expectedData) {
		t.Errorf("expected data %s, got %s", expectedData, data)
	}
}

func TestGraphQLClient_Execute_WithVariables(t *testing.T) {
	expectedData := json.RawMessage(`{"data":"test"}`)
	response := GraphQLResponse{
		Data: expectedData,
	}
	responseBytes, _ := json.Marshal(response)

	cred := &mockGraphQLCredential{response: responseBytes}
	client := NewGraphQLClient(cred)

	variables := map[string]any{
		"owner":  "org",
		"repo":   "repo",
		"number": 123,
	}

	data, err := client.Execute(context.Background(), PRQuery, variables)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if data == nil {
		t.Error("expected non-nil data")
	}
}

func TestGraphQLClient_Execute_GraphQLErrors(t *testing.T) {
	response := GraphQLResponse{
		Errors: []GraphQLError{
			{Message: "Resource not found", Type: "NOT_FOUND"},
		},
	}
	responseBytes, _ := json.Marshal(response)

	cred := &mockGraphQLCredential{response: responseBytes}
	client := NewGraphQLClient(cred)

	_, err := client.Execute(context.Background(), "query { test }", nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var gqlErr *GraphQLErrorList
	if !errors.As(err, &gqlErr) {
		t.Fatalf("expected GraphQLErrorList, got %T", err)
	}

	if !gqlErr.IsNotFound() {
		t.Error("expected IsNotFound to be true")
	}
}

func TestGraphQLClient_Execute_MultipleErrors(t *testing.T) {
	response := GraphQLResponse{
		Errors: []GraphQLError{
			{Message: "Error 1"},
			{Message: "Error 2"},
		},
	}
	responseBytes, _ := json.Marshal(response)

	cred := &mockGraphQLCredential{response: responseBytes}
	client := NewGraphQLClient(cred)

	_, err := client.Execute(context.Background(), "query { test }", nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	errMsg := err.Error()
	if errMsg != "graphql errors: Error 1; Error 2" {
		t.Errorf("unexpected error message: %s", errMsg)
	}
}

func TestGraphQLClient_Execute_APIError(t *testing.T) {
	cred := &mockGraphQLCredential{err: errors.New("connection refused")}
	client := NewGraphQLClient(cred)

	_, err := client.Execute(context.Background(), "query { test }", nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !errors.Is(err, cred.err) && err.Error() == "" {
		t.Errorf("expected wrapped error, got: %v", err)
	}
}

func TestGraphQLClient_Execute_RateLimited(t *testing.T) {
	cred := &mockGraphQLCredential{err: errors.New("API rate limit exceeded")}
	client := NewGraphQLClient(cred)

	_, err := client.Execute(context.Background(), "query { test }", nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	errMsg := err.Error()
	if errMsg == "" || !strings.Contains(errMsg, "rate limit") {
		t.Errorf("expected rate limit error, got: %s", errMsg)
	}
}

func TestGraphQLClient_Execute_InvalidJSON(t *testing.T) {
	cred := &mockGraphQLCredential{response: []byte("not json")}
	client := NewGraphQLClient(cred)

	_, err := client.Execute(context.Background(), "query { test }", nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestGraphQLErrorList_Error_Empty(t *testing.T) {
	err := &GraphQLErrorList{}
	if err.Error() != "unknown graphql error" {
		t.Errorf("unexpected error message: %s", err.Error())
	}
}

func TestGraphQLErrorList_Error_Single(t *testing.T) {
	err := &GraphQLErrorList{
		Errors: []GraphQLError{{Message: "test error"}},
	}
	if err.Error() != "graphql error: test error" {
		t.Errorf("unexpected error message: %s", err.Error())
	}
}

func TestMustMarshalVariables_Empty(t *testing.T) {
	result := mustMarshalVariables(nil)
	if result != "{}" {
		t.Errorf("expected {}, got %s", result)
	}
}

func TestMustMarshalVariables_WithValues(t *testing.T) {
	vars := map[string]any{"key": "value"}
	result := mustMarshalVariables(vars)
	if result != `{"key":"value"}` {
		t.Errorf("expected {\"key\":\"value\"}, got %s", result)
	}
}
