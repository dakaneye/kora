package slack

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAPIRequest_ResponseSizeLimit(t *testing.T) {
	// Test that response size limit is enforced
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Write more than maxResponseSize (10MB)
		w.Header().Set("Content-Type", "application/json")
		// Write 11MB of data
		data := strings.Repeat("a", 11*1024*1024)
		w.Write([]byte(data)) //nolint:errcheck // test helper
	}))
	defer server.Close()

	provider := newMockAuthProvider()
	ds, err := NewDataSource(provider, WithBaseURL(server.URL))
	if err != nil {
		t.Fatalf("NewDataSource() error = %v", err)
	}

	ctx := context.Background()
	cred, _ := provider.GetCredential(ctx)
	_, err = ds.apiRequest(ctx, cred.Value(), "test", nil)
	if err == nil {
		t.Error("Expected error for oversized response")
	}
	if !strings.Contains(err.Error(), "too large") {
		t.Errorf("Expected 'too large' error, got: %v", err)
	}
}

func TestAPIRequest_InvalidJSON(t *testing.T) {
	// Test handling of invalid JSON response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("{invalid json")) //nolint:errcheck // test helper
	}))
	defer server.Close()

	provider := newMockAuthProvider()
	ds, err := NewDataSource(provider, WithBaseURL(server.URL))
	if err != nil {
		t.Fatalf("NewDataSource() error = %v", err)
	}

	ctx := context.Background()
	cred, _ := provider.GetCredential(ctx)
	// This tests that apiRequest returns the raw body, letting the caller handle JSON parsing
	body, err := ds.apiRequest(ctx, cred.Value(), "test", nil)
	if err != nil {
		t.Errorf("apiRequest() should return body even if JSON is invalid: %v", err)
	}
	if len(body) == 0 {
		t.Error("Expected body to be returned")
	}
}

func TestAPIRequest_NonOKStatus(t *testing.T) {
	//nolint:govet // test struct field order prioritizes readability
	tests := []struct {
		name       string
		statusCode int
		wantErr    string
	}{
		{
			name:       "bad request",
			statusCode: http.StatusBadRequest,
			wantErr:    "status 400",
		},
		{
			name:       "unauthorized",
			statusCode: http.StatusUnauthorized,
			wantErr:    "status 401",
		},
		{
			name:       "internal server error",
			statusCode: http.StatusInternalServerError,
			wantErr:    "status 500",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
			}))
			defer server.Close()

			provider := newMockAuthProvider()
			ds, err := NewDataSource(provider, WithBaseURL(server.URL))
			if err != nil {
				t.Fatalf("NewDataSource() error = %v", err)
			}

			ctx := context.Background()
			cred, _ := provider.GetCredential(ctx)
			_, err = ds.apiRequest(ctx, cred.Value(), "test", nil)
			if err == nil {
				t.Error("Expected error for non-OK status")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("Expected error containing %q, got: %v", tt.wantErr, err)
			}
		})
	}
}

func TestAPIRequest_RetryAfterHeader(t *testing.T) {
	// Test that Retry-After header is included in rate limit error
	retryAfterValues := []string{"30", "60", "120"}

	for _, retryAfter := range retryAfterValues {
		t.Run(fmt.Sprintf("retry_after_%s", retryAfter), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Retry-After", retryAfter)
				w.WriteHeader(http.StatusTooManyRequests)
			}))
			defer server.Close()

			provider := newMockAuthProvider()
			ds, err := NewDataSource(provider, WithBaseURL(server.URL))
			if err != nil {
				t.Fatalf("NewDataSource() error = %v", err)
			}

			ctx := context.Background()
			cred, _ := provider.GetCredential(ctx)
			_, err = ds.apiRequest(ctx, cred.Value(), "test", nil)
			if err == nil {
				t.Error("Expected error for rate limiting")
			}
			if !strings.Contains(err.Error(), retryAfter) {
				t.Errorf("Expected error to contain retry-after value %q, got: %v", retryAfter, err)
			}
			if !strings.Contains(err.Error(), "rate limited") {
				t.Errorf("Expected rate limited error, got: %v", err)
			}
		})
	}
}

func TestAPIRequest_ContextCancellation(t *testing.T) {
	// Test that context cancellation is respected
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify context is passed correctly
		select {
		case <-r.Context().Done():
			// Context was canceled
			return
		default:
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"ok": true}`)) //nolint:errcheck // test helper
		}
	}))
	defer server.Close()

	provider := newMockAuthProvider()
	ds, err := NewDataSource(provider, WithBaseURL(server.URL))
	if err != nil {
		t.Fatalf("NewDataSource() error = %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	cred, _ := provider.GetCredential(context.Background())
	_, err = ds.apiRequest(ctx, cred.Value(), "test", nil)
	if err == nil {
		t.Error("Expected error with canceled context")
	}
}

func TestAPIRequest_EmptyToken(t *testing.T) {
	// Test that empty token results in 401 from server
	// This simulates a server that validates tokens
	authHeaderSeen := ""
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Record Authorization header for debugging
		authHeaderSeen = r.Header.Get("Authorization")

		// Server checks for valid token format
		// Empty string creates "Bearer " which the server sees as just "Bearer"
		if authHeaderSeen == "Bearer " || len(authHeaderSeen) <= 7 {
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"ok": false, "error": "invalid_auth"}`)) //nolint:errcheck // test helper
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"ok": true}`)) //nolint:errcheck // test helper
	}))
	defer server.Close()

	provider := newMockAuthProvider()
	ds, err := NewDataSource(provider, WithBaseURL(server.URL))
	if err != nil {
		t.Fatalf("NewDataSource() error = %v", err)
	}

	ctx := context.Background()
	// Empty token will get "Bearer " header
	_, err = ds.apiRequest(ctx, "", "test", nil)
	if err == nil {
		t.Fatalf("Expected error with empty token, got nil (auth header was: %q)", authHeaderSeen)
	}
	if !strings.Contains(err.Error(), "401") {
		t.Errorf("Expected 401 error, got: %v (auth header was: %q)", err, authHeaderSeen)
	}
}
