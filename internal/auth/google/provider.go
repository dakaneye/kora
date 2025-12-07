// Package google provides Google OAuth authentication for Calendar and Gmail.
// Ground truth defined in specs/efas/0002-auth-provider.md
//
// IT IS FORBIDDEN TO CHANGE THIS IMPLEMENTATION without updating EFA 0002.
// Claude MUST stop and ask before modifying this file.
package google

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/dakaneye/kora/internal/auth"
	"github.com/dakaneye/kora/internal/auth/keychain"
)

// keychainKeyPrefix is the prefix for Google OAuth keychain keys.
// Format: google-oauth-{email}
const keychainKeyPrefix = "google-oauth-"

// GoogleAuthProvider implements auth.AuthProvider for Google via OAuth 2.0.
// IT IS FORBIDDEN TO CHANGE THIS IMPLEMENTATION without updating EFA 0002.
//
// SECURITY:
//   - Tokens are stored in macOS Keychain, never in files
//   - Tokens are NEVER logged
//   - Automatic token refresh before expiry (5-minute buffer)
//   - OAuth client credentials from environment variables only
//
// OAuth Flow:
//  1. User calls InitiateLogin() or Authenticate() triggers login
//  2. Browser opens Google consent screen
//  3. User approves scopes (calendar.readonly, gmail.readonly)
//  4. Redirect to localhost:8765/callback with auth code
//  5. Exchange code for tokens via Google token endpoint
//  6. Store tokens in keychain as: google-oauth-{email}
//
//nolint:revive // name matches EFA 0002 specification
type GoogleAuthProvider struct {
	keychain keychain.Keychain
	config   *OAuthConfig
	email    string
}

// NewGoogleAuthProvider creates a Google auth provider for a specific email.
// Returns auth.ErrOAuthConfigMissing if OAuth credentials are not configured.
//
// The email parameter identifies which Google account to authenticate.
// Credentials are stored in keychain with key: google-oauth-{email}
func NewGoogleAuthProvider(kc keychain.Keychain, email string) (*GoogleAuthProvider, error) {
	if email == "" {
		return nil, fmt.Errorf("%w: email required", auth.ErrCredentialInvalid)
	}

	config, err := GetOAuthConfig()
	if err != nil {
		return nil, err
	}

	return &GoogleAuthProvider{
		keychain: kc,
		email:    email,
		config:   config,
	}, nil
}

// Service returns the service this provider authenticates.
func (p *GoogleAuthProvider) Service() auth.Service {
	return auth.ServiceGoogle
}

// Authenticate verifies that valid credentials exist and are usable.
// Returns nil if authentication is valid.
// Returns auth.ErrNotAuthenticated if credentials are missing.
// Returns auth.ErrCredentialInvalid if credentials are malformed.
//
// This method checks for stored credentials and refreshes them if expired.
// It does NOT trigger the OAuth browser flow - use InitiateLogin() for that.
func (p *GoogleAuthProvider) Authenticate(ctx context.Context) error {
	cred, err := p.GetCredential(ctx)
	if err != nil {
		return err
	}
	if !cred.IsValid() {
		return auth.ErrCredentialInvalid
	}
	return nil
}

// GetCredential retrieves the credential for this service.
// Returns auth.ErrNotAuthenticated if no credential exists.
//
// If the stored token is expired, it will be automatically refreshed.
// The refreshed token is saved back to the keychain.
func (p *GoogleAuthProvider) GetCredential(ctx context.Context) (auth.Credential, error) {
	// Try to load from keychain
	cred, err := p.loadFromKeychain(ctx)
	if err != nil {
		return nil, err
	}

	// Check if token needs refresh
	if cred.IsExpired() {
		cred, err = p.refreshToken(ctx, cred)
		if err != nil {
			return nil, fmt.Errorf("refreshing token: %w", err)
		}
	}

	return cred, nil
}

// IsAuthenticated returns true if valid credentials exist.
// This is a non-blocking check that does not refresh tokens.
func (p *GoogleAuthProvider) IsAuthenticated(ctx context.Context) bool {
	cred, err := p.loadFromKeychain(ctx)
	if err != nil {
		return false
	}
	return cred.IsValid()
}

