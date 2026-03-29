// Package config loads optional YAML configuration from ~/.kora.yaml.
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config holds optional filters for data sources. Zero values mean no filtering.
type Config struct {
	GitHub GitHubConfig `yaml:"github"`
	Linear LinearConfig `yaml:"linear"`
}

// GitHubConfig filters GitHub queries by org and/or repo.
type GitHubConfig struct {
	Orgs  []string `yaml:"orgs"`
	Repos []string `yaml:"repos"`
}

// LinearConfig filters Linear queries by team key.
type LinearConfig struct {
	Teams []string `yaml:"teams"`
}

// Load reads ~/.kora.yaml if it exists. Returns an empty Config (no filters) if
// the file doesn't exist. Returns an error only if the file exists but is invalid.
func Load() (Config, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return Config{}, nil
	}

	path := filepath.Join(home, ".kora.yaml")
	data, err := os.ReadFile(path) //nolint:gosec // G304: path is always ~/.kora.yaml, not user-controlled
	if err != nil {
		if os.IsNotExist(err) {
			return Config{}, nil
		}
		return Config{}, fmt.Errorf("read config: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse %s: %w", path, err)
	}
	return cfg, nil
}
