// Package google provides Google OAuth authentication for Calendar and Gmail.
// Ground truth defined in specs/efas/0002-auth-provider.md
//
// IT IS FORBIDDEN TO CHANGE THIS IMPLEMENTATION without updating EFA 0002.
// Claude MUST stop and ask before modifying this file.
package google

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/dakaneye/kora/internal/auth"
)

// expiryBuffer is the time before expiry when we consider a token expired.
// This ensures we refresh before the token becomes invalid.
const expiryBuffer = 5 * time.Minute

// GoogleOAuthCredential represents Google OAuth 2.0 tokens.
// IT IS FORBIDDEN TO CHANGE THIS TYPE without updating EFA 0002.
//
// SECURITY REQUIREMENTS:
//   - accessToken and refreshToken MUST NEVER be logged
//   - Redacted() returns a hash-based fingerprint for log correlation
//   - IsExpired() includes 5-minute buffer for safe refresh
//
//nolint:revive // name matches EFA 0002 specification
type GoogleOAuthCredential struct {
	accessToken  string
	refreshToken string
	expiry       time.Time
	email        string
}

// NewGoogleOAuthCredential creates a Google OAuth credential.
// Returns auth.ErrCredentialInvalid if required fields are empty.
func NewGoogleOAuthCredential(accessToken, refreshToken, email string, expiry time.Time) (*GoogleOAuthCredential, error) {
	if accessToken == "" {
		return nil, fmt.Errorf("%w: access token required", auth.ErrCredentialInvalid)
	}
	if refreshToken == "" {
		return nil, fmt.Errorf("%w: refresh token required", auth.ErrCredentialInvalid)
	}
	if email == "" {
		return nil, fmt.Errorf("%w: email required", auth.ErrCredentialInvalid)
	}
	return &GoogleOAuthCredential{
		accessToken:  accessToken,
		refreshToken: refreshToken,
		expiry:       expiry,
		email:        email,
	}, nil
}

// Type returns the credential type.
func (c *GoogleOAuthCredential) Type() auth.CredentialType {
	return auth.CredentialTypeOAuth
}

// Value returns the access token for API calls.
// WARNING: This value MUST NEVER be logged.
// IT IS FORBIDDEN TO LOG THIS VALUE. See EFA 0002.
func (c *GoogleOAuthCredential) Value() string {
	return c.accessToken
}

// Redacted returns a safe-to-log representation.
// Format: "google:{email}:[8-char fingerprint]"
// SECURITY: Shows only a hash-based fingerprint, not any part of the actual token.
func (c *GoogleOAuthCredential) Redacted() string {
	h := sha256.Sum256([]byte(c.accessToken))
	fingerprint := hex.EncodeToString(h[:4])
	return fmt.Sprintf("google:%s:[%s]", c.email, fingerprint)
}

// IsValid checks that all required fields are present.
// Does NOT verify the token works with Google APIs.
func (c *GoogleOAuthCredential) IsValid() bool {
	return c.accessToken != "" && c.refreshToken != "" && c.email != ""
}

// IsExpired returns true if the token has expired or will expire within 5 minutes.
// The 5-minute buffer ensures we refresh before the token becomes invalid.
func (c *GoogleOAuthCredential) IsExpired() bool {
	if c.expiry.IsZero() {
		return true // No expiry means we should refresh
	}
	return time.Now().After(c.expiry.Add(-expiryBuffer))
}

// Email returns the associated Google account email.
func (c *GoogleOAuthCredential) Email() string {
	return c.email
}

// RefreshToken returns the refresh token for obtaining new access tokens.
// WARNING: This value MUST NEVER be logged.
// IT IS FORBIDDEN TO LOG THIS VALUE. See EFA 0002.
func (c *GoogleOAuthCredential) RefreshToken() string {
	return c.refreshToken
}

// Expiry returns when the access token expires.
func (c *GoogleOAuthCredential) Expiry() time.Time {
	return c.expiry
}

// String implements fmt.Stringer with redaction for safety.
func (c *GoogleOAuthCredential) String() string {
	return c.Redacted()
}

// WithNewAccessToken returns a new credential with an updated access token and expiry.
// Used after token refresh. The refresh token and email remain unchanged.
func (c *GoogleOAuthCredential) WithNewAccessToken(accessToken string, expiry time.Time) *GoogleOAuthCredential {
	return &GoogleOAuthCredential{
		accessToken:  accessToken,
		refreshToken: c.refreshToken,
		expiry:       expiry,
		email:        c.email,
	}
}
