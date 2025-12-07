---
authors: Samuel Dacanay <samuel@dakaneye.com>
state: draft
agents: golang-pro, documentation-engineer, prompt-engineer
---

# EFA 0003: DataSource Interface Ground Truth

This EFA defines the DataSource interface for fetching events from external services (GitHub, Google Calendar, Gmail, and future datasources). It specifies how datasources integrate with the authentication layer (EFA 0002) and produce normalized events (EFA 0001).

## Motivation & Prior Art

Datasources are the bridge between external services and Kora's event model. Without a strict interface definition, Claude may:
- Implement inconsistent fetch patterns across datasources
- Ignore rate limiting and error handling requirements
- Fail to handle partial success scenarios
- Mix credential handling into data fetching

**Goals:**
- Single canonical DataSource interface all data fetchers implement
- Clear concurrency model for parallel fetching
- Proper error handling with partial success support
- Rate limit awareness built into the interface

**Non-goals:**
- Caching layer (v1 is in-memory, single invocation)
- Webhook/push-based data sources
- Datasource auto-discovery or plugins

## Detailed Design

### Architecture Overview

```
┌─────────────────────────────────────────────────────────────────────────┐
│                           Digest Engine                                  │
│                                                                          │
│  ┌──────────────────────────────────────────────────────────────────┐   │
│  │                       DataSourceRunner                            │   │
│  │  - Runs datasources concurrently via errgroup                    │   │
│  │  - Aggregates events from all sources                            │   │
│  │  - Handles partial failures gracefully                           │   │
│  └──────────────────────────────────────────────────────────────────┘   │
│                                │                                         │
│           ┌───────────────────┼────────────────────────────────────┐    │
│           ▼                   ▼                ▼                    ▼    │
│  ┌─────────────────┐ ┌─────────────────┐ ┌──────────────┐ ┌──────────┐ │
│  │ GitHubDataSource│ │GoogleCalendar   │ │GmailDataSrc  │ │ Future   │ │
│  │                 │ │DataSource       │ │              │ │ Sources  │ │
│  │ AuthProvider ●──┼─┤AuthProvider ●───┼─┤AuthProvider●─┼─┤          │ │
│  │                 │ │                 │ │              │ │          │ │
│  └────────┬────────┘ └────────┬────────┘ └──────┬───────┘ └──────────┘ │
│           │                   │                  │                       │
│           ▼                   ▼                  ▼                       │
│    ┌────────────┐       ┌──────────────┐  ┌──────────────┐             │
│    │ gh CLI API │       │ Calendar API │  │  Gmail API   │             │
│    └────────────┘       └──────────────┘  └──────────────┘             │
└─────────────────────────────────────────────────────────────────────────┘
```

### DataSource Interface

