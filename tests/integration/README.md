# Integration Tests

This directory contains integration tests for Kora's authentication system that interact with real external systems.

## Running Integration Tests

### Prerequisites

#### GitHub Integration Tests
- `gh` CLI must be installed (`brew install gh`)
- Must be authenticated: `gh auth login`

#### Slack/Keychain Integration Tests (macOS only)
- Running on macOS (darwin)
- User must grant keychain access when prompted
- Not supported in CI environments without keychain access

### Run All Integration Tests

```bash
# Run all integration tests
go test -tags=integration ./tests/integration/... -v

# Run with coverage
go test -tags=integration ./tests/integration/... -v -coverprofile=coverage.out
```

### Run Specific Integration Tests

```bash
# GitHub authentication tests only
go test -tags=integration ./tests/integration/... -v -run TestGitHub

# Slack/Keychain tests only (macOS)
go test -tags=integration ./tests/integration/... -v -run TestSlack
```

## Test Behavior

### Skipped Tests

Integration tests will skip automatically if prerequisites are not met:

- **GitHub tests**: Skip if `gh` CLI is not installed or not authenticated
- **Slack/Keychain tests**: Skip if not on macOS or running in CI

### Test Cleanup

All integration tests use `t.Cleanup()` to remove test data:
- Keychain entries created during tests are deleted after completion
- No persistent state should remain after test runs

## Unit Tests vs Integration Tests

### Unit Tests (`internal/auth/*_test.go`)
- No external dependencies
- Use mocks (MockKeychain, non-existent gh paths)
- Run by default: `go test ./internal/auth/...`
- Fast, deterministic, always pass in CI

### Integration Tests (`tests/integration/*_test.go`)
- Require real external systems (gh CLI, macOS Keychain)
- Use `//go:build integration` tag
- Only run with `-tags=integration` flag
- May be skipped if prerequisites not met

## Coverage

Integration tests are designed to complement unit tests:

- **Unit test coverage**: Core logic, error paths, edge cases
  - `internal/auth/slack`: 97.4%
  - `internal/auth/github`: 64.8%
  - `internal/auth/keychain`: 6.8% (darwin-specific code)

- **Integration test coverage**: Real system interactions
  - Actual keychain operations (macOS Security framework)
  - Real gh CLI delegation
  - End-to-end authentication flows

Combined coverage target: >80% for `internal/auth` packages.

## Security Considerations

Integration tests handle real credentials:
- Use test-specific keychain keys
- Clean up all test credentials after completion
- Skip tests in CI environments without secure credential storage
- Never commit credentials to version control

## Troubleshooting

### "gh CLI not authenticated"
```bash
# Authenticate gh CLI
gh auth login
```

### "Keychain access denied"
- macOS will prompt for keychain access on first run
- Click "Always Allow" to prevent repeated prompts
- Tests will fail if access is denied

### "build constraints exclude all Go files"
- This is expected when running without `-tags=integration`
- Integration tests are intentionally excluded from default test runs
