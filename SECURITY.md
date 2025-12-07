# Security Policy

## Reporting Vulnerabilities

Report security vulnerabilities via GitHub issues. This is a personal project with no formal security disclosure process.

For critical issues, contact: samuel@dakaneye.com

## Credential Handling

Kora handles credentials from external services (GitHub, Slack). Security is a priority.

### GitHub Authentication

**CLI Delegation - Zero Credential Storage**

Kora uses `gh` CLI delegation for all GitHub operations:

- Kora NEVER sees your GitHub token
- Kora NEVER stores GitHub credentials
- All GitHub API calls are delegated to `gh` CLI
- Token management is entirely handled by `gh`

**Setup:**
```bash
# Authenticate with gh CLI
gh auth login

# Kora will delegate all GitHub operations to gh
kora digest --since 16h
```

**Security guarantees:**
- Token never leaves `gh` CLI's control
- No credential files created by Kora
- No token exposure in logs or errors
- Credential theft impossible (Kora doesn't have it)

### Slack Authentication

**macOS Keychain Storage (Recommended)**

Slack tokens are stored in macOS Keychain with OS-managed encryption:

```bash
# Store token in Keychain
security add-generic-password -s "kora" -a "slack-token" -w "xoxp-..."

# Kora retrieves from Keychain as needed
kora digest --since 16h
```

**Keychain details:**
- Service name: `kora`
- Account name: `slack-token`
- Requires user password to access
- Encrypted by macOS

**Environment Variable Fallback (Less Secure)**

For CI/CD or when Keychain is unavailable:

```bash
export KORA_SLACK_TOKEN="xoxp-..."
kora digest --since 16h
```

**Security warnings:**
- Environment variables visible to process tree
- May appear in crash dumps
- May leak via `/proc` on some systems
- Use Keychain when possible

### Credential Redaction

All credentials are automatically redacted in logs:

**GitHub:**
- No token ever retrieved, nothing to redact
- Only username shown: `github:username`

**Slack:**
- Token values NEVER logged
- Only fingerprint shown: `xoxp-[8charHash]`
- Fingerprint is SHA256 hash - safe for correlation, not cracking

**Example log output:**
```
[INFO] authenticated with github:dakaneye
[INFO] authenticated with slack token: xoxp-[a1b2c3d4]
```

**Configuration:**
```yaml
# ~/.kora/config.yaml
security:
  redact_credentials: true  # Always on by default
```

## Network Security

### TLS Verification

TLS verification is ALWAYS enabled and cannot be disabled:

- All HTTP clients use TLS 1.2+
- Certificate validation always active
- No insecure fallback mode
- Connection timeouts prevent hangs

**Configured in code:**
```go
httpClient := &http.Client{
    Transport: &http.Transport{
        TLSClientConfig: &tls.Config{
            MinVersion: tls.VersionTLS12,
            // InsecureSkipVerify is NEVER set to true
        },
    },
    Timeout: 30 * time.Second,
}
```

### Input Validation

All user-provided input is validated:

**Organization names:**
- Alphanumeric characters only
- Hyphens allowed
- No path traversal characters
- Length limited

**Workspace names:**
- Same restrictions as organization names
- Validated against injection patterns

**Time windows:**
- Parsed via Go duration format
- Bounded to reasonable ranges
- Negative values rejected

## Data Storage

### In-Memory Only

Kora v1 stores NO persistent data:

- No database
- No cache files
- No credential files
- No log files on disk

All data exists only in process memory during execution.

### Temporary Files

Kora creates NO temporary files during normal operation.

## Dependencies

Kora has minimal dependencies to reduce attack surface:

**Direct dependencies:**
```
github.com/spf13/cobra      # CLI framework
golang.org/x/sync           # Concurrency utilities
gopkg.in/yaml.v3           # Config parsing
```

**Security practices:**
- Dependencies reviewed before adding
- `go mod verify` in CI
- Regular dependency updates
- `gosec` scans for vulnerabilities

## Audit Trail

### Logging

All credential operations are logged (with redaction):

```
[INFO] authenticating with github
[INFO] github authentication successful: github:username
[INFO] authenticating with slack
[INFO] slack authentication successful: xoxp-[fingerprint]
```

### No Secrets in Logs

Enforced at multiple levels:

1. **Code level:** Credential types implement `Redacted()` method
2. **Type level:** `String()` methods return redacted values
3. **CI level:** `gosec` scans for hardcoded secrets
4. **Review level:** Security checklist for auth PRs

## Security Checklist

Before merging auth-related code:

- [ ] No credential values in any log statement
- [ ] No credential values in any error message
- [ ] All credential types implement `Redacted()`
- [ ] `String()` methods return `Redacted()`
- [ ] No plaintext file storage
- [ ] CLI delegation used where available (GitHub)
- [ ] Keychain used for stored credentials (Slack)
- [ ] TLS verification enabled
- [ ] Timeouts set on all network operations
- [ ] Input validation on config values
- [ ] Integration tests don't commit credentials
- [ ] `gosec` scan passes

## Known Limitations

### Slack Enterprise Requirement

Slack integration requires enterprise workspace approval:

- User tokens (`xoxp-*`) need workspace admin approval
- Most users won't have access to Slack integration
- This is a Slack API limitation, not a Kora limitation

### macOS Only

Current implementation is macOS-specific:

- Keychain integration uses `security` CLI
- No cross-platform credential storage (yet)
- Use environment variables on other platforms (when ported)

### Token Lifetime

Kora does not handle token refresh:

- GitHub: `gh` CLI handles refresh automatically
- Slack: User tokens don't expire (by default)
- Manual re-authentication required if tokens invalidate

## Threat Model

### In Scope

Threats Kora actively mitigates:

- Credential theft from process memory
- Credential exposure in logs
- Credential exposure in error messages
- Credential storage in plaintext files
- Man-in-the-middle attacks (TLS)
- Command injection via config
- Path traversal via config
- Denial of service via large responses (10MB limit)

### Out of Scope

Threats not addressed (by design or limitation):

- Compromised `gh` CLI (trust boundary)
- Compromised macOS Keychain (OS responsibility)
- Malicious datasource implementations (code review)
- Process memory scraping (OS responsibility)
- Kernel-level attacks (OS responsibility)

## Incident Response

If you discover credentials exposed:

1. **GitHub:**
   - Run `gh auth logout`
   - Run `gh auth login` to get new token
   - Revoke old token at github.com/settings/tokens

2. **Slack:**
   - Revoke token at api.slack.com/apps
   - Remove from Keychain: `security delete-generic-password -s "kora" -a "slack-token"`
   - Generate new token (if workspace allows)

3. **Report:**
   - File GitHub issue or email samuel@dakaneye.com
   - Include steps to reproduce (no credentials in report)

## Security Updates

This project uses:

- Dependabot for dependency updates
- `gosec` for static security analysis
- `golangci-lint` for code quality

Check GitHub Actions for latest security scan results.