```go
// Package datasources provides abstractions for fetching events from external services.
// Ground truth defined in specs/efas/0003-datasource-interface.md
//
// IT IS FORBIDDEN TO CHANGE the DataSource interface without updating EFA 0003.
package datasources

import (
	"context"
	"errors"
	"time"

	"github.com/dakaneye/kora/internal/models"
)

// DataSource fetches events from an external service.
// Each service (GitHub, Linear, Calendar, Gmail) has one DataSource implementation.
//
// Implementations must:
//   - Respect context cancellation at all stages
//   - Handle rate limiting gracefully (backoff, partial results)
//   - Return partial results when possible (some calls succeed, others fail)
//   - Never log or expose credentials (delegate to AuthProvider)
//   - Validate all events before returning (use Event.Validate())
//
// IT IS FORBIDDEN TO CHANGE THIS INTERFACE without updating EFA 0003.
type DataSource interface {
	// Name returns a human-readable identifier for logging.
	// Format: lowercase with hyphens (e.g., "github-prs", "linear-issues").
	Name() string

	// Service returns which service this datasource connects to.
	// Used for grouping events and associating with AuthProviders.
	Service() models.Source

	// Fetch retrieves events since the given timestamp.
	// Returns events that occurred after 'since' (exclusive).
	//
	// Error handling:
	//   - Returns (events, nil) on full success
	//   - Returns (events, err) on partial success (some events retrieved)
	//   - Returns (nil, err) on complete failure
	//
	// The returned events MUST:
	//   - Pass Event.Validate()
	//   - Have Timestamp > since
	//   - Be sorted by Timestamp ascending
	//
	// Context handling:
	//   - Respect ctx.Done() for cancellation
	//   - Return ctx.Err() if cancelled
	//   - Use ctx for all network operations
	Fetch(ctx context.Context, opts FetchOptions) (*FetchResult, error)
}

// FetchOptions configures a Fetch operation.
// IT IS FORBIDDEN TO ADD FIELDS without updating EFA 0003.
type FetchOptions struct {
	// Since is the exclusive lower bound for event timestamps.
	// Events with Timestamp <= Since are excluded.
	// Required: must not be zero.
	Since time.Time

	// Limit is the maximum number of events to return.
	// 0 means no limit (use service default, typically 100).
	// Implementations should respect rate limits over this limit.
	Limit int

	// Filter contains optional filter criteria.
	// Interpretation is datasource-specific.
	Filter *FetchFilter
}

// FetchFilter provides optional filtering criteria.
// Not all datasources support all filters; unsupported filters are ignored.
type FetchFilter struct {
	// EventTypes limits results to specific event types.
	// Empty slice means all types supported by the datasource.
	EventTypes []models.EventType

	// MinPriority filters to events with priority <= this value.
	// 0 means no priority filter.
	// Remember: priority 1 is highest, 5 is lowest.
	MinPriority models.Priority

	// RequiresAction filters to only actionable events.
	// false means all events, true means only RequiresAction=true.
	RequiresAction bool
}

// Validate checks that FetchOptions are valid.
func (o FetchOptions) Validate() error {
	if o.Since.IsZero() {
		return errors.New("FetchOptions.Since is required")
	}
	if o.Limit < 0 {
		return errors.New("FetchOptions.Limit must be non-negative")
	}
	return nil
}

// FetchResult contains the results of a Fetch operation.
type FetchResult struct {
	// Events contains the fetched events, sorted by Timestamp ascending.
	Events []models.Event

	// Partial indicates some events may be missing due to errors.
	// When true, Errors contains details about what failed.
	Partial bool

	// Errors contains non-fatal errors encountered during fetch.
	// These did not prevent returning partial results.
	Errors []error

	// RateLimited indicates the fetch was cut short due to rate limiting.
	// The caller may retry after RateLimitReset.
	RateLimited bool

	// RateLimitReset is when rate limiting expires (zero if not rate limited).
	RateLimitReset time.Time

	// Stats contains fetch statistics for observability.
	Stats FetchStats
}

// FetchStats provides observability data about a fetch operation.
type FetchStats struct {
	// Duration is how long the fetch took.
	Duration time.Duration

	// APICallCount is the number of API calls made.
	APICallCount int

	// EventsFetched is the total events before filtering.
	EventsFetched int

	// EventsReturned is the count after filtering.
	EventsReturned int
}

// HasEvents returns true if any events were fetched.
func (r *FetchResult) HasEvents() bool {
	return len(r.Events) > 0
}

// HasErrors returns true if any errors occurred.
func (r *FetchResult) HasErrors() bool {
	return len(r.Errors) > 0
}

// CombinedError returns all errors as a single error, or nil if none.
func (r *FetchResult) CombinedError() error {
	if !r.HasErrors() {
		return nil
	}
	return errors.Join(r.Errors...)
}
```

### Sentinel Errors

```go
// Sentinel errors for datasource operations.
// IT IS FORBIDDEN TO ADD ERRORS without updating EFA 0003.
var (
	// ErrNotAuthenticated indicates the datasource's auth provider has no valid credentials.
	ErrNotAuthenticated = errors.New("datasource: not authenticated")

	// ErrRateLimited indicates the service rate limit was exceeded.
	// Check FetchResult.RateLimitReset for when to retry.
	ErrRateLimited = errors.New("datasource: rate limited")

	// ErrServiceUnavailable indicates the external service is down or unreachable.
	ErrServiceUnavailable = errors.New("datasource: service unavailable")

	// ErrTimeout indicates the operation exceeded the context deadline.
	ErrTimeout = errors.New("datasource: timeout")

	// ErrInvalidResponse indicates the service returned malformed data.
	ErrInvalidResponse = errors.New("datasource: invalid response")
)
```

### DataSourceRunner

