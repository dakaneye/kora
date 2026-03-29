package exec

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
)

// Result holds the output of a subprocess execution.
type Result struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

// Runner abstracts subprocess execution for testing.
type Runner interface {
	Run(ctx context.Context, name string, args ...string) (Result, error)
}

// DefaultRunner uses real os/exec.
type DefaultRunner struct{}

func (d *DefaultRunner) Run(ctx context.Context, name string, args ...string) (Result, error) {
	return Run(ctx, name, args...)
}

// Run executes a command with the given arguments and returns the captured output.
// Returns a non-nil error if the command fails or the context is cancelled.
// The Result is always populated (even on error) so callers can inspect stderr.
func Run(ctx context.Context, name string, args ...string) (Result, error) {
	cmd := exec.CommandContext(ctx, name, args...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	result := Result{
		Stdout: stdout.String(),
		Stderr: stderr.String(),
	}

	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			result.ExitCode = exitErr.ExitCode()
		} else {
			result.ExitCode = -1
		}
		return result, fmt.Errorf("exec %s: %w (stderr: %s)", name, err, result.Stderr)
	}

	return result, nil
}
