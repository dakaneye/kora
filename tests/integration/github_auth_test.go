//go:build integration

package integration

import (
	"context"
	"os/exec"
	"testing"

	"github.com/dakaneye/kora/internal/auth/github"
)

// TestGitHubAuthProvider_Integration tests GitHub authentication with real gh CLI.
// This test requires:
//   - gh CLI to be installed
//   - User to be authenticated via `gh auth login`
func TestGitHubAuthProvider_Integration(t *testing.T) {
	// Check if gh CLI is installed
	if _, err := exec.LookPath("gh"); err != nil {
		t.Skip("gh CLI not installed, skipping integration test")
	}

	// Check if gh is authenticated
	cmd := exec.Command("gh", "auth", "status")
	if err := cmd.Run(); err != nil {
		t.Skip("gh CLI not authenticated, run 'gh auth login' first")
	}

	provider := github.NewGitHubAuthProvider("")
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

		// Verify Value() returns empty string for delegated creds
		if cred.Value() != "" {
			t.Errorf("credential.Value() = %q, want empty string", cred.Value())
		}

		// Verify Redacted() has expected format
		redacted := cred.Redacted()
		if len(redacted) < 7 || redacted[:7] != "github:" {
			t.Errorf("credential.Redacted() = %q, want format 'github:username'", redacted)
		}
	})

	t.Run("ExecuteAPI", func(t *testing.T) {
		cred, err := provider.GetCredential(ctx)
		if err != nil {
			t.Fatalf("GetCredential() error = %v", err)
		}

		// Cast to GitHubDelegatedCredential to access ExecuteAPI
		ghCred, ok := cred.(*github.GitHubDelegatedCredential)
		if !ok {
			t.Fatal("credential is not *GitHubDelegatedCredential")
		}

		// Test API call - get current user
		output, err := ghCred.ExecuteAPI(ctx, "user", "--jq", ".login")
		if err != nil {
			t.Fatalf("ExecuteAPI() error = %v", err)
		}

		if len(output) == 0 {
			t.Error("ExecuteAPI() returned empty output")
		}

		// Verify username matches
		username := ghCred.Username()
		if username == "" {
			t.Error("Username() returned empty string")
		}
	})
}

// TestGitHubAuthProvider_NotAuthenticated tests behavior when gh is not authenticated.
func TestGitHubAuthProvider_NotAuthenticated(t *testing.T) {
	// Check if gh CLI is installed
	if _, err := exec.LookPath("gh"); err != nil {
		t.Skip("gh CLI not installed, skipping integration test")
	}

	// Check if gh is authenticated - if so, skip (we can't test unauthenticated state)
	cmd := exec.Command("gh", "auth", "status")
	if err := cmd.Run(); err == nil {
		t.Skip("gh CLI is authenticated, cannot test unauthenticated state without disrupting user auth")
	}

	provider := github.NewGitHubAuthProvider("")
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
