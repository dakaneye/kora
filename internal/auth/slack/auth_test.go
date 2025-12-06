package slack

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"log/slog"
	"os"
	"strings"
	"testing"

	"github.com/dakaneye/kora/internal/auth"
	"github.com/dakaneye/kora/internal/auth/keychain"
)

// MockKeychain implements keychain.Keychain for testing.
type MockKeychain struct {
	data map[string]string
	err  error
}

func NewMockKeychain() *MockKeychain {
	return &MockKeychain{
		data: make(map[string]string),
	}
}

func (m *MockKeychain) Get(ctx context.Context, key string) (string, error) {
	if m.err != nil {
		return "", m.err
	}
	val, ok := m.data[key]
	if !ok {
		return "", keychain.ErrNotFound
	}
	return val, nil
}

func (m *MockKeychain) Set(ctx context.Context, key, value string) error {
	if m.err != nil {
		return m.err
	}
	m.data[key] = value
	return nil
}

func (m *MockKeychain) Delete(ctx context.Context, key string) error {
	if m.err != nil {
		return m.err
	}
	delete(m.data, key)
	return nil
}

func (m *MockKeychain) Exists(ctx context.Context, key string) bool {
	_, ok := m.data[key]
	return ok
}

func TestNewSlackToken(t *testing.T) {
	tests := []struct {
		name    string
		token   string
		wantErr bool
	}{
		{
			name:    "valid token with xoxp prefix",
			token:   "xoxp-1234567890123",
			wantErr: false,
		},
		{
			name:    "valid long token",
			token:   "xoxp-123456789012345678901234567890",
			wantErr: false,
		},
		{
			name:    "invalid token without xoxp prefix",
			token:   "abcd-1234567890123",
			wantErr: true,
		},
		{
			name:    "invalid token too short",
			token:   "xoxp-123",
			wantErr: true,
		},
		{
			name:    "invalid token only prefix",
			token:   "xoxp-",
			wantErr: true,
		},
		{
			name:    "invalid empty token",
			token:   "",
			wantErr: true,
		},
		{
			name:    "invalid token with wrong prefix",
			token:   "xoxb-1234567890123",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token, err := NewSlackToken(tt.token)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewSlackToken() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if token == nil {
					t.Error("NewSlackToken() returned nil token")
				}
				if !token.IsValid() {
					t.Error("NewSlackToken() returned invalid token")
				}
			}
		})
	}
}

func TestSlackToken_IsValid(t *testing.T) {
	tests := []struct {
		name  string
		token string
		valid bool
	}{
		{
			name:  "valid token",
			token: "xoxp-1234567890123",
			valid: true,
		},
		{
			name:  "invalid without prefix",
			token: "1234567890123",
			valid: false,
		},
		{
			name:  "invalid too short",
			token: "xoxp-123",
			valid: false,
		},
		{
			name:  "invalid empty",
			token: "",
			valid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token := &SlackToken{token: tt.token}
			if got := token.IsValid(); got != tt.valid {
				t.Errorf("SlackToken.IsValid() = %v, want %v", got, tt.valid)
			}
		})
	}
}

func TestSlackToken_Redacted(t *testing.T) {
	// Test that Redacted() returns fingerprint format
	token, err := NewSlackToken("xoxp-1234567890123-4567890123456-7890123456789-abc123def456ghi789jkl012mno345")
	if err != nil {
		t.Fatalf("NewSlackToken() error = %v", err)
	}

	redacted := token.Redacted()

	// Check format: xoxp-[8chars]
	if !strings.HasPrefix(redacted, "xoxp-[") {
		t.Errorf("Redacted() = %q, expected prefix 'xoxp-['", redacted)
	}
	if !strings.HasSuffix(redacted, "]") {
		t.Errorf("Redacted() = %q, expected suffix ']'", redacted)
	}

	// Extract fingerprint
	fingerprint := strings.TrimPrefix(redacted, "xoxp-[")
	fingerprint = strings.TrimSuffix(fingerprint, "]")

	if len(fingerprint) != 8 {
		t.Errorf("Redacted() fingerprint length = %d, want 8", len(fingerprint))
	}

	// Verify it's a valid hex string
	if _, err := hex.DecodeString(fingerprint); err != nil {
		t.Errorf("Redacted() fingerprint %q is not valid hex: %v", fingerprint, err)
	}

	// Verify the fingerprint is deterministic
	h := sha256.Sum256([]byte(token.Value()))
	expectedFingerprint := hex.EncodeToString(h[:4])
	if fingerprint != expectedFingerprint {
		t.Errorf("Redacted() fingerprint = %q, want %q", fingerprint, expectedFingerprint)
	}

	// Verify no part of actual token is exposed
	actualToken := token.Value()
	if strings.Contains(actualToken, fingerprint) {
		t.Error("Redacted() exposes part of actual token")
	}
}

