# Adding a New Datasource

Step-by-step guide for implementing new datasources in Kora.

## Prerequisites

Before adding a new datasource:

1. **Read the EFAs:**
   - `specs/efas/0001-event-model.md` - Event structure and validation
   - `specs/efas/0002-auth-provider.md` - Authentication requirements
   - `specs/efas/0003-datasource-interface.md` - DataSource interface specification

2. **Understand the architecture:**
   - Read `docs/architecture.md`
   - Review existing implementations in `internal/datasources/github/` and `internal/datasources/slack/`

3. **Have API access:**
   - Test API credentials
   - Understand API rate limits
   - Know API response formats

## Overview

Adding a datasource involves:

1. Updating EFA 0001 with new source constants and event types
2. Creating an auth provider (EFA 0002)
3. Implementing the DataSource interface (EFA 0003)
4. Writing tests
5. Registering in the CLI

## Step 1: Update EFA 0001 (Event Model)

**CRITICAL: This step is REQUIRED before writing code.**

### 1.1 Add Source Constant

Edit `specs/efas/0001-event-model.md`:

```go
// Add to Source Constants section
const (
    SourceGitHub Source = "github"
    SourceSlack  Source = "slack"
    SourceLinear Source = "linear"  // ADD THIS
)

// Update validSources map
var validSources = map[Source]struct{}{
    SourceGitHub: {},
    SourceSlack:  {},
    SourceLinear: {},  // ADD THIS
}
```

### 1.2 Add Event Types

```go
// Add to EventType constants
const (
    // ... existing types ...

    // Linear event types
    EventTypeLinearIssueAssigned EventType = "linear_issue_assigned"
    EventTypeLinearIssueMention  EventType = "linear_issue_mention"
)

// Update validEventTypes map
var validEventTypes = map[EventType]struct{}{
    // ... existing types ...
    EventTypeLinearIssueAssigned: {},
    EventTypeLinearIssueMention:  {},
}
```

### 1.3 Define Metadata Keys

Add to "Metadata Keys by Source" section:

```markdown
#### Linear Metadata

| Key | Type | Description |
|-----|------|-------------|
| `team` | string | Team name |
| `project` | string | Project name |
| `state` | string | Issue state: "todo", "in_progress", "done" |
| `priority` | int | Linear priority (0-4) |
| `labels` | []string | Issue labels |
```

Update `allowedMetadataKeys` map:

```go
var allowedMetadataKeys = map[Source]map[string]struct{}{
    // ... existing sources ...
    SourceLinear: {
        "team":     {},
        "project":  {},
        "state":    {},
        "priority": {},
        "labels":   {},
    },
}
```

### 1.4 Update Priority Assignment Rules

Add to "Priority Assignment Rules" table:

```markdown
| Condition | Priority |
|-----------|----------|
| ... existing rules ...
| Linear issue assigned | 3 (Medium) |
| Linear mention | 3 (Medium) |
```

### 1.5 Add Example Event

```json
{
  "type": "linear_issue_assigned",
  "title": "Assigned: Fix authentication bug",
  "source": "linear",
  "url": "https://linear.app/team/issue/123",
  "author": {
    "name": "Jane Developer",
    "username": "jane"
  },
  "timestamp": "2025-12-06T08:00:00Z",
  "priority": 3,
  "requires_action": true,
  "metadata": {
    "team": "engineering",
    "project": "auth-system",
    "state": "todo",
    "priority": 1,
    "labels": ["bug", "security"]
  }
}
```

### 1.6 Commit EFA Changes

```bash
git add specs/efas/0001-event-model.md
git commit -m "docs(efa): add Linear source and event types to EFA 0001"
```

## Step 2: Implement Event Model Constants

Now update `internal/models/` to match the EFA:

### 2.1 Update models/source.go

