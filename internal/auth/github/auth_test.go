package github

import (
	"context"
	"os/exec"
	"strings"
	"testing"

	"github.com/dakaneye/kora/internal/auth"
)

func TestNewGitHubDelegatedCredential(t *testing.T) {
	tests := []struct {
		name     string
		username string
		ghPath   string
		wantErr  bool
	}{
		{
			name:     "valid credential with username",
			username: "testuser",
			ghPath:   "gh",
			wantErr:  false,
		},
		{
			name:     "valid credential with custom gh path",
			username: "testuser",
			ghPath:   "/usr/local/bin/gh",
			wantErr:  false,
		},
		{
			name:     "empty username should error",
			username: "",
			ghPath:   "gh",
			wantErr:  true,
		},
		{
			name:     "empty gh path defaults to gh",
			username: "testuser",
			ghPath:   "",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cred, err := NewGitHubDelegatedCredential(tt.username, tt.ghPath)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewGitHubDelegatedCredential() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if cred == nil {
					t.Error("NewGitHubDelegatedCredential() returned nil credential")
					return
				}
				if cred.Username() != tt.username {
					t.Errorf("Username() = %q, want %q", cred.Username(), tt.username)
				}
				expectedPath := tt.ghPath
				if expectedPath == "" {
					expectedPath = "gh"
				}
				if cred.ghPath != expectedPath {
					t.Errorf("ghPath = %q, want %q", cred.ghPath, expectedPath)
				}
			}
		})
	}
}

func TestGitHubDelegatedCredential_Value(t *testing.T) {
	cred, err := NewGitHubDelegatedCredential("testuser", "gh")
	if err != nil {
		t.Fatalf("NewGitHubDelegatedCredential() error = %v", err)
	}

	// Value() should always return empty string for delegated credentials
	if cred.Value() != "" {
		t.Errorf("Value() = %q, want empty string", cred.Value())
	}
}

