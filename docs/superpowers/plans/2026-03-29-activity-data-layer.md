# Kora v2: Activity Data Layer Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Rewrite Kora as a single-purpose CLI that gathers work activity from GitHub, Gmail, Calendar, and Linear, outputting raw JSON for Claude to synthesize.

**Architecture:** Four Source implementations behind a common interface, orchestrated with parallel auth checking and parallel fetching. Each source delegates to an existing CLI tool (`gh`, `gws`, `linear`). A thin `exec` package handles subprocess execution.

**Tech Stack:** Go 1.25, `golang.org/x/sync/errgroup`, `gh` CLI, `gws` CLI, `linear` CLI (schpet/linear-cli)

**Spec:** `docs/superpowers/specs/2026-03-29-activity-data-layer-design.md`

---

## File Structure

| File | Responsibility |
|------|---------------|
| `cmd/kora/main.go` | Flag parsing (`--since`), creates sources, runs orchestrator, prints JSON or errors |
| `internal/source/source.go` | `Source` interface, `Run` orchestrator (parallel auth, sequential reauth, parallel fetch) |
| `internal/source/source_test.go` | Unit tests for orchestrator logic with mock sources |
| `internal/source/github.go` | GitHub source — shells out to `gh` CLI |
| `internal/source/github_test.go` | Unit tests with mocked `gh` subprocess calls |
| `internal/source/gmail.go` | Gmail source — shells out to `gws` CLI |
| `internal/source/gmail_test.go` | Unit tests with mocked `gws` subprocess calls |
| `internal/source/calendar.go` | Calendar source — shells out to `gws` CLI |
| `internal/source/calendar_test.go` | Unit tests with mocked `gws` subprocess calls |
| `internal/source/linear.go` | Linear source — shells out to `linear` CLI |
| `internal/source/linear_test.go` | Unit tests with mocked `linear` subprocess calls |
| `internal/exec/exec.go` | Subprocess runner: execute command, capture stdout/stderr, return structured result |
| `internal/exec/exec_test.go` | Unit tests for exec package |
| `tests/integration/sources_test.go` | Integration tests that call real CLI tools (build-tagged) |
| `tests/e2e/kora_test.go` | End-to-end tests that run the compiled `kora` binary |

---

### Task 1: Clean slate — delete v1 code, update go.mod

**Files:**
- Delete: `internal/auth/`, `internal/models/`, `internal/datasources/`, `internal/config/`, `internal/output/`, `internal/storage/`, `cmd/kora/` (all files), `tests/integration/` (all files), `specs/efas/`, `scripts/`, `configs/`, `pkg/`
- Modify: `go.mod`
- Modify: `Makefile`

- [ ] **Step 1: Delete all v1 source code**

```bash
rm -rf internal/auth internal/models internal/datasources internal/config internal/output internal/storage
rm -rf cmd/kora
rm -rf tests/integration
rm -rf specs/efas specs/prototype
rm -rf scripts configs pkg
rm -f .mcp.json
```

- [ ] **Step 2: Strip go.mod to essentials**

`go.mod` should only contain:

```go
module github.com/dakaneye/kora

go 1.25.1

require (
	golang.org/x/sync v0.18.0
)
```

Remove cobra, yaml, sqlite, and all indirect deps. Run `go mod tidy` after.

- [ ] **Step 3: Update Makefile**

Replace the Makefile with:

```makefile
BINARY_NAME := kora
BUILD_DIR := bin
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
DATE := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS := -ldflags "-X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)"

.PHONY: all build test test-integration test-e2e lint clean help

all: lint test build

build:
	@mkdir -p $(BUILD_DIR)
	go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/kora

test:
	go test -race -v ./internal/...

test-integration:
	go test -race -v -tags=integration ./tests/integration/...

test-e2e: build
	go test -race -v -tags=e2e ./tests/e2e/...

lint:
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run ./...; \
	else \
		echo "golangci-lint not installed"; \
		exit 1; \
	fi

install: build
	@mkdir -p ~/.local/bin
	@cp $(BUILD_DIR)/$(BINARY_NAME) ~/.local/bin/
	@echo "Installed to ~/.local/bin/$(BINARY_NAME)"

clean:
	@rm -rf $(BUILD_DIR)

help:
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@echo "  build            Build the kora binary"
	@echo "  test             Run unit tests"
	@echo "  test-integration Run integration tests (requires CLI tools + auth)"
	@echo "  test-e2e         Run end-to-end tests (builds binary first)"
	@echo "  lint             Run golangci-lint"
	@echo "  install          Install to ~/.local/bin"
	@echo "  clean            Remove build artifacts"
```

- [ ] **Step 4: Create directory structure**

```bash
mkdir -p cmd/kora internal/source internal/exec tests/integration tests/e2e
```

- [ ] **Step 5: Verify clean state**

Run: `go mod tidy`
Expected: no errors, go.sum updated

- [ ] **Step 6: Commit**

```bash
git add -A
git commit -m "chore: delete v1 code, prepare for v2 rewrite"
```

---

### Task 2: Exec package — subprocess runner

**Files:**
- Create: `internal/exec/exec.go`
- Create: `internal/exec/exec_test.go`

- [ ] **Step 1: Write the failing test for successful command execution**