```go
// Source identifies which datasource produced an event.
type Source string

const (
    SourceGitHub Source = "github"
    SourceSlack  Source = "slack"
    SourceLinear Source = "linear"  // ADD THIS
)

var validSources = map[Source]struct{}{
    SourceGitHub: {},
    SourceSlack:  {},
    SourceLinear: {},  // ADD THIS
}
```

### 2.2 Update models/event.go

```go
// EventType constants for Linear
const (
    EventTypeLinearIssueAssigned EventType = "linear_issue_assigned"
    EventTypeLinearIssueMention  EventType = "linear_issue_mention"
)

// Update validEventTypes
var validEventTypes = map[EventType]struct{}{
    // ... existing types ...
    EventTypeLinearIssueAssigned: {},
    EventTypeLinearIssueMention:  {},
}

// Update allowedMetadataKeys
var allowedMetadataKeys = map[Source]map[string]struct{}{
    // ... existing sources ...
    SourceLinear: {
        "team":     {},
        "project":  {},
        "state":    {},
        "priority": {},
        "labels":   {},
    },
}
```

### 2.3 Write Tests

```go
// models/event_test.go

func TestEvent_Validate_LinearMetadata(t *testing.T) {
    tests := []struct {
        name    string
        event   Event
        wantErr bool
    }{
        {
            name: "valid linear metadata",
            event: Event{
                Type:      EventTypeLinearIssueAssigned,
                Title:     "Test",
                Source:    SourceLinear,
                URL:       "https://linear.app/team/issue/123",
                Author:    Person{Username: "user"},
                Timestamp: time.Now(),
                Priority:  PriorityMedium,
                Metadata: map[string]any{
                    "team":    "eng",
                    "project": "auth",
                    "state":   "todo",
                },
            },
            wantErr: false,
        },
        {
            name: "invalid linear metadata key",
            event: Event{
                Type:      EventTypeLinearIssueAssigned,
                Source:    SourceLinear,
                Metadata:  map[string]any{
                    "invalid_key": "value",
                },
            },
            wantErr: true,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := tt.event.Validate()
            if (err != nil) != tt.wantErr {
                t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
            }
        })
    }
}
```

## Step 3: Create Auth Provider

### 3.1 Create Package Structure

```bash
mkdir -p internal/auth/linear
```

### 3.2 Implement AuthProvider

```go
// internal/auth/linear/provider.go

package linear

import (
    "context"
    "fmt"

    "github.com/dakaneye/kora/internal/auth"
    "github.com/dakaneye/kora/internal/auth/keychain"
)

// Provider implements auth.AuthProvider for Linear.
// Credentials are stored in macOS Keychain.
type Provider struct {
    keychain keychain.Keychain
}

const (
    linearKeychainKey = "linear-api-key"
    linearEnvVarName  = "KORA_LINEAR_API_KEY"
)

// NewProvider creates a Linear auth provider.
func NewProvider(keychain keychain.Keychain) *Provider {
    return &Provider{keychain: keychain}
}

func (p *Provider) Service() auth.Service {
    return auth.Service("linear")  // Add to auth.Service constants
}

func (p *Provider) Authenticate(ctx context.Context) error {
    cred, err := p.GetCredential(ctx)
    if err != nil {
        return err
    }
    if !cred.IsValid() {
        return auth.ErrCredentialInvalid
    }
    return nil
}

func (p *Provider) GetCredential(ctx context.Context) (auth.Credential, error) {
    // 1. Try keychain
    apiKey, err := p.keychain.Get(ctx, linearKeychainKey)
    if err == nil {
        return NewAPIKey(apiKey)
    }
    if !errors.Is(err, keychain.ErrNotFound) {
        return nil, fmt.Errorf("linear auth: keychain: %w", err)
    }

    // 2. Fall back to env var
    if apiKey := os.Getenv(linearEnvVarName); apiKey != "" {
        return NewAPIKey(apiKey)
    }

    return nil, fmt.Errorf("linear auth: %w: set %s or store in keychain",
        auth.ErrNotAuthenticated, linearEnvVarName)
}

func (p *Provider) IsAuthenticated(ctx context.Context) bool {
    return p.Authenticate(ctx) == nil
}
```