```go
// DataSourceRunner executes multiple datasources concurrently.
// IT IS FORBIDDEN TO CHANGE THIS TYPE without updating EFA 0003.
type DataSourceRunner struct {
	sources []DataSource
	timeout time.Duration
}

// RunnerOption configures a DataSourceRunner.
type RunnerOption func(*DataSourceRunner)

// WithTimeout sets the per-datasource timeout.
// Default is 30 seconds.
func WithTimeout(d time.Duration) RunnerOption {
	return func(r *DataSourceRunner) {
		r.timeout = d
	}
}

// NewRunner creates a DataSourceRunner with the given datasources.
func NewRunner(sources []DataSource, opts ...RunnerOption) *DataSourceRunner {
	r := &DataSourceRunner{
		sources: sources,
		timeout: 30 * time.Second,
	}
	for _, opt := range opts {
		opt(r)
	}
	return r
}

// Run executes all datasources concurrently and aggregates results.
//
// Execution model:
//   - Each datasource runs in its own goroutine
//   - Per-datasource timeout is applied via context
//   - Failures in one datasource do not affect others
//   - Results are aggregated and sorted by timestamp
//
// The returned RunResult contains:
//   - All events from successful datasources
//   - Per-datasource errors for failed datasources
//   - Statistics for observability
func (r *DataSourceRunner) Run(ctx context.Context, opts FetchOptions) (*RunResult, error) {
	if err := opts.Validate(); err != nil {
		return nil, fmt.Errorf("invalid fetch options: %w", err)
	}

	type sourceResult struct {
		name   string
		result *FetchResult
		err    error
	}

	results := make(chan sourceResult, len(r.sources))

	g, gctx := errgroup.WithContext(ctx)

	for _, src := range r.sources {
		src := src // capture loop variable
		g.Go(func() error {
			// Apply per-datasource timeout
			srcCtx, cancel := context.WithTimeout(gctx, r.timeout)
			defer cancel()

			result, err := src.Fetch(srcCtx, opts)
			results <- sourceResult{
				name:   src.Name(),
				result: result,
				err:    err,
			}
			// Don't return error - we want all datasources to run
			return nil
		})
	}

	// Wait for all goroutines to complete
	_ = g.Wait()
	close(results)

	// Aggregate results
	runResult := &RunResult{
		SourceResults: make(map[string]*FetchResult),
		SourceErrors:  make(map[string]error),
	}

	for sr := range results {
		if sr.err != nil {
			runResult.SourceErrors[sr.name] = sr.err
			// Still include partial results if available
			if sr.result != nil && sr.result.HasEvents() {
				runResult.SourceResults[sr.name] = sr.result
				runResult.Events = append(runResult.Events, sr.result.Events...)
			}
		} else if sr.result != nil {
			runResult.SourceResults[sr.name] = sr.result
			runResult.Events = append(runResult.Events, sr.result.Events...)
		}
	}

	// Sort all events by timestamp
	sort.Slice(runResult.Events, func(i, j int) bool {
		return runResult.Events[i].Timestamp.Before(runResult.Events[j].Timestamp)
	})

	return runResult, nil
}

// RunResult contains aggregated results from all datasources.
type RunResult struct {
	// Events contains all events from all datasources, sorted by Timestamp.
	Events []models.Event

	// SourceResults contains per-datasource results.
	SourceResults map[string]*FetchResult

	// SourceErrors contains per-datasource errors for failed datasources.
	SourceErrors map[string]error
}

// Success returns true if all datasources succeeded.
func (r *RunResult) Success() bool {
	return len(r.SourceErrors) == 0
}

// Partial returns true if some datasources succeeded but others failed.
func (r *RunResult) Partial() bool {
	return len(r.SourceErrors) > 0 && len(r.SourceResults) > 0
}

// TotalEvents returns the count of all fetched events.
func (r *RunResult) TotalEvents() int {
	return len(r.Events)
}
```

### GitHub DataSource Implementation

The GitHub datasource uses a two-phase GraphQL approach for rich context:

#### Phase 1: Search for Items
Uses GitHub search queries to discover relevant PRs and issues:
1. `review-requested:@me is:open -draft:true type:pr` - Review requests (highest priority)
2. `mentions:@me type:pr` - PR mentions
3. `mentions:@me type:issue` - Issue mentions
4. `assignee:@me is:open type:issue` - Assigned issues
5. `author:@me is:open type:pr` - User's own PRs (for status tracking)

Search queries use `SearchPRsQuery` and `SearchIssuesQuery` GraphQL operations which return basic info (number, title, URL, updatedAt, repository, author).

#### Phase 2: Fetch Full Context
For each search result, fetch complete metadata using `PRQuery` or `IssueQuery` GraphQL operations. These queries fetch all EFA 0001 metadata fields in a single request:

**PR Context includes:**
- Review requests (user vs team, with team slugs)
- Reviews (author, state: approved/changes_requested/commented)
- CI checks (name, status, conclusion) and rollup state
- Files changed (path, additions, deletions)
- Labels, milestone, linked issues
- Comments counts, unresolved threads
- Draft status, mergeability
- Branch names (head_ref, base_ref)
- Timestamps (created_at, updated_at)

**Issue Context includes:**
- Assignees
- Labels, milestone
- Comments (recent 10 with author, body, created_at)
- Linked PRs (via cross-references)
- Reactions (counts by type)
- Timeline summary (assigned, labeled, mentioned events)
- Timestamps

#### CODEOWNERS Integration
After fetching events, the GitHub datasource optionally checks CODEOWNERS:

1. **Get current user login** via `gh api user` (cached)
2. **For each PR event**, check if user is a codeowner:
   - Fetch CODEOWNERS file for the repository
   - Check each changed file path against CODEOWNERS patterns
   - Match user login or team membership
3. **Create EventTypePRCodeowner** if:
   - User owns changed files
   - User is NOT already an explicit reviewer
   - User has NOT already reviewed the PR

This prevents duplicate events for PRs where the user is both a codeowner and an explicit reviewer.

#### Event Deduplication
The same PR/issue can appear in multiple searches (e.g., user is mentioned AND requested as reviewer). Events are deduplicated by URL, with relationships merged:

