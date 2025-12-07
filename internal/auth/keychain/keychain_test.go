package keychain

import (
	"testing"
)

func TestValidateKey(t *testing.T) {
	tests := []struct {
		name    string
		key     string
		wantErr bool
	}{
		{
			name:    "valid key in allowlist",
			key:     "slack-token",
			wantErr: false,
		},
		{
			name:    "invalid key not in allowlist",
			key:     "not-allowed",
			wantErr: true,
		},
		{
			name:    "malformed key with uppercase",
			key:     "UPPERCASE",
			wantErr: true,
		},
		{
			name:    "malformed key with spaces",
			key:     "has spaces",
			wantErr: true,
		},
		{
			name:    "malformed key starting with hyphen",
			key:     "-starts-hyphen",
			wantErr: true,
		},
		{
			name:    "malformed key ending with hyphen",
			key:     "ends-hyphen-",
			wantErr: true,
		},
		{
			name:    "malformed key with special chars",
			key:     "special@chars",
			wantErr: true,
		},
		{
			name:    "empty key",
			key:     "",
			wantErr: true,
		},
		{
			name:    "single character key not in allowlist",
			key:     "a",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateKey(tt.key)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateKey(%q) error = %v, wantErr %v", tt.key, err, tt.wantErr)
			}
		})
	}
}

func TestKeyPattern(t *testing.T) {
	tests := []struct {
		name  string
		key   string
		match bool
	}{
		{
			name:  "valid lowercase with hyphens",
			key:   "slack-token",
			match: true,
		},
		{
			name:  "valid lowercase alphanumeric",
			key:   "token123",
			match: true,
		},
		{
			name:  "valid mixed alphanumeric with hyphens",
			key:   "api-token-v2",
			match: true,
		},
		{
			name:  "invalid uppercase",
			key:   "Slack-Token",
			match: false,
		},
		{
			name:  "invalid starting with number",
			key:   "123-token",
			match: false,
		},
		{
			name:  "invalid starting with hyphen",
			key:   "-slack-token",
			match: false,
		},
		{
			name:  "invalid ending with hyphen",
			key:   "slack-token-",
			match: false,
		},
		{
			name:  "invalid with underscore",
			key:   "slack_token",
			match: false,
		},
		{
			name:  "invalid with dot",
			key:   "slack.token",
			match: false,
		},
		{
			name:  "invalid single char",
			key:   "a",
			match: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			match := keyPattern.MatchString(tt.key)
			if match != tt.match {
				t.Errorf("keyPattern.MatchString(%q) = %v, want %v", tt.key, match, tt.match)
			}
		})
	}
}

func TestKeychainServiceName(t *testing.T) {
	// Ensure the service name is set to "kora" as per EFA 0002
	if keychainServiceName != "kora" {
		t.Errorf("keychainServiceName = %q, want %q", keychainServiceName, "kora")
	}
}

func TestAllowedKeychainKeys(t *testing.T) {
	// Verify that allowedKeychainKeys contains slack-token
	if _, ok := allowedKeychainKeys["slack-token"]; !ok {
		t.Error("allowedKeychainKeys missing required key: slack-token")
	}

	// Verify that all keys in allowlist match the keyPattern
	for key := range allowedKeychainKeys {
		if !keyPattern.MatchString(key) {
			t.Errorf("allowedKeychainKeys contains invalid key %q that doesn't match keyPattern", key)
		}
	}
}

func TestGoogleOAuthKeyPattern(t *testing.T) {
	tests := []struct {
		name  string
		key   string
		match bool
	}{
		{
			name:  "valid google-oauth email",
			key:   "google-oauth-user@example.com",
			match: true,
		},
		{
			name:  "valid google-oauth with subdomain",
			key:   "google-oauth-user@mail.example.com",
			match: true,
		},
		{
			name:  "valid google-oauth with plus addressing",
			key:   "google-oauth-user+tag@example.com",
			match: true,
		},
		{
			name:  "valid google-oauth with dots in local part",
			key:   "google-oauth-first.last@example.com",
			match: true,
		},
		{
			name:  "valid google-oauth with numbers",
			key:   "google-oauth-user123@example123.com",
			match: true,
		},
		{
			name:  "invalid missing prefix",
			key:   "user@example.com",
			match: false,
		},
		{
			name:  "invalid wrong prefix",
			key:   "oauth-google-user@example.com",
			match: false,
		},
		{
			name:  "invalid missing @",
			key:   "google-oauth-userexample.com",
			match: false,
		},
		{
			name:  "invalid missing domain",
			key:   "google-oauth-user@",
			match: false,
		},
		{
			name:  "invalid missing TLD",
			key:   "google-oauth-user@example",
			match: false,
		},
		{
			name:  "invalid TLD too short",
			key:   "google-oauth-user@example.c",
			match: false,
		},
		{
			name:  "invalid empty email",
			key:   "google-oauth-",
			match: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			match := googleOAuthKeyPattern.MatchString(tt.key)
			if match != tt.match {
				t.Errorf("googleOAuthKeyPattern.MatchString(%q) = %v, want %v", tt.key, match, tt.match)
			}
		})
	}
}

func TestValidateKey_GoogleOAuth(t *testing.T) {
	tests := []struct {
		name    string
		key     string
		wantErr bool
	}{
		{
			name:    "valid google-oauth key",
			key:     "google-oauth-user@example.com",
			wantErr: false,
		},
		{
			name:    "valid google-oauth with gmail domain",
			key:     "google-oauth-test.user@gmail.com",
			wantErr: false,
		},
		{
			name:    "valid google-oauth with plus addressing",
			key:     "google-oauth-user+label@example.org",
			wantErr: false,
		},
		{
			name:    "invalid google-oauth without proper email format",
			key:     "google-oauth-not-an-email",
			wantErr: true,
		},
		{
			name:    "invalid google-oauth empty email",
			key:     "google-oauth-@.com",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateKey(tt.key)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateKey(%q) error = %v, wantErr %v", tt.key, err, tt.wantErr)
			}
		})
	}
}
