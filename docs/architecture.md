# Kora Architecture

System overview, package structure, and data flow.

## System Overview

Kora follows a layered architecture with clear separation of concerns:

```
┌────────────────────────────────────────────────────────────────────┐
│                            CLI Layer                                │
│                         (cmd/kora/)                                 │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐                         │
│  │ digest   │  │ version  │  │ root     │                         │
│  └────┬─────┘  └──────────┘  └──────────┘                         │
└───────┼────────────────────────────────────────────────────────────┘
        │
        ▼
┌────────────────────────────────────────────────────────────────────┐
│                      Configuration Layer                            │
│                      (internal/config/)                             │
│  ┌────────────────────────────────────────────────────────┐        │
│  │ Config struct: datasources, digest, security           │        │
│  │ YAML loading: ~/.kora/config.yaml                      │        │
│  └────────────────────────────────────────────────────────┘        │
└───────┬────────────────────────────────────────────────────────────┘
        │
        ▼
┌────────────────────────────────────────────────────────────────────┐
│                     Authentication Layer                            │
│                       (internal/auth/)                              │
│  ┌─────────────────────────┐  ┌─────────────────────────┐         │
│  │  GitHubAuthProvider     │  │  SlackAuthProvider      │         │
│  │  - gh CLI delegation    │  │  - Keychain storage     │         │
│  │  - Never sees token     │  │  - Env var fallback     │         │
│  └─────────────────────────┘  └─────────────────────────┘         │
└───────┬───────────────────────────┬────────────────────────────────┘
        │                           │
        ▼                           ▼
┌────────────────────────────────────────────────────────────────────┐
│                       DataSource Layer                              │
│                   (internal/datasources/)                           │
│  ┌──────────────────────────────────────────────────────────────┐  │
│  │                    DataSourceRunner                           │  │
│  │  - Concurrent execution via errgroup                         │  │
│  │  - Per-datasource timeout                                    │  │
│  │  - Partial failure handling                                  │  │
│  └──────────────────────────────────────────────────────────────┘  │
│           │                                 │                       │
│           ▼                                 ▼                       │
│  ┌──────────────────┐            ┌──────────────────┐             │
│  │ GitHubDataSource │            │ SlackDataSource  │             │
│  │ - PR reviews     │            │ - DMs            │             │
│  │ - Mentions       │            │ - Mentions       │             │
│  │ - Assignments    │            │                  │             │
│  └──────────────────┘            └──────────────────┘             │
└───────┬────────────────────────────────┬───────────────────────────┘
        │                                │
        ▼                                ▼
┌────────────────────────────────────────────────────────────────────┐
│                          Event Model                                │
│                       (internal/models/)                            │
│  ┌────────────────────────────────────────────────────────┐        │
│  │ Event struct (EFA 0001):                               │        │
│  │ - Type, Title, Source, URL, Author, Timestamp          │        │
│  │ - Priority (1-5), RequiresAction, Metadata             │        │
│  │ - Validation rules                                     │        │
│  └────────────────────────────────────────────────────────┘        │
└───────┬────────────────────────────────────────────────────────────┘
        │
        ▼
┌────────────────────────────────────────────────────────────────────┐
│                         Output Layer                                │
│                       (internal/output/)                            │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐            │
│  │ JSONFormatter│  │ TextFormatter│  │ (future)     │            │
│  │ - json       │  │ - text       │  │              │            │
│  │ - json-pretty│  │              │  │              │            │
│  └──────────────┘  └──────────────┘  └──────────────┘            │
└────────────────────────────────────────────────────────────────────┘
```

## Package Structure

### cmd/kora/

CLI entry point using Cobra:

- `main.go` - Application entry point
- `root.go` - Root command configuration
- `digest.go` - Digest command implementation
- `version.go` - Version command

### internal/auth/

Authentication providers (EFA 0002):

**Interfaces:**
- `AuthProvider` - Service authentication interface
- `Credential` - Credential with safe redaction

**Implementations:**
- `github/provider.go` - GitHub auth via `gh` CLI delegation
- `slack/provider.go` - Slack auth via Keychain/env var
- `keychain/macos.go` - macOS Keychain wrapper

**Security guarantees:**
- Credentials never logged (always redacted)
- GitHub tokens never stored (CLI delegation)
- Slack tokens in Keychain (OS encryption)

### internal/datasources/

Event fetching from external services (EFA 0003):

**Core:**
- `runner.go` - Concurrent datasource execution
- `options.go` - Fetch options and filters
- `errors.go` - Sentinel errors

**Implementations:**
- `github/datasource.go` - GitHub PR/issue fetching
- `slack/datasource.go` - Slack DM/mention fetching

**Concurrency model:**
- All datasources run in parallel via `errgroup`
- Per-datasource timeout (default 30s)
- One failure doesn't block others
- Partial results returned when possible

