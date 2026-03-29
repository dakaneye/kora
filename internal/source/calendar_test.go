package source_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/dakaneye/kora/internal/source"
)

func TestCalendar_Name(t *testing.T) {
	cal := source.NewCalendar(nil)
	if cal.Name() != "calendar" {
		t.Errorf("name = %q, want %q", cal.Name(), "calendar")
	}
}

func TestCalendar_CheckAuth_Success(t *testing.T) {
	runner := &fakeRunner{
		results: map[string]fakeResult{
			"gws auth status": {stdout: `{"token_valid": true}`},
		},
	}
	cal := source.NewCalendar(runner)
	if err := cal.CheckAuth(t.Context()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCalendar_CheckAuth_Failure(t *testing.T) {
	runner := &fakeRunner{
		results: map[string]fakeResult{
			"gws auth status": {err: "token expired"},
		},
	}
	cal := source.NewCalendar(runner)
	if err := cal.CheckAuth(t.Context()); err == nil {
		t.Fatal("expected error")
	}
}

func TestCalendar_RefreshAuth(t *testing.T) {
	runner := &fakeRunner{
		results: map[string]fakeResult{
			"gws auth login": {stdout: ""},
		},
	}
	cal := source.NewCalendar(runner)
	if err := cal.RefreshAuth(t.Context()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCalendar_Fetch(t *testing.T) {
	eventsJSON := `{"items":[{"summary":"Standup","start":{"dateTime":"2026-03-29T09:00:00Z"}}]}`
	runner := &fakeRunner{
		results: map[string]fakeResult{
			"gws calendar events list": {stdout: eventsJSON},
		},
	}
	cal := source.NewCalendar(runner)
	data, err := cal.Fetch(t.Context(), 8*time.Hour)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result map[string]json.RawMessage
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if _, ok := result["events"]; !ok {
		t.Error("missing 'events' key in output")
	}
}