```go
// Before deduplication:
// Event 1: pr_mention, user_relationships=["mentioned"]
// Event 2: pr_review, user_relationships=["direct_reviewer"]

// After deduplication (higher priority wins):
// Event: pr_review, user_relationships=["mentioned", "direct_reviewer"]
```

#### Priority Calculation
Priorities follow EFA 0001 rules:

**PR Review Requests:**
- Direct user request → Priority 2 (High), relationship "direct_reviewer"
- Team-only request → Priority 3 (Medium), relationship "team_reviewer"

**PR Author (own PRs):**
- CI failing/error → Priority 1 (Critical), title "CI failing: ...", RequiresAction=true
- Changes requested → Priority 2 (High), title "Changes requested: ...", RequiresAction=true
- Has approvals → Priority 3 (Medium), title "Ready to merge: ...", RequiresAction=false
- Awaiting review → Priority 3 (Medium), title "Awaiting review: ...", RequiresAction=false
- Default → Priority 3 (Medium), title "Your PR: ...", RequiresAction=false

**PR Codeowner:**
- Always Priority 3 (Medium), RequiresAction=true, title "You own files in: ..."

**Mentions and Assignments:**
- Always Priority 3 (Medium)

#### Security: gh CLI Delegation
All API calls use `GitHubDelegatedCredential.ExecuteAPI()` which delegates to `gh api` CLI. The datasource never sees or handles GitHub tokens directly (per EFA 0002).

### Google Calendar DataSource Implementation

The Google Calendar datasource fetches upcoming meetings and events requiring user attention.

#### API Calls
Uses Google Calendar API v3:
- `calendars.list()` - Get user's calendar list
- `events.list()` - List events in a time range

#### Filtering Rules

**Include:**
- Accepted meetings (responseStatus: "accepted")
- Tentative meetings (responseStatus: "tentative")
- Events needing RSVP (responseStatus: "needsAction")
- Events user is organizing with pending RSVPs (has attendees with "needsAction")
- All-day events (for context)

**Exclude:**
- Declined events (responseStatus: "declined")
- Events outside the Since window (start time before Since)
- Past events (end time before Now)

**Recurring Events:**
- Only fetch the next occurrence within the time window
- Use `singleEvents=true` parameter to expand recurring events

#### Event Type and Priority Calculation

```go
func calculateCalendarEventType(event CalendarEvent, now time.Time) (EventType, Priority, bool) {
	// Check if upcoming within 1 hour
	if event.StartTime.Sub(now) <= time.Hour && event.StartTime.After(now) {
		return EventTypeCalendarUpcoming, PriorityHigh, false
	}

	// Check response status
	switch event.ResponseStatus {
	case "needsAction":
		return EventTypeCalendarNeedsRSVP, PriorityMedium, true
	case "tentative":
		return EventTypeCalendarTentative, PriorityMedium, false
	case "accepted":
		if event.IsAllDay {
			return EventTypeCalendarAllDay, PriorityInfo, false
		}
		return EventTypeCalendarMeeting, PriorityMedium, false
	}

	// Check if user is organizer awaiting responses
	if event.IsOrganizer && len(event.PendingRSVPs) > 0 {
		return EventTypeCalendarOrganizerPending, PriorityMedium, true
	}

	return EventTypeCalendarAllDay, PriorityInfo, false
}
```

**Priority Rules:**
- `calendar_upcoming` (starts within 1hr) → Priority 2 (High), RequiresAction=false
- `calendar_needs_rsvp` (no response) → Priority 3 (Medium), RequiresAction=true
- `calendar_organizer_pending` (organizing, awaiting RSVPs) → Priority 3 (Medium), RequiresAction=true
- `calendar_tentative` → Priority 3 (Medium), RequiresAction=false
- `calendar_meeting` (accepted) → Priority 3 (Medium), RequiresAction=false
- `calendar_all_day` → Priority 4 (Info), RequiresAction=false

#### Concurrent Fetching
Multiple calendars are fetched concurrently using errgroup:

```go
func (d *GoogleCalendarDataSource) Fetch(ctx context.Context, opts FetchOptions) (*FetchResult, error) {
	g, gctx := errgroup.WithContext(ctx)

	var mu sync.Mutex
	var allEvents []models.Event
	var errors []error

	for _, calendarID := range d.calendarIDs {
		calendarID := calendarID // capture loop variable
		g.Go(func() error {
			events, err := d.fetchCalendar(gctx, calendarID, opts)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				errors = append(errors, err)
			} else {
				allEvents = append(allEvents, events...)
			}
			return nil // Don't fail entire fetch on single calendar error
		})
	}

	_ = g.Wait()

	// Sort by start time
	sort.Slice(allEvents, func(i, j int) bool {
		return allEvents[i].Timestamp.Before(allEvents[j].Timestamp)
	})

	result := &FetchResult{
		Events: allEvents,
		Errors: errors,
		Partial: len(errors) > 0 && len(allEvents) > 0,
	}
	return result, nil
}
```

