---
authors: Samuel Dacanay <samuel@dakaneye.com>
state: draft
agents: golang-pro, documentation-engineer, prompt-engineer
---

# EFA 0002: Authentication Provider Ground Truth

This EFA defines the authentication architecture for Kora, including the AuthProvider interface, credential types, and security requirements.

## Motivation & Prior Art

Authentication is security-critical. Without a strict interface definition, Claude may:
- Store credentials in plaintext
- Log tokens accidentally
- Bypass the keychain for "convenience"
- Invent new auth methods that don't follow security best practices

**Goals:**
- Single AuthProvider interface all datasources use
- Credentials never stored in plaintext
- Credentials never logged (even partially)
- macOS Keychain as primary credential store
- Support OAuth 2.0 flows with automatic token refresh

**Non-goals:**
- Cross-platform support (macOS only in v1)
- Credential rotation automation

## Detailed Design

### Authentication Strategy

```
┌─────────────────────────────────────────────────────────────┐
│                     Priority Order                          │
├─────────────────────────────────────────────────────────────┤
│ 1. CLI Delegation (GitHub via `gh`)                         │
│    - Most secure: no credential storage                     │
│    - Kora never sees the token                              │
├─────────────────────────────────────────────────────────────┤
│ 2. OAuth 2.0 Flow (Google via browser)                      │
│    - Authorization Code flow with localhost callback        │
│    - Tokens in keychain with automatic refresh              │
│    - Kora bundles OAuth client credentials                  │
├─────────────────────────────────────────────────────────────┤
│ 3. macOS Keychain (Slack token)                             │
│    - OS-managed encryption                                  │
│    - Requires user password to access                       │
├─────────────────────────────────────────────────────────────┤
│ 4. Environment Variable (fallback)                          │
│    - For CI/CD or when keychain unavailable                 │
│    - Last resort only                                       │
└─────────────────────────────────────────────────────────────┘
```

### Core Interfaces

#### AuthProvider Interface

```go
// Package auth provides authentication abstractions for external services.
// IT IS FORBIDDEN TO CHANGE THESE INTERFACES without updating EFA 0002.
package auth

import (
	"context"
	"errors"
)

// Service identifies an authentication target.
type Service string

const (
	ServiceGitHub Service = "github"
	ServiceSlack  Service = "slack"
	ServiceGoogle Service = "google"
)

// AuthProvider manages authentication for a specific service.
// Each service (GitHub, Slack, Google) has exactly one AuthProvider implementation.
//
// Implementations must:
//   - Never log or expose credential values
//   - Respect context cancellation on all operations
//   - Return ErrNotAuthenticated when credentials are missing/invalid
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

// Sentinel errors for authentication operations.
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
)
```

#### Credential Interface

```go
// Credential represents an authentication credential with safe redaction.
// Implementations MUST ensure the credential value is never exposed via
// String(), logging, or any other method except Value().
type Credential interface {
	// Type returns the credential type (e.g., "token", "oauth").
	Type() CredentialType

	// Value returns the raw credential value.
	// WARNING: This value MUST NEVER be logged.
	Value() string

	// Redacted returns a safe-to-log representation.
	// Format: first 4 chars + "..." + last 4 chars for tokens,
	// or a type indicator for non-token credentials.
	Redacted() string

	// IsValid performs format validation on the credential.
	// Does not verify the credential works with the service.
	IsValid() bool
}

// CredentialType identifies the kind of credential.
type CredentialType string

const (
	CredentialTypeOAuth CredentialType = "oauth"  // OAuth token (GitHub, Google)
	CredentialTypeToken CredentialType = "token"  // API token (Slack xoxp-*)
)
```

#### Keychain Interface

```go
// Package keychain provides secure credential storage abstraction.
// IT IS FORBIDDEN TO CHANGE THIS INTERFACE without updating EFA 0002.
package keychain

import (
	"context"
	"errors"
)

// Keychain abstracts secure credential storage operations.
// On macOS, this wraps the Security framework via the `security` CLI.
//
// All operations use:
//   - Service name: "kora"
//   - Account name: the key parameter (e.g., "slack-token")
type Keychain interface {
	// Get retrieves a credential value from the keychain.
	// Returns ErrNotFound if the credential doesn't exist.
	Get(ctx context.Context, key string) (string, error)

	// Set stores a credential value in the keychain.
	// Overwrites any existing value for the same key.
	Set(ctx context.Context, key, value string) error

	// Delete removes a credential from the keychain.
	// Returns nil if the credential didn't exist.
	Delete(ctx context.Context, key string) error

	// Exists checks if a credential exists in the keychain.
	Exists(ctx context.Context, key string) bool
}

// Keychain-specific errors.
var (
	// ErrNotFound indicates the requested credential doesn't exist.
	ErrNotFound = errors.New("keychain: credential not found")

	// ErrAccessDenied indicates the keychain denied access.
	ErrAccessDenied = errors.New("keychain: access denied")

	// ErrUnavailable indicates the keychain service is not available.
	ErrUnavailable = errors.New("keychain: service unavailable")
)
```

### Credential Implementations

#### SlackToken

