package source_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/dakaneye/kora/internal/source"
)

func TestGitHub_Name(t *testing.T) {
	gh := source.NewGitHub(nil)
	if gh.Name() != "github" {
		t.Errorf("name = %q, want %q", gh.Name(), "github")
	}
}

func TestGitHub_CheckAuth_Success(t *testing.T) {
	runner := &fakeRunner{
		results: map[string]fakeResult{
			"gh auth status": {stdout: "Logged in to github.com"},
		},
	}
	gh := source.NewGitHub(runner)
	if err := gh.CheckAuth(t.Context()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGitHub_CheckAuth_Failure(t *testing.T) {
	runner := &fakeRunner{
		results: map[string]fakeResult{
			"gh auth status": {err: "not logged in"},
		},
	}
	gh := source.NewGitHub(runner)
	if err := gh.CheckAuth(t.Context()); err == nil {
		t.Fatal("expected error for failed auth")
	}
}

func TestGitHub_Fetch(t *testing.T) {
	prs := `[{"number":1,"title":"fix bug"}]`
	issues := `[{"number":2,"title":"add feature"}]`
	runner := &fakeRunner{
		results: map[string]fakeResult{
			"gh search prs --review-requested=@me": {stdout: prs},
			"gh search prs --author=@me":           {stdout: prs},
			"gh search issues --assignee=@me":      {stdout: issues},
			"gh search prs --commenter=@me":        {stdout: prs},
		},
	}
	gh := source.NewGitHub(runner)
	data, err := gh.Fetch(t.Context(), 8*time.Hour)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result map[string]json.RawMessage
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	for _, key := range []string{"review_requests", "authored_prs", "assigned_issues", "commented_prs"} {
		if _, ok := result[key]; !ok {
			t.Errorf("missing key %q in output", key)
		}
	}
}

func TestGitHub_RefreshAuth(t *testing.T) {
	runner := &fakeRunner{
		results: map[string]fakeResult{
			"gh auth refresh": {stdout: ""},
		},
	}
	gh := source.NewGitHub(runner)
	if err := gh.RefreshAuth(t.Context()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
