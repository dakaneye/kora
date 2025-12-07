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

func TestDataSource_Fetch_MalformedTimestamps(t *testing.T) {
	// Test that malformed timestamps are skipped gracefully
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
							TS:        "invalid-timestamp",
							Text:      "Message with invalid timestamp",
							User:      "U999",
							Permalink: "https://test.slack.com/archives/C123/p1",
						},
						{
							TS:        "1733500000.000001",
							Text:      "Valid message",
							User:      "U999",
							Permalink: "https://test.slack.com/archives/C123/p2",
						},
					},
				},
			}
		case "/users.conversations":
			resp = slackConversationsResponse{OK: true}
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

	// Should have skipped the invalid timestamp message
	if len(result.Events) != 1 {
		t.Errorf("Expected 1 event (invalid timestamp skipped), got %d", len(result.Events))
	}
}

func TestDataSource_Fetch_MissingAuthor(t *testing.T) {
	// Test that messages with missing author use placeholder
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
							Text:      "Message with no author",
							User:      "", // No user
							Username:  "", // No username
							Permalink: "https://test.slack.com/archives/C123/p1",
						},
					},
				},
			}
		case "/users.conversations":
			resp = slackConversationsResponse{OK: true}
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

	if len(result.Events) == 0 {
		t.Fatal("Expected at least one event")
	}

	event := result.Events[0]
	if event.Author.Username == "" {
		t.Error("Expected placeholder username for missing author")
	}
}

func TestDataSource_Fetch_ThreadReplies(t *testing.T) {
	// Test that thread replies are correctly identified
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
							ThreadTS:  "1733500000.000001", // Same = parent message
							Text:      "Parent message",
							User:      "U999",
							Permalink: "https://test.slack.com/archives/C123/p1",
						},
						{
							TS:        "1733500000.000002",
							ThreadTS:  "1733500000.000001", // Different from TS means this is a reply
							Text:      "Thread reply",
							User:      "U999",
							Permalink: "https://test.slack.com/archives/C123/p2",
						},
					},
				},
			}
		case "/users.conversations":
			resp = slackConversationsResponse{OK: true}
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

	if len(result.Events) != 2 {
		t.Fatalf("Expected 2 events, got %d", len(result.Events))
	}

	// First event (parent) should not be a thread reply
	if result.Events[0].Metadata["is_thread_reply"].(bool) {
		t.Error("Parent message should not be marked as thread reply")
	}

	// Second event (reply) should be a thread reply
	if !result.Events[1].Metadata["is_thread_reply"].(bool) {
		t.Error("Thread reply should be marked as thread reply")
	}

	// Both should have thread_ts metadata
	if result.Events[0].Metadata["thread_ts"] == "" {
		t.Error("Parent message should have thread_ts metadata")
	}
	if result.Events[1].Metadata["thread_ts"] == "" {
		t.Error("Thread reply should have thread_ts metadata")
	}
}

func TestDataSource_Fetch_EmptyToken(t *testing.T) {
	// Test handling of empty token from auth provider
	provider := newMockAuthProvider()
	// Create a mock with empty token
	provider.credential = &mockCredential{value: "", credType: "token"}

	ds, err := NewDataSource(provider)
	if err != nil {
		t.Fatalf("NewDataSource() error = %v", err)
	}

	ctx := context.Background()
	opts := datasources.FetchOptions{Since: time.Now().Add(-time.Hour)}

	_, err = ds.Fetch(ctx, opts)
	if err == nil {
		t.Error("Expected error with empty token")
	}
}

