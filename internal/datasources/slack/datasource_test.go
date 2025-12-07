package slack

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/dakaneye/kora/internal/auth"
	"github.com/dakaneye/kora/internal/datasources"
	"github.com/dakaneye/kora/internal/models"
)

func TestNewDataSource(t *testing.T) {
	tests := []struct {
		name        string
		authService auth.Service
		wantErr     bool
	}{
		{
			name:        "valid slack auth provider",
			authService: auth.ServiceSlack,
			wantErr:     false,
		},
		{
			name:        "wrong auth provider type",
			authService: auth.ServiceGitHub,
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := &mockAuthProvider{service: tt.authService, authenticated: true}
			_, err := NewDataSource(provider)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewDataSource() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestDataSource_Name(t *testing.T) {
	provider := newMockAuthProvider()
	ds, err := NewDataSource(provider)
	if err != nil {
		t.Fatalf("NewDataSource() error = %v", err)
	}

	if got := ds.Name(); got != "slack" {
		t.Errorf("Name() = %q, want %q", got, "slack")
	}
}

func TestDataSource_Service(t *testing.T) {
	provider := newMockAuthProvider()
	ds, err := NewDataSource(provider)
	if err != nil {
		t.Fatalf("NewDataSource() error = %v", err)
	}

	if got := ds.Service(); got != models.SourceSlack {
		t.Errorf("Service() = %v, want %v", got, models.SourceSlack)
	}
}

func TestDataSource_Fetch_AuthError(t *testing.T) {
	provider := newMockAuthProvider()
	provider.authenticated = false

	ds, err := NewDataSource(provider)
	if err != nil {
		t.Fatalf("NewDataSource() error = %v", err)
	}

	ctx := context.Background()
	opts := datasources.FetchOptions{Since: time.Now().Add(-time.Hour)}

	_, err = ds.Fetch(ctx, opts)
	if err == nil {
		t.Error("Fetch() should return error when not authenticated")
	}
}

func TestDataSource_Fetch_InvalidOptions(t *testing.T) {
	provider := newMockAuthProvider()
	ds, err := NewDataSource(provider)
	if err != nil {
		t.Fatalf("NewDataSource() error = %v", err)
	}

	ctx := context.Background()
	opts := datasources.FetchOptions{Since: time.Time{}} // Zero time = invalid

	_, err = ds.Fetch(ctx, opts)
	if err == nil {
		t.Error("Fetch() should return error for invalid options")
	}
}

func TestDataSource_Fetch_Success(t *testing.T) {
	// Setup mock server
	authTestCalled := false
	searchCalled := false
	conversationsCalled := false

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify Authorization header is set (but don't log the actual value)
		if r.Header.Get("Authorization") == "" {
			t.Error("Authorization header not set")
		}

		var resp any
		switch r.URL.Path {
		case "/auth.test":
			authTestCalled = true
			resp = slackAuthTestResponse{
				OK:     true,
				UserID: "U12345678",
				User:   "testuser",
				TeamID: "T12345678",
				Team:   "testteam",
			}
		case "/search.messages":
			searchCalled = true
			resp = slackSearchResponse{
				OK: true,
				Messages: struct {
					Matches []slackMessage `json:"matches"`
				}{
					Matches: []slackMessage{
						{
							TS:        "1733500000.000001",
							Text:      "Hello <@U12345678>",
							User:      "U99999999",
							Username:  "sender",
							Permalink: "https://test.slack.com/archives/C123/p1733500000000001",
							Channel: struct {
								ID   string `json:"id"`
								Name string `json:"name"`
							}{
								ID:   "C12345678",
								Name: "general",
							},
						},
					},
				},
			}
		case "/users.conversations":
			conversationsCalled = true
			resp = slackConversationsResponse{
				OK: true,
				Channels: []slackChannel{
					{ID: "D12345678", Name: "", User: "U99999999"},
				},
			}
		case "/conversations.history":
			resp = slackHistoryResponse{
				OK: true,
				Messages: []slackMessage{
					{
						TS:   "1733500000.000002",
						Text: "Hey, are you there?",
						User: "U99999999",
					},
				},
			}
		default:
			t.Errorf("Unexpected API call: %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			t.Errorf("Failed to encode response: %v", err)
		}
	}))
	defer server.Close()

	provider := newMockAuthProvider()
	ds, err := NewDataSource(provider, WithBaseURL(server.URL))
	if err != nil {
		t.Fatalf("NewDataSource() error = %v", err)
	}

	ctx := context.Background()
	// Use a time before our mock timestamps
	since := time.Unix(1733400000, 0)
	opts := datasources.FetchOptions{Since: since}

	result, err := ds.Fetch(ctx, opts)
	if err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}

	// Verify API calls were made
	if !authTestCalled {
		t.Error("auth.test was not called")
	}
	if !searchCalled {
		t.Error("search.messages was not called")
	}
	if !conversationsCalled {
		t.Error("users.conversations was not called")
	}

	// Verify we got events
	if len(result.Events) == 0 {
		t.Error("Expected events, got none")
	}

	// Verify event validation
	for i, event := range result.Events {
		if err := event.Validate(); err != nil {
			t.Errorf("Event %d failed validation: %v", i, err)
		}
	}

	// Check we have both mention and DM events
	hasMention := false
	hasDM := false
	for _, e := range result.Events {
		if e.Type == models.EventTypeSlackMention {
			hasMention = true
		}
		if e.Type == models.EventTypeSlackDM {
			hasDM = true
		}
	}
	if !hasMention {
		t.Error("Expected at least one mention event")
	}
	if !hasDM {
		t.Error("Expected at least one DM event")
	}
}

func TestDataSource_Fetch_RateLimited(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/auth.test" {
			resp := slackAuthTestResponse{OK: true, UserID: "U12345678"}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(resp) //nolint:errcheck // test helper // test helper
			return
		}
		// Return 429 for all other requests
		w.Header().Set("Retry-After", "30")
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()

	provider := newMockAuthProvider()
	ds, err := NewDataSource(provider, WithBaseURL(server.URL))
	if err != nil {
		t.Fatalf("NewDataSource() error = %v", err)
	}

	ctx := context.Background()
	opts := datasources.FetchOptions{Since: time.Now().Add(-time.Hour)}

	result, err := ds.Fetch(ctx, opts)
	// Should return partial result even with rate limiting
	// err may be nil (partial success) or non-nil (complete failure)
	_ = err
	if result == nil {
		t.Fatal("Expected result even with rate limiting errors")
	}
	if len(result.Errors) == 0 {
		t.Error("Expected errors in result due to rate limiting")
	}
}

