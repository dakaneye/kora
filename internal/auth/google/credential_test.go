package google

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"
	"time"

	"github.com/dakaneye/kora/internal/auth"
)

func TestNewGoogleOAuthCredential(t *testing.T) {
	tests := []struct {
		name         string
		accessToken  string
		refreshToken string
		email        string
		expiry       time.Time
		wantErr      bool
	}{
		{
			name:         "valid credential with all fields",
			accessToken:  "ya29.a0AfH6SMC...",
			refreshToken: "1//0gZxyz...",
			email:        "user@example.com",
			expiry:       time.Now().Add(1 * time.Hour),
			wantErr:      false,
		},
		{
			name:         "valid credential with zero expiry",
			accessToken:  "ya29.a0AfH6SMC...",
			refreshToken: "1//0gZxyz...",
			email:        "user@example.com",
			expiry:       time.Time{},
			wantErr:      false,
		},
		{
			name:         "empty access token should error",
			accessToken:  "",
			refreshToken: "1//0gZxyz...",
			email:        "user@example.com",
			expiry:       time.Now().Add(1 * time.Hour),
			wantErr:      true,
		},
		{
			name:         "empty refresh token should error",
			accessToken:  "ya29.a0AfH6SMC...",
			refreshToken: "",
			email:        "user@example.com",
			expiry:       time.Now().Add(1 * time.Hour),
			wantErr:      true,
		},
		{
			name:         "empty email should error",
			accessToken:  "ya29.a0AfH6SMC...",
			refreshToken: "1//0gZxyz...",
			email:        "",
			expiry:       time.Now().Add(1 * time.Hour),
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cred, err := NewGoogleOAuthCredential(tt.accessToken, tt.refreshToken, tt.email, tt.expiry)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewGoogleOAuthCredential() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if cred == nil {
					t.Error("NewGoogleOAuthCredential() returned nil credential")
					return
				}
				if cred.accessToken != tt.accessToken {
					t.Errorf("accessToken = %q, want %q", cred.accessToken, tt.accessToken)
				}
				if cred.refreshToken != tt.refreshToken {
					t.Errorf("refreshToken = %q, want %q", cred.refreshToken, tt.refreshToken)
				}
				if cred.email != tt.email {
					t.Errorf("email = %q, want %q", cred.email, tt.email)
				}
				if !cred.expiry.Equal(tt.expiry) {
					t.Errorf("expiry = %v, want %v", cred.expiry, tt.expiry)
				}
			}
		})
	}
}

func TestGoogleOAuthCredential_Type(t *testing.T) {
	cred, err := NewGoogleOAuthCredential(
		"ya29.a0AfH6SMC...",
		"1//0gZxyz...",
		"user@example.com",
		time.Now().Add(1*time.Hour),
	)
	if err != nil {
		t.Fatalf("NewGoogleOAuthCredential() error = %v", err)
	}

	if cred.Type() != auth.CredentialTypeOAuth {
		t.Errorf("Type() = %q, want %q", cred.Type(), auth.CredentialTypeOAuth)
	}
}

func TestGoogleOAuthCredential_Value(t *testing.T) {
	accessToken := "ya29.a0AfH6SMC..."
	cred, err := NewGoogleOAuthCredential(
		accessToken,
		"1//0gZxyz...",
		"user@example.com",
		time.Now().Add(1*time.Hour),
	)
	if err != nil {
		t.Fatalf("NewGoogleOAuthCredential() error = %v", err)
	}

	if cred.Value() != accessToken {
		t.Errorf("Value() = %q, want %q", cred.Value(), accessToken)
	}
}

func TestGoogleOAuthCredential_Redacted(t *testing.T) {
	tests := []struct {
		name        string
		accessToken string
		email       string
	}{
		{
			name:        "standard email and token",
			accessToken: "ya29.a0AfH6SMC...",
			email:       "user@example.com",
		},
		{
			name:        "different token same email",
			accessToken: "ya29.DIFFERENT...",
			email:       "user@example.com",
		},
		{
			name:        "email with plus addressing",
			accessToken: "ya29.a0AfH6SMC...",
			email:       "user+tag@example.com",
		},
		{
			name:        "subdomain email",
			accessToken: "ya29.a0AfH6SMC...",
			email:       "user@subdomain.example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cred, err := NewGoogleOAuthCredential(
				tt.accessToken,
				"1//0gZxyz...",
				tt.email,
				time.Now().Add(1*time.Hour),
			)
			if err != nil {
				t.Fatalf("NewGoogleOAuthCredential() error = %v", err)
			}

			redacted := cred.Redacted()

			// Verify format: "google:{email}:[8-char-hex]"
			if !strings.HasPrefix(redacted, "google:") {
				t.Errorf("Redacted() = %q, want prefix 'google:'", redacted)
			}
			if !strings.Contains(redacted, tt.email) {
				t.Errorf("Redacted() = %q, want to contain email %q", redacted, tt.email)
			}

			// Extract fingerprint part
			parts := strings.Split(redacted, ":")
			if len(parts) != 3 {
				t.Errorf("Redacted() = %q, want format 'google:{email}:[fingerprint]'", redacted)
				return
			}
			fingerprint := strings.Trim(parts[2], "[]")
			if len(fingerprint) != 8 {
				t.Errorf("fingerprint = %q, want 8 hex characters", fingerprint)
			}

			// Verify it's valid hex
			if _, err := hex.DecodeString(fingerprint); err != nil {
				t.Errorf("fingerprint %q is not valid hex: %v", fingerprint, err)
			}

			// Verify no part of actual token appears
			if strings.Contains(redacted, tt.accessToken) {
				t.Errorf("Redacted() contains actual access token - SECURITY VIOLATION")
			}

			// Verify same token produces same fingerprint
			cred2, _ := NewGoogleOAuthCredential(
				tt.accessToken,
				"1//0gZxyz...",
				tt.email,
				time.Now().Add(1*time.Hour),
			)
			if cred2.Redacted() != redacted {
				t.Errorf("same token produced different fingerprints: %q vs %q", cred2.Redacted(), redacted)
			}
		})
	}
}