#### Auth Delegation
Uses GoogleAuthProvider (EFA 0002) for OAuth credentials. Never handles tokens directly:

```go
cred, err := d.authProvider.GetCredential(ctx)
if err != nil {
	return nil, fmt.Errorf("calendar auth: %w", err)
}

// Use credential for API calls
req, _ := http.NewRequestWithContext(ctx, "GET", calendarURL, nil)
req.Header.Set("Authorization", "Bearer "+cred.Value())
```

### Gmail DataSource Implementation

The Gmail datasource fetches unread and unreplied emails from real people (excluding automated emails and mailing lists).

#### API Calls
Uses Gmail API v1:
- `messages.list()` - Search for messages with query
- `messages.get()` - Get full message with headers
- `messages.batchGet()` - Batch fetch multiple messages (up to 100)

#### Gmail Queries

Two primary queries to capture different email states:

```go
const (
	// Query 1: Unread emails
	queryUnread = "is:unread after:%d"

	// Query 2: Unreplied emails where user is direct recipient
	queryUnreplied = "in:inbox -in:sent after:%d"
)
```

#### Filtering Rules

**Include:**
- Unread from real people (no List-Unsubscribe header)
- Unreplied where user is direct recipient (in To: field)
- Important senders (configured per account)

**Exclude:**
- Mailing lists (List-Unsubscribe header present)
- Automated senders (noreply@, no-reply@, donotreply@, notifications@)
- Read AND replied (user has sent a reply in the thread)

**Important Senders:**
Configurable per Gmail account in config:

```yaml
google:
  gmail:
    - email: work@company.com
      important_senders:
        - manager@company.com
        - vp@company.com
```

#### Event Type and Priority Calculation

```go
func calculateEmailPriority(msg GmailMessage, importantSenders []string) (EventType, Priority, bool) {
	// Check if from important sender or Gmail-marked important
	if msg.IsImportant || isFromImportantSender(msg.FromEmail, importantSenders) {
		return EventTypeEmailImportant, PriorityHigh, true
	}

	// Check if directly addressed (in To: field) and unread
	if msg.IsDirectlyAddressed && msg.IsUnread {
		return EventTypeEmailDirect, PriorityMedium, true
	}

	// CC'd emails
	if msg.IsCCd {
		return EventTypeEmailCC, PriorityLow, false
	}

	return EventTypeEmailCC, PriorityLow, false
}
```

**Priority Rules:**
- `email_important` (important sender or Gmail-marked) → Priority 2 (High), RequiresAction=true
- `email_direct` (user in To:, unread) → Priority 3 (Medium), RequiresAction=true
- `email_cc` (user in CC:) → Priority 4 (Low), RequiresAction=false

#### Deduplication
Same message can appear in both unread and unreplied queries. Deduplicate by message_id:

```go
func deduplicateEmails(events []models.Event) []models.Event {
	seen := make(map[string]bool)
	var deduplicated []models.Event

	// Sort by priority (higher priority first)
	sort.Slice(events, func(i, j int) bool {
		return events[i].Priority < events[j].Priority
	})

	for _, event := range events {
		messageID := event.Metadata["message_id"].(string)
		if !seen[messageID] {
			seen[messageID] = true
			deduplicated = append(deduplicated, event)
		}
	}

	return deduplicated
}
```

#### Mailing List Detection

Check for List-Unsubscribe header:

```go
func isMailingList(headers map[string]string) bool {
	// List-Unsubscribe header indicates a mailing list
	if _, ok := headers["List-Unsubscribe"]; ok {
		return true
	}
	// Also check for List-Id
	if _, ok := headers["List-Id"]; ok {
		return true
	}
	return false
}
```

#### Automated Sender Detection

```go
var automatedSenderPatterns = []*regexp.Regexp{
	regexp.MustCompile(`^noreply@`),
	regexp.MustCompile(`^no-reply@`),
	regexp.MustCompile(`^donotreply@`),
	regexp.MustCompile(`^notifications@`),
	regexp.MustCompile(`^automated@`),
	regexp.MustCompile(`^bot@`),
}

func isAutomatedSender(email string) bool {
	for _, pattern := range automatedSenderPatterns {
		if pattern.MatchString(strings.ToLower(email)) {
			return true
		}
	}
	return false
}
```

#### Auth Delegation
Uses GoogleAuthProvider (EFA 0002) for OAuth credentials. Same auth provider instance shared with GoogleCalendarDataSource:

```go
cred, err := d.authProvider.GetCredential(ctx)
if err != nil {
	return nil, fmt.Errorf("gmail auth: %w", err)
}

// Use credential for API calls
req, _ := http.NewRequestWithContext(ctx, "GET", gmailURL, nil)
req.Header.Set("Authorization", "Bearer "+cred.Value())
```

