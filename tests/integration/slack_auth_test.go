//go:build integration && darwin

package integration

import (
	"context"
	"os"
	"runtime"
	"testing"

	"github.com/dakaneye/kora/internal/auth/keychain"
	"github.com/dakaneye/kora/internal/auth/slack"
)

const (
	testKeychainKey = "slack-token"
	testToken       = "xoxp-test-integration-1234567890-0987654321-abcdef123456"
)

// TestSlackAuthProvider_Integration tests Slack authentication with real macOS Keychain.
// This test requires:
//   - Running on macOS (darwin)
//   - User to grant keychain access when prompted
func TestSlackAuthProvider_Integration(t *testing.T) {
	// Skip if not on macOS
	if runtime.GOOS != "darwin" {
		t.Skip("Keychain integration tests only run on macOS")
	}

	// Skip if running in CI without keychain access
	if os.Getenv("CI") != "" {
		t.Skip("Skipping keychain integration test in CI environment")
	}

	kc := keychain.NewMacOSKeychain("")
	ctx := context.Background()

	// Clean up any existing test entry
	t.Cleanup(func() {
		_ = kc.Delete(context.Background(), testKeychainKey)
	})

	t.Run("Set and Get", func(t *testing.T) {
		// Set token in keychain
		err := kc.Set(ctx, testKeychainKey, testToken)
		if err != nil {
			t.Fatalf("kc.Set() error = %v", err)
		}

		// Retrieve token from keychain
		retrieved, err := kc.Get(ctx, testKeychainKey)
		if err != nil {
			t.Fatalf("kc.Get() error = %v", err)
		}

		if retrieved != testToken {
			t.Errorf("kc.Get() = %q, want %q", retrieved, testToken)
		}
	})

	t.Run("Exists", func(t *testing.T) {
		// Set token
		err := kc.Set(ctx, testKeychainKey, testToken)
		if err != nil {
			t.Fatalf("kc.Set() error = %v", err)
		}

		// Check existence
		if !kc.Exists(ctx, testKeychainKey) {
			t.Error("kc.Exists() = false, want true")
		}
	})

	t.Run("Delete", func(t *testing.T) {
		// Set token
		err := kc.Set(ctx, testKeychainKey, testToken)
		if err != nil {
			t.Fatalf("kc.Set() error = %v", err)
		}

		// Delete token
		err = kc.Delete(ctx, testKeychainKey)
		if err != nil {
			t.Fatalf("kc.Delete() error = %v", err)
		}

		// Verify deleted
		if kc.Exists(ctx, testKeychainKey) {
			t.Error("kc.Exists() = true, want false after delete")
		}

		// Get should return not found
		_, err = kc.Get(ctx, testKeychainKey)
		if err != keychain.ErrNotFound {
			t.Errorf("kc.Get() error = %v, want %v", err, keychain.ErrNotFound)
		}
	})

	t.Run("Delete non-existent", func(t *testing.T) {
		// Delete non-existent entry should not error
		err := kc.Delete(ctx, testKeychainKey)
		if err != nil {
			t.Errorf("kc.Delete() non-existent error = %v, want nil", err)
		}
	})

	t.Run("Get non-existent", func(t *testing.T) {
		// Ensure key doesn't exist
		_ = kc.Delete(ctx, testKeychainKey)

		// Get should return not found
		_, err := kc.Get(ctx, testKeychainKey)
		if err != keychain.ErrNotFound {
			t.Errorf("kc.Get() non-existent error = %v, want %v", err, keychain.ErrNotFound)
		}
	})

	t.Run("SlackAuthProvider with Keychain", func(t *testing.T) {
		// Set token in keychain
		err := kc.Set(ctx, testKeychainKey, testToken)
		if err != nil {
			t.Fatalf("kc.Set() error = %v", err)
		}

		provider := slack.NewSlackAuthProvider(kc, nil)

		// Test Authenticate
		err = provider.Authenticate(ctx)
		if err != nil {
			t.Fatalf("provider.Authenticate() error = %v", err)
		}

		// Test IsAuthenticated
		if !provider.IsAuthenticated(ctx) {
			t.Error("provider.IsAuthenticated() = false, want true")
		}

		// Test GetCredential
		cred, err := provider.GetCredential(ctx)
		if err != nil {
			t.Fatalf("provider.GetCredential() error = %v", err)
		}

		if cred == nil {
			t.Fatal("provider.GetCredential() returned nil credential")
		}

		if !cred.IsValid() {
			t.Error("provider.GetCredential() returned invalid credential")
		}

		// Verify credential value
		if cred.Value() != testToken {
			t.Errorf("credential.Value() = %q, want %q", cred.Value(), testToken)
		}

		// Verify credential type
		if cred.Type() != "token" {
			t.Errorf("credential.Type() = %q, want %q", cred.Type(), "token")
		}

		// Verify redaction doesn't expose token
		redacted := cred.Redacted()
		if redacted == "" {
			t.Error("credential.Redacted() returned empty string")
		}
		if redacted == testToken {
			t.Error("credential.Redacted() exposed raw token")
		}
		// Should have format xoxp-[hash]
		if len(redacted) < 7 || redacted[:5] != "xoxp-" {
			t.Errorf("credential.Redacted() = %q, want format 'xoxp-[hash]'", redacted)
		}
	})

	t.Run("SlackAuthProvider not authenticated", func(t *testing.T) {
		// Ensure token doesn't exist
		_ = kc.Delete(ctx, testKeychainKey)

		provider := slack.NewSlackAuthProvider(kc, nil)

		// Test Authenticate should fail
		err := provider.Authenticate(ctx)
		if err == nil {
			t.Error("provider.Authenticate() expected error when token not found")
		}

		// Test IsAuthenticated should return false
		if provider.IsAuthenticated(ctx) {
			t.Error("provider.IsAuthenticated() = true, want false")
		}

		// Test GetCredential should fail
		_, err = provider.GetCredential(ctx)
		if err == nil {
			t.Error("provider.GetCredential() expected error when token not found")
		}
	})

	t.Run("Keychain update overwrites", func(t *testing.T) {
		// Set initial token
		token1 := "xoxp-initial-token-12345"
		err := kc.Set(ctx, testKeychainKey, token1)
		if err != nil {
			t.Fatalf("kc.Set() token1 error = %v", err)
		}

		// Update with new token
		token2 := "xoxp-updated-token-67890"
		err = kc.Set(ctx, testKeychainKey, token2)
		if err != nil {
			t.Fatalf("kc.Set() token2 error = %v", err)
		}

		// Retrieve should get updated token
		retrieved, err := kc.Get(ctx, testKeychainKey)
		if err != nil {
			t.Fatalf("kc.Get() error = %v", err)
		}

		if retrieved != token2 {
			t.Errorf("kc.Get() = %q, want %q (updated token)", retrieved, token2)
		}
	})
}

// TestKeychain_InvalidKey tests keychain operations with invalid keys.
func TestKeychain_InvalidKey(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("Keychain integration tests only run on macOS")
	}

	if os.Getenv("CI") != "" {
		t.Skip("Skipping keychain integration test in CI environment")
	}

	kc := keychain.NewMacOSKeychain("")
	ctx := context.Background()

	invalidKeys := []string{
		"not-in-allowlist",
		"UPPERCASE",
		"has spaces",
		"special@chars",
		"",
	}

	for _, key := range invalidKeys {
		t.Run("Get invalid key: "+key, func(t *testing.T) {
			_, err := kc.Get(ctx, key)
			if err == nil {
				t.Errorf("kc.Get(%q) expected error, got nil", key)
			}
		})

		t.Run("Set invalid key: "+key, func(t *testing.T) {
			err := kc.Set(ctx, key, "value")
			if err == nil {
				t.Errorf("kc.Set(%q) expected error, got nil", key)
			}
		})

		t.Run("Delete invalid key: "+key, func(t *testing.T) {
			err := kc.Delete(ctx, key)
			if err == nil {
				t.Errorf("kc.Delete(%q) expected error, got nil", key)
			}
		})
	}
}
