package exec_test

import (
	"context"
	"testing"

	"github.com/dakaneye/kora/internal/exec"
)

func TestRun_Success(t *testing.T) {
	ctx := t.Context()
	result, err := exec.Run(ctx, "echo", "hello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Stdout != "hello\n" {
		t.Errorf("stdout = %q, want %q", result.Stdout, "hello\n")
	}
	if result.ExitCode != 0 {
		t.Errorf("exit code = %d, want 0", result.ExitCode)
	}
}

func TestRun_Failure(t *testing.T) {
	ctx := t.Context()
	result, err := exec.Run(ctx, "false")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if result.ExitCode != 1 {
		t.Errorf("exit code = %d, want 1", result.ExitCode)
	}
}

func TestRun_NotFound(t *testing.T) {
	ctx := t.Context()
	_, err := exec.Run(ctx, "nonexistent-command-xyz")
	if err == nil {
		t.Fatal("expected error for missing command")
	}
}

func TestRun_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	_, err := exec.Run(ctx, "sleep", "10")
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
}

func TestRun_CapturesStderr(t *testing.T) {
	ctx := t.Context()
	result, err := exec.Run(ctx, "sh", "-c", "echo oops >&2; exit 1")
	if err == nil {
		t.Fatal("expected error")
	}
	if result.Stderr != "oops\n" {
		t.Errorf("stderr = %q, want %q", result.Stderr, "oops\n")
	}
}
