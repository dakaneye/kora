package source_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dakaneye/kora/internal/exec"
)

func loadFixture(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("load fixture %s: %v", name, err)
	}
	return string(data)
}

type fakeResult struct {
	stdout string
	stderr string
	err    string
}

// fakeRunner mocks exec.Run for testing. Keys are matched by prefix against
// the command string (name + " " + args joined by " ").
type fakeRunner struct {
	results map[string]fakeResult
}

func (f *fakeRunner) Run(ctx context.Context, name string, args ...string) (exec.Result, error) {
	cmd := name + " " + strings.Join(args, " ")
	for prefix, fr := range f.results {
		if strings.HasPrefix(cmd, prefix) {
			r := exec.Result{Stdout: fr.stdout, Stderr: fr.stderr}
			if fr.err != "" {
				r.ExitCode = 1
				return r, errors.New(fr.err)
			}
			return r, nil
		}
	}
	return exec.Result{ExitCode: 1}, errors.New("fakeRunner: no match for " + cmd)
}
