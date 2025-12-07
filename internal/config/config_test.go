package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	// Datasources defaults
	if !cfg.Datasources.GitHub.Enabled {
		t.Error("GitHub should be enabled by default")
	}

	// Digest defaults
	if cfg.Digest.Window != 16*time.Hour {
		t.Errorf("Window should default to 16h, got %v", cfg.Digest.Window)
	}
	if cfg.Digest.Timezone != "Local" {
		t.Errorf("Timezone should default to Local, got %s", cfg.Digest.Timezone)
	}
	if cfg.Digest.Format != "text" {
		t.Errorf("Format should default to text, got %s", cfg.Digest.Format)
	}

	// Security defaults
	if !cfg.Security.RedactCredentials {
		t.Error("RedactCredentials should be true by default")
	}
	if cfg.Security.DatasourceTimeout != 30*time.Second {
		t.Errorf("DatasourceTimeout should default to 30s, got %v", cfg.Security.DatasourceTimeout)
	}
	// Note: TLS verification is always enabled (not configurable per EFA 0002)
}

func TestDefaultConfig_Validates(t *testing.T) {
	cfg := DefaultConfig()
	if err := cfg.Validate(); err != nil {
		t.Errorf("Default config should be valid: %v", err)
	}
}

func TestLoad_NonExistentFile(t *testing.T) {
	cfg, err := Load("/nonexistent/path/config.yaml")
	if err != nil {
		t.Errorf("Load should not fail for nonexistent file: %v", err)
	}
	if cfg == nil {
		t.Fatal("Expected default config, got nil")
	}
	// Should return defaults
	if cfg.Digest.Format != "text" {
		t.Errorf("Expected default format 'text', got %s", cfg.Digest.Format)
	}
}

func TestLoad_ValidYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	yaml := `
datasources:
  github:
    enabled: true
    orgs:
      - test-org
digest:
  window: 8h
  timezone: America/New_York
  format: json
security:
  datasource_timeout: 60s
`
	if err := os.WriteFile(path, []byte(yaml), 0o600); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	// Verify parsed values
	if !cfg.Datasources.GitHub.Enabled {
		t.Error("GitHub should be enabled")
	}
	if len(cfg.Datasources.GitHub.Orgs) != 1 || cfg.Datasources.GitHub.Orgs[0] != "test-org" {
		t.Errorf("Unexpected orgs: %v", cfg.Datasources.GitHub.Orgs)
	}
	if cfg.Digest.Window != 8*time.Hour {
		t.Errorf("Expected 8h window, got %v", cfg.Digest.Window)
	}
	if cfg.Digest.Timezone != "America/New_York" {
		t.Errorf("Expected America/New_York timezone, got %s", cfg.Digest.Timezone)
	}
	if cfg.Digest.Format != "json" {
		t.Errorf("Expected json format, got %s", cfg.Digest.Format)
	}
	if cfg.Security.DatasourceTimeout != 60*time.Second {
		t.Errorf("Expected 60s timeout, got %v", cfg.Security.DatasourceTimeout)
	}
}

func TestLoad_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	if err := os.WriteFile(path, []byte("not: valid: yaml: ["), 0o600); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	_, err := Load(path)
	if err == nil {
		t.Error("Expected error for invalid YAML")
	}
}

func TestLoad_ValidationError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	// GitHub disabled - should fail validation (no datasources enabled)
	yaml := `
datasources:
  github:
    enabled: false
`
	if err := os.WriteFile(path, []byte(yaml), 0o600); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	_, err := Load(path)
	if err == nil {
		t.Error("Expected validation error when no datasources enabled")
	}
}

func TestLoad_Directory(t *testing.T) {
	dir := t.TempDir()
	_, err := Load(dir)
	if err == nil {
		t.Error("Expected error when loading a directory")
	}
}

func TestDefaultConfigPath(t *testing.T) {
	path := DefaultConfigPath()
	if path == "" {
		t.Skip("Could not determine home directory")
	}
	if !filepath.IsAbs(path) {
		t.Errorf("Expected absolute path, got %s", path)
	}
	if filepath.Base(path) != "config.yaml" {
		t.Errorf("Expected config.yaml filename, got %s", filepath.Base(path))
	}
}
