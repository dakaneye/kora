// Package google provides Google OAuth authentication for Calendar and Gmail.
// Ground truth defined in specs/efas/0002-auth-provider.md
//
// IT IS FORBIDDEN TO CHANGE THIS IMPLEMENTATION without updating EFA 0002.
// Claude MUST stop and ask before modifying this file.
package google

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/dakaneye/kora/internal/auth"
)

// OAuth endpoint constants.
// SECURITY: All endpoints use HTTPS. IT IS FORBIDDEN to use HTTP.
const (
	googleAuthURL = "https://accounts.google.com/o/oauth2/v2/auth"
	//nolint:gosec // G101: This is an OAuth endpoint URL, not a credential
	googleTokenURL    = "https://oauth2.googleapis.com/token"
	googleUserInfoURL = "https://www.googleapis.com/oauth2/v2/userinfo"
)

// OAuth flow configuration.
const (
	callbackPort = 8765
	authTimeout  = 5 * time.Minute
	httpTimeout  = 30 * time.Second
)

// googleScopes defines the OAuth scopes for Google Calendar and Gmail read-only access.
var googleScopes = []string{
	"https://www.googleapis.com/auth/calendar.readonly",
	"https://www.googleapis.com/auth/gmail.readonly",
	"email", // To get user's email
}

// OAuthConfig holds OAuth client credentials.
// SECURITY: These should be provided via environment variables, never hardcoded.
type OAuthConfig struct {
	ClientID     string
	ClientSecret string
}

// GetOAuthConfig loads OAuth credentials from environment variables.
// Returns auth.ErrOAuthConfigMissing if credentials are not set.
//
// Required environment variables:
//   - KORA_GOOGLE_CLIENT_ID: OAuth 2.0 Client ID
//   - KORA_GOOGLE_CLIENT_SECRET: OAuth 2.0 Client Secret
func GetOAuthConfig() (*OAuthConfig, error) {
	clientID := os.Getenv("KORA_GOOGLE_CLIENT_ID")
	clientSecret := os.Getenv("KORA_GOOGLE_CLIENT_SECRET")

	if clientID == "" {
		return nil, fmt.Errorf("%w: KORA_GOOGLE_CLIENT_ID environment variable not set", auth.ErrOAuthConfigMissing)
	}
	if clientSecret == "" {
		return nil, fmt.Errorf("%w: KORA_GOOGLE_CLIENT_SECRET environment variable not set", auth.ErrOAuthConfigMissing)
	}

	return &OAuthConfig{
		ClientID:     clientID,
		ClientSecret: clientSecret,
	}, nil
}

// InitiateOAuthFlow starts the browser-based OAuth flow and returns credentials on success.
// This opens a browser window for user consent and waits for the callback.
//
// OAuth Flow:
//  1. Generate CSRF state token
//  2. Start localhost callback server on port 8765
//  3. Open browser to Google consent screen
//  4. User approves, Google redirects to localhost:8765/callback
//  5. Exchange auth code for tokens
//  6. Return GoogleOAuthCredential
//
// SECURITY:
//   - CSRF state token prevents cross-site request forgery
//   - All token exchanges use HTTPS
//   - Tokens are NEVER logged
func InitiateOAuthFlow(ctx context.Context, config *OAuthConfig) (*GoogleOAuthCredential, error) {
	// Generate CSRF state token
	state, err := generateState()
	if err != nil {
		return nil, fmt.Errorf("google oauth: generating state: %w", err)
	}

	// Channel to receive the auth code or error
	resultChan := make(chan oauthResult, 1)

	// Start callback server
	redirectURI := fmt.Sprintf("http://localhost:%d/callback", callbackPort)
	server, err := startCallbackServer(state, resultChan)
	if err != nil {
		return nil, fmt.Errorf("google oauth: starting callback server: %w", err)
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		//nolint:errcheck // best-effort shutdown
		_ = server.Shutdown(shutdownCtx)
	}()

	// Build authorization URL
	authURL := buildAuthURL(config.ClientID, redirectURI, state)

	// Open browser
	if err := openBrowser(authURL); err != nil {
		return nil, fmt.Errorf("google oauth: opening browser: %w", err)
	}

	fmt.Println("Opening browser for Google authentication...")
	fmt.Printf("If the browser doesn't open, visit:\n%s\n\n", authURL)

	// Wait for callback with timeout
	ctx, cancel := context.WithTimeout(ctx, authTimeout)
	defer cancel()

	select {
	case result := <-resultChan:
		if result.err != nil {
			return nil, fmt.Errorf("google oauth: %w", result.err)
		}
		return exchangeCodeForTokens(ctx, config, result.code, redirectURI)
	case <-ctx.Done():
		return nil, fmt.Errorf("google oauth: %w: user did not complete authentication within %v", auth.ErrOAuthFlowFailed, authTimeout)
	}
}

