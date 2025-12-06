// Package keychain provides secure credential storage abstraction.
// Ground truth defined in specs/efas/0002-auth-provider.md
//
// IT IS FORBIDDEN TO CHANGE THIS INTERFACE without updating EFA 0002.
// Claude MUST stop and ask before modifying this file.
package keychain

import (
	"context"
	"errors"
	"fmt"
	"regexp"
)

// Keychain abstracts secure credential storage operations.
// On macOS, this wraps the Security framework via the `security` CLI.
//
// IT IS FORBIDDEN TO CHANGE THIS INTERFACE without updating EFA 0002.
//
// All operations use:
//   - Service name: "kora"
//   - Account name: the key parameter (e.g., "slack-token")
type Keychain interface {
	// Get retrieves a credential value from the keychain.
	// Returns ErrNotFound if the credential doesn't exist.
	Get(ctx context.Context, key string) (string, error)

	// Set stores a credential value in the keychain.
	// Overwrites any existing value for the same key.
	Set(ctx context.Context, key, value string) error

	// Delete removes a credential from the keychain.
	// Returns nil if the credential didn't exist.
	Delete(ctx context.Context, key string) error

	// Exists checks if a credential exists in the keychain.
	Exists(ctx context.Context, key string) bool
}

// Keychain-specific errors.
// IT IS FORBIDDEN TO CHANGE THESE ERRORS without updating EFA 0002.
var (
	// ErrNotFound indicates the requested credential doesn't exist.
	ErrNotFound = errors.New("keychain: credential not found")

	// ErrAccessDenied indicates the keychain denied access.
	ErrAccessDenied = errors.New("keychain: access denied")

	// ErrUnavailable indicates the keychain service is not available.
	ErrUnavailable = errors.New("keychain: service unavailable")
)

// keychainServiceName is the service identifier for all Kora keychain entries.
const keychainServiceName = "kora"

// allowedKeychainKeys is the authoritative set of valid keychain keys.
// SECURITY: This prevents key injection attacks where a malicious key
// could be crafted to escape the account name parameter.
// IT IS FORBIDDEN TO ADD KEYS without updating this allowlist and EFA 0002.
var allowedKeychainKeys = map[string]struct{}{
	"slack-token": {},
	// Add new keys here as needed, e.g.:
	// "linear-token": {},
	// "notion-token": {},
}

// keyPattern validates keychain key format.
// Only lowercase alphanumeric characters and hyphens are allowed.
// Keys must start with a letter and end with a letter or number.
var keyPattern = regexp.MustCompile(`^[a-z][a-z0-9-]*[a-z0-9]$`)

// validateKey ensures the key is in the allowlist and matches the safe pattern.
// Returns an error if the key is invalid or not allowed.
func validateKey(key string) error {
	if _, ok := allowedKeychainKeys[key]; !ok {
		return fmt.Errorf("keychain: key %q not in allowlist", key)
	}
	if !keyPattern.MatchString(key) {
		return fmt.Errorf("keychain: key %q contains invalid characters", key)
	}
	return nil
}