### internal/models/

Core domain models (EFA 0001):

**Event model:**
- `event.go` - Event struct and validation
- `person.go` - Person (author) representation
- `priority.go` - Priority enum (1-5)
- `source.go` - Source enum (github, slack)

**Design principles:**
- Flat structure (no nesting)
- Type-safe enums
- Built-in validation
- Metadata escape hatch for source-specific data

### internal/output/

Event formatting for different outputs:

- `formatter.go` - Formatter interface
- `json.go` - JSON formatters (compact and pretty)
- `text.go` - Text formatter (human-readable)

**Design:**
- No ANSI colors (AI-friendly)
- No box-drawing characters
- Sorted by priority and timestamp

### internal/config/

YAML configuration loading:

- `config.go` - Config struct and loading
- `validate.go` - Config validation

**Default locations:**
- `~/.kora/config.yaml` (user config)
- `configs/kora.yaml.example` (template)

### pkg/

Shared utilities (currently minimal):

- Future: clock abstraction for testing
- Future: structured logger

## Data Flow

### Digest Generation Flow

```
1. Parse CLI args (--since, --format, --config)
   ↓
2. Load config from ~/.kora/config.yaml (or --config path)
   ↓
3. Initialize auth providers
   │
   ├─→ GitHub: Check gh CLI auth status
   └─→ Slack: Load token from Keychain or env var
   ↓
4. Create datasources with auth providers
   │
   ├─→ GitHubDataSource(githubAuth)
   └─→ SlackDataSource(slackAuth)
   ↓
5. Create DataSourceRunner with datasources
   ↓
6. Run datasources concurrently
   │
   ├─→ GitHub: 4 parallel searches (reviews, mentions, issues, assignments)
   │   └─→ Parse JSON → Convert to Event → Validate
   │
   └─→ Slack: 2 parallel fetches (DMs, mentions)
       └─→ Parse JSON → Convert to Event → Validate
   ↓
7. Aggregate results
   │
   ├─→ Merge events from all sources
   ├─→ Deduplicate by URL
   ├─→ Sort by timestamp
   └─→ Record errors from failed sources
   ↓
8. Format events
   │
   ├─→ JSON: Serialize Event array
   ├─→ JSON-pretty: Serialize with indentation
   └─→ Text: Render human-readable format
   ↓
9. Output to stdout
```

### Authentication Flow

**GitHub (CLI Delegation):**
```
1. Check gh CLI installed: exec.LookPath("gh")
   ↓
2. Verify auth status: gh auth status
   ↓
3. Create GitHubDelegatedCredential
   │
   └─→ Stores username only, NOT token
   ↓
4. All API calls via: gh api <endpoint>
   │
   └─→ Token never leaves gh CLI
```

**Slack (Keychain Storage):**
```
1. Try macOS Keychain first
   │
   ├─→ security find-generic-password -s kora -a slack-token -w
   └─→ Returns token or "not found"
   ↓
2. Fallback to environment variable
   │
   └─→ KORA_SLACK_TOKEN
   ↓
3. Create SlackToken credential
   │
   ├─→ Validate format: starts with xoxp-
   └─→ Generate SHA256 fingerprint for logging
   ↓
4. Use in Authorization: Bearer <token>
```

### Error Handling Flow

**Partial Failure Example:**
```
DataSourceRunner runs:
  - GitHub datasource: SUCCESS (5 events)
  - Slack datasource: FAILURE (rate limited)

Result:
  - Events: 5 events from GitHub
  - SourceResults: {"github": FetchResult{...}}
  - SourceErrors: {"slack": ErrRateLimited}
  - Output: Shows 5 events + warning about Slack failure
```

## EFA Governance System

Kora uses EFA (Explainer For Agents) documents to define ground truth specifications for AI agents:

**Purpose:**
- Prevent specification drift across AI iterations
- Provide explicit constraints for code modifications
- Document architectural decisions in machine-readable format

**EFA Documents:**
- `specs/efas/0001-event-model.md` - Event struct, validation, metadata keys
- `specs/efas/0002-auth-provider.md` - Auth interface, credential security
- `specs/efas/0003-datasource-interface.md` - DataSource interface, concurrency

**Protection mechanisms:**
- Code comments: `IT IS FORBIDDEN TO CHANGE without updating EFA`
- AI Agent Rules section with explicit constraints
- Stop-and-ask triggers for protected operations

**Workflow:**
1. AI reads EFA before modifying governed code
2. AI checks "Stop and Ask" triggers
3. If modification requires EFA update, ask user first
4. Update EFA before or alongside code changes

## Extension Points

### Adding a New Datasource

See `docs/datasources.md` for detailed guide.