func TestGitHubDelegatedCredential_Redacted(t *testing.T) {
	tests := []struct {
		name     string
		username string
		want     string
	}{
		{
			name:     "standard username",
			username: "testuser",
			want:     "github:testuser",
		},
		{
			name:     "username with hyphens",
			username: "test-user",
			want:     "github:test-user",
		},
		{
			name:     "username with numbers",
			username: "user123",
			want:     "github:user123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cred, err := NewGitHubDelegatedCredential(tt.username, "gh")
			if err != nil {
				t.Fatalf("NewGitHubDelegatedCredential() error = %v", err)
			}
			if got := cred.Redacted(); got != tt.want {
				t.Errorf("Redacted() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestGitHubDelegatedCredential_String(t *testing.T) {
	cred, err := NewGitHubDelegatedCredential("testuser", "gh")
	if err != nil {
		t.Fatalf("NewGitHubDelegatedCredential() error = %v", err)
	}

	// String() should return same as Redacted()
	if cred.String() != cred.Redacted() {
		t.Errorf("String() = %q, want %q", cred.String(), cred.Redacted())
	}
}

func TestGitHubDelegatedCredential_Type(t *testing.T) {
	cred, err := NewGitHubDelegatedCredential("testuser", "gh")
	if err != nil {
		t.Fatalf("NewGitHubDelegatedCredential() error = %v", err)
	}

	if cred.Type() != auth.CredentialTypeOAuth {
		t.Errorf("Type() = %q, want %q", cred.Type(), auth.CredentialTypeOAuth)
	}
}

func TestGitHubDelegatedCredential_IsValid(t *testing.T) {
	tests := []struct {
		name     string
		username string
		valid    bool
	}{
		{
			name:     "valid with username",
			username: "testuser",
			valid:    true,
		},
		{
			name:     "invalid with empty username via struct",
			username: "",
			valid:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Bypass constructor validation for empty username test
			cred := &GitHubDelegatedCredential{
				username: tt.username,
				ghPath:   "gh",
			}
			if got := cred.IsValid(); got != tt.valid {
				t.Errorf("IsValid() = %v, want %v", got, tt.valid)
			}
		})
	}
}

func TestGitHubAuthProvider_Service(t *testing.T) {
	provider := NewGitHubAuthProvider("")
	if provider.Service() != auth.ServiceGitHub {
		t.Errorf("Service() = %q, want %q", provider.Service(), auth.ServiceGitHub)
	}
}

func TestGitHubAuthProvider_NewWithEmptyPath(t *testing.T) {
	provider := NewGitHubAuthProvider("")
	if provider.ghPath != "gh" {
		t.Errorf("NewGitHubAuthProvider(\"\") ghPath = %q, want %q", provider.ghPath, "gh")
	}
}

func TestGitHubAuthProvider_NewWithCustomPath(t *testing.T) {
	customPath := "/usr/local/bin/gh"
	provider := NewGitHubAuthProvider(customPath)
	if provider.ghPath != customPath {
		t.Errorf("NewGitHubAuthProvider(%q) ghPath = %q, want %q", customPath, provider.ghPath, customPath)
	}
}

func TestGitHubAuthProvider_Authenticate_GHNotFound(t *testing.T) {
	// Use a non-existent path to simulate gh not found
	provider := NewGitHubAuthProvider("/nonexistent/gh")
	err := provider.Authenticate(context.Background())
	if err == nil {
		t.Error("Authenticate() expected error when gh not found")
		return
	}
	if !strings.Contains(err.Error(), auth.ErrGHCLINotFound.Error()) {
		t.Errorf("Authenticate() error = %v, want error containing %q", err, auth.ErrGHCLINotFound.Error())
	}
}

func TestGitHubAuthProvider_IsAuthenticated(t *testing.T) {
	// Test with non-existent gh path - should return false
	provider := NewGitHubAuthProvider("/nonexistent/gh")
	if provider.IsAuthenticated(context.Background()) {
		t.Error("IsAuthenticated() = true, want false when gh not found")
	}
}

// TestGitHubAuthProvider_Authenticate_NotAuthenticated tests the case where
// gh CLI is installed but not authenticated. This is tricky to test without
// mocking exec.Command or having gh installed, so we check error type.
func TestGitHubAuthProvider_Authenticate_NotAuthenticated(t *testing.T) {
	// Skip if gh is not installed
	if _, err := exec.LookPath("gh"); err != nil {
		t.Skip("gh CLI not installed, skipping test")
	}

	// We can't reliably test this without mocking, as we don't want to
	// mess with the user's gh auth state. This test documents the expected
	// behavior but skips actual execution.
	t.Skip("Cannot reliably test gh auth status without mocking")
}

// TestGitHubDelegatedCredential_ExecuteAPI tests API execution.
// This requires gh to be installed and authenticated, so we'll test
// the error case only in unit tests.
func TestGitHubDelegatedCredential_ExecuteAPI_InvalidCommand(t *testing.T) {
	// Use a non-existent gh path to test error handling
	cred := &GitHubDelegatedCredential{
		username: "testuser",
		ghPath:   "/nonexistent/gh",
	}

	_, err := cred.ExecuteAPI(context.Background(), "user")
	if err == nil {
		t.Error("ExecuteAPI() expected error with non-existent gh")
	}
}

// TestGitHubDelegatedCredential_Username tests the Username accessor.
func TestGitHubDelegatedCredential_Username(t *testing.T) {
	expectedUsername := "octocat"
	cred, err := NewGitHubDelegatedCredential(expectedUsername, "gh")
	if err != nil {
		t.Fatalf("NewGitHubDelegatedCredential() error = %v", err)
	}

	if got := cred.Username(); got != expectedUsername {
		t.Errorf("Username() = %q, want %q", got, expectedUsername)
	}
}

// TestGitHubAuthProvider_runGH tests the internal runGH method.
func TestGitHubAuthProvider_runGH_Error(t *testing.T) {
	provider := NewGitHubAuthProvider("/nonexistent/gh")
	_, err := provider.runGH(context.Background(), "version")
	if err == nil {
		t.Error("runGH() expected error with non-existent gh")
	}
}

// TestTimeouts verifies timeout constants are set.
func TestTimeouts(t *testing.T) {
	if authTimeout.Seconds() <= 0 {
		t.Errorf("authTimeout = %v, want positive duration", authTimeout)
	}
	if apiTimeout.Seconds() <= 0 {
		t.Errorf("apiTimeout = %v, want positive duration", apiTimeout)
	}
	if apiTimeout <= authTimeout {
		t.Errorf("apiTimeout (%v) should be greater than authTimeout (%v)", apiTimeout, authTimeout)
	}
}
