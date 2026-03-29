// Package exec provides subprocess execution with captured output.
package exec

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
)

// Result holds the captured output and exit code of a subprocess.
type Result struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

// Runner executes subprocesses.
type Runner interface {
	Run(ctx context.Context, name string, args ...string) (Result, error)
	RunInteractive(ctx context.Context, name string, args ...string) error
}

// DefaultRunner delegates to the real exec.CommandContext.
type DefaultRunner struct{}

// Run executes a command via the operating system.
func (d *DefaultRunner) Run(ctx context.Context, name string, args ...string) (Result, error) {
	return Run(ctx, name, args...)
}

// RunInteractive executes a command with the terminal attached.
func (d *DefaultRunner) RunInteractive(ctx context.Context, name string, args ...string) error {
	return RunInteractive(ctx, name, args...)
}

// Run executes a command with the given arguments and returns the captured output.
// Returns a non-nil error if the command fails or the context is canceled.
// The Result is always populated (even on error) so callers can inspect stderr.
func Run(ctx context.Context, name string, args ...string) (Result, error) {
	cmd := exec.CommandContext(ctx, name, args...) //nolint:gosec // G204: subprocess execution is this package's purpose

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

// RunInteractive executes a command with stdin/stdout/stderr connected to the
// terminal so the user can interact with prompts (e.g. auth login flows).
func RunInteractive(ctx context.Context, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...) //nolint:gosec // G204: subprocess execution is this package's purpose
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("exec %s: %w", name, err)
	}
	return nil
}
