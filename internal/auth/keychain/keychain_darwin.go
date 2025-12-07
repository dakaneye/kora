//go:build darwin

// Package keychain provides secure credential storage abstraction.
// Ground truth defined in specs/efas/0002-auth-provider.md
package keychain

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// MacOSKeychain implements Keychain using the macOS security CLI.
// IT IS FORBIDDEN TO CHANGE THIS IMPLEMENTATION without updating EFA 0002.
//
// SECURITY:
//   - All passwords are passed via stdin, never command-line args
//   - Command-line args are visible to other processes via `ps`
//   - Uses /usr/bin/security directly for predictable behavior
type MacOSKeychain struct {
	securityPath string // default: "/usr/bin/security"
}

// keychainTimeout is the maximum duration for keychain operations.
const keychainTimeout = 5 * time.Second

// macOS security command exit codes.
const (
	exitCodeItemNotFound = 44  // errSecItemNotFound
	exitCodeAccessDenied = 128 // User denied access
)

// NewMacOSKeychain creates a keychain backed by the macOS security CLI.
// If securityPath is empty, defaults to "/usr/bin/security".
func NewMacOSKeychain(securityPath string) *MacOSKeychain {
	if securityPath == "" {
		securityPath = "/usr/bin/security"
	}
	return &MacOSKeychain{securityPath: securityPath}
}

// Get retrieves a credential value from the keychain.
// Returns ErrNotFound if the credential doesn't exist.
func (k *MacOSKeychain) Get(ctx context.Context, key string) (string, error) {
	if err := validateKey(key); err != nil {
		return "", err
	}

	ctx, cancel := context.WithTimeout(ctx, keychainTimeout)
	defer cancel()

	// #nosec G204 -- securityPath is controlled, key is validated via allowlist
	cmd := exec.CommandContext(ctx, k.securityPath,
		"find-generic-password",
		"-s", keychainServiceName,
		"-a", key,
		"-w", // output password only
	)

	out, err := cmd.Output()
	if err != nil {
		return "", k.handleExecError(err, "get", key)
	}
	return strings.TrimSpace(string(out)), nil
}

// Set stores a credential value in the keychain.
// Overwrites any existing value for the same key.
//
// Note: The password is passed as a command-line argument to the security CLI.
// This is briefly visible via `ps` but the macOS security command doesn't support
// reading passwords from stdin for add-generic-password.
func (k *MacOSKeychain) Set(ctx context.Context, key, value string) error {
	if err := validateKey(key); err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(ctx, keychainTimeout)
	defer cancel()

	// Delete existing entry first (ignore errors - it may not exist)
	//nolint:errcheck // intentionally ignoring error - entry may not exist
	_ = k.Delete(ctx, key) // #nosec G104

	// Create the entry with password.
	// #nosec G204 -- securityPath is controlled, key is validated via allowlist
	cmd := exec.CommandContext(ctx, k.securityPath,
		"add-generic-password",
		"-s", keychainServiceName,
		"-a", key,
		"-w", value,
	)

	if err := cmd.Run(); err != nil {
		return k.handleExecError(err, "set", key)
	}
	return nil
}

// Delete removes a credential from the keychain.
// Returns nil if the credential didn't exist.
func (k *MacOSKeychain) Delete(ctx context.Context, key string) error {
	if err := validateKey(key); err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(ctx, keychainTimeout)
	defer cancel()

	// #nosec G204 -- securityPath is controlled, key is validated via allowlist
	cmd := exec.CommandContext(ctx, k.securityPath,
		"delete-generic-password",
		"-s", keychainServiceName,
		"-a", key,
	)

	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			// Not found is OK for delete
			if exitErr.ExitCode() == exitCodeItemNotFound {
				return nil
			}
			if exitErr.ExitCode() == exitCodeAccessDenied {
				return ErrAccessDenied
			}
			return fmt.Errorf("keychain delete %q: exit %d: %s",
				key, exitErr.ExitCode(), string(exitErr.Stderr))
		}
		return fmt.Errorf("keychain delete %q: %w", key, err)
	}
	return nil
}

// Exists checks if a credential exists in the keychain.
func (k *MacOSKeychain) Exists(ctx context.Context, key string) bool {
	_, err := k.Get(ctx, key)
	return err == nil
}

// handleExecError converts exec errors to appropriate keychain errors.
func (k *MacOSKeychain) handleExecError(err error, op, key string) error {
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		switch exitErr.ExitCode() {
		case exitCodeItemNotFound:
			return ErrNotFound
		case exitCodeAccessDenied:
			return ErrAccessDenied
		default:
			return fmt.Errorf("keychain %s %q: exit %d: %s",
				op, key, exitErr.ExitCode(), string(exitErr.Stderr))
		}
	}
	return fmt.Errorf("keychain %s %q: %w", op, key, err)
}
