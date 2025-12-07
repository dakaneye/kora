package google

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/dakaneye/kora/internal/auth"
	"github.com/dakaneye/kora/internal/auth/keychain"
)

// mockKeychain implements keychain.Keychain for testing.
type mockKeychain struct {
	store map[string]string
	err   error // If set, all operations return this error
}

func newMockKeychain() *mockKeychain {
	return &mockKeychain{
		store: make(map[string]string),
	}
}

func (m *mockKeychain) Get(ctx context.Context, key string) (string, error) {
	if m.err != nil {
		return "", m.err
	}
	if v, ok := m.store[key]; ok {
		return v, nil
	}
	return "", keychain.ErrNotFound
}

func (m *mockKeychain) Set(ctx context.Context, key, value string) error {
	if m.err != nil {
		return m.err
	}
	m.store[key] = value
	return nil
}

func (m *mockKeychain) Delete(ctx context.Context, key string) error {
	if m.err != nil {
		return m.err
	}
	delete(m.store, key)
	return nil
}

func (m *mockKeychain) Exists(ctx context.Context, key string) bool {
	_, ok := m.store[key]
	return ok
}

func TestNewGoogleAuthProvider(t *testing.T) {
	// Set up OAuth config env vars
	t.Setenv("KORA_GOOGLE_CLIENT_ID", "test-client-id")
	t.Setenv("KORA_GOOGLE_CLIENT_SECRET", "test-client-secret")

	kc := newMockKeychain()

	tests := []struct {
		name        string
		email       string
		wantErr     bool
		expectedErr error
	}{
		{
			name:        "valid provider with email",
			email:       "test@example.com",
			wantErr:     false,
			expectedErr: nil,
		},
		{
			name:        "empty email should error",
			email:       "",
			wantErr:     true,
			expectedErr: auth.ErrCredentialInvalid,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider, err := NewGoogleAuthProvider(kc, tt.email)

			if (err != nil) != tt.wantErr {
				t.Errorf("NewGoogleAuthProvider() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr {
				if tt.expectedErr != nil && !errors.Is(err, tt.expectedErr) {
					t.Errorf("NewGoogleAuthProvider() error = %v, want error %v", err, tt.expectedErr)
				}
				return
			}

			if provider == nil {
				t.Error("NewGoogleAuthProvider() returned nil provider")
				return
			}

			if provider.email != tt.email {
				t.Errorf("provider.email = %q, want %q", provider.email, tt.email)
			}

			if provider.keychain == nil {
				t.Error("provider.keychain is nil")
			}

			if provider.config == nil {
				t.Error("provider.config is nil")
			}
		})
	}
}

func TestNewGoogleAuthProvider_MissingOAuthConfig(t *testing.T) {
	// Clear OAuth env vars
	t.Setenv("KORA_GOOGLE_CLIENT_ID", "")
	t.Setenv("KORA_GOOGLE_CLIENT_SECRET", "")

	kc := newMockKeychain()

	_, err := NewGoogleAuthProvider(kc, "test@example.com")
	if err == nil {
		t.Error("NewGoogleAuthProvider() expected error when OAuth config missing")
		return
	}

	if !errors.Is(err, auth.ErrOAuthConfigMissing) {
		t.Errorf("NewGoogleAuthProvider() error = %v, want %v", err, auth.ErrOAuthConfigMissing)
	}
}

func TestGoogleAuthProvider_Service(t *testing.T) {
	t.Setenv("KORA_GOOGLE_CLIENT_ID", "test-client-id")
	t.Setenv("KORA_GOOGLE_CLIENT_SECRET", "test-client-secret")

	kc := newMockKeychain()
	provider, err := NewGoogleAuthProvider(kc, "test@example.com")
	if err != nil {
		t.Fatalf("NewGoogleAuthProvider() error = %v", err)
	}

	if provider.Service() != auth.ServiceGoogle {
		t.Errorf("Service() = %q, want %q", provider.Service(), auth.ServiceGoogle)
	}
}

