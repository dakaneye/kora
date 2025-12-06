# Session 1: Bootstrap Go Project Structure

## Agent Requirements (MANDATORY)

**YOU MUST invoke the following agents using the Task tool. Do NOT implement directly.**

| Agent | Invoke For | Task Tool Call |
|-------|------------|----------------|
| `golang-pro` | Project structure, packages, Makefile | `Task(subagent_type="golang-pro", prompt="...")` |
| `deployment-engineer` | CI workflow, dependabot | `Task(subagent_type="deployment-engineer", prompt="...")` |

### Pre-Flight Checklist
- [ ] I will use Task tool to invoke `golang-pro` for Go code
- [ ] I will use Task tool to invoke `deployment-engineer` for CI/CD
- [ ] I will NOT write Go code or CI configs directly

---

## Objective
Initialize Go module, directory structure, and development tooling per `specs/repository-layout.md`.

## Files to Create
```
go.mod
go.sum
cmd/kora/main.go               # Minimal main with Cobra placeholder
internal/auth/.keep
internal/datasources/.keep
internal/models/.keep
internal/output/.keep
internal/config/.keep
pkg/clock/clock.go             # Time utilities (mockable)
pkg/logger/logger.go           # Structured logging (slog)
Makefile
.github/workflows/ci.yml
.github/dependabot.yml
.golangci.yml
.gitignore
```

---

## Task 1: Invoke golang-pro Agent

**MANDATORY**: Use the Task tool with this prompt:

```
Task(
  subagent_type="golang-pro",
  prompt="""
Initialize the Kora Go project with the following requirements:

1. Initialize Go module: github.com/dakaneye/kora (Go 1.23+)

2. Create directory structure:
   - cmd/kora/
   - internal/auth/, internal/datasources/, internal/models/, internal/output/, internal/config/
   - pkg/clock/, pkg/logger/
   - scripts/, configs/, docs/, tests/integration/, tests/testdata/

3. Create pkg/clock/clock.go:
   - Clock interface with Now() method
   - RealClock implementation
   - MockClock for testing
   - Include tests in clock_test.go

4. Create pkg/logger/logger.go:
   - Wrapper around slog
   - Redact() function for credential safety
   - Include tests in logger_test.go

5. Create cmd/kora/main.go:
   - Minimal Cobra setup with root and version commands
   - Version variables: version, commit, date

6. Create Makefile with targets:
   - build: Build to bin/kora with ldflags
   - test: Run unit tests with race detector
   - test-integration: Run integration tests
   - test-coverage: Coverage report
   - lint: Run golangci-lint
   - security-scan: Run gosec
   - install: Install to ~/.local/bin
   - clean: Remove artifacts

7. Create .golangci.yml:
   - Use v2 format
   - Enable: errcheck, govet, staticcheck, gosec, gocritic, revive
   - Configure for this project

8. Create .gitignore for Go project

Write all files with proper Go conventions and run go mod tidy.
"""
)
```

---

## Task 2: Invoke deployment-engineer Agent

**MANDATORY**: Use the Task tool with this prompt:

```
Task(
  subagent_type="deployment-engineer",
  prompt="""
Create GitHub Actions CI/CD configuration for the Kora Go project:

1. Create .github/workflows/ci.yml:
   - Trigger on push to main and pull requests
   - Run on macos-latest (this is a macOS-only tool)
   - Jobs: lint, test, build
   - Use Go 1.23
   - Use golangci-lint-action for linting
   - Upload coverage to codecov
   - Verify binary works with ./bin/kora version

2. Create .github/dependabot.yml:
   - Weekly updates for gomod
   - Weekly updates for github-actions
   - Assign to dakaneye
   - Use commit prefixes: deps:, ci:

Ensure workflows are production-ready and follow GitHub Actions best practices.
"""
)
```

---

## Definition of Done
- [ ] `go build ./...` succeeds
- [ ] `make lint` runs golangci-lint successfully
- [ ] `make test` runs and passes
- [ ] CI workflow file exists and is valid YAML
- [ ] Directory structure matches `specs/repository-layout.md`
- [ ] `./bin/kora version` outputs version info

## EFA Constraints
None - structural setup only.

## Next Session
Session 2: Implement Core Models (EFA 0001)
