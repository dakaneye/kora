package models

import "testing"

func TestPriority_IsValid(t *testing.T) {
	tests := []struct {
		name     string
		priority Priority
		want     bool
	}{
		{"Priority 1 (Critical) is valid", PriorityCritical, true},
		{"Priority 2 (High) is valid", PriorityHigh, true},
		{"Priority 3 (Medium) is valid", PriorityMedium, true},
		{"Priority 4 (Low) is valid", PriorityLow, true},
		{"Priority 5 (Info) is valid", PriorityInfo, true},
		{"Priority 0 is invalid", Priority(0), false},
		{"Priority 6 is invalid", Priority(6), false},
		{"Priority -1 is invalid", Priority(-1), false},
		{"Priority 100 is invalid", Priority(100), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.priority.IsValid(); got != tt.want {
				t.Errorf("Priority.IsValid() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPriority_Constants(t *testing.T) {
	// Verify priority constants have correct values
	tests := []struct {
		constant Priority
		expected int
	}{
		{PriorityCritical, 1},
		{PriorityHigh, 2},
		{PriorityMedium, 3},
		{PriorityLow, 4},
		{PriorityInfo, 5},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			if int(tt.constant) != tt.expected {
				t.Errorf("Priority constant = %d, want %d", tt.constant, tt.expected)
			}
			if !tt.constant.IsValid() {
				t.Errorf("Priority constant %d should be valid", tt.constant)
			}
		})
	}
}

func TestPriority_Range(t *testing.T) {
	// Test the full range boundary conditions
	tests := []struct {
		name     string
		priority Priority
		want     bool
	}{
		{"minimum valid (1)", Priority(1), true},
		{"maximum valid (5)", Priority(5), true},
		{"below minimum (0)", Priority(0), false},
		{"above maximum (6)", Priority(6), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.priority.IsValid(); got != tt.want {
				t.Errorf("Priority(%d).IsValid() = %v, want %v", tt.priority, got, tt.want)
			}
		})
	}
}

func TestPriority_String(t *testing.T) {
	//nolint:govet // test struct field order prioritizes readability
	tests := []struct {
		name     string
		priority Priority
		want     string
	}{
		{"Critical", PriorityCritical, "Critical"},
		{"High", PriorityHigh, "High"},
		{"Medium", PriorityMedium, "Medium"},
		{"Low", PriorityLow, "Low"},
		{"Info", PriorityInfo, "Info"},
		{"Unknown (0)", Priority(0), "Unknown"},
		{"Unknown (6)", Priority(6), "Unknown"},
		{"Unknown (-1)", Priority(-1), "Unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.priority.String(); got != tt.want {
				t.Errorf("Priority.String() = %v, want %v", got, tt.want)
			}
		})
	}
}
