//go:build integration

package integration_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/dakaneye/kora/internal/source"
)

func TestGitHub_Integration(t *testing.T) {
	gh := source.NewGitHub(nil, nil, nil)
	if err := gh.CheckAuth(t.Context()); err != nil {
		t.Skipf("gh auth not configured: %v", err)
	}

	data, err := gh.Fetch(t.Context(), 24*time.Hour)
	if err != nil {
		t.Fatalf("fetch failed: %v", err)
	}

	var result map[string]json.RawMessage
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	for _, key := range []string{"review_requests", "authored_prs", "assigned_issues", "commented_prs"} {
		if _, ok := result[key]; !ok {
			t.Errorf("missing key %q", key)
		}
	}
	t.Logf("github returned %d bytes", len(data))
}

func TestGmail_Integration(t *testing.T) {
	gm := source.NewGmail(nil)
	if err := gm.CheckAuth(t.Context()); err != nil {
		t.Skipf("gws auth not configured: %v", err)
	}

	data, err := gm.Fetch(t.Context(), 24*time.Hour)
	if err != nil {
		t.Fatalf("fetch failed: %v", err)
	}

	var result map[string]json.RawMessage
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if _, ok := result["messages"]; !ok {
		t.Error("missing 'messages' key")
	}
	t.Logf("gmail returned %d bytes", len(data))
}

func TestCalendar_Integration(t *testing.T) {
	cal := source.NewCalendar(nil)
	if err := cal.CheckAuth(t.Context()); err != nil {
		t.Skipf("gws auth not configured: %v", err)
	}

	data, err := cal.Fetch(t.Context(), 24*time.Hour)
	if err != nil {
		t.Fatalf("fetch failed: %v", err)
	}

	var result map[string]json.RawMessage
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if _, ok := result["events"]; !ok {
		t.Error("missing 'events' key")
	}
	t.Logf("calendar returned %d bytes", len(data))
}

func TestLinear_Integration(t *testing.T) {
	lin := source.NewLinear(nil, nil)
	if err := lin.CheckAuth(t.Context()); err != nil {
		t.Skipf("linear auth not configured: %v", err)
	}

	data, err := lin.Fetch(t.Context(), 7*24*time.Hour)
	if err != nil {
		t.Fatalf("fetch failed: %v", err)
	}

	var result map[string]json.RawMessage
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	t.Logf("linear returned %d bytes", len(data))
}