func TestGoogleAuthProvider_IsAuthenticated(t *testing.T) {
	t.Setenv("KORA_GOOGLE_CLIENT_ID", "test-client-id")
	t.Setenv("KORA_GOOGLE_CLIENT_SECRET", "test-client-secret")

	ctx := context.Background()

	tests := []struct {
		name           string
		setupKeychain  func(*mockKeychain, string)
		expectedResult bool
	}{
		{
			name: "authenticated with valid credential",
			setupKeychain: func(kc *mockKeychain, key string) {
				cred := storedCredential{
					AccessToken:  "valid-access-token",
					RefreshToken: "valid-refresh-token",
					Expiry:       time.Now().Add(1 * time.Hour),
					Email:        "test@example.com",
				}
				data, _ := json.Marshal(cred)
				kc.store[key] = string(data)
			},
			expectedResult: true,
		},
		{
			name: "not authenticated - no credential",
			setupKeychain: func(kc *mockKeychain, key string) {
				// Don't store anything
			},
			expectedResult: false,
		},
		{
			name: "authenticated - expired credential (IsAuthenticated doesn't check expiry)",
			setupKeychain: func(kc *mockKeychain, key string) {
				cred := storedCredential{
					AccessToken:  "expired-access-token",
					RefreshToken: "valid-refresh-token",
					Expiry:       time.Now().Add(-1 * time.Hour), // Expired
					Email:        "test@example.com",
				}
				data, _ := json.Marshal(cred)
				kc.store[key] = string(data)
			},
			// IsAuthenticated() only checks IsValid() which doesn't verify expiry
			// Expiry is only checked in GetCredential() which triggers refresh
			expectedResult: true,
		},
		{
			name: "not authenticated - invalid credential format",
			setupKeychain: func(kc *mockKeychain, key string) {
				kc.store[key] = "invalid-json-data"
			},
			expectedResult: false,
		},
		{
			name: "not authenticated - empty access token",
			setupKeychain: func(kc *mockKeychain, key string) {
				cred := storedCredential{
					AccessToken:  "", // Empty
					RefreshToken: "valid-refresh-token",
					Expiry:       time.Now().Add(1 * time.Hour),
					Email:        "test@example.com",
				}
				data, _ := json.Marshal(cred)
				kc.store[key] = string(data)
			},
			expectedResult: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kc := newMockKeychain()
			provider, err := NewGoogleAuthProvider(kc, "test@example.com")
			if err != nil {
				t.Fatalf("NewGoogleAuthProvider() error = %v", err)
			}

			tt.setupKeychain(kc, provider.keychainKey())

			result := provider.IsAuthenticated(ctx)
			if result != tt.expectedResult {
				t.Errorf("IsAuthenticated() = %v, want %v", result, tt.expectedResult)
			}
		})
	}
}

