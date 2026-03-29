# Kora

Single-purpose CLI that gathers work activity from GitHub, Gmail, Calendar, and Linear into structured JSON. Designed as a data layer for AI-powered work automation — Claude (or any consumer) synthesizes the raw data into morning briefs, weekly digests, and status reports.

## Prerequisites

- Go 1.25+
- [`gh`](https://cli.github.com/) — GitHub CLI
- [`gws`](https://github.com/googleworkspace/cli) — Google Workspace CLI
- [`linear`](https://github.com/schpet/linear-cli) — Linear CLI

Each tool must be installed and authenticated before running Kora.

## Install

```bash
make install  # builds and installs to ~/.local/bin
```

## Usage

```bash
kora --since 8h    # activity from last 8 hours
kora --since 168h  # activity from last 7 days (week)
```

Output is JSON to stdout:

```json
{
  "fetched_at": "2026-03-29T08:00:00Z",
  "since": "8h0m0s",
  "sources": {
    "github": { "review_requests": [...], "authored_prs": [...], ... },
    "gmail": { "messages": [...] },
    "calendar": { "events": {...} },
    "linear": { "assigned_issues": {...}, "cycles": {...}, ... }
  }
}
```

Exits 0 on success, 1 if any source fails. Errors go to stderr as JSON.

## Development

```bash
make test              # unit tests
make test-integration  # real CLI tools (requires auth)
make test-e2e          # compiled binary tests
make lint              # golangci-lint
```

## How It Works

Kora delegates to existing CLI tools rather than reimplementing API clients:

| Source | CLI Tool | Data |
|--------|----------|------|
| GitHub | `gh` | PRs to review, authored PRs, assigned issues |
| Gmail | `gws` | Unread messages with metadata |
| Calendar | `gws` | Events in time window |
| Linear | `linear` + `linear api` | Assigned issues, cycles, comments, completions |

All sources are fetched in parallel. Each source runs its own sub-queries in parallel too.
