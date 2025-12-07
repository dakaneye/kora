//go:build integration

package integration

import (
	"context"
	"strings"
	"testing"

	"github.com/dakaneye/kora/internal/auth/google"
	"github.com/dakaneye/kora/internal/auth/keychain"
)

// TestGoogleAuthProvider_Integration tests Google OAuth authentication with real keychain.
// This test requires:
//   - macOS (for keychain support)
//   - Google OAuth credentials stored in keychain as "google-oauth-{email}"
//   - GOOGLE_TEST_EMAIL environment variable set to the test email
func TestGoogleAuthProvider_Integration(t *testing.T) {
	email := requireGoogleAuth(t)

	kc := keychain.NewMacOSKeychain("")
	provider, err := google.NewGoogleAuthProvider(kc, email)
	if err != nil {
		t.Fatalf("NewGoogleAuthProvider() error = %v", err)
	}

	ctx := context.Background()

	t.Run("Authenticate", func(t *testing.T) {
		err := provider.Authenticate(ctx)
		if err != nil {
			t.Fatalf("Authenticate() error = %v", err)
		}
	})

	t.Run("IsAuthenticated", func(t *testing.T) {
		if !provider.IsAuthenticated(ctx) {
			t.Error("IsAuthenticated() = false, want true")
		}
	})

	t.Run("GetCredential", func(t *testing.T) {
		cred, err := provider.GetCredential(ctx)
		if err != nil {
			t.Fatalf("GetCredential() error = %v", err)
		}

		if cred == nil {
			t.Fatal("GetCredential() returned nil credential")
		}

		if !cred.IsValid() {
			t.Error("GetCredential() returned invalid credential")
		}

		// Verify credential type
		if cred.Type() != "oauth" {
			t.Errorf("credential.Type() = %q, want %q", cred.Type(), "oauth")
		}

		// Verify Value() returns non-empty access token
		if cred.Value() == "" {
			t.Error("credential.Value() returned empty string, want access token")
		}

		// Verify Redacted() has expected format: "google:{email}:[hash]"
		redacted := cred.Redacted()
		expectedPrefix := "google:" + email + ":["
		if !strings.HasPrefix(redacted, expectedPrefix) {
			t.Errorf("credential.Redacted() = %q, want format %q", redacted, expectedPrefix+"...]")
		}

		// Verify redacted format ends with ]
		if !strings.HasSuffix(redacted, "]") {
			t.Errorf("credential.Redacted() = %q, want format ending with ']'", redacted)
		}
	})

	t.Run("GetCredential_WithEmail", func(t *testing.T) {
		cred, err := provider.GetCredential(ctx)
		if err != nil {
			t.Fatalf("GetCredential() error = %v", err)
		}

		// Cast to GoogleOAuthCredential to access Email()
		googleCred, ok := cred.(*google.GoogleOAuthCredential)
		if !ok {
			t.Fatal("credential is not *GoogleOAuthCredential")
		}

		// Verify email matches
		credEmail := googleCred.Email()
		if credEmail != email {
			t.Errorf("credential.Email() = %q, want %q", credEmail, email)
		}
	})

	t.Run("GetCredential_Refreshable", func(t *testing.T) {
		cred, err := provider.GetCredential(ctx)
		if err != nil {
			t.Fatalf("GetCredential() error = %v", err)
		}

		// Cast to GoogleOAuthCredential to access RefreshToken()
		googleCred, ok := cred.(*google.GoogleOAuthCredential)
		if !ok {
			t.Fatal("credential is not *GoogleOAuthCredential")
		}

		// Verify refresh token exists
		refreshToken := googleCred.RefreshToken()
		if refreshToken == "" {
			t.Error("credential.RefreshToken() returned empty string")
		}

		// Verify expiry is set
		expiry := googleCred.Expiry()
		if expiry.IsZero() {
			t.Error("credential.Expiry() returned zero time")
		}
	})

	t.Run("Provider_Email", func(t *testing.T) {
		providerEmail := provider.Email()
		if providerEmail != email {
			t.Errorf("provider.Email() = %q, want %q", providerEmail, email)
		}
	})
}

// TestGoogleAuthProvider_NotAuthenticated tests behavior when credentials don't exist.
func TestGoogleAuthProvider_NotAuthenticated(t *testing.T) {
	skipInCI(t)
	skipNonMacOS(t)

	// Use a non-existent email that won't have credentials
	nonExistentEmail := "nonexistent-test@example.com"

	kc := keychain.NewMacOSKeychain("")
	provider, err := google.NewGoogleAuthProvider(kc, nonExistentEmail)
	if err != nil {
		t.Fatalf("NewGoogleAuthProvider() error = %v", err)
	}

	ctx := context.Background()

	t.Run("Authenticate should fail", func(t *testing.T) {
		err := provider.Authenticate(ctx)
		if err == nil {
			t.Error("Authenticate() expected error when not authenticated")
		}
	})

	t.Run("IsAuthenticated should return false", func(t *testing.T) {
		if provider.IsAuthenticated(ctx) {
			t.Error("IsAuthenticated() = true, want false")
		}
	})

	t.Run("GetCredential should fail", func(t *testing.T) {
		_, err := provider.GetCredential(ctx)
		if err == nil {
			t.Error("GetCredential() expected error when not authenticated")
		}
	})
}

// TestGoogleAuthProvider_InvalidEmail tests creation with invalid email.
func TestGoogleAuthProvider_InvalidEmail(t *testing.T) {
	skipInCI(t)
	skipNonMacOS(t)

	kc := keychain.NewMacOSKeychain("")

	t.Run("Empty email", func(t *testing.T) {
		_, err := google.NewGoogleAuthProvider(kc, "")
		if err == nil {
			t.Error("NewGoogleAuthProvider(\"\") expected error for empty email")
		}
	})
}