```go
// SlackToken represents a Slack user token (xoxp-*).
// IT IS FORBIDDEN TO CHANGE THIS TYPE without updating EFA 0002.
type SlackToken struct {
	token string
}

// NewSlackToken creates a SlackToken from a raw token string.
// Returns ErrCredentialInvalid if the token format is invalid.
func NewSlackToken(token string) (*SlackToken, error) {
	t := &SlackToken{token: token}
	if !t.IsValid() {
		return nil, fmt.Errorf("%w: slack token must start with xoxp-", ErrCredentialInvalid)
	}
	return t, nil
}

func (t *SlackToken) Type() CredentialType { return CredentialTypeToken }
func (t *SlackToken) Value() string        { return t.token }

// Redacted returns a safe-to-log representation.
// SECURITY: Shows only a hash-based fingerprint, not any part of the actual token.
// This prevents partial token exposure while still allowing log correlation.
func (t *SlackToken) Redacted() string {
	if len(t.token) < 15 {
		return "xoxp-[invalid]"
	}
	// Generate a fingerprint from the token for log correlation
	// Using first 8 chars of SHA256 hash - enough for correlation, not for cracking
	h := sha256.Sum256([]byte(t.token))
	fingerprint := hex.EncodeToString(h[:4])
	return fmt.Sprintf("xoxp-[%s]", fingerprint)
}

// IsValid checks if the token has the correct format.
func (t *SlackToken) IsValid() bool {
	return strings.HasPrefix(t.token, "xoxp-") && len(t.token) > 10
}

// String implements fmt.Stringer with redaction for safety.
func (t *SlackToken) String() string { return t.Redacted() }
```

#### GitHubDelegatedCredential

```go
// GitHubDelegatedCredential represents GitHub auth via gh CLI delegation.
// IT IS FORBIDDEN TO CHANGE THIS TYPE without updating EFA 0002.
//
// SECURITY: This credential does NOT store or expose the actual OAuth token.
// All API calls are delegated to the gh CLI, which handles token management.
// This is the most secure approach as Kora never sees the token.
type GitHubDelegatedCredential struct {
	username string
	ghPath   string // path to gh CLI for delegation
}

// NewGitHubDelegatedCredential creates a delegated credential.
// Returns ErrCredentialInvalid if username is empty.
func NewGitHubDelegatedCredential(username, ghPath string) (*GitHubDelegatedCredential, error) {
	if username == "" {
		return nil, fmt.Errorf("%w: username is required", ErrCredentialInvalid)
	}
	if ghPath == "" {
		ghPath = "gh"
	}
	return &GitHubDelegatedCredential{username: username, ghPath: ghPath}, nil
}

func (c *GitHubDelegatedCredential) Type() CredentialType { return CredentialTypeOAuth }

// Value returns empty string - tokens are never extracted.
// Use ExecuteAPI() for authenticated API calls instead.
func (c *GitHubDelegatedCredential) Value() string { return "" }

// Redacted returns "github:username" (no token to redact).
func (c *GitHubDelegatedCredential) Redacted() string {
	return fmt.Sprintf("github:%s", c.username)
}

// IsValid returns true if username is set (gh CLI handles token validation).
func (c *GitHubDelegatedCredential) IsValid() bool {
	return c.username != ""
}

func (c *GitHubDelegatedCredential) Username() string { return c.username }
func (c *GitHubDelegatedCredential) String() string   { return c.Redacted() }

// ExecuteAPI executes a GitHub API call via gh CLI delegation.
// This is the ONLY way to make authenticated GitHub API calls.
// SECURITY: The token never leaves gh CLI's control.
func (c *GitHubDelegatedCredential) ExecuteAPI(ctx context.Context, endpoint string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	cmdArgs := append([]string{"api", endpoint}, args...)
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
```

#### GoogleOAuthCredential

```go
// GoogleOAuthCredential represents Google OAuth 2.0 tokens.
// IT IS FORBIDDEN TO CHANGE THIS TYPE without updating EFA 0002.
//
// SECURITY: This credential stores OAuth tokens obtained via browser-based
// Authorization Code flow. Tokens are encrypted in macOS Keychain and
// automatically refreshed before expiry.
type GoogleOAuthCredential struct {
	accessToken  string
	refreshToken string
	expiry       time.Time
	email        string
}

// NewGoogleOAuthCredential creates a Google OAuth credential.
// Returns ErrCredentialInvalid if required fields are missing.
func NewGoogleOAuthCredential(accessToken, refreshToken, email string, expiry time.Time) (*GoogleOAuthCredential, error) {
	c := &GoogleOAuthCredential{
		accessToken:  accessToken,
		refreshToken: refreshToken,
		expiry:       expiry,
		email:        email,
	}
	if !c.IsValid() {
		return nil, fmt.Errorf("%w: access_token, refresh_token, and email are required", ErrCredentialInvalid)
	}
	return c, nil
}

func (c *GoogleOAuthCredential) Type() CredentialType { return CredentialTypeOAuth }

// Value returns the access token.
// WARNING: This value MUST NEVER be logged. Use Redacted() instead.
func (c *GoogleOAuthCredential) Value() string { return c.accessToken }

// Redacted returns a safe-to-log representation.
// Format: "google:{email}:[fingerprint]"
// SECURITY: Shows only the email and a token fingerprint for correlation.
func (c *GoogleOAuthCredential) Redacted() string {
	h := sha256.Sum256([]byte(c.accessToken))
	fingerprint := hex.EncodeToString(h[:4])
	return fmt.Sprintf("google:%s:[%s]", c.email, fingerprint)
}

// IsValid checks if all required fields are present.
// Does NOT verify the token works with Google APIs.
func (c *GoogleOAuthCredential) IsValid() bool {
	return c.accessToken != "" && c.refreshToken != "" && c.email != ""
}

// IsExpired returns true if the access token is expired or expires within 5 minutes.
// The 5-minute buffer ensures we refresh before the token becomes unusable.
func (c *GoogleOAuthCredential) IsExpired() bool {
	return time.Now().After(c.expiry.Add(-5 * time.Minute))
}

// Email returns the Google account email this credential is for.
func (c *GoogleOAuthCredential) Email() string { return c.email }

// RefreshToken returns the refresh token for obtaining new access tokens.
// WARNING: This value MUST NEVER be logged.
func (c *GoogleOAuthCredential) RefreshToken() string { return c.refreshToken }

// Expiry returns when the access token expires.
func (c *GoogleOAuthCredential) Expiry() time.Time { return c.expiry }

// String implements fmt.Stringer with redaction for safety.
func (c *GoogleOAuthCredential) String() string { return c.Redacted() }
```

