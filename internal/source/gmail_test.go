package source_test

import (
	"encoding/json"
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
	listResponse := `{"messages":[{"id":"msg1"},{"id":"msg2"}]}`
	msg1 := `{"id":"msg1","payload":{"headers":[{"name":"From","value":"alice@example.com"},{"name":"Subject","value":"Hello"}]}}`
	msg2 := `{"id":"msg2","payload":{"headers":[{"name":"From","value":"bob@example.com"},{"name":"Subject","value":"Meeting"}]}}`
	// fakeRunner prefix matching will match both get calls to the same result.
	// For this test, that's acceptable — we're verifying structure, not per-message content.
	_ = msg2

	runner := &fakeRunner{
		results: map[string]fakeResult{
			"gws gmail users messages list": {stdout: listResponse},
			"gws gmail users messages get":  {stdout: msg1},
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