// oauthResult holds the result from the OAuth callback.
type oauthResult struct {
	err  error
	code string
}

// generateState creates a cryptographically random state token for CSRF protection.
func generateState() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("reading random bytes: %w", err)
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

// buildAuthURL constructs the Google OAuth authorization URL.
func buildAuthURL(clientID, redirectURI, state string) string {
	params := url.Values{
		"client_id":     {clientID},
		"redirect_uri":  {redirectURI},
		"response_type": {"code"},
		"scope":         {strings.Join(googleScopes, " ")},
		"state":         {state},
		"access_type":   {"offline"}, // Request refresh token
		"prompt":        {"consent"}, // Always show consent to ensure refresh token
	}
	return googleAuthURL + "?" + params.Encode()
}

// startCallbackServer starts an HTTP server to receive the OAuth callback.
// SECURITY: Validates CSRF state token on every callback.
func startCallbackServer(expectedState string, resultChan chan<- oauthResult) (*http.Server, error) {
	lc := net.ListenConfig{}
	listener, err := lc.Listen(context.Background(), "tcp", fmt.Sprintf(":%d", callbackPort))
	if err != nil {
		return nil, fmt.Errorf("listening on port %d: %w", callbackPort, err)
	}

	mux := http.NewServeMux()
	server := &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		// SECURITY: Validate CSRF state token
		state := r.URL.Query().Get("state")
		if state != expectedState {
			resultChan <- oauthResult{err: fmt.Errorf("%w: invalid state parameter (possible CSRF)", auth.ErrOAuthFlowFailed)}
			http.Error(w, "Invalid state parameter", http.StatusBadRequest)
			return
		}

		// Check for OAuth error
		if errParam := r.URL.Query().Get("error"); errParam != "" {
			errDesc := r.URL.Query().Get("error_description")
			resultChan <- oauthResult{err: fmt.Errorf("%w: %s - %s", auth.ErrOAuthFlowFailed, errParam, errDesc)}
			w.Header().Set("Content-Type", "text/html")
			//nolint:errcheck // best-effort response to browser
			_, _ = fmt.Fprintf(w, "<html><body><h1>Authentication Failed</h1><p>%s</p><p>You can close this window.</p></body></html>", errDesc)
			return
		}

		// Get authorization code
		code := r.URL.Query().Get("code")
		if code == "" {
			resultChan <- oauthResult{err: fmt.Errorf("%w: no authorization code received", auth.ErrOAuthFlowFailed)}
			http.Error(w, "No authorization code", http.StatusBadRequest)
			return
		}

		// Success!
		w.Header().Set("Content-Type", "text/html")
		//nolint:errcheck // best-effort response to browser
		_, _ = fmt.Fprint(w, "<html><body><h1>Authentication Successful!</h1><p>You can close this window and return to Kora.</p></body></html>")
		resultChan <- oauthResult{code: code}
	})

	go func() {
		if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
			resultChan <- oauthResult{err: fmt.Errorf("callback server: %w", err)}
		}
	}()

	return server, nil
}

// openBrowser opens the default browser to the given URL.
// Supports macOS, Linux, and Windows.
func openBrowser(targetURL string) error {
	// Use a short timeout context for opening the browser
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.CommandContext(ctx, "open", targetURL)
	case "linux":
		cmd = exec.CommandContext(ctx, "xdg-open", targetURL)
	case "windows":
		cmd = exec.CommandContext(ctx, "rundll32", "url.dll,FileProtocolHandler", targetURL)
	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}
	return cmd.Start()
}