```go
// internal/exec/exec_test.go
package exec_test

import (
	"context"
	"testing"

	"github.com/dakaneye/kora/internal/exec"
)

func TestRun_Success(t *testing.T) {
	ctx := t.Context()
	result, err := exec.Run(ctx, "echo", "hello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Stdout != "hello\n" {
		t.Errorf("stdout = %q, want %q", result.Stdout, "hello\n")
	}
	if result.ExitCode != 0 {
		t.Errorf("exit code = %d, want 0", result.ExitCode)
	}
}

func TestRun_Failure(t *testing.T) {
	ctx := t.Context()
	result, err := exec.Run(ctx, "false")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if result.ExitCode != 1 {
		t.Errorf("exit code = %d, want 1", result.ExitCode)
	}
}

func TestRun_NotFound(t *testing.T) {
	ctx := t.Context()
	_, err := exec.Run(ctx, "nonexistent-command-xyz")
	if err == nil {
		t.Fatal("expected error for missing command")
	}
}

func TestRun_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	_, err := exec.Run(ctx, "sleep", "10")
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
}

func TestRun_CapturesStderr(t *testing.T) {
	ctx := t.Context()
	result, err := exec.Run(ctx, "sh", "-c", "echo oops >&2; exit 1")
	if err == nil {
		t.Fatal("expected error")
	}
	if result.Stderr != "oops\n" {
		t.Errorf("stderr = %q, want %q", result.Stderr, "oops\n")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -race -v ./internal/exec/...`
Expected: FAIL — package does not exist

- [ ] **Step 3: Write the implementation**

```go
// internal/exec/exec.go
package exec

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
)

// Result holds the output of a subprocess execution.
type Result struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

// Run executes a command with the given arguments and returns the captured output.
// Returns a non-nil error if the command fails or the context is cancelled.
// The Result is always populated (even on error) so callers can inspect stderr.
func Run(ctx context.Context, name string, args ...string) (Result, error) {
	cmd := exec.CommandContext(ctx, name, args...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	result := Result{
		Stdout: stdout.String(),
		Stderr: stderr.String(),
	}

	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			result.ExitCode = exitErr.ExitCode()
		} else {
			result.ExitCode = -1
		}
		return result, fmt.Errorf("exec %s: %w (stderr: %s)", name, err, result.Stderr)
	}

	return result, nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test -race -v ./internal/exec/...`
Expected: all 5 tests PASS

- [ ] **Step 5: Commit**

```bash
git add internal/exec/
git commit -m "feat: add exec package for subprocess execution"
```

---

### Task 3: Source interface and orchestrator

**Files:**
- Create: `internal/source/source.go`
- Create: `internal/source/source_test.go`

- [ ] **Step 1: Write the failing tests for the orchestrator**

```go
// internal/source/source_test.go
package source_test

import (
	"context"
	"encoding/json"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dakaneye/kora/internal/source"
)

// mockSource implements source.Source for testing.
type mockSource struct {
	name      string
	authErr   error
	fetchData json.RawMessage
	fetchErr  error
	authDelay time.Duration
	fetchDelay time.Duration
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
	return m.authErr
}

func (m *mockSource) Fetch(ctx context.Context, since time.Duration) (json.RawMessage, error) {
	if m.fetchDelay > 0 {
		select {
		case <-time.After(m.fetchDelay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
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
		&mockSource{name: "bad", authErr: errors.New("auth expired")},
	}

	_, err := source.Run(t.Context(), sources, 8*time.Hour)
	if err == nil {
		t.Fatal("expected error when auth fails")
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
	var concurrent atomic.Int32
	var maxConcurrent atomic.Int32

	makeSource := func(name string) *mockSource {
		return &mockSource{
			name:      name,
			authDelay: 50 * time.Millisecond,
			fetchData: json.RawMessage(`{}`),
		}
	}

	// We can't easily instrument mockSource for concurrency tracking here,
	// but we can verify both sources complete in ~50ms (parallel) not ~100ms (sequential).
	sources := []source.Source{makeSource("a"), makeSource("b")}
	_ = concurrent
	_ = maxConcurrent

	start := time.Now()
	_, err := source.Run(t.Context(), sources, 8*time.Hour)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Parallel auth should complete in ~50ms, not ~100ms
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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -race -v ./internal/source/...`
Expected: FAIL — types not defined

- [ ] **Step 3: Write the implementation**

```go
// internal/source/source.go
package source

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"
)

// Source gathers activity data from a single external service.
type Source interface {
	Name() string
	CheckAuth(ctx context.Context) error
	RefreshAuth(ctx context.Context) error
	Fetch(ctx context.Context, since time.Duration) (json.RawMessage, error)
}

// Result is the top-level output envelope.
type Result struct {
	FetchedAt string                        `json:"fetched_at"`
	Since     string                        `json:"since"`
	Sources   map[string]json.RawMessage    `json:"sources"`
}

// RunError collects per-source errors.
type RunError struct {
	Errors []SourceError
}

// SourceError describes a failure in a single source.
type SourceError struct {
	Source string `json:"source"`
	Phase  string `json:"phase"`
	Err    string `json:"error"`
}

func (e *RunError) Error() string {
	msgs := make([]string, len(e.Errors))
	for i, se := range e.Errors {
		msgs[i] = fmt.Sprintf("%s (%s): %s", se.Source, se.Phase, se.Err)
	}
	return strings.Join(msgs, "; ")
}

// Run orchestrates auth checking and data fetching across all sources.
//
//  1. Check auth for all sources in parallel.
//  2. For any that fail, run RefreshAuth sequentially (may open browser).
//  3. Re-check those sources.
//  4. If any still fail, return error with details.
//  5. Fetch all sources in parallel.
//  6. If any fetch fails, return error with details.
func Run(ctx context.Context, sources []Source, since time.Duration) (Result, error) {
	result := Result{
		FetchedAt: time.Now().UTC().Format(time.RFC3339),
		Since:     since.String(),
		Sources:   make(map[string]json.RawMessage),
	}

	if len(sources) == 0 {
		return result, nil
	}

	// Phase 1: Check auth in parallel
	authFailed := checkAuthParallel(ctx, sources)

	// Phase 2: Refresh failed sources sequentially
	if len(authFailed) > 0 {
		for _, s := range authFailed {
			if err := s.RefreshAuth(ctx); err != nil {
				continue
			}
		}
		// Phase 3: Re-check previously failed sources
		stillFailed := checkAuthParallel(ctx, authFailed)
		if len(stillFailed) > 0 {
			runErr := &RunError{}
			for _, s := range stillFailed {
				runErr.Errors = append(runErr.Errors, SourceError{
					Source: s.Name(),
					Phase:  "auth",
					Err:    "authentication failed after refresh attempt",
				})
			}
			return result, runErr
		}
	}

	// Phase 4: Fetch all in parallel
	var mu sync.Mutex
	var fetchErrors []SourceError

	g, gctx := errgroup.WithContext(ctx)
	for _, s := range sources {
		g.Go(func() error {
			data, err := s.Fetch(gctx, since)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				fetchErrors = append(fetchErrors, SourceError{
					Source: s.Name(),
					Phase:  "fetch",
					Err:    err.Error(),
				})
				return err
			}
			result.Sources[s.Name()] = data
			return nil
		})
	}

	_ = g.Wait()

	if len(fetchErrors) > 0 {
		return result, &RunError{Errors: fetchErrors}
	}

	return result, nil
}

// checkAuthParallel checks auth for all sources concurrently.
// Returns the subset of sources whose auth check failed.
func checkAuthParallel(ctx context.Context, sources []Source) []Source {
	type authResult struct {
		source Source
		err    error
	}

	results := make([]authResult, len(sources))
	var wg sync.WaitGroup

	for i, s := range sources {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results[i] = authResult{source: s, err: s.CheckAuth(ctx)}
		}()
	}
	wg.Wait()

	var failed []Source
	for _, r := range results {
		if r.err != nil {
			failed = append(failed, r.source)
		}
	}
	return failed
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test -race -v ./internal/source/...`
Expected: all 5 tests PASS