func TestSlackToken_RedactedInvalid(t *testing.T) {
	// Test redaction of invalid/short tokens
	token := &SlackToken{token: "xoxp-123"}
	redacted := token.Redacted()
	if redacted != "xoxp-[invalid]" {
		t.Errorf("Redacted() for invalid token = %q, want 'xoxp-[invalid]'", redacted)
	}
}

func TestSlackToken_String(t *testing.T) {
	token, err := NewSlackToken("xoxp-1234567890123")
	if err != nil {
		t.Fatalf("NewSlackToken() error = %v", err)
	}

	// String() should return same as Redacted()
	if token.String() != token.Redacted() {
		t.Errorf("String() = %q, want %q", token.String(), token.Redacted())
	}
}

func TestSlackToken_Value(t *testing.T) {
	expectedToken := "xoxp-1234567890123"
	token, err := NewSlackToken(expectedToken)
	if err != nil {
		t.Fatalf("NewSlackToken() error = %v", err)
	}

	if token.Value() != expectedToken {
		t.Errorf("Value() = %q, want %q", token.Value(), expectedToken)
	}
}

func TestSlackToken_Type(t *testing.T) {
	token, err := NewSlackToken("xoxp-1234567890123")
	if err != nil {
		t.Fatalf("NewSlackToken() error = %v", err)
	}

	if token.Type() != auth.CredentialTypeToken {
		t.Errorf("Type() = %q, want %q", token.Type(), auth.CredentialTypeToken)
	}
}

func TestSlackAuthProvider_Service(t *testing.T) {
	provider := NewSlackAuthProvider(NewMockKeychain(), nil)
	if provider.Service() != auth.ServiceSlack {
		t.Errorf("Service() = %q, want %q", provider.Service(), auth.ServiceSlack)
	}
}

func TestSlackAuthProvider_GetCredential_FromKeychain(t *testing.T) {
	mock := NewMockKeychain()
	validToken := "xoxp-1234567890123"
	mock.data["slack-token"] = validToken

	provider := NewSlackAuthProvider(mock, slog.Default())
	ctx := context.Background()

	cred, err := provider.GetCredential(ctx)
	if err != nil {
		t.Fatalf("GetCredential() error = %v", err)
	}

	if cred == nil {
		t.Fatal("GetCredential() returned nil credential")
	}

	// Verify credential value
	if cred.Value() != validToken {
		t.Errorf("GetCredential() value = %q, want %q", cred.Value(), validToken)
	}

	// Verify credential type
	if cred.Type() != auth.CredentialTypeToken {
		t.Errorf("GetCredential() type = %q, want %q", cred.Type(), auth.CredentialTypeToken)
	}
}

func TestSlackAuthProvider_GetCredential_InvalidKeychainToken(t *testing.T) {
	mock := NewMockKeychain()
	mock.data["slack-token"] = "invalid-token" // no xoxp- prefix

	provider := NewSlackAuthProvider(mock, slog.Default())
	ctx := context.Background()

	_, err := provider.GetCredential(ctx)
	if err == nil {
		t.Error("GetCredential() expected error for invalid token format")
	}
	if !strings.Contains(err.Error(), "keychain token invalid") {
		t.Errorf("GetCredential() error = %v, want error containing 'keychain token invalid'", err)
	}
}

func TestSlackAuthProvider_GetCredential_KeychainError(t *testing.T) {
	mock := NewMockKeychain()
	mock.err = keychain.ErrAccessDenied

	provider := NewSlackAuthProvider(mock, slog.Default())
	ctx := context.Background()

	_, err := provider.GetCredential(ctx)
	if err == nil {
		t.Error("GetCredential() expected error when keychain fails")
	}
	if !strings.Contains(err.Error(), "keychain access failed") {
		t.Errorf("GetCredential() error = %v, want error containing 'keychain access failed'", err)
	}
}

func TestSlackAuthProvider_GetCredential_EnvVarFallback(t *testing.T) {
	// Clear env var first
	oldEnv := os.Getenv("KORA_SLACK_TOKEN")
	defer func() {
		if oldEnv != "" {
			os.Setenv("KORA_SLACK_TOKEN", oldEnv)
		} else {
			os.Unsetenv("KORA_SLACK_TOKEN")
		}
	}()

	validToken := "xoxp-1234567890123"
	os.Setenv("KORA_SLACK_TOKEN", validToken)

	// Mock keychain returns not found
	mock := NewMockKeychain()
	mock.err = keychain.ErrNotFound

	provider := NewSlackAuthProvider(mock, slog.Default())
	ctx := context.Background()

	cred, err := provider.GetCredential(ctx)
	if err != nil {
		t.Fatalf("GetCredential() error = %v", err)
	}

	if cred.Value() != validToken {
		t.Errorf("GetCredential() value = %q, want %q", cred.Value(), validToken)
	}
}

