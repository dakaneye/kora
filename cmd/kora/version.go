package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

// Version information set via ldflags at build time.
// Example: go build -ldflags "-X main.version=1.0.0 -X main.commit=abc123 -X main.date=2025-01-01"
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Long:  "Print the version, commit hash, and build date of kora.",
	Run: func(_ *cobra.Command, _ []string) {
		fmt.Printf("kora version %s (commit: %s, built: %s)\n", version, commit, date)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}

// Version returns the current version string.
func Version() string {
	return version
}