// InitiateLogin starts the OAuth browser flow and stores credentials on success.
// This opens a browser window for user consent.
//
// Returns an error if:
//   - The OAuth flow fails or times out
//   - The authenticated email doesn't match the expected email
//   - Credentials cannot be stored in the keychain
func (p *GoogleAuthProvider) InitiateLogin(ctx context.Context) error {
	cred, err := InitiateOAuthFlow(ctx, p.config)
	if err != nil {
		return err
	}

	// Verify email matches the expected email
	if cred.Email() != p.email {
		return fmt.Errorf("google oauth: authenticated as %s but expected %s", cred.Email(), p.email)
	}

	return p.saveToKeychain(ctx, cred)
}

// Logout removes stored credentials from the keychain.
func (p *GoogleAuthProvider) Logout(ctx context.Context) error {
	return p.keychain.Delete(ctx, p.keychainKey())
}

// Email returns the Google account email this provider manages.
func (p *GoogleAuthProvider) Email() string {
	return p.email
}

// keychainKey returns the keychain key for this email.
func (p *GoogleAuthProvider) keychainKey() string {
	return keychainKeyPrefix + p.email
}

// storedCredential is the JSON structure stored in keychain.
// SECURITY: This structure contains sensitive tokens that MUST NEVER be logged.
type storedCredential struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	Expiry       time.Time `json:"expiry"`
	Email        string    `json:"email"`
}

// loadFromKeychain retrieves and parses credentials from the keychain.
func (p *GoogleAuthProvider) loadFromKeychain(ctx context.Context) (*GoogleOAuthCredential, error) {
	data, err := p.keychain.Get(ctx, p.keychainKey())
	if err != nil {
		if errors.Is(err, keychain.ErrNotFound) {
			return nil, fmt.Errorf("%w: no credentials for %s", auth.ErrNotAuthenticated, p.email)
		}
		return nil, fmt.Errorf("keychain access: %w", err)
	}

	var stored storedCredential
	if err := json.Unmarshal([]byte(data), &stored); err != nil {
		return nil, fmt.Errorf("%w: invalid stored credential format", auth.ErrCredentialInvalid)
	}

	return NewGoogleOAuthCredential(stored.AccessToken, stored.RefreshToken, stored.Email, stored.Expiry)
}

// saveToKeychain stores credentials in the keychain as JSON.
// SECURITY: The credential values are stored encrypted by the keychain.
func (p *GoogleAuthProvider) saveToKeychain(ctx context.Context, cred *GoogleOAuthCredential) error {
	stored := storedCredential{
		AccessToken:  cred.Value(),
		RefreshToken: cred.RefreshToken(),
		Expiry:       cred.Expiry(),
		Email:        cred.Email(),
	}

	data, err := json.Marshal(stored)
	if err != nil {
		return fmt.Errorf("marshaling credential: %w", err)
	}

	if err := p.keychain.Set(ctx, p.keychainKey(), string(data)); err != nil {
		return fmt.Errorf("storing credential: %w", err)
	}

	return nil
}

// refreshToken obtains a new access token using the refresh token.
// The new credential is automatically saved to the keychain.
func (p *GoogleAuthProvider) refreshToken(ctx context.Context, cred *GoogleOAuthCredential) (*GoogleOAuthCredential, error) {
	newAccessToken, newExpiry, err := RefreshAccessToken(ctx, p.config, cred.RefreshToken())
	if err != nil {
		return nil, err
	}

	newCred := cred.WithNewAccessToken(newAccessToken, newExpiry)

	// Save refreshed credential to keychain
	if err := p.saveToKeychain(ctx, newCred); err != nil {
		return nil, fmt.Errorf("saving refreshed credential: %w", err)
	}

	return newCred, nil
}

// Ensure GoogleAuthProvider implements auth.AuthProvider.
var _ auth.AuthProvider = (*GoogleAuthProvider)(nil)