func TestGoogleAuthProvider_GetCredential(t *testing.T) {
	t.Setenv("KORA_GOOGLE_CLIENT_ID", "test-client-id")
	t.Setenv("KORA_GOOGLE_CLIENT_SECRET", "test-client-secret")

	ctx := context.Background()

	tests := []struct {
		name          string
		setupKeychain func(*mockKeychain, string)
		wantErr       bool
		expectedErr   error
	}{
		{
			name: "get valid credential",
			setupKeychain: func(kc *mockKeychain, key string) {
				cred := storedCredential{
					AccessToken:  "valid-access-token",
					RefreshToken: "valid-refresh-token",
					Expiry:       time.Now().Add(1 * time.Hour),
					Email:        "test@example.com",
				}
				data, _ := json.Marshal(cred)
				kc.store[key] = string(data)
			},
			wantErr:     false,
			expectedErr: nil,
		},
		{
			name: "no credential returns ErrNotAuthenticated",
			setupKeychain: func(kc *mockKeychain, key string) {
				// Don't store anything
			},
			wantErr:     true,
			expectedErr: auth.ErrNotAuthenticated,
		},
		{
			name: "invalid JSON returns ErrCredentialInvalid",
			setupKeychain: func(kc *mockKeychain, key string) {
				kc.store[key] = "not-valid-json"
			},
			wantErr:     true,
			expectedErr: auth.ErrCredentialInvalid,
		},
		{
			name: "empty access token returns ErrCredentialInvalid",
			setupKeychain: func(kc *mockKeychain, key string) {
				cred := storedCredential{
					AccessToken:  "", // Invalid
					RefreshToken: "valid-refresh-token",
					Expiry:       time.Now().Add(1 * time.Hour),
					Email:        "test@example.com",
				}
				data, _ := json.Marshal(cred)
				kc.store[key] = string(data)
			},
			wantErr:     true,
			expectedErr: auth.ErrCredentialInvalid,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kc := newMockKeychain()
			provider, err := NewGoogleAuthProvider(kc, "test@example.com")
			if err != nil {
				t.Fatalf("NewGoogleAuthProvider() error = %v", err)
			}

			tt.setupKeychain(kc, provider.keychainKey())

			cred, err := provider.GetCredential(ctx)

			if (err != nil) != tt.wantErr {
				t.Errorf("GetCredential() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr {
				if tt.expectedErr != nil && !errors.Is(err, tt.expectedErr) {
					t.Errorf("GetCredential() error = %v, want error %v", err, tt.expectedErr)
				}
				return
			}

			if cred == nil {
				t.Error("GetCredential() returned nil credential")
				return
			}

			// Verify credential properties
			if cred.Type() != auth.CredentialTypeOAuth {
				t.Errorf("credential.Type() = %q, want %q", cred.Type(), auth.CredentialTypeOAuth)
			}

			if !cred.IsValid() {
				t.Error("credential.IsValid() = false, want true")
			}
		})
	}
}

func TestGoogleAuthProvider_GetCredential_RefreshExpired(t *testing.T) {
	t.Setenv("KORA_GOOGLE_CLIENT_ID", "test-client-id")
	t.Setenv("KORA_GOOGLE_CLIENT_SECRET", "test-client-secret")

	ctx := context.Background()
	kc := newMockKeychain()

	provider, err := NewGoogleAuthProvider(kc, "test@example.com")
	if err != nil {
		t.Fatalf("NewGoogleAuthProvider() error = %v", err)
	}

	// Store an expired credential
	expiredCred := storedCredential{
		AccessToken:  "expired-access-token",
		RefreshToken: "valid-refresh-token",
		Expiry:       time.Now().Add(-1 * time.Hour), // Expired
		Email:        "test@example.com",
	}
	data, _ := json.Marshal(expiredCred)
	kc.store[provider.keychainKey()] = string(data)

	// GetCredential should attempt to refresh but fail (no real HTTP server)
	_, err = provider.GetCredential(ctx)
	if err == nil {
		t.Error("GetCredential() expected error when refreshing expired token without HTTP mock")
		return
	}

	// Should contain "refreshing token" in error message
	if !strings.Contains(err.Error(), "refreshing token") {
		t.Errorf("GetCredential() error = %v, want error containing 'refreshing token'", err)
	}
}

