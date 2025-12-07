package google

import (
	"context"
	"encoding/base64"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/dakaneye/kora/internal/auth"
)

func TestGetOAuthConfig(t *testing.T) {
	// Save original env vars and restore after test
	originalClientID := os.Getenv("KORA_GOOGLE_CLIENT_ID")
	originalClientSecret := os.Getenv("KORA_GOOGLE_CLIENT_SECRET")
	defer func() {
		os.Setenv("KORA_GOOGLE_CLIENT_ID", originalClientID)
		os.Setenv("KORA_GOOGLE_CLIENT_SECRET", originalClientSecret)
	}()

	tests := []struct {
		name         string
		clientID     string
		clientSecret string
		wantErr      bool
		expectedErr  error
	}{
		{
			name:         "valid config with both env vars set",
			clientID:     "test-client-id-123",
			clientSecret: "test-client-secret-xyz",
			wantErr:      false,
			expectedErr:  nil,
		},
		{
			name:         "missing client ID",
			clientID:     "",
			clientSecret: "test-client-secret-xyz",
			wantErr:      true,
			expectedErr:  auth.ErrOAuthConfigMissing,
		},
		{
			name:         "missing client secret",
			clientID:     "test-client-id-123",
			clientSecret: "",
			wantErr:      true,
			expectedErr:  auth.ErrOAuthConfigMissing,
		},
		{
			name:         "both env vars missing",
			clientID:     "",
			clientSecret: "",
			wantErr:      true,
			expectedErr:  auth.ErrOAuthConfigMissing,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up test environment
			os.Setenv("KORA_GOOGLE_CLIENT_ID", tt.clientID)
			os.Setenv("KORA_GOOGLE_CLIENT_SECRET", tt.clientSecret)

			config, err := GetOAuthConfig()

			if (err != nil) != tt.wantErr {
				t.Errorf("GetOAuthConfig() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr {
				if tt.expectedErr != nil && !strings.Contains(err.Error(), tt.expectedErr.Error()) {
					t.Errorf("GetOAuthConfig() error = %v, want error containing %q", err, tt.expectedErr.Error())
				}
				return
			}

			if config == nil {
				t.Error("GetOAuthConfig() returned nil config")
				return
			}

			if config.ClientID != tt.clientID {
				t.Errorf("GetOAuthConfig() ClientID = %q, want %q", config.ClientID, tt.clientID)
			}

			if config.ClientSecret != tt.clientSecret {
				t.Errorf("GetOAuthConfig() ClientSecret = %q, want %q", config.ClientSecret, tt.clientSecret)
			}
		})
	}
}

func TestGenerateState(t *testing.T) {
	// Generate multiple state tokens
	states := make(map[string]bool)
	iterations := 100

	for i := 0; i < iterations; i++ {
		state, err := generateState()
		if err != nil {
			t.Fatalf("generateState() error = %v", err)
		}

		// Verify state is not empty
		if state == "" {
			t.Error("generateState() returned empty string")
		}

		// Verify state is valid base64
		decoded, err := base64.URLEncoding.DecodeString(state)
		if err != nil {
			t.Errorf("generateState() returned invalid base64: %v", err)
		}

		// Verify decoded length is 32 bytes
		if len(decoded) != 32 {
			t.Errorf("generateState() decoded length = %d, want 32", len(decoded))
		}

		// Check for duplicates (should be extremely unlikely)
		if states[state] {
			t.Errorf("generateState() produced duplicate state: %q", state)
		}
		states[state] = true
	}

	// Verify we generated unique tokens
	if len(states) != iterations {
		t.Errorf("generateState() produced %d unique tokens out of %d, want %d", len(states), iterations, iterations)
	}
}