func TestDataSource_Fetch_SkipsOwnMessages(t *testing.T) {
	myUserID := "U12345678"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var resp any
		switch r.URL.Path {
		case "/auth.test":
			resp = slackAuthTestResponse{OK: true, UserID: myUserID}
		case "/search.messages":
			resp = slackSearchResponse{OK: true}
		case "/users.conversations":
			resp = slackConversationsResponse{
				OK:       true,
				Channels: []slackChannel{{ID: "D123", User: "U999"}},
			}
		case "/conversations.history":
			resp = slackHistoryResponse{
				OK: true,
				Messages: []slackMessage{
					{TS: "1733500000.000001", Text: "My own message", User: myUserID},
					{TS: "1733500000.000002", Text: "Someone else's message", User: "U999"},
				},
			}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp) //nolint:errcheck // test helper
	}))
	defer server.Close()

	provider := newMockAuthProvider()
	ds, err := NewDataSource(provider, WithBaseURL(server.URL))
	if err != nil {
		t.Fatalf("NewDataSource() error = %v", err)
	}

	ctx := context.Background()
	since := time.Unix(1733400000, 0)
	opts := datasources.FetchOptions{Since: since}

	result, err := ds.Fetch(ctx, opts)
	if err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}

	// Should only have one DM event (the one from someone else)
	dmCount := 0
	for _, e := range result.Events {
		if e.Type == models.EventTypeSlackDM {
			dmCount++
			// Verify it's not our own message
			if e.Author.Username == myUserID {
				t.Error("Own messages should be skipped")
			}
		}
	}
	if dmCount != 1 {
		t.Errorf("Expected 1 DM event (from others), got %d", dmCount)
	}
}