// tokenResponse represents the JSON response from Google's token endpoint.
// SECURITY: These values MUST NEVER be logged.
type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	IDToken      string `json:"id_token,omitempty"`
	TokenType    string `json:"token_type"`
	Scope        string `json:"scope"`
	ExpiresIn    int    `json:"expires_in"`
}

// exchangeCodeForTokens exchanges an authorization code for access and refresh tokens.
// SECURITY: All communication uses HTTPS. Tokens are NEVER logged.
func exchangeCodeForTokens(ctx context.Context, config *OAuthConfig, code, redirectURI string) (*GoogleOAuthCredential, error) {
	data := url.Values{
		"code":          {code},
		"client_id":     {config.ClientID},
		"client_secret": {config.ClientSecret},
		"redirect_uri":  {redirectURI},
		"grant_type":    {"authorization_code"},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, googleTokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("creating token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{Timeout: httpTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("token exchange request: %w", err)
	}
	defer func() {
		//nolint:errcheck // best-effort close
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		// Read error response but don't include potentially sensitive info
		return nil, fmt.Errorf("token exchange failed: status %d", resp.StatusCode)
	}

	var tokenResp tokenResponse
	if decodeErr := json.NewDecoder(resp.Body).Decode(&tokenResp); decodeErr != nil {
		return nil, fmt.Errorf("decoding token response: %w", decodeErr)
	}

	if tokenResp.AccessToken == "" {
		return nil, fmt.Errorf("no access token in response")
	}
	if tokenResp.RefreshToken == "" {
		return nil, fmt.Errorf("no refresh token in response (user may need to revoke and re-authorize)")
	}

	// Get user email from userinfo endpoint
	email, err := getUserEmail(ctx, tokenResp.AccessToken)
	if err != nil {
		return nil, fmt.Errorf("getting user email: %w", err)
	}

	expiry := time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second)

	return NewGoogleOAuthCredential(tokenResp.AccessToken, tokenResp.RefreshToken, email, expiry)
}

// userInfoResponse represents the JSON response from Google's userinfo endpoint.
type userInfoResponse struct {
	Email string `json:"email"`
}

// getUserEmail retrieves the user's email address from Google's userinfo API.
func getUserEmail(ctx context.Context, accessToken string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, googleUserInfoURL, http.NoBody)
	if err != nil {
		return "", fmt.Errorf("creating userinfo request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("userinfo request: %w", err)
	}
	defer func() {
		//nolint:errcheck // best-effort close
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("userinfo request failed: status %d", resp.StatusCode)
	}

	var userInfo userInfoResponse
	if decodeErr := json.NewDecoder(resp.Body).Decode(&userInfo); decodeErr != nil {
		return "", fmt.Errorf("decoding userinfo response: %w", decodeErr)
	}

	if userInfo.Email == "" {
		return "", fmt.Errorf("no email in userinfo response")
	}

	return userInfo.Email, nil
}

// RefreshAccessToken obtains a new access token using the refresh token.
// Returns the new access token and its expiry time.
//
// SECURITY:
//   - The refresh token is NEVER logged
//   - All communication uses HTTPS
func RefreshAccessToken(ctx context.Context, config *OAuthConfig, refreshToken string) (string, time.Time, error) {
	data := url.Values{
		"refresh_token": {refreshToken},
		"client_id":     {config.ClientID},
		"client_secret": {config.ClientSecret},
		"grant_type":    {"refresh_token"},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, googleTokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return "", time.Time{}, fmt.Errorf("creating refresh request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{Timeout: httpTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("%w: %v", auth.ErrTokenRefreshFailed, err)
	}
	defer func() {
		//nolint:errcheck // best-effort close
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return "", time.Time{}, fmt.Errorf("%w: status %d", auth.ErrTokenRefreshFailed, resp.StatusCode)
	}

	var tokenResp tokenResponse
	if decodeErr := json.NewDecoder(resp.Body).Decode(&tokenResp); decodeErr != nil {
		return "", time.Time{}, fmt.Errorf("decoding refresh response: %w", decodeErr)
	}

	if tokenResp.AccessToken == "" {
		return "", time.Time{}, fmt.Errorf("no access token in refresh response")
	}

	expiry := time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second)
	return tokenResp.AccessToken, expiry, nil
}