## AI Agent Guidelines

**THIS SECTION IS CRITICAL. READ IT CAREFULLY.**

**AI agents, including Claude, Copilot, and any other LLM-based coding assistants: THE RULES IN THIS SECTION ARE ABSOLUTE REQUIREMENTS.**

### Rule 1: Preserve the DataSource Interface

**NEVER modify the DataSource interface without updating this EFA first.**

The interface has these exact methods:
- `Name() string`
- `Service() models.Source`
- `Fetch(ctx context.Context, opts FetchOptions) (*FetchResult, error)`

Adding, removing, or changing method signatures requires EFA update.

### Rule 2: Context Must Be Respected

**ALL network operations MUST use the provided context.**

**CORRECT:**
```go
func (d *DataSource) Fetch(ctx context.Context, opts FetchOptions) (*FetchResult, error) {
    resp, err := d.client.DoWithContext(ctx, req)
    // ...
}
```

**FORBIDDEN:**
```go
func (d *DataSource) Fetch(ctx context.Context, opts FetchOptions) (*FetchResult, error) {
    resp, err := d.client.Do(req)  // STOP. Not using context.
}
```

### Rule 3: Partial Success Must Be Supported

**Datasources MUST return partial results when possible.**

If 3 out of 4 API calls succeed, return the events from the successful calls and report errors in `FetchResult.Errors`.

**CORRECT:**
```go
result := &FetchResult{}
events1, err1 := d.fetchPRs(ctx)
if err1 != nil {
    result.Errors = append(result.Errors, err1)
} else {
    result.Events = append(result.Events, events1...)
    result.Stats.APICallCount++
}
// Continue with other fetches...
result.Partial = len(result.Errors) > 0 && len(result.Events) > 0
```

**FORBIDDEN:**
```go
events1, err1 := d.fetchPRs(ctx)
if err1 != nil {
    return nil, err1  // STOP. Fails entire fetch on first error.
}
```

### Rule 4: Events Must Be Validated

**ALL returned events MUST pass Event.Validate() as defined in EFA 0001.**

```go
for _, event := range events {
    if err := event.Validate(); err != nil {
        // Log and skip, or add to errors
        fetchErrors = append(fetchErrors, fmt.Errorf("validation: %w", err))
        continue
    }
    validEvents = append(validEvents, event)
}
```

### Rule 5: Credentials Must Not Be Logged

**NEVER log credential values. See EFA 0002 for full requirements.**

The datasource receives credentials from AuthProvider. It must:
- Use credentials only for API authentication
- Never log credential values
- Never include credentials in error messages

### Rule 6: Rate Limiting Must Be Handled

**Datasources MUST detect and report rate limiting.**

```go
if resp.StatusCode == http.StatusTooManyRequests {
    result.RateLimited = true
    if retryAfter := resp.Header.Get("Retry-After"); retryAfter != "" {
        // Parse and set result.RateLimitReset
    }
    return result, ErrRateLimited
}
```

### Rule 7: GraphQL Queries Must Fetch Complete Context

**For GitHub datasource, use PRQuery/IssueQuery to fetch ALL EFA 0001 metadata fields.**

The two-phase approach is required:
1. Search for items (lightweight)
2. Fetch full context for each item (complete metadata)

**CORRECT:**
```go
// Phase 1: Search
items, err := searchPRs(ctx, client, "review-requested:@me", 100)

// Phase 2: Fetch full context for each
for _, item := range items {
    metadata, err := fetchPRFullContext(ctx, client, item.Owner, item.Repo, item.Number)
    // metadata contains ALL fields from PRQuery
}
```

**FORBIDDEN:**
```go
// STOP - using search results only, missing rich context
items, err := searchPRs(ctx, client, query, 100)
for _, item := range items {
    // Missing: full context fetch
    event := models.Event{
        Metadata: map[string]any{
            "repo": item.Repository,  // Missing all other metadata!
        },
    }
}
```

### Rule 8: CODEOWNERS Check Must Not Duplicate Reviewers

**When checking CODEOWNERS, skip PRs where user is already an explicit reviewer.**

**CORRECT:**
```go
// Check existing review requests
for _, req := range reviewRequests {
    if req["login"] == currentUser && req["type"] == "user" {
        // Already a reviewer, don't create codeowner event
        return nil, nil
    }
}

// Check existing reviews
for _, review := range reviews {
    if review["author"] == currentUser {
        // Already reviewed, don't create codeowner event
        return nil, nil
    }
}

// Only now check CODEOWNERS
if isCodeowner {
    return createCodeownerEvent(event), nil
}
```

**FORBIDDEN:**
```go
// STOP - creating codeowner event without checking reviewer status
if isCodeowner {
    return createCodeownerEvent(event), nil  // May duplicate reviewer event
}
```

