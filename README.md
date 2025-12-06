# Kora

A Go CLI that aggregates GitHub and Slack activity into a prioritized morning digest.

## What It Does

```bash
kora digest --since 16h --format terminal
```

Fetches overnight activity and outputs a prioritized list:
- GitHub PR review requests and mentions
- GitHub issue assignments and mentions
- Slack DMs and @mentions

## Status

**Not yet implemented.** See `specs/` for design documents.

## Planned Features (v1)

- **Datasources**: GitHub (via `gh` CLI), Slack (via user token)
- **Output formats**: Terminal (pretty), Markdown, JSON
- **Auth**: macOS Keychain for Slack, `gh` CLI delegation for GitHub
- **Platform**: macOS only

## Design Docs

| Document | Description |
|----------|-------------|
| `specs/repository-layout.md` | Repository structure and v1 scope |
| `specs/prototype/README.md` | Original vision and datasource priority |
| `specs/prototype/chatgpt-feasability.txt` | Architecture brainstorm |
| `specs/prototype/hooks.md` | Claude Code accomplishment tracking |

## Claude Integration

Kora is designed to be invoked by Claude:

```bash
# Direct invocation
kora digest --format json

# Or as an MCP tool (after setup)
# Returns JSON for Claude to summarize
```

## Requirements

- Go 1.25+
- macOS (Keychain integration)
- `gh` CLI authenticated (`gh auth login`)
- Slack user token (xoxp-*) in Keychain or env

## License

Private - personal use only