func TestDataSource_Fetch_ContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate slow response
		time.Sleep(100 * time.Millisecond)
		resp := slackAuthTestResponse{OK: true, UserID: "U12345678"}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp) //nolint:errcheck // test helper
	}))
	defer server.Close()

	provider := newMockAuthProvider()
	ds, err := NewDataSource(provider, WithBaseURL(server.URL))
	if err != nil {
		t.Fatalf("NewDataSource() error = %v", err)
	}

	// Create a context that's already canceled
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	opts := datasources.FetchOptions{Since: time.Now().Add(-time.Hour)}

	_, err = ds.Fetch(ctx, opts)
	if err == nil {
		t.Error("Expected error with canceled context")
	}
}

func TestDataSource_EventValidation(t *testing.T) {
	// Test that all events pass EFA 0001 validation
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var resp any
		switch r.URL.Path {
		case "/auth.test":
			resp = slackAuthTestResponse{OK: true, UserID: "U12345678"}
		case "/search.messages":
			resp = slackSearchResponse{
				OK: true,
				Messages: struct {
					Matches []slackMessage `json:"matches"`
				}{
					Matches: []slackMessage{
						{
							TS:        "1733500000.000001",
							Text:      "Test mention",
							User:      "U999",
							Permalink: "https://test.slack.com/archives/C123/p1",
							Channel: struct {
								ID   string `json:"id"`
								Name string `json:"name"`
							}{Name: "general"},
						},
					},
				},
			}
		case "/users.conversations":
			resp = slackConversationsResponse{
				OK:       true,
				Channels: []slackChannel{{ID: "D123"}},
			}
		case "/conversations.history":
			resp = slackHistoryResponse{
				OK: true,
				Messages: []slackMessage{
					{TS: "1733500000.000002", Text: "Test DM", User: "U999"},
				},
			}
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp) //nolint:errcheck // test helper
	}))
	defer server.Close()

	provider := newMockAuthProvider()
	ds, err := NewDataSource(provider, WithBaseURL(server.URL))
	if err != nil {
		t.Fatalf("NewDataSource() error = %v", err)
	}

	ctx := context.Background()
	since := time.Unix(1733400000, 0)
	opts := datasources.FetchOptions{Since: since}

	result, err := ds.Fetch(ctx, opts)
	if err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}

	for i, event := range result.Events {
		// Verify required fields per EFA 0001
		if !event.Type.IsValid() {
			t.Errorf("Event %d has invalid type: %s", i, event.Type)
		}
		if event.Title == "" || len(event.Title) > 100 {
			t.Errorf("Event %d title invalid: %q (len=%d)", i, event.Title, len(event.Title))
		}
		if event.Source != models.SourceSlack {
			t.Errorf("Event %d has wrong source: %s", i, event.Source)
		}
		if event.Author.Username == "" {
			t.Errorf("Event %d missing author username", i)
		}
		if event.Timestamp.IsZero() {
			t.Errorf("Event %d has zero timestamp", i)
		}
		if !event.Priority.IsValid() {
			t.Errorf("Event %d has invalid priority: %d", i, event.Priority)
		}

		// Verify metadata keys are allowed per EFA 0001
		allowedKeys := map[string]bool{
			"workspace":       true,
			"channel":         true,
			"thread_ts":       true,
			"is_thread_reply": true,
		}
		for k := range event.Metadata {
			if !allowedKeys[k] {
				t.Errorf("Event %d has disallowed metadata key: %s", i, k)
			}
		}

		// Verify priority rules per EFA 0001
		switch event.Type {
		case models.EventTypeSlackDM:
			if event.Priority != models.PriorityHigh {
				t.Errorf("DM event has wrong priority: %d (want %d)", event.Priority, models.PriorityHigh)
			}
			if !event.RequiresAction {
				t.Error("DM event should have RequiresAction=true")
			}
		case models.EventTypeSlackMention:
			if event.Priority != models.PriorityMedium {
				t.Errorf("Mention event has wrong priority: %d (want %d)", event.Priority, models.PriorityMedium)
			}
		}
	}
}