### 3.3 Create Credential Type

```go
// internal/auth/linear/credential.go

package linear

import (
    "crypto/sha256"
    "encoding/hex"
    "fmt"
    "strings"

    "github.com/dakaneye/kora/internal/auth"
)

// APIKey represents a Linear API key.
// SECURITY: Value is never logged. Only fingerprint shown.
type APIKey struct {
    key string
}

// NewAPIKey creates a Linear API key credential.
func NewAPIKey(key string) (*APIKey, error) {
    k := &APIKey{key: key}
    if !k.IsValid() {
        return nil, fmt.Errorf("%w: invalid linear api key format", auth.ErrCredentialInvalid)
    }
    return k, nil
}

func (k *APIKey) Type() auth.CredentialType {
    return auth.CredentialType("api_key")  // Add to CredentialType constants
}

func (k *APIKey) Value() string {
    return k.key
}

// Redacted returns a safe-to-log fingerprint.
// SECURITY: Shows hash-based fingerprint, not actual key.
func (k *APIKey) Redacted() string {
    if len(k.key) < 8 {
        return "lin_[invalid]"
    }
    h := sha256.Sum256([]byte(k.key))
    fingerprint := hex.EncodeToString(h[:4])
    return fmt.Sprintf("lin_[%s]", fingerprint)
}

func (k *APIKey) IsValid() bool {
    // Linear API keys start with "lin_api_"
    return strings.HasPrefix(k.key, "lin_api_") && len(k.key) > 16
}

func (k *APIKey) String() string {
    return k.Redacted()
}
```

### 3.4 Write Auth Tests

```go
// internal/auth/linear/provider_test.go

func TestProvider_GetCredential(t *testing.T) {
    tests := []struct {
        name           string
        keychainValue  string
        keychainError  error
        envValue       string
        wantErr        bool
        wantType       auth.CredentialType
    }{
        {
            name:          "keychain success",
            keychainValue: "lin_api_abc123def456",
            wantErr:       false,
            wantType:      auth.CredentialType("api_key"),
        },
        {
            name:          "fallback to env var",
            keychainError: keychain.ErrNotFound,
            envValue:      "lin_api_xyz789",
            wantErr:       false,
        },
        {
            name:          "no credentials",
            keychainError: keychain.ErrNotFound,
            wantErr:       true,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // Test implementation
        })
    }
}
```

## Step 4: Implement DataSource

### 4.1 Create Package Structure

```bash
mkdir -p internal/datasources/linear
mkdir -p internal/datasources/linear/testdata
```

### 4.2 Implement DataSource Interface

