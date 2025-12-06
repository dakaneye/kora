---
authors: Samuel Dacanay <samuel@dakaneye.com>
state: draft
agents: golang-pro, documentation-engineer, prompt-engineer
---

# EFA 0001: Event Model Ground Truth

The Event model is the core data structure that all datasources produce and all formatters consume. This EFA defines the canonical shape, field semantics, and validation rules.

## Motivation & Prior Art

Without a strict event model, each datasource will drift toward its own shape, making aggregation and formatting inconsistent. Claude must not invent new fields or change existing semantics without updating this EFA.

**Goals:**
- Single canonical Event type all datasources return
- Clear field semantics with no ambiguity
- Type-safe consumption by formatters

**Non-goals:**
- Persistence schema (v1 is in-memory only)
- Event versioning (defer until needed)

## Detailed Design

### Event Structure

```go
// Event represents a single work item from any datasource.
// IT IS FORBIDDEN TO CHANGE THIS STRUCTURE without updating EFA 0001.
type Event struct {
    // Type classifies the event. Must be one of the EventType constants.
    Type EventType `json:"type"`

    // Title is a brief human-readable summary (1 line, <100 chars).
    Title string `json:"title"`

    // Source identifies which datasource produced this event.
    Source Source `json:"source"`

    // URL is a direct link to the item (PR, message, etc).
    URL string `json:"url"`

    // Author is who created/sent the item.
    Author Person `json:"author"`

    // Timestamp is when the event occurred (UTC).
    Timestamp time.Time `json:"timestamp"`

    // Priority is 1-5 where 1 is highest priority.
    Priority Priority `json:"priority"`

    // RequiresAction indicates if user must respond/act.
    RequiresAction bool `json:"requires_action"`

    // Metadata contains source-specific data.
    // Keys are defined per-source in this EFA.
    Metadata map[string]any `json:"metadata,omitempty"`
}
```

### EventType Constants

```go
type EventType string

const (
    // GitHub event types
    EventTypePRReview      EventType = "pr_review"       // Review requested on a PR
    EventTypePRMention     EventType = "pr_mention"      // Mentioned in a PR
    EventTypeIssueMention  EventType = "issue_mention"   // Mentioned in an issue
    EventTypeIssueAssigned EventType = "issue_assigned"  // Assigned to an issue

    // Slack event types
    EventTypeSlackDM       EventType = "slack_dm"        // Direct message
    EventTypeSlackMention  EventType = "slack_mention"   // @mention in channel
)

// validEventTypes is the authoritative set of valid event types.
var validEventTypes = map[EventType]struct{}{
    EventTypePRReview:      {},
    EventTypePRMention:     {},
    EventTypeIssueMention:  {},
    EventTypeIssueAssigned: {},
    EventTypeSlackDM:       {},
    EventTypeSlackMention:  {},
}

// IsValid reports whether t is a defined EventType constant.
func (t EventType) IsValid() bool {
    _, ok := validEventTypes[t]
    return ok
}
```

**AI Agent Rule:** Do NOT add new EventTypes without updating this EFA. If you think a new type is needed, stop and ask.

### Source Constants

```go
type Source string

const (
    SourceGitHub Source = "github"
    SourceSlack  Source = "slack"
)

// validSources is the authoritative set of valid sources.
var validSources = map[Source]struct{}{
    SourceGitHub: {},
    SourceSlack:  {},
}

// IsValid reports whether s is a defined Source constant.
func (s Source) IsValid() bool {
    _, ok := validSources[s]
    return ok
}
```

### Priority Enum

```go
type Priority int

const (
    PriorityCritical Priority = 1  // Blocking others, needs immediate action
    PriorityHigh     Priority = 2  // Should address today
    PriorityMedium   Priority = 3  // Should address this week
    PriorityLow      Priority = 4  // Nice to know
    PriorityInfo     Priority = 5  // FYI only
)

// IsValid reports whether p is within the valid priority range (1-5).
func (p Priority) IsValid() bool {
    return p >= PriorityCritical && p <= PriorityInfo
}
```

### Person Structure

