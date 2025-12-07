// Package github implements the GitHub datasource using gh CLI delegation.
// Ground truth defined in specs/efas/0003-datasource-interface.md
//
// IT IS FORBIDDEN TO CHANGE the core fetch logic without updating EFA 0003.
// Claude MUST stop and ask before modifying interface implementations.
//
// SECURITY: All API calls are delegated to GitHubDelegatedCredential.ExecuteAPI().
// This datasource NEVER sees or handles GitHub tokens directly.
// See EFA 0002 for credential security requirements.
package github

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/dakaneye/kora/internal/auth"
	"github.com/dakaneye/kora/internal/datasources"
	"github.com/dakaneye/kora/internal/models"
)

// githubCredential is an internal interface for GitHub credentials that can execute API calls.
// This allows both real and mock credentials to be used in tests.
type githubCredential interface {
	ExecuteAPI(ctx context.Context, endpoint string, args ...string) ([]byte, error)
}

// DataSource fetches events from GitHub via gh CLI delegation.
//
// SECURITY: All API calls are delegated to GitHubDelegatedCredential.ExecuteAPI().
// This datasource NEVER sees or handles GitHub tokens directly.
type DataSource struct {
	authProvider auth.AuthProvider
	orgs         []string // organizations to search (optional filter)
}

// Option configures the GitHub DataSource.
type Option func(*DataSource)

// WithOrgs limits searches to specific organizations.
// When set, all search queries will include org:X filters.
func WithOrgs(orgs []string) Option {
	return func(d *DataSource) {
		d.orgs = orgs
	}
}

// NewDataSource creates a GitHub datasource.
// The authProvider must be a GitHub auth provider (Service() == auth.ServiceGitHub).
//
// Returns an error if the authProvider is not for GitHub service.
func NewDataSource(authProvider auth.AuthProvider, opts ...Option) (*DataSource, error) {
	if authProvider.Service() != auth.ServiceGitHub {
		return nil, fmt.Errorf("github datasource requires github auth provider, got %s", authProvider.Service())
	}

	d := &DataSource{
		authProvider: authProvider,
	}
	for _, opt := range opts {
		opt(d)
	}
	return d, nil
}

// Name returns the datasource identifier for logging.
func (d *DataSource) Name() string {
	return "github"
}

// Service returns the service this datasource connects to.
func (d *DataSource) Service() models.Source {
	return models.SourceGitHub
}

// Fetch retrieves GitHub events (PR reviews, mentions, issue assignments).
//
// This implementation uses GraphQL for rich context. Search strategy:
//  1. review-requested:@me is:open -draft:true type:pr (highest priority)
//  2. mentions:@me type:pr (PR mentions)
//  3. mentions:@me type:issue (issue mentions)
//  4. assignee:@me is:open type:issue (assigned issues)
//
// For each search result, full context is fetched via PRQuery/IssueQuery
// to populate all EFA 0001 metadata fields.
//
// EFA 0003: Context must be used for all network operations.
// EFA 0003: Partial success must be supported.
// EFA 0001: All returned events must pass Validate().
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

	// Type assert to GitHubDelegatedCredential per EFA 0002
	// The credential must implement ExecuteAPI for delegated API access
	ghCred, ok := cred.(githubCredential)
	if !ok {
		return nil, fmt.Errorf("github fetch: credential does not support ExecuteAPI, got %T", cred)
	}

	// Create GraphQL client for rich context fetching
	gqlClient := NewGraphQLClient(ghCred)

	result := &datasources.FetchResult{
		Stats: datasources.FetchStats{},
	}
	startTime := time.Now()

	// Execute all searches with partial success support (EFA 0003)
	var allEvents []models.Event
	var fetchErrors []error

	// 1. PR review requests (highest priority) - GraphQL
	prReviews, err := d.fetchPRReviewRequestsGraphQL(ctx, gqlClient, opts.Since, d.orgs)
	if err != nil {
		fetchErrors = append(fetchErrors, fmt.Errorf("pr reviews: %w", err))
	} else {
		allEvents = append(allEvents, prReviews...)
		result.Stats.APICallCount++ // Search call (context fetches are counted per item internally)
	}

	// Check context cancellation between calls
	if ctx.Err() != nil {
		result.Events = allEvents
		result.Errors = fetchErrors
		result.Partial = len(allEvents) > 0
		result.Stats.Duration = time.Since(startTime)
		return result, ctx.Err()
	}

	// 2. PR mentions - GraphQL
	prMentions, err := d.fetchPRMentionsGraphQL(ctx, gqlClient, opts.Since, d.orgs)
	if err != nil {
		fetchErrors = append(fetchErrors, fmt.Errorf("pr mentions: %w", err))
	} else {
		allEvents = append(allEvents, prMentions...)
		result.Stats.APICallCount++
	}

	if ctx.Err() != nil {
		result.Events = allEvents
		result.Errors = fetchErrors
		result.Partial = len(allEvents) > 0
		result.Stats.Duration = time.Since(startTime)
		return result, ctx.Err()
	}

	// 3. Issue mentions - GraphQL
	issueMentions, err := d.fetchIssueMentionsGraphQL(ctx, gqlClient, opts.Since, d.orgs)
	if err != nil {
		fetchErrors = append(fetchErrors, fmt.Errorf("issue mentions: %w", err))
	} else {
		allEvents = append(allEvents, issueMentions...)
		result.Stats.APICallCount++
	}

	if ctx.Err() != nil {
		result.Events = allEvents
		result.Errors = fetchErrors
		result.Partial = len(allEvents) > 0
		result.Stats.Duration = time.Since(startTime)
		return result, ctx.Err()
	}

	// 4. Assigned issues - GraphQL
	assignedIssues, err := d.fetchAssignedIssuesGraphQL(ctx, gqlClient, opts.Since, d.orgs)
	if err != nil {
		fetchErrors = append(fetchErrors, fmt.Errorf("assigned issues: %w", err))
	} else {
		allEvents = append(allEvents, assignedIssues...)
		result.Stats.APICallCount++
	}

	// Deduplicate by URL (same item can appear in multiple searches)
	allEvents = deduplicateEvents(allEvents)

	// Record fetched count before filtering
	result.Stats.EventsFetched = len(allEvents)

	// Filter by options
	allEvents = filterEvents(allEvents, opts)
	result.Stats.EventsReturned = len(allEvents)

	// Apply limit
	if opts.Limit > 0 && len(allEvents) > opts.Limit {
		allEvents = allEvents[:opts.Limit]
	}

	// Sort by timestamp ascending (EFA 0003 requirement)
	sort.Slice(allEvents, func(i, j int) bool {
		return allEvents[i].Timestamp.Before(allEvents[j].Timestamp)
	})

	// Validate all events (EFA 0001 requirement)
	validEvents := make([]models.Event, 0, len(allEvents))
	for i := range allEvents {
		if err := allEvents[i].Validate(); err != nil {
			fetchErrors = append(fetchErrors, fmt.Errorf("event validation: %w", err))
			continue
		}
		validEvents = append(validEvents, allEvents[i])
	}

	result.Events = validEvents
	result.Errors = fetchErrors
	result.Partial = len(fetchErrors) > 0 && len(validEvents) > 0
	result.Stats.Duration = time.Since(startTime)

	return result, nil
}
