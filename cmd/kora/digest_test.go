package main

import (
	"testing"
	"time"
)

func TestParseSince(t *testing.T) {
	now := time.Now()
	defaultWindow := 16 * time.Hour

	//nolint:govet // fieldalignment in tests is not a concern
	tests := []struct {
		name        string
		value       string
		checkResult func(t *testing.T, result time.Time)
		wantErr     bool
	}{
		{
			name:  "empty uses default",
			value: "",
			checkResult: func(t *testing.T, result time.Time) {
				expected := now.Add(-defaultWindow)
				// Allow 1 second tolerance for test execution time
				diff := result.Sub(expected)
				if diff < -time.Second || diff > time.Second {
					t.Errorf("Expected ~%v ago, got %v ago", defaultWindow, now.Sub(result))
				}
			},
			wantErr: false,
		},
		{
			name:  "duration 8h",
			value: "8h",
			checkResult: func(t *testing.T, result time.Time) {
				expected := now.Add(-8 * time.Hour)
				diff := result.Sub(expected)
				if diff < -time.Second || diff > time.Second {
					t.Errorf("Expected ~8h ago, got %v ago", now.Sub(result))
				}
			},
			wantErr: false,
		},
		{
			name:  "duration 30m",
			value: "30m",
			checkResult: func(t *testing.T, result time.Time) {
				expected := now.Add(-30 * time.Minute)
				diff := result.Sub(expected)
				if diff < -time.Second || diff > time.Second {
					t.Errorf("Expected ~30m ago, got %v ago", now.Sub(result))
				}
			},
			wantErr: false,
		},
		{
			name:  "duration 1h30m",
			value: "1h30m",
			checkResult: func(t *testing.T, result time.Time) {
				expected := now.Add(-90 * time.Minute)
				diff := result.Sub(expected)
				if diff < -time.Second || diff > time.Second {
					t.Errorf("Expected ~1h30m ago, got %v ago", now.Sub(result))
				}
			},
			wantErr: false,
		},
		{
			name:    "negative duration",
			value:   "-8h",
			wantErr: true,
		},
		{
			name:    "zero duration",
			value:   "0s",
			wantErr: true,
		},
		{
			name:  "RFC3339 timestamp",
			value: "2025-01-01T09:00:00Z",
			checkResult: func(t *testing.T, result time.Time) {
				expected, _ := time.Parse(time.RFC3339, "2025-01-01T09:00:00Z")
				if !result.Equal(expected) {
					t.Errorf("Expected %v, got %v", expected, result)
				}
			},
			wantErr: false,
		},
		{
			name:  "RFC3339 with timezone",
			value: "2025-01-01T09:00:00-05:00",
			checkResult: func(t *testing.T, result time.Time) {
				expected, _ := time.Parse(time.RFC3339, "2025-01-01T09:00:00-05:00")
				if !result.Equal(expected) {
					t.Errorf("Expected %v, got %v", expected, result)
				}
			},
			wantErr: false,
		},
		{
			name:    "invalid format",
			value:   "yesterday",
			wantErr: true,
		},
		{
			name:    "invalid RFC3339",
			value:   "2025-01-01",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseSince(tt.value, defaultWindow)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseSince() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && tt.checkResult != nil {
				tt.checkResult(t, result)
			}
		})
	}
}

func TestParseSince_FutureTimestamp(t *testing.T) {
	// A timestamp in the future should be rejected
	future := time.Now().Add(24 * time.Hour).Format(time.RFC3339)
	_, err := parseSince(future, 16*time.Hour)
	if err == nil {
		t.Error("Expected error for future timestamp")
	}
}
