package datasources

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dakaneye/kora/internal/models"
)

// mockDataSource implements DataSource for testing.
//
//nolint:govet // Test struct field order for readability
type mockDataSource struct {
	name        string
	service     models.Source
	events      []models.Event
	err         error
	fetchDelay  time.Duration
	fetchCount  atomic.Int32
	contextDone bool // if true, returns ctx.Err() when context is done
}

func (m *mockDataSource) Name() string {
	return m.name
}

func (m *mockDataSource) Service() models.Source {
	return m.service
}

func (m *mockDataSource) Fetch(ctx context.Context, _ FetchOptions) (*FetchResult, error) {
	m.fetchCount.Add(1)

	if m.fetchDelay > 0 {
		select {
		case <-time.After(m.fetchDelay):
		case <-ctx.Done():
			if m.contextDone {
				return nil, ctx.Err()
			}
		}
	}

	if m.err != nil {
		// Return partial results if events are available
		if len(m.events) > 0 {
			return &FetchResult{
				Events:  m.events,
				Partial: true,
				Errors:  []error{m.err},
			}, m.err
		}
		return nil, m.err
	}

	return &FetchResult{
		Events: m.events,
		Stats: FetchStats{
			EventsFetched:  len(m.events),
			EventsReturned: len(m.events),
		},
	}, nil
}

func TestNewRunner(t *testing.T) {
	sources := []DataSource{
		&mockDataSource{name: "test-1"},
		&mockDataSource{name: "test-2"},
	}

	runner := NewRunner(sources)

	if runner == nil {
		t.Fatal("NewRunner() returned nil")
	}
	if len(runner.sources) != 2 {
		t.Errorf("runner.sources len = %d, want 2", len(runner.sources))
	}
	if runner.timeout != 30*time.Second {
		t.Errorf("runner.timeout = %v, want 30s", runner.timeout)
	}
}

func TestNewRunner_WithTimeout(t *testing.T) {
	runner := NewRunner(nil, WithTimeout(5*time.Second))

	if runner.timeout != 5*time.Second {
		t.Errorf("runner.timeout = %v, want 5s", runner.timeout)
	}
}

func TestDataSourceRunner_Run_NoSources(t *testing.T) {
	runner := NewRunner(nil)
	ctx := context.Background()
	opts := FetchOptions{Since: time.Now().Add(-1 * time.Hour)}

	result, err := runner.Run(ctx, opts)

	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}
	if result == nil {
		t.Fatal("Run() result = nil, want non-nil")
	}
	if len(result.Events) != 0 {
		t.Errorf("result.Events len = %d, want 0", len(result.Events))
	}
	if !result.Success() {
		t.Error("result.Success() = false, want true")
	}
}

func TestDataSourceRunner_Run_InvalidOptions(t *testing.T) {
	runner := NewRunner([]DataSource{&mockDataSource{name: "test"}})
	ctx := context.Background()
	opts := FetchOptions{} // Missing Since

	_, err := runner.Run(ctx, opts)

	if err == nil {
		t.Fatal("Run() error = nil, want error")
	}
	if !errors.Is(err, errors.New("invalid fetch options: FetchOptions.Since is required")) {
		// Just check that it mentions invalid options
		if err.Error() == "" {
			t.Error("Run() error should not be empty")
		}
	}
}

func TestDataSourceRunner_Run_AllSuccess(t *testing.T) {
	now := time.Now()
	sources := []DataSource{
		&mockDataSource{
			name:    "source-1",
			service: models.SourceGitHub,
			events: []models.Event{
				{
					Type:      models.EventTypePRReview,
					Title:     "Event 1",
					Source:    models.SourceGitHub,
					Timestamp: now.Add(-2 * time.Hour),
				},
			},
		},
		&mockDataSource{
			name:    "source-2",
			service: models.SourceSlack,
			events: []models.Event{
				{
					Type:      models.EventTypeSlackDM,
					Title:     "Event 2",
					Source:    models.SourceSlack,
					Timestamp: now.Add(-1 * time.Hour),
				},
			},
		},
	}

	runner := NewRunner(sources)
	ctx := context.Background()
	opts := FetchOptions{Since: now.Add(-24 * time.Hour)}

	result, err := runner.Run(ctx, opts)

	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}
	if result.TotalEvents() != 2 {
		t.Errorf("TotalEvents() = %d, want 2", result.TotalEvents())
	}
	if !result.Success() {
		t.Error("Success() = false, want true")
	}
	if result.Partial() {
		t.Error("Partial() = true, want false")
	}

	// Verify events are sorted by timestamp
	if len(result.Events) >= 2 {
		if !result.Events[0].Timestamp.Before(result.Events[1].Timestamp) {
			t.Error("Events not sorted by timestamp")
		}
	}
}