### Rule 9: user_relationships Must Be Populated

**All GitHub events MUST populate the user_relationships metadata field.**

See EFA 0001 for valid relationship values:
- "author" - User created this PR
- "direct_reviewer" - User directly requested
- "team_reviewer" - User's team requested
- "mentioned" - User @mentioned
- "codeowner" - User owns changed files
- "assignee" - User assigned to issue

### Rule 10: Priority Must Follow EFA 0001 Rules

**Priority calculation MUST use the rules defined in EFA 0001.**

For PR author events, check CI status first, then review state:
```go
if ciRollup == "failure" || ciRollup == "error" {
    return PriorityCritical // 1
}
if hasChangesRequested {
    return PriorityHigh // 2
}
return PriorityMedium // 3
```

For PR review requests, check request type:
```go
if hasDirectUserRequest {
    return PriorityHigh // 2
}
return PriorityMedium // 3 (team-only)
```

### Rule 11: Google Calendar Must Fetch All Configured Calendars Concurrently

**Google Calendar datasource MUST use errgroup to fetch multiple calendars in parallel.**

**CORRECT:**
```go
g, gctx := errgroup.WithContext(ctx)
for _, calendarID := range d.calendarIDs {
    calendarID := calendarID // capture
    g.Go(func() error {
        events, err := d.fetchCalendar(gctx, calendarID, opts)
        // Aggregate events...
        return nil // Don't fail entire fetch
    })
}
_ = g.Wait()
```

**FORBIDDEN:**
```go
// STOP - sequential fetching is too slow
for _, calendarID := range d.calendarIDs {
    events, err := d.fetchCalendar(ctx, calendarID, opts)
    // ...
}
```

### Rule 12: Gmail Must Filter Mailing Lists

**Gmail datasource MUST check List-Unsubscribe header to exclude mailing lists.**

**CORRECT:**
```go
headers := getMessageHeaders(message)
if _, ok := headers["List-Unsubscribe"]; ok {
    // Skip mailing list
    continue
}
if _, ok := headers["List-Id"]; ok {
    // Skip mailing list
    continue
}
```

**FORBIDDEN:**
```go
// STOP - not filtering mailing lists
// All emails will be included, including spam and newsletters
```

### Rule 13: Gmail Must Detect Automated Senders

**Gmail datasource MUST filter automated sender patterns (noreply@, no-reply@, etc).**

**CORRECT:**
```go
if isAutomatedSender(fromEmail) {
    // Skip automated sender
    continue
}

func isAutomatedSender(email string) bool {
    patterns := []string{"noreply@", "no-reply@", "donotreply@", "notifications@"}
    for _, pattern := range patterns {
        if strings.HasPrefix(strings.ToLower(email), pattern) {
            return true
        }
    }
    return false
}
```

**FORBIDDEN:**
```go
// STOP - not filtering automated senders
// Will include system notifications and automated emails
```

### Rule 14: Google Datasources Must Use GoogleAuthProvider

**Google Calendar and Gmail datasources MUST use GoogleAuthProvider, never handle tokens directly.**

**CORRECT:**
```go
cred, err := d.authProvider.GetCredential(ctx)
if err != nil {
    return nil, fmt.Errorf("calendar auth: %w", err)
}
req.Header.Set("Authorization", "Bearer "+cred.Value())
```

**FORBIDDEN:**
```go
// STOP - extracting token from keychain directly
token, err := d.keychain.Get(ctx, "google-token")  // Bypass auth provider
req.Header.Set("Authorization", "Bearer "+token)
```

### Rule 15: New Datasources Require EFA Update

**Adding a new datasource implementation requires updating this EFA** with:
- Service constant in models.Source (EFA 0001)
- Auth provider in auth.Service (EFA 0002)
- Implementation pattern documented here

### Stop and Ask Triggers

**STOP AND ASK THE USER** if you encounter:

1. **Need to change DataSource interface**: Requires EFA update discussion
2. **New sentinel error needed**: Add to this EFA first
3. **Different concurrency model**: Discuss before implementing
4. **Credential handling changes**: Review EFA 0002 first
5. **New metadata keys**: Update EFA 0001 first
6. **Change to GraphQL queries**: Update query documentation in this EFA
7. **Change to Google Calendar event filtering**: Discuss filtering rules first
8. **Change to Gmail query logic**: Discuss query patterns and filtering first
9. **New automated sender pattern**: Add to automatedSenderPatterns list
10. **New mailing list detection**: Document in this EFA

### Code Protection Comments

Include these in datasource code:

```go
// Package datasources provides abstractions for fetching events.
// Ground truth defined in specs/efas/0003-datasource-interface.md
//
// IT IS FORBIDDEN TO CHANGE the DataSource interface without updating EFA 0003.
// Claude MUST stop and ask before modifying interface methods.
package datasources

// Fetch retrieves events since the given timestamp.
// EFA 0003: Context must be used for all network operations.
// EFA 0003: Partial success must be supported.
// EFA 0001: All returned events must pass Validate().
func (d *DataSource) Fetch(ctx context.Context, opts FetchOptions) (*FetchResult, error)
```

