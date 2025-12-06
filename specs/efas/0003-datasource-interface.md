---
authors: Samuel Dacanay <samuel@dakaneye.com>
state: draft
agents: golang-pro, documentation-engineer, prompt-engineer
---

# EFA 0003: DataSource Interface Ground Truth

This EFA defines the DataSource interface for fetching events from external services (GitHub, Slack). It specifies how datasources integrate with the authentication layer (EFA 0002) and produce normalized events (EFA 0001).

## Motivation & Prior Art

Datasources are the bridge between external services and Kora's event model. Without a strict interface definition, Claude may:
- Implement inconsistent fetch patterns across datasources
- Ignore rate limiting and error handling requirements
- Fail to handle partial success scenarios
- Mix credential handling into data fetching

**Goals:**
- Single DataSource interface all data fetchers implement
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
│           ┌───────────────────┼────────────────────┐                    │
│           ▼                   ▼                    ▼                    │
│  ┌─────────────────┐ ┌─────────────────┐ ┌─────────────────┐           │
│  │ GitHubDataSource│ │ SlackDataSource │ │ Future Sources  │           │
│  │                 │ │                 │ │                 │           │
│  │ AuthProvider ●──┼─┤ AuthProvider ●──┼─┤ AuthProvider ●  │           │
│  │                 │ │                 │ │                 │           │
│  └────────┬────────┘ └────────┬────────┘ └─────────────────┘           │
│           │                   │                                         │
│           ▼                   ▼                                         │
│    ┌────────────┐       ┌────────────┐                                 │
│    │ gh CLI API │       │ Slack API  │                                 │
│    └────────────┘       └────────────┘                                 │
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
// Each service (GitHub, Slack) has one DataSource implementation.
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
	// Format: lowercase with hyphens (e.g., "github-prs", "slack-mentions").
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

