package source

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/dakaneye/kora/internal/exec"
)

// cliSource provides shared auth logic for sources backed by CLI tools.
type cliSource struct {
	name        string
	runner      exec.Runner
	cli         string
	checkArgs   []string
	refreshArgs []string
}

func (s *cliSource) Name() string { return s.name }

func (s *cliSource) CheckAuth(ctx context.Context) error {
	_, err := s.runner.Run(ctx, s.cli, s.checkArgs...)
	if err != nil {
		return fmt.Errorf("%s auth check: %w", s.name, err)
	}
	return nil
}

func (s *cliSource) RefreshAuth(ctx context.Context) error {
	fmt.Fprintf(os.Stderr, "%s: refreshing auth via %s %s\n", s.name, s.cli, strings.Join(s.refreshArgs, " "))
	if err := s.runner.RunInteractive(ctx, s.cli, s.refreshArgs...); err != nil {
		return fmt.Errorf("%s auth refresh: %w", s.name, err)
	}
	return nil
}

// parallelQuery describes a single sub-query to run in parallel.
type parallelQuery struct {
	key  string
	args []string
}

// fetchParallel runs multiple CLI sub-queries concurrently and returns their
// raw JSON outputs keyed by query key. All errors are collected and joined.
func fetchParallel(ctx context.Context, runner exec.Runner, cli string, queries []parallelQuery) (map[string]json.RawMessage, error) {
	var mu sync.Mutex
	results := make(map[string]json.RawMessage, len(queries))
	var errs []error

	var wg sync.WaitGroup
	for _, q := range queries {
		wg.Add(1)
		go func() {
			defer wg.Done()
			result, err := runner.Run(ctx, cli, q.args...)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				errs = append(errs, fmt.Errorf("%s: %w", q.key, err))
				return
			}
			results[q.key] = json.RawMessage(result.Stdout)
		}()
	}
	wg.Wait()

	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}
	return results, nil
}
