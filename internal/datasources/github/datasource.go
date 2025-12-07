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
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/dakaneye/kora/internal/auth"
	"github.com/dakaneye/kora/internal/codeowners"
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
//
//nolint:govet // Field order prioritizes readability over memory alignment
type DataSource struct {
	authProvider auth.AuthProvider
	orgs         []string // organizations to search (optional filter)

	// CODEOWNERS support (optional)
	codeownersFetcher *codeowners.Fetcher      // fetches CODEOWNERS files from repos
	teamResolver      *codeowners.TeamResolver // resolves team memberships

	// Current user caching
	currentUser     string    // cached GitHub login
	currentUserOnce sync.Once // ensures single fetch
	currentUserErr  error     // cached error from user fetch
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

// WithCodeownersFetcher enables CODEOWNERS-based event detection.
// When set, the datasource will check if the current user owns changed
// files in PRs and create EventTypePRCodeowner events accordingly.
func WithCodeownersFetcher(fetcher *codeowners.Fetcher) Option {
	return func(d *DataSource) {
		d.codeownersFetcher = fetcher
	}
}

// WithTeamResolver enables team membership resolution for CODEOWNERS.
// When set, team references like @org/team in CODEOWNERS files will be
// resolved to check if the current user is a member.
func WithTeamResolver(resolver *codeowners.TeamResolver) Option {
	return func(d *DataSource) {
		d.teamResolver = resolver
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

// Fetch retrieves GitHub events (PR reviews, mentions, issue assignments, authored PRs).
//
// This implementation uses GraphQL for rich context. Search strategy:
//  1. review-requested:@me is:open -draft:true type:pr (highest priority)
//  2. mentions:@me type:pr (PR mentions)
//  3. mentions:@me type:issue (issue mentions)
//  4. assignee:@me is:open type:issue (assigned issues)
//  5. author:LOGIN is:open type:pr (user's own PRs for status tracking)
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

	if ctx.Err() != nil {
		result.Events = allEvents
		result.Errors = fetchErrors
		result.Partial = len(allEvents) > 0
		result.Stats.Duration = time.Since(startTime)
		return result, ctx.Err()
	}

	// 5. Authored PRs - GraphQL
	// Track user's own open PRs to show CI failures, review state, etc.
	authoredPRs, err := d.fetchAuthoredPRsGraphQL(ctx, gqlClient, ghCred, opts.Since, d.orgs)
	if err != nil {
		fetchErrors = append(fetchErrors, fmt.Errorf("authored prs: %w", err))
	} else {
		allEvents = append(allEvents, authoredPRs...)
		result.Stats.APICallCount++
	}

	// Deduplicate by URL (same item can appear in multiple searches)
	// Also merges user_relationships when same PR appears for multiple reasons
	allEvents = models.DeduplicateEvents(allEvents)

	// 6. Check CODEOWNERS for PR events (optional)
	// Only process if codeownersFetcher is configured
	if d.codeownersFetcher != nil {
		currentUser, userErr := d.getCurrentUser(ctx, ghCred)
		if userErr != nil {
			// Non-fatal: log error but continue without CODEOWNERS
			fetchErrors = append(fetchErrors, fmt.Errorf("codeowners: %w", userErr))
		} else if currentUser != "" {
			// Check each PR event for CODEOWNERS matches
			var codeownerEvents []models.Event
		codeownerLoop:
			for i := range allEvents {
				// Only process PR events (not issues)
				eventType := allEvents[i].Type
				if eventType != models.EventTypePRReview &&
					eventType != models.EventTypePRMention &&
					eventType != models.EventTypePRAuthor {
					continue
				}

				// Check context cancellation
				select {
				case <-ctx.Done():
					// Partial success with what we have
					break codeownerLoop
				default:
				}

				codeownerEvent, err := d.checkCodeownerEvents(ctx, &allEvents[i], currentUser)
				if err != nil {
					// Non-fatal: log but continue
					fetchErrors = append(fetchErrors, err)
					continue
				}
				if codeownerEvent != nil {
					codeownerEvents = append(codeownerEvents, *codeownerEvent)
				}
			}

			// Add codeowner events
			if len(codeownerEvents) > 0 {
				allEvents = append(allEvents, codeownerEvents...)
				// Deduplicate again in case same PR appears as both reviewer and codeowner
				allEvents = models.DeduplicateEvents(allEvents)
			}
		}
	}

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

// getCurrentUser fetches and caches the current GitHub user login.
// Uses sync.Once to ensure only one API call is made across multiple invocations.
//
// Returns the cached user login or an error if the fetch failed.
func (d *DataSource) getCurrentUser(ctx context.Context, cred githubCredential) (string, error) {
	d.currentUserOnce.Do(func() {
		// Call gh api user to get current user info
		out, err := cred.ExecuteAPI(ctx, "user")
		if err != nil {
			d.currentUserErr = fmt.Errorf("fetching current user: %w", err)
			return
		}

		var user struct {
			Login string `json:"login"`
		}
		if err := json.Unmarshal(out, &user); err != nil {
			d.currentUserErr = fmt.Errorf("parsing user response: %w", err)
			return
		}

		if user.Login == "" {
			d.currentUserErr = fmt.Errorf("empty login in user response")
			return
		}

		d.currentUser = user.Login
	})

	return d.currentUser, d.currentUserErr
}

// checkCodeownerEvents checks if the user is a codeowner of changed files in a PR.
// Returns a codeowner event if the user owns files but is NOT already a reviewer.
//
// This method is only called when codeownersFetcher is configured.
//
// Per EFA 0001:
//   - EventType: models.EventTypePRCodeowner
//   - Priority: models.PriorityMedium (3)
//   - RequiresAction: true
//   - user_relationships includes "codeowner"
func (d *DataSource) checkCodeownerEvents(
	ctx context.Context,
	event *models.Event,
	currentUser string,
) (*models.Event, error) {
	// Skip if no CODEOWNERS fetcher configured
	if d.codeownersFetcher == nil {
		return nil, nil
	}

	// Extract repo from metadata
	repo, repoOK := event.Metadata["repo"].(string)
	if !repoOK || repo == "" {
		return nil, nil
	}

	// Check existing user_relationships - skip if already a reviewer
	if existingRels, relsOK := event.Metadata["user_relationships"].([]string); relsOK {
		for _, rel := range existingRels {
			if rel == "reviewer" || rel == "requested_reviewer" {
				// Already a reviewer, don't create duplicate codeowner event
				return nil, nil
			}
		}
	}

	// Also check review_requests to see if user is explicitly requested
	if reviewRequests, reqOK := event.Metadata["review_requests"].([]map[string]any); reqOK {
		for _, req := range reviewRequests {
			if login, loginOK := req["login"].(string); loginOK {
				if strings.EqualFold(login, currentUser) {
					// User is explicitly requested as reviewer
					return nil, nil
				}
			}
		}
	}

	// Check reviews to see if user has already reviewed
	if reviews, revOK := event.Metadata["reviews"].([]map[string]any); revOK {
		for _, review := range reviews {
			if author, authOK := review["author"].(string); authOK {
				if strings.EqualFold(author, currentUser) {
					// User has already reviewed this PR
					return nil, nil
				}
			}
		}
	}

	// Get CODEOWNERS ruleset for the repo
	ruleset, err := d.codeownersFetcher.GetRuleset(ctx, repo)
	if err != nil {
		// Non-fatal: log and continue
		return nil, fmt.Errorf("fetching CODEOWNERS for %s: %w", repo, err)
	}
	if ruleset == nil {
		// No CODEOWNERS file in this repo
		return nil, nil
	}

	// Get files_changed from metadata
	filesChanged, filesOK := event.Metadata["files_changed"].([]map[string]any)
	if !filesOK || len(filesChanged) == 0 {
		return nil, nil
	}

	// Check each file against CODEOWNERS
	isCodeowner := false
	for _, file := range filesChanged {
		path, pathOK := file["path"].(string)
		if !pathOK || path == "" {
			continue
		}

		owners := ruleset.Match(path)
		if len(owners) == 0 {
			continue
		}

		// Check if current user is an owner
		for _, owner := range owners {
			// Direct user match (with or without @ prefix)
			ownerClean := strings.TrimPrefix(owner, "@")
			if strings.EqualFold(ownerClean, currentUser) {
				isCodeowner = true
				break
			}

			// Team reference like @org/team
			if strings.Contains(ownerClean, "/") && d.teamResolver != nil {
				isMember, err := d.teamResolver.IsMember(ctx, ownerClean, currentUser)
				if err != nil {
					// Non-fatal: team resolution failed, continue checking
					continue
				}
				if isMember {
					isCodeowner = true
					break
				}
			}
		}

		if isCodeowner {
			break
		}
	}

	if !isCodeowner {
		return nil, nil
	}

	// User is a codeowner - create event
	// Copy metadata and add codeowner to relationships
	newMetadata := make(map[string]any, len(event.Metadata))
	for k, v := range event.Metadata {
		newMetadata[k] = v
	}

	// Update user_relationships to include codeowner
	var relationships []string
	if existingRels, existingOK := event.Metadata["user_relationships"].([]string); existingOK {
		relationships = make([]string, len(existingRels), len(existingRels)+1)
		copy(relationships, existingRels)
	}
	relationships = append(relationships, "codeowner")
	newMetadata["user_relationships"] = relationships

	codeownerEvent := models.Event{
		Type:           models.EventTypePRCodeowner,
		Title:          truncateTitle(fmt.Sprintf("You own files in: %s", event.Title)),
		Source:         models.SourceGitHub,
		URL:            event.URL,
		Author:         event.Author,
		Timestamp:      event.Timestamp,
		Priority:       models.PriorityMedium, // Per EFA 0001
		RequiresAction: true,
		Metadata:       newMetadata,
	}

	return &codeownerEvent, nil
}