### Provider Implementations

#### GitHubAuthProvider

```go
// GitHubAuthProvider implements auth.AuthProvider for GitHub via gh CLI delegation.
// IT IS FORBIDDEN TO CHANGE THIS IMPLEMENTATION without updating EFA 0002.
//
// SECURITY: This provider delegates ALL authentication to the gh CLI tool:
//   - Kora NEVER extracts or stores GitHub tokens
//   - Authentication status is checked via `gh auth status`
//   - API calls are delegated via GitHubDelegatedCredential.ExecuteAPI()
//   - The token never leaves gh CLI's control
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

func (p *GitHubAuthProvider) Service() Service { return ServiceGitHub }

func (p *GitHubAuthProvider) Authenticate(ctx context.Context) error {
	if _, err := exec.LookPath(p.ghPath); err != nil {
		return fmt.Errorf("github auth: %w", ErrGHCLINotFound)
	}

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, p.ghPath, "auth", "status")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("github auth: %w", ErrGHCLINotAuthenticated)
	}
	return nil
}

func (p *GitHubAuthProvider) GetCredential(ctx context.Context) (Credential, error) {
	if err := p.Authenticate(ctx); err != nil {
		return nil, err
	}

	// Get username from gh CLI - this is safe to cache
	username, err := p.runGH(ctx, "api", "user", "--jq", ".login")
	if err != nil {
		return nil, fmt.Errorf("github auth: failed to get username: %w", err)
	}

	// SECURITY: Return a delegated credential, NOT the actual token.
	// All API calls will go through gh CLI.
	cred, err := NewGitHubDelegatedCredential(strings.TrimSpace(username), p.ghPath)
	if err != nil {
		return nil, fmt.Errorf("github auth: %w", err)
	}
	return cred, nil
}

func (p *GitHubAuthProvider) IsAuthenticated(ctx context.Context) bool {
	return p.Authenticate(ctx) == nil
}

// runGH executes a gh CLI command and returns stdout.
// Returns an error if the command fails or times out.
func (p *GitHubAuthProvider) runGH(ctx context.Context, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

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
```

#### SlackAuthProvider

```go
// SlackAuthProvider implements auth.AuthProvider for Slack.
// IT IS FORBIDDEN TO CHANGE THIS IMPLEMENTATION without updating EFA 0002.
//
// Credential lookup order:
//  1. macOS Keychain (preferred)
//  2. Environment variable KORA_SLACK_TOKEN (fallback)
type SlackAuthProvider struct {
	keychain Keychain
}

const (
	slackKeychainKey = "slack-token"
	slackEnvVarName  = "KORA_SLACK_TOKEN"
)

// NewSlackAuthProvider creates a Slack auth provider with the given keychain.
func NewSlackAuthProvider(keychain Keychain) *SlackAuthProvider {
	return &SlackAuthProvider{keychain: keychain}
}

func (p *SlackAuthProvider) Service() Service { return ServiceSlack }

func (p *SlackAuthProvider) Authenticate(ctx context.Context) error {
	cred, err := p.GetCredential(ctx)
	if err != nil {
		return err
	}
	if !cred.IsValid() {
		return ErrCredentialInvalid
	}
	return nil
}

func (p *SlackAuthProvider) GetCredential(ctx context.Context) (Credential, error) {
	// 1. Try keychain first
	token, err := p.keychain.Get(ctx, slackKeychainKey)
	if err == nil {
		cred, err := NewSlackToken(token)
		if err != nil {
			// Token in keychain has invalid format
			return nil, fmt.Errorf("slack auth: keychain token invalid: %w", err)
		}
		return cred, nil
	}
	// Only fall through if not found; other errors should propagate
	if !errors.Is(err, ErrNotFound) {
		return nil, fmt.Errorf("slack auth: keychain access failed: %w", err)
	}

	// 2. Fall back to environment variable
	// SECURITY WARNING: Env vars are less secure than keychain.
	// They may be exposed via /proc, crash dumps, or child processes.
	// Log a warning so users know to migrate to keychain.
	if token := os.Getenv(slackEnvVarName); token != "" {
		log.Warn("using Slack token from environment variable - consider storing in keychain for better security")
		cred, err := NewSlackToken(token)
		if err != nil {
			return nil, fmt.Errorf("slack auth: %s invalid: %w", slackEnvVarName, err)
		}
		return cred, nil
	}

	return nil, fmt.Errorf("slack auth: %w: set %s or store in keychain",
		ErrNotAuthenticated, slackEnvVarName)
}

func (p *SlackAuthProvider) IsAuthenticated(ctx context.Context) bool {
	return p.Authenticate(ctx) == nil
}
```

#### GoogleAuthProvider

