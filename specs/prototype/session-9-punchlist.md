# Session 9: Integration Testing and Security Audit

## Agent Requirements (MANDATORY)

**YOU MUST invoke the following agents using the Task tool. Do NOT implement directly.**

| Agent | Invoke For | Task Tool Call |
|-------|------------|----------------|
| `test-automator` | Integration test design | `Task(subagent_type="test-automator", prompt="...")` |
| `security-auditor` | Full security audit | `Task(subagent_type="security-auditor", prompt="...")` |
| `deployment-engineer` | Security CI workflow | `Task(subagent_type="deployment-engineer", prompt="...")` |
| `golang-pro` | Fix security issues | `Task(subagent_type="golang-pro", prompt="...")` |

### Pre-Flight Checklist
- [ ] I will use Task tool to invoke `test-automator` for integration tests
- [ ] I will use Task tool to invoke `security-auditor` for full audit
- [ ] I will use Task tool to invoke `deployment-engineer` for security workflow
- [ ] I will use Task tool to invoke `golang-pro` ONLY to fix issues found
- [ ] I will NOT write code directly except as directed by agents

---

## Objective
End-to-end integration tests with real APIs, comprehensive security audit.

## Dependencies
- Session 8 complete (CLI works end-to-end)

## Files to Create
```
tests/integration/digest_test.go           # Full digest workflow
tests/integration/partial_failure_test.go  # One source fails
tests/integration/setup_test.go            # Test setup helpers
scripts/security-scan.sh                   # gosec + trivy wrapper
.github/workflows/security.yml             # Security scanning workflow
.trivyignore                               # False positive exceptions
```

---

## Task 1: Invoke test-automator Agent

**MANDATORY**: Use the Task tool with this prompt:

```
Task(
  subagent_type="test-automator",
  prompt="""
Create integration tests for Kora (tests/integration/).

All tests should use //go:build integration tag.

1. tests/integration/setup_test.go:
   - TestMain that skips if KORA_INTEGRATION_TESTS != "1"
   - requireGitHubAuth(t) helper that checks gh auth status
   - requireSlackAuth(t) helper that checks KORA_SLACK_TOKEN or keychain
   - Skip tests gracefully if auth not available

2. tests/integration/digest_test.go:
   - TestDigestEndToEnd:
     - Requires both GitHub and Slack auth
     - Runs full digest with 24h window
     - Verifies events returned
     - Verifies all events pass Validate()

   - TestDigestGitHubOnly:
     - Requires only GitHub auth
     - Runs with Slack disabled in config
     - Verifies GitHub events returned

   - TestDigestSlackOnly:
     - Requires only Slack auth
     - Runs with GitHub disabled
     - Verifies Slack events returned

3. tests/integration/partial_failure_test.go:
   - TestPartialFailure_AuthError:
     - Mock one datasource to have invalid auth
     - Verify other datasource events still returned
     - Verify RunResult.Partial == true
     - Verify error recorded in SourceErrors

   - TestPartialFailure_Timeout:
     - Create datasource with very short timeout
     - Verify timeout error recorded
     - Verify other sources complete

4. Add Makefile target:
   test-integration: KORA_INTEGRATION_TESTS=1 go test -v -tags=integration ./tests/integration/...
"""
)
```

---

## Task 2: Invoke security-auditor Agent

**MANDATORY**: Use the Task tool with this prompt:

```
Task(
  subagent_type="security-auditor",
  prompt="""
Perform FULL security audit of Kora codebase.

## Credential Security (EFA 0002)

1. Credential Logging Audit:
   - Search ALL .go files for log statements containing credential patterns
   - Patterns: token, password, secret, credential, xoxp-, ghp_, Authorization
   - Flag ANY log statement that might expose credentials
   - Verify all Credential.String() methods return Redacted()

2. Credential Storage Audit:
   - Verify GitHub tokens are NEVER stored (only delegation)
   - Verify Slack tokens use Keychain
   - Check for any plaintext credential files

3. Error Message Audit:
   - Check all error wrapping doesn't include credential values
   - Verify fmt.Errorf calls don't include sensitive data

## Network Security

4. TLS Verification:
   - Search for InsecureSkipVerify (should not exist)
   - Verify all http.Client has proper TLS config

5. Timeout Audit:
   - Verify all http.Client has Timeout set
   - Verify all exec.CommandContext has timeout
   - Check for unbounded operations

## Input Validation

6. Config Validation:
   - Check all config values are validated
   - Look for path traversal vulnerabilities
   - Check for injection in search queries

## Code Patterns to Flag

Run these searches and report findings:

```bash
# Credential patterns in logs
grep -rn "log\." --include="*.go" | grep -i "token\|secret\|password\|credential"

# Direct credential usage
grep -rn "\.Value()" --include="*.go" | grep -v "_test.go"

# InsecureSkipVerify
grep -rn "InsecureSkipVerify" --include="*.go"

# Missing timeouts
grep -rn "http.Client{}" --include="*.go"

# Potential injection
grep -rn "fmt.Sprintf.*query" --include="*.go"
```

## Report Format

For each finding:
- Severity: CRITICAL/HIGH/MEDIUM/LOW
- File:Line
- Description
- Recommended fix

CRITICAL findings MUST be fixed before merge.
"""
)
```

---

## Task 3: Invoke deployment-engineer Agent

**MANDATORY**: Use the Task tool with this prompt:

```
Task(
  subagent_type="deployment-engineer",
  prompt="""
Create security scanning workflow for Kora.

1. .github/workflows/security.yml:
   - Trigger: push to main, pull requests, weekly schedule
   - Run on macos-latest
   - Jobs:
     a. gosec: Run gosec static analysis
        - Install gosec
        - Run with SARIF output
        - Upload to GitHub Security tab
     b. trivy: Run trivy vulnerability scan
        - Use aquasecurity/trivy-action
        - Scan filesystem for vulnerabilities
        - Upload SARIF to GitHub Security
     c. credential-check: Custom credential audit
        - Run grep patterns for credential leaks
        - Fail if patterns found in non-test files

2. scripts/security-scan.sh:
   - Run gosec with JSON output
   - Run trivy fs scan
   - Run credential pattern grep
   - Exit non-zero if any issues found
   - Add to Makefile as security-scan target

3. .trivyignore:
   - Empty initially
   - Add comments explaining how to document exceptions

4. Update Makefile:
   - Add security-scan target that runs scripts/security-scan.sh
"""
)
```

---

## Task 4: Fix Security Issues (if any)

**ONLY IF** security-auditor finds issues, invoke golang-pro:

```
Task(
  subagent_type="golang-pro",
  prompt="""
Fix the following security issues found in Kora:

[LIST FINDINGS FROM SECURITY AUDIT HERE]

For each finding:
1. Understand the vulnerability
2. Apply the recommended fix
3. Add tests to prevent regression
4. Add comments explaining the security consideration
"""
)
```

---

## Security Checklist (from CLAUDE.md)

### Must Pass
- [ ] No credentials in logs (even debug level)
- [ ] Keychain operations handle "not found" gracefully
- [ ] TLS verification enabled for all HTTP clients
- [ ] Timeouts set on all network operations
- [ ] Input validation on config values

### Audit Results Template
```
gosec findings: ___ high, ___ medium, ___ low
trivy findings: ___ critical, ___ high, ___ medium
credential grep: ___ findings (should be 0)
coverage: ___% (target >80%)
```

---

## Definition of Done
- [ ] Integration tests run with real GitHub
- [ ] Integration tests run with real Slack
- [ ] Partial failure tests pass
- [ ] `make security-scan` runs successfully
- [ ] No CRITICAL security findings
- [ ] Credential grep audit: 0 findings
- [ ] All HTTP clients have timeouts
- [ ] TLS verification enabled everywhere
- [ ] Test coverage >80%
- [ ] Security workflow in CI
- [ ] `make test` passes
- [ ] `make test-integration` passes (with auth)

## Next Session
Session 10: Documentation and Release Preparation
