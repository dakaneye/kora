package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/dakaneye/kora/internal/auth/github"
	"github.com/dakaneye/kora/internal/auth/google"
	"github.com/dakaneye/kora/internal/auth/keychain"
	"github.com/dakaneye/kora/internal/config"
	"github.com/dakaneye/kora/internal/datasources"
	githubds "github.com/dakaneye/kora/internal/datasources/github"
	"github.com/dakaneye/kora/internal/datasources/gmail"
	"github.com/dakaneye/kora/internal/datasources/google_calendar"
	"github.com/dakaneye/kora/internal/output"
)

// Digest command flags
var (
	sinceFlag  string
	formatFlag string
)

var digestCmd = &cobra.Command{
	Use:   "digest",
	Short: "Generate your morning digest",
	Long: `Generate a prioritized digest of work updates from GitHub.

The digest includes:
  - PR review requests
  - Mentions in PRs and issues
  - Assigned issues

Examples:
  # Digest from the last 16 hours (default)
  kora digest

  # Digest from the last 8 hours
  kora digest --since 8h

  # Digest since a specific time (RFC3339 format)
  kora digest --since 2025-12-05T09:00:00Z

  # Output as JSON for scripting
  kora digest --format json`,
	RunE: runDigest,
}

func init() {
	digestCmd.Flags().StringVarP(&sinceFlag, "since", "s", "", "time window (duration like 16h) or RFC3339 timestamp")
	digestCmd.Flags().StringVarP(&formatFlag, "format", "f", "", "output format: json, json-pretty, text (default from config)")

	rootCmd.AddCommand(digestCmd)
}

// runDigest executes the digest command.
func runDigest(cmd *cobra.Command, _ []string) error {
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}

	// Load configuration
	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	// Parse --since flag
	since, err := parseSince(sinceFlag, cfg.Digest.Window)
	if err != nil {
		return fmt.Errorf("invalid --since: %w", err)
	}

	// Determine output format
	format := formatFlag
	if format == "" {
		format = cfg.Digest.Format
	}
	if !config.IsValidFormat(format) {
		return fmt.Errorf("invalid --format: %q (supported: %s)", format, strings.Join(config.ValidFormats(), ", "))
	}

	// Initialize datasources
	sources, initErrors := initDatasources(ctx, cfg)
	if len(sources) == 0 {
		// No datasources initialized successfully
		for name, err := range initErrors {
			fmt.Fprintf(os.Stderr, "Warning: %s: %v\n", name, err)
		}
		return fmt.Errorf("no datasources available")
	}

	// Log initialization warnings if verbose
	if verbose {
		for name, err := range initErrors {
			fmt.Fprintf(os.Stderr, "Warning: %s: %v\n", name, err)
		}
	}

	// Create runner with configured timeout
	runner := datasources.NewRunner(sources, datasources.WithTimeout(cfg.Security.DatasourceTimeout))

	// Execute fetch
	fetchOpts := datasources.FetchOptions{
		Since: since,
	}
	startTime := time.Now()
	result, err := runner.Run(ctx, fetchOpts)
	if err != nil {
		return fmt.Errorf("fetch failed: %w", err)
	}
	duration := time.Since(startTime)

	// Convert source errors to string map for formatter
	sourceErrors := make(map[string]string)
	for name, err := range result.SourceErrors {
		sourceErrors[name] = err.Error()
	}

	// Create stats for formatter
	stats := output.NewFormatStats(result.Events, duration, sourceErrors)

	// Create formatter and format output
	formatter, err := output.NewFormatter(format)
	if err != nil {
		return fmt.Errorf("create formatter: %w", err)
	}

	out, err := formatter.Format(result.Events, stats)
	if err != nil {
		return fmt.Errorf("format output: %w", err)
	}

	// Print output
	fmt.Print(out)

	// Set exit code based on result
	if !result.Success() && !result.Partial() {
		// Complete failure (no events, only errors)
		os.Exit(ExitFailure)
	}
	if result.Partial() {
		// Partial success
		os.Exit(ExitPartialFailure)
	}

	return nil
}