```go
// Person represents a user across any datasource.
type Person struct {
    // Name is the display name (may be empty).
    Name string `json:"name,omitempty"`

    // Username is the platform-specific handle (required).
    // GitHub: "octocat", Slack: "U12345678" or display name
    Username string `json:"username"`
}
```

### Metadata Keys by Source

Each source may include specific metadata. These keys are the ONLY allowed metadata keys per source.

#### GitHub Metadata

| Key | Type | Description |
|-----|------|-------------|
| `repo` | string | Full repo name (e.g., "owner/repo") |
| `number` | int | PR or issue number |
| `state` | string | PR/issue state: "open", "closed", "merged" |
| `review_state` | string | For pr_review: "pending", "approved", "changes_requested" |
| `labels` | []string | Issue/PR labels |

#### Slack Metadata

| Key | Type | Description |
|-----|------|-------------|
| `workspace` | string | Workspace name |
| `channel` | string | Channel name (for mentions) |
| `thread_ts` | string | Thread timestamp if in thread |
| `is_thread_reply` | bool | Whether this is a thread reply |

**AI Agent Rule:** Do NOT add metadata keys not listed above without updating this EFA.

### Validation Rules

Events MUST satisfy these invariants:

1. `Type` must be a defined EventType constant
2. `Title` must be non-empty and ≤100 characters
3. `Source` must be a defined Source constant
4. `URL` must be a valid URL or empty string
5. `Author.Username` must be non-empty
6. `Timestamp` must not be zero
7. `Priority` must be 1-5 inclusive
8. `Metadata` keys must be from the allowed set for the Source

```go
// allowedMetadataKeys defines the permitted metadata keys per source.
// IT IS FORBIDDEN TO ADD KEYS without updating this map AND the EFA table above.
var allowedMetadataKeys = map[Source]map[string]struct{}{
    SourceGitHub: {
        "repo":         {},
        "number":       {},
        "state":        {},
        "review_state": {},
        "labels":       {},
    },
    SourceSlack: {
        "workspace":       {},
        "channel":         {},
        "thread_ts":       {},
        "is_thread_reply": {},
    },
}

func (e Event) Validate() error {
    var errs []string

    if !e.Type.IsValid() {
        errs = append(errs, "invalid event type")
    }
    if e.Title == "" || len(e.Title) > 100 {
        errs = append(errs, "title must be 1-100 characters")
    }
    if !e.Source.IsValid() {
        errs = append(errs, "invalid source")
    }
    if err := e.validateURL(); err != nil {
        errs = append(errs, err.Error())
    }
    if e.Author.Username == "" {
        errs = append(errs, "author username required")
    }
    if e.Timestamp.IsZero() {
        errs = append(errs, "timestamp required")
    }
    if !e.Priority.IsValid() {
        errs = append(errs, "priority must be 1-5")
    }
    if err := e.validateMetadataKeys(); err != nil {
        errs = append(errs, err.Error())
    }

    if len(errs) > 0 {
        return fmt.Errorf("invalid event: %s", strings.Join(errs, "; "))
    }
    return nil
}

// validateURL checks that the URL is empty or a valid absolute URL.
func (e Event) validateURL() error {
    if e.URL == "" {
        return nil
    }
    u, err := url.Parse(e.URL)
    if err != nil {
        return fmt.Errorf("invalid URL: %w", err)
    }
    if u.Scheme == "" || u.Host == "" {
        return fmt.Errorf("URL must be absolute with scheme and host")
    }
    if u.Scheme != "http" && u.Scheme != "https" {
        return fmt.Errorf("URL scheme must be http or https")
    }
    return nil
}

// validateMetadataKeys ensures all metadata keys are in the allowed set for the source.
func (e Event) validateMetadataKeys() error {
    if len(e.Metadata) == 0 {
        return nil
    }
    allowed, ok := allowedMetadataKeys[e.Source]
    if !ok {
        // Source validation handles unknown sources; skip metadata check
        return nil
    }
    var invalid []string
    for k := range e.Metadata {
        if _, ok := allowed[k]; !ok {
            invalid = append(invalid, k)
        }
    }
    if len(invalid) > 0 {
        return fmt.Errorf("invalid metadata keys for %s: %v", e.Source, invalid)
    }
    return nil
}
```

