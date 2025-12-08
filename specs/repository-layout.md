# Kora Repository Layout Plan

> A Go-based personal assistant CLI tool. Start simple, evolve as needed.

## Overview

Kora aggregates data from GitHub to provide morning briefings that Claude can invoke via Bash or MCP. Additional datasources (Linear, Calendar, Gmail) are planned for future iterations.

## Repository Structure

```
kora/
├── cmd/
│   └── kora/                      # Main CLI entry point
│       ├── main.go
│       ├── init.go                # Initialize memory store
│       └── db.go                  # Database admin commands
│
├── internal/
│   ├── config/                    # Configuration management
│   │   ├── config.go
│   │   └── validation.go
│   │
│   ├── auth/                      # Authentication abstraction
│   │   ├── auth.go                # Core interfaces
│   │   ├── provider.go            # AuthProvider interface
│   │   ├── github/
│   │   │   ├── auth.go            # gh CLI delegation
│   │   │   └── auth_test.go
│   │   └── keychain/
│   │       ├── keychain.go        # Interface
│   │       ├── keychain_darwin.go # macOS implementation
│   │       └── keychain_test.go
│   │
│   ├── datasources/               # Pluggable data sources
│   │   ├── datasource.go          # Interface + registry
│   │   └── github/
│   │       ├── client.go
│   │       ├── prs.go
│   │       ├── issues.go
│   │       └── client_test.go
│   │
│   ├── models/                    # Domain models
│   │   ├── event.go               # Core Event model
│   │   ├── person.go
│   │   └── priority.go
│   │
│   ├── output/                    # Output formatting
│   │   ├── formatter.go           # Interface
│   │   ├── terminal.go
│   │   ├── markdown.go
│   │   └── json.go
│   │
│   └── storage/                   # Memory store
│       ├── schema.sql             # SQLite schema with FTS5
│       ├── store.go               # Store initialization and access
│       └── migrations.go          # Version-tracked schema migrations
│
├── pkg/
│   ├── clock/                     # Time utilities
│   │   └── clock.go
│   └── logger/                    # Structured logging
│       └── logger.go
│
├── scripts/
│   ├── install-mcp.sh             # Install as MCP tool
│   └── security-scan.sh           # Local security scanning
│
├── configs/
│   └── kora.yaml.example          # Example configuration
│
├── docs/
│   ├── architecture.md            # Architecture decisions
│   └── datasources.md             # How to add datasources
│
├── specs/                         # Design documents
│   ├── prototype/                 # Original design notes
│   ├── memory-mcp.md              # MCP configuration for SQLite access
│   ├── efas/                      # Explainer For Agents (ground truth)
│   │   ├── AUTHORING.md           # How to write EFAs
│   │   ├── 0001-event-model.md    # Event struct, validation
│   │   ├── 0002-auth-provider.md  # Auth interfaces, security
│   │   ├── 0003-datasource-interface.md  # DataSource interface
│   │   └── 0004-tool-responsibility.md   # Data vs intelligence separation
│   └── repository-layout.md       # This document
│
├── tests/
│   ├── integration/
│   │   ├── digest_test.go
│   │   ├── github_auth_test.go
│   │   └── memory_test.go         # Memory store lifecycle tests
│   └── testdata/
│       └── github_prs.json
│
├── .github/
│   ├── workflows/
│   │   ├── ci.yml                 # Test, lint, build
│   │   └── security.yml           # Security scanning
│   └── dependabot.yml
│
├── go.mod
├── go.sum
├── .golangci.yml
├── .trivyignore
├── Makefile
├── SECURITY.md
└── README.md
```

## Core Concepts

### EFA Ground Truth
The `specs/efas/` directory contains **Explainer For Agents (EFAs)** - formal specifications that define ground truth for AI agents. Before modifying core interfaces, check:
- **EFA 0001**: Event model, validation rules, metadata keys
- **EFA 0002**: Auth providers, credential security, CLI delegation
- **EFA 0003**: DataSource interface, concurrency, partial success
- **EFA 0004**: Tool responsibility, data vs intelligence separation

**Protected code patterns** are marked with comments like:
```go
// IT IS FORBIDDEN TO CHANGE THIS without updating EFA XXXX.
```

### 1. Interface-Driven Design
Key interfaces enable testing and future extensibility (see EFAs for ground truth):
- `DataSource` - Fetch events from external services (EFA 0003)
- `AuthProvider` - Manage authentication per service (EFA 0002)
- `Credential` - Represent and validate credentials (EFA 0002)
- `Formatter` - Output events in different formats