```go
// GoogleAuthProvider implements auth.AuthProvider for Google via OAuth 2.0.
// IT IS FORBIDDEN TO CHANGE THIS IMPLEMENTATION without updating EFA 0002.
//
// SECURITY: This provider manages OAuth 2.0 Authorization Code flow:
//   - Opens browser for user consent
//   - Localhost callback server (port 8765) receives auth code
//   - Exchanges code for access + refresh tokens
//   - Stores tokens in macOS Keychain per email address
//   - Automatically refreshes tokens before expiry
//
// OAuth Flow:
//   1. User triggers auth for email (e.g., work@company.com)
//   2. Browser opens Google consent screen
//   3. User approves scopes (calendar.readonly, gmail.readonly)
//   4. Redirect to localhost:8765/callback?code=AUTH_CODE&state=CSRF_TOKEN
//   5. Exchange code for tokens via Google token endpoint
//   6. Store in keychain: google-oauth-{email}
//
// Token Refresh:
//   - Before each API call, check expiry (IsExpired checks 5min buffer)
//   - If expired, use refresh token to get new access token
//   - Update keychain with new tokens
type GoogleAuthProvider struct {
	keychain     Keychain
	email        string
	clientID     string
	clientSecret string
	redirectURL  string // default: "http://localhost:8765/callback"
	callbackPort int    // default: 8765
}

const (
	// googleOAuthKeychainKeyPrefix is the prefix for Google OAuth keychain keys.
	// Format: google-oauth-{email}
	googleOAuthKeychainKeyPrefix = "google-oauth-"

	// Google OAuth endpoints
	googleAuthURL  = "https://accounts.google.com/o/oauth2/auth"
	googleTokenURL = "https://oauth2.googleapis.com/token"

	// Scopes for Google Calendar and Gmail read-only access
	googleOAuthScopes = "https://www.googleapis.com/auth/calendar.readonly https://www.googleapis.com/auth/gmail.readonly"
)

// NewGoogleAuthProvider creates a Google auth provider for the given email.
// clientID and clientSecret are bundled in Kora's build.
// If redirectURL is empty, defaults to "http://localhost:8765/callback".
func NewGoogleAuthProvider(keychain Keychain, email, clientID, clientSecret, redirectURL string, callbackPort int) *GoogleAuthProvider {
	if redirectURL == "" {
		redirectURL = "http://localhost:8765/callback"
	}
	if callbackPort == 0 {
		callbackPort = 8765
	}
	return &GoogleAuthProvider{
		keychain:     keychain,
		email:        email,
		clientID:     clientID,
		clientSecret: clientSecret,
		redirectURL:  redirectURL,
		callbackPort: callbackPort,
	}
}

func (p *GoogleAuthProvider) Service() Service { return ServiceGoogle }

func (p *GoogleAuthProvider) Authenticate(ctx context.Context) error {
	// Check if we have a valid credential
	cred, err := p.GetCredential(ctx)
	if err != nil {
		// No credential found or error - trigger OAuth flow
		return p.triggerOAuthFlow(ctx)
	}

	// Check if token is expired
	googleCred, ok := cred.(*GoogleOAuthCredential)
	if !ok {
		return fmt.Errorf("google auth: invalid credential type: %T", cred)
	}

	if googleCred.IsExpired() {
		// Token expired - refresh it
		return p.refreshToken(ctx, googleCred)
	}

	return nil
}

func (p *GoogleAuthProvider) GetCredential(ctx context.Context) (Credential, error) {
	// Get credential from keychain
	key := googleOAuthKeychainKeyPrefix + p.email
	data, err := p.keychain.Get(ctx, key)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, fmt.Errorf("google auth: %w: run auth flow for %s", ErrNotAuthenticated, p.email)
		}
		return nil, fmt.Errorf("google auth: keychain access failed: %w", err)
	}

	// Parse JSON credential
	var tokenData struct {
		AccessToken  string    `json:"access_token"`
		RefreshToken string    `json:"refresh_token"`
		Expiry       time.Time `json:"expiry"`
		Email        string    `json:"email"`
	}
	if err := json.Unmarshal([]byte(data), &tokenData); err != nil {
		return nil, fmt.Errorf("google auth: invalid keychain data: %w", err)
	}

	cred, err := NewGoogleOAuthCredential(
		tokenData.AccessToken,
		tokenData.RefreshToken,
		tokenData.Email,
		tokenData.Expiry,
	)
	if err != nil {
		return nil, fmt.Errorf("google auth: %w", err)
	}

	// Check if expired and refresh if needed
	if cred.IsExpired() {
		if err := p.refreshToken(ctx, cred); err != nil {
			return nil, err
		}
		// Get updated credential from keychain
		return p.GetCredential(ctx)
	}

	return cred, nil
}

func (p *GoogleAuthProvider) IsAuthenticated(ctx context.Context) bool {
	return p.Authenticate(ctx) == nil
}

// triggerOAuthFlow starts the browser-based OAuth flow.
// Opens browser, starts callback server, waits for auth code, exchanges for tokens.
func (p *GoogleAuthProvider) triggerOAuthFlow(ctx context.Context) error {
	// Generate CSRF state token
	stateBytes := make([]byte, 32)
	if _, err := rand.Read(stateBytes); err != nil {
		return fmt.Errorf("google auth: failed to generate state token: %w", err)
	}
	state := base64.URLEncoding.EncodeToString(stateBytes)

	// Build authorization URL
	authURL := fmt.Sprintf("%s?client_id=%s&redirect_uri=%s&response_type=code&scope=%s&state=%s&access_type=offline&prompt=consent",
		googleAuthURL,
		url.QueryEscape(p.clientID),
		url.QueryEscape(p.redirectURL),
		url.QueryEscape(googleOAuthScopes),
		url.QueryEscape(state),
	)

	// Start callback server
	authCodeChan := make(chan string, 1)
	errChan := make(chan error, 1)
	server := &http.Server{Addr: fmt.Sprintf(":%d", p.callbackPort)}

	http.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		// Validate CSRF state
		if r.URL.Query().Get("state") != state {
			errChan <- fmt.Errorf("google auth: CSRF state mismatch")
			http.Error(w, "Invalid state parameter", http.StatusBadRequest)
			return
		}

		// Get authorization code
		code := r.URL.Query().Get("code")
		if code == "" {
			errChan <- fmt.Errorf("google auth: no authorization code received")
			http.Error(w, "No authorization code", http.StatusBadRequest)
			return
		}

		// Send success response to browser
		fmt.Fprintf(w, "<html><body><h1>Authentication successful!</h1><p>You can close this window and return to Kora.</p></body></html>")

		authCodeChan <- code
	})

	// Start server in background
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errChan <- fmt.Errorf("google auth: callback server failed: %w", err)
		}
	}()

	// Open browser
	log.Info("Opening browser for Google authentication", "email", p.email)
	if err := openBrowser(authURL); err != nil {
		server.Shutdown(ctx)
		return fmt.Errorf("google auth: failed to open browser: %w", err)
	}

	// Wait for auth code or error
	var authCode string
	select {
	case authCode = <-authCodeChan:
		// Success - shutdown server
		server.Shutdown(ctx)
	case err := <-errChan:
		server.Shutdown(ctx)
		return err
	case <-ctx.Done():
		server.Shutdown(ctx)
		return fmt.Errorf("google auth: %w: context cancelled", ErrOAuthFlowFailed)
	case <-time.After(5 * time.Minute):
		server.Shutdown(ctx)
		return fmt.Errorf("google auth: %w: timeout waiting for user consent", ErrOAuthFlowFailed)
	}

	// Exchange auth code for tokens
	return p.exchangeCodeForTokens(ctx, authCode)
}

// exchangeCodeForTokens exchanges an authorization code for access and refresh tokens.
func (p *GoogleAuthProvider) exchangeCodeForTokens(ctx context.Context, code string) error {
	// Build token request
	data := url.Values{
		"client_id":     {p.clientID},
		"client_secret": {p.clientSecret},
		"code":          {code},
		"redirect_uri":  {p.redirectURL},
		"grant_type":    {"authorization_code"},
	}

	req, err := http.NewRequestWithContext(ctx, "POST", googleTokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return fmt.Errorf("google auth: failed to create token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	// Make request
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("google auth: token exchange failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("google auth: token exchange failed: %d %s", resp.StatusCode, string(body))
	}

	// Parse response
	var tokenResp struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
		TokenType    string `json:"token_type"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return fmt.Errorf("google auth: failed to parse token response: %w", err)
	}

	// Store tokens in keychain
	expiry := time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second)
	return p.storeTokens(ctx, tokenResp.AccessToken, tokenResp.RefreshToken, expiry)
}