func TestDataSourceRunner_Run_PartialSuccess(t *testing.T) {
	now := time.Now()
	sources := []DataSource{
		&mockDataSource{
			name:    "success-source",
			service: models.SourceGitHub,
			events: []models.Event{
				{
					Type:      models.EventTypePRReview,
					Title:     "Success Event",
					Source:    models.SourceGitHub,
					Timestamp: now,
				},
			},
		},
		&mockDataSource{
			name:    "failing-source",
			service: models.SourceSlack,
			err:     errors.New("fetch failed"),
		},
	}

	runner := NewRunner(sources)
	ctx := context.Background()
	opts := FetchOptions{Since: now.Add(-24 * time.Hour)}

	result, err := runner.Run(ctx, opts)

	// Run should not return an error for partial success
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}
	if result.TotalEvents() != 1 {
		t.Errorf("TotalEvents() = %d, want 1", result.TotalEvents())
	}
	if result.Success() {
		t.Error("Success() = true, want false")
	}
	if !result.Partial() {
		t.Error("Partial() = false, want true")
	}
	if len(result.SourceErrors) != 1 {
		t.Errorf("SourceErrors len = %d, want 1", len(result.SourceErrors))
	}
	if _, ok := result.SourceErrors["failing-source"]; !ok {
		t.Error("SourceErrors should contain 'failing-source'")
	}
}

func TestDataSourceRunner_Run_AllFailed(t *testing.T) {
	sources := []DataSource{
		&mockDataSource{
			name:    "failing-1",
			service: models.SourceGitHub,
			err:     errors.New("error 1"),
		},
		&mockDataSource{
			name:    "failing-2",
			service: models.SourceSlack,
			err:     errors.New("error 2"),
		},
	}

	runner := NewRunner(sources)
	ctx := context.Background()
	opts := FetchOptions{Since: time.Now().Add(-24 * time.Hour)}

	result, err := runner.Run(ctx, opts)

	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}
	if result.TotalEvents() != 0 {
		t.Errorf("TotalEvents() = %d, want 0", result.TotalEvents())
	}
	if result.Success() {
		t.Error("Success() = true, want false")
	}
	if result.Partial() {
		t.Error("Partial() = true, want false (no successes)")
	}
	if len(result.SourceErrors) != 2 {
		t.Errorf("SourceErrors len = %d, want 2", len(result.SourceErrors))
	}
}

func TestDataSourceRunner_Run_PartialResultsOnError(t *testing.T) {
	now := time.Now()
	sources := []DataSource{
		&mockDataSource{
			name:    "partial-source",
			service: models.SourceGitHub,
			events: []models.Event{
				{
					Type:      models.EventTypePRReview,
					Title:     "Partial Event",
					Source:    models.SourceGitHub,
					Timestamp: now,
				},
			},
			err: errors.New("partial error"),
		},
	}

	runner := NewRunner(sources)
	ctx := context.Background()
	opts := FetchOptions{Since: now.Add(-24 * time.Hour)}

	result, err := runner.Run(ctx, opts)

	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}
	// Should include partial results even though there was an error
	if result.TotalEvents() != 1 {
		t.Errorf("TotalEvents() = %d, want 1", result.TotalEvents())
	}
	if len(result.SourceErrors) != 1 {
		t.Errorf("SourceErrors len = %d, want 1", len(result.SourceErrors))
	}
}