```go
// internal/datasources/linear/datasource.go

package linear

import (
    "context"
    "encoding/json"
    "fmt"
    "net/http"
    "sort"
    "time"

    "github.com/dakaneye/kora/internal/auth"
    "github.com/dakaneye/kora/internal/datasources"
    "github.com/dakaneye/kora/internal/models"
)

// DataSource fetches events from Linear.
type DataSource struct {
    authProvider auth.AuthProvider
    httpClient   *http.Client
    baseURL      string
}

// DataSourceOption configures the Linear DataSource.
type DataSourceOption func(*DataSource)

// WithHTTPClient sets a custom HTTP client.
func WithHTTPClient(client *http.Client) DataSourceOption {
    return func(d *DataSource) {
        d.httpClient = client
    }
}

// NewDataSource creates a Linear datasource.
func NewDataSource(authProvider auth.AuthProvider, opts ...DataSourceOption) (*DataSource, error) {
    if authProvider.Service() != auth.Service("linear") {
        return nil, fmt.Errorf("linear datasource requires linear auth provider")
    }

    d := &DataSource{
        authProvider: authProvider,
        httpClient: &http.Client{
            Timeout: 30 * time.Second,
        },
        baseURL: "https://api.linear.app/graphql",
    }
    for _, opt := range opts {
        opt(d)
    }
    return d, nil
}

func (d *DataSource) Name() string           { return "linear" }
func (d *DataSource) Service() models.Source { return models.SourceLinear }

// Fetch retrieves Linear events (issue assignments, mentions).
//
// Implementation:
//   1. Get credential from auth provider
//   2. Query Linear GraphQL API for assigned issues
//   3. Query for mentions in issue comments
//   4. Convert responses to Event structs
//   5. Validate all events
//   6. Return sorted by timestamp
func (d *DataSource) Fetch(ctx context.Context, opts datasources.FetchOptions) (*datasources.FetchResult, error) {
    if err := opts.Validate(); err != nil {
        return nil, fmt.Errorf("linear fetch: %w", err)
    }

    // Get credential
    cred, err := d.authProvider.GetCredential(ctx)
    if err != nil {
        return nil, fmt.Errorf("linear fetch: %w", datasources.ErrNotAuthenticated)
    }

    apiKey := cred.Value()
    if apiKey == "" {
        return nil, fmt.Errorf("linear fetch: %w", datasources.ErrNotAuthenticated)
    }

    result := &datasources.FetchResult{
        Stats: datasources.FetchStats{},
    }
    startTime := time.Now()

    var allEvents []models.Event
    var fetchErrors []error

    // 1. Fetch assigned issues
    assigned, err := d.fetchAssignedIssues(ctx, apiKey, opts.Since)
    if err != nil {
        fetchErrors = append(fetchErrors, fmt.Errorf("assigned issues: %w", err))
    } else {
        allEvents = append(allEvents, assigned...)
        result.Stats.APICallCount++
    }

    // 2. Fetch mentions
    mentions, err := d.fetchMentions(ctx, apiKey, opts.Since)
    if err != nil {
        fetchErrors = append(fetchErrors, fmt.Errorf("mentions: %w", err))
    } else {
        allEvents = append(allEvents, mentions...)
        result.Stats.APICallCount++
    }

    // Deduplicate by URL
    allEvents = deduplicateEvents(allEvents)

    // Sort by timestamp
    sort.Slice(allEvents, func(i, j int) bool {
        return allEvents[i].Timestamp.Before(allEvents[j].Timestamp)
    })

    // Validate all events
    for i, event := range allEvents {
        if err := event.Validate(); err != nil {
            fetchErrors = append(fetchErrors, fmt.Errorf("event %d validation: %w", i, err))
        }
    }

    result.Events = allEvents
    result.Errors = fetchErrors
    result.Partial = len(fetchErrors) > 0 && len(allEvents) > 0
    result.Stats.EventsFetched = len(allEvents)
    result.Stats.EventsReturned = len(allEvents)
    result.Stats.Duration = time.Since(startTime)

    return result, nil
}

// fetchAssignedIssues queries Linear for issues assigned to the authenticated user.
func (d *DataSource) fetchAssignedIssues(ctx context.Context, apiKey string, since time.Time) ([]models.Event, error) {
    query := `
        query {
            issues(filter: { assignee: { isMe: { eq: true } } }) {
                nodes {
                    id
                    title
                    url
                    state { name }
                    priority
                    team { name }
                    project { name }
                    labels { nodes { name } }
                    createdAt
                    updatedAt
                    creator { name email }
                }
            }
        }
    `

    resp, err := d.graphqlRequest(ctx, apiKey, query, nil)
    if err != nil {
        return nil, err
    }

    var gqlResp struct {
        Data struct {
            Issues struct {
                Nodes []linearIssue `json:"nodes"`
            } `json:"issues"`
        } `json:"data"`
    }

    if err := json.Unmarshal(resp, &gqlResp); err != nil {
        return nil, fmt.Errorf("parse response: %w", err)
    }

    events := make([]models.Event, 0, len(gqlResp.Data.Issues.Nodes))
    for _, issue := range gqlResp.Data.Issues.Nodes {
        // Skip issues updated before since time
        if !issue.UpdatedAt.After(since) {
            continue
        }

        event := models.Event{
            Type:           models.EventTypeLinearIssueAssigned,
            Title:          truncateTitle(fmt.Sprintf("Assigned: %s", issue.Title)),
            Source:         models.SourceLinear,
            URL:            issue.URL,
            Author:         models.Person{Name: issue.Creator.Name, Username: issue.Creator.Email},
            Timestamp:      issue.UpdatedAt,
            Priority:       models.PriorityMedium,  // Per EFA 0001 priority rules
            RequiresAction: true,
            Metadata: map[string]any{
                "team":     issue.Team.Name,
                "project":  issue.Project.Name,
                "state":    issue.State.Name,
                "priority": issue.Priority,
                "labels":   extractLabels(issue.Labels.Nodes),
            },
        }
        events = append(events, event)
    }

    return events, nil
}

// fetchMentions queries Linear for mentions in issue comments.
func (d *DataSource) fetchMentions(ctx context.Context, apiKey string, since time.Time) ([]models.Event, error) {
    // Implementation similar to fetchAssignedIssues
    // Query Linear GraphQL for comments mentioning the user
    return nil, nil
}

// graphqlRequest makes an authenticated GraphQL request to Linear API.
func (d *DataSource) graphqlRequest(ctx context.Context, apiKey, query string, variables map[string]any) ([]byte, error) {
    reqBody := map[string]any{
        "query":     query,
        "variables": variables,
    }

    bodyJSON, err := json.Marshal(reqBody)
    if err != nil {
        return nil, fmt.Errorf("marshal request: %w", err)
    }

    req, err := http.NewRequestWithContext(ctx, http.MethodPost, d.baseURL, bytes.NewReader(bodyJSON))
    if err != nil {
        return nil, fmt.Errorf("create request: %w", err)
    }

    // SECURITY: API key only used in Authorization header, never logged
    req.Header.Set("Authorization", apiKey)
    req.Header.Set("Content-Type", "application/json")

    resp, err := d.httpClient.Do(req)
    if err != nil {
        return nil, fmt.Errorf("do request: %w", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        return nil, fmt.Errorf("api error: status %d", resp.StatusCode)
    }

    body, err := io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024)) // 10MB limit
    if err != nil {
        return nil, fmt.Errorf("read response: %w", err)
    }

    return body, nil
}

// Linear API response types
type linearIssue struct {
    ID        string    `json:"id"`
    Title     string    `json:"title"`
    URL       string    `json:"url"`
    State     struct {
        Name string `json:"name"`
    } `json:"state"`
    Priority  int       `json:"priority"`
    Team      struct {
        Name string `json:"name"`
    } `json:"team"`
    Project   struct {
        Name string `json:"name"`
    } `json:"project"`
    Labels    struct {
        Nodes []struct {
            Name string `json:"name"`
        } `json:"nodes"`
    } `json:"labels"`
    CreatedAt time.Time `json:"createdAt"`
    UpdatedAt time.Time `json:"updatedAt"`
    Creator   struct {
        Name  string `json:"name"`
        Email string `json:"email"`
    } `json:"creator"`
}

// Helper functions
func truncateTitle(title string) string {
    if len(title) <= 100 {
        return title
    }
    return title[:97] + "..."
}

func extractLabels(nodes []struct{ Name string }) []string {
    labels := make([]string, len(nodes))
    for i, n := range nodes {
        labels[i] = n.Name
    }
    return labels
}

func deduplicateEvents(events []models.Event) []models.Event {
    seen := make(map[string]bool)
    result := make([]models.Event, 0, len(events))
    for _, e := range events {
        if !seen[e.URL] {
            seen[e.URL] = true
            result = append(result, e)
        }
    }
    return result
}
```

