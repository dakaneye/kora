package datasources

import (
	"testing"
	"time"

	"github.com/dakaneye/kora/internal/models"
)

func TestFetchOptions_Validate(t *testing.T) {
	//nolint:govet // Test struct field order for readability
	tests := []struct {
		name    string
		opts    FetchOptions
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid options with required fields",
			opts: FetchOptions{
				Since: time.Now().Add(-24 * time.Hour),
			},
			wantErr: false,
		},
		{
			name: "valid options with all fields",
			opts: FetchOptions{
				Since: time.Now().Add(-24 * time.Hour),
				Limit: 100,
				Filter: &FetchFilter{
					EventTypes:     []models.EventType{models.EventTypePRReview},
					MinPriority:    models.PriorityMedium,
					RequiresAction: true,
				},
			},
			wantErr: false,
		},
		{
			name: "valid options with zero limit",
			opts: FetchOptions{
				Since: time.Now().Add(-24 * time.Hour),
				Limit: 0,
			},
			wantErr: false,
		},
		{
			name:    "invalid - zero Since time",
			opts:    FetchOptions{},
			wantErr: true,
			errMsg:  "FetchOptions.Since is required",
		},
		{
			name: "invalid - negative Limit",
			opts: FetchOptions{
				Since: time.Now().Add(-24 * time.Hour),
				Limit: -1,
			},
			wantErr: true,
			errMsg:  "FetchOptions.Limit must be non-negative",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.opts.Validate()
			if tt.wantErr {
				if err == nil {
					t.Errorf("Validate() error = nil, want error containing %q", tt.errMsg)
					return
				}
				if tt.errMsg != "" && err.Error() != tt.errMsg {
					t.Errorf("Validate() error = %q, want %q", err.Error(), tt.errMsg)
				}
			} else if err != nil {
				t.Errorf("Validate() unexpected error = %v", err)
			}
		})
	}
}

func TestFetchFilter(t *testing.T) {
	// FetchFilter has no validation, just ensure it can hold data correctly
	filter := &FetchFilter{
		EventTypes:     []models.EventType{models.EventTypePRReview, models.EventTypeSlackDM},
		MinPriority:    models.PriorityHigh,
		RequiresAction: true,
	}

	if len(filter.EventTypes) != 2 {
		t.Errorf("EventTypes len = %d, want 2", len(filter.EventTypes))
	}
	if filter.MinPriority != models.PriorityHigh {
		t.Errorf("MinPriority = %d, want %d", filter.MinPriority, models.PriorityHigh)
	}
	if !filter.RequiresAction {
		t.Error("RequiresAction = false, want true")
	}
}
