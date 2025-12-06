# Session 3: Implement Auth Providers (EFA 0002)

## Agent Requirements (MANDATORY)

**YOU MUST invoke the following agents using the Task tool. Do NOT implement directly.**

| Agent | Invoke For | Task Tool Call |
|-------|------------|----------------|
| `golang-pro` | Auth interfaces, implementations | `Task(subagent_type="golang-pro", prompt="...")` |
| `security-auditor` | **CRITICAL**: Review all auth code | `Task(subagent_type="security-auditor", prompt="...")` |
| `test-automator` | Unit and integration tests | `Task(subagent_type="test-automator", prompt="...")` |

### Pre-Flight Checklist
- [ ] I will use Task tool to invoke `golang-pro` for implementation
- [ ] I will use Task tool to invoke `security-auditor` to review credential handling
- [ ] I will use Task tool to invoke `test-automator` for tests
- [ ] I will NOT write auth code directly
- [ ] I have read EFA 0002 before starting

---

## Objective
Create AuthProvider and Credential interfaces, implement GitHub CLI delegation and Slack Keychain storage.

## Dependencies
- Session 2 complete (models exist for Source type)

## Files to Create
```
internal/auth/auth.go              # Service, AuthProvider interface
internal/auth/errors.go            # Sentinel errors
internal/auth/credential.go        # Credential, CredentialType interfaces
internal/auth/github/auth.go       # GitHubAuthProvider, GitHubDelegatedCredential
internal/auth/github/auth_test.go  # Mock gh CLI tests
internal/auth/slack/auth.go        # SlackAuthProvider, SlackToken
internal/auth/slack/auth_test.go   # Mock keychain tests
internal/auth/keychain/keychain.go         # Keychain interface
internal/auth/keychain/keychain_darwin.go  # macOS security command wrapper
internal/auth/keychain/keychain_test.go    # Keychain tests
tests/integration/github_auth_test.go      # //go:build integration
tests/integration/slack_auth_test.go       # //go:build integration
```

---

## Task 1: Invoke golang-pro Agent

**MANDATORY**: Use the Task tool with this prompt:

```
Task(
  subagent_type="golang-pro",
  prompt="""
Implement the Auth Provider system for Kora per EFA 0002 (specs/efas/0002-auth-provider.md).

Read the EFA first, then implement:

1. internal/auth/auth.go:
   - Service type with constants: ServiceGitHub, ServiceSlack
   - AuthProvider interface: Service(), Authenticate(ctx), GetCredential(ctx), IsAuthenticated(ctx)
   - Add file header: // Ground truth defined in specs/efas/0002-auth-provider.md

2. internal/auth/errors.go:
   - Sentinel errors: ErrNotAuthenticated, ErrCredentialNotFound, ErrCredentialInvalid,
     ErrKeychainUnavailable, ErrGHCLINotFound, ErrGHCLINotAuthenticated

3. internal/auth/credential.go:
   - Credential interface: Type(), Value(), Redacted(), IsValid()
   - CredentialType constants: CredentialTypeOAuth, CredentialTypeToken

4. internal/auth/keychain/keychain.go:
   - Keychain interface: Get(ctx, key), Set(ctx, key, value), Delete(ctx, key), Exists(ctx, key)
   - allowedKeychainKeys map with only "slack-token" for v1
   - validateKey() function

5. internal/auth/keychain/keychain_darwin.go:
   - MacOSKeychain implementation using /usr/bin/security command
   - Service name: "kora"
   - Use stdin for password (NOT command-line args for security)
   - Handle exit codes: 44=not found, 128=access denied

6. internal/auth/slack/auth.go:
   - SlackToken implementing Credential
   - IsValid() checks xoxp- prefix
   - Redacted() returns hash-based fingerprint using SHA256 (NOT partial token)
   - String() returns Redacted()
   - SlackAuthProvider: tries keychain first, falls back to KORA_SLACK_TOKEN env var

7. internal/auth/github/auth.go:
   - GitHubDelegatedCredential: username, ghPath fields
   - Value() returns empty string (tokens never extracted)
   - ExecuteAPI(ctx, endpoint, args...) method for gh api calls
   - Redacted() returns "github:username"
   - GitHubAuthProvider: delegates to gh auth status, never stores tokens

CRITICAL SECURITY:
- NEVER log credential values
- String() MUST return Redacted()
- Add EFA protection comments
"""
)
```