### 4.3 Create Test Fixtures

```json
// internal/datasources/linear/testdata/assigned_issues.json
{
  "data": {
    "issues": {
      "nodes": [
        {
          "id": "issue-123",
          "title": "Fix authentication bug",
          "url": "https://linear.app/team/issue/123",
          "state": { "name": "In Progress" },
          "priority": 1,
          "team": { "name": "Engineering" },
          "project": { "name": "Auth System" },
          "labels": { "nodes": [
            { "name": "bug" },
            { "name": "security" }
          ]},
          "createdAt": "2025-12-06T08:00:00Z",
          "updatedAt": "2025-12-06T09:00:00Z",
          "creator": {
            "name": "Jane Developer",
            "email": "jane@company.com"
          }
        }
      ]
    }
  }
}
```

### 4.4 Write DataSource Tests

```go
// internal/datasources/linear/datasource_test.go

func TestDataSource_Fetch(t *testing.T) {
    tests := []struct {
        name       string
        mockFile   string
        wantEvents int
        wantErr    bool
    }{
        {
            name:       "fetch assigned issues",
            mockFile:   "testdata/assigned_issues.json",
            wantEvents: 1,
            wantErr:    false,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // Setup mock HTTP server
            // Load testdata
            // Execute Fetch
            // Assert results
        })
    }
}
```

