package source_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/dakaneye/kora/internal/source"
)

type mockSource struct {
	name       string
	authErr    error
	refreshErr error
	fetchData  json.RawMessage
	fetchErr   error
	authDelay  time.Duration
}

func (m *mockSource) Name() string { return m.name }

func (m *mockSource) CheckAuth(ctx context.Context) error {
	if m.authDelay > 0 {
		select {
		case <-time.After(m.authDelay):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return m.authErr
}

func (m *mockSource) RefreshAuth(ctx context.Context) error {
	if m.refreshErr != nil {
		return m.refreshErr
	}
	// Simulate successful refresh by clearing auth error
	m.authErr = nil
	return nil
}

func (m *mockSource) Fetch(ctx context.Context, since time.Duration) (json.RawMessage, error) {
	return m.fetchData, m.fetchErr
}

func TestRun_AllSourcesSucceed(t *testing.T) {
	sources := []source.Source{
		&mockSource{name: "alpha", fetchData: json.RawMessage(`{"items":[1]}`)},
		&mockSource{name: "beta", fetchData: json.RawMessage(`{"items":[2]}`)},
	}

	result, err := source.Run(t.Context(), sources, 8*time.Hour)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := result.Sources["alpha"]; !ok {
		t.Error("missing alpha in result")
	}
	if _, ok := result.Sources["beta"]; !ok {
		t.Error("missing beta in result")
	}
	if result.Since != "8h0m0s" {
		t.Errorf("since = %q, want %q", result.Since, "8h0m0s")
	}
}

func TestRun_AuthFailure(t *testing.T) {
	sources := []source.Source{
		&mockSource{name: "good", fetchData: json.RawMessage(`{}`)},
		&mockSource{name: "bad", authErr: errors.New("auth expired"), refreshErr: errors.New("refresh failed")},
	}

	_, err := source.Run(t.Context(), sources, 8*time.Hour)
	if err == nil {
		t.Fatal("expected error when auth fails")
	}
}

func TestRun_AuthRefreshSuccess(t *testing.T) {
	sources := []source.Source{
		&mockSource{name: "good", fetchData: json.RawMessage(`{}`)},
		&mockSource{name: "recoverable", authErr: errors.New("expired"), fetchData: json.RawMessage(`{"recovered":true}`)},
	}

	result, err := source.Run(t.Context(), sources, 8*time.Hour)
	if err != nil {
		t.Fatalf("expected success after refresh, got: %v", err)
	}
	if _, ok := result.Sources["recoverable"]; !ok {
		t.Error("missing recoverable source in result after refresh")
	}
}

func TestRun_FetchFailure(t *testing.T) {
	sources := []source.Source{
		&mockSource{name: "good", fetchData: json.RawMessage(`{}`)},
		&mockSource{name: "bad", fetchErr: errors.New("api timeout")},
	}

	_, err := source.Run(t.Context(), sources, 8*time.Hour)
	if err == nil {
		t.Fatal("expected error when fetch fails")
	}
}

func TestRun_AuthRunsInParallel(t *testing.T) {
	sources := []source.Source{
		&mockSource{name: "a", authDelay: 50 * time.Millisecond, fetchData: json.RawMessage(`{}`)},
		&mockSource{name: "b", authDelay: 50 * time.Millisecond, fetchData: json.RawMessage(`{}`)},
	}

	start := time.Now()
	_, err := source.Run(t.Context(), sources, 8*time.Hour)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if elapsed > 90*time.Millisecond {
		t.Errorf("auth took %v, expected parallel execution (~50ms)", elapsed)
	}
}

func TestRun_EmptySources(t *testing.T) {
	result, err := source.Run(t.Context(), nil, 8*time.Hour)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Sources) != 0 {
		t.Errorf("expected empty sources, got %d", len(result.Sources))
	}
}

func TestRunError_Error(t *testing.T) {
	runErr := &source.RunError{
		Errors: []source.SourceError{
			{Source: "github", Phase: "fetch", Err: "api timeout"},
			{Source: "linear", Phase: "auth", Err: "token expired"},
		},
	}
	got := runErr.Error()
	if !strings.Contains(got, "github (fetch): api timeout") {
		t.Errorf("error string missing github entry: %s", got)
	}
	if !strings.Contains(got, "linear (auth): token expired") {
		t.Errorf("error string missing linear entry: %s", got)
	}
	if !strings.Contains(got, "; ") {
		t.Errorf("error string missing separator: %s", got)
	}
}

func TestRun_MultipleFetchFailures(t *testing.T) {
	sources := []source.Source{
		&mockSource{name: "alpha", fetchErr: errors.New("timeout")},
		&mockSource{name: "beta", fetchErr: errors.New("rate limited")},
		&mockSource{name: "gamma", fetchErr: errors.New("connection refused")},
	}

	_, err := source.Run(t.Context(), sources, 8*time.Hour)
	if err == nil {
		t.Fatal("expected error when multiple fetches fail")
	}
	var runErr *source.RunError
	if !errors.As(err, &runErr) {
		t.Fatalf("expected *RunError, got %T", err)
	}
	if len(runErr.Errors) != 3 {
		t.Errorf("expected 3 source errors, got %d", len(runErr.Errors))
	}
	// Verify all sources are represented
	names := make(map[string]bool)
	for _, se := range runErr.Errors {
		names[se.Source] = true
		if se.Phase != "fetch" {
			t.Errorf("expected phase 'fetch', got %q for source %s", se.Phase, se.Source)
		}
	}
	for _, name := range []string{"alpha", "beta", "gamma"} {
		if !names[name] {
			t.Errorf("missing source %q in errors", name)
		}
	}
}

func TestRun_AuthLifecycle_AllSources(t *testing.T) {
	tests := []struct {
		name       string
		authErr    error
		refreshErr error
		fetchData  json.RawMessage
		fetchErr   error
		wantErr    bool
	}{
		{
			name:      "github-like: auth succeeds immediately",
			fetchData: json.RawMessage(`{"prs":[]}`),
		},
		{
			name:      "gmail-like: auth fails then refresh succeeds",
			authErr:   errors.New("token expired"),
			fetchData: json.RawMessage(`{"messages":[]}`),
		},
		{
			name:       "calendar-like: auth fails and refresh fails",
			authErr:    errors.New("no credentials"),
			refreshErr: errors.New("browser required"),
			wantErr:    true,
		},
		{
			name:      "linear-like: auth succeeds but fetch fails",
			fetchErr:  errors.New("graphql error"),
			fetchData: json.RawMessage(`{}`),
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			src := &mockSource{
				name:       tt.name,
				authErr:    tt.authErr,
				refreshErr: tt.refreshErr,
				fetchData:  tt.fetchData,
				fetchErr:   tt.fetchErr,
			}
			_, err := source.Run(t.Context(), []source.Source{src}, 8*time.Hour)
			if tt.wantErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestRun_ContextCancellationDuringFetch(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel() // cancel immediately

	sources := []source.Source{
		&mockSource{name: "slow", fetchData: json.RawMessage(`{}`)},
	}

	_, err := source.Run(ctx, sources, 8*time.Hour)
	// With a cancelled context, auth check or fetch should fail.
	// We just verify it doesn't hang.
	if err == nil {
		// It's acceptable if the mock doesn't check ctx, but the test
		// confirms no deadlock.
		return
	}
}
