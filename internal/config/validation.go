package config

import (
	"errors"
	"fmt"
	"regexp"
	"time"
)

// validOrgPattern matches valid GitHub organization/user names.
// GitHub usernames: alphanumeric, hyphens, 1-39 chars, no leading/trailing hyphens.
var validOrgPattern = regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9-]{0,37}[a-zA-Z0-9])?$`)

// validFormats is the set of allowed output formats.
var validFormats = map[string]struct{}{
	"json":        {},
	"json-pretty": {},
	"text":        {},
}

// Validate checks that the configuration is valid.
// Returns an error describing all validation failures.
func (c *Config) Validate() error {
	var errs []error

	// Validate datasources
	if err := c.Datasources.Validate(); err != nil {
		errs = append(errs, err)
	}

	// Validate digest
	if err := c.Digest.Validate(); err != nil {
		errs = append(errs, err)
	}

	// Validate security
	if err := c.Security.Validate(); err != nil {
		errs = append(errs, err)
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}

// Validate checks that datasources configuration is valid.
func (c *DatasourcesConfig) Validate() error {
	var errs []error

	// At least one datasource must be enabled
	if !c.GitHub.Enabled {
		errs = append(errs, errors.New("at least one datasource must be enabled"))
	}

	// Validate GitHub org names to prevent search query injection
	for _, org := range c.GitHub.Orgs {
		if !validOrgPattern.MatchString(org) {
			errs = append(errs, fmt.Errorf("invalid github org name: %q", org))
		}
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}

// Validate checks that digest configuration is valid.
func (c *DigestConfig) Validate() error {
	var errs []error

	// Window must be positive (0 is invalid, as we need some time range)
	if c.Window <= 0 {
		errs = append(errs, errors.New("digest.window must be positive"))
	}

	// Format must be one of the valid formats
	if _, ok := validFormats[c.Format]; !ok {
		errs = append(errs, fmt.Errorf("digest.format must be one of: json, json-pretty, text; got %q", c.Format))
	}

	// Timezone must be valid if specified (non-empty and not "Local")
	if c.Timezone != "" && c.Timezone != "Local" {
		if _, err := time.LoadLocation(c.Timezone); err != nil {
			errs = append(errs, fmt.Errorf("digest.timezone %q is invalid: %w", c.Timezone, err))
		}
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}

// Validate checks that security configuration is valid.
func (c *SecurityConfig) Validate() error {
	var errs []error

	// DatasourceTimeout must be positive
	if c.DatasourceTimeout <= 0 {
		errs = append(errs, errors.New("security.datasource_timeout must be positive"))
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}

// IsValidFormat returns true if format is a supported output format.
func IsValidFormat(format string) bool {
	_, ok := validFormats[format]
	return ok
}

// ValidFormats returns the list of supported output formats.
func ValidFormats() []string {
	return []string{"json", "json-pretty", "text"}
}
