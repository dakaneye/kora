package source_test

import (
	"encoding/json"
	"strings"
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

func TestCalendar_Fetch_MalformedJSON(t *testing.T) {
	runner := &fakeRunner{
		results: map[string]fakeResult{
			"gws calendar events list": {stdout: `{not json`},
		},
	}
	cal := source.NewCalendar(runner)
	_, err := cal.Fetch(t.Context(), 8*time.Hour)
	if err == nil {
		t.Fatal("expected error for malformed events response")
	}
	if !strings.Contains(err.Error(), "calendar parse") {
		t.Errorf("error should mention calendar parse, got: %v", err)
	}
}

func TestCalendar_Fetch_EmptyEvents(t *testing.T) {
	emptyEvents := `{"kind":"calendar#events","summary":"primary","items":[]}`

	runner := &fakeRunner{
		results: map[string]fakeResult{
			"gws calendar events list": {stdout: emptyEvents},
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
		t.Error("missing 'events' key even with empty items")
	}
}

func TestCalendar_Fetch_WithFixture(t *testing.T) {
	fixture := loadFixture(t, "gws_calendar_events.json")
	runner := &fakeRunner{
		results: map[string]fakeResult{
			"gws calendar events list": {stdout: fixture},
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
