package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"time"

	"github.com/dakaneye/kora/internal/source"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	sinceStr := flag.String("since", "16h", "time window to look back (e.g. 8h, 7d)")
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Printf("kora %s (%s) built %s\n", version, commit, date)
		os.Exit(0)
	}

	since, err := time.ParseDuration(*sinceStr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "invalid --since value %q: %v\n", *sinceStr, err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	sources := []source.Source{
		source.NewGitHub(nil),
		source.NewGmail(nil),
		source.NewCalendar(nil),
		source.NewLinear(nil),
	}

	result, err := source.Run(ctx, sources, since)
	if err != nil {
		errOutput := map[string]any{"errors": err.Error()}
		errJSON, _ := json.Marshal(errOutput)
		fmt.Fprintln(os.Stderr, string(errJSON))
		os.Exit(1)
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(result); err != nil {
		fmt.Fprintf(os.Stderr, "encoding output: %v\n", err)
		os.Exit(1)
	}
}
