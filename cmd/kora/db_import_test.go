package main

import (
	"testing"
)

// TestValidateGoalRecord tests validation of goal records
func TestValidateGoalRecord(t *testing.T) {
	tests := []struct {
		name        string
		record      map[string]any
		wantErrors  int
		errContains string
	}{
		{
			name: "valid goal record",
			record: map[string]any{
				"id":         "goal-1",
				"title":      "Complete v1",
				"status":     "active",
				"priority":   3,
				"created_at": "2024-01-01T00:00:00Z",
				"updated_at": "2024-01-01T00:00:00Z",
			},
			wantErrors: 0,
		},
		{
			name: "missing id",
			record: map[string]any{
				"title":      "Complete v1",
				"created_at": "2024-01-01T00:00:00Z",
				"updated_at": "2024-01-01T00:00:00Z",
			},
			wantErrors:  1,
			errContains: "missing required field \"id\"",
		},
		{
			name: "missing title",
			record: map[string]any{
				"id":         "goal-1",
				"created_at": "2024-01-01T00:00:00Z",
				"updated_at": "2024-01-01T00:00:00Z",
			},
			wantErrors:  1,
			errContains: "missing required field \"title\"",
		},
		{
			name: "missing created_at",
			record: map[string]any{
				"id":         "goal-1",
				"title":      "Complete v1",
				"updated_at": "2024-01-01T00:00:00Z",
			},
			wantErrors:  1,
			errContains: "missing required field \"created_at\"",
		},
		{
			name: "missing updated_at",
			record: map[string]any{
				"id":         "goal-1",
				"title":      "Complete v1",
				"created_at": "2024-01-01T00:00:00Z",
			},
			wantErrors:  1,
			errContains: "missing required field \"updated_at\"",
		},
		{
			name: "invalid status enum",
			record: map[string]any{
				"id":         "goal-1",
				"title":      "Complete v1",
				"status":     "invalid",
				"created_at": "2024-01-01T00:00:00Z",
				"updated_at": "2024-01-01T00:00:00Z",
			},
			wantErrors:  1,
			errContains: "must be one of",
		},
		{
			name: "priority too low",
			record: map[string]any{
				"id":         "goal-1",
				"title":      "Complete v1",
				"priority":   0,
				"created_at": "2024-01-01T00:00:00Z",
				"updated_at": "2024-01-01T00:00:00Z",
			},
			wantErrors:  1,
			errContains: "must be 1-5",
		},
		{
			name: "priority too high",
			record: map[string]any{
				"id":         "goal-1",
				"title":      "Complete v1",
				"priority":   6,
				"created_at": "2024-01-01T00:00:00Z",
				"updated_at": "2024-01-01T00:00:00Z",
			},
			wantErrors:  1,
			errContains: "must be 1-5",
		},
		{
			name: "invalid timestamp format",
			record: map[string]any{
				"id":         "goal-1",
				"title":      "Complete v1",
				"created_at": "invalid-timestamp",
				"updated_at": "2024-01-01T00:00:00Z",
			},
			wantErrors:  1,
			errContains: "invalid timestamp format",
		},
		{
			name: "invalid tags JSON",
			record: map[string]any{
				"id":         "goal-1",
				"title":      "Complete v1",
				"tags":       "{not valid json}",
				"created_at": "2024-01-01T00:00:00Z",
				"updated_at": "2024-01-01T00:00:00Z",
			},
			wantErrors:  1,
			errContains: "invalid JSON array",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stats := &importStats{}
			seenIDs := make(map[string]bool)
			validateGoalRecord(tt.record, "test", seenIDs, stats)

			if len(stats.Errors) != tt.wantErrors {
				t.Errorf("got %d errors, want %d\nErrors: %v", len(stats.Errors), tt.wantErrors, stats.Errors)
			}

			if tt.errContains != "" && len(stats.Errors) > 0 {
				found := false
				for _, err := range stats.Errors {
					if containsString(err, tt.errContains) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected error containing %q, got: %v", tt.errContains, stats.Errors)
				}
			}
		})
	}
}