```go
// Package github implements the GitHub datasource using gh CLI delegation.
// Ground truth defined in specs/efas/0003-datasource-interface.md
//
// IT IS FORBIDDEN TO CHANGE the core fetch logic without updating EFA 0003.
package github

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/dakaneye/kora/internal/auth"
	"github.com/dakaneye/kora/internal/datasources"
	"github.com/dakaneye/kora/internal/models"
)

// DataSource fetches events from GitHub via gh CLI delegation.
//
// SECURITY: All API calls are delegated to GitHubDelegatedCredential.ExecuteAPI().
// This datasource NEVER sees or handles GitHub tokens directly.
type DataSource struct {
	authProvider auth.AuthProvider
	credential   *auth.GitHubDelegatedCredential
	orgs         []string // organizations to search
}

// DataSourceOption configures the GitHub DataSource.
type DataSourceOption func(*DataSource)

// WithOrgs limits searches to specific organizations.
func WithOrgs(orgs []string) DataSourceOption {
	return func(d *DataSource) {
		d.orgs = orgs
	}
}

// NewDataSource creates a GitHub datasource.
// The authProvider must return a GitHubDelegatedCredential.
func NewDataSource(authProvider auth.AuthProvider, opts ...DataSourceOption) (*DataSource, error) {
	if authProvider.Service() != auth.ServiceGitHub {
		return nil, fmt.Errorf("github datasource requires github auth provider")
	}

	d := &DataSource{
		authProvider: authProvider,
	}
	for _, opt := range opts {
		opt(d)
	}
	return d, nil
}

func (d *DataSource) Name() string           { return "github" }
func (d *DataSource) Service() models.Source { return models.SourceGitHub }

// Fetch retrieves GitHub events (PR reviews, mentions, issue assignments).
//
// Search strategy:
//  1. review-requested:@me is:open -draft:true (highest priority)
//  2. author:@me is:open (PRs with activity on your PRs)
//  3. mentions:@me type:pr (PR mentions)
//  4. mentions:@me type:issue (issue mentions)
//  5. assignee:@me is:open (assigned issues)
//
// SECURITY: Uses GitHubDelegatedCredential.ExecuteAPI() for all API calls.
// The GitHub token never leaves gh CLI's control.
func (d *DataSource) Fetch(ctx context.Context, opts datasources.FetchOptions) (*datasources.FetchResult, error) {
	if err := opts.Validate(); err != nil {
		return nil, fmt.Errorf("github fetch: %w", err)
	}

	// Get credential (validates auth)
	cred, err := d.authProvider.GetCredential(ctx)
	if err != nil {
		return nil, fmt.Errorf("github fetch: %w", datasources.ErrNotAuthenticated)
	}

	ghCred, ok := cred.(*auth.GitHubDelegatedCredential)
	if !ok {
		return nil, fmt.Errorf("github fetch: expected GitHubDelegatedCredential, got %T", cred)
	}

	result := &datasources.FetchResult{
		Stats: datasources.FetchStats{},
	}
	startTime := time.Now()

	// Execute all searches
	var allEvents []models.Event
	var fetchErrors []error

	// 1. PR review requests (highest priority)
	prReviews, err := d.fetchPRReviewRequests(ctx, ghCred, opts.Since)
	if err != nil {
		fetchErrors = append(fetchErrors, fmt.Errorf("pr reviews: %w", err))
	} else {
		allEvents = append(allEvents, prReviews...)
		result.Stats.APICallCount++
	}

	// 2. PR mentions
	prMentions, err := d.fetchPRMentions(ctx, ghCred, opts.Since)
	if err != nil {
		fetchErrors = append(fetchErrors, fmt.Errorf("pr mentions: %w", err))
	} else {
		allEvents = append(allEvents, prMentions...)
		result.Stats.APICallCount++
	}

	// 3. Issue mentions
	issueMentions, err := d.fetchIssueMentions(ctx, ghCred, opts.Since)
	if err != nil {
		fetchErrors = append(fetchErrors, fmt.Errorf("issue mentions: %w", err))
	} else {
		allEvents = append(allEvents, issueMentions...)
		result.Stats.APICallCount++
	}

	// 4. Assigned issues
	assignedIssues, err := d.fetchAssignedIssues(ctx, ghCred, opts.Since)
	if err != nil {
		fetchErrors = append(fetchErrors, fmt.Errorf("assigned issues: %w", err))
	} else {
		allEvents = append(allEvents, assignedIssues...)
		result.Stats.APICallCount++
	}

	// Deduplicate by URL
	allEvents = deduplicateEvents(allEvents)

	// Filter by options
	result.Stats.EventsFetched = len(allEvents)
	allEvents = filterEvents(allEvents, opts)
	result.Stats.EventsReturned = len(allEvents)

	// Apply limit
	if opts.Limit > 0 && len(allEvents) > opts.Limit {
		allEvents = allEvents[:opts.Limit]
	}

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
	result.Stats.Duration = time.Since(startTime)

	return result, nil
}

// fetchPRReviewRequests fetches PRs where user's review is requested.
// Query: review-requested:@me is:open -draft:true
func (d *DataSource) fetchPRReviewRequests(ctx context.Context, cred *auth.GitHubDelegatedCredential, since time.Time) ([]models.Event, error) {
	query := fmt.Sprintf("review-requested:@me is:open -draft:true updated:>=%s", since.Format("2006-01-02"))

	// Add org filter if configured
	if len(d.orgs) > 0 {
		query += " " + buildOrgFilter(d.orgs)
	}

	out, err := cred.ExecuteAPI(ctx, "search/issues",
		"-X", "GET",
		"-f", fmt.Sprintf("q=%s", query),
		"-f", "per_page=100",
	)
	if err != nil {
		return nil, fmt.Errorf("search pr reviews: %w", err)
	}

	var searchResult ghSearchResult
	if err := json.Unmarshal(out, &searchResult); err != nil {
		return nil, fmt.Errorf("parse pr reviews: %w", err)
	}

	events := make([]models.Event, 0, len(searchResult.Items))
	for _, item := range searchResult.Items {
		if !item.PullRequest.IsSet() {
			continue // Not a PR
		}

		event := models.Event{
			Type:           models.EventTypePRReview,
			Title:          truncateTitle(fmt.Sprintf("Review requested: %s", item.Title)),
			Source:         models.SourceGitHub,
			URL:            item.HTMLURL,
			Author:         models.Person{Name: item.User.Login, Username: item.User.Login},
			Timestamp:      item.UpdatedAt,
			Priority:       models.PriorityHigh, // PR reviews are high priority
			RequiresAction: true,
			Metadata: map[string]any{
				"repo":         item.RepositoryURL,
				"number":       item.Number,
				"state":        item.State,
				"review_state": "pending",
				"labels":       extractLabels(item.Labels),
			},
		}
		events = append(events, event)
	}

	return events, nil
}

// fetchPRMentions fetches PRs where user is mentioned.
// Query: mentions:@me type:pr
func (d *DataSource) fetchPRMentions(ctx context.Context, cred *auth.GitHubDelegatedCredential, since time.Time) ([]models.Event, error) {
	query := fmt.Sprintf("mentions:@me type:pr updated:>=%s", since.Format("2006-01-02"))

	if len(d.orgs) > 0 {
		query += " " + buildOrgFilter(d.orgs)
	}

	out, err := cred.ExecuteAPI(ctx, "search/issues",
		"-X", "GET",
		"-f", fmt.Sprintf("q=%s", query),
		"-f", "per_page=100",
	)
	if err != nil {
		return nil, fmt.Errorf("search pr mentions: %w", err)
	}

	var searchResult ghSearchResult
	if err := json.Unmarshal(out, &searchResult); err != nil {
		return nil, fmt.Errorf("parse pr mentions: %w", err)
	}

	events := make([]models.Event, 0, len(searchResult.Items))
	for _, item := range searchResult.Items {
		event := models.Event{
			Type:           models.EventTypePRMention,
			Title:          truncateTitle(fmt.Sprintf("Mentioned in PR: %s", item.Title)),
			Source:         models.SourceGitHub,
			URL:            item.HTMLURL,
			Author:         models.Person{Name: item.User.Login, Username: item.User.Login},
			Timestamp:      item.UpdatedAt,
			Priority:       models.PriorityMedium, // Mentions are medium priority
			RequiresAction: false,                 // May or may not need action
			Metadata: map[string]any{
				"repo":   item.RepositoryURL,
				"number": item.Number,
				"state":  item.State,
				"labels": extractLabels(item.Labels),
			},
		}
		events = append(events, event)
	}

	return events, nil
}

// fetchIssueMentions fetches issues where user is mentioned.
// Query: mentions:@me type:issue
func (d *DataSource) fetchIssueMentions(ctx context.Context, cred *auth.GitHubDelegatedCredential, since time.Time) ([]models.Event, error) {
	query := fmt.Sprintf("mentions:@me type:issue updated:>=%s", since.Format("2006-01-02"))

	if len(d.orgs) > 0 {
		query += " " + buildOrgFilter(d.orgs)
	}

	out, err := cred.ExecuteAPI(ctx, "search/issues",
		"-X", "GET",
		"-f", fmt.Sprintf("q=%s", query),
		"-f", "per_page=100",
	)
	if err != nil {
		return nil, fmt.Errorf("search issue mentions: %w", err)
	}

	var searchResult ghSearchResult
	if err := json.Unmarshal(out, &searchResult); err != nil {
		return nil, fmt.Errorf("parse issue mentions: %w", err)
	}

	events := make([]models.Event, 0, len(searchResult.Items))
	for _, item := range searchResult.Items {
		if item.PullRequest.IsSet() {
			continue // Skip PRs in issue search
		}

		event := models.Event{
			Type:           models.EventTypeIssueMention,
			Title:          truncateTitle(fmt.Sprintf("Mentioned in issue: %s", item.Title)),
			Source:         models.SourceGitHub,
			URL:            item.HTMLURL,
			Author:         models.Person{Name: item.User.Login, Username: item.User.Login},
			Timestamp:      item.UpdatedAt,
			Priority:       models.PriorityMedium,
			RequiresAction: false,
			Metadata: map[string]any{
				"repo":   item.RepositoryURL,
				"number": item.Number,
				"state":  item.State,
				"labels": extractLabels(item.Labels),
			},
		}
		events = append(events, event)
	}

	return events, nil
}

// fetchAssignedIssues fetches issues assigned to the user.
// Query: assignee:@me is:open
func (d *DataSource) fetchAssignedIssues(ctx context.Context, cred *auth.GitHubDelegatedCredential, since time.Time) ([]models.Event, error) {
	query := fmt.Sprintf("assignee:@me is:open type:issue updated:>=%s", since.Format("2006-01-02"))

	if len(d.orgs) > 0 {
		query += " " + buildOrgFilter(d.orgs)
	}

	out, err := cred.ExecuteAPI(ctx, "search/issues",
		"-X", "GET",
		"-f", fmt.Sprintf("q=%s", query),
		"-f", "per_page=100",
	)
	if err != nil {
		return nil, fmt.Errorf("search assigned issues: %w", err)
	}

	var searchResult ghSearchResult
	if err := json.Unmarshal(out, &searchResult); err != nil {
		return nil, fmt.Errorf("parse assigned issues: %w", err)
	}

	events := make([]models.Event, 0, len(searchResult.Items))
	for _, item := range searchResult.Items {
		event := models.Event{
			Type:           models.EventTypeIssueAssigned,
			Title:          truncateTitle(fmt.Sprintf("Assigned: %s", item.Title)),
			Source:         models.SourceGitHub,
			URL:            item.HTMLURL,
			Author:         models.Person{Name: item.User.Login, Username: item.User.Login},
			Timestamp:      item.UpdatedAt,
			Priority:       models.PriorityMedium,
			RequiresAction: true,
			Metadata: map[string]any{
				"repo":   item.RepositoryURL,
				"number": item.Number,
				"state":  item.State,
				"labels": extractLabels(item.Labels),
			},
		}
		events = append(events, event)
	}

	return events, nil
}

// GitHub API response types
type ghSearchResult struct {
	TotalCount int      `json:"total_count"`
	Items      []ghItem `json:"items"`
}

type ghItem struct {
	Number        int        `json:"number"`
	Title         string     `json:"title"`
	State         string     `json:"state"`
	HTMLURL       string     `json:"html_url"`
	RepositoryURL string     `json:"repository_url"`
	User          ghUser     `json:"user"`
	Labels        []ghLabel  `json:"labels"`
	UpdatedAt     time.Time  `json:"updated_at"`
	PullRequest   ghPR       `json:"pull_request"`
}

type ghUser struct {
	Login string `json:"login"`
}

type ghLabel struct {
	Name string `json:"name"`
}

type ghPR struct {
	URL string `json:"url"`
}

func (pr ghPR) IsSet() bool {
	return pr.URL != ""
}

// Helper functions

func buildOrgFilter(orgs []string) string {
	if len(orgs) == 0 {
		return ""
	}
	var parts []string
	for _, org := range orgs {
		parts = append(parts, fmt.Sprintf("org:%s", org))
	}
	return strings.Join(parts, " ")
}

func truncateTitle(title string) string {
	if len(title) <= 100 {
		return title
	}
	return title[:97] + "..."
}

func extractLabels(labels []ghLabel) []string {
	result := make([]string, len(labels))
	for i, l := range labels {
		result[i] = l.Name
	}
	return result
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

func filterEvents(events []models.Event, opts datasources.FetchOptions) []models.Event {
	if opts.Filter == nil {
		return events
	}

	result := make([]models.Event, 0, len(events))
	for _, e := range events {
		// Filter by event types
		if len(opts.Filter.EventTypes) > 0 {
			found := false
			for _, t := range opts.Filter.EventTypes {
				if e.Type == t {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		// Filter by priority
		if opts.Filter.MinPriority > 0 && e.Priority > opts.Filter.MinPriority {
			continue
		}

		// Filter by requires action
		if opts.Filter.RequiresAction && !e.RequiresAction {
			continue
		}

		result = append(result, e)
	}
	return result
}
```

