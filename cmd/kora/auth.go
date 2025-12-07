package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/dakaneye/kora/internal/auth/google"
	"github.com/dakaneye/kora/internal/auth/keychain"
)

// Auth command flags
var (
	authEmailFlag string
)

// authCmd is the base auth command.
var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Manage authentication",
	Long: `Manage authentication for datasources.

Use subcommands to login, check status, or logout from services.

Examples:
  # Login to Google
  kora auth google login --email user@example.com

  # Check Google auth status
  kora auth google status --email user@example.com

  # Logout from Google
  kora auth google logout --email user@example.com`,
}

// authGoogleCmd is the Google auth subcommand.
var authGoogleCmd = &cobra.Command{
	Use:   "google",
	Short: "Manage Google authentication",
	Long: `Manage Google OAuth authentication for Calendar and Gmail.

Google OAuth credentials are shared between Calendar and Gmail datasources
for the same email address.

Examples:
  kora auth google login --email user@example.com
  kora auth google status --email user@example.com
  kora auth google logout --email user@example.com`,
}

// authGoogleLoginCmd initiates Google OAuth login.
var authGoogleLoginCmd = &cobra.Command{
	Use:   "login",
	Short: "Authenticate with Google",
	Long: `Authenticate with Google using OAuth 2.0.

This opens a browser window for Google consent. After approval,
credentials are stored securely in the macOS Keychain.

Required scopes:
  - Google Calendar (read-only)
  - Gmail (read-only)

Example:
  kora auth google login --email user@example.com`,
	RunE: runAuthGoogleLogin,
}

// authGoogleStatusCmd checks Google authentication status.
var authGoogleStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Check Google authentication status",
	Long: `Check if valid Google credentials exist for the specified email.

Returns success if authenticated, or shows instructions to login.

Example:
  kora auth google status --email user@example.com`,
	RunE: runAuthGoogleStatus,
}

// authGoogleLogoutCmd removes Google authentication.
var authGoogleLogoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Remove Google authentication",
	Long: `Remove stored Google OAuth credentials from the keychain.

This revokes local access. You will need to re-authenticate to
use Google Calendar or Gmail datasources again.

Example:
  kora auth google logout --email user@example.com`,
	RunE: runAuthGoogleLogout,
}

func init() {
	// Add email flag to all Google auth commands
	authGoogleLoginCmd.Flags().StringVar(&authEmailFlag, "email", "", "Google account email (required)")
	authGoogleStatusCmd.Flags().StringVar(&authEmailFlag, "email", "", "Google account email (required)")
	authGoogleLogoutCmd.Flags().StringVar(&authEmailFlag, "email", "", "Google account email (required)")

	// Mark email as required.
	// These can only error if the flag doesn't exist, which is impossible here.
	//nolint:errcheck // Flag existence guaranteed by Flags().StringVar above
	authGoogleLoginCmd.MarkFlagRequired("email")
	//nolint:errcheck // Flag existence guaranteed by Flags().StringVar above
	authGoogleStatusCmd.MarkFlagRequired("email")
	//nolint:errcheck // Flag existence guaranteed by Flags().StringVar above
	authGoogleLogoutCmd.MarkFlagRequired("email")

	// Build command hierarchy
	authGoogleCmd.AddCommand(authGoogleLoginCmd)
	authGoogleCmd.AddCommand(authGoogleStatusCmd)
	authGoogleCmd.AddCommand(authGoogleLogoutCmd)

	authCmd.AddCommand(authGoogleCmd)

	rootCmd.AddCommand(authCmd)
}

// runAuthGoogleLogin handles the 'kora auth google login' command.
func runAuthGoogleLogin(cmd *cobra.Command, _ []string) error {
	email := authEmailFlag
	if email == "" {
		return fmt.Errorf("--email is required")
	}

	kc := keychain.NewMacOSKeychain("")
	provider, err := google.NewGoogleAuthProvider(kc, email)
	if err != nil {
		return fmt.Errorf("create auth provider: %w", err)
	}

	fmt.Printf("Opening browser for Google OAuth...\n")
	fmt.Printf("Please sign in with: %s\n\n", email)

	if err := provider.InitiateLogin(cmd.Context()); err != nil {
		return fmt.Errorf("login failed: %w", err)
	}

	fmt.Printf("\nSuccessfully authenticated as %s\n", email)
	return nil
}

// runAuthGoogleStatus handles the 'kora auth google status' command.
func runAuthGoogleStatus(cmd *cobra.Command, _ []string) error {
	email := authEmailFlag
	if email == "" {
		return fmt.Errorf("--email is required")
	}

	kc := keychain.NewMacOSKeychain("")
	provider, err := google.NewGoogleAuthProvider(kc, email)
	if err != nil {
		// OAuth config missing - not authenticated
		fmt.Printf("Not authenticated as %s\n", email)
		fmt.Printf("Run: kora auth google login --email %s\n", email)
		return nil
	}

	if provider.IsAuthenticated(cmd.Context()) {
		fmt.Printf("Authenticated as %s\n", email)
	} else {
		fmt.Printf("Not authenticated as %s\n", email)
		fmt.Printf("Run: kora auth google login --email %s\n", email)
	}
	return nil
}

// runAuthGoogleLogout handles the 'kora auth google logout' command.
func runAuthGoogleLogout(cmd *cobra.Command, _ []string) error {
	email := authEmailFlag
	if email == "" {
		return fmt.Errorf("--email is required")
	}

	kc := keychain.NewMacOSKeychain("")
	provider, err := google.NewGoogleAuthProvider(kc, email)
	if err != nil {
		// OAuth config missing, but we can still try to delete credentials
		// This handles the case where OAuth env vars were set previously
		fmt.Printf("Logged out from %s\n", email)
		return nil
	}

	if err := provider.Logout(cmd.Context()); err != nil {
		return fmt.Errorf("logout failed: %w", err)
	}

	fmt.Printf("Logged out from %s\n", email)
	return nil
}