func TestGoogleAuthProvider_Authenticate(t *testing.T) {
	t.Setenv("KORA_GOOGLE_CLIENT_ID", "test-client-id")
	t.Setenv("KORA_GOOGLE_CLIENT_SECRET", "test-client-secret")

	ctx := context.Background()

	tests := []struct {
		name          string
		setupKeychain func(*mockKeychain, string)
		wantErr       bool
		expectedErr   error
	}{
		{
			name: "authenticate with valid credential",
			setupKeychain: func(kc *mockKeychain, key string) {
				cred := storedCredential{
					AccessToken:  "valid-access-token",
					RefreshToken: "valid-refresh-token",
					Expiry:       time.Now().Add(1 * time.Hour),
					Email:        "test@example.com",
				}
				data, _ := json.Marshal(cred)
				kc.store[key] = string(data)
			},
			wantErr:     false,
			expectedErr: nil,
		},
		{
			name: "authenticate fails with no credential",
			setupKeychain: func(kc *mockKeychain, key string) {
				// Don't store anything
			},
			wantErr:     true,
			expectedErr: auth.ErrNotAuthenticated,
		},
		{
			name: "authenticate fails with invalid credential",
			setupKeychain: func(kc *mockKeychain, key string) {
				cred := storedCredential{
					AccessToken:  "", // Invalid
					RefreshToken: "valid-refresh-token",
					Expiry:       time.Now().Add(1 * time.Hour),
					Email:        "test@example.com",
				}
				data, _ := json.Marshal(cred)
				kc.store[key] = string(data)
			},
			wantErr:     true,
			expectedErr: auth.ErrCredentialInvalid,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kc := newMockKeychain()
			provider, err := NewGoogleAuthProvider(kc, "test@example.com")
			if err != nil {
				t.Fatalf("NewGoogleAuthProvider() error = %v", err)
			}

			tt.setupKeychain(kc, provider.keychainKey())

			err = provider.Authenticate(ctx)

			if (err != nil) != tt.wantErr {
				t.Errorf("Authenticate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr && tt.expectedErr != nil {
				if !errors.Is(err, tt.expectedErr) {
					t.Errorf("Authenticate() error = %v, want error %v", err, tt.expectedErr)
				}
			}
		})
	}
}

func TestGoogleAuthProvider_Logout(t *testing.T) {
	t.Setenv("KORA_GOOGLE_CLIENT_ID", "test-client-id")
	t.Setenv("KORA_GOOGLE_CLIENT_SECRET", "test-client-secret")

	ctx := context.Background()
	kc := newMockKeychain()

	provider, err := NewGoogleAuthProvider(kc, "test@example.com")
	if err != nil {
		t.Fatalf("NewGoogleAuthProvider() error = %v", err)
	}

	// Store a credential
	cred := storedCredential{
		AccessToken:  "access-token",
		RefreshToken: "refresh-token",
		Expiry:       time.Now().Add(1 * time.Hour),
		Email:        "test@example.com",
	}
	data, _ := json.Marshal(cred)
	key := provider.keychainKey()
	kc.store[key] = string(data)

	// Verify credential exists
	if !kc.Exists(ctx, key) {
		t.Fatal("credential should exist before logout")
	}

	// Logout
	err = provider.Logout(ctx)
	if err != nil {
		t.Errorf("Logout() error = %v", err)
	}

	// Verify credential was deleted
	if kc.Exists(ctx, key) {
		t.Error("credential still exists after logout")
	}
}

func TestGoogleAuthProvider_Email(t *testing.T) {
	t.Setenv("KORA_GOOGLE_CLIENT_ID", "test-client-id")
	t.Setenv("KORA_GOOGLE_CLIENT_SECRET", "test-client-secret")

	kc := newMockKeychain()
	email := "test@example.com"

	provider, err := NewGoogleAuthProvider(kc, email)
	if err != nil {
		t.Fatalf("NewGoogleAuthProvider() error = %v", err)
	}

	if provider.Email() != email {
		t.Errorf("Email() = %q, want %q", provider.Email(), email)
	}
}