- [ ] **Step 5: Commit**

```bash
git add internal/source/source.go internal/source/source_test.go
git commit -m "feat: add Source interface and Run orchestrator"
```

---

### Task 4: GitHub source

**Files:**
- Create: `internal/source/github.go`
- Create: `internal/source/github_test.go`

- [ ] **Step 1: Write the failing tests**

```go
// internal/source/github_test.go
package source_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/dakaneye/kora/internal/source"
)

func TestGitHub_Name(t *testing.T) {
	gh := source.NewGitHub(nil)
	if gh.Name() != "github" {
		t.Errorf("name = %q, want %q", gh.Name(), "github")
	}
}

func TestGitHub_CheckAuth_Success(t *testing.T) {
	runner := &fakeRunner{
		results: map[string]fakeResult{
			"gh auth status": {stdout: "Logged in to github.com"},
		},
	}
	gh := source.NewGitHub(runner)
	if err := gh.CheckAuth(t.Context()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGitHub_CheckAuth_Failure(t *testing.T) {
	runner := &fakeRunner{
		results: map[string]fakeResult{
			"gh auth status": {err: "not logged in"},
		},
	}
	gh := source.NewGitHub(runner)
	if err := gh.CheckAuth(t.Context()); err == nil {
		t.Fatal("expected error for failed auth")
	}
}

func TestGitHub_Fetch(t *testing.T) {
	prs := `[{"number":1,"title":"fix bug"}]`
	issues := `[{"number":2,"title":"add feature"}]`
	runner := &fakeRunner{
		results: map[string]fakeResult{
			"gh search prs --review-requested=@me":  {stdout: prs},
			"gh search prs --author=@me":            {stdout: prs},
			"gh search issues --assignee=@me":       {stdout: issues},
			"gh search prs --commenter=@me":         {stdout: prs},
		},
	}
	gh := source.NewGitHub(runner)
	data, err := gh.Fetch(t.Context(), 8*time.Hour)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result map[string]json.RawMessage
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	for _, key := range []string{"review_requests", "authored_prs", "assigned_issues", "commented_prs"} {
		if _, ok := result[key]; !ok {
			t.Errorf("missing key %q in output", key)
		}
	}
}
```

We need a `fakeRunner` to mock subprocess calls. Add it to a shared test helper:

```go
// internal/source/runner_test.go
package source_test

import (
	"context"
	"errors"
	"strings"

	"github.com/dakaneye/kora/internal/exec"
)

type fakeResult struct {
	stdout string
	stderr string
	err    string
}

// fakeRunner mocks exec.Run for testing. Keys are matched by prefix against
// the command string (name + " " + args joined by " ").
type fakeRunner struct {
	results map[string]fakeResult
}

func (f *fakeRunner) Run(ctx context.Context, name string, args ...string) (exec.Result, error) {
	cmd := name + " " + strings.Join(args, " ")
	for prefix, fr := range f.results {
		if strings.HasPrefix(cmd, prefix) {
			r := exec.Result{Stdout: fr.stdout, Stderr: fr.stderr}
			if fr.err != "" {
				r.ExitCode = 1
				return r, errors.New(fr.err)
			}
			return r, nil
		}
	}
	return exec.Result{ExitCode: 1}, errors.New("fakeRunner: no match for " + cmd)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -race -v ./internal/source/... -run TestGitHub`
Expected: FAIL — `NewGitHub` not defined

- [ ] **Step 3: Write the implementation**

First, define the `Runner` interface in `exec.go` so sources can accept a mockable dependency:

Add to `internal/exec/exec.go`:

```go
// Runner abstracts subprocess execution for testing.
type Runner interface {
	Run(ctx context.Context, name string, args ...string) (Result, error)
}

// DefaultRunner uses real os/exec.
type DefaultRunner struct{}

func (d *DefaultRunner) Run(ctx context.Context, name string, args ...string) (Result, error) {
	return Run(ctx, name, args...)
}
```

Then write the GitHub source:

