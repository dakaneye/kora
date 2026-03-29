package source_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/dakaneye/kora/internal/source"
)

func TestLinear_Name(t *testing.T) {
	lin := source.NewLinear(nil)
	if lin.Name() != "linear" {
		t.Errorf("name = %q, want %q", lin.Name(), "linear")
	}
}

func TestLinear_CheckAuth_Success(t *testing.T) {
	runner := &fakeRunner{
		results: map[string]fakeResult{
			"linear auth whoami": {stdout: "sam@netboxlabs.com"},
		},
	}
	lin := source.NewLinear(runner)
	if err := lin.CheckAuth(t.Context()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLinear_CheckAuth_Failure(t *testing.T) {
	runner := &fakeRunner{
		results: map[string]fakeResult{
			"linear auth whoami": {err: "not authenticated"},
		},
	}
	lin := source.NewLinear(runner)
	if err := lin.CheckAuth(t.Context()); err == nil {
		t.Fatal("expected error for failed auth")
	}
}

func TestLinear_RefreshAuth(t *testing.T) {
	runner := &fakeRunner{
		results: map[string]fakeResult{
			"linear auth login": {stdout: ""},
		},
	}
	lin := source.NewLinear(runner)
	if err := lin.RefreshAuth(t.Context()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLinear_Fetch(t *testing.T) {
	// The fakeRunner uses prefix matching. All `linear api` calls will match
	// the same prefix, so we return a generic valid JSON response.
	apiResponse := `{"data":{"viewer":{"assignedIssues":{"nodes":[{"identifier":"ENG-123","title":"Fix bug"}]}}}}`

	runner := &fakeRunner{
		results: map[string]fakeResult{
			"linear api": {stdout: apiResponse},
		},
	}
	lin := source.NewLinear(runner)
	data, err := lin.Fetch(t.Context(), 7*24*time.Hour)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result map[string]json.RawMessage
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	// Verify we get all expected sub-keys
	for _, key := range []string{"assigned_issues", "cycles", "commented_issues", "completed_issues"} {
		if _, ok := result[key]; !ok {
			t.Errorf("missing key %q in output", key)
		}
	}
}
