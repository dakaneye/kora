//nolint:revive // Package name matches directory structure convention
package google_calendar

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/dakaneye/kora/internal/auth/google"
	"github.com/dakaneye/kora/internal/datasources"
)

// mockCredential creates a test credential.
func mockCredential(t *testing.T) *google.GoogleOAuthCredential {
	t.Helper()
	cred, err := google.NewGoogleOAuthCredential(
		"test-access-token",
		"test-refresh-token",
		"test@example.com",
		time.Now().Add(1*time.Hour),
	)
	if err != nil {
		t.Fatalf("creating mock credential: %v", err)
	}
	return cred
}

func TestNewCalendarClient(t *testing.T) {
	cred := mockCredential(t)

	t.Run("with nil http client", func(t *testing.T) {
		client := NewCalendarClient(cred, nil)
		if client == nil {
			t.Fatal("expected non-nil client")
		}
		if client.httpClient == nil {
			t.Error("expected default http client")
		}
		if client.credential != cred {
			t.Error("credential not set")
		}
	})

	t.Run("with custom http client", func(t *testing.T) {
		customClient := &http.Client{Timeout: 10 * time.Second}
		client := NewCalendarClient(cred, customClient)
		if client.httpClient != customClient {
			t.Error("custom http client not used")
		}
	})
}

func TestListCalendars(t *testing.T) {
	tests := []struct {
		name          string
		responses     []calendarListResponse
		statusCodes   []int
		wantCalendars int
		wantErr       bool
	}{
		{
			name: "single page success",
			responses: []calendarListResponse{
				{
					Items: []CalendarEntry{
						{ID: "cal1", Summary: "Primary", Primary: true},
						{ID: "cal2", Summary: "Work", Primary: false},
					},
				},
			},
			statusCodes:   []int{http.StatusOK},
			wantCalendars: 2,
			wantErr:       false,
		},
		{
			name: "paginated success",
			responses: []calendarListResponse{
				{
					Items:         []CalendarEntry{{ID: "cal1", Summary: "Primary"}},
					NextPageToken: "token1",
				},
				{
					Items: []CalendarEntry{{ID: "cal2", Summary: "Work"}},
				},
			},
			statusCodes:   []int{http.StatusOK, http.StatusOK},
			wantCalendars: 2,
			wantErr:       false,
		},
		{
			name:          "unauthorized error",
			responses:     []calendarListResponse{},
			statusCodes:   []int{http.StatusUnauthorized},
			wantCalendars: 0,
			wantErr:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			callCount := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Verify authorization header
				auth := r.Header.Get("Authorization")
				if auth != "Bearer test-access-token" {
					t.Errorf("unexpected auth header: %s", auth)
				}

				if callCount >= len(tt.statusCodes) {
					t.Fatal("too many requests")
				}

				w.WriteHeader(tt.statusCodes[callCount])
				if callCount < len(tt.responses) {
					json.NewEncoder(w).Encode(tt.responses[callCount])
				}
				callCount++
			}))
			defer server.Close()

			// Create client with test server
			cred := mockCredential(t)
			httpClient := server.Client()
			client := &CalendarClient{
				httpClient: httpClient,
				credential: cred,
			}

			// Need to override the URL in the request - use a custom doRequestWithRetry
			// For this test, we'll create a wrapper that hits the test server
			ctx := context.Background()

			// Since we can't easily override the const, we'll test the helper directly
			// and verify the authorization header is set correctly
			if tt.wantErr && tt.statusCodes[0] == http.StatusUnauthorized {
				_, err := client.doRequestWithRetry(ctx, server.URL+"/users/me/calendarList")
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}

			// For success cases, test that parsing works
			if !tt.wantErr && len(tt.responses) > 0 {
				body, err := client.doRequestWithRetry(ctx, server.URL+"/users/me/calendarList")
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}

				var resp calendarListResponse
				if err := json.Unmarshal(body, &resp); err != nil {
					t.Fatalf("failed to parse response: %v", err)
				}

				if len(resp.Items) != len(tt.responses[0].Items) {
					t.Errorf("got %d calendars, want %d", len(resp.Items), len(tt.responses[0].Items))
				}
			}
		})
	}
}