## Implications for Cross-cutting Concerns

- [x] Security Implications
- [x] Performance Implications
- [x] Testing Implications

### Security Implications

| Threat | Mitigation |
|--------|------------|
| Credential exposure in logs | Datasources delegate auth to AuthProvider, never log tokens |
| Credential in error messages | Error messages reference source, not credentials |
| MITM attacks | TLS 1.2+ required, certificate validation enabled |
| Response tampering | Validate all events before returning |
| OAuth token exposure | GoogleAuthProvider handles tokens, datasources never see them |
| Expired OAuth tokens | GoogleAuthProvider auto-refreshes before API calls |

### Performance Implications

| Concern | Approach |
|---------|----------|
| Multiple datasources | Run concurrently via DataSourceRunner |
| Large response bodies | GraphQL queries fetch only needed fields |
| Rate limiting | Detect and report, support partial results |
| Timeout handling | Per-datasource timeout via context |
| Duplicate API calls | Two-phase approach: search once, fetch context per item |
| Multiple Google calendars | Concurrent fetching via errgroup |
| Gmail batch operations | Use batchGet() for up to 100 messages |

### Testing Implications

1. **Mock HTTP responses** for unit tests
2. **Use testdata/** fixtures for API response examples
3. **Test partial failure** scenarios (some calls succeed, others fail)
4. **Test rate limiting** detection and reporting
5. **Validate all test events** pass Event.Validate()
6. **Test CODEOWNERS** matching logic with various patterns
7. **Test deduplication** merges relationships correctly
8. **Test priority** calculation for all combinations
9. **Test Google Calendar** event filtering (accepted, tentative, needsAction)
10. **Test Gmail** mailing list detection (List-Unsubscribe header)
11. **Test Gmail** automated sender detection (noreply patterns)
12. **Test Gmail** important sender matching

```go
// Example test structure
func TestGitHubDataSource_Fetch(t *testing.T) {
    tests := []struct {
        name          string
        mockResponses map[string]string // endpoint -> response file
        wantEvents    int
        wantErrors    int
        wantPartial   bool
    }{
        {
            name: "all searches succeed",
            mockResponses: map[string]string{
                "search/issues?q=review-requested": "testdata/github_pr_reviews.json",
                "search/issues?q=mentions":         "testdata/github_mentions.json",
            },
            wantEvents:  5,
            wantErrors:  0,
            wantPartial: false,
        },
        {
            name: "partial success - one search fails",
            mockResponses: map[string]string{
                "search/issues?q=review-requested": "testdata/github_pr_reviews.json",
                "search/issues?q=mentions":         "", // empty = 500 error
            },
            wantEvents:  3,
            wantErrors:  1,
            wantPartial: true,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // Setup mock server, auth provider
            // Execute fetch
            // Assert results
        })
    }
}

func TestGoogleCalendarDataSource_Fetch(t *testing.T) {
    tests := []struct {
        name       string
        calEvents  []CalendarEvent
        wantTypes  []models.EventType
        wantCount  int
    }{
        {
            name: "upcoming meeting within 1 hour",
            calEvents: []CalendarEvent{
                {StartTime: time.Now().Add(45 * time.Minute), ResponseStatus: "accepted"},
            },
            wantTypes: []models.EventType{models.EventTypeCalendarUpcoming},
            wantCount: 1,
        },
        {
            name: "needs RSVP",
            calEvents: []CalendarEvent{
                {StartTime: time.Now().Add(2 * time.Hour), ResponseStatus: "needsAction"},
            },
            wantTypes: []models.EventType{models.EventTypeCalendarNeedsRSVP},
            wantCount: 1,
        },
    }
    // ...
}

func TestGmailDataSource_Fetch(t *testing.T) {
    tests := []struct {
        name         string
        messages     []GmailMessage
        wantFiltered int
        wantTypes    []models.EventType
    }{
        {
            name: "filters mailing lists",
            messages: []GmailMessage{
                {Headers: map[string]string{"List-Unsubscribe": "<...>"}},
                {Headers: map[string]string{"From": "person@example.com"}},
            },
            wantFiltered: 1,
        },
        {
            name: "filters automated senders",
            messages: []GmailMessage{
                {From: "noreply@github.com"},
                {From: "person@example.com"},
            },
            wantFiltered: 1,
        },
    }
    // ...
}
```

## Open Questions

1. Should FetchOptions support pagination cursors for resumable fetches?
2. Should we add a `Health() error` method for pre-flight checks?
3. Should rate limit backoff be automatic or caller-controlled?
4. Should Gmail datasource support custom query filters from config?
5. Should Google Calendar support filtering by specific calendars in FetchOptions?
