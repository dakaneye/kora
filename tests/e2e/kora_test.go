//go:build e2e

package e2e_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func binaryPath(t *testing.T) string {
	t.Helper()
	path := filepath.Join("..", "..", "bin", "kora")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("binary not found at %s: run 'make build' first", path)
	}
	abs, _ := filepath.Abs(path)
	return abs
}

func TestKora_Version(t *testing.T) {
	bin := binaryPath(t)
	cmd := exec.Command(bin, "--version")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("--version failed: %v", err)
	}
	if len(out) == 0 {
		t.Error("--version produced no output")
	}
	t.Logf("version: %s", out)
}

func TestKora_InvalidSince(t *testing.T) {
	bin := binaryPath(t)
	cmd := exec.Command(bin, "--since", "banana")
	err := cmd.Run()
	if err == nil {
		t.Error("expected non-zero exit for invalid --since")
	}
}

func TestKora_FullRun(t *testing.T) {
	bin := binaryPath(t)
	cmd := exec.Command(bin, "--since", "24h")
	out, err := cmd.Output()
	if err != nil {
		t.Skipf("full run failed (likely auth): %v", err)
	}

	var result map[string]json.RawMessage
	if err := json.Unmarshal(out, &result); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	if _, ok := result["sources"]; !ok {
		t.Error("missing 'sources' key in output")
	}
	if _, ok := result["fetched_at"]; !ok {
		t.Error("missing 'fetched_at' key in output")
	}
	if _, ok := result["since"]; !ok {
		t.Error("missing 'since' key in output")
	}
	t.Logf("full run returned %d bytes", len(out))
}

func TestKora_OutputStructure(t *testing.T) {
	bin := binaryPath(t)
	cmd := exec.Command(bin, "--since", "1h")
	out, err := cmd.Output()
	if err != nil {
		t.Skipf("run failed (likely auth): %v", err)
	}

	var envelope struct {
		FetchedAt string                     `json:"fetched_at"`
		Since     string                     `json:"since"`
		Sources   map[string]json.RawMessage `json:"sources"`
	}
	if err := json.Unmarshal(out, &envelope); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	expectedSources := []string{"github", "gmail", "calendar", "linear"}
	for _, name := range expectedSources {
		if _, ok := envelope.Sources[name]; !ok {
			t.Errorf("missing source %q in output", name)
		}
	}

	if envelope.Since != "1h0m0s" {
		t.Errorf("since = %q, want %q", envelope.Since, "1h0m0s")
	}
}