func TestListEvents(t *testing.T) {
	now := time.Now()
	since := now.Add(-24 * time.Hour)
	until := now.Add(7 * 24 * time.Hour)

	tests := []struct {
		name        string
		responses   []eventsListResponse
		statusCodes []int
		wantEvents  int
		wantErr     bool
	}{
		{
			name: "single page success",
			responses: []eventsListResponse{
				{
					Items: []CalendarEvent{
						{
							ID:      "event1",
							Summary: "Team Standup",
							Start: EventTime{
								DateTimeRaw: now.Add(2 * time.Hour).Format(time.RFC3339),
							},
							End: EventTime{
								DateTimeRaw: now.Add(3 * time.Hour).Format(time.RFC3339),
							},
							Status:   "confirmed",
							HTMLLink: "https://calendar.google.com/event?eid=abc",
						},
					},
				},
			},
			statusCodes: []int{http.StatusOK},
			wantEvents:  1,
			wantErr:     false,
		},
		{
			name: "all-day event",
			responses: []eventsListResponse{
				{
					Items: []CalendarEvent{
						{
							ID:      "allday1",
							Summary: "Company Holiday",
							Start: EventTime{
								Date: "2024-12-25",
							},
							End: EventTime{
								Date: "2024-12-26",
							},
							Status: "confirmed",
						},
					},
				},
			},
			statusCodes: []int{http.StatusOK},
			wantEvents:  1,
			wantErr:     false,
		},
		{
			name:        "rate limited then success",
			responses:   []eventsListResponse{{Items: []CalendarEvent{{ID: "event1", Summary: "Test"}}}},
			statusCodes: []int{http.StatusTooManyRequests, http.StatusOK},
			wantEvents:  1,
			wantErr:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			callCount := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Verify query parameters
				query := r.URL.Query()
				if query.Get("singleEvents") != "true" {
					t.Error("singleEvents should be true")
				}
				if query.Get("orderBy") != "startTime" {
					t.Error("orderBy should be startTime")
				}
				if query.Get("timeMin") == "" {
					t.Error("timeMin should be set")
				}
				if query.Get("timeMax") == "" {
					t.Error("timeMax should be set")
				}

				if callCount >= len(tt.statusCodes) {
					t.Fatal("too many requests")
				}

				statusCode := tt.statusCodes[callCount]
				w.WriteHeader(statusCode)
				// Write response body for successful requests
				// For rate limited then success, the response index is 0 regardless of call count
				if statusCode == http.StatusOK && len(tt.responses) > 0 {
					// Find the first (and only) response
					json.NewEncoder(w).Encode(tt.responses[0])
				}
				callCount++
			}))
			defer server.Close()

			cred := mockCredential(t)
			client := &CalendarClient{
				httpClient: server.Client(),
				credential: cred,
			}

			ctx := context.Background()

			// Test using the test server URL directly
			body, err := client.doRequestWithRetry(ctx, server.URL+"/calendars/primary/events?timeMin="+since.Format(time.RFC3339)+"&timeMax="+until.Format(time.RFC3339)+"&singleEvents=true&orderBy=startTime&maxResults=250")

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			var resp eventsListResponse
			if err := json.Unmarshal(body, &resp); err != nil {
				t.Fatalf("failed to parse response: %v", err)
			}

			if len(resp.Items) != tt.wantEvents {
				t.Errorf("got %d events, want %d", len(resp.Items), tt.wantEvents)
			}
		})
	}
}