```go
// internal/source/github.go
package source

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/dakaneye/kora/internal/exec"
)

// GitHub fetches activity from GitHub via the gh CLI.
type GitHub struct {
	runner exec.Runner
}

func NewGitHub(runner exec.Runner) *GitHub {
	if runner == nil {
		runner = &exec.DefaultRunner{}
	}
	return &GitHub{runner: runner}
}

func (g *GitHub) Name() string { return "github" }

func (g *GitHub) CheckAuth(ctx context.Context) error {
	_, err := g.runner.Run(ctx, "gh", "auth", "status")
	if err != nil {
		return fmt.Errorf("github auth check: %w", err)
	}
	return nil
}

func (g *GitHub) RefreshAuth(ctx context.Context) error {
	_, err := g.runner.Run(ctx, "gh", "auth", "refresh")
	if err != nil {
		return fmt.Errorf("github auth refresh: %w", err)
	}
	return nil
}

func (g *GitHub) Fetch(ctx context.Context, since time.Duration) (json.RawMessage, error) {
	cutoff := time.Now().Add(-since).Format("2006-01-02")
	jsonFields := "number,title,url,state,updatedAt,repository,author,labels,createdAt"

	type subQuery struct {
		key  string
		args []string
	}

	queries := []subQuery{
		{"review_requests", []string{"search", "prs", "--review-requested=@me", "--updated=>=" + cutoff, "--json", jsonFields, "--limit", "100"}},
		{"authored_prs", []string{"search", "prs", "--author=@me", "--updated=>=" + cutoff, "--json", jsonFields, "--limit", "100"}},
		{"assigned_issues", []string{"search", "issues", "--assignee=@me", "--updated=>=" + cutoff, "--json", jsonFields, "--limit", "100"}},
		{"commented_prs", []string{"search", "prs", "--commenter=@me", "--updated=>=" + cutoff, "--json", jsonFields, "--limit", "100"}},
	}

	var mu sync.Mutex
	results := make(map[string]json.RawMessage)
	var fetchErr error

	var wg sync.WaitGroup
	for _, q := range queries {
		wg.Add(1)
		go func() {
			defer wg.Done()
			result, err := g.runner.Run(ctx, "gh", q.args...)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				if fetchErr == nil {
					fetchErr = fmt.Errorf("github %s: %w", q.key, err)
				}
				return
			}
			results[q.key] = json.RawMessage(result.Stdout)
		}()
	}
	wg.Wait()

	if fetchErr != nil {
		return nil, fetchErr
	}

	data, err := json.Marshal(results)
	if err != nil {
		return nil, fmt.Errorf("github marshal: %w", err)
	}
	return data, nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test -race -v ./internal/source/... -run TestGitHub`
Expected: all 3 tests PASS

- [ ] **Step 5: Commit**

```bash
git add internal/exec/exec.go internal/source/github.go internal/source/github_test.go internal/source/runner_test.go
git commit -m "feat: add GitHub source via gh CLI"
```

---

### Task 5: Gmail source

**Files:**
- Create: `internal/source/gmail.go`
- Create: `internal/source/gmail_test.go`

- [ ] **Step 1: Write the failing tests**

```go
// internal/source/gmail_test.go
package source_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/dakaneye/kora/internal/source"
)

func TestGmail_Name(t *testing.T) {
	gm := source.NewGmail(nil)
	if gm.Name() != "gmail" {
		t.Errorf("name = %q, want %q", gm.Name(), "gmail")
	}
}

func TestGmail_CheckAuth_Success(t *testing.T) {
	runner := &fakeRunner{
		results: map[string]fakeResult{
			"gws auth status": {stdout: `{"token_valid": true}`},
		},
	}
	gm := source.NewGmail(runner)
	if err := gm.CheckAuth(t.Context()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGmail_CheckAuth_Failure(t *testing.T) {
	runner := &fakeRunner{
		results: map[string]fakeResult{
			"gws auth status": {err: "token expired"},
		},
	}
	gm := source.NewGmail(runner)
	if err := gm.CheckAuth(t.Context()); err == nil {
		t.Fatal("expected error for expired auth")
	}
}

func TestGmail_Fetch(t *testing.T) {
	listResponse := `{"messages":[{"id":"msg1"},{"id":"msg2"}]}`
	msg1 := `{"id":"msg1","payload":{"headers":[{"name":"From","value":"alice@example.com"},{"name":"Subject","value":"Hello"},{"name":"Date","value":"Sat, 29 Mar 2026 08:00:00 +0000"}]}}`
	msg2 := `{"id":"msg2","payload":{"headers":[{"name":"From","value":"bob@example.com"},{"name":"Subject","value":"Meeting"},{"name":"Date","value":"Sat, 29 Mar 2026 09:00:00 +0000"}]}}`

	runner := &fakeRunner{
		results: map[string]fakeResult{
			"gws gmail users messages list": {stdout: listResponse},
			"gws gmail users messages get":  {stdout: msg1}, // simplified: returns same for any get
		},
	}
	// For a real test we'd need per-message routing, but this validates the structure
	_ = msg2
	gm := source.NewGmail(runner)
	data, err := gm.Fetch(t.Context(), 8*time.Hour)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result map[string]json.RawMessage
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if _, ok := result["messages"]; !ok {
		t.Error("missing 'messages' key in output")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -race -v ./internal/source/... -run TestGmail`
Expected: FAIL — `NewGmail` not defined

- [ ] **Step 3: Write the implementation**

