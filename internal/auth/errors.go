// Package auth provides authentication abstractions for external services.
// Ground truth defined in specs/efas/0002-auth-provider.md
package auth

import "errors"

// Sentinel errors for authentication operations.
// IT IS FORBIDDEN TO CHANGE THESE ERRORS without updating EFA 0002.
var (
	// ErrNotAuthenticated indicates credentials are missing or invalid.
	ErrNotAuthenticated = errors.New("not authenticated")

	// ErrCredentialNotFound indicates no credential exists in storage.
	ErrCredentialNotFound = errors.New("credential not found")

	// ErrCredentialInvalid indicates the credential format is invalid.
	ErrCredentialInvalid = errors.New("credential format invalid")

	// ErrKeychainUnavailable indicates the system keychain is not accessible.
	ErrKeychainUnavailable = errors.New("keychain unavailable")

	// ErrGHCLINotFound indicates the gh CLI is not installed.
	ErrGHCLINotFound = errors.New("gh CLI not found")

	// ErrGHCLINotAuthenticated indicates gh CLI has no active session.
	ErrGHCLINotAuthenticated = errors.New("gh CLI not authenticated")

	// ErrOAuthFlowFailed indicates the OAuth browser flow failed.
	ErrOAuthFlowFailed = errors.New("oauth flow failed")

	// ErrTokenRefreshFailed indicates token refresh failed.
	ErrTokenRefreshFailed = errors.New("token refresh failed")

	// ErrOAuthConfigMissing indicates OAuth client credentials are not configured.
	ErrOAuthConfigMissing = errors.New("oauth client credentials not configured")
)
