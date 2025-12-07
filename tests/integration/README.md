# Integration Tests

This directory contains integration tests for Kora that interact with real external systems, including:
- Authentication system (GitHub gh CLI, macOS Keychain)
- Datasource fetching (GitHub API, Slack API)
- CLI command execution (digest command)
- Partial failure scenarios

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
# Run all integration tests (with KORA_INTEGRATION_TESTS flag)
KORA_INTEGRATION_TESTS=1 go test -tags=integration ./tests/integration/... -v

# Or use the Makefile
make test-integration

# Run with coverage
KORA_INTEGRATION_TESTS=1 go test -tags=integration ./tests/integration/... -v -coverprofile=coverage.out
```

### Run Specific Integration Tests

```bash
# GitHub authentication tests only
KORA_INTEGRATION_TESTS=1 go test -tags=integration ./tests/integration/... -v -run TestGitHubAuth

# GitHub datasource tests only
KORA_INTEGRATION_TESTS=1 go test -tags=integration ./tests/integration/... -v -run TestGitHubDataSource

# Slack/Keychain tests only (macOS)
KORA_INTEGRATION_TESTS=1 go test -tags=integration ./tests/integration/... -v -run TestSlack

# Digest command tests only
KORA_INTEGRATION_TESTS=1 go test -tags=integration ./tests/integration/... -v -run TestDigestCommand

# Partial failure tests only
KORA_INTEGRATION_TESTS=1 go test -tags=integration ./tests/integration/... -v -run TestPartialFailure
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

## Test Categories

### Authentication Tests (`*_auth_test.go`)
- **GitHub**: Tests gh CLI delegation, username retrieval, API execution
- **Slack**: Tests macOS Keychain operations, token storage/retrieval

### Datasource Tests (`github_test.go`)
- **Fetch with real API**: Fetches actual GitHub events from last 24h
- **Event validation**: Verifies all events pass EFA 0001 Validate()
- **Rate limiting**: Tests rate limit handling and info
- **Concurrency**: Tests concurrent fetch operations
- **Context cancellation**: Tests proper context handling
- **Invalid options**: Tests validation of fetch parameters

### CLI Command Tests (`digest_test.go`)
- **JSON output**: Tests digest command with JSON format
- **Text output**: Tests digest command with text format
- **Short window**: Tests with 1-hour lookback
- **Timeout handling**: Tests datasource timeout configuration
- **Output formats**: Tests all format options (json, json-pretty, text)
- **Version flag**: Tests version command
- **Live execution**: End-to-end digest with real config

### Partial Failure Tests (`partial_failure_test.go`)
- **Slack disabled**: Verifies GitHub events returned when Slack is disabled
- **GitHub disabled**: Verifies Slack events returned when GitHub is disabled
- **Both sources**: Tests runner with both datasources enabled
- **One source fails**: Tests behavior when one datasource fails completely
- **Empty results**: Tests behavior when no events are found

## Unit Tests vs Integration Tests

### Unit Tests (`internal/*/\*_test.go`)
- No external dependencies
- Use mocks (MockKeychain, mock HTTP clients)
- Run by default: `go test ./...`
- Fast, deterministic, always pass in CI

### Integration Tests (`tests/integration/*_test.go`)
- Require real external systems (gh CLI, macOS Keychain, GitHub/Slack APIs)
- Use `//go:build integration` tag
- Only run with `KORA_INTEGRATION_TESTS=1` and `-tags=integration`
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