// parseSince parses the --since flag value.
// Accepts either a duration (e.g., "16h", "30m") or an RFC3339 timestamp.
// If value is empty, uses the default window from config.
func parseSince(value string, defaultWindow time.Duration) (time.Time, error) {
	now := time.Now()

	// If empty, use default window
	if value == "" {
		return now.Add(-defaultWindow), nil
	}

	// Try parsing as duration first
	if d, err := time.ParseDuration(value); err == nil {
		if d <= 0 {
			return time.Time{}, fmt.Errorf("duration must be positive, got %s", value)
		}
		return now.Add(-d), nil
	}

	// Try parsing as RFC3339
	t, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}, fmt.Errorf("must be a duration (e.g., 16h) or RFC3339 timestamp (e.g., 2025-12-05T09:00:00Z): %s", value)
	}

	// Timestamp must be in the past
	if t.After(now) {
		return time.Time{}, fmt.Errorf("timestamp must be in the past: %s", value)
	}

	return t, nil
}

// initDatasources initializes enabled datasources based on configuration.
// Returns successfully initialized datasources and a map of initialization errors.
// At least one datasource must be available; errors for individual datasources
// are returned but don't prevent other datasources from being used.
func initDatasources(ctx context.Context, cfg *config.Config) (sources []datasources.DataSource, initErrors map[string]error) {
	initErrors = make(map[string]error)

	// Initialize GitHub datasource if enabled
	if cfg.Datasources.GitHub.Enabled {
		ghAuth := github.NewGitHubAuthProvider("")
		if err := ghAuth.Authenticate(ctx); err != nil {
			initErrors["github"] = fmt.Errorf("auth failed: %w", err)
		} else {
			var opts []githubds.Option
			if len(cfg.Datasources.GitHub.Orgs) > 0 {
				opts = append(opts, githubds.WithOrgs(cfg.Datasources.GitHub.Orgs))
			}
			if len(cfg.Datasources.GitHub.WatchedRepos) > 0 {
				opts = append(opts, githubds.WithWatchedRepos(cfg.Datasources.GitHub.WatchedRepos))
			}
			ghDS, err := githubds.NewDataSource(ghAuth, opts...)
			if err != nil {
				initErrors["github"] = fmt.Errorf("init failed: %w", err)
			} else {
				sources = append(sources, ghDS)
			}
		}
	}

	// Create auth providers for unique Google emails (shared between calendar and gmail)
	// Each email needs one OAuth credential, shared across Calendar and Gmail datasources
	kc := keychain.NewMacOSKeychain("")
	googleProviders := make(map[string]*google.GoogleAuthProvider)
	for _, email := range cfg.UniqueGoogleEmails() {
		provider, err := google.NewGoogleAuthProvider(kc, email)
		if err != nil {
			initErrors[fmt.Sprintf("google:%s", email)] = fmt.Errorf("auth provider init failed: %w", err)
			continue
		}
		googleProviders[email] = provider
	}

	// Initialize Google Calendar datasources
	for _, calCfg := range cfg.Datasources.Google.Calendars {
		provider, ok := googleProviders[calCfg.Email]
		if !ok {
			// Auth provider failed earlier, already logged
			continue
		}
		if err := provider.Authenticate(ctx); err != nil {
			initErrors[fmt.Sprintf("google-calendar:%s", calCfg.Email)] = fmt.Errorf("auth failed: %w", err)
			continue
		}
		calID := calCfg.CalendarID
		if calID == "" {
			calID = "primary"
		}
		ds, err := google_calendar.NewGoogleCalendarDataSource(provider,
			google_calendar.WithCalendarIDs([]string{calID}))
		if err != nil {
			initErrors[fmt.Sprintf("google-calendar:%s", calCfg.Email)] = fmt.Errorf("init failed: %w", err)
			continue
		}
		sources = append(sources, ds)
	}

	// Initialize Gmail datasources
	for _, gmailCfg := range cfg.Datasources.Google.Gmail {
		provider, ok := googleProviders[gmailCfg.Email]
		if !ok {
			// Auth provider failed earlier, already logged
			continue
		}
		if err := provider.Authenticate(ctx); err != nil {
			initErrors[fmt.Sprintf("gmail:%s", gmailCfg.Email)] = fmt.Errorf("auth failed: %w", err)
			continue
		}
		ds, err := gmail.NewGmailDataSource(provider,
			gmail.WithImportantSenders(gmailCfg.ImportantSenders))
		if err != nil {
			initErrors[fmt.Sprintf("gmail:%s", gmailCfg.Email)] = fmt.Errorf("init failed: %w", err)
			continue
		}
		sources = append(sources, ds)
	}

	return sources, initErrors
}
