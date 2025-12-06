# Session 10: Documentation and Release Preparation

## Agent Requirements (MANDATORY)

**YOU MUST invoke the following agents using the Task tool. Do NOT implement directly.**

| Agent | Invoke For | Task Tool Call |
|-------|------------|----------------|
| `documentation-engineer` | All documentation | `Task(subagent_type="documentation-engineer", prompt="...")` |
| `bash-pro` | MCP install script | `Task(subagent_type="bash-pro", prompt="...")` |
| `code-reviewer` | Final codebase review | `Task(subagent_type="code-reviewer", prompt="...")` |

### Pre-Flight Checklist
- [ ] I will use Task tool to invoke `documentation-engineer` for docs
- [ ] I will use Task tool to invoke `bash-pro` for scripts
- [ ] I will use Task tool to invoke `code-reviewer` for final review
- [ ] I will NOT write documentation or scripts directly

---

## Objective
Complete user documentation, CI hardening, release configuration.

## Dependencies
- Session 9 complete (all tests passing, security audit done)

## Files to Create
```
README.md                           # User-facing documentation
SECURITY.md                         # Security policy
docs/architecture.md                # System design, EFA references
docs/datasources.md                 # How to add datasources
scripts/install-mcp.sh              # Install as MCP tool for Claude
```

---

## Task 1: Invoke documentation-engineer Agent

**MANDATORY**: Use the Task tool with this prompt:

```
Task(
  subagent_type="documentation-engineer",
  prompt="""
Create comprehensive documentation for Kora.

1. README.md:
   - Project description: Morning digest aggregator for GitHub and Slack
   - Features list
   - Installation: make build, make install
   - Quick Start:
     - GitHub auth: gh auth login
     - Slack auth: keychain or KORA_SLACK_TOKEN
     - Run: kora digest --since 16h
   - Configuration: ~/.kora/config.yaml with example
   - Usage examples for all flags and formats
   - Security summary (link to SECURITY.md)
   - Development: make targets
   - Link to architecture docs

2. SECURITY.md:
   - Vulnerability reporting process
   - Credential handling policy:
     - GitHub: delegation via gh CLI, never stored
     - Slack: macOS Keychain, env var fallback
   - Logging policy: all credentials redacted
   - Network security: TLS 1.2+, timeouts
   - Supply chain: pinned deps, dependabot, scanning

3. docs/architecture.md:
   - System overview with ASCII diagram
   - Package structure explanation
   - EFA governance explanation with links to EFAs
   - Data flow: Auth -> DataSource -> Event -> Formatter
   - Security architecture
   - Extension points

4. docs/datasources.md:
   - How to add a new datasource
   - DataSource interface reference
   - EFA constraints to follow
   - Step-by-step guide:
     1. Update EFA 0001 for new EventType/Source
     2. Create auth provider (EFA 0002)
     3. Implement DataSource (EFA 0003)
     4. Add tests
     5. Wire into CLI
   - Link to existing implementations as examples

Keep documentation concise and technical. No marketing fluff.
"""
)
```

---

## Task 2: Invoke bash-pro Agent

**MANDATORY**: Use the Task tool with this prompt:

```
Task(
  subagent_type="bash-pro",
  prompt="""
Create MCP installation script for Kora.

scripts/install-mcp.sh:

Purpose: Install Kora as an MCP tool for Claude Code.

Requirements:
1. Check kora binary exists at ~/.local/bin/kora
2. Create MCP tools directory: ~/.claude/mcp-tools/
3. Create kora.json MCP tool definition:
   {
     "name": "kora",
     "description": "Get morning digest of GitHub PRs, issues, and Slack messages",
     "command": "$HOME/.local/bin/kora",
     "args": ["digest", "--format", "json"],
     "timeout": 30000
   }
4. Print success message with usage example

Script requirements:
- Use set -euo pipefail
- Check prerequisites before proceeding
- Provide helpful error messages
- Make idempotent (safe to run multiple times)
- Add to Makefile as install-mcp target

Also update Makefile to add:
- install-mcp: ./scripts/install-mcp.sh target
"""
)
```

---

## Task 3: Invoke code-reviewer Agent

**MANDATORY**: Final review of entire codebase:

```
Task(
  subagent_type="code-reviewer",
  prompt="""
Final code review of Kora codebase before release.

Review all packages:
- cmd/kora/
- internal/auth/
- internal/config/
- internal/datasources/
- internal/models/
- internal/output/
- pkg/clock/
- pkg/logger/

Check for:

1. Code Quality:
   - Dead code (unused functions, variables)
   - TODO comments that should be resolved
   - Inconsistent naming
   - Missing error handling

2. Documentation:
   - Package comments present
   - Exported functions documented
   - EFA protection comments in governed files

3. Security:
   - Any credential handling issues missed
   - Input validation coverage
   - Error message safety

4. Testing:
   - Test coverage gaps
   - Missing edge cases
   - Test quality

5. EFA Compliance:
   - internal/models/ follows EFA 0001
   - internal/auth/ follows EFA 0002
   - internal/datasources/ follows EFA 0003
   - Protection comments present

Report format:
- Category
- File:Line
- Issue description
- Severity: HIGH/MEDIUM/LOW
- Recommended action

No MEDIUM or higher issues should remain for release.
"""
)
```

---

## Final Verification

After all agents complete, manually verify:

```bash
# Build
make build

# Run all checks
make lint
make test
make security-scan

# Manual test
./bin/kora version
./bin/kora digest --help
./bin/kora digest --since 16h --format terminal
./bin/kora digest --since 16h --format json | jq .
./bin/kora digest --since 16h --format markdown

# Install MCP (optional)
./scripts/install-mcp.sh
```

---

## Definition of Done
- [ ] README.md complete with installation, usage, examples
- [ ] SECURITY.md documents credential handling
- [ ] docs/architecture.md explains system design
- [ ] docs/datasources.md guides new datasource creation
- [ ] scripts/install-mcp.sh works
- [ ] Code review passes (no MEDIUM+ issues)
- [ ] All make targets work
- [ ] Manual verification complete

## Success Criteria (Full Project)

When all sessions are complete:

- [ ] `kora digest --since 16h --format terminal` produces prioritized digest
- [ ] GitHub datasource fetches PRs, issues, mentions
- [ ] Slack datasource fetches DMs and @mentions
- [ ] All three output formats work
- [ ] No credentials logged anywhere
- [ ] All tests pass: unit, integration, security
- [ ] Test coverage >80%
- [ ] CI validates on every commit
- [ ] All EFAs (0001, 0002, 0003) implemented correctly
- [ ] Documentation complete