func TestGoogleAuthProvider_keychainKey(t *testing.T) {
	t.Setenv("KORA_GOOGLE_CLIENT_ID", "test-client-id")
	t.Setenv("KORA_GOOGLE_CLIENT_SECRET", "test-client-secret")

	tests := []struct {
		name          string
		email         string
		expectedKey   string
	}{
		{
			name:        "standard email",
			email:       "user@example.com",
			expectedKey: "google-oauth-user@example.com",
		},
		{
			name:        "email with plus addressing",
			email:       "user+tag@example.com",
			expectedKey: "google-oauth-user+tag@example.com",
		},
		{
			name:        "email with subdomain",
			email:       "user@mail.example.com",
			expectedKey: "google-oauth-user@mail.example.com",
		},
		{
			name:        "gmail address",
			email:       "test.user@gmail.com",
			expectedKey: "google-oauth-test.user@gmail.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kc := newMockKeychain()
			provider, err := NewGoogleAuthProvider(kc, tt.email)
			if err != nil {
				t.Fatalf("NewGoogleAuthProvider() error = %v", err)
			}

			key := provider.keychainKey()
			if key != tt.expectedKey {
				t.Errorf("keychainKey() = %q, want %q", key, tt.expectedKey)
			}

			// Verify key has correct prefix
			if !strings.HasPrefix(key, keychainKeyPrefix) {
				t.Errorf("keychainKey() = %q, want prefix %q", key, keychainKeyPrefix)
			}

			// Verify key contains email
			if !strings.Contains(key, tt.email) {
				t.Errorf("keychainKey() = %q, want to contain email %q", key, tt.email)
			}
		})
	}
}

func TestGoogleAuthProvider_KeychainError(t *testing.T) {
	t.Setenv("KORA_GOOGLE_CLIENT_ID", "test-client-id")
	t.Setenv("KORA_GOOGLE_CLIENT_SECRET", "test-client-secret")

	ctx := context.Background()

	// Create keychain that always returns error
	kc := &mockKeychain{
		store: make(map[string]string),
		err:   errors.New("keychain service unavailable"),
	}

	provider, err := NewGoogleAuthProvider(kc, "test@example.com")
	if err != nil {
		t.Fatalf("NewGoogleAuthProvider() error = %v", err)
	}

	// All keychain operations should fail
	_, err = provider.GetCredential(ctx)
	if err == nil {
		t.Error("GetCredential() expected error when keychain fails")
	}

	err = provider.Authenticate(ctx)
	if err == nil {
		t.Error("Authenticate() expected error when keychain fails")
	}

	if provider.IsAuthenticated(ctx) {
		t.Error("IsAuthenticated() should return false when keychain fails")
	}
}

func TestKeychainKeyPrefix(t *testing.T) {
	// Verify the keychain key prefix is set correctly per EFA 0002
	expectedPrefix := "google-oauth-"
	if keychainKeyPrefix != expectedPrefix {
		t.Errorf("keychainKeyPrefix = %q, want %q", keychainKeyPrefix, expectedPrefix)
	}
}

func TestGoogleAuthProvider_ImplementsAuthProvider(t *testing.T) {
	t.Setenv("KORA_GOOGLE_CLIENT_ID", "test-client-id")
	t.Setenv("KORA_GOOGLE_CLIENT_SECRET", "test-client-secret")

	kc := newMockKeychain()
	provider, err := NewGoogleAuthProvider(kc, "test@example.com")
	if err != nil {
		t.Fatalf("NewGoogleAuthProvider() error = %v", err)
	}

	// Verify provider implements auth.AuthProvider interface
	var _ auth.AuthProvider = provider
}

func TestStoredCredential_JSONSerialization(t *testing.T) {
	now := time.Now().Truncate(time.Second)
	original := storedCredential{
		AccessToken:  "test-access-token",
		RefreshToken: "test-refresh-token",
		Expiry:       now,
		Email:        "test@example.com",
	}

	// Marshal to JSON
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	// Unmarshal back
	var restored storedCredential
	err = json.Unmarshal(data, &restored)
	if err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	// Verify all fields match
	if restored.AccessToken != original.AccessToken {
		t.Errorf("AccessToken = %q, want %q", restored.AccessToken, original.AccessToken)
	}
	if restored.RefreshToken != original.RefreshToken {
		t.Errorf("RefreshToken = %q, want %q", restored.RefreshToken, original.RefreshToken)
	}
	if !restored.Expiry.Equal(original.Expiry) {
		t.Errorf("Expiry = %v, want %v", restored.Expiry, original.Expiry)
	}
	if restored.Email != original.Email {
		t.Errorf("Email = %q, want %q", restored.Email, original.Email)
	}
}