// refreshToken uses the refresh token to get a new access token.
func (p *GoogleAuthProvider) refreshToken(ctx context.Context, cred *GoogleOAuthCredential) error {
	// Build refresh request
	data := url.Values{
		"client_id":     {p.clientID},
		"client_secret": {p.clientSecret},
		"refresh_token": {cred.RefreshToken()},
		"grant_type":    {"refresh_token"},
	}

	req, err := http.NewRequestWithContext(ctx, "POST", googleTokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return fmt.Errorf("google auth: failed to create refresh request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	// Make request
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("google auth: %w: %v", ErrTokenRefreshFailed, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("google auth: %w: %d %s", ErrTokenRefreshFailed, resp.StatusCode, string(body))
	}

	// Parse response
	var tokenResp struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
		TokenType   string `json:"token_type"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return fmt.Errorf("google auth: failed to parse refresh response: %w", err)
	}

	// Store updated tokens (refresh token remains the same)
	expiry := time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second)
	log.Debug("refreshed Google OAuth token", "email", p.email, "expires_in", tokenResp.ExpiresIn)
	return p.storeTokens(ctx, tokenResp.AccessToken, cred.RefreshToken(), expiry)
}

// storeTokens saves OAuth tokens to keychain.
func (p *GoogleAuthProvider) storeTokens(ctx context.Context, accessToken, refreshToken string, expiry time.Time) error {
	tokenData := map[string]interface{}{
		"access_token":  accessToken,
		"refresh_token": refreshToken,
		"expiry":        expiry,
		"email":         p.email,
	}

	data, err := json.Marshal(tokenData)
	if err != nil {
		return fmt.Errorf("google auth: failed to marshal token data: %w", err)
	}

	key := googleOAuthKeychainKeyPrefix + p.email
	if err := p.keychain.Set(ctx, key, string(data)); err != nil {
		return fmt.Errorf("google auth: failed to store tokens in keychain: %w", err)
	}

	log.Debug("stored Google OAuth tokens in keychain", "email", p.email)
	return nil
}

// openBrowser opens the default browser to the given URL.
// Works on macOS, Linux, and Windows.
func openBrowser(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}
	return cmd.Start()
}
```

### macOS Keychain Implementation

```go
// MacOSKeychain implements Keychain using the macOS security CLI.
// IT IS FORBIDDEN TO CHANGE THIS IMPLEMENTATION without updating EFA 0002.
type MacOSKeychain struct {
	securityPath string // default: "/usr/bin/security"
}

const keychainServiceName = "kora"

// allowedKeychainKeys is the authoritative set of valid keychain keys.
// SECURITY: This prevents key injection attacks where a malicious key
// could be crafted to escape the account name parameter.
// IT IS FORBIDDEN TO ADD KEYS without updating this allowlist.
var allowedKeychainKeys = map[string]struct{}{
	"slack-token": {},
	// Google OAuth keys use pattern: google-oauth-{email}
	// Validated separately by googleOAuthKeyPattern
}

// keyPattern validates keychain key format.
// Only alphanumeric characters, hyphens, @ and dots are allowed.
var keyPattern = regexp.MustCompile(`^[a-z][a-z0-9@.-]*[a-z0-9]$`)

// googleOAuthKeyPattern validates google-oauth-{email} format.
var googleOAuthKeyPattern = regexp.MustCompile(`^google-oauth-[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)

// validateKey ensures the key is in the allowlist or matches a valid pattern.
// Returns an error if the key is invalid or not allowed.
func validateKey(key string) error {
	// Check explicit allowlist first
	if _, ok := allowedKeychainKeys[key]; ok {
		return nil
	}

	// Check google-oauth-{email} pattern
	if googleOAuthKeyPattern.MatchString(key) {
		return nil
	}

	// Check general pattern as fallback
	if !keyPattern.MatchString(key) {
		return fmt.Errorf("keychain: key %q contains invalid characters", key)
	}

	return fmt.Errorf("keychain: key %q not in allowlist", key)
}

// NewMacOSKeychain creates a keychain backed by the macOS security CLI.
// If securityPath is empty, defaults to "/usr/bin/security".
func NewMacOSKeychain(securityPath string) *MacOSKeychain {
	if securityPath == "" {
		securityPath = "/usr/bin/security"
	}
	return &MacOSKeychain{securityPath: securityPath}
}

func (k *MacOSKeychain) Get(ctx context.Context, key string) (string, error) {
	if err := validateKey(key); err != nil {
		return "", err
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, k.securityPath,
		"find-generic-password", "-s", keychainServiceName, "-a", key, "-w")

	out, err := cmd.Output()
	if err != nil {
		// Check exit code for "not found" (exit code 44 on macOS)
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			// Exit code 44 = errSecItemNotFound
			if exitErr.ExitCode() == 44 {
				return "", ErrNotFound
			}
			// Exit code 128 = user denied access
			if exitErr.ExitCode() == 128 {
				return "", ErrAccessDenied
			}
			return "", fmt.Errorf("keychain get %q: exit %d: %s",
				key, exitErr.ExitCode(), string(exitErr.Stderr))
		}
		return "", fmt.Errorf("keychain get %q: %w", key, err)
	}
	return strings.TrimSpace(string(out)), nil
}

func (k *MacOSKeychain) Set(ctx context.Context, key, value string) error {
	if err := validateKey(key); err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	// SECURITY: Pass the credential via stdin, not command-line args.
	// Command-line args are visible to other processes via `ps`.
	// The macOS security command accepts password from stdin with -w flag omitted
	// when stdin is provided, but that's not well-documented.
	// Alternative: Use -T flag with pipe, or write to a temp file (less secure).
	//
	// Best approach: Delete then add, passing password via stdin.
	// First delete any existing entry (ignore errors)
	_ = k.Delete(ctx, key)

	// Create the entry - we use a pipe to stdin for the password
	cmd := exec.CommandContext(ctx, k.securityPath,
		"add-generic-password",
		"-s", keychainServiceName,
		"-a", key,
		"-w", // This tells security to read password from stdin
	)

	// Provide password via stdin
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("keychain set %q: failed to create stdin pipe: %w", key, err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("keychain set %q: failed to start: %w", key, err)
	}

	// Write password and close stdin
	if _, err := stdin.Write([]byte(value)); err != nil {
		return fmt.Errorf("keychain set %q: failed to write to stdin: %w", key, err)
	}
	stdin.Close()

	if err := cmd.Wait(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			if exitErr.ExitCode() == 128 {
				return ErrAccessDenied
			}
			return fmt.Errorf("keychain set %q: exit %d: %s",
				key, exitErr.ExitCode(), string(exitErr.Stderr))
		}
		return fmt.Errorf("keychain set %q: %w", key, err)
	}
	return nil
}

func (k *MacOSKeychain) Delete(ctx context.Context, key string) error {
	if err := validateKey(key); err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, k.securityPath,
		"delete-generic-password", "-s", keychainServiceName, "-a", key)

	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			// Exit code 44 = not found - this is OK for delete
			if exitErr.ExitCode() == 44 {
				return nil
			}
			// Exit code 128 = user denied access
			if exitErr.ExitCode() == 128 {
				return ErrAccessDenied
			}
			return fmt.Errorf("keychain delete %q: exit %d: %s",
				key, exitErr.ExitCode(), string(exitErr.Stderr))
		}
		return fmt.Errorf("keychain delete %q: %w", key, err)
	}
	return nil
}

