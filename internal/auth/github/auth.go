// Package github provides GitHub authentication for Kora via gh CLI delegation.
// Ground truth defined in specs/efas/0002-auth-provider.md
//
// IT IS FORBIDDEN TO CHANGE THIS IMPLEMENTATION without updating EFA 0002.
// Claude MUST stop and ask before modifying this file.
//
// SECURITY: This package delegates ALL authentication to the gh CLI tool:
//   - Kora NEVER extracts or stores GitHub tokens
//   - Authentication status is checked via `gh auth status`
//   - API calls are delegated via GitHubDelegatedCredential.ExecuteAPI()
//   - The token never leaves gh CLI's control
package github

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/dakaneye/kora/internal/auth"
)

// Command timeouts.
const (
	authTimeout = 10 * time.Second
	apiTimeout  = 30 * time.Second
)

// GitHubDelegatedCredential represents GitHub auth via gh CLI delegation.
// IT IS FORBIDDEN TO CHANGE THIS TYPE without updating EFA 0002.
//
// SECURITY: This credential does NOT store or expose the actual OAuth token.
// All API calls are delegated to the gh CLI, which handles token management.
// This is the most secure approach as Kora never sees the token.
//
//nolint:revive // name matches EFA 0002 specification
type GitHubDelegatedCredential struct {
	username string
	ghPath   string // path to gh CLI for delegation
}

// NewGitHubDelegatedCredential creates a delegated credential.
// Returns ErrCredentialInvalid if username is empty.
func NewGitHubDelegatedCredential(username, ghPath string) (*GitHubDelegatedCredential, error) {
	if username == "" {
		return nil, fmt.Errorf("%w: username is required", auth.ErrCredentialInvalid)
	}
	if ghPath == "" {
		ghPath = "gh"
	}
	return &GitHubDelegatedCredential{username: username, ghPath: ghPath}, nil
}

// Type returns the credential type.
func (c *GitHubDelegatedCredential) Type() auth.CredentialType {
	return auth.CredentialTypeOAuth
}

// Value returns empty string - tokens are never extracted.
// Use ExecuteAPI() for authenticated API calls instead.
//
// SECURITY: GitHub tokens are NEVER exposed. This method exists only to
// satisfy the Credential interface. All API access must go through ExecuteAPI().
func (c *GitHubDelegatedCredential) Value() string {
	return ""
}

// Redacted returns "github:username" (no token to redact).
func (c *GitHubDelegatedCredential) Redacted() string {
	return fmt.Sprintf("github:%s", c.username)
}

// IsValid returns true if username is set (gh CLI handles token validation).
func (c *GitHubDelegatedCredential) IsValid() bool {
	return c.username != ""
}

// Username returns the authenticated GitHub username.
func (c *GitHubDelegatedCredential) Username() string {
	return c.username
}

// String implements fmt.Stringer with redaction for safety.
func (c *GitHubDelegatedCredential) String() string {
	return c.Redacted()
}

// ExecuteAPI executes a GitHub API call via gh CLI delegation.
// This is the ONLY way to make authenticated GitHub API calls.
// SECURITY: The token never leaves gh CLI's control.
//
// Example:
//
//	out, err := cred.ExecuteAPI(ctx, "user")
//	out, err := cred.ExecuteAPI(ctx, "repos/{owner}/{repo}/pulls", "--jq", ".[].number")
func (c *GitHubDelegatedCredential) ExecuteAPI(ctx context.Context, endpoint string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, apiTimeout)
	defer cancel()

	cmdArgs := append([]string{"api", endpoint}, args...)
	// #nosec G204 -- ghPath is from constructor, not user input
	cmd := exec.CommandContext(ctx, c.ghPath, cmdArgs...)
	out, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return nil, fmt.Errorf("gh api %s failed: %s", endpoint, string(exitErr.Stderr))
		}
		return nil, fmt.Errorf("gh api %s: %w", endpoint, err)
	}
	return out, nil
}

// GitHubAuthProvider implements auth.AuthProvider for GitHub via gh CLI delegation.
// IT IS FORBIDDEN TO CHANGE THIS IMPLEMENTATION without updating EFA 0002.
//
// SECURITY: This provider delegates ALL authentication to the gh CLI tool:
//   - Kora NEVER extracts or stores GitHub tokens
//   - Authentication status is checked via `gh auth status`
//   - API calls are delegated via GitHubDelegatedCredential.ExecuteAPI()
//   - The token never leaves gh CLI's control
//
//nolint:revive // name matches EFA 0002 specification
type GitHubAuthProvider struct {
	ghPath string // default: "gh"
}

// NewGitHubAuthProvider creates a GitHub auth provider.
// If ghPath is empty, defaults to "gh" (found via PATH).
func NewGitHubAuthProvider(ghPath string) *GitHubAuthProvider {
	if ghPath == "" {
		ghPath = "gh"
	}
	return &GitHubAuthProvider{ghPath: ghPath}
}

// Service returns the service this provider authenticates.
func (p *GitHubAuthProvider) Service() auth.Service {
	return auth.ServiceGitHub
}

// Authenticate verifies that valid credentials exist and are usable.
// Checks if gh CLI is installed and authenticated.
func (p *GitHubAuthProvider) Authenticate(ctx context.Context) error {
	if _, err := exec.LookPath(p.ghPath); err != nil {
		return fmt.Errorf("github auth: %w", auth.ErrGHCLINotFound)
	}

	ctx, cancel := context.WithTimeout(ctx, authTimeout)
	defer cancel()

	// #nosec G204 -- ghPath is from constructor, not user input
	cmd := exec.CommandContext(ctx, p.ghPath, "auth", "status")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("github auth: %w", auth.ErrGHCLINotAuthenticated)
	}
	return nil
}

// GetCredential retrieves the GitHub credential.
// Returns a delegated credential that executes API calls via gh CLI.
//
// SECURITY: This returns a delegated credential, NOT the actual token.
// All API calls will go through gh CLI.
func (p *GitHubAuthProvider) GetCredential(ctx context.Context) (auth.Credential, error) {
	if err := p.Authenticate(ctx); err != nil {
		return nil, err
	}

	// Get username from gh CLI - this is safe to cache
	username, err := p.runGH(ctx, "api", "user", "--jq", ".login")
	if err != nil {
		return nil, fmt.Errorf("github auth: failed to get username: %w", err)
	}

	cred, err := NewGitHubDelegatedCredential(strings.TrimSpace(username), p.ghPath)
	if err != nil {
		return nil, fmt.Errorf("github auth: %w", err)
	}
	return cred, nil
}

// IsAuthenticated returns true if valid credentials exist.
func (p *GitHubAuthProvider) IsAuthenticated(ctx context.Context) bool {
	return p.Authenticate(ctx) == nil
}

// runGH executes a gh CLI command and returns stdout.
// Returns an error if the command fails or times out.
func (p *GitHubAuthProvider) runGH(ctx context.Context, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, authTimeout)
	defer cancel()

	// #nosec G204 -- ghPath is from constructor, not user input
	cmd := exec.CommandContext(ctx, p.ghPath, args...)
	out, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return "", fmt.Errorf("gh %v failed: %s", args, string(exitErr.Stderr))
		}
		return "", fmt.Errorf("gh %v: %w", args, err)
	}
	return string(out), nil
}