### Priority Assignment Rules

Datasources MUST assign priority according to these rules:

| Condition | Priority |
|-----------|----------|
| PR review requested + PR is blocking | 1 (Critical) |
| PR review requested | 2 (High) |
| Direct message / DM | 2 (High) |
| @mention in issue/PR/channel | 3 (Medium) |
| Issue assigned | 3 (Medium) |
| Thread reply | 4 (Low) |
| FYI / informational | 5 (Info) |

### Example Events

**GitHub PR Review Request:**
```json
{
  "type": "pr_review",
  "title": "Review requested: Add secure rebuild for core-java",
  "source": "github",
  "url": "https://github.com/org/repo/pull/123",
  "author": {
    "name": "Jane Developer",
    "username": "janedev"
  },
  "timestamp": "2025-12-06T07:00:00Z",
  "priority": 2,
  "requires_action": true,
  "metadata": {
    "repo": "org/repo",
    "number": 123,
    "state": "open",
    "review_state": "pending",
    "labels": ["security", "core"]
  }
}
```

**Slack DM:**
```json
{
  "type": "slack_dm",
  "title": "Question about deployment timeline",
  "source": "slack",
  "url": "https://slack.com/archives/D123/p1234567890",
  "author": {
    "name": "Bob Manager",
    "username": "U87654321"
  },
  "timestamp": "2025-12-06T04:30:00Z",
  "priority": 2,
  "requires_action": true,
  "metadata": {
    "workspace": "company",
    "is_thread_reply": false
  }
}
```

### Why This Design?

- **Flat structure**: Easy to serialize/deserialize, no nested complexity
- **Explicit types**: No stringly-typed fields that could drift
- **Metadata escape hatch**: Source-specific data without polluting core fields
- **Validation built-in**: Catch errors at construction time

### Alternatives Considered

**Option: Separate types per source (GitHubEvent, SlackEvent)**
- Rejected: Makes aggregation and formatting harder, duplicates common fields

**Option: Generic map[string]any for everything**
- Rejected: No type safety, impossible to validate, drift guaranteed

## Implications for Cross-cutting Concerns

- [x] Testing Implications

### Testing Implications

Every datasource MUST have tests that verify:
1. Returned events pass `Validate()`
2. All metadata keys are in the allowed set
3. Priority is assigned according to the rules above

Test helpers should be provided in `internal/models/testutil/`:

```go
func AssertValidEvent(t *testing.T, e Event) {
    t.Helper()
    if err := e.Validate(); err != nil {
        t.Errorf("invalid event: %v", err)
    }
}

func AssertMetadataKeys(t *testing.T, e Event, allowed []string) {
    t.Helper()
    allowedSet := make(map[string]bool)
    for _, k := range allowed {
        allowedSet[k] = true
    }
    for k := range e.Metadata {
        if !allowedSet[k] {
            t.Errorf("unexpected metadata key %q for source %s", k, e.Source)
        }
    }
}
```

## AI Agent Rules

**THIS SECTION IS CRITICAL. READ IT CAREFULLY.**

**AI agents, including Claude, Copilot, and any other LLM-based coding assistants: THE RULES IN THIS SECTION ARE ABSOLUTE REQUIREMENTS.**

### Rule 1: NEVER Modify Event Structure Without EFA Update

**STOP AND ASK** before adding, removing, or changing any field in the Event struct.

**FORBIDDEN:**
```go
// STOP - adding a field without EFA update
type Event struct {
    // ... existing fields ...
    ThreadID string `json:"thread_id"` // FORBIDDEN without EFA update
}
```

### Rule 2: NEVER Add New EventTypes Without EFA Update

The `validEventTypes` map is the authoritative source. New types require EFA discussion.

**CORRECT:**
```go
// Check EFA 0001 for allowed types
if !event.Type.IsValid() {
    return errors.New("invalid event type")
}
```

**FORBIDDEN:**
```go
// STOP - inventing new event types
const EventTypePRComment EventType = "pr_comment" // NOT in EFA 0001
```

