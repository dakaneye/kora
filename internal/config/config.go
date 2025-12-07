// Package config handles configuration loading and validation for Kora.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

// Config is the root configuration structure for Kora.
//
//nolint:govet // Field order prioritizes semantic grouping over memory alignment
type Config struct {
	Datasources DatasourcesConfig `yaml:"datasources"`
	Digest      DigestConfig      `yaml:"digest"`
	Security    SecurityConfig    `yaml:"security"`
}

// DatasourcesConfig configures which datasources are enabled.
type DatasourcesConfig struct {
	GitHub GitHubConfig `yaml:"github"`
}

// GitHubConfig configures the GitHub datasource.
//
//nolint:govet // Field order prioritizes semantic grouping over memory alignment
type GitHubConfig struct {
	// Enabled controls whether GitHub datasource is active.
	Enabled bool `yaml:"enabled"`
	// Orgs limits searches to specific organizations (empty = all).
	Orgs []string `yaml:"orgs,omitempty"`
}

// DigestConfig configures digest generation behavior.
//
//nolint:govet // Field order prioritizes semantic grouping over memory alignment
type DigestConfig struct {
	// Window is the default time window for fetching events.
	// Default: 16h
	Window time.Duration `yaml:"window"`
	// Timezone for display. Use IANA zone names (e.g., "America/New_York").
	// Default: "Local"
	Timezone string `yaml:"timezone"`
	// Format is the default output format: "json", "json-pretty", "text".
	// Default: "text"
	Format string `yaml:"format"`
}

// SecurityConfig configures security-related settings.
//
//nolint:govet // Field order prioritizes semantic grouping over memory alignment
type SecurityConfig struct {
	// RedactCredentials controls whether credentials are redacted in logs.
	// Default: true
	RedactCredentials bool `yaml:"redact_credentials"`
	// DatasourceTimeout is the per-datasource timeout.
	// Default: 30s
	DatasourceTimeout time.Duration `yaml:"datasource_timeout"`
	// Note: TLS verification is always enabled and cannot be disabled.
	// This is a security requirement per EFA 0002.
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() *Config {
	return &Config{
		Datasources: DatasourcesConfig{
			GitHub: GitHubConfig{
				Enabled: true,
			},
		},
		Digest: DigestConfig{
			Window:   16 * time.Hour,
			Timezone: "Local",
			Format:   "text",
		},
		Security: SecurityConfig{
			RedactCredentials: true,
			DatasourceTimeout: 30 * time.Second,
		},
	}
}

// DefaultConfigPath returns the default configuration file path.
// On macOS: ~/.kora/config.yaml
func DefaultConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".kora", "config.yaml")
}

// Load reads configuration from a YAML file and applies defaults.
// If path is empty, uses DefaultConfigPath().
// If the file doesn't exist, returns DefaultConfig() with no error.
func Load(path string) (*Config, error) {
	if path == "" {
		path = DefaultConfigPath()
	}

	// Start with defaults
	cfg := DefaultConfig()

	// Check if file exists
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		// No config file, use defaults
		return cfg, nil
	}
	if err != nil {
		return nil, fmt.Errorf("config: stat %s: %w", path, err)
	}
	if info.IsDir() {
		return nil, fmt.Errorf("config: %s is a directory", path)
	}

	// Read and parse file
	// #nosec G304 -- path is from config flag or default path, not arbitrary user input
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("config: read %s: %w", path, err)
	}

	// Parse YAML on top of defaults
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("config: parse %s: %w", path, err)
	}

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("config: validate: %w", err)
	}

	return cfg, nil
}

// MustLoad loads configuration or panics on error.
// Use only in initialization where config errors should be fatal.
func MustLoad(path string) *Config {
	cfg, err := Load(path)
	if err != nil {
		panic(err)
	}
	return cfg
}