// TestValidateCommitmentRecord tests validation of commitment records
func TestValidateCommitmentRecord(t *testing.T) {
	tests := []struct {
		name        string
		record      map[string]any
		wantErrors  int
		errContains string
	}{
		{
			name: "valid commitment record",
			record: map[string]any{
				"id":         "commit-1",
				"title":      "Review PR",
				"status":     "active",
				"due_date":   "2024-01-15T00:00:00Z",
				"created_at": "2024-01-01T00:00:00Z",
				"updated_at": "2024-01-01T00:00:00Z",
			},
			wantErrors: 0,
		},
		{
			name: "missing due_date",
			record: map[string]any{
				"id":         "commit-1",
				"title":      "Review PR",
				"created_at": "2024-01-01T00:00:00Z",
				"updated_at": "2024-01-01T00:00:00Z",
			},
			wantErrors:  1,
			errContains: "missing required field \"due_date\"",
		},
		{
			name: "invalid status enum",
			record: map[string]any{
				"id":         "commit-1",
				"title":      "Review PR",
				"status":     "invalid",
				"due_date":   "2024-01-15T00:00:00Z",
				"created_at": "2024-01-01T00:00:00Z",
				"updated_at": "2024-01-01T00:00:00Z",
			},
			wantErrors:  1,
			errContains: "must be one of",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stats := &importStats{}
			seenIDs := make(map[string]bool)
			validateCommitmentRecord(tt.record, "test", seenIDs, stats)

			if len(stats.Errors) != tt.wantErrors {
				t.Errorf("got %d errors, want %d\nErrors: %v", len(stats.Errors), tt.wantErrors, stats.Errors)
			}

			if tt.errContains != "" && len(stats.Errors) > 0 {
				found := false
				for _, err := range stats.Errors {
					if containsString(err, tt.errContains) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected error containing %q, got: %v", tt.errContains, stats.Errors)
				}
			}
		})
	}
}

// TestValidateAccomplishmentRecord tests validation of accomplishment records
func TestValidateAccomplishmentRecord(t *testing.T) {
	tests := []struct {
		name        string
		record      map[string]any
		wantErrors  int
		errContains string
	}{
		{
			name: "valid accomplishment record",
			record: map[string]any{
				"id":              "acc-1",
				"title":           "Shipped feature X",
				"accomplished_at": "2024-01-15T00:00:00Z",
				"created_at":      "2024-01-01T00:00:00Z",
				"updated_at":      "2024-01-01T00:00:00Z",
			},
			wantErrors: 0,
		},
		{
			name: "missing accomplished_at",
			record: map[string]any{
				"id":         "acc-1",
				"title":      "Shipped feature X",
				"created_at": "2024-01-01T00:00:00Z",
				"updated_at": "2024-01-01T00:00:00Z",
			},
			wantErrors:  1,
			errContains: "missing required field \"accomplished_at\"",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stats := &importStats{}
			seenIDs := make(map[string]bool)
			validateAccomplishmentRecord(tt.record, "test", seenIDs, stats)

			if len(stats.Errors) != tt.wantErrors {
				t.Errorf("got %d errors, want %d\nErrors: %v", len(stats.Errors), tt.wantErrors, stats.Errors)
			}

			if tt.errContains != "" && len(stats.Errors) > 0 {
				found := false
				for _, err := range stats.Errors {
					if containsString(err, tt.errContains) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected error containing %q, got: %v", tt.errContains, stats.Errors)
				}
			}
		})
	}
}

