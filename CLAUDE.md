# Kora - Claude Development Guide

## Project Overview

Kora is a single-purpose Go CLI that gathers work activity from multiple sources (GitHub, Gmail, Calendar, Linear) and outputs structured JSON. Claude consumes this data to produce morning briefs, weekly digests, and EOD captures. Kora fetches; Claude synthesizes.

## Architecture

Kora delegates to existing CLI tools instead of reimplementing API clients:
- **GitHub**: `gh` CLI
- **Gmail/Calendar**: `gws` CLI (Google Workspace CLI)
- **Linear**: `linear` CLI (schpet/linear-cli) + `linear api` for GraphQL

### Directory Structure

```
cmd/kora/main.go           # CLI entry point — flag parsing, source wiring, JSON output
internal/
  source/                   # Source interface and all implementations
    source.go               # Source interface, Result type, Run orchestrator
    github.go               # GitHub via gh CLI
    gmail.go                # Gmail via gws CLI
    calendar.go             # Calendar via gws CLI
    linear.go               # Linear via linear CLI + GraphQL
  exec/                     # Subprocess execution
    exec.go                 # Run function, Runner interface
tests/
  integration/              # Real CLI tool tests (build-tagged)
  e2e/                      # Compiled binary tests (build-tagged)
docs/superpowers/
  specs/                    # Design specs
  plans/                    # Implementation plans
```

### Source Interface

```go
type Source interface {
    Name() string
    CheckAuth(ctx context.Context) error
    RefreshAuth(ctx context.Context) error
    Fetch(ctx context.Context, since time.Duration) (json.RawMessage, error)
}
```

### Auth Lifecycle

1. Check all sources in parallel
2. Failed sources get sequential refresh (may open browser)
3. Re-check refreshed sources
4. If any still fail → exit 1

### Output Contract

Stdout: JSON envelope with raw data from each source (not normalized)
Stderr: JSON errors on failure
Exit 0 on success, exit 1 on any source failure

## Code Standards

### Go Conventions
- Context as first parameter
- Error wrapping with `%w`
- `sync.WaitGroup` for parallel sub-queries within each source
- `errgroup` for top-level source orchestration
- Table-driven tests

### Project Rules
- **Kora is a data layer** — no filtering, ranking, or relevance scoring
- **Raw passthrough** — each source returns its CLI tool's JSON, Kora doesn't normalize
- **Kora adds metadata only where useful and not duplicative**
- **CLI delegation** — never store credentials, never make raw HTTP calls (except Linear GraphQL via `linear api`)
- **No config file** — flags and CLI tool defaults only
- **macOS only** — don't add cross-platform abstractions

### Testing
- Unit tests mock the `exec.Runner` interface
- Integration tests (build-tagged) call real CLI tools
- E2E tests (build-tagged) run the compiled binary
- All tests must pass before committing

## Quick Reference

```bash
# Build
make build

# Test
make test              # unit tests
make test-integration  # requires CLI tools + auth
make test-e2e          # builds binary, runs it

# Lint
make lint

# Run
kora --since 8h        # last 8 hours
kora --since 168h      # last 7 days
kora --version

# Install
make install           # installs to ~/.local/bin
```

## Required CLI Tools

| Tool | Install | Auth |
|------|---------|------|
| `gh` | `brew install gh` | `gh auth login` |
| `gws` | See github.com/googleworkspace/cli | `gws auth login` |
| `linear` | `brew install schpet/tap/linear` | `linear auth login` |
