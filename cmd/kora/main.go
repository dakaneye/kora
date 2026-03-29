// Package main is the entry point for the kora CLI.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"time"

	"github.com/dakaneye/kora/internal/config"
	"github.com/dakaneye/kora/internal/source"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	os.Exit(run())
}

func run() int {
	sinceStr := flag.String("since", "16h", "time window to look back (e.g. 8h, 7d)")
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Printf("kora %s (%s) built %s\n", version, commit, date)
		return 0
	}

	since, err := parseSince(*sinceStr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "invalid --since value %q: %v\n", *sinceStr, err)
		return 1
	}

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "config: %v\n", err)
		return 1
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	sources := []source.Source{
		source.NewGitHub(nil, cfg.GitHub.Orgs, cfg.GitHub.Repos),
		source.NewGmail(nil),
		source.NewCalendar(nil),
		source.NewLinear(nil, cfg.Linear.Teams),
	}

	result, err := source.Run(ctx, sources, since)
	if err != nil {
		errOutput := map[string]any{"errors": err.Error()}
		errJSON, marshalErr := json.Marshal(errOutput)
		if marshalErr != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			return 1
		}
		fmt.Fprintln(os.Stderr, string(errJSON))
		return 1
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(result); err != nil {
		fmt.Fprintf(os.Stderr, "encoding output: %v\n", err)
		return 1
	}
	return 0
}

// parseSince parses a duration string, supporting "d" suffix for days
// in addition to standard time.ParseDuration units.
func parseSince(s string) (time.Duration, error) {
	if strings.HasSuffix(s, "d") {
		days := strings.TrimSuffix(s, "d")
		n, err := strconv.Atoi(days)
		if err != nil {
			return 0, fmt.Errorf("invalid days value %q: %w", s, err)
		}
		if n < 0 {
			return 0, fmt.Errorf("negative duration %q not allowed", s)
		}
		return time.Duration(n) * 24 * time.Hour, nil
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return 0, err
	}
	if d < 0 {
		return 0, fmt.Errorf("negative duration %q not allowed", s)
	}
	return d, nil
}