// TestValidateContextRecord tests validation of context records
func TestValidateContextRecord(t *testing.T) {
	tests := []struct {
		name        string
		record      map[string]any
		wantErrors  int
		errContains string
	}{
		{
			name: "valid context record",
			record: map[string]any{
				"id":          "ctx-1",
				"entity_type": "person",
				"entity_id":   "alice",
				"title":       "Alice context",
				"body":        "Alice prefers async",
				"created_at":  "2024-01-01T00:00:00Z",
				"updated_at":  "2024-01-01T00:00:00Z",
			},
			wantErrors: 0,
		},
		{
			name: "missing entity_type",
			record: map[string]any{
				"id":         "ctx-1",
				"entity_id":  "alice",
				"title":      "Alice context",
				"body":       "Alice prefers async",
				"created_at": "2024-01-01T00:00:00Z",
				"updated_at": "2024-01-01T00:00:00Z",
			},
			wantErrors:  1,
			errContains: "missing required field \"entity_type\"",
		},
		{
			name: "missing entity_id",
			record: map[string]any{
				"id":          "ctx-1",
				"entity_type": "person",
				"title":       "Alice context",
				"body":        "Alice prefers async",
				"created_at":  "2024-01-01T00:00:00Z",
				"updated_at":  "2024-01-01T00:00:00Z",
			},
			wantErrors:  1,
			errContains: "missing required field \"entity_id\"",
		},
		{
			name: "missing body",
			record: map[string]any{
				"id":          "ctx-1",
				"entity_type": "person",
				"entity_id":   "alice",
				"title":       "Alice context",
				"created_at":  "2024-01-01T00:00:00Z",
				"updated_at":  "2024-01-01T00:00:00Z",
			},
			wantErrors:  1,
			errContains: "missing required field \"body\"",
		},
		{
			name: "invalid entity_type",
			record: map[string]any{
				"id":          "ctx-1",
				"entity_type": "invalid",
				"entity_id":   "alice",
				"title":       "Alice context",
				"body":        "Alice prefers async",
				"created_at":  "2024-01-01T00:00:00Z",
				"updated_at":  "2024-01-01T00:00:00Z",
			},
			wantErrors:  1,
			errContains: "must be one of",
		},
		{
			name: "invalid urgency",
			record: map[string]any{
				"id":          "ctx-1",
				"entity_type": "person",
				"entity_id":   "alice",
				"title":       "Alice context",
				"body":        "Alice prefers async",
				"urgency":     "invalid",
				"created_at":  "2024-01-01T00:00:00Z",
				"updated_at":  "2024-01-01T00:00:00Z",
			},
			wantErrors:  1,
			errContains: "must be one of",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stats := &importStats{}
			seenIDs := make(map[string]bool)
			validateContextRecord(tt.record, "test", seenIDs, stats)

			if len(stats.Errors) != tt.wantErrors {
				t.Errorf("got %d errors, want %d\nErrors: %v", len(stats.Errors), tt.wantErrors, stats.Errors)
			}

			if tt.errContains != "" && len(stats.Errors) > 0 {
				found := false
				for _, err := range stats.Errors {
					if containsString(err, tt.errContains) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected error containing %q, got: %v", tt.errContains, stats.Errors)
				}
			}
		})
	}
}

// TestValidateImportData_DuplicateIDs tests duplicate ID detection within file
func TestValidateImportData_DuplicateIDs(t *testing.T) {
	data := &exportData{
		SchemaVersion: 1,
		Goals: []map[string]any{
			{
				"id":         "goal-1",
				"title":      "First",
				"created_at": "2024-01-01T00:00:00Z",
				"updated_at": "2024-01-01T00:00:00Z",
			},
			{
				"id":         "goal-1", // Duplicate
				"title":      "Second",
				"created_at": "2024-01-01T00:00:00Z",
				"updated_at": "2024-01-01T00:00:00Z",
			},
		},
	}

	stats := &importStats{}
	err := validateImportData(data, stats)
	if err != nil {
		t.Fatalf("validateImportData returned error: %v", err)
	}

	if len(stats.Errors) == 0 {
		t.Error("expected duplicate ID error, got none")
	}

	found := false
	for _, errMsg := range stats.Errors {
		if containsString(errMsg, "duplicate ID") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'duplicate ID' in errors, got: %v", stats.Errors)
	}
}

// containsString checks if s contains substr.
func containsString(s, substr string) bool {
	return len(s) >= len(substr) && searchSubstring(s, substr) >= 0
}

// searchSubstring finds the first index of substr in s, or -1 if not found.
func searchSubstring(s, substr string) int {
	n := len(substr)
	for i := 0; i <= len(s)-n; i++ {
		if s[i:i+n] == substr {
			return i
		}
	}
	return -1
}