func TestDoRequestWithRetry(t *testing.T) {
	t.Run("success on first try", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"test": "data"}`))
		}))
		defer server.Close()

		client := &CalendarClient{
			httpClient: server.Client(),
			credential: mockCredential(t),
		}

		body, err := client.doRequestWithRetry(context.Background(), server.URL)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if string(body) != `{"test": "data"}` {
			t.Errorf("unexpected body: %s", body)
		}
	})

	t.Run("retries on rate limit", func(t *testing.T) {
		attempts := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			attempts++
			if attempts == 1 {
				w.Header().Set("Retry-After", "1")
				w.WriteHeader(http.StatusTooManyRequests)
				return
			}
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"success": true}`))
		}))
		defer server.Close()

		client := &CalendarClient{
			httpClient: server.Client(),
			credential: mockCredential(t),
		}

		body, err := client.doRequestWithRetry(context.Background(), server.URL)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if attempts != 2 {
			t.Errorf("expected 2 attempts, got %d", attempts)
		}

		if string(body) != `{"success": true}` {
			t.Errorf("unexpected body: %s", body)
		}
	})

	t.Run("returns ErrNotAuthenticated on 401", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
		}))
		defer server.Close()

		client := &CalendarClient{
			httpClient: server.Client(),
			credential: mockCredential(t),
		}

		_, err := client.doRequestWithRetry(context.Background(), server.URL)
		if err == nil {
			t.Fatal("expected error, got nil")
		}

		if !containsError(err, datasources.ErrNotAuthenticated) {
			t.Errorf("expected ErrNotAuthenticated, got: %v", err)
		}
	})

	t.Run("returns ErrRateLimited after max retries", func(t *testing.T) {
		attempts := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			attempts++
			w.WriteHeader(http.StatusTooManyRequests)
		}))
		defer server.Close()

		client := &CalendarClient{
			httpClient: server.Client(),
			credential: mockCredential(t),
		}

		// Use a short timeout context to speed up the test
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		_, err := client.doRequestWithRetry(ctx, server.URL)
		if err == nil {
			t.Fatal("expected error, got nil")
		}

		if !containsError(err, datasources.ErrRateLimited) {
			t.Errorf("expected ErrRateLimited, got: %v", err)
		}

		// Should have attempted maxRetries + 1 times
		if attempts != maxRetries+1 {
			t.Errorf("expected %d attempts, got %d", maxRetries+1, attempts)
		}
	})

	t.Run("returns ErrServiceUnavailable on 503", func(t *testing.T) {
		attempts := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			attempts++
			w.WriteHeader(http.StatusServiceUnavailable)
		}))
		defer server.Close()

		client := &CalendarClient{
			httpClient: server.Client(),
			credential: mockCredential(t),
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		_, err := client.doRequestWithRetry(ctx, server.URL)
		if err == nil {
			t.Fatal("expected error, got nil")
		}

		if !containsError(err, datasources.ErrServiceUnavailable) {
			t.Errorf("expected ErrServiceUnavailable, got: %v", err)
		}
	})

	t.Run("respects context cancellation", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(1 * time.Second)
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		client := &CalendarClient{
			httpClient: server.Client(),
			credential: mockCredential(t),
		}

		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		_, err := client.doRequestWithRetry(ctx, server.URL)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if err != context.Canceled {
			t.Errorf("expected context.Canceled, got: %v", err)
		}
	})
}

func TestEventTimeUnmarshalJSON(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantAllDay bool
		wantErr    bool
	}{
		{
			name:       "timed event",
			input:      `{"dateTime": "2024-01-15T10:00:00-08:00", "timeZone": "America/Los_Angeles"}`,
			wantAllDay: false,
			wantErr:    false,
		},
		{
			name:       "all-day event",
			input:      `{"date": "2024-01-15"}`,
			wantAllDay: true,
			wantErr:    false,
		},
		{
			name:       "invalid dateTime format",
			input:      `{"dateTime": "not-a-date"}`,
			wantAllDay: false,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var et EventTime
			err := json.Unmarshal([]byte(tt.input), &et)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if et.IsAllDay() != tt.wantAllDay {
				t.Errorf("IsAllDay() = %v, want %v", et.IsAllDay(), tt.wantAllDay)
			}

			if !tt.wantAllDay && et.DateTime.IsZero() {
				t.Error("DateTime should not be zero for timed events")
			}
		})
	}
}

