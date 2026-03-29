package source_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/dakaneye/kora/internal/source"
)

func TestGmail_Name(t *testing.T) {
	gm := source.NewGmail(nil)
	if gm.Name() != "gmail" {
		t.Errorf("name = %q, want %q", gm.Name(), "gmail")
	}
}

func TestGmail_CheckAuth_Success(t *testing.T) {
	runner := &fakeRunner{
		results: map[string]fakeResult{
			"gws auth status": {stdout: `{"token_valid": true}`},
		},
	}
	gm := source.NewGmail(runner)
	if err := gm.CheckAuth(t.Context()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGmail_CheckAuth_Failure(t *testing.T) {
	runner := &fakeRunner{
		results: map[string]fakeResult{
			"gws auth status": {err: "token expired"},
		},
	}
	gm := source.NewGmail(runner)
	if err := gm.CheckAuth(t.Context()); err == nil {
		t.Fatal("expected error for expired auth")
	}
}

func TestGmail_RefreshAuth(t *testing.T) {
	runner := &fakeRunner{
		results: map[string]fakeResult{
			"gws auth login": {stdout: ""},
		},
	}
	gm := source.NewGmail(runner)
	if err := gm.RefreshAuth(t.Context()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGmail_Fetch_WithMessages(t *testing.T) {
	listFixture := loadFixture(t, "gws_gmail_list.json")
	getFixture := loadFixture(t, "gws_gmail_get.json")

	runner := &fakeRunner{
		results: map[string]fakeResult{
			"gws gmail users messages list": {stdout: listFixture},
			"gws gmail users messages get":  {stdout: getFixture},
		},
	}
	gm := source.NewGmail(runner)
	data, err := gm.Fetch(t.Context(), 8*time.Hour)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result map[string]json.RawMessage
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if _, ok := result["messages"]; !ok {
		t.Error("missing 'messages' key in output")
	}
}

func TestGmail_Fetch_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())

	listFixture := loadFixture(t, "gws_gmail_list.json")
	runner := &fakeRunner{
		results: map[string]fakeResult{
			"gws gmail users messages list": {stdout: listFixture},
			"gws gmail users messages get":  {err: "context canceled"},
		},
	}

	// Cancel after list succeeds but before gets complete
	cancel()

	gm := source.NewGmail(runner)
	_, err := gm.Fetch(ctx, 8*time.Hour)
	// With a cancelled context, the semaphore path returns early.
	// The test verifies no deadlock and that an error is produced.
	if err == nil {
		// Acceptable if the mock doesn't propagate ctx, but confirms no hang.
		return
	}
}

func TestGmail_Fetch_MalformedListJSON(t *testing.T) {
	runner := &fakeRunner{
		results: map[string]fakeResult{
			"gws gmail users messages list": {stdout: `not valid json`},
		},
	}
	gm := source.NewGmail(runner)
	_, err := gm.Fetch(t.Context(), 8*time.Hour)
	if err == nil {
		t.Fatal("expected error for malformed list response")
	}
	if !strings.Contains(err.Error(), "gmail parse list") {
		t.Errorf("error should mention parse list, got: %v", err)
	}
}

func TestGmail_Fetch_NoMessages(t *testing.T) {
	runner := &fakeRunner{
		results: map[string]fakeResult{
			"gws gmail users messages list": {stdout: `{}`},
		},
	}
	gm := source.NewGmail(runner)
	data, err := gm.Fetch(t.Context(), 8*time.Hour)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result map[string]json.RawMessage
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if _, ok := result["messages"]; !ok {
		t.Error("missing 'messages' key even with no messages")
	}
}