```go
// internal/source/gmail.go
package source

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/dakaneye/kora/internal/exec"
)

// Gmail fetches email activity via the gws CLI.
type Gmail struct {
	runner exec.Runner
}

func NewGmail(runner exec.Runner) *Gmail {
	if runner == nil {
		runner = &exec.DefaultRunner{}
	}
	return &Gmail{runner: runner}
}

func (g *Gmail) Name() string { return "gmail" }

func (g *Gmail) CheckAuth(ctx context.Context) error {
	_, err := g.runner.Run(ctx, "gws", "auth", "status")
	if err != nil {
		return fmt.Errorf("gmail auth check: %w", err)
	}
	return nil
}

func (g *Gmail) RefreshAuth(ctx context.Context) error {
	_, err := g.runner.Run(ctx, "gws", "auth", "login")
	if err != nil {
		return fmt.Errorf("gmail auth refresh: %w", err)
	}
	return nil
}

func (g *Gmail) Fetch(ctx context.Context, since time.Duration) (json.RawMessage, error) {
	after := time.Now().Add(-since).Format("2006/01/02")
	query := fmt.Sprintf("is:unread after:%s", after)
	listParams := fmt.Sprintf(`{"userId":"me","q":"%s","maxResults":100}`, query)

	// Phase 1: List message IDs
	listResult, err := g.runner.Run(ctx, "gws", "gmail", "users", "messages", "list", "--params", listParams)
	if err != nil {
		return nil, fmt.Errorf("gmail list: %w", err)
	}

	var listResp struct {
		Messages []struct {
			ID string `json:"id"`
		} `json:"messages"`
	}
	if err := json.Unmarshal([]byte(listResult.Stdout), &listResp); err != nil {
		return nil, fmt.Errorf("gmail parse list: %w", err)
	}

	if len(listResp.Messages) == 0 {
		empty, _ := json.Marshal(map[string]any{"messages": []any{}})
		return empty, nil
	}

	// Phase 2: Fetch each message's metadata in parallel
	var mu sync.Mutex
	messages := make([]json.RawMessage, 0, len(listResp.Messages))
	var fetchErr error

	var wg sync.WaitGroup
	for _, msg := range listResp.Messages {
		wg.Add(1)
		go func() {
			defer wg.Done()
			getParams := fmt.Sprintf(`{"userId":"me","id":"%s","format":"metadata","metadataHeaders":["From","Subject","Date"]}`, msg.ID)
			getResult, err := g.runner.Run(ctx, "gws", "gmail", "users", "messages", "get", "--params", getParams)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				if fetchErr == nil {
					fetchErr = fmt.Errorf("gmail get %s: %w", msg.ID, err)
				}
				return
			}
			messages = append(messages, json.RawMessage(getResult.Stdout))
		}()
	}
	wg.Wait()

	if fetchErr != nil {
		return nil, fetchErr
	}

	data, err := json.Marshal(map[string]any{"messages": messages})
	if err != nil {
		return nil, fmt.Errorf("gmail marshal: %w", err)
	}
	return data, nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test -race -v ./internal/source/... -run TestGmail`
Expected: all 3 tests PASS

- [ ] **Step 5: Commit**

```bash
git add internal/source/gmail.go internal/source/gmail_test.go
git commit -m "feat: add Gmail source via gws CLI"
```

---

### Task 6: Calendar source

**Files:**
- Create: `internal/source/calendar.go`
- Create: `internal/source/calendar_test.go`

- [ ] **Step 1: Write the failing tests**

```go
// internal/source/calendar_test.go
package source_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/dakaneye/kora/internal/source"
)

func TestCalendar_Name(t *testing.T) {
	cal := source.NewCalendar(nil)
	if cal.Name() != "calendar" {
		t.Errorf("name = %q, want %q", cal.Name(), "calendar")
	}
}

func TestCalendar_CheckAuth_Success(t *testing.T) {
	runner := &fakeRunner{
		results: map[string]fakeResult{
			"gws auth status": {stdout: `{"token_valid": true}`},
		},
	}
	cal := source.NewCalendar(runner)
	if err := cal.CheckAuth(t.Context()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCalendar_Fetch(t *testing.T) {
	eventsJSON := `{"items":[{"summary":"Standup","start":{"dateTime":"2026-03-29T09:00:00Z"}}]}`
	runner := &fakeRunner{
		results: map[string]fakeResult{
			"gws auth status":           {stdout: `{"token_valid": true}`},
			"gws calendar events list":  {stdout: eventsJSON},
		},
	}
	cal := source.NewCalendar(runner)
	data, err := cal.Fetch(t.Context(), 8*time.Hour)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result map[string]json.RawMessage
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if _, ok := result["events"]; !ok {
		t.Error("missing 'events' key in output")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -race -v ./internal/source/... -run TestCalendar`
Expected: FAIL — `NewCalendar` not defined

- [ ] **Step 3: Write the implementation**

```go
// internal/source/calendar.go
package source

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/dakaneye/kora/internal/exec"
)

// Calendar fetches calendar events via the gws CLI.
type Calendar struct {
	runner exec.Runner
}

func NewCalendar(runner exec.Runner) *Calendar {
	if runner == nil {
		runner = &exec.DefaultRunner{}
	}
	return &Calendar{runner: runner}
}

func (c *Calendar) Name() string { return "calendar" }

func (c *Calendar) CheckAuth(ctx context.Context) error {
	_, err := c.runner.Run(ctx, "gws", "auth", "status")
	if err != nil {
		return fmt.Errorf("calendar auth check: %w", err)
	}
	return nil
}

func (c *Calendar) RefreshAuth(ctx context.Context) error {
	_, err := c.runner.Run(ctx, "gws", "auth", "login")
	if err != nil {
		return fmt.Errorf("calendar auth refresh: %w", err)
	}
	return nil
}

func (c *Calendar) Fetch(ctx context.Context, since time.Duration) (json.RawMessage, error) {
	now := time.Now().UTC()
	timeMin := now.Add(-since).Format(time.RFC3339)
	timeMax := now.Format(time.RFC3339)
	params := fmt.Sprintf(`{"calendarId":"primary","timeMin":"%s","timeMax":"%s","singleEvents":true,"orderBy":"startTime"}`, timeMin, timeMax)

	result, err := c.runner.Run(ctx, "gws", "calendar", "events", "list", "--params", params)
	if err != nil {
		return nil, fmt.Errorf("calendar fetch: %w", err)
	}

	// Wrap raw response under "events" key
	var raw json.RawMessage
	if err := json.Unmarshal([]byte(result.Stdout), &raw); err != nil {
		return nil, fmt.Errorf("calendar parse: %w", err)
	}

	data, err := json.Marshal(map[string]json.RawMessage{"events": raw})
	if err != nil {
		return nil, fmt.Errorf("calendar marshal: %w", err)
	}
	return data, nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test -race -v ./internal/source/... -run TestCalendar`