func TestDataSource_Fetch_UserIDCaching(t *testing.T) {
	// Test that user ID is cached after first fetch
	authTestCallCount := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var resp any
		switch r.URL.Path {
		case "/auth.test":
			authTestCallCount++
			resp = slackAuthTestResponse{OK: true, UserID: "U12345678"}
		case "/search.messages":
			resp = slackSearchResponse{OK: true}
		case "/users.conversations":
			resp = slackConversationsResponse{OK: true}
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
	opts := datasources.FetchOptions{Since: time.Now().Add(-time.Hour)}

	// First fetch
	_, err = ds.Fetch(ctx, opts)
	if err != nil {
		t.Fatalf("First Fetch() error = %v", err)
	}

	if authTestCallCount != 1 {
		t.Errorf("Expected 1 auth.test call, got %d", authTestCallCount)
	}

	// Second fetch - should use cached user ID
	_, err = ds.Fetch(ctx, opts)
	if err != nil {
		t.Fatalf("Second Fetch() error = %v", err)
	}

	if authTestCallCount != 1 {
		t.Errorf("Expected still 1 auth.test call (cached), got %d", authTestCallCount)
	}
}

func TestDataSource_Fetch_WorkspaceMetadata(t *testing.T) {
	// Test that workspace metadata is extracted correctly
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
							Text:      "Test message",
							User:      "U999",
							Permalink: "https://myworkspace.slack.com/archives/C123/p1",
							Channel: struct {
								ID   string `json:"id"`
								Name string `json:"name"`
							}{
								Name: "general",
							},
						},
					},
				},
			}
		case "/users.conversations":
			resp = slackConversationsResponse{OK: true}
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

	if len(result.Events) == 0 {
		t.Fatal("Expected at least one event")
	}

	event := result.Events[0]
	workspace, ok := event.Metadata["workspace"]
	if !ok {
		t.Error("Expected workspace metadata")
	}
	if workspace != "myworkspace" {
		t.Errorf("Expected workspace='myworkspace', got %q", workspace)
	}

	channel, ok := event.Metadata["channel"]
	if !ok {
		t.Error("Expected channel metadata")
	}
	if channel != "general" {
		t.Errorf("Expected channel='general', got %q", channel)
	}
}

func TestDataSource_Fetch_Limit(t *testing.T) {
	// Test that FetchOptions.Limit is respected
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var resp any
		switch r.URL.Path {
		case "/auth.test":
			resp = slackAuthTestResponse{OK: true, UserID: "U12345678"}
		case "/search.messages":
			// Return 5 messages
			matches := make([]slackMessage, 5)
			for i := 0; i < 5; i++ {
				matches[i] = slackMessage{
					TS:        "1733500000.00000" + string('1'+rune(i)),
					Text:      "Message",
					User:      "U999",
					Permalink: "https://test.slack.com/archives/C123/p" + string('1'+rune(i)),
				}
			}
			resp = slackSearchResponse{
				OK: true,
				Messages: struct {
					Matches []slackMessage `json:"matches"`
				}{
					Matches: matches,
				},
			}
		case "/users.conversations":
			resp = slackConversationsResponse{OK: true}
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
	opts := datasources.FetchOptions{
		Since: since,
		Limit: 3, // Limit to 3 events
	}

	result, err := ds.Fetch(ctx, opts)
	if err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}

	if len(result.Events) != 3 {
		t.Errorf("Expected 3 events (limited), got %d", len(result.Events))
	}
}

func TestDataSource_Fetch_DMsPartialFailure(t *testing.T) {
	// Test partial success when some DM channels fail to fetch
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var resp any
		switch r.URL.Path {
		case "/auth.test":
			resp = slackAuthTestResponse{OK: true, UserID: "U12345678"}
		case "/search.messages":
			resp = slackSearchResponse{OK: true}
		case "/users.conversations":
			resp = slackConversationsResponse{
				OK: true,
				Channels: []slackChannel{
					{ID: "D123", User: "U999"},
					{ID: "D456", User: "U888"},
				},
			}
		case "/conversations.history":
			// Fail for first channel, succeed for second
			channelID := r.URL.Query().Get("channel")
			if channelID == "D123" {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			resp = slackHistoryResponse{
				OK: true,
				Messages: []slackMessage{
					{TS: "1733500000.000001", Text: "Success message", User: "U888"},
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
		t.Fatalf("Fetch() should succeed with partial results: %v", err)
	}

	// Should have events from the successful channel
	if len(result.Events) == 0 {
		t.Error("Expected at least one event from successful channel")
	}

	// Should have DM event
	hasDM := false
	for _, e := range result.Events {
		if e.Type == models.EventTypeSlackDM {
			hasDM = true
			break
		}
	}
	if !hasDM {
		t.Error("Expected at least one DM event")
	}
}

// mockCredential implements auth.Credential for testing
type mockCredential struct {
	value    string
	credType string
}

func (m *mockCredential) Type() auth.CredentialType {
	return auth.CredentialType(m.credType)
}

func (m *mockCredential) Value() string {
	return m.value
}

func (m *mockCredential) Redacted() string {
	if m.value == "" {
		return "<empty>"
	}
	return "mock-token-***"
}

func (m *mockCredential) IsValid() bool {
	return m.value != ""
}