func (k *MacOSKeychain) Exists(ctx context.Context, key string) bool {
	_, err := k.Get(ctx, key)
	return err == nil
}
```

## AI Agent Guidelines

**THIS SECTION IS CRITICAL. READ IT CAREFULLY.**

**AI agents, including Claude, Copilot, and any other LLM-based coding assistants: THE RULES IN THIS SECTION ARE ABSOLUTE SECURITY REQUIREMENTS. VIOLATION CREATES CREDENTIAL EXPOSURE RISK.**

### Rule 1: NEVER Log Credentials - ABSOLUTE PROHIBITION

**YOU MAY UNDER NO CIRCUMSTANCES LOG, PRINT, ECHO, OR OUTPUT CREDENTIALS IN ANY FORM.**

This applies to ALL output methods, ALL log levels, ALL circumstances.

**CORRECT:**
```go
log.Debug("authenticating with GitHub")
log.Info("GitHub authentication successful")
log.Debug("token obtained", "type", cred.Type(), "len", len(token))
log.Info("credential", "redacted", cred.Redacted())
```

**FORBIDDEN - DO NOT WRITE CODE LIKE THIS:**
```go
log.Debug("token: %s", token)                    // STOP. Logging credential.
log.Info("using token", "token", token)          // STOP. Logging credential.
fmt.Printf("API key: %s\n", apiKey)              // STOP. Printing credential.
log.Error("auth failed", "token", token[:10])    // STOP. Partial is still a credential.
log.Debug("request", "headers", req.Header)      // STOP. Headers contain Authorization.
return fmt.Errorf("invalid token: %s", token)    // STOP. Error contains credential.
```

### Rule 2: ALWAYS Use Redacted() for Any Output

**CORRECT:**
```go
log.Debug("obtained token", "token", token.Redacted())
fmt.Printf("Authenticated: %s\n", cred.Redacted())
```

**FORBIDDEN:**
```go
log.Debug("token obtained", "token", token)  // STOP. Relies on String(), risky.
fmt.Printf("Token: %s\n", token.Value())     // STOP. Exposing value.
```

### Rule 3: NEVER Store Credentials in Plaintext

**CORRECT:**
```go
// Keychain storage
keychain.Set(ctx, "slack-token", token)
keychain.Set(ctx, "google-oauth-user@gmail.com", tokenJSON)

