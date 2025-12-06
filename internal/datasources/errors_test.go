package datasources

import (
	"errors"
	"testing"
)

func TestSentinelErrors(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		errMsg string
	}{
		{
			name:   "ErrNotAuthenticated",
			err:    ErrNotAuthenticated,
			errMsg: "datasource: not authenticated",
		},
		{
			name:   "ErrRateLimited",
			err:    ErrRateLimited,
			errMsg: "datasource: rate limited",
		},
		{
			name:   "ErrServiceUnavailable",
			err:    ErrServiceUnavailable,
			errMsg: "datasource: service unavailable",
		},
		{
			name:   "ErrTimeout",
			err:    ErrTimeout,
			errMsg: "datasource: timeout",
		},
		{
			name:   "ErrInvalidResponse",
			err:    ErrInvalidResponse,
			errMsg: "datasource: invalid response",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err.Error() != tt.errMsg {
				t.Errorf("%s.Error() = %q, want %q", tt.name, tt.err.Error(), tt.errMsg)
			}
		})
	}
}

func TestSentinelErrors_ErrorsIs(t *testing.T) {
	// Test that sentinel errors work with errors.Is
	//nolint:govet // Test struct field order for readability
	tests := []struct {
		name      string
		err       error
		target    error
		wantMatch bool
	}{
		{
			name:      "ErrNotAuthenticated matches itself",
			err:       ErrNotAuthenticated,
			target:    ErrNotAuthenticated,
			wantMatch: true,
		},
		{
			name:      "wrapped ErrNotAuthenticated matches",
			err:       errors.Join(errors.New("context"), ErrNotAuthenticated),
			target:    ErrNotAuthenticated,
			wantMatch: true,
		},
		{
			name:      "ErrRateLimited does not match ErrNotAuthenticated",
			err:       ErrRateLimited,
			target:    ErrNotAuthenticated,
			wantMatch: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := errors.Is(tt.err, tt.target); got != tt.wantMatch {
				t.Errorf("errors.Is(%v, %v) = %v, want %v", tt.err, tt.target, got, tt.wantMatch)
			}
		})
	}
}

func TestSentinelErrors_Distinct(t *testing.T) {
	// Ensure all sentinel errors are distinct
	errs := []error{
		ErrNotAuthenticated,
		ErrRateLimited,
		ErrServiceUnavailable,
		ErrTimeout,
		ErrInvalidResponse,
	}

	for i, err1 := range errs {
		for j, err2 := range errs {
			if i != j && errors.Is(err1, err2) {
				t.Errorf("sentinel errors %v and %v should be distinct", err1, err2)
			}
		}
	}
}