## Step 5: Register in CLI

### 5.1 Update cmd/kora/digest.go

```go
// Add import
import (
    "github.com/dakaneye/kora/internal/auth/linear"
    linearDS "github.com/dakaneye/kora/internal/datasources/linear"
)

// In the digest command function
func runDigest(cmd *cobra.Command, args []string) error {
    // ... existing code ...

    // Initialize auth providers
    githubAuth := github.NewAuthProvider("")
    slackAuth := slack.NewAuthProvider(keychain.NewMacOSKeychain(""))
    linearAuth := linear.NewProvider(keychain.NewMacOSKeychain(""))  // ADD THIS

    // Create datasources
    var datasources []datasources.DataSource

    if cfg.Datasources.GitHub.Enabled {
        ds, err := githubDataSource.NewDataSource(githubAuth, ...)
        if err != nil { return err }
        datasources = append(datasources, ds)
    }

    if cfg.Datasources.Slack.Enabled {
        ds, err := slackDataSource.NewDataSource(slackAuth, ...)
        if err != nil { return err }
        datasources = append(datasources, ds)
    }

    // ADD THIS BLOCK
    if cfg.Datasources.Linear.Enabled {
        ds, err := linearDS.NewDataSource(linearAuth)
        if err != nil {
            return fmt.Errorf("create linear datasource: %w", err)
        }
        datasources = append(datasources, ds)
    }

    // ... rest of code ...
}
```

### 5.2 Update Config

```go
// internal/config/config.go

type DatasourcesConfig struct {
    GitHub GitHubConfig `yaml:"github"`
    Slack  SlackConfig  `yaml:"slack"`
    Linear LinearConfig `yaml:"linear"`  // ADD THIS
}

// ADD THIS TYPE
type LinearConfig struct {
    Enabled bool `yaml:"enabled"`
}

// Update DefaultConfig()
func DefaultConfig() *Config {
    return &Config{
        Datasources: DatasourcesConfig{
            GitHub: GitHubConfig{Enabled: true},
            Slack:  SlackConfig{Enabled: true},
            Linear: LinearConfig{Enabled: false},  // ADD THIS
        },
        // ... rest ...
    }
}
```

### 5.3 Update Example Config

```yaml
# configs/kora.yaml.example

datasources:
  github:
    enabled: true
  slack:
    enabled: true
  linear:  # ADD THIS SECTION
    enabled: false
    # Note: Requires Linear API key in keychain or KORA_LINEAR_API_KEY env var
```