Expected: all 2 tests PASS

- [ ] **Step 5: Commit**

```bash
git add internal/source/calendar.go internal/source/calendar_test.go
git commit -m "feat: add Calendar source via gws CLI"
```

---

### Task 7: Linear source

**Files:**
- Create: `internal/source/linear.go`
- Create: `internal/source/linear_test.go`

- [ ] **Step 1: Write the failing tests**

```go
// internal/source/linear_test.go
package source_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/dakaneye/kora/internal/source"
)

func TestLinear_Name(t *testing.T) {
	lin := source.NewLinear(nil)
	if lin.Name() != "linear" {
		t.Errorf("name = %q, want %q", lin.Name(), "linear")
	}
}

func TestLinear_CheckAuth_Success(t *testing.T) {
	runner := &fakeRunner{
		results: map[string]fakeResult{
			"linear auth whoami": {stdout: `sam@netboxlabs.com`},
		},
	}
	lin := source.NewLinear(runner)
	if err := lin.CheckAuth(t.Context()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLinear_CheckAuth_Failure(t *testing.T) {
	runner := &fakeRunner{
		results: map[string]fakeResult{
			"linear auth whoami": {err: "not authenticated"},
		},
	}
	lin := source.NewLinear(runner)
	if err := lin.CheckAuth(t.Context()); err == nil {
		t.Fatal("expected error for failed auth")
	}
}

func TestLinear_Fetch(t *testing.T) {
	assignedJSON := `{"data":{"viewer":{"assignedIssues":{"nodes":[{"identifier":"ENG-123","title":"Fix bug"}]}}}}`
	cyclesJSON := `{"data":{"team":{"cycles":{"nodes":[{"number":5,"startsAt":"2026-03-24","endsAt":"2026-03-31"}]}}}}`
	commentedJSON := `{"data":{"issueSearch":{"nodes":[{"identifier":"ENG-456","title":"Review needed"}]}}}`
	completedJSON := `{"data":{"issueSearch":{"nodes":[{"identifier":"ENG-789","title":"Done thing"}]}}}`

	runner := &fakeRunner{
		results: map[string]fakeResult{
			"linear api { viewer { assignedIssues": {stdout: assignedJSON},
			"linear api { teams":                   {stdout: cyclesJSON},
			"linear api { issueSearch(filter":       {stdout: commentedJSON}, // matches both commented and completed
		},
	}
	// The fakeRunner prefix matching is simplified for this test.
	// In practice we'll need more precise matching for the two different issueSearch queries.
	lin := source.NewLinear(runner)
	data, err := lin.Fetch(t.Context(), 8*time.Hour)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result map[string]json.RawMessage
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	// Verify at least some keys exist
	if len(result) == 0 {
		t.Error("expected non-empty result")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -race -v ./internal/source/... -run TestLinear`
Expected: FAIL — `NewLinear` not defined

- [ ] **Step 3: Write the implementation**

```go
// internal/source/linear.go
package source

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/dakaneye/kora/internal/exec"
)

// Linear fetches activity from Linear via the linear CLI and its GraphQL API passthrough.
type Linear struct {
	runner exec.Runner
}

func NewLinear(runner exec.Runner) *Linear {
	if runner == nil {
		runner = &exec.DefaultRunner{}
	}
	return &Linear{runner: runner}
}

func (l *Linear) Name() string { return "linear" }

func (l *Linear) CheckAuth(ctx context.Context) error {
	_, err := l.runner.Run(ctx, "linear", "auth", "whoami")
	if err != nil {
		return fmt.Errorf("linear auth check: %w", err)
	}
	return nil
}

func (l *Linear) RefreshAuth(ctx context.Context) error {
	_, err := l.runner.Run(ctx, "linear", "auth", "login")
	if err != nil {
		return fmt.Errorf("linear auth refresh: %w", err)
	}
	return nil
}

func (l *Linear) Fetch(ctx context.Context, since time.Duration) (json.RawMessage, error) {
	cutoff := time.Now().Add(-since).Format(time.RFC3339)

	type subQuery struct {
		key   string
		query string
	}

	queries := []subQuery{
		{
			key: "assigned_issues",
			query: `{
				viewer {
					assignedIssues(first: 100, orderBy: updatedAt) {
						nodes {
							identifier title state { name type } priority priorityLabel
							url updatedAt createdAt completedAt
							team { name key } project { name }
							labels { nodes { name } }
						}
					}
				}
			}`,
		},
		{
			key: "cycles",
			query: `{
				teams {
					nodes {
						name key
						cycles(first: 5, orderBy: createdAt) {
							nodes {
								number name startsAt endsAt
								progress completedScopeCount scopeCount
							}
						}
					}
				}
			}`,
		},
		{
			key: "commented_issues",
			query: fmt.Sprintf(`{
				issueSearch(filter: { comments: { user: { isMe: { eq: true } }, updatedAt: { gte: "%s" } } }, first: 100) {
					nodes {
						identifier title state { name type } priority priorityLabel
						url updatedAt
						team { name key }
					}
				}
			}`, cutoff),
		},
		{
			key: "completed_issues",
			query: fmt.Sprintf(`{
				issueSearch(filter: { completedAt: { gte: "%s" }, assignee: { isMe: { eq: true } } }, first: 100) {
					nodes {
						identifier title state { name type } priority priorityLabel
						url updatedAt completedAt
						team { name key } project { name }
					}
				}
			}`, cutoff),
		},
	}

	var mu sync.Mutex
	results := make(map[string]json.RawMessage)
	var fetchErr error

	var wg sync.WaitGroup
	for _, q := range queries {
		wg.Add(1)
		go func() {
			defer wg.Done()
			result, err := l.runner.Run(ctx, "linear", "api", q.query)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				if fetchErr == nil {
					fetchErr = fmt.Errorf("linear %s: %w", q.key, err)
				}
				return
			}
			results[q.key] = json.RawMessage(result.Stdout)
		}()
	}
	wg.Wait()

	if fetchErr != nil {
		return nil, fetchErr
	}

	data, err := json.Marshal(results)
	if err != nil {
		return nil, fmt.Errorf("linear marshal: %w", err)
	}
	return data, nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test -race -v ./internal/source/... -run TestLinear`
