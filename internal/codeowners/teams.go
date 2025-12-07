// Package codeowners provides fetching and parsing for GitHub CODEOWNERS files.
//
// This file implements team membership resolution for CODEOWNERS matching.
// Teams referenced in CODEOWNERS files (e.g., @org/team-name) need their
// memberships resolved to match against the current user.
//
// SECURITY: All API calls are delegated to the credential's ExecuteAPI method.
// This code NEVER sees or handles GitHub tokens directly.
// See EFA 0002 for credential security requirements.
package codeowners

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
)

// TeamResolver resolves team memberships via the GitHub API.
// It caches team membership lists to avoid redundant API calls when
// checking multiple CODEOWNERS references to the same team.
//
// Thread-safety: All public methods are safe for concurrent use.
// The cache is protected by a read-write mutex to allow multiple
// concurrent reads while serializing writes.
type TeamResolver struct {
	cred  githubCredential
	cache map[string][]string // "org/team" -> list of member logins
	mu    sync.RWMutex
}

// NewTeamResolver creates a TeamResolver with the given GitHub credential.
// The credential must implement ExecuteAPI for gh CLI delegation.
//
// Returns nil if cred is nil.
func NewTeamResolver(cred githubCredential) *TeamResolver {
	if cred == nil {
		return nil
	}
	return &TeamResolver{
		cred:  cred,
		cache: make(map[string][]string),
	}
}

// IsMember checks if a user is a member of a team.
//
// The team parameter should be in "org/team-name" format, matching the
// CODEOWNERS @org/team-name syntax (without the @ prefix).
//
// Returns true if the user is a member, false otherwise.
// Returns an error if the team format is invalid or the API call fails.
//
// Examples:
//
//	resolver.IsMember(ctx, "acme/backend-team", "octocat")
//	resolver.IsMember(ctx, "chainguard/security", "engineer1")
func (t *TeamResolver) IsMember(ctx context.Context, team, login string) (bool, error) {
	if t == nil {
		return false, nil
	}

	if login == "" {
		return false, nil
	}

	members, err := t.GetMembers(ctx, team)
	if err != nil {
		return false, err
	}

	// Case-insensitive comparison for GitHub logins
	for _, member := range members {
		if strings.EqualFold(member, login) {
			return true, nil
		}
	}

	return false, nil
}

// GetMembers returns all members of a team.
// Results are cached to avoid redundant API calls.
//
// The team parameter should be in "org/team-name" format, matching the
// CODEOWNERS @org/team-name syntax (without the @ prefix).
//
// Returns an empty slice (not nil) if the team has no members.
// Returns an error if the team format is invalid or the API call fails.
//
// Errors:
//   - Invalid team format (missing "/" separator)
//   - API errors (rate limiting, network issues, team not found)
func (t *TeamResolver) GetMembers(ctx context.Context, team string) ([]string, error) {
	if t == nil {
		return nil, nil
	}

	// Validate team format
	org, teamSlug, err := parseTeamRef(team)
	if err != nil {
		return nil, err
	}

	// Check cache first (read lock)
	t.mu.RLock()
	if members, ok := t.cache[team]; ok {
		t.mu.RUnlock()
		return members, nil
	}
	t.mu.RUnlock()

	// Not in cache, fetch from GitHub
	members, err := t.fetchMembers(ctx, org, teamSlug)
	if err != nil {
		return nil, err
	}

	// Store in cache (write lock)
	t.mu.Lock()
	// Double-check: another goroutine might have populated the cache
	if existing, ok := t.cache[team]; ok {
		t.mu.Unlock()
		return existing, nil
	}
	t.cache[team] = members
	t.mu.Unlock()

	return members, nil
}

// InvalidateCache clears the cache for a specific team or all teams.
//
// If team is empty, all cached memberships are cleared.
// If team is specified, only that team's cached membership is removed.
//
// This is useful when:
//   - Team membership has changed
//   - Memory pressure requires cache eviction
//   - Refresh is explicitly requested
func (t *TeamResolver) InvalidateCache(team string) {
	if t == nil {
		return
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	if team == "" {
		// Clear all
		t.cache = make(map[string][]string)
	} else {
		delete(t.cache, team)
	}
}

// CacheSize returns the number of cached teams.
// This is useful for testing and monitoring.
func (t *TeamResolver) CacheSize() int {
	if t == nil {
		return 0
	}

	t.mu.RLock()
	defer t.mu.RUnlock()
	return len(t.cache)
}

// teamMemberResponse represents a team member from the GitHub API.
type teamMemberResponse struct {
	Login string `json:"login"`
}

// fetchMembers retrieves team members from the GitHub API.
//
// GitHub API: GET /orgs/{org}/teams/{team_slug}/members
// Returns a JSON array of user objects with "login" field.
func (t *TeamResolver) fetchMembers(ctx context.Context, org, teamSlug string) ([]string, error) {
	// Build the API endpoint
	endpoint := fmt.Sprintf("orgs/%s/teams/%s/members", org, teamSlug)

	// Execute via gh CLI delegation
	out, err := t.cred.ExecuteAPI(ctx, endpoint)
	if err != nil {
		// Wrap error with context
		return nil, fmt.Errorf("fetching team %s/%s members: %w", org, teamSlug, err)
	}

	// Parse the response
	var members []teamMemberResponse
	if err := json.Unmarshal(out, &members); err != nil {
		return nil, fmt.Errorf("parsing team members response for %s/%s: %w", org, teamSlug, err)
	}

	// Extract logins
	logins := make([]string, len(members))
	for i, m := range members {
		logins[i] = m.Login
	}

	return logins, nil
}

// parseTeamRef parses a team reference in "org/team-name" format.
// Returns the org and team slug separately.
//
// Returns an error if the format is invalid.
func parseTeamRef(team string) (org, teamSlug string, err error) {
	if team == "" {
		return "", "", fmt.Errorf("invalid team format: empty string")
	}

	parts := strings.SplitN(team, "/", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid team format: %q (expected org/team-name)", team)
	}

	org = strings.TrimSpace(parts[0])
	teamSlug = strings.TrimSpace(parts[1])

	if org == "" {
		return "", "", fmt.Errorf("invalid team format: %q (empty org)", team)
	}
	if teamSlug == "" {
		return "", "", fmt.Errorf("invalid team format: %q (empty team slug)", team)
	}

	return org, teamSlug, nil
}