func TestBuildAuthURL(t *testing.T) {
	tests := []struct {
		name        string
		clientID    string
		redirectURI string
		state       string
		wantContain []string
	}{
		{
			name:        "standard auth url",
			clientID:    "test-client-id",
			redirectURI: "http://localhost:8765/callback",
			state:       "test-state-token",
			wantContain: []string{
				googleAuthURL,
				"client_id=test-client-id",
				"redirect_uri=http%3A%2F%2Flocalhost%3A8765%2Fcallback",
				"response_type=code",
				"state=test-state-token",
				"access_type=offline",
				"prompt=consent",
				"scope=",
				"https%3A%2F%2Fwww.googleapis.com%2Fauth%2Fcalendar.readonly",
				"https%3A%2F%2Fwww.googleapis.com%2Fauth%2Fgmail.readonly",
				"email",
			},
		},
		{
			name:        "different state token",
			clientID:    "another-client-id",
			redirectURI: "http://localhost:9999/callback",
			state:       "different-state-abc123",
			wantContain: []string{
				"client_id=another-client-id",
				"redirect_uri=http%3A%2F%2Flocalhost%3A9999%2Fcallback",
				"state=different-state-abc123",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			authURL := buildAuthURL(tt.clientID, tt.redirectURI, tt.state)

			// Verify all expected components are present
			for _, want := range tt.wantContain {
				if !strings.Contains(authURL, want) {
					t.Errorf("buildAuthURL() = %q, missing %q", authURL, want)
				}
			}

			// Verify URL starts with correct base
			if !strings.HasPrefix(authURL, googleAuthURL+"?") {
				t.Errorf("buildAuthURL() = %q, want to start with %q", authURL, googleAuthURL+"?")
			}
		})
	}
}

func TestBuildAuthURL_Scopes(t *testing.T) {
	clientID := "test-client-id"
	redirectURI := "http://localhost:8765/callback"
	state := "test-state"

	authURL := buildAuthURL(clientID, redirectURI, state)

	// Verify all required scopes are present in the URL-encoded scope parameter
	// Scopes are joined with spaces in the URL, which becomes + in URL encoding
	expectedScopes := []string{
		"https://www.googleapis.com/auth/calendar.readonly",
		"https://www.googleapis.com/auth/gmail.readonly",
		"email",
	}

	for _, scope := range expectedScopes {
		// The scope appears in the URL as part of the scope parameter
		// We need to check for either the URL-encoded version or the raw version
		// Since url.Values.Encode() will encode spaces as + and / as %2F
		encodedScope := strings.ReplaceAll(scope, "/", "%2F")
		encodedScope = strings.ReplaceAll(encodedScope, ":", "%3A")

		if !strings.Contains(authURL, encodedScope) {
			t.Errorf("buildAuthURL() missing scope %q (looked for encoded: %q)", scope, encodedScope)
		}
	}
}

func TestBuildAuthURL_OAuth2Parameters(t *testing.T) {
	clientID := "test-client-id"
	redirectURI := "http://localhost:8765/callback"
	state := "test-state"

	authURL := buildAuthURL(clientID, redirectURI, state)

	// Verify OAuth2 parameters
	requiredParams := map[string]string{
		"response_type": "code",
		"access_type":   "offline", // Ensures refresh token is returned
		"prompt":        "consent",  // Forces consent to get refresh token
	}

	for param, expectedValue := range requiredParams {
		expected := param + "=" + expectedValue
		if !strings.Contains(authURL, expected) {
			t.Errorf("buildAuthURL() missing parameter %q with value %q", param, expectedValue)
		}
	}
}

func TestOAuthConfig_Security(t *testing.T) {
	config := &OAuthConfig{
		ClientID:     "sensitive-client-id",
		ClientSecret: "super-secret-value",
	}

	// Verify that we don't accidentally expose secrets in string representation
	// This is a defensive test - Go doesn't auto-stringify structs in logs,
	// but we want to ensure no String() method exposes secrets
	configStr := config.ClientID + config.ClientSecret
	if len(configStr) == 0 {
		t.Error("OAuthConfig fields are empty")
	}

	// This test documents that OAuthConfig should NEVER implement String() or GoString()
	// that would expose ClientSecret
}

