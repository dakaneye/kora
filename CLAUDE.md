# Kora - Claude Development Guide

## Project Overview

Kora is a Go CLI tool that aggregates work updates from GitHub and Slack into a prioritized morning digest. Claude can invoke it directly via Bash or as an MCP tool.

**Specs**: `specs/prototype/` contains the original design docs. `specs/repository-layout.md` defines v1 scope.

## EFA (Explainer For Agents) Documents

EFAs are formal specifications that define ground truth for AI agents working on this codebase. They prevent drift where Claude reinvents formulas or data structures with each iteration.

### Working with EFAs

**Before modifying code**, check for EFA governance:

1. **Look for EFA references** in file headers:
   ```go
   // Ground truth defined in specs/efas/0001-event-model.md
   // IT IS FORBIDDEN TO CHANGE without updating EFA 0001
   ```

2. **Check `specs/efas/` directory** for relevant specifications

3. **Read the EFA's "AI Agent Rules" section** for explicit constraints

### Known EFAs

| EFA | File | Governs |
|-----|------|---------|
| 0001 | `specs/efas/0001-event-model.md` | Event struct, validation, metadata keys, priority rules |
| 0002 | `specs/efas/0002-auth-provider.md` | Auth provider interface, credential security, CLI delegation |
| 0003 | `specs/efas/0003-datasource-interface.md` | DataSource interface, concurrent fetching, partial success |

### Protected Code Detection

When you encounter comments like:
- "IT IS FORBIDDEN TO CHANGE without updating EFA"
- "Ground truth defined in specs/efas/"
- "See EFA 0001 for specification"
- "Claude MUST stop and ask before modifying"

**STOP and ask the user** before making changes. These modules require EFA review first.

### Proposing EFA Changes

If an EFA needs updating:

1. **Explain why** the EFA should change
2. **Show the proposed change** to the EFA document
3. **Wait for approval** before modifying code
4. **Update the EFA** before or alongside code changes
5. **Update AI Agent Rules** in the EFA to reflect new constraints

### Creating New EFAs

All EFAs MUST be authored collaboratively using:
- `golang-pro` agent (implementation feasibility)
- `documentation-engineer` agent (structure and clarity)
- `prompt-engineer` agent (AI Agent Rules)

See `specs/efas/AUTHORING.md` for complete guidelines.

## Core Concepts

Reference these concepts from `~/.claude/concepts/` as needed:

| Concept | Path | When to Apply |
|---------|------|---------------|
| Radical Candor | `~/.claude/concepts/truth/principal0.md` | Always - absolute truthfulness, no simulated functionality |
| Truth-Focused | `~/.claude/concepts/truth/truth-focused.md` | Feedback, reviews - direct, fact-driven communication |
| Hermeneutic Circle | `~/.claude/concepts/circle.md` | Complex problems - understand parts through whole, whole through parts |
| Planning | `~/.claude/concepts/planning.md` | Feature breakdown, implementation sequencing |
| Code Review | `~/.claude/concepts/code-review.md` | PR reviews, architecture decisions |
| Refactoring | `~/.claude/concepts/refactoring.md` | Code improvement without behavior change |

**Truth principles apply universally**: Never simulate functionality, never create illusions of working code, fail by telling the truth.

## Recommended Agents

Prefer using the appropriate agent for specialized tasks. Agents run in independent contexts with domain expertise.

| Task | Agent | When to Use |
|------|-------|-------------|
| Go implementation | `golang-pro` | Writing idiomatic Go, concurrency patterns, interfaces |
| Security review | `security-auditor` | Auth code, credential handling, API clients |
| Code review | `code-reviewer` | PR reviews, architecture decisions |
| Test creation | `test-automator` | Unit tests, integration tests, mocks |
| Shell scripts | `bash-pro` | MCP install script, CI helpers |
| Planning | `project-task-planner` | Breaking down features, implementation order |
| Refactoring | `code-refactorer` | Cleaning up code without changing behavior |
| Documentation | `documentation-engineer` | Creating/updating EFAs, API docs |