**Steps:**
1. Add source constant to `models.Source` (EFA 0001)
2. Add event types to `models.EventType` (EFA 0001)
3. Create auth provider in `internal/auth/<service>/`
4. Create datasource in `internal/datasources/<service>/`
5. Implement `DataSource` interface
6. Update EFA 0003 with new implementation

**Example: Linear datasource**
```go
// 1. Add to models/source.go (requires EFA 0001 update)
const SourceLinear Source = "linear"

// 2. Create internal/auth/linear/provider.go
type LinearAuthProvider struct { ... }

// 3. Create internal/datasources/linear/datasource.go
type DataSource struct { ... }
func (d *DataSource) Fetch(ctx, opts) (*FetchResult, error) { ... }

// 4. Register in cmd/kora/digest.go
linearAuth := linear.NewAuthProvider(...)
linearDS := linear.NewDataSource(linearAuth)
runner := datasources.NewRunner([]datasources.DataSource{
    githubDS, slackDS, linearDS,
})
```

### Adding a New Output Format

```go
// 1. Create internal/output/myformat.go
type MyFormatter struct{}

func (f *MyFormatter) Format(events []models.Event, stats *FormatStats) (string, error) {
    // Render events in custom format
}

// 2. Register in internal/output/formatter.go
func NewFormatter(format string) (Formatter, error) {
    switch format {
    case "json": return NewJSONFormatter(false), nil
    case "myformat": return NewMyFormatter(), nil
    // ...
    }
}
```

### Adding New Event Metadata

**REQUIRES EFA 0001 UPDATE**

```go
// 1. Update specs/efas/0001-event-model.md
// Add to metadata keys table for the source

// 2. Update models/event.go
var allowedMetadataKeys = map[Source]map[string]struct{}{
    SourceGitHub: {
        // ... existing keys ...
        "new_key": {}, // Add here
    },
}

// 3. Use in datasource
event := models.Event{
    // ...
    Metadata: map[string]any{
        "new_key": value,
    },
}
```

## Testing Strategy

### Unit Tests

Each package has tests alongside code:

- `auth/*_test.go` - Auth provider tests with mocks
- `datasources/*_test.go` - DataSource tests with testdata fixtures
- `models/*_test.go` - Event validation tests
- `output/*_test.go` - Formatter output tests

**Test patterns:**
- Table-driven tests for multiple scenarios
- Testdata fixtures for API responses
- Mock interfaces for external dependencies

### Integration Tests

Located in `tests/integration/`:

- `github_test.go` - Real GitHub API calls (requires `gh auth`)
- `slack_test.go` - Real Slack API calls (requires token)
- Build tag: `//go:build integration`

**Run with:**
```bash
make test-integration  # Requires authentication
```

### Test Coverage

Target: >80% coverage

**Check with:**
```bash
make test-coverage
go tool cover -html=coverage.out
```

## Performance Characteristics

### Concurrency

- Datasources run in parallel (N goroutines for N sources)
- Per-datasource timeout (default 30s)
- No global timeout (allow partial success)

### Memory

- In-memory only (no persistence)
- Event limit per datasource: configurable via `FetchOptions.Limit`
- HTTP response limit: 10MB per request

### Network

- TLS 1.2+ required
- Connection timeout: 30s
- Read timeout: 30s
- No connection pooling (single invocation)

## Security Model

See `SECURITY.md` for full details.

**Trust boundaries:**
```
┌─────────────────────────────────────────────────────┐
│ Trusted:                                             │
│ - gh CLI (GitHub auth delegation)                   │
│ - macOS Keychain (Slack token storage)              │
│ - User's ~/.kora/config.yaml                        │
│ - Go standard library                                │
└─────────────────────────────────────────────────────┘
         │
         ▼
┌─────────────────────────────────────────────────────┐
│ Kora Process:                                        │
│ - Validates all inputs                               │
│ - Redacts all credentials in logs                    │
│ - Uses TLS for all network calls                     │
│ - Validates all events before returning              │
└─────────────────────────────────────────────────────┘
         │
         ▼
┌─────────────────────────────────────────────────────┐
│ Untrusted:                                           │
│ - External API responses (GitHub, Slack)             │
│ - Network infrastructure                             │
│ - User's terminal (output only)                      │
└─────────────────────────────────────────────────────┘
```

## Future Considerations

### v2 Features (Not Yet Implemented)

- Persistent storage (SQLite)
- Event history and trends
- Digest scheduling (cron integration)
- Web UI for event browsing
- Notification support (desktop, email)
- Additional datasources (Linear, Notion, etc.)
- Cross-platform support (Linux, Windows)

### Known Limitations

- macOS only (Keychain dependency)
- No caching (fetch on every invocation)
- No pagination (fetches up to limit per source)
- No event deduplication across runs
- Slack requires enterprise workspace approval