Expected: all 3 tests PASS

- [ ] **Step 5: Commit**

```bash
git add internal/source/linear.go internal/source/linear_test.go
git commit -m "feat: add Linear source via linear CLI"
```

---

### Task 8: CLI entry point (main.go)

**Files:**
- Create: `cmd/kora/main.go`

- [ ] **Step 1: Write the implementation**

No TDD for main — it's a thin wiring layer. We'll validate it via e2e tests in Task 10.

```go
// cmd/kora/main.go
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"time"

	"github.com/dakaneye/kora/internal/source"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	sinceStr := flag.String("since", "16h", "time window to look back (e.g. 8h, 7d)")
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Printf("kora %s (%s) built %s\n", version, commit, date)
		os.Exit(0)
	}

	since, err := time.ParseDuration(*sinceStr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "invalid --since value %q: %v\n", *sinceStr, err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	sources := []source.Source{
		source.NewGitHub(nil),
		source.NewGmail(nil),
		source.NewCalendar(nil),
		source.NewLinear(nil),
	}

	result, err := source.Run(ctx, sources, since)
	if err != nil {
		errOutput := map[string]any{"errors": err.Error()}
		errJSON, _ := json.Marshal(errOutput)
		fmt.Fprintln(os.Stderr, string(errJSON))
		os.Exit(1)
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(result); err != nil {
		fmt.Fprintf(os.Stderr, "encoding output: %v\n", err)
		os.Exit(1)
	}
}
```

- [ ] **Step 2: Verify it compiles**

Run: `go build ./cmd/kora`
Expected: no errors

- [ ] **Step 3: Verify --version works**

Run: `go run ./cmd/kora --version`
Expected: `kora dev (none) built unknown`

- [ ] **Step 4: Commit**

```bash
git add cmd/kora/main.go
git commit -m "feat: add CLI entry point"
```

---

### Task 9: Integration tests (real CLI tools)

**Files:**
- Create: `tests/integration/sources_test.go`

These tests call real CLI tools and require auth to be set up. They are build-tagged so they only run with `make test-integration`.

- [ ] **Step 1: Write the integration tests**

```go
//go:build integration

// tests/integration/sources_test.go
package integration_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/dakaneye/kora/internal/source"
)

func TestGitHub_Integration(t *testing.T) {
	gh := source.NewGitHub(nil)
	if err := gh.CheckAuth(t.Context()); err != nil {
		t.Skipf("gh auth not configured: %v", err)
	}

	data, err := gh.Fetch(t.Context(), 24*time.Hour)
	if err != nil {
		t.Fatalf("fetch failed: %v", err)
	}

	var result map[string]json.RawMessage
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	for _, key := range []string{"review_requests", "authored_prs", "assigned_issues", "commented_prs"} {
		if _, ok := result[key]; !ok {
			t.Errorf("missing key %q", key)
		}
	}
	t.Logf("github returned %d bytes", len(data))
}

func TestGmail_Integration(t *testing.T) {
	gm := source.NewGmail(nil)
	if err := gm.CheckAuth(t.Context()); err != nil {
		t.Skipf("gws auth not configured: %v", err)
	}

	data, err := gm.Fetch(t.Context(), 24*time.Hour)
	if err != nil {
		t.Fatalf("fetch failed: %v", err)
	}

	var result map[string]json.RawMessage
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if _, ok := result["messages"]; !ok {
		t.Error("missing 'messages' key")
	}
	t.Logf("gmail returned %d bytes", len(data))
}

func TestCalendar_Integration(t *testing.T) {
	cal := source.NewCalendar(nil)
	if err := cal.CheckAuth(t.Context()); err != nil {
		t.Skipf("gws auth not configured: %v", err)
	}

	data, err := cal.Fetch(t.Context(), 24*time.Hour)
	if err != nil {
		t.Fatalf("fetch failed: %v", err)
	}

	var result map[string]json.RawMessage
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if _, ok := result["events"]; !ok {
		t.Error("missing 'events' key")
	}
	t.Logf("calendar returned %d bytes", len(data))
}

func TestLinear_Integration(t *testing.T) {
	lin := source.NewLinear(nil)
	if err := lin.CheckAuth(t.Context()); err != nil {
		t.Skipf("linear auth not configured: %v", err)
	}

	data, err := lin.Fetch(t.Context(), 7*24*time.Hour)
	if err != nil {
		t.Fatalf("fetch failed: %v", err)
	}

	var result map[string]json.RawMessage
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	t.Logf("linear returned %d bytes", len(data))
}
```

- [ ] **Step 2: Run integration tests**