func TestGoogleOAuthCredential_Redacted_DifferentTokensDifferentFingerprints(t *testing.T) {
	email := "user@example.com"
	token1 := "ya29.a0AfH6SMC..."
	token2 := "ya29.DIFFERENT..."

	cred1, err := NewGoogleOAuthCredential(token1, "1//0gZxyz...", email, time.Now().Add(1*time.Hour))
	if err != nil {
		t.Fatalf("NewGoogleOAuthCredential() error = %v", err)
	}

	cred2, err := NewGoogleOAuthCredential(token2, "1//0gZxyz...", email, time.Now().Add(1*time.Hour))
	if err != nil {
		t.Fatalf("NewGoogleOAuthCredential() error = %v", err)
	}

	if cred1.Redacted() == cred2.Redacted() {
		t.Errorf("different tokens produced same redacted value: %q", cred1.Redacted())
	}
}

func TestGoogleOAuthCredential_IsValid(t *testing.T) {
	tests := []struct {
		name         string
		accessToken  string
		refreshToken string
		email        string
		valid        bool
	}{
		{
			name:         "valid with all fields",
			accessToken:  "ya29.a0AfH6SMC...",
			refreshToken: "1//0gZxyz...",
			email:        "user@example.com",
			valid:        true,
		},
		{
			name:         "invalid with empty access token",
			accessToken:  "",
			refreshToken: "1//0gZxyz...",
			email:        "user@example.com",
			valid:        false,
		},
		{
			name:         "invalid with empty refresh token",
			accessToken:  "ya29.a0AfH6SMC...",
			refreshToken: "",
			email:        "user@example.com",
			valid:        false,
		},
		{
			name:         "invalid with empty email",
			accessToken:  "ya29.a0AfH6SMC...",
			refreshToken: "1//0gZxyz...",
			email:        "",
			valid:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Bypass constructor validation to test IsValid() directly
			cred := &GoogleOAuthCredential{
				accessToken:  tt.accessToken,
				refreshToken: tt.refreshToken,
				email:        tt.email,
				expiry:       time.Now().Add(1 * time.Hour),
			}
			if got := cred.IsValid(); got != tt.valid {
				t.Errorf("IsValid() = %v, want %v", got, tt.valid)
			}
		})
	}
}

func TestGoogleOAuthCredential_IsExpired(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name    string
		expiry  time.Time
		expired bool
	}{
		{
			name:    "token not expired - 1 hour from now",
			expiry:  now.Add(1 * time.Hour),
			expired: false,
		},
		{
			name:    "token expires in 10 minutes - not yet in buffer",
			expiry:  now.Add(10 * time.Minute),
			expired: false,
		},
		{
			name:    "token expires in 3 minutes - within buffer",
			expiry:  now.Add(3 * time.Minute),
			expired: true,
		},
		{
			name:    "token expires in exactly 5 minutes - buffer boundary",
			expiry:  now.Add(5 * time.Minute),
			expired: true, // Within the 5-minute buffer, so considered expired
		},
		{
			name:    "token already expired",
			expiry:  now.Add(-1 * time.Hour),
			expired: true,
		},
		{
			name:    "zero expiry time",
			expiry:  time.Time{},
			expired: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cred := &GoogleOAuthCredential{
				accessToken:  "ya29.a0AfH6SMC...",
				refreshToken: "1//0gZxyz...",
				email:        "user@example.com",
				expiry:       tt.expiry,
			}
			if got := cred.IsExpired(); got != tt.expired {
				t.Errorf("IsExpired() = %v, want %v (expiry: %v, now: %v, diff: %v)",
					got, tt.expired, tt.expiry, now, tt.expiry.Sub(now))
			}
		})
	}
}