## Step 6: Test End-to-End

### 6.1 Store Credentials

```bash
# Store Linear API key in Keychain
security add-generic-password -s "kora" -a "linear-api-key" -w "lin_api_YOUR_KEY_HERE"
```

### 6.2 Enable in Config

```yaml
# ~/.kora/config.yaml
datasources:
  linear:
    enabled: true
```

### 6.3 Run Digest

```bash
# Build with new datasource
make build

# Run digest
./bin/kora digest --since 16h --format json-pretty

# Verify Linear events appear
```

### 6.4 Run Tests

```bash
# Unit tests
make test

# Integration tests (requires Linear API access)
KORA_LINEAR_API_KEY="lin_api_..." make test-integration
```

## Reference Implementations

Study these existing implementations:

### GitHub DataSource
Location: `internal/datasources/github/datasource.go`

**Highlights:**
- CLI delegation pattern (no token storage)
- Multiple parallel API calls (PR reviews, mentions, issues, assignments)
- Deduplication by URL
- Comprehensive validation

**Use this as reference for:**
- Multiple query patterns
- CLI integration
- Error handling with partial success

### Slack DataSource
Location: `internal/datasources/slack/datasource.go`

**Highlights:**
- Token-based auth with HTTP client
- GraphQL-like search API
- DM and mention fetching
- Rate limit handling

**Use this as reference for:**
- HTTP API integration
- Token authentication
- Rate limit detection

## Testing Checklist

Before considering the datasource complete:

- [ ] All events pass `Event.Validate()`
- [ ] All metadata keys are in `allowedMetadataKeys`
- [ ] Priority follows EFA 0001 rules
- [ ] Credentials are redacted in logs
- [ ] Partial failure is handled gracefully
- [ ] Context cancellation is respected
- [ ] Events are sorted by timestamp
- [ ] Deduplication works correctly
- [ ] Unit tests cover success and failure cases
- [ ] Integration tests work with real API
- [ ] Test fixtures represent real API responses
- [ ] gosec scan passes
- [ ] golangci-lint passes

## Troubleshooting

### Events Failing Validation

**Problem:** Events returned but validation fails

**Solution:**
1. Check `Event.Validate()` error message
2. Verify all fields are populated correctly
3. Ensure metadata keys are in `allowedMetadataKeys`
4. Verify EventType is in `validEventTypes`

### Auth Failures

**Problem:** `ErrNotAuthenticated` returned

**Solution:**
1. Verify credential storage: `security find-generic-password -s "kora" -a "<key-name>"`
2. Check credential format validation in `IsValid()`
3. Test credential manually with API (curl)
4. Check auth provider logs for specific error

### Missing Events

**Problem:** API returns data but events don't appear

**Solution:**
1. Check timestamp filtering (events must be after `since`)
2. Verify deduplication isn't removing valid events
3. Check partial failure - errors may be hiding in `FetchResult.Errors`
4. Add debug logging to see raw API responses

### Rate Limiting

**Problem:** API returns 429 Too Many Requests

**Solution:**
1. Detect rate limiting: `resp.StatusCode == http.StatusTooManyRequests`
2. Set `FetchResult.RateLimited = true`
3. Parse `Retry-After` header
4. Return partial results with error

## Getting Help

If you're stuck:

1. Read the EFAs thoroughly
2. Study GitHub and Slack implementations
3. Check `docs/architecture.md` for system overview
4. Run with verbose logging: `KORA_DEBUG=1 kora digest`
5. Ask in GitHub issues with specific error messages

## Next Steps

After implementing a datasource:

1. Update `docs/architecture.md` with new datasource
2. Add example events to README
3. Update SECURITY.md if new auth pattern
4. Consider contributing back upstream (if public repo)
5. Plan next datasource based on user feedback