func TestSlackAuthProvider_GetCredential_EnvVarInvalid(t *testing.T) {
	oldEnv := os.Getenv("KORA_SLACK_TOKEN")
	defer func() {
		if oldEnv != "" {
			os.Setenv("KORA_SLACK_TOKEN", oldEnv)
		} else {
			os.Unsetenv("KORA_SLACK_TOKEN")
		}
	}()

	os.Setenv("KORA_SLACK_TOKEN", "invalid-token")

	mock := NewMockKeychain()
	mock.err = keychain.ErrNotFound

	provider := NewSlackAuthProvider(mock, slog.Default())
	ctx := context.Background()

	_, err := provider.GetCredential(ctx)
	if err == nil {
		t.Error("GetCredential() expected error for invalid env var token")
	}
	if !strings.Contains(err.Error(), "KORA_SLACK_TOKEN invalid") {
		t.Errorf("GetCredential() error = %v, want error containing 'KORA_SLACK_TOKEN invalid'", err)
	}
}

func TestSlackAuthProvider_GetCredential_NotFound(t *testing.T) {
	oldEnv := os.Getenv("KORA_SLACK_TOKEN")
	defer func() {
		if oldEnv != "" {
			os.Setenv("KORA_SLACK_TOKEN", oldEnv)
		} else {
			os.Unsetenv("KORA_SLACK_TOKEN")
		}
	}()
	os.Unsetenv("KORA_SLACK_TOKEN")

	mock := NewMockKeychain()
	mock.err = keychain.ErrNotFound

	provider := NewSlackAuthProvider(mock, slog.Default())
	ctx := context.Background()

	_, err := provider.GetCredential(ctx)
	if err == nil {
		t.Error("GetCredential() expected error when no credential found")
	}
	if !errors.Is(err, auth.ErrNotAuthenticated) {
		t.Errorf("GetCredential() error should wrap auth.ErrNotAuthenticated, got %v", err)
	}
}

func TestSlackAuthProvider_Authenticate(t *testing.T) {
	//nolint:govet // test struct field order prioritizes readability
	tests := []struct {
		name      string
		setupMock func(*MockKeychain)
		setupEnv  func() func()
		wantErr   bool
	}{
		{
			name: "authenticated via keychain",
			setupMock: func(m *MockKeychain) {
				m.data["slack-token"] = "xoxp-1234567890123"
			},
			setupEnv: func() func() { return func() {} },
			wantErr:  false,
		},
		{
			name: "authenticated via env var",
			setupMock: func(m *MockKeychain) {
				m.err = keychain.ErrNotFound
			},
			setupEnv: func() func() {
				old := os.Getenv("KORA_SLACK_TOKEN")
				os.Setenv("KORA_SLACK_TOKEN", "xoxp-1234567890123")
				return func() {
					if old != "" {
						os.Setenv("KORA_SLACK_TOKEN", old)
					} else {
						os.Unsetenv("KORA_SLACK_TOKEN")
					}
				}
			},
			wantErr: false,
		},
		{
			name: "not authenticated",
			setupMock: func(m *MockKeychain) {
				m.err = keychain.ErrNotFound
			},
			setupEnv: func() func() {
				old := os.Getenv("KORA_SLACK_TOKEN")
				os.Unsetenv("KORA_SLACK_TOKEN")
				return func() {
					if old != "" {
						os.Setenv("KORA_SLACK_TOKEN", old)
					}
				}
			},
			wantErr: true,
		},
		{
			name: "invalid token format",
			setupMock: func(m *MockKeychain) {
				m.data["slack-token"] = "invalid"
			},
			setupEnv: func() func() { return func() {} },
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cleanup := tt.setupEnv()
			defer cleanup()

			mock := NewMockKeychain()
			tt.setupMock(mock)

			provider := NewSlackAuthProvider(mock, slog.Default())
			err := provider.Authenticate(context.Background())

			if (err != nil) != tt.wantErr {
				t.Errorf("Authenticate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSlackAuthProvider_IsAuthenticated(t *testing.T) {
	mock := NewMockKeychain()
	mock.data["slack-token"] = "xoxp-1234567890123"

	provider := NewSlackAuthProvider(mock, slog.Default())
	if !provider.IsAuthenticated(context.Background()) {
		t.Error("IsAuthenticated() = false, want true")
	}

	// Test not authenticated
	mock2 := NewMockKeychain()
	mock2.err = keychain.ErrNotFound
	provider2 := NewSlackAuthProvider(mock2, slog.Default())
	if provider2.IsAuthenticated(context.Background()) {
		t.Error("IsAuthenticated() = true, want false")
	}
}
