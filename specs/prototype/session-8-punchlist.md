# Session 8: Implement CLI Commands

## Agent Requirements (MANDATORY)

**YOU MUST invoke the following agents using the Task tool. Do NOT implement directly.**

| Agent | Invoke For | Task Tool Call |
|-------|------------|----------------|
| `golang-pro` | CLI commands and config | `Task(subagent_type="golang-pro", prompt="...")` |
| `test-automator` | Command and config tests | `Task(subagent_type="test-automator", prompt="...")` |

### Pre-Flight Checklist
- [ ] I will use Task tool to invoke `golang-pro` for implementation
- [ ] I will use Task tool to invoke `test-automator` for tests
- [ ] I will NOT write Go code directly

---

## Objective
Create `kora digest` command with Cobra, wire up datasources, auth, and formatters.

## Dependencies
- Session 4 complete (DataSourceRunner exists)
- Session 5 complete (GitHub datasource exists)
- Session 6 complete (Slack datasource exists)
- Session 7 complete (formatters exist)

## Files to Create/Modify
```
cmd/kora/main.go                # Update with proper structure
cmd/kora/root.go                # Root command definition
cmd/kora/digest.go              # Digest command implementation
cmd/kora/version.go             # Version command
internal/config/config.go       # YAML config loading
internal/config/validation.go   # Config validation
internal/config/config_test.go  # Config tests
configs/kora.yaml.example       # Example configuration
```

---

## Task 1: Invoke golang-pro Agent

**MANDATORY**: Use the Task tool with this prompt:

```
Task(
  subagent_type="golang-pro",
  prompt="""
Implement the CLI commands for Kora using Cobra.

1. internal/config/config.go:
   - Config struct with Datasources, Digest, Security sections
   - DatasourcesConfig: GitHub (Enabled, Orgs), Slack (Enabled, Workspaces)
   - DigestConfig: Window (duration), Timezone, Output (format)
   - SecurityConfig: RedactCredentials, DatasourceTimeout, VerifyTLS
   - Load(path string) (*Config, error) function
   - Default config path: ~/.kora/config.yaml
   - Apply defaults for missing values

2. internal/config/validation.go:
   - Validate() method on Config
   - Check DatasourceTimeout is positive
   - Check Output is valid format
   - Check at least one datasource enabled

3. cmd/kora/root.go:
   - Root command with Use="kora", Short/Long descriptions
   - Persistent flags: --config, --verbose
   - Execute() function

4. cmd/kora/version.go:
   - Version command showing version, commit, date
   - Variables set via ldflags at build time

5. cmd/kora/digest.go:
   - Digest command with flags:
     - --since/-s: Time window (default "16h"), accepts duration or RFC3339
     - --format/-f: Output format (default "terminal")
   - parseSince(string) (time.Time, error) function
   - runDigest implementation:
     a. Load config (merge with flags)
     b. Parse --since into time.Time
     c. Initialize auth providers (GitHub, Slack)
     d. Initialize enabled datasources
     e. Create DataSourceRunner
     f. Run with FetchOptions
     g. Create formatter
     h. Format and print output
     i. Set exit code: 0=success, 1=partial, 2=failure

6. cmd/kora/main.go:
   - Minimal: call root.Execute()
   - Exit with code from Execute

Example usage:
  kora digest --since 16h --format terminal
  kora digest --since 2025-12-05T17:00:00Z --format json
  kora digest --config ./my-config.yaml

7. configs/kora.yaml.example:
   - Complete example config with comments
   - Show all available options
"""
)
```

---

## Task 2: Invoke test-automator Agent

**MANDATORY**: Use the Task tool with this prompt:

```
Task(
  subagent_type="test-automator",
  prompt="""
Create tests for Kora CLI (cmd/kora/) and config (internal/config/).

1. internal/config/config_test.go:

   Load Tests:
   - Test Load() with valid YAML
   - Test Load() with missing file (should use defaults)
   - Test Load() with invalid YAML returns error
   - Test default values applied correctly

   Validation Tests:
   - Test valid config passes validation
   - Test zero timeout fails
   - Test invalid output format fails
   - Test no datasources enabled fails

2. cmd/kora/digest_test.go:

   parseSince Tests:
   - Test "16h" parses as duration
   - Test "2h30m" parses as duration
   - Test RFC3339 timestamp parses correctly
   - Test invalid format returns error

   Flag Tests:
   - Test --since flag is parsed
   - Test --format flag accepts terminal/markdown/json
   - Test --format with invalid value errors
   - Test --config flag path is used

3. Integration-style tests (with mocks):
   - Test digest command with mock datasources
   - Test partial failure sets exit code 1
   - Test complete failure sets exit code 2

Create mock datasources that return fixed events for testing.
Target >80% coverage.
"""
)
```

---

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success - all datasources worked |
| 1 | Partial success - some datasources failed |
| 2 | Failure - command error or all datasources failed |

## Example Config

```yaml
datasources:
  github:
    enabled: true
    orgs:
      - chainguard-dev

  slack:
    enabled: true
    workspaces:
      - chainguard

digest:
  window: 16h
  timezone: America/Los_Angeles
  output: terminal

security:
  redact_credentials: true
  datasource_timeout: 30s
  verify_tls: true
```

---

## Definition of Done
- [ ] `kora digest` command works end-to-end
- [ ] `--since` accepts durations and RFC3339
- [ ] `--format` accepts terminal/markdown/json
- [ ] `--config` loads YAML config
- [ ] Events sorted by Priority then Timestamp
- [ ] Exit codes: 0=success, 1=partial, 2=failure
- [ ] Help text is clear (`kora digest --help`)
- [ ] Config validation works
- [ ] Test coverage >80%
- [ ] `make test` passes
- [ ] Manual test: `./bin/kora digest --since 16h` works

## Next Session
Session 9: Integration Testing and Security Audit