---

## Task 2: Invoke security-auditor Agent

**MANDATORY**: After golang-pro completes, use this prompt:

```
Task(
  subagent_type="security-auditor",
  prompt="""
CRITICAL SECURITY REVIEW for Kora auth code (internal/auth/).

Review per EFA 0002 security requirements:

1. Credential Logging Audit:
   - Search for ANY log statements containing credentials
   - Verify all Credential types implement Redacted() correctly
   - Verify String() methods return Redacted()
   - Check error messages don't contain credential values

2. Credential Storage Audit:
   - Verify GitHub tokens are NEVER stored (delegation only)
   - Verify Slack tokens use Keychain, not files
   - Check keychain key validation prevents injection

3. Code Patterns to Flag:
   - log.*(.*token.*) without Redacted()
   - fmt.*(.*Value().*)
   - error.*credential.*value
   - Any hardcoded credentials

4. TLS/Timeout Audit:
   - Verify exec.CommandContext uses timeouts
   - Check for InsecureSkipVerify (should not exist)

Report:
- List all findings with file:line references
- Severity: CRITICAL/HIGH/MEDIUM/LOW
- Recommended fixes

If ANY credential logging is found, this is a CRITICAL finding that must be fixed before merge.
"""
)
```

---

## Task 3: Invoke test-automator Agent

**MANDATORY**: Use the Task tool with this prompt:

```
Task(
  subagent_type="test-automator",
  prompt="""
Create comprehensive tests for Kora auth system (internal/auth/).

1. internal/auth/keychain/keychain_test.go:
   - Test key validation (allowed vs disallowed keys)
   - Mock exec.Command for security CLI
   - Test Get/Set/Delete/Exists with mocked responses
   - Test error handling: not found, access denied

2. internal/auth/slack/auth_test.go:
   - Test SlackToken.IsValid() with valid/invalid tokens
   - Test SlackToken.Redacted() returns fingerprint, not partial token
   - Test SlackAuthProvider with mock keychain
   - Test env var fallback when keychain empty

3. internal/auth/github/auth_test.go:
   - Test GitHubDelegatedCredential.Value() returns empty
   - Test GitHubDelegatedCredential.Redacted() format
   - Test GitHubAuthProvider.Authenticate() with mock gh CLI
   - Test ExecuteAPI() with mock responses

4. tests/integration/github_auth_test.go:
   - //go:build integration tag
   - Test with real gh CLI (requires gh auth login)
   - Skip if gh not authenticated

5. tests/integration/slack_auth_test.go:
   - //go:build integration tag
   - Test real keychain operations
   - Clean up keychain entries after test
   - Skip if KORA_SLACK_TOKEN not set

Use testify/mock or manual mocks for exec.Command.
Target >80% coverage.
"""
)
```

---

## EFA 0002 Key Constraints

### Security Rules (ABSOLUTE)
1. **NEVER** log credential values, even partially
2. **NEVER** include credentials in error messages
3. **ALWAYS** use Redacted() for any output
4. **ALWAYS** implement String() to return Redacted()

### Credential Redaction
- **SlackToken.Redacted()**: `xoxp-[8-char-sha256-fingerprint]`
- **GitHubDelegatedCredential.Redacted()**: `github:username`

### Keychain Key Allowlist
Only these keys are permitted in v1:
```go
var allowedKeychainKeys = map[string]struct{}{
    "slack-token": {},
}
```

---

## Definition of Done
- [ ] AuthProvider interface matches EFA 0002
- [ ] GitHubAuthProvider delegates to gh CLI (no token storage)
- [ ] SlackAuthProvider uses keychain with env var fallback
- [ ] All Credential types implement Redacted() correctly
- [ ] String() methods return Redacted()
- [ ] Security audit passes with no CRITICAL findings
- [ ] Unit tests pass with mocked commands
- [ ] Integration tests pass (with real auth)
- [ ] `make test` passes

## Next Session
Session 4: Implement DataSource Interface (EFA 0003)