### 2. Event Model (EFA 0001)
All datasources produce normalized events that must pass `Event.Validate()`:
```
Event:
  - Type (pr_review, pr_mention, issue_mention, issue_assigned, pr_comment_mention, pr_closed, pr_author, pr_codeowner)
  - Title (1-100 chars), Source, URL
  - Author (Person with Username required)
  - Timestamp (UTC, non-zero), Priority (1-5)
  - RequiresAction (bool)
  - Metadata (map - keys must be from EFA 0001 allowed set per source)
```
See EFA 0001 for complete validation rules and metadata key allowlists.

### 3. Authentication Strategy

**Current Implementation:**
1. **CLI Delegation** (GitHub via `gh`) - Most secure, no credential storage

**Future Datasources:**
2. **macOS Keychain** (for services requiring token storage) - OS-managed security
3. **Environment Variable** - Fallback when keychain unavailable

**Auth Provider Interface (see EFA 0002):**
- `Service()` - Return which service this provider authenticates
- `Authenticate(ctx)` - Validate credentials exist and are usable
- `GetCredential(ctx)` - Retrieve credential (GitHub: delegated, future: tokens)
- `IsAuthenticated(ctx)` - Check auth status (non-blocking)

**Note:** GitHub uses CLI delegation via `GitHubDelegatedCredential.ExecuteAPI()` - Kora never extracts or stores GitHub tokens.

**macOS Keychain Implementation (future datasources):**
- Use `security` command-line tool
- Service name: "kora"
- Account name: credential key (e.g., "linear-token")

### 4. Configuration

**Precedence:**
1. CLI flags
2. Environment variables
3. `~/.kora/config.yaml`
4. Defaults

**Example config:**
```yaml
datasources:
  github:
    enabled: true
    orgs:
      - chainguard-dev

digest:
  window: 16h
  timezone: America/Los_Angeles
  output: terminal

data_dir: ~/.kora/data

security:
  redact_credentials: true
  datasource_timeout: 30s
  verify_tls: true
```

### 5. Execution Model
- Process datasources concurrently using `errgroup`
- Don't fail entire digest if one source fails
- Log errors, continue with partial results
- Respect timeouts (30s default)

### 6. Memory Store
**Schema ownership**: Kora owns schema, migrations, admin commands
**Claude access**: Direct via SQLite MCP server (no CRUD wrapper)
**Storage**: `~/.kora/data/kora.db`
**Tables**: goals, commitments, accomplishments, context, memory_search (FTS5)

## Security Design

### Threat Model

**Trust Boundaries:**
- User machine → Kora CLI: Trusted
- Kora → External APIs: Untrusted (TLS required)
- Kora → macOS Keychain: Trusted

**Attack Vectors & Mitigations:**
- Credential theft from files → Use keychain or CLI delegation only
- MITM attacks → TLS 1.2+, certificate validation
- Credential leakage in logs → Redact all credentials
- Compromised dependencies → CI security scanning
- Privilege escalation → Runs as user, no elevation needed

### Security Controls

1. **Credential Management**
   - Never store credentials in plaintext
   - Prefer CLI delegation (gh) over credential storage
   - Use macOS Keychain for tokens (future datasources)
   - Environment variables as last resort
   - Redact all credentials in logs

2. **Network Security**
   - TLS 1.2+ for all API calls
   - Certificate validation enabled
   - 30s timeout on all requests
   - No telemetry or analytics

3. **Data Protection**
   - Memory store uses SQLite at `~/.kora/data/kora.db`
   - Minimal metadata collection
   - No sensitive data in logs

4. **Dependencies**
   - Pin versions in go.mod
   - Dependabot for updates
   - gosec + Trivy in CI

5. **Input Validation**
   - Validate all config
   - Sanitize API responses
   - No shell execution from user input

### Credential Storage

| Service | Method | Storage | Why |
|---------|--------|---------|-----|
| GitHub | `gh` CLI | None (gh manages) | Most secure |
| Future sources | Token | Keychain → Env var | Keychain preferred |

## CI/CD Design

### GitHub Actions Workflows

**CI (`.github/workflows/ci.yml`):**
- Lint with golangci-lint
- Test on macOS with Go 1.25
- Build and verify binary
- Upload coverage to Codecov

**Security (`.github/workflows/security.yml`):**
- gosec static analysis → SARIF to GitHub Security
- Trivy vulnerability scan → SARIF to GitHub Security
- Dependency review on PRs (fail on moderate+)
- Weekly scheduled scans

**Dependabot (`.github/dependabot.yml`):**
- Weekly Go module updates
- Weekly GitHub Actions updates
- Auto-assign to @dakaneye
- Commit prefixes: `deps:`, `ci:`

### Local Development

**Makefile targets:**
```makefile
build               # Build to bin/kora
test                # Unit tests only
test-integration    # Integration tests (requires auth)
test-coverage       # All tests with coverage
lint                # golangci-lint
security-scan       # gosec + Trivy
install             # Build and install to ~/.local/bin
clean               # Remove artifacts
```

