# Kora v2: Activity Data Layer

## Purpose

Kora is a single-purpose CLI tool that gathers work activity from multiple data sources and outputs structured JSON. Claude (or any consumer) synthesizes the raw data into digests, briefs, and reports. Kora fetches; it does not filter, rank, or interpret.

## Decisions

| Decision | Answer |
|----------|--------|
| Interface | CLI only, no MCP, no subcommands -- just `kora` |
| Output | Raw passthrough from each source's CLI tool, Kora adds metadata only where useful and not duplicative |
| Sources | GitHub (`gh`), Gmail (`gws`), Calendar (`gws`), Linear (`linear-cli` + GraphQL API) |
| Parallelism | All sources in parallel, sub-parallelism within each source |
| Time window | `--since` duration only (e.g. `--since 8h`, `--since 7d`) |
| Errors | Exit non-zero if any enabled source fails |
| Config | None -- flags and CLI tool defaults |
| Auth | Delegate to each CLI tool; auto-retry auth once (may open browser) |
| Existing code | Greenfield rewrite, nearly nothing carried forward from v1 |
| Governance | Replace EFAs with in-repo skills as needed |

## Architecture

### Directory structure

```
cmd/kora/main.go              -- flag parsing, orchestrator, JSON output
internal/source/source.go     -- Source interface, Run function
internal/source/github.go     -- GitHub via gh CLI
internal/source/gmail.go      -- Gmail via gws CLI
internal/source/calendar.go   -- Calendar via gws CLI
internal/source/linear.go     -- Linear via linear-cli + GraphQL API
internal/exec/exec.go         -- subprocess helper (run cmd, capture stdout/stderr)
```

Plus tests for each file.

### Source interface

```go
type Source interface {
    Name() string
    CheckAuth(ctx context.Context) error
    RefreshAuth(ctx context.Context) error
    Fetch(ctx context.Context, since time.Duration) (json.RawMessage, error)
}
```

Each source implements these four methods. `CheckAuth` verifies credentials via the underlying CLI tool. `RefreshAuth` attempts to re-authenticate (may open a browser for interactive flows). `Fetch` returns raw JSON from the underlying tool(s).

Each source is responsible for translating the `since` duration into whatever format its underlying tool expects (e.g., `gh` search uses `updated:>YYYY-MM-DD`, `gws` uses ISO timestamps in query params, Linear GraphQL uses `updatedAt` filters).

### Orchestration (main.go)

```
1. Parse --since flag (default 16h)
2. Create all four sources
3. Check auth for all sources in parallel
4. Any auth failures -> run their refresh flows sequentially (may open browser)
5. Re-check those sources
6. If any still fail -> report errors to stderr, exit 1
7. Fetch all sources in parallel (with sub-parallelism within each source)
8. If any fetch fails -> report errors to stderr, exit 1
9. Assemble envelope JSON, print to stdout, exit 0
```

## Per-source details

### GitHub (`gh` CLI)

**Auth check:** `gh auth status`
**Auth refresh:** `gh auth refresh`

**Parallel sub-calls (all via `gh`):**
- `review_requests` -- `gh search prs --review-requested=@me --json ...`
- `authored_prs` -- `gh search prs --author=@me --json ...`
- `assigned_issues` -- `gh search issues --assignee=@me --json ...`
- `commented_prs` -- `gh search prs --commenter=@me --json ...`

**Output:** sub-keyed by query type.

### Gmail (`gws` CLI)

**Auth check:** `gws auth status`
**Auth refresh:** `gws auth login`

**Fetch strategy:**
1. `gws gmail users messages list` with time-windowed query -- returns message IDs
2. `gws gmail users messages get` for each ID in parallel -- returns metadata (From, Subject, Date)

**Output:** array of message metadata.

### Calendar (`gws` CLI)

**Auth check:** `gws auth status` (shared with Gmail)
**Auth refresh:** `gws auth login` (shared with Gmail)

**Fetch:** `gws calendar events list` with `timeMin`/`timeMax` derived from `--since`.

**Output:** raw events array.

### Linear (hybrid: `linear-cli` + GraphQL API)

**Auth check:** `linear auth whoami`
**Auth refresh:** `linear auth login`
**Token for GraphQL:** `linear auth token` (pipes stored token to stdout)

**CLI calls:**
- `linear issue list --all-states --json` -- assigned issues
- `linear cycle list --json` -- cycle info

**GraphQL API calls** (using token from `linear auth token`):
- Issues I commented on (no CLI filter for this)
- Completed issues filtered by time window (CLI has no date filter)

**Output:** sub-keyed (`assigned_issues`, `cycles`, `commented_issues`, `completed_issues`).

## Output contract

### Success (stdout, exit 0)

```json
{
  "fetched_at": "2026-03-29T08:00:00Z",
  "since": "16h",
  "sources": {
    "github": {
      "review_requests": [],
      "authored_prs": [],
      "assigned_issues": [],
      "commented_prs": []
    },
    "gmail": {
      "messages": []
    },
    "calendar": {
      "events": []
    },
    "linear": {
      "assigned_issues": [],
      "cycles": [],
      "commented_issues": [],
      "completed_issues": []
    }
  }
}
```

Each source controls its own shape. Items within each array are raw JSON from the underlying CLI tool or API. Kora does not normalize or transform the data.

### Failure (stderr, exit 1)

```json
{
  "errors": [
    {"source": "calendar", "phase": "auth", "error": "gws auth expired and refresh failed"},
    {"source": "linear", "phase": "fetch", "error": "linear-cli not found in PATH"}
  ]
}
```

Stdout gets nothing on failure.

## Error handling

- Exit non-zero if any source fails (auth or fetch)
- Auth check runs in parallel first; failures trigger sequential refresh (may open browser)
- After one refresh attempt, if auth still fails, report and exit
- Fetch errors (timeouts, API errors, malformed responses) reported with source and phase context

## Auth lifecycle

```
1. Check all sources in parallel
2. Collect failures
3. If any failed:
   a. Run refresh for each failed source sequentially (browser may open)
   b. Re-check those sources
   c. If any still fail -> exit 1 with error details
4. All auth valid -> proceed to fetch
```

Sequential refresh only runs for sources that actually need it. Fast path (all auth valid) has zero sequential work.

## What gets deleted from v1

- `internal/auth/` -- CLI tools handle auth
- `internal/models/` -- no Event model, raw passthrough
- `internal/datasources/` -- replaced by `internal/source/`
- `internal/config/` -- no config file
- `internal/output/` -- just print JSON to stdout
- `internal/storage/` -- no database
- `cmd/kora/` subcommands -- single command, no subcommands
- `specs/efas/` -- replaced by in-repo skills as needed
- `tests/integration/` -- rewritten for new architecture

## What carries forward

- `go.mod` module path
- `Makefile` (updated for new structure)
- Core principle from EFA 0004: Kora = data layer, Claude = intelligence layer

## Future considerations (not in scope)

- Team activity (different queries to same sources, driven by flags)
- Additional sources (Slack, Notion, etc.)
- Config file (if flag count grows unwieldy)
- MCP wrapper (if CLI-via-Bash proves insufficient)