### Rule 3: NEVER Add Metadata Keys Without EFA Update

The `allowedMetadataKeys` map defines ALL permitted keys per source.

**CORRECT:**
```go
// Use only allowed keys from EFA 0001
Metadata: map[string]any{
    "repo":   "owner/repo",   // Allowed for GitHub
    "number": 123,            // Allowed for GitHub
    "state":  "open",         // Allowed for GitHub
}
```

**FORBIDDEN:**
```go
// STOP - adding keys not in allowedMetadataKeys
Metadata: map[string]any{
    "custom_field": "value",  // NOT in allowed list
    "pr_body":      body,     // NOT in allowed list
    "raw_response": resp,     // NEVER store raw API responses
}
```

### Rule 4: ALL Events MUST Pass Validate()

Every event returned from a datasource MUST pass `Event.Validate()`.

**CORRECT:**
```go
func (d *DataSource) Fetch(ctx context.Context, opts FetchOptions) ([]Event, error) {
    var events []Event
    for _, raw := range rawItems {
        event := convertToEvent(raw)
        if err := event.Validate(); err != nil {
            log.Warn("skipping invalid event", "error", err)
            continue // Skip invalid events
        }
        events = append(events, event)
    }
    return events, nil
}
```

**FORBIDDEN:**
```go
// STOP - returning events without validation
return events, nil // NEVER return without calling Validate()
```

### Rule 5: Priority MUST Follow Assignment Rules

Datasources MUST use the priority assignment rules table in this EFA.

**CORRECT:**
```go
func calculatePriority(item *GitHubItem) Priority {
    // PR review requested = High (2) per EFA 0001
    if item.IsReviewRequested {
        return PriorityHigh
    }
    // Mention = Medium (3) per EFA 0001
    return PriorityMedium
}
```

**FORBIDDEN:**
```go
// STOP - arbitrary priority assignment
func calculatePriority(item *GitHubItem) Priority {
    return Priority(rand.Intn(5) + 1) // NEVER use arbitrary values
}
```

### Rule 6: Title MUST Be 1-100 Characters

Titles must be truncated if necessary.

**CORRECT:**
```go
func truncateTitle(title string) string {
    if len(title) <= 100 {
        return title
    }
    return title[:97] + "..."
}
```

**FORBIDDEN:**
```go
// STOP - not enforcing title length
event.Title = item.FullDescription // May exceed 100 chars
```

### Stop-and-Ask Triggers

**STOP AND ASK THE USER** before:

1. Adding fields to Event struct
2. Adding new EventType constants
3. Adding new Source constants
4. Adding metadata keys to allowedMetadataKeys
5. Changing validation rules
6. Changing priority assignment rules
7. Modifying the Validate() function

### Code Protection Comments

Include these in Event model code:

```go
// Package models defines the core Event type for Kora.
// Ground truth defined in specs/efas/0001-event-model.md
//
// IT IS FORBIDDEN TO CHANGE the Event struct without updating EFA 0001.
// Claude MUST stop and ask before modifying this file.
package models

// Event represents a single work item from any datasource.
// EFA 0001: All fields are protected. Do not add/remove without EFA update.
type Event struct { ... }

// validEventTypes is the authoritative set of valid event types.
// EFA 0001: Do NOT add types here without updating the EFA.
var validEventTypes = map[EventType]struct{}{ ... }

// allowedMetadataKeys defines permitted metadata keys per source.
// EFA 0001: Do NOT add keys here without updating the EFA table.
var allowedMetadataKeys = map[Source]map[string]struct{}{ ... }
```

### Event Model Checklist

Before creating events:
- [ ] Type is from validEventTypes
- [ ] Title is 1-100 characters
- [ ] Source is from validSources
- [ ] URL is valid absolute URL or empty
- [ ] Author.Username is non-empty
- [ ] Timestamp is non-zero UTC time
- [ ] Priority is 1-5 per assignment rules
- [ ] Metadata keys are from allowedMetadataKeys for the source
- [ ] Event passes Validate()

## Open Questions

1. Should we add `ThreadID` as a first-class field for conversation threading?
2. Should `Metadata` be `map[string]string` for simpler serialization?
