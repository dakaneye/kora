// Package codeowners provides fetching and parsing for GitHub CODEOWNERS files.
//
// This file implements fetching CODEOWNERS files from GitHub repositories
// via gh CLI delegation per EFA 0002. Files are cached in memory to avoid
// redundant API calls when processing multiple PRs from the same repository.
//
// SECURITY: All API calls are delegated to the credential's ExecuteAPI method.
// This code NEVER sees or handles GitHub tokens directly.
// See EFA 0002 for credential security requirements.
package codeowners

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
)

// githubCredential is the interface for gh CLI delegation per EFA 0002.
// This allows both real and mock credentials to be used.
//
// SECURITY: The ExecuteAPI method delegates API calls to gh CLI.
// The GitHub token never leaves gh CLI's control.
type githubCredential interface {
	ExecuteAPI(ctx context.Context, endpoint string, args ...string) ([]byte, error)
}

// codeownersLocations defines the standard GitHub CODEOWNERS file locations
// in order of priority. GitHub checks these locations in this order.
var codeownersLocations = []string{
	".github/CODEOWNERS",
	"CODEOWNERS",
	"docs/CODEOWNERS",
}

// Fetcher retrieves and caches CODEOWNERS files from GitHub repositories.
//
// Thread-safety: All public methods are safe for concurrent use.
// The cache is protected by a read-write mutex to allow multiple
// concurrent reads while serializing writes.
type Fetcher struct {
	cred  githubCredential
	cache map[string]*Ruleset
	mu    sync.RWMutex
}

// NewFetcher creates a CODEOWNERS fetcher with the given GitHub credential.
// The credential must implement ExecuteAPI for gh CLI delegation.
//
// Returns nil if cred is nil.
func NewFetcher(cred githubCredential) *Fetcher {
	if cred == nil {
		return nil
	}
	return &Fetcher{
		cred:  cred,
		cache: make(map[string]*Ruleset),
	}
}

// GetRuleset returns the CODEOWNERS ruleset for a repository, using cache if available.
//
// The repo parameter should be in "owner/repo" format (e.g., "octocat/Hello-World").
// Returns nil ruleset without error if no CODEOWNERS file exists in the repository.
//
// This method checks the standard CODEOWNERS locations in order:
//  1. .github/CODEOWNERS
//  2. CODEOWNERS (root)
//  3. docs/CODEOWNERS
//
// The first found file is used and cached. Subsequent calls for the same repo
// return the cached ruleset immediately.
//
// Errors:
//   - Invalid repo format (missing "/" separator)
//   - API errors (rate limiting, network issues)
//   - Parse errors (malformed CODEOWNERS content)
func (f *Fetcher) GetRuleset(ctx context.Context, repo string) (*Ruleset, error) {
	if f == nil {
		return nil, nil
	}

	// Validate repo format
	if !strings.Contains(repo, "/") {
		return nil, fmt.Errorf("invalid repo format: %q (expected owner/repo)", repo)
	}

	// Check cache first (read lock)
	f.mu.RLock()
	if rs, ok := f.cache[repo]; ok {
		f.mu.RUnlock()
		return rs, nil
	}
	f.mu.RUnlock()

	// Not in cache, fetch from GitHub
	rs, err := f.fetchFromGitHub(ctx, repo)
	if err != nil {
		return nil, err
	}

	// Store in cache (write lock)
	f.mu.Lock()
	// Double-check: another goroutine might have populated the cache
	if existing, ok := f.cache[repo]; ok {
		f.mu.Unlock()
		return existing, nil
	}
	f.cache[repo] = rs
	f.mu.Unlock()

	return rs, nil
}

