//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestDigestCommand_GitHubOnly tests the digest command with only GitHub enabled.
func TestDigestCommand_GitHubOnly(t *testing.T) {
	requireGitHubAuth(t)

	// Create temporary config file with only GitHub enabled
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "kora.yaml")

	configContent := `
datasources:
  github:
    enabled: true
  slack:
    enabled: false

digest:
  window: 24h
  format: json

security:
  datasource_timeout: 30s
`

	if err := os.WriteFile(configPath, []byte(configContent), 0600); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	// Build kora binary for testing
	koraPath := filepath.Join(tmpDir, "kora")
	buildCmd := exec.Command("go", "build", "-o", koraPath, "../../cmd/kora")
	if output, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to build kora: %v\n%s", err, output)
	}

	t.Run("JSON output", func(t *testing.T) {
		cmd := exec.Command(koraPath, "digest", "--config", configPath, "--format", "json", "--since", "24h")
		output, err := cmd.CombinedOutput()

		// Command should succeed (exit 0) or partial success (exit 1)
		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				// Exit code 1 is acceptable (partial failure)
				if exitErr.ExitCode() > 1 {
					t.Fatalf("digest command failed with exit code %d: %s", exitErr.ExitCode(), output)
				}
			} else {
				t.Fatalf("digest command error: %v", err)
			}
		}

		// Verify output is valid JSON
		var result map[string]interface{}
		if err := json.Unmarshal(output, &result); err != nil {
			t.Fatalf("Output is not valid JSON: %v\nOutput: %s", err, output)
		}

		t.Logf("JSON output structure: %v", result)

		// Verify expected fields exist
		if _, ok := result["events"]; !ok {
			t.Error("JSON output missing 'events' field")
		}

		// Check events array
		if events, ok := result["events"].([]interface{}); ok {
			t.Logf("Fetched %d events", len(events))

			// Validate each event structure if we got any
			for i, evt := range events {
				eventMap, ok := evt.(map[string]interface{})
				if !ok {
					t.Errorf("Event[%d] is not a map", i)
					continue
				}

				// Check required fields
				requiredFields := []string{"type", "title", "source", "url", "author", "timestamp", "priority"}
				for _, field := range requiredFields {
					if _, exists := eventMap[field]; !exists {
						t.Errorf("Event[%d] missing required field: %s", i, field)
					}
				}

				// Verify source is github
				if source, ok := eventMap["source"].(string); ok {
					if source != "github" {
						t.Errorf("Event[%d] source = %q, want 'github'", i, source)
					}
				}
			}
		}
	})

	t.Run("Text output", func(t *testing.T) {
		cmd := exec.Command(koraPath, "digest", "--config", configPath, "--format", "text", "--since", "24h")
		output, err := cmd.CombinedOutput()

		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				if exitErr.ExitCode() > 1 {
					t.Fatalf("digest command failed: %s", output)
				}
			} else {
				t.Fatalf("digest command error: %v", err)
			}
		}

		// Verify output is non-empty
		if len(output) == 0 {
			t.Error("Text output is empty")
		}

		t.Logf("Text output length: %d bytes", len(output))
	})

	t.Run("Short window", func(t *testing.T) {
		// 1 hour window - less likely to have events
		cmd := exec.Command(koraPath, "digest", "--config", configPath, "--format", "json", "--since", "1h")
		output, err := cmd.CombinedOutput()

		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				if exitErr.ExitCode() > 1 {
					t.Fatalf("digest command failed: %s", output)
				}
			}
		}

		var result map[string]interface{}
		if err := json.Unmarshal(output, &result); err != nil {
			t.Fatalf("Output is not valid JSON: %v", err)
		}

		if events, ok := result["events"].([]interface{}); ok {
			t.Logf("1-hour window: fetched %d events", len(events))
		}
	})

	t.Run("Invalid since parameter", func(t *testing.T) {
		cmd := exec.Command(koraPath, "digest", "--config", configPath, "--since", "invalid")
		output, err := cmd.CombinedOutput()

		// Should fail with non-zero exit code
		if err == nil {
			t.Error("Expected error for invalid --since parameter")
		}

		// Error message should be in output
		if !strings.Contains(string(output), "invalid") && !strings.Contains(string(output), "since") {
			t.Logf("Error output: %s", output)
		}
	})
}

