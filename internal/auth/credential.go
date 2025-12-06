// Package auth provides authentication abstractions for external services.
// Ground truth defined in specs/efas/0002-auth-provider.md
package auth

// Credential represents an authentication credential with safe redaction.
// IT IS FORBIDDEN TO CHANGE THIS INTERFACE without updating EFA 0002.
//
// Implementations MUST ensure the credential value is never exposed via
// String(), logging, or any other method except Value().
//
// SECURITY REQUIREMENTS:
//   - Value() returns the raw credential - NEVER log this
//   - Redacted() returns a safe-to-log representation
//   - String() MUST return Redacted() for fmt.Stringer safety
type Credential interface {
	// Type returns the credential type (e.g., "token", "oauth").
	Type() CredentialType

	// Value returns the raw credential value.
	// WARNING: This value MUST NEVER be logged.
	// IT IS FORBIDDEN TO LOG THIS VALUE. See EFA 0002.
	Value() string

	// Redacted returns a safe-to-log representation.
	// For tokens: hash-based fingerprint (NOT partial token).
	// For delegated: type indicator with identifier.
	Redacted() string

	// IsValid performs format validation on the credential.
	// Does not verify the credential works with the service.
	IsValid() bool
}

// CredentialType identifies the kind of credential.
// IT IS FORBIDDEN TO CHANGE THESE TYPES without updating EFA 0002.
type CredentialType string

const (
	// CredentialTypeOAuth represents an OAuth token (GitHub via gh CLI).
	CredentialTypeOAuth CredentialType = "oauth"
	// CredentialTypeToken represents an API token (Slack xoxp-*).
	CredentialTypeToken CredentialType = "token"
)