// InvalidateCache clears the cache for a specific repository or all repositories.
//
// If repo is empty, all cached rulesets are cleared.
// If repo is specified, only that repository's cached ruleset is removed.
//
// This is useful when:
//   - CODEOWNERS file has been updated
//   - Memory pressure requires cache eviction
//   - Refresh is explicitly requested
func (f *Fetcher) InvalidateCache(repo string) {
	if f == nil {
		return
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	if repo == "" {
		// Clear all
		f.cache = make(map[string]*Ruleset)
	} else {
		delete(f.cache, repo)
	}
}

// CacheSize returns the number of cached repositories.
// This is useful for testing and monitoring.
func (f *Fetcher) CacheSize() int {
	if f == nil {
		return 0
	}

	f.mu.RLock()
	defer f.mu.RUnlock()
	return len(f.cache)
}

// fetchFromGitHub retrieves the CODEOWNERS file from GitHub.
// It tries each standard location in order until one is found.
// Returns nil ruleset without error if no CODEOWNERS file exists.
func (f *Fetcher) fetchFromGitHub(ctx context.Context, repo string) (*Ruleset, error) {
	for _, location := range codeownersLocations {
		rs, found, err := f.tryFetchLocation(ctx, repo, location)
		if err != nil {
			return nil, fmt.Errorf("fetching %s from %s: %w", location, repo, err)
		}
		if found {
			return rs, nil
		}
	}

	// No CODEOWNERS file found in any location - this is valid
	return nil, nil
}

// contentResponse represents the GitHub API response for file contents.
type contentResponse struct {
	Content  string `json:"content"`  // Base64-encoded file content
	Encoding string `json:"encoding"` // Always "base64" for files
	Type     string `json:"type"`     // "file" for files, "dir" for directories
	Message  string `json:"message"`  // Error message when not found
}

// tryFetchLocation attempts to fetch CODEOWNERS from a specific path.
// Returns (ruleset, found, error).
//
// found=false with nil error means the file doesn't exist at this location.
// found=true with nil error means the file was found and parsed successfully.
// Non-nil error indicates an API or parse error.
func (f *Fetcher) tryFetchLocation(ctx context.Context, repo, path string) (*Ruleset, bool, error) {
	// Build the API endpoint
	// GitHub API: GET /repos/{owner}/{repo}/contents/{path}
	endpoint := fmt.Sprintf("repos/%s/contents/%s", repo, path)

	// Execute via gh CLI delegation
	out, err := f.cred.ExecuteAPI(ctx, endpoint)
	if err != nil {
		// Check if it's a 404 (file not found)
		errStr := err.Error()
		if strings.Contains(errStr, "404") || strings.Contains(errStr, "Not Found") {
			return nil, false, nil
		}
		// Other API errors should be reported
		return nil, false, fmt.Errorf("api call failed: %w", err)
	}

	// Parse the response
	var resp contentResponse
	if unmarshalErr := json.Unmarshal(out, &resp); unmarshalErr != nil {
		return nil, false, fmt.Errorf("parsing response: %w", unmarshalErr)
	}

	// Check if it's a file (not a directory)
	if resp.Type == "dir" {
		return nil, false, nil
	}

	// Decode base64 content
	content, decodeErr := decodeContent(resp.Content, resp.Encoding)
	if decodeErr != nil {
		return nil, false, fmt.Errorf("decoding content: %w", decodeErr)
	}

	// Parse the CODEOWNERS content
	rs, parseErr := Parse(content)
	if parseErr != nil {
		return nil, false, fmt.Errorf("parsing CODEOWNERS: %w", parseErr)
	}

	return rs, true, nil
}

// decodeContent decodes the file content based on its encoding.
// GitHub API returns base64-encoded content for files.
func decodeContent(content, encoding string) ([]byte, error) {
	if encoding != "base64" && encoding != "" {
		return nil, fmt.Errorf("unsupported encoding: %s", encoding)
	}

	// GitHub API includes newlines in base64 content for readability
	// We need to remove them before decoding
	content = strings.ReplaceAll(content, "\n", "")

	decoded, err := base64.StdEncoding.DecodeString(content)
	if err != nil {
		return nil, fmt.Errorf("base64 decode: %w", err)
	}

	return decoded, nil
}