// TestDigestCommand_Timeout tests datasource timeout handling.
func TestDigestCommand_Timeout(t *testing.T) {
	requireGitHubAuth(t)

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "kora.yaml")

	// Set very short timeout to potentially trigger timeout
	configContent := `
datasources:
  github:
    enabled: true
  slack:
    enabled: false

digest:
  window: 24h
  format: json

security:
  datasource_timeout: 1ns  # Extremely short timeout
`

	if err := os.WriteFile(configPath, []byte(configContent), 0600); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	koraPath := filepath.Join(tmpDir, "kora")
	buildCmd := exec.Command("go", "build", "-o", koraPath, "../../cmd/kora")
	if output, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to build kora: %v\n%s", err, output)
	}

	cmd := exec.Command(koraPath, "digest", "--config", configPath, "--format", "json")
	output, err := cmd.CombinedOutput()

	// Should either fail or return partial results
	if err != nil {
		t.Logf("Expected timeout error occurred: %v", err)
	} else {
		// If it succeeded, check if we got any events
		var result map[string]interface{}
		if json.Unmarshal(output, &result) == nil {
			if events, ok := result["events"].([]interface{}); ok {
				t.Logf("Timeout config but got %d events (timing-dependent)", len(events))
			}
		}
	}
}

// TestDigestCommand_OutputFormats tests different output format options.
func TestDigestCommand_OutputFormats(t *testing.T) {
	requireGitHubAuth(t)

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "kora.yaml")

	configContent := `
datasources:
  github:
    enabled: true
  slack:
    enabled: false

digest:
  window: 24h
  format: text

security:
  datasource_timeout: 30s
`

	if err := os.WriteFile(configPath, []byte(configContent), 0600); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	koraPath := filepath.Join(tmpDir, "kora")
	buildCmd := exec.Command("go", "build", "-o", koraPath, "../../cmd/kora")
	if output, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to build kora: %v\n%s", err, output)
	}

	formats := []string{"json", "json-pretty", "text"}

	for _, format := range formats {
		t.Run("Format_"+format, func(t *testing.T) {
			cmd := exec.Command(koraPath, "digest", "--config", configPath, "--format", format, "--since", "24h")
			output, err := cmd.CombinedOutput()

			if err != nil {
				if exitErr, ok := err.(*exec.ExitError); ok {
					if exitErr.ExitCode() > 1 {
						t.Fatalf("digest failed for format %s: %s", format, output)
					}
				}
			}

			// Verify output is non-empty
			if len(output) == 0 {
				t.Errorf("Output is empty for format: %s", format)
			}

			// For JSON formats, verify it's valid JSON
			if strings.HasPrefix(format, "json") {
				var result interface{}
				if err := json.Unmarshal(output, &result); err != nil {
					t.Errorf("Format %s output is not valid JSON: %v", format, err)
				}
			}

			t.Logf("Format %s: %d bytes output", format, len(output))
		})
	}
}

// TestDigestCommand_VersionFlag tests the version command.
func TestDigestCommand_VersionFlag(t *testing.T) {
	tmpDir := t.TempDir()
	koraPath := filepath.Join(tmpDir, "kora")

	buildCmd := exec.Command("go", "build", "-o", koraPath, "../../cmd/kora")
	if output, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to build kora: %v\n%s", err, output)
	}

	cmd := exec.Command(koraPath, "version")
	output, err := cmd.CombinedOutput()

	if err != nil {
		t.Fatalf("version command failed: %v\n%s", err, output)
	}

	// Should contain version information
	outputStr := string(output)
	if !strings.Contains(outputStr, "version") && !strings.Contains(outputStr, "kora") {
		t.Errorf("Version output doesn't look right: %s", outputStr)
	}

	t.Logf("Version output: %s", outputStr)
}

// TestDigestCommand_LiveExecution tests actual CLI execution in real-time.
// This is the most complete end-to-end test but depends on GitHub having events.
func TestDigestCommand_LiveExecution(t *testing.T) {
	requireGitHubAuth(t)

	// Use default config location if it exists, otherwise skip
	homeDir, err := os.UserHomeDir()
	if err != nil {
		t.Skip("Cannot determine home directory")
	}

	configPath := filepath.Join(homeDir, ".config", "kora", "config.yaml")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Skip("No kora config file found at " + configPath)
	}

	tmpDir := t.TempDir()
	koraPath := filepath.Join(tmpDir, "kora")

	buildCmd := exec.Command("go", "build", "-o", koraPath, "../../cmd/kora")
	if output, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to build kora: %v\n%s", err, output)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, koraPath, "digest", "--format", "json", "--since", "24h")
	output, err := cmd.CombinedOutput()

	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			t.Fatal("digest command timed out after 60s")
		}
		// Partial failure is acceptable
		if exitErr, ok := err.(*exec.ExitError); ok {
			if exitErr.ExitCode() > 1 {
				t.Fatalf("digest failed: %s", output)
			}
			t.Logf("Partial success (exit code %d)", exitErr.ExitCode())
		}
	}

	t.Logf("Live execution completed: %d bytes output", len(output))

	// Parse and validate output
	var result map[string]interface{}
	if err := json.Unmarshal(output, &result); err != nil {
		t.Fatalf("Output is not valid JSON: %v", err)
	}

	if events, ok := result["events"].([]interface{}); ok {
		t.Logf("Live execution fetched %d events", len(events))
		if len(events) > 0 {
			t.Log("✓ Successfully fetched real GitHub events")
		}
	}
}