func TestGoogleOAuthCredential_Email(t *testing.T) {
	email := "user@example.com"
	cred, err := NewGoogleOAuthCredential(
		"ya29.a0AfH6SMC...",
		"1//0gZxyz...",
		email,
		time.Now().Add(1*time.Hour),
	)
	if err != nil {
		t.Fatalf("NewGoogleOAuthCredential() error = %v", err)
	}

	if got := cred.Email(); got != email {
		t.Errorf("Email() = %q, want %q", got, email)
	}
}

func TestGoogleOAuthCredential_RefreshToken(t *testing.T) {
	refreshToken := "1//0gZxyz..."
	cred, err := NewGoogleOAuthCredential(
		"ya29.a0AfH6SMC...",
		refreshToken,
		"user@example.com",
		time.Now().Add(1*time.Hour),
	)
	if err != nil {
		t.Fatalf("NewGoogleOAuthCredential() error = %v", err)
	}

	if got := cred.RefreshToken(); got != refreshToken {
		t.Errorf("RefreshToken() = %q, want %q", got, refreshToken)
	}
}

func TestGoogleOAuthCredential_Expiry(t *testing.T) {
	expiry := time.Now().Add(1 * time.Hour).Truncate(time.Second)
	cred, err := NewGoogleOAuthCredential(
		"ya29.a0AfH6SMC...",
		"1//0gZxyz...",
		"user@example.com",
		expiry,
	)
	if err != nil {
		t.Fatalf("NewGoogleOAuthCredential() error = %v", err)
	}

	if got := cred.Expiry(); !got.Equal(expiry) {
		t.Errorf("Expiry() = %v, want %v", got, expiry)
	}
}

func TestGoogleOAuthCredential_String(t *testing.T) {
	cred, err := NewGoogleOAuthCredential(
		"ya29.a0AfH6SMC...",
		"1//0gZxyz...",
		"user@example.com",
		time.Now().Add(1*time.Hour),
	)
	if err != nil {
		t.Fatalf("NewGoogleOAuthCredential() error = %v", err)
	}

	// String() should return same as Redacted() for safety
	if cred.String() != cred.Redacted() {
		t.Errorf("String() = %q, want %q", cred.String(), cred.Redacted())
	}

	// String() should never contain the actual token
	if strings.Contains(cred.String(), cred.accessToken) {
		t.Errorf("String() contains actual access token - SECURITY VIOLATION")
	}
}

func TestGoogleOAuthCredential_WithNewAccessToken(t *testing.T) {
	originalAccessToken := "ya29.ORIGINAL..."
	originalRefreshToken := "1//0gZxyz..."
	originalEmail := "user@example.com"
	originalExpiry := time.Now().Add(1 * time.Hour).Truncate(time.Second)

	original, err := NewGoogleOAuthCredential(
		originalAccessToken,
		originalRefreshToken,
		originalEmail,
		originalExpiry,
	)
	if err != nil {
		t.Fatalf("NewGoogleOAuthCredential() error = %v", err)
	}

	newAccessToken := "ya29.NEW..."
	newExpiry := time.Now().Add(2 * time.Hour).Truncate(time.Second)

	updated := original.WithNewAccessToken(newAccessToken, newExpiry)

	// Verify new values
	if updated.accessToken != newAccessToken {
		t.Errorf("updated.accessToken = %q, want %q", updated.accessToken, newAccessToken)
	}
	if !updated.expiry.Equal(newExpiry) {
		t.Errorf("updated.expiry = %v, want %v", updated.expiry, newExpiry)
	}

	// Verify preserved values
	if updated.refreshToken != originalRefreshToken {
		t.Errorf("updated.refreshToken = %q, want %q", updated.refreshToken, originalRefreshToken)
	}
	if updated.email != originalEmail {
		t.Errorf("updated.email = %q, want %q", updated.email, originalEmail)
	}

	// Verify original unchanged
	if original.accessToken != originalAccessToken {
		t.Errorf("original.accessToken was modified: %q, want %q", original.accessToken, originalAccessToken)
	}
	if !original.expiry.Equal(originalExpiry) {
		t.Errorf("original.expiry was modified: %v, want %v", original.expiry, originalExpiry)
	}
}

func TestGoogleOAuthCredential_RedactedFingerprint(t *testing.T) {
	// Test that the fingerprint matches expected SHA256 calculation
	accessToken := "test-token-123"
	email := "test@example.com"

	cred, err := NewGoogleOAuthCredential(
		accessToken,
		"1//0gZxyz...",
		email,
		time.Now().Add(1*time.Hour),
	)
	if err != nil {
		t.Fatalf("NewGoogleOAuthCredential() error = %v", err)
	}

	// Calculate expected fingerprint
	h := sha256.Sum256([]byte(accessToken))
	expectedFingerprint := hex.EncodeToString(h[:4])
	expectedRedacted := "google:test@example.com:[" + expectedFingerprint + "]"

	if cred.Redacted() != expectedRedacted {
		t.Errorf("Redacted() = %q, want %q", cred.Redacted(), expectedRedacted)
	}
}

func TestExpiryBuffer(t *testing.T) {
	if expiryBuffer != 5*time.Minute {
		t.Errorf("expiryBuffer = %v, want 5m", expiryBuffer)
	}
}