// gh CLI delegation (no storage)
cmd := exec.Command("gh", "auth", "token")
```

**FORBIDDEN:**
```go
os.WriteFile("token.txt", []byte(token), 0644)      // STOP. Plaintext file.
os.WriteFile(".kora/credentials", token, 0600)      // STOP. Still plaintext.
viper.Set("github.token", token)                    // STOP. Config file storage.
os.WriteFile("google-creds.json", tokenData, 0600)  // STOP. Plaintext OAuth tokens.
```

### Rule 4: ALWAYS Prefer CLI Delegation or OAuth Flow

For services with existing CLIs (GitHub → `gh`), delegate authentication entirely.
For services with OAuth (Google), use browser-based Authorization Code flow with keychain storage.

**CORRECT:**
```go
// GitHub: Use gh CLI - Kora never stores GitHub tokens
func (g *GitHubProvider) GetToken(ctx context.Context) (string, error) {
    return execGhAuthToken(ctx)  // Delegate to gh
}

// Google: Use OAuth flow - Kora stores tokens in keychain with refresh
func (g *GoogleProvider) Authenticate(ctx context.Context) error {
    cred, err := g.GetCredential(ctx)
    if err != nil || cred.IsExpired() {
        return g.triggerOAuthFlow(ctx)  // Browser-based consent
    }
    return nil
}
```

**FORBIDDEN:**
```go
// DO NOT store GitHub tokens ourselves
func (g *GitHubProvider) SaveToken(token string) error {
    return g.keychain.Save("github", token)  // STOP. Use gh CLI.
}

// DO NOT store OAuth tokens in files
func (g *GoogleProvider) SaveToken(token string) error {
    return os.WriteFile("token.json", []byte(token), 0600)  // STOP. Use keychain.
}
```

### Rule 5: NEVER Log OAuth Tokens or Refresh Tokens

OAuth credentials have additional security requirements:

**CORRECT:**
```go
log.Debug("Google OAuth token refreshed", "email", cred.Email(), "expires_in", expiresIn)
log.Info("authenticated with Google", "credential", cred.Redacted())
```

**FORBIDDEN:**
```go
log.Debug("access token", "token", accessToken)           // STOP. Logging token.
log.Debug("refresh token", "token", refreshToken)         // STOP. Logging refresh token.
log.Debug("token response", "data", tokenResponse)        // STOP. Contains tokens.
fmt.Printf("OAuth credentials: %+v\n", oauthCreds)       // STOP. Contains tokens.
```

### Rule 6: ALWAYS Validate CSRF State in OAuth Callbacks

**CORRECT:**
```go
http.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
    // SECURITY: Validate CSRF state token
    if r.URL.Query().Get("state") != expectedState {
        http.Error(w, "Invalid state parameter", http.StatusBadRequest)
        return
    }
    // Continue with auth code exchange
})
```

**FORBIDDEN:**
```go
http.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
    // STOP. No CSRF validation - security vulnerability.
    code := r.URL.Query().Get("code")
    // Exchange code for tokens...
})
```

### Rule 7: ALWAYS Check Token Expiry Before API Calls

**CORRECT:**
```go
func (c *GoogleClient) FetchCalendar(ctx context.Context) ([]Event, error) {
    // SECURITY: Check expiry and refresh if needed
    if c.credential.IsExpired() {
        if err := c.provider.Authenticate(ctx); err != nil {
            return nil, err
        }
        c.credential, _ = c.provider.GetCredential(ctx)
    }
    // Make API call with fresh token
    return c.fetchWithToken(ctx, c.credential.Value())
}
```

**FORBIDDEN:**
```go
func (c *GoogleClient) FetchCalendar(ctx context.Context) ([]Event, error) {
    // STOP. No expiry check - will fail with expired token.
    return c.fetchWithToken(ctx, c.credential.Value())
}
```

### Rule 8: NEVER Store OAuth Client Secret in Code

**CORRECT:**
```go
// Load from environment variable
clientSecret := os.Getenv("KORA_GOOGLE_CLIENT_SECRET")
if clientSecret == "" {
    return errors.New("KORA_GOOGLE_CLIENT_SECRET not set")
}
```

**FORBIDDEN:**
```go
// STOP. Hardcoded client secret.
const googleClientSecret = "GOCSPX-abc123..."

