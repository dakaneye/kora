# Kora Implementation Punchlist

> 10 development sessions to build the Kora CLI tool from scratch.

## Session Overview

| Session | Focus | Primary Agents | Dependencies |
|---------|-------|----------------|--------------|
| [1](session-1-punchlist.md) | Project bootstrap | `golang-pro`, `deployment-engineer` | None |
| [2](session-2-punchlist.md) | Core models (EFA 0001) | `golang-pro`, `test-automator` | 1 |
| [3](session-3-punchlist.md) | Auth providers (EFA 0002) | `golang-pro`, `security-auditor` | 2 |
| [4](session-4-punchlist.md) | DataSource interface (EFA 0003) | `golang-pro`, `test-automator` | 3 |
| [5](session-5-punchlist.md) | GitHub datasource | `golang-pro`, `security-auditor` | 4 |
| [6](session-6-punchlist.md) | Slack datasource | `golang-pro`, `security-auditor` | 4 |
| [7](session-7-punchlist.md) | Output formatters | `golang-pro`, `test-automator` | 2 |
| [8](session-8-punchlist.md) | CLI commands | `golang-pro` | 4-7 |
| [9](session-9-punchlist.md) | Integration tests + security | `test-automator`, `security-auditor`, `deployment-engineer` | 8 |
| [10](session-10-punchlist.md) | Documentation + release | `documentation-engineer`, `bash-pro`, `code-reviewer` | 9 |

## Dependency Graph

```
Session 1: Bootstrap
    │
    ▼
Session 2: Models (EFA 0001)
    │
    ├───────────────────────────┐
    ▼                           ▼
Session 3: Auth (EFA 0002)    Session 7: Formatters
    │
    ▼
Session 4: DataSource (EFA 0003)
    │
    ├─────────────┐
    ▼             ▼
Session 5:    Session 6:
GitHub DS     Slack DS
    │             │
    └──────┬──────┘
           │
           ▼
    Session 8: CLI
           │
           ▼
    Session 9: Integration + Security
           │
           ▼
    Session 10: Docs + Release
```

## Agent Summary

| Agent | Sessions Used | Primary Responsibilities |
|-------|---------------|--------------------------|
| `golang-pro` | 1-8 | Go implementation, interfaces, concurrency |
| `test-automator` | 2, 4, 7, 9 | Unit tests, integration tests, mocks |
| `security-auditor` | 3, 5, 6, 9 | Credential handling, security review |
| `deployment-engineer` | 1, 9 | CI/CD workflows, GitHub Actions |
| `documentation-engineer` | 10 | User docs, architecture docs |
| `bash-pro` | 10 | Shell scripts, MCP install |
| `code-reviewer` | 10 | Final codebase review |

## EFA Governance

Sessions 2-4 implement EFA-governed code:

| EFA | Session | Governs |
|-----|---------|---------|
| [0001](../efas/0001-event-model.md) | 2 | Event model, validation, metadata |
| [0002](../efas/0002-auth-provider.md) | 3 | Auth providers, credential security |
| [0003](../efas/0003-datasource-interface.md) | 4 | DataSource interface, concurrency |

**Before modifying EFA-governed code**: Read the EFA, propose changes, get approval.

## Parallel Work Opportunities

Sessions 5 & 6 (GitHub and Slack datasources) can be worked on in parallel after Session 4 is complete.

Session 7 (Formatters) only depends on Session 2 and can be worked on in parallel with Sessions 3-6.

## Success Criteria

When all sessions are complete:

- [ ] `kora digest --since 16h --format terminal` produces prioritized digest
- [ ] GitHub datasource fetches PRs, issues, mentions
- [ ] Slack datasource fetches DMs and @mentions
- [ ] All three output formats work (terminal, markdown, JSON)
- [ ] No credentials logged anywhere
- [ ] All tests pass: unit, integration, security
- [ ] Test coverage >80%
- [ ] CI validates on every commit
- [ ] All EFAs implemented correctly
- [ ] Documentation complete

## How to Use This Punchlist

1. Start with Session 1, complete all tasks
2. Move to Session 2 when Session 1 is done
3. For each session:
   - Read the punchlist file
   - Use the specified agents
   - Complete all tasks
   - Verify "Definition of Done" checklist
   - Move to next session

Run `/punchlist` to work through sessions interactively with iterative refinement.

---

**Created**: 2025-12-06
**Status**: Ready for execution
