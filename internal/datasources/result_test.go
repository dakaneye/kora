package datasources

import (
	"errors"
	"testing"
	"time"

	"github.com/dakaneye/kora/internal/models"
)

func TestFetchResult_HasEvents(t *testing.T) {
	tests := []struct {
		name   string
		result FetchResult
		want   bool
	}{
		{
			name:   "no events",
			result: FetchResult{Events: nil},
			want:   false,
		},
		{
			name:   "empty events slice",
			result: FetchResult{Events: []models.Event{}},
			want:   false,
		},
		{
			name: "has events",
			result: FetchResult{
				Events: []models.Event{{Title: "test"}},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.result.HasEvents(); got != tt.want {
				t.Errorf("HasEvents() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFetchResult_HasErrors(t *testing.T) {
	tests := []struct {
		name   string
		result FetchResult
		want   bool
	}{
		{
			name:   "no errors",
			result: FetchResult{Errors: nil},
			want:   false,
		},
		{
			name:   "empty errors slice",
			result: FetchResult{Errors: []error{}},
			want:   false,
		},
		{
			name: "has errors",
			result: FetchResult{
				Errors: []error{errors.New("test error")},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.result.HasErrors(); got != tt.want {
				t.Errorf("HasErrors() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFetchResult_CombinedError(t *testing.T) {
	//nolint:govet // Test struct field order for readability
	tests := []struct {
		name       string
		result     FetchResult
		wantNil    bool
		wantErrStr string
	}{
		{
			name:    "no errors returns nil",
			result:  FetchResult{Errors: nil},
			wantNil: true,
		},
		{
			name:    "empty errors returns nil",
			result:  FetchResult{Errors: []error{}},
			wantNil: true,
		},
		{
			name: "single error",
			result: FetchResult{
				Errors: []error{errors.New("first error")},
			},
			wantNil:    false,
			wantErrStr: "first error",
		},
		{
			name: "multiple errors joined",
			result: FetchResult{
				Errors: []error{
					errors.New("first error"),
					errors.New("second error"),
				},
			},
			wantNil:    false,
			wantErrStr: "first error\nsecond error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.result.CombinedError()
			if tt.wantNil {
				if got != nil {
					t.Errorf("CombinedError() = %v, want nil", got)
				}
				return
			}
			if got == nil {
				t.Error("CombinedError() = nil, want error")
				return
			}
			if got.Error() != tt.wantErrStr {
				t.Errorf("CombinedError().Error() = %q, want %q", got.Error(), tt.wantErrStr)
			}
		})
	}
}

func TestFetchStats(t *testing.T) {
	stats := FetchStats{
		Duration:       5 * time.Second,
		APICallCount:   3,
		EventsFetched:  100,
		EventsReturned: 50,
	}

	if stats.Duration != 5*time.Second {
		t.Errorf("Duration = %v, want %v", stats.Duration, 5*time.Second)
	}
	if stats.APICallCount != 3 {
		t.Errorf("APICallCount = %d, want 3", stats.APICallCount)
	}
	if stats.EventsFetched != 100 {
		t.Errorf("EventsFetched = %d, want 100", stats.EventsFetched)
	}
	if stats.EventsReturned != 50 {
		t.Errorf("EventsReturned = %d, want 50", stats.EventsReturned)
	}
}

func TestFetchResult_RateLimitFields(t *testing.T) {
	resetTime := time.Now().Add(1 * time.Hour)
	result := FetchResult{
		RateLimited:    true,
		RateLimitReset: resetTime,
	}

	if !result.RateLimited {
		t.Error("RateLimited = false, want true")
	}
	if !result.RateLimitReset.Equal(resetTime) {
		t.Errorf("RateLimitReset = %v, want %v", result.RateLimitReset, resetTime)
	}
}