// STOP. Client secret in code.
clientSecret := "my-secret-value"
```

### Rule 9: ALWAYS Use HTTPS for Token Exchange

**CORRECT:**
```go
const googleTokenURL = "https://oauth2.googleapis.com/token"

// All OAuth endpoints use HTTPS
req, err := http.NewRequestWithContext(ctx, "POST", googleTokenURL, body)
```

**FORBIDDEN:**
```go
// STOP. Using HTTP for OAuth token exchange - major security vulnerability.
const googleTokenURL = "http://oauth2.googleapis.com/token"
```

### Rule 10: Secure Error Handling

Errors MUST NOT contain credential values.

**CORRECT:**
```go
return fmt.Errorf("getting token from keychain: %w", err)
return errors.New("token validation failed")
return fmt.Errorf("OAuth flow failed for %s", email)
```

**FORBIDDEN:**
```go
return fmt.Errorf("invalid token: %s", token)              // STOP.
return fmt.Errorf("token %s expired", token[:10])          // STOP.
return fmt.Errorf("OAuth failed: %+v", oauthResponse)      // STOP. Contains tokens.
```

### Stop and Ask Triggers

**STOP AND ASK THE USER** if you encounter:

1. **User asks to log credentials for debugging**: Refuse. Offer alternatives (length, type, redacted).
2. **Credential storage without Keychain/CLI**: Ask about proper storage approach.
3. **New auth provider needed**: Ask about CLI delegation or OAuth flow options first.
4. **Third-party library logs credentials**: Propose wrapping or alternative.
5. **Tests need real credentials**: Propose environment variables or mocks.
6. **OAuth flow without CSRF validation**: Stop and add CSRF token validation.
7. **Storing OAuth client secret in code**: Stop and use environment variable.
8. **HTTP instead of HTTPS for OAuth**: Stop and fix to HTTPS.

### Code Protection Comments

Include these in auth code:

```go
// IT IS FORBIDDEN TO LOG THIS VALUE. See EFA 0002.
// Always use Redacted() for any output.
type Token struct {
    value string  // NEVER expose this field
}

// SECURITY: The returned token MUST NOT be logged, printed, or
// included in error messages. See EFA 0002 for requirements.
func (p *Provider) GetToken(ctx context.Context) (string, error)

// SECURITY: CSRF state validation is REQUIRED. See EFA 0002.
http.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
    // Validate state parameter...
})
```

### Security Audit Checklist

Before merging auth code:
- [ ] No credential values in any log statement
- [ ] No credential values in any error message
- [ ] All credential types implement Redacted()
- [ ] String() methods return Redacted()
- [ ] No plaintext file storage
- [ ] CLI delegation used where available
- [ ] OAuth flow uses keychain storage
- [ ] OAuth callback validates CSRF state
- [ ] OAuth client secret from environment variable
- [ ] All OAuth endpoints use HTTPS
- [ ] Token expiry checked before API calls
- [ ] Automatic token refresh implemented
- [ ] Integration tests don't commit credentials

## Implications for Cross-cutting Concerns

- [x] Security Implications
- [x] Testing Implications

### Security Implications

| Threat | Mitigation |
|--------|------------|
| Credential theft from files | Never store in files, use Keychain |
| Credential in logs | Redacted() interface method |
| Credential in error messages | Never include value in errors |
| Process memory scraping | Minimize credential lifetime |
| Shell history exposure | Use Keychain, not CLI args |
| OAuth token exposure | Keychain storage, HTTPS only |
| CSRF in OAuth callback | State token validation |
| Expired OAuth tokens | Automatic refresh with 5min buffer |
| OAuth client secret exposure | Environment variable, never in code |
| Man-in-the-middle on OAuth | HTTPS required for all OAuth endpoints |
| Refresh token theft | Keychain storage, never logged |

### Testing Implications

1. **Mock Keychain** for unit tests - never call real `security` command
2. **Mock CommandExecuter** for `gh` CLI tests
3. **Mock OAuth HTTP responses** for Google auth tests
4. **Integration tests** tagged `//go:build integration`

```go
// MockKeychain for testing
type MockKeychain struct {
	store map[string]string
}

func (m *MockKeychain) Get(ctx context.Context, key string) (string, error) {
	if v, ok := m.store[key]; ok {
		return v, nil
	}
	return "", ErrNotFound
}

// MockOAuthServer for testing Google OAuth flow
type MockOAuthServer struct {
	server *httptest.Server
	authCodes map[string]tokenResponse
}

func NewMockOAuthServer() *MockOAuthServer {
	// Setup mock OAuth endpoints
}
```

## Open Questions

1. Should we support credential refresh/rotation hints?
2. Should we add a `kora auth` subcommand for managing credentials?
3. Should we support multiple Google OAuth client credentials (user-provided)?
