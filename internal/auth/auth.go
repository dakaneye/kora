// Package auth provides authentication abstractions for external services.
// Ground truth defined in specs/efas/0002-auth-provider.md
//
// IT IS FORBIDDEN TO CHANGE THESE INTERFACES without updating EFA 0002.
// Claude MUST stop and ask before modifying this file.
package auth

import (
	"context"
)

// Service identifies an authentication target.
// IT IS FORBIDDEN TO CHANGE THIS TYPE without updating EFA 0002.
type Service string

const (
	// ServiceGitHub identifies the GitHub authentication service.
	ServiceGitHub Service = "github"
)

// AuthProvider manages authentication for a specific service.
// Each service (GitHub, Slack) has exactly one AuthProvider implementation.
//
// IT IS FORBIDDEN TO CHANGE THIS INTERFACE without updating EFA 0002.
//
// Implementations must:
//   - Never log or expose credential values
//   - Respect context cancellation on all operations
//   - Return ErrNotAuthenticated when credentials are missing/invalid
//
//nolint:revive // name matches EFA 0002 specification
type AuthProvider interface {
	// Service returns the service this provider authenticates.
	Service() Service

	// Authenticate verifies that valid credentials exist and are usable.
	// Returns nil if authentication is valid, ErrNotAuthenticated if not,
	// or another error if verification failed.
	Authenticate(ctx context.Context) error

	// GetCredential retrieves the credential for this service.
	// Returns ErrNotAuthenticated if no credential exists.
	// The returned Credential must have a working Redacted() method.
	GetCredential(ctx context.Context) (Credential, error)

	// IsAuthenticated returns true if valid credentials exist.
	// This is a non-blocking check; use Authenticate for validation.
	IsAuthenticated(ctx context.Context) bool
}