## Code Standards

### Go Conventions
- Follow `@~/.claude/docs/go.md` for idiomatic Go
- Use `context.Context` as first parameter
- Functional options for configuration
- `errgroup` for concurrent datasource fetching
- Interfaces in consumer packages, not producer
- Table-driven tests

### Project-Specific Rules

1. **No credential storage in code** - Use `gh` CLI delegation or macOS Keychain
2. **Redact all credentials** - Never log tokens, even partially
3. **Fail gracefully** - One datasource failure shouldn't crash the digest
4. **In-memory only** - v1 has no persistence layer
5. **macOS only** - Don't add cross-platform abstractions yet

### Error Handling
```go
// Always wrap errors with context
if err := client.Fetch(ctx); err != nil {
    return fmt.Errorf("fetching github PRs: %w", err)
}

// Use custom error types for auth failures
var ErrNotAuthenticated = errors.New("not authenticated")
```

### Testing
- Unit tests with mocked HTTP responses (use `testdata/`)
- Integration tests for auth providers (tagged `//go:build integration`)
- Target >80% coverage
- No skipped tests - fix or remove them

## Directory Structure

```
cmd/kora/           # CLI entry point
internal/
  auth/             # Auth providers - EFA 0002 governed
  datasources/      # GitHub, Slack fetching - EFA 0003 governed
  models/           # Event, Person, Priority - EFA 0001 governed
  output/           # Formatters (terminal, markdown, JSON)
  config/           # YAML config loading
pkg/                # Shared utilities (clock, logger)
specs/
  efas/             # Explainer For Agents (ground truth)
    AUTHORING.md    # How to write EFAs
    0001-event-model.md
    0002-auth-provider.md
    0003-datasource-interface.md
tests/integration/  # Auth integration tests
```

## Commands Reference

**Prefer using `~/.claude/commands/*` for common workflows.** Commands are thin wrappers that invoke the right agents and concepts with proper configuration.

| Command | Purpose | When to Use |
|---------|---------|-------------|
| `/specify` | Plan new features with deep analysis | Starting a new feature, unclear requirements |
| `/punchlist` | Execute ordered task lists | Multi-step implementations, iterative work |
| `/refactor` | Improve code structure | Code cleanup without behavior change |
| `/create-pr` | Generate PR summaries | Before merging, PR descriptions |
| `/review-code` | Comprehensive code review | PR reviews, architecture decisions |
| `/hermeneutic` | Apply hermeneutic circle analysis | Complex problems requiring contextual understanding |
| `/migrate` | Plan technology stack migrations | Moving to new tools/frameworks |
| `/fix-gas` | Fix GitHub Advanced Security alerts | Security vulnerability remediation |

See `~/.claude/commands/README.md` for full command documentation.

## Implementation Priorities

1. **Auth layer first** - GitHub (`gh` CLI), Slack (Keychain)
2. **Models second** - Event interface, Person struct
3. **Datasources third** - GitHub PRs/Issues, Slack DMs/mentions
4. **CLI fourth** - Cobra commands, output formatting
5. **Tests throughout** - Don't defer testing

## Security Checklist

Before merging auth-related code:
- [ ] No credentials in logs (even debug level)
- [ ] Keychain operations handle "not found" gracefully
- [ ] TLS verification enabled for all HTTP clients
- [ ] Timeouts set on all network operations
- [ ] Input validation on config values

## CI Requirements

- Go 1.25+ (use latest)
- All tests pass on macOS
- golangci-lint clean
- gosec finds no high/critical issues
- No new dependencies without justification

## Quick Reference

```bash
# Build
make build

# Test
make test
make test-integration  # requires auth setup

# Lint
make lint

# Security scan
make security-scan

# Run digest
./bin/kora digest --since 16h --format terminal
```
