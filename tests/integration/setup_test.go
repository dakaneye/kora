//go:build integration

package integration

import (
	"context"
	"os"
	"os/exec"
	"runtime"
	"testing"

	"github.com/dakaneye/kora/internal/auth/keychain"
	"github.com/dakaneye/kora/internal/config"
	"github.com/dakaneye/kora/internal/datasources"
)

// Note: Integration tests are gated by the //go:build integration tag.
// Run with: go test -tags=integration ./tests/integration/...

// requireGitHubAuth skips the test if GitHub authentication is not available.
// It checks for gh CLI installation and authentication status.
func requireGitHubAuth(t *testing.T) {
	t.Helper()

	// Check if gh CLI is installed
	ghPath, err := exec.LookPath("gh")
	if err != nil {
		t.Skip("gh CLI not installed - skipping GitHub integration test")
	}

	// Check if gh is authenticated
	cmd := exec.Command(ghPath, "auth", "status")
	if err := cmd.Run(); err != nil {
		t.Skip("gh CLI not authenticated - run 'gh auth login' first")
	}
}

// requireSlackAuth skips the test if Slack authentication is not available.
// It checks macOS platform and verifies slack-token exists in keychain.
func requireSlackAuth(t *testing.T) {
	t.Helper()

	// Only macOS supports keychain
	if runtime.GOOS != "darwin" {
		t.Skip("Slack authentication requires macOS keychain")
	}

	// Skip in CI environments
	if os.Getenv("CI") != "" {
		t.Skip("Skipping Slack integration test in CI environment")
	}

	// Check if slack-token exists in keychain
	kc := keychain.NewMacOSKeychain("")
	if !kc.Exists(context.Background(), "slack-token") {
		t.Skip("slack-token not found in keychain - configure Slack authentication first")
	}
}

// testConfig creates a minimal test configuration for integration tests.
// It allows enabling/disabling specific datasources for testing scenarios.
func testConfig(enableGitHub bool) *config.Config {
	return &config.Config{
		Datasources: config.DatasourcesConfig{
			GitHub: config.GitHubConfig{
				Enabled: enableGitHub,
				Orgs:    nil, // no org filter for integration tests
			},
		},
		Digest: config.DigestConfig{
			Window: 24 * 3600 * 1000000000, // 24h in nanoseconds
			Format: "json",
		},
		Security: config.SecurityConfig{
			DatasourceTimeout: 30 * 1000000000, // 30s in nanoseconds
		},
	}
}

// defaultFetchOptions returns FetchOptions with sensible defaults for integration tests.
// Uses 24 hours lookback to ensure we get some events in most cases.
func defaultFetchOptions() datasources.FetchOptions {
	return datasources.FetchOptions{
		Since: testSince24h(),
		Limit: 0, // no limit
	}
}

// requireGoogleAuth skips the test if Google authentication is not available.
// It checks macOS platform, CI environment, and verifies google-oauth-{email} exists in keychain.
// Returns the email address to use for testing from GOOGLE_TEST_EMAIL environment variable.
func requireGoogleAuth(t *testing.T) string {
	t.Helper()

	skipNonMacOS(t)
	skipInCI(t)

	// Get test email from environment
	email := os.Getenv("GOOGLE_TEST_EMAIL")
	if email == "" {
		t.Skip("GOOGLE_TEST_EMAIL not set - configure Google authentication first")
	}

	// Check if google-oauth-{email} exists in keychain
	kc := keychain.NewMacOSKeychain("")
	keychainKey := "google-oauth-" + email
	if !kc.Exists(context.Background(), keychainKey) {
		t.Skipf("google-oauth-%s not found in keychain - configure Google authentication first", email)
	}

	return email
}

// skipInCI skips the test if running in a CI environment.
func skipInCI(t *testing.T) {
	t.Helper()
	if os.Getenv("CI") != "" {
		t.Skip("Skipping integration test in CI environment")
	}
}

// skipNonMacOS skips the test if not running on macOS.
func skipNonMacOS(t *testing.T) {
	t.Helper()
	if runtime.GOOS != "darwin" {
		t.Skip("Test requires macOS keychain")
	}
}
