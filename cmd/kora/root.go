package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// Global flags
var (
	configPath string
	verbose    bool
)

// Exit codes
const (
	ExitSuccess        = 0
	ExitPartialFailure = 1
	ExitFailure        = 2
)

// rootCmd is the base command when called without any subcommands.
var rootCmd = &cobra.Command{
	Use:   "kora",
	Short: "Morning digest aggregator for GitHub and Slack",
	Long: `Kora aggregates work updates from GitHub and Slack into a
prioritized morning digest that helps you start your day focused
on what matters most.

Run 'kora digest' to generate your morning digest.
Run 'kora version' to see version information.`,
	SilenceUsage:  true, // Don't print usage on errors
	SilenceErrors: true, // We handle errors ourselves
}

func init() {
	// Persistent flags available to all subcommands
	rootCmd.PersistentFlags().StringVar(&configPath, "config", "", "config file (default is $HOME/.kora/config.yaml)")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")
}

// Execute runs the root command and returns an exit code.
// Use this instead of calling rootCmd.Execute() directly.
func Execute() int {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		return ExitFailure
	}
	return ExitSuccess
}