### Slack DataSource Implementation

```go
// Package slack implements the Slack datasource using the Slack Web API.
// Ground truth defined in specs/efas/0003-datasource-interface.md
//
// IT IS FORBIDDEN TO CHANGE the core fetch logic without updating EFA 0003.
package slack

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/dakaneye/kora/internal/auth"
	"github.com/dakaneye/kora/internal/datasources"
	"github.com/dakaneye/kora/internal/models"
)

// DataSource fetches events from Slack (DMs and mentions).
//
// SECURITY: The Slack token is retrieved from auth.AuthProvider and used
// only for API calls. It is never logged or exposed.
type DataSource struct {
	authProvider auth.AuthProvider
	httpClient   *http.Client
	baseURL      string
	userID       string // cached user ID for mention searches
}

// DataSourceOption configures the Slack DataSource.
type DataSourceOption func(*DataSource)

// WithHTTPClient sets a custom HTTP client.
func WithHTTPClient(client *http.Client) DataSourceOption {
	return func(d *DataSource) {
		d.httpClient = client
	}
}

// WithBaseURL sets a custom Slack API base URL (for testing).
func WithBaseURL(baseURL string) DataSourceOption {
	return func(d *DataSource) {
		d.baseURL = baseURL
	}
}

// NewDataSource creates a Slack datasource.
func NewDataSource(authProvider auth.AuthProvider, opts ...DataSourceOption) (*DataSource, error) {
	if authProvider.Service() != auth.ServiceSlack {
		return nil, fmt.Errorf("slack datasource requires slack auth provider")
	}

	d := &DataSource{
		authProvider: authProvider,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		baseURL: "https://slack.com/api",
	}
	for _, opt := range opts {
		opt(d)
	}
	return d, nil
}

func (d *DataSource) Name() string           { return "slack" }
func (d *DataSource) Service() models.Source { return models.SourceSlack }

// Fetch retrieves Slack events (DMs and mentions).
//
// Search strategy:
//  1. Get user ID via auth.test
//  2. Search for @mentions: search.messages with <@USER_ID>
//  3. List DM conversations and fetch unread messages
//
// SECURITY: Token is obtained from AuthProvider and used only in Authorization header.
// It is never logged or included in error messages.
func (d *DataSource) Fetch(ctx context.Context, opts datasources.FetchOptions) (*datasources.FetchResult, error) {
	if err := opts.Validate(); err != nil {
		return nil, fmt.Errorf("slack fetch: %w", err)
	}

	// Get credential
	cred, err := d.authProvider.GetCredential(ctx)
	if err != nil {
		return nil, fmt.Errorf("slack fetch: %w", datasources.ErrNotAuthenticated)
	}

	token := cred.Value()
	if token == "" {
		return nil, fmt.Errorf("slack fetch: %w", datasources.ErrNotAuthenticated)
	}

	result := &datasources.FetchResult{
		Stats: datasources.FetchStats{},
	}
	startTime := time.Now()

	// Get user ID if not cached
	if d.userID == "" {
		userID, err := d.getUserID(ctx, token)
		if err != nil {
			return nil, fmt.Errorf("slack fetch: get user id: %w", err)
		}
		d.userID = userID
		result.Stats.APICallCount++
	}

	var allEvents []models.Event
	var fetchErrors []error

	// 1. Search for mentions
	mentions, err := d.fetchMentions(ctx, token, opts.Since)
	if err != nil {
		fetchErrors = append(fetchErrors, fmt.Errorf("mentions: %w", err))
	} else {
		allEvents = append(allEvents, mentions...)
	}

	// 2. Fetch DMs
	dms, err := d.fetchDMs(ctx, token, opts.Since)
	if err != nil {
		fetchErrors = append(fetchErrors, fmt.Errorf("dms: %w", err))
	} else {
		allEvents = append(allEvents, dms...)
	}

	// Deduplicate by URL
	allEvents = deduplicateEvents(allEvents)

	// Filter by options
	result.Stats.EventsFetched = len(allEvents)
	allEvents = filterEvents(allEvents, opts)
	result.Stats.EventsReturned = len(allEvents)

	// Apply limit
	if opts.Limit > 0 && len(allEvents) > opts.Limit {
		allEvents = allEvents[:opts.Limit]
	}

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
	result.Stats.Duration = time.Since(startTime)

	return result, nil
}

// getUserID retrieves the authenticated user's ID via auth.test.
func (d *DataSource) getUserID(ctx context.Context, token string) (string, error) {
	resp, err := d.apiRequest(ctx, token, "auth.test", nil)
	if err != nil {
		return "", err
	}

	var authResp struct {
		OK     bool   `json:"ok"`
		Error  string `json:"error"`
		UserID string `json:"user_id"`
	}
	if err := json.Unmarshal(resp, &authResp); err != nil {
		return "", fmt.Errorf("parse auth.test: %w", err)
	}
	if !authResp.OK {
		return "", fmt.Errorf("auth.test failed: %s", authResp.Error)
	}

	return authResp.UserID, nil
}

// fetchMentions searches for @mentions of the user.
// Uses search.messages API with query: <@USER_ID> after:DATE
func (d *DataSource) fetchMentions(ctx context.Context, token string, since time.Time) ([]models.Event, error) {
	query := fmt.Sprintf("<@%s> after:%s", d.userID, since.Format("2006-01-02"))

	params := url.Values{
		"query": {query},
		"count": {"100"},
		"sort":  {"timestamp"},
	}

	resp, err := d.apiRequest(ctx, token, "search.messages", params)
	if err != nil {
		return nil, err
	}

	var searchResp slackSearchResponse
	if err := json.Unmarshal(resp, &searchResp); err != nil {
		return nil, fmt.Errorf("parse search.messages: %w", err)
	}
	if !searchResp.OK {
		return nil, fmt.Errorf("search.messages failed: %s", searchResp.Error)
	}

	events := make([]models.Event, 0, len(searchResp.Messages.Matches))
	for _, msg := range searchResp.Messages.Matches {
		// Parse Slack timestamp to time.Time
		ts, err := parseSlackTimestamp(msg.TS)
		if err != nil {
			continue // Skip malformed timestamps
		}

		// Skip messages from before the since time
		if !ts.After(since) {
			continue
		}

		event := models.Event{
			Type:           models.EventTypeSlackMention,
			Title:          truncateTitle(stripMrkdwn(msg.Text)),
			Source:         models.SourceSlack,
			URL:            msg.Permalink,
			Author:         models.Person{Username: msg.Username, Name: msg.Username},
			Timestamp:      ts,
			Priority:       models.PriorityMedium, // Mentions are medium priority
			RequiresAction: false,
			Metadata: map[string]any{
				"workspace":       getWorkspaceFromPermalink(msg.Permalink),
				"channel":         msg.Channel.Name,
				"is_thread_reply": msg.TS != msg.ThreadTS && msg.ThreadTS != "",
			},
		}
		if msg.ThreadTS != "" {
			event.Metadata["thread_ts"] = msg.ThreadTS
		}

		events = append(events, event)
	}

	return events, nil
}

// fetchDMs retrieves direct messages since the given time.
// Strategy: List IM conversations, then fetch history for each.
func (d *DataSource) fetchDMs(ctx context.Context, token string, since time.Time) ([]models.Event, error) {
	// 1. List IM conversations
	params := url.Values{
		"types":            {"im"},
		"exclude_archived": {"true"},
		"limit":            {"100"},
	}

	resp, err := d.apiRequest(ctx, token, "users.conversations", params)
	if err != nil {
		return nil, err
	}

	var convResp slackConversationsResponse
	if err := json.Unmarshal(resp, &convResp); err != nil {
		return nil, fmt.Errorf("parse users.conversations: %w", err)
	}
	if !convResp.OK {
		return nil, fmt.Errorf("users.conversations failed: %s", convResp.Error)
	}

	// 2. Fetch history for each DM channel
	var allEvents []models.Event
	sinceUnix := fmt.Sprintf("%d.000000", since.Unix())

	for _, channel := range convResp.Channels {
		histParams := url.Values{
			"channel": {channel.ID},
			"oldest":  {sinceUnix},
			"limit":   {"50"},
		}

		histResp, err := d.apiRequest(ctx, token, "conversations.history", histParams)
		if err != nil {
			continue // Skip this channel on error
		}

		var history slackHistoryResponse
		if err := json.Unmarshal(histResp, &history); err != nil {
			continue
		}
		if !history.OK {
			continue
		}

		for _, msg := range history.Messages {
			// Skip own messages
			if msg.User == d.userID {
				continue
			}

			ts, err := parseSlackTimestamp(msg.TS)
			if err != nil {
				continue
			}

			// Skip messages from before the since time
			if !ts.After(since) {
				continue
			}

			event := models.Event{
				Type:           models.EventTypeSlackDM,
				Title:          truncateTitle(stripMrkdwn(msg.Text)),
				Source:         models.SourceSlack,
				URL:            buildDMPermalink(channel.ID, msg.TS),
				Author:         models.Person{Username: msg.User},
				Timestamp:      ts,
				Priority:       models.PriorityHigh, // DMs are high priority
				RequiresAction: true,
				Metadata: map[string]any{
					"is_thread_reply": msg.ThreadTS != "" && msg.TS != msg.ThreadTS,
				},
			}
			if msg.ThreadTS != "" {
				event.Metadata["thread_ts"] = msg.ThreadTS
			}

			allEvents = append(allEvents, event)
		}
	}

	return allEvents, nil
}

// apiRequest makes an authenticated request to the Slack API.
// SECURITY: Token is only used in Authorization header, never logged.
func (d *DataSource) apiRequest(ctx context.Context, token, method string, params url.Values) ([]byte, error) {
	reqURL := fmt.Sprintf("%s/%s", d.baseURL, method)
	if params != nil {
		reqURL += "?" + params.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	// SECURITY: Token is only used here, in the Authorization header.
	// It must never be logged or included in error messages.
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := d.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("api request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusTooManyRequests {
		// Rate limited
		retryAfter := resp.Header.Get("Retry-After")
		return nil, fmt.Errorf("%w: retry after %s", datasources.ErrRateLimited, retryAfter)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("api error: status %d", resp.StatusCode)
	}

	// Read response body with limit to prevent memory exhaustion
	body := make([]byte, 0, 1024*1024) // 1MB initial capacity
	buf := make([]byte, 32*1024)       // 32KB read buffer
	for {
		n, err := resp.Body.Read(buf)
		if n > 0 {
			body = append(body, buf[:n]...)
			if len(body) > 10*1024*1024 { // 10MB limit
				return nil, fmt.Errorf("response too large")
			}
		}
		if err != nil {
			break
		}
	}

	return body, nil
}

// Slack API response types

type slackSearchResponse struct {
	OK       bool   `json:"ok"`
	Error    string `json:"error"`
	Messages struct {
		Matches []slackMessage `json:"matches"`
	} `json:"messages"`
}

type slackConversationsResponse struct {
	OK       bool           `json:"ok"`
	Error    string         `json:"error"`
	Channels []slackChannel `json:"channels"`
}

type slackHistoryResponse struct {
	OK       bool           `json:"ok"`
	Error    string         `json:"error"`
	Messages []slackMessage `json:"messages"`
}

type slackChannel struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type slackMessage struct {
	TS        string `json:"ts"`
	ThreadTS  string `json:"thread_ts"`
	Text      string `json:"text"`
	User      string `json:"user"`
	Username  string `json:"username"`
	Permalink string `json:"permalink"`
	Channel   struct {
		Name string `json:"name"`
	} `json:"channel"`
}

// Helper functions

func parseSlackTimestamp(ts string) (time.Time, error) {
	// Slack timestamps are "1234567890.123456"
	parts := strings.Split(ts, ".")
	if len(parts) != 2 {
		return time.Time{}, fmt.Errorf("invalid slack timestamp: %s", ts)
	}

	var sec, usec int64
	if _, err := fmt.Sscanf(parts[0], "%d", &sec); err != nil {
		return time.Time{}, err
	}
	if _, err := fmt.Sscanf(parts[1], "%d", &usec); err != nil {
		return time.Time{}, err
	}

	return time.Unix(sec, usec*1000), nil
}

func truncateTitle(title string) string {
	// Remove newlines for single-line title
	title = strings.ReplaceAll(title, "\n", " ")
	title = strings.TrimSpace(title)

	if len(title) <= 100 {
		return title
	}
	return title[:97] + "..."
}

func stripMrkdwn(text string) string {
	// Basic mrkdwn stripping - remove user mentions format
	// <@U12345|name> -> @name
	// <#C12345|channel> -> #channel
	// <https://url|text> -> text

	result := text

	// User mentions
	for {
		start := strings.Index(result, "<@")
		if start == -1 {
			break
		}
		end := strings.Index(result[start:], ">")
		if end == -1 {
			break
		}
		mention := result[start : start+end+1]
		if idx := strings.Index(mention, "|"); idx != -1 {
			name := mention[idx+1 : len(mention)-1]
			result = result[:start] + "@" + name + result[start+end+1:]
		} else {
			result = result[:start] + "@user" + result[start+end+1:]
		}
	}

	// Channel mentions
	for {
		start := strings.Index(result, "<#")
		if start == -1 {
			break
		}
		end := strings.Index(result[start:], ">")
		if end == -1 {
			break
		}
		mention := result[start : start+end+1]
		if idx := strings.Index(mention, "|"); idx != -1 {
			name := mention[idx+1 : len(mention)-1]
			result = result[:start] + "#" + name + result[start+end+1:]
		} else {
			result = result[:start] + "#channel" + result[start+end+1:]
		}
	}

	return result
}

func getWorkspaceFromPermalink(permalink string) string {
	// Extract workspace from https://workspace.slack.com/archives/...
	u, err := url.Parse(permalink)
	if err != nil {
		return ""
	}
	parts := strings.Split(u.Host, ".")
	if len(parts) > 0 {
		return parts[0]
	}
	return ""
}

func buildDMPermalink(channelID, ts string) string {
	// Build a permalink for DM messages
	// Format: https://app.slack.com/client/T.../D.../p...
	return fmt.Sprintf("slack://channel?team=&id=%s&message=%s", channelID, strings.Replace(ts, ".", "", 1))
}

func deduplicateEvents(events []models.Event) []models.Event {
	seen := make(map[string]bool)
	result := make([]models.Event, 0, len(events))
	for _, e := range events {
		key := e.URL
		if key == "" {
			key = fmt.Sprintf("%s-%s-%s", e.Source, e.Type, e.Timestamp.String())
		}
		if !seen[key] {
			seen[key] = true
			result = append(result, e)
		}
	}
	return result
}

func filterEvents(events []models.Event, opts datasources.FetchOptions) []models.Event {
	if opts.Filter == nil {
		return events
	}

	result := make([]models.Event, 0, len(events))
	for _, e := range events {
		// Filter by event types
		if len(opts.Filter.EventTypes) > 0 {
			found := false
			for _, t := range opts.Filter.EventTypes {
				if e.Type == t {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		// Filter by priority
		if opts.Filter.MinPriority > 0 && e.Priority > opts.Filter.MinPriority {
			continue
		}

		// Filter by requires action
		if opts.Filter.RequiresAction && !e.RequiresAction {
			continue
		}

		result = append(result, e)
	}
	return result
}
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
    }
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

### Rule 7: New Datasources Require EFA Update

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

### Performance Implications

| Concern | Approach |
|---------|----------|
| Multiple datasources | Run concurrently via DataSourceRunner |
| Large response bodies | 10MB limit on API responses |
| Rate limiting | Detect and report, support partial results |
| Timeout handling | Per-datasource timeout via context |

### Testing Implications

1. **Mock HTTP responses** for unit tests
2. **Use testdata/** fixtures for API response examples
3. **Test partial failure** scenarios (some calls succeed, others fail)
4. **Test rate limiting** detection and reporting
5. **Validate all test events** pass Event.Validate()

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
```

## Open Questions

1. Should FetchOptions support pagination cursors for resumable fetches?
2. Should we add a `Health() error` method for pre-flight checks?
3. Should rate limit backoff be automatic or caller-controlled?