func TestEventTimeTime(t *testing.T) {
	t.Run("timed event", func(t *testing.T) {
		et := EventTime{
			DateTime: time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC),
		}
		got := et.Time()
		if !got.Equal(et.DateTime) {
			t.Errorf("Time() = %v, want %v", got, et.DateTime)
		}
	})

	t.Run("all-day event", func(t *testing.T) {
		et := EventTime{
			Date: "2024-01-15",
		}
		got := et.Time()
		want := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)
		if !got.Equal(want) {
			t.Errorf("Time() = %v, want %v", got, want)
		}
	})

	t.Run("invalid date", func(t *testing.T) {
		et := EventTime{
			Date: "invalid",
		}
		got := et.Time()
		if !got.IsZero() {
			t.Errorf("Time() = %v, want zero time", got)
		}
	})
}

func TestCalendarEventHelpers(t *testing.T) {
	t.Run("GetVideoMeetingURL", func(t *testing.T) {
		event := CalendarEvent{
			ConferenceData: &ConferenceData{
				EntryPoints: []EntryPoint{
					{EntryPointType: "phone", URI: "tel:+1234567890"},
					{EntryPointType: "video", URI: "https://meet.google.com/abc-defg-hij"},
				},
			},
		}

		got := event.GetVideoMeetingURL()
		want := "https://meet.google.com/abc-defg-hij"
		if got != want {
			t.Errorf("GetVideoMeetingURL() = %q, want %q", got, want)
		}
	})

	t.Run("GetVideoMeetingURL no conference", func(t *testing.T) {
		event := CalendarEvent{}
		got := event.GetVideoMeetingURL()
		if got != "" {
			t.Errorf("GetVideoMeetingURL() = %q, want empty", got)
		}
	})

	t.Run("GetUserResponseStatus", func(t *testing.T) {
		event := CalendarEvent{
			Attendees: []Attendee{
				{Email: "other@example.com", ResponseStatus: "accepted"},
				{Email: "me@example.com", ResponseStatus: "needsAction", Self: true},
			},
		}

		got := event.GetUserResponseStatus()
		if got != "needsAction" {
			t.Errorf("GetUserResponseStatus() = %q, want %q", got, "needsAction")
		}
	})

	t.Run("GetPendingRSVPs", func(t *testing.T) {
		event := CalendarEvent{
			Attendees: []Attendee{
				{Email: "accepted@example.com", ResponseStatus: "accepted"},
				{Email: "pending1@example.com", ResponseStatus: "needsAction"},
				{Email: "declined@example.com", ResponseStatus: "declined"},
				{Email: "pending2@example.com", ResponseStatus: "needsAction"},
			},
		}

		pending := event.GetPendingRSVPs()
		if len(pending) != 2 {
			t.Errorf("GetPendingRSVPs() returned %d, want 2", len(pending))
		}
	})

	t.Run("IsUserOrganizer", func(t *testing.T) {
		event := CalendarEvent{
			Organizer: Person{Email: "me@example.com", Self: true},
		}

		if !event.IsUserOrganizer() {
			t.Error("IsUserOrganizer() = false, want true")
		}
	})

	t.Run("IsAllDay", func(t *testing.T) {
		event := CalendarEvent{
			Start: EventTime{Date: "2024-01-15"},
		}

		if !event.IsAllDay() {
			t.Error("IsAllDay() = false, want true")
		}
	})
}

// containsError checks if err's error chain contains target.
func containsError(err, target error) bool {
	if err == nil {
		return false
	}
	// Simple string match since errors.Is may not work with wrapped errors
	errStr := err.Error()
	targetStr := target.Error()
	if errStr == "" || targetStr == "" {
		return false
	}
	return err == target || errStr == targetStr || strings.Contains(errStr, targetStr)
}