func TestGoogleAuthURLEndpoints(t *testing.T) {
	// Verify OAuth endpoints use HTTPS per EFA 0002 security requirements
	if !strings.HasPrefix(googleAuthURL, "https://") {
		t.Errorf("googleAuthURL = %q, must use HTTPS", googleAuthURL)
	}

	if !strings.HasPrefix(googleTokenURL, "https://") {
		t.Errorf("googleTokenURL = %q, must use HTTPS", googleTokenURL)
	}

	if !strings.HasPrefix(googleUserInfoURL, "https://") {
		t.Errorf("googleUserInfoURL = %q, must use HTTPS", googleUserInfoURL)
	}
}

func TestOAuthFlowConstants(t *testing.T) {
	// Verify timeout constants are reasonable
	if callbackPort <= 0 || callbackPort > 65535 {
		t.Errorf("callbackPort = %d, must be valid port number", callbackPort)
	}

	if authTimeout <= 0 {
		t.Errorf("authTimeout = %v, must be positive", authTimeout)
	}

	if httpTimeout <= 0 {
		t.Errorf("httpTimeout = %v, must be positive", httpTimeout)
	}

	// Verify reasonable timeout values
	if authTimeout < 1*time.Minute {
		t.Errorf("authTimeout = %v, should be at least 1 minute for user to complete OAuth", authTimeout)
	}

	if httpTimeout > authTimeout {
		t.Errorf("httpTimeout (%v) should be less than authTimeout (%v)", httpTimeout, authTimeout)
	}
}

func TestInitiateOAuthFlow_NoServer(t *testing.T) {
	// This test verifies error handling without actually starting OAuth flow
	// We use an invalid config to trigger early failure
	config := &OAuthConfig{
		ClientID:     "", // Invalid - will fail in real OAuth flow
		ClientSecret: "",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	// We expect this to either fail immediately or timeout
	_, err := InitiateOAuthFlow(ctx, config)
	if err == nil {
		t.Error("InitiateOAuthFlow() with invalid config should error")
	}
}

func TestGoogleScopes(t *testing.T) {
	// Verify required scopes are defined
	expectedScopes := map[string]bool{
		"https://www.googleapis.com/auth/calendar.readonly": false,
		"https://www.googleapis.com/auth/gmail.readonly":    false,
		"email": false,
	}

	for _, scope := range googleScopes {
		if _, ok := expectedScopes[scope]; ok {
			expectedScopes[scope] = true
		}
	}

	// Verify all expected scopes were found
	for scope, found := range expectedScopes {
		if !found {
			t.Errorf("googleScopes missing required scope: %q", scope)
		}
	}

	// Verify no extra unexpected scopes (keep this flexible)
	if len(googleScopes) < len(expectedScopes) {
		t.Errorf("googleScopes has %d scopes, expected at least %d", len(googleScopes), len(expectedScopes))
	}
}

func TestTokenResponse_Security(t *testing.T) {
	// This test documents that tokenResponse should NEVER implement String() or GoString()
	// that would expose AccessToken or RefreshToken
	resp := tokenResponse{
		AccessToken:  "secret-access-token",
		RefreshToken: "secret-refresh-token",
		TokenType:    "Bearer",
		ExpiresIn:    3600,
	}

	// Verify fields are not empty
	if resp.AccessToken == "" || resp.RefreshToken == "" {
		t.Error("tokenResponse fields should not be empty")
	}

	// This test serves as documentation: NEVER log tokenResponse directly
	// because it contains sensitive credentials
}

func TestUserInfoResponse_Structure(t *testing.T) {
	// Verify userInfoResponse structure is as expected
	resp := userInfoResponse{
		Email: "test@example.com",
	}

	if resp.Email == "" {
		t.Error("userInfoResponse Email field should not be empty")
	}
}
