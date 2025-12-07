package config

import (
	"testing"
	"time"
)

func TestConfig_Validate(t *testing.T) {
	//nolint:govet // fieldalignment in tests is not a concern
	tests := []struct {
		name    string
		modify  func(*Config)
		wantErr bool
	}{
		{
			name:    "valid default config",
			modify:  func(_ *Config) {},
			wantErr: false,
		},
		{
			name: "no datasources enabled",
			modify: func(c *Config) {
				c.Datasources.GitHub.Enabled = false
			},
			wantErr: true,
		},
		{
			name: "invalid format",
			modify: func(c *Config) {
				c.Digest.Format = "markdown"
			},
			wantErr: true,
		},
		{
			name: "zero window",
			modify: func(c *Config) {
				c.Digest.Window = 0
			},
			wantErr: true,
		},
		{
			name: "negative window",
			modify: func(c *Config) {
				c.Digest.Window = -1 * time.Hour
			},
			wantErr: true,
		},
		{
			name: "zero timeout",
			modify: func(c *Config) {
				c.Security.DatasourceTimeout = 0
			},
			wantErr: true,
		},
		{
			name: "negative timeout",
			modify: func(c *Config) {
				c.Security.DatasourceTimeout = -1 * time.Second
			},
			wantErr: true,
		},
		{
			name: "invalid timezone",
			modify: func(c *Config) {
				c.Digest.Timezone = "Invalid/Timezone"
			},
			wantErr: true,
		},
		{
			name: "valid timezone",
			modify: func(c *Config) {
				c.Digest.Timezone = "America/New_York"
			},
			wantErr: false,
		},
		{
			name: "Local timezone",
			modify: func(c *Config) {
				c.Digest.Timezone = "Local"
			},
			wantErr: false,
		},
		{
			name: "UTC timezone",
			modify: func(c *Config) {
				c.Digest.Timezone = "UTC"
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := DefaultConfig()
			tt.modify(cfg)
			err := cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestDatasourcesConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     DatasourcesConfig
		wantErr bool
	}{
		{
			name: "github enabled",
			cfg: DatasourcesConfig{
				GitHub: GitHubConfig{Enabled: true},
			},
			wantErr: false,
		},
		{
			name: "github disabled no google",
			cfg: DatasourcesConfig{
				GitHub: GitHubConfig{Enabled: false},
			},
			wantErr: true,
		},
		{
			name: "github disabled with google calendar",
			cfg: DatasourcesConfig{
				GitHub: GitHubConfig{Enabled: false},
				Google: GoogleConfig{
					Calendars: []CalendarConfig{
						{Email: "test@example.com"},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "github disabled with gmail",
			cfg: DatasourcesConfig{
				GitHub: GitHubConfig{Enabled: false},
				Google: GoogleConfig{
					Gmail: []GmailConfig{
						{Email: "test@example.com"},
					},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestGoogleConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     GoogleConfig
		wantErr bool
	}{
		{
			name:    "empty config",
			cfg:     GoogleConfig{},
			wantErr: false,
		},
		{
			name: "valid calendar",
			cfg: GoogleConfig{
				Calendars: []CalendarConfig{
					{Email: "test@example.com", CalendarID: "primary"},
				},
			},
			wantErr: false,
		},
		{
			name: "valid gmail",
			cfg: GoogleConfig{
				Gmail: []GmailConfig{
					{Email: "test@example.com"},
				},
			},
			wantErr: false,
		},
		{
			name: "valid gmail with important senders",
			cfg: GoogleConfig{
				Gmail: []GmailConfig{
					{
						Email:            "test@example.com",
						ImportantSenders: []string{"boss@company.com", "@company.com"},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "calendar missing email",
			cfg: GoogleConfig{
				Calendars: []CalendarConfig{
					{Email: ""},
				},
			},
			wantErr: true,
		},
		{
			name: "gmail missing email",
			cfg: GoogleConfig{
				Gmail: []GmailConfig{
					{Email: ""},
				},
			},
			wantErr: true,
		},
		{
			name: "calendar invalid email format",
			cfg: GoogleConfig{
				Calendars: []CalendarConfig{
					{Email: "not-an-email"},
				},
			},
			wantErr: true,
		},
		{
			name: "gmail invalid email format",
			cfg: GoogleConfig{
				Gmail: []GmailConfig{
					{Email: "missing-at-sign"},
				},
			},
			wantErr: true,
		},
		{
			name: "multiple errors",
			cfg: GoogleConfig{
				Calendars: []CalendarConfig{
					{Email: ""},
					{Email: "bad"},
				},
				Gmail: []GmailConfig{
					{Email: ""},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestDigestConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     DigestConfig
		wantErr bool
	}{
		{
			name: "valid config",
			cfg: DigestConfig{
				Window:   16 * time.Hour,
				Timezone: "Local",
				Format:   "text",
			},
			wantErr: false,
		},
		{
			name: "json format",
			cfg: DigestConfig{
				Window:   16 * time.Hour,
				Timezone: "Local",
				Format:   "json",
			},
			wantErr: false,
		},
		{
			name: "json-pretty format",
			cfg: DigestConfig{
				Window:   16 * time.Hour,
				Timezone: "Local",
				Format:   "json-pretty",
			},
			wantErr: false,
		},
		{
			name: "invalid format",
			cfg: DigestConfig{
				Window:   16 * time.Hour,
				Timezone: "Local",
				Format:   "csv",
			},
			wantErr: true,
		},
		{
			name: "empty format",
			cfg: DigestConfig{
				Window:   16 * time.Hour,
				Timezone: "Local",
				Format:   "",
			},
			wantErr: true,
		},
		{
			name: "zero window",
			cfg: DigestConfig{
				Window:   0,
				Timezone: "Local",
				Format:   "text",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestIsValidFormat(t *testing.T) {
	tests := []struct {
		format string
		want   bool
	}{
		{"json", true},
		{"json-pretty", true},
		{"text", true},
		{"markdown", false},
		{"terminal", false},
		{"", false},
		{"JSON", false}, // case-sensitive
	}

	for _, tt := range tests {
		t.Run(tt.format, func(t *testing.T) {
			if got := IsValidFormat(tt.format); got != tt.want {
				t.Errorf("IsValidFormat(%q) = %v, want %v", tt.format, got, tt.want)
			}
		})
	}
}

func TestValidFormats(t *testing.T) {
	formats := ValidFormats()
	if len(formats) != 3 {
		t.Errorf("Expected 3 formats, got %d", len(formats))
	}

	expected := map[string]bool{"json": true, "json-pretty": true, "text": true}
	for _, f := range formats {
		if !expected[f] {
			t.Errorf("Unexpected format: %s", f)
		}
	}
}