Run: `make test-integration`
Expected: tests either PASS or SKIP (if CLI tools aren't authed)

- [ ] **Step 3: Commit**

```bash
git add tests/integration/sources_test.go
git commit -m "test: add integration tests for all sources"
```

---

### Task 10: End-to-end tests (compiled binary)

**Files:**
- Create: `tests/e2e/kora_test.go`

These tests build the actual `kora` binary and run it as a subprocess, validating the full pipeline.

- [ ] **Step 1: Write the e2e tests**

```go
//go:build e2e

// tests/e2e/kora_test.go
package e2e_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func binaryPath(t *testing.T) string {
	t.Helper()
	// Expect binary at bin/kora (built by make test-e2e)
	path := filepath.Join("..", "..", "bin", "kora")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("binary not found at %s: run 'make build' first", path)
	}
	abs, _ := filepath.Abs(path)
	return abs
}

func TestKora_Version(t *testing.T) {
	bin := binaryPath(t)
	cmd := exec.Command(bin, "--version")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("--version failed: %v", err)
	}
	if len(out) == 0 {
		t.Error("--version produced no output")
	}
	t.Logf("version: %s", out)
}

func TestKora_InvalidSince(t *testing.T) {
	bin := binaryPath(t)
	cmd := exec.Command(bin, "--since", "banana")
	err := cmd.Run()
	if err == nil {
		t.Error("expected non-zero exit for invalid --since")
	}
}

func TestKora_FullRun(t *testing.T) {
	bin := binaryPath(t)
	cmd := exec.Command(bin, "--since", "24h")
	out, err := cmd.Output()
	if err != nil {
		// If any source auth fails, this will exit non-zero — that's expected in CI
		t.Skipf("full run failed (likely auth): %v", err)
	}

	var result map[string]json.RawMessage
	if err := json.Unmarshal(out, &result); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	if _, ok := result["sources"]; !ok {
		t.Error("missing 'sources' key in output")
	}
	if _, ok := result["fetched_at"]; !ok {
		t.Error("missing 'fetched_at' key in output")
	}
	if _, ok := result["since"]; !ok {
		t.Error("missing 'since' key in output")
	}
	t.Logf("full run returned %d bytes", len(out))
}

func TestKora_OutputStructure(t *testing.T) {
	bin := binaryPath(t)
	cmd := exec.Command(bin, "--since", "1h")
	out, err := cmd.Output()
	if err != nil {
		t.Skipf("run failed (likely auth): %v", err)
	}

	var envelope struct {
		FetchedAt string                     `json:"fetched_at"`
		Since     string                     `json:"since"`
		Sources   map[string]json.RawMessage `json:"sources"`
	}
	if err := json.Unmarshal(out, &envelope); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	expectedSources := []string{"github", "gmail", "calendar", "linear"}
	for _, name := range expectedSources {
		if _, ok := envelope.Sources[name]; !ok {
			t.Errorf("missing source %q in output", name)
		}
	}

	if envelope.Since != "1h0m0s" {
		t.Errorf("since = %q, want %q", envelope.Since, "1h0m0s")
	}
}
```

- [ ] **Step 2: Run e2e tests**

Run: `make test-e2e`
Expected: version test PASS, full run tests either PASS or SKIP based on auth

- [ ] **Step 3: Commit**

```bash
git add tests/e2e/kora_test.go
git commit -m "test: add end-to-end tests for kora binary"
```

---

### Task 11: Update CLAUDE.md and README

**Files:**
- Modify: `CLAUDE.md`
- Modify: `README.md`

- [ ] **Step 1: Rewrite CLAUDE.md**

Replace the entire CLAUDE.md to reflect the new v2 architecture. Remove all EFA references, update directory structure, update commands, update agent recommendations. The new CLAUDE.md should describe:

- Project purpose (single-purpose activity gatherer)
- Directory structure (`cmd/kora/`, `internal/source/`, `internal/exec/`)
- Source interface contract
- Auth lifecycle
- How to run (`kora --since 8h`)
- Testing (`make test`, `make test-integration`, `make test-e2e`)
- No config file
- CLI tools required (`gh`, `gws`, `linear`)

- [ ] **Step 2: Rewrite README.md**

Simple README:
- What kora does (one paragraph)
- Prerequisites (Go 1.25+, `gh`, `gws`, `linear` CLIs)
- Install (`make install`)
- Usage (`kora --since 8h`)
- Output format (brief JSON example)
- Development (`make test`, `make lint`)

- [ ] **Step 3: Commit**

```bash
git add CLAUDE.md README.md
git commit -m "docs: update CLAUDE.md and README for v2 architecture"
```

---

### Task 12: Code review iteration

- [ ] **Step 1: Run /review-code**

Invoke the `/review-code` skill against the full codebase. Address all findings.

- [ ] **Step 2: Iterate until grade A**

Fix issues found by review. Re-run `/review-code`. Repeat until the review produces an A grade.

- [ ] **Step 3: Commit any fixes**

```bash
git add -A
git commit -m "refactor: address code review findings"
```

---

### Task 13: Public repo setup

- [ ] **Step 1: Run /public-repo-setup**

Invoke the `/public-repo-setup` skill. This configures:
- LICENSE file
- CONTRIBUTING.md
- GitHub Actions CI
- Release workflow
- Any other public-repo requirements

- [ ] **Step 2: Address any findings from setup**

Follow the skill's guidance to completion.

- [ ] **Step 3: Commit**

```bash
git add -A
git commit -m "chore: configure repo for public release"
```

---

### Task 14: Final verification

- [ ] **Step 1: Run full test suite**

```bash
make test
make test-integration
make test-e2e
```

All must pass.

- [ ] **Step 2: Run lint**

```bash
make lint
```

Must be clean.

- [ ] **Step 3: Final /review-code**

Run `/review-code` one more time to confirm A grade holds after public-repo-setup changes.

- [ ] **Step 4: Manual smoke test**

```bash
kora --since 8h | jq '.sources | keys'
```

Expected: `["calendar", "github", "gmail", "linear"]`
