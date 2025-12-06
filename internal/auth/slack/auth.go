// Package slack provides Slack authentication for Kora.
// Ground truth defined in specs/efas/0002-auth-provider.md
//
// IT IS FORBIDDEN TO CHANGE THIS IMPLEMENTATION without updating EFA 0002.
// Claude MUST stop and ask before modifying this file.
package slack

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/dakaneye/kora/internal/auth"
	"github.com/dakaneye/kora/internal/auth/keychain"
)

// SlackToken represents a Slack user token (xoxp-*).
// IT IS FORBIDDEN TO CHANGE THIS TYPE without updating EFA 0002.
//
// SECURITY: The token field is unexported and must only be accessed
// via Value(). String() returns Redacted() to prevent accidental logging.
//
//nolint:revive // name matches EFA 0002 specification
type SlackToken struct {
	token string // NEVER expose this field - use Value() or Redacted()
}

// NewSlackToken creates a SlackToken from a raw token string.
// Returns ErrCredentialInvalid if the token format is invalid.
func NewSlackToken(token string) (*SlackToken, error) {
	t := &SlackToken{token: token}
	if !t.IsValid() {
		return nil, fmt.Errorf("%w: slack token must start with xoxp-", auth.ErrCredentialInvalid)
	}
	return t, nil
}

// Type returns the credential type.
func (t *SlackToken) Type() auth.CredentialType {
	return auth.CredentialTypeToken
}

// Value returns the raw token value.
// WARNING: This value MUST NEVER be logged.
// IT IS FORBIDDEN TO LOG THIS VALUE. See EFA 0002.
func (t *SlackToken) Value() string {
	return t.token
}

// Redacted returns a safe-to-log representation.
// SECURITY: Shows only a hash-based fingerprint, not any part of the actual token.
// This prevents partial token exposure while still allowing log correlation.
//
// Format: "xoxp-[8-char-sha256]" where the hash is derived from the full token.
func (t *SlackToken) Redacted() string {
	if len(t.token) < 15 {
		return "xoxp-[invalid]"
	}
	// Generate a fingerprint from the token for log correlation.
	// Using first 8 chars of SHA256 hash - enough for correlation, not for cracking.
	h := sha256.Sum256([]byte(t.token))
	fingerprint := hex.EncodeToString(h[:4])
	return fmt.Sprintf("xoxp-[%s]", fingerprint)
}

// IsValid checks if the token has the correct format.
// Slack user tokens must start with "xoxp-" and have reasonable length.
func (t *SlackToken) IsValid() bool {
	return strings.HasPrefix(t.token, "xoxp-") && len(t.token) > 10
}

// String implements fmt.Stringer with redaction for safety.
// SECURITY: Always returns Redacted() to prevent accidental logging.
func (t *SlackToken) String() string {
	return t.Redacted()
}

// Keychain key and environment variable names.
const (
	slackKeychainKey = "slack-token"
	slackEnvVarName  = "KORA_SLACK_TOKEN"
)

// SlackAuthProvider implements auth.AuthProvider for Slack.
// IT IS FORBIDDEN TO CHANGE THIS IMPLEMENTATION without updating EFA 0002.
//
// Credential lookup order:
//  1. macOS Keychain (preferred)
//  2. Environment variable KORA_SLACK_TOKEN (fallback with warning)
//
//nolint:revive // name matches EFA 0002 specification
type SlackAuthProvider struct {
	keychain keychain.Keychain
	logger   *slog.Logger
}

// NewSlackAuthProvider creates a Slack auth provider with the given keychain.
// If logger is nil, uses slog.Default().
func NewSlackAuthProvider(kc keychain.Keychain, logger *slog.Logger) *SlackAuthProvider {
	if logger == nil {
		logger = slog.Default()
	}
	return &SlackAuthProvider{
		keychain: kc,
		logger:   logger,
	}
}

// Service returns the service this provider authenticates.
func (p *SlackAuthProvider) Service() auth.Service {
	return auth.ServiceSlack
}

// Authenticate verifies that valid credentials exist and are usable.
func (p *SlackAuthProvider) Authenticate(ctx context.Context) error {
	cred, err := p.GetCredential(ctx)
	if err != nil {
		return err
	}
	if !cred.IsValid() {
		return auth.ErrCredentialInvalid
	}
	return nil
}

// GetCredential retrieves the Slack credential.
// Tries keychain first, falls back to environment variable with warning.
func (p *SlackAuthProvider) GetCredential(ctx context.Context) (auth.Credential, error) {
	// 1. Try keychain first (preferred)
	token, err := p.keychain.Get(ctx, slackKeychainKey)
	if err == nil {
		cred, credErr := NewSlackToken(token)
		if credErr != nil {
			// Token in keychain has invalid format
			return nil, fmt.Errorf("slack auth: keychain token invalid: %w", credErr)
		}
		return cred, nil
	}

	// Only fall through if not found; other errors should propagate
	if !errors.Is(err, keychain.ErrNotFound) {
		return nil, fmt.Errorf("slack auth: keychain access failed: %w", err)
	}

	// 2. Fall back to environment variable
	// SECURITY WARNING: Env vars are less secure than keychain.
	// They may be exposed via /proc, crash dumps, or child processes.
	if token := os.Getenv(slackEnvVarName); token != "" {
		p.logger.Warn("using Slack token from environment variable - consider storing in keychain for better security",
			"env_var", slackEnvVarName)
		cred, err := NewSlackToken(token)
		if err != nil {
			return nil, fmt.Errorf("slack auth: %s invalid: %w", slackEnvVarName, err)
		}
		return cred, nil
	}

	return nil, fmt.Errorf("slack auth: %w: set %s or store in keychain",
		auth.ErrNotAuthenticated, slackEnvVarName)
}

// IsAuthenticated returns true if valid credentials exist.
func (p *SlackAuthProvider) IsAuthenticated(ctx context.Context) bool {
	return p.Authenticate(ctx) == nil
}