## Integration Testing

### GitHub Auth Test
- Verify `gh` CLI delegation works
- Check credential type and redaction
- Requires: `gh auth login` completed

### Memory Store Test
- Full lifecycle: init, insert, query, backup
- FTS sync triggers
- Prune and error handling

**Run with:**
```bash
make test-integration
```

## What's In v1

1. **CLI tool** with `digest` command
   - `kora digest [--since 16h] [--format terminal]`
   - Exit codes: 0=success, 1=partial, 2=failure

2. **Authentication**
   - GitHub via `gh` CLI delegation
   - Integration tests for auth

3. **Datasources**
   - GitHub: PRs and Issues
   - Future: Linear, Calendar, Gmail

4. **Output formats**
   - Terminal (pretty table)
   - Markdown
   - JSON (for Claude)

5. **CI/CD**
   - Automated testing (macOS)
   - Security scanning (gosec, Trivy)
   - Dependency updates (Dependabot)

6. **Testing**
   - Unit tests with mocked HTTP
   - Integration tests for auth
   - >80% coverage

7. **Memory Store**
   - Schema initialization (`kora init`)
   - Admin commands (`kora db stats|validate|prune|backup|export|path`)
   - SQLite with FTS5 full-text search
   - Claude direct access via SQLite MCP

## How Claude Uses Kora

### Direct Bash
```bash
kora digest --since 16h --format json
```

### MCP Tool
```bash
# scripts/install-mcp.sh creates ~/.claude/mcp-tools/kora.json
{
  "name": "kora",
  "description": "Get morning digest and work updates",
  "command": "kora",
  "args": ["digest", "--format", "json"],
  "timeout": 10000
}
```

## Key Decisions

1. **Go** - Fast, concurrent, single binary
2. **CLI first** - Simple Claude integration, no infra
3. **In-memory** - No persistence needed for digest
4. **macOS only** - Single platform simplifies v1
5. **Keychain** - OS-provided security beats custom encryption
6. **CLI delegation** - Most secure auth (no cred storage)
7. **SQLite + MCP** - Kora as schema owner, Claude accesses directly (no wrapper overhead)

## Success Criteria

- ✅ Digest completes in <10s
- ✅ GitHub datasource works
- ✅ Auth secure (no plaintext creds)
- ✅ All credentials redacted in logs
- ✅ Three output formats
- ✅ Claude can invoke and use results
- ✅ Test coverage >80%
- ✅ Security scans pass
- ✅ CI/CD operational

## Next Steps

1. Initialize repo and Go module
2. Create auth interfaces and keychain (macOS)
3. Implement GitHub auth provider with tests
4. Create Event model and DataSource interface
5. Implement GitHub datasource
6. Wire up CLI with Cobra
7. Add output formatters
8. Set up GitHub Actions (CI, security)
9. Write documentation (README, SECURITY.md)
10. Plan future datasources (Linear, Calendar, Gmail)

## Example Output

### Terminal
```
╔═══════════════════════════════════════════════════════════════════════╗
║ Morning Digest - December 6, 2025 9:00 AM PST                        ║
╠═══════════════════════════════════════════════════════════════════════╣
║ Priority 1 - Requires Action (2 items)                               ║
╠═══════════════════════════════════════════════════════════════════════╣
║ [PR Review] Add secure rebuild for core-java                         ║
║ │ Source: github.com/chainguard-dev/internal-dev/pulls/1234          ║
║ │ Author: @teammate1 • 2 hours ago                                   ║
║ └─ Waiting on your review                                            ║
╠═══════════════════════════════════════════════════════════════════════╣
║ [Issue Assigned] Customer onboarding - Acme Corp                     ║
║ │ Source: github.com/chainguard-dev/internal-dev/issues/789          ║
║ │ Author: @salesrep • 4 hours ago                                    ║
║ └─ Needs initial assessment                                          ║
╚═══════════════════════════════════════════════════════════════════════╝

Summary: 2 items require action, 4 for awareness
Execution time: 3.8s
```

### JSON (for Claude)
```json
[
  {
    "type": "pr_review",
    "title": "Add secure rebuild for core-java",
    "source": "github",
    "url": "https://github.com/chainguard-dev/internal-dev/pulls/1234",
    "author": {
      "name": "Teammate One",
      "username": "teammate1"
    },
    "timestamp": "2025-12-06T07:00:00Z",
    "priority": 2,
    "requires_action": true,
    "metadata": {
      "repo": "chainguard-dev/internal-dev",
      "number": 1234,
      "state": "open",
      "user_relationships": ["direct_reviewer"],
      "ci_rollup": "success"
    }
  }
]
```

---

**Status**: Ready for implementation
**Date**: 2025-12-06