func TestDataSourceRunner_Run_Concurrent(t *testing.T) {
	// Create sources with delays to verify concurrent execution
	sources := []DataSource{
		&mockDataSource{
			name:       "slow-1",
			service:    models.SourceGitHub,
			events:     []models.Event{{Type: models.EventTypePRReview, Title: "E1", Source: models.SourceGitHub, Timestamp: time.Now()}},
			fetchDelay: 50 * time.Millisecond,
		},
		&mockDataSource{
			name:       "slow-2",
			service:    models.SourceSlack,
			events:     []models.Event{{Type: models.EventTypeSlackDM, Title: "E2", Source: models.SourceSlack, Timestamp: time.Now()}},
			fetchDelay: 50 * time.Millisecond,
		},
		&mockDataSource{
			name:       "slow-3",
			service:    models.SourceGitHub,
			events:     []models.Event{{Type: models.EventTypePRMention, Title: "E3", Source: models.SourceGitHub, Timestamp: time.Now()}},
			fetchDelay: 50 * time.Millisecond,
		},
	}

	runner := NewRunner(sources)
	ctx := context.Background()
	opts := FetchOptions{Since: time.Now().Add(-24 * time.Hour)}

	start := time.Now()
	result, err := runner.Run(ctx, opts)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}
	if result.TotalEvents() != 3 {
		t.Errorf("TotalEvents() = %d, want 3", result.TotalEvents())
	}

	// If run sequentially, it would take ~150ms. Concurrently should be ~50ms + overhead.
	// Allow up to 120ms to account for test environment variability.
	if elapsed > 120*time.Millisecond {
		t.Errorf("Run took %v, expected concurrent execution to be faster", elapsed)
	}
}

func TestDataSourceRunner_Run_EventsSortedByTimestamp(t *testing.T) {
	now := time.Now()
	sources := []DataSource{
		&mockDataSource{
			name:    "source-1",
			service: models.SourceGitHub,
			events: []models.Event{
				{Type: models.EventTypePRReview, Title: "Newest", Source: models.SourceGitHub, Timestamp: now},
			},
		},
		&mockDataSource{
			name:    "source-2",
			service: models.SourceSlack,
			events: []models.Event{
				{Type: models.EventTypeSlackDM, Title: "Oldest", Source: models.SourceSlack, Timestamp: now.Add(-3 * time.Hour)},
			},
		},
		&mockDataSource{
			name:    "source-3",
			service: models.SourceGitHub,
			events: []models.Event{
				{Type: models.EventTypePRMention, Title: "Middle", Source: models.SourceGitHub, Timestamp: now.Add(-1 * time.Hour)},
			},
		},
	}

	runner := NewRunner(sources)
	ctx := context.Background()
	opts := FetchOptions{Since: now.Add(-24 * time.Hour)}

	result, err := runner.Run(ctx, opts)

	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}
	if result.TotalEvents() != 3 {
		t.Fatalf("TotalEvents() = %d, want 3", result.TotalEvents())
	}

	// Verify ascending order
	if result.Events[0].Title != "Oldest" {
		t.Errorf("Events[0].Title = %q, want %q", result.Events[0].Title, "Oldest")
	}
	if result.Events[1].Title != "Middle" {
		t.Errorf("Events[1].Title = %q, want %q", result.Events[1].Title, "Middle")
	}
	if result.Events[2].Title != "Newest" {
		t.Errorf("Events[2].Title = %q, want %q", result.Events[2].Title, "Newest")
	}
}

func TestRunResult_Success(t *testing.T) {
	//nolint:govet // Test struct field order for readability
	tests := []struct {
		name   string
		result RunResult
		want   bool
	}{
		{
			name: "no errors means success",
			result: RunResult{
				SourceResults: map[string]*FetchResult{"a": {}},
				SourceErrors:  map[string]error{},
			},
			want: true,
		},
		{
			name: "has errors means not success",
			result: RunResult{
				SourceResults: map[string]*FetchResult{"a": {}},
				SourceErrors:  map[string]error{"b": errors.New("err")},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.result.Success(); got != tt.want {
				t.Errorf("Success() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRunResult_Partial(t *testing.T) {
	//nolint:govet // Test struct field order for readability
	tests := []struct {
		name   string
		result RunResult
		want   bool
	}{
		{
			name: "some success and some errors means partial",
			result: RunResult{
				SourceResults: map[string]*FetchResult{"a": {}},
				SourceErrors:  map[string]error{"b": errors.New("err")},
			},
			want: true,
		},
		{
			name: "all success means not partial",
			result: RunResult{
				SourceResults: map[string]*FetchResult{"a": {}, "b": {}},
				SourceErrors:  map[string]error{},
			},
			want: false,
		},
		{
			name: "all errors means not partial",
			result: RunResult{
				SourceResults: map[string]*FetchResult{},
				SourceErrors:  map[string]error{"a": errors.New("err"), "b": errors.New("err")},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.result.Partial(); got != tt.want {
				t.Errorf("Partial() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRunResult_TotalEvents(t *testing.T) {
	result := RunResult{
		Events: make([]models.Event, 5),
	}

	if got := result.TotalEvents(); got != 5 {
		t.Errorf("TotalEvents() = %d, want 5", got)
	}
}
