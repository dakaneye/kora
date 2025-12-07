# Kora

CLI tool that aggregates GitHub and Slack activity into a prioritized morning digest.

## Features

- **GitHub**: PR review requests, PR mentions, issue mentions, issue assignments
- **Slack**: Direct messages, @mentions in channels (requires enterprise workspace)
- **Output formats**: JSON, JSON (pretty-printed), text
- **Authentication**: GitHub via `gh` CLI delegation, Slack via macOS Keychain
- **Concurrent fetching**: Datasources run in parallel with graceful failure handling
- **EFA governance**: AI-friendly architecture with explicit ground truth specifications

## Installation

```bash
make build
make install
```

Installs to `~/.local/bin/kora`. Ensure `~/.local/bin` is in your PATH.

### MCP Integration (Claude Code)

Install Kora as an MCP tool so Claude Code can invoke it directly:

```bash
make install-mcp
```

This creates a project-scoped `.mcp.json` configuration. Claude Code can then invoke kora digest as a tool:

```
You: "What's in my digest?"
Claude: *invokes kora-digest tool* "You have 3 PRs requiring review..."
```

The MCP tool uses `--format json` for structured output and defaults to a 16-hour window. Customize by editing `.mcp.json`.

## Quick Start

### 1. Authenticate GitHub

```bash
gh auth login
```

Kora uses `gh` CLI delegation, never sees your GitHub token.

### 2. Configure Slack (Optional)

Slack integration requires enterprise workspace approval. Most users won't have access.

If you have a Slack user token (`xoxp-*`):

```bash
# Store in macOS Keychain (recommended)
security add-generic-password -s "kora" -a "slack-token" -w "xoxp-..."

# Or set environment variable (less secure)
export KORA_SLACK_TOKEN="xoxp-..."
```

### 3. Run Digest

```bash
# Fetch overnight activity (16 hour window)
kora digest --since 16h

# Fetch last 8 hours
kora digest --since 8h

# Output formats
kora digest --since 16h --format json
kora digest --since 16h --format json-pretty
kora digest --since 16h --format text  # default
```

## Configuration

Create `~/.kora/config.yaml` to customize behavior:

```yaml
datasources:
  github:
    enabled: true
    orgs:
      - my-org
  slack:
    enabled: true

digest:
  window: 16h
  timezone: Local
  format: text

security:
  redact_credentials: true
  datasource_timeout: 30s
```

See `configs/kora.yaml.example` for full configuration options.

## Usage

```bash
# Basic digest with default window (16h)
kora digest

# Custom time window
kora digest --since 8h
kora digest --since 24h

# Specific output format
kora digest --format json
kora digest --format json-pretty
kora digest --format text

# Custom config file
kora digest --config ~/custom-kora.yaml

# Version info
kora version
```

## Output Formats

### text (default)
Human-readable text format optimized for terminal display:
```
Work Digest (16 hours)
3 events requiring action

[PRIORITY 2 - HIGH] Review requested: Add secure rebuild for core-java
  Source: github
  URL: https://github.com/org/repo/pull/123
  Author: janedev
  Time: 2025-12-06 07:00:00
  Requires action: yes
...
```

### json
Compact JSON for programmatic consumption:
```json
{"events":[{"type":"pr_review","title":"Review requested: Add secure rebuild","source":"github",...}],"stats":{...}}
```

### json-pretty
Indented JSON for readability:
```json
{
  "events": [
    {
      "type": "pr_review",
      "title": "Review requested: Add secure rebuild",
      ...
    }
  ],
  ...
}
```

## Development

### Build

```bash
make build        # Build binary to bin/kora
make install      # Build and install to ~/.local/bin
```

### Test

```bash
make test                  # Unit tests
make test-integration      # Integration tests (requires auth)
make test-coverage         # Coverage report
```

### Lint

```bash
make lint          # golangci-lint + gosec
make security-scan # gosec only
```

### Clean

```bash
make clean         # Remove build artifacts
```

## Project Structure

```
cmd/kora/           # CLI entry point
internal/
  auth/             # Authentication providers (GitHub gh CLI, Slack Keychain)
  datasources/      # Event fetching (GitHub, Slack)
  models/           # Core Event model
  output/           # Formatters (JSON, text)
  config/           # YAML configuration
pkg/                # Shared utilities
specs/
  efas/             # Explainer For Agents (ground truth specs)
tests/integration/  # Integration tests
```

## Documentation

- `docs/architecture.md` - System architecture and data flow
- `docs/datasources.md` - How to add new datasources
- `specs/efas/` - Ground truth specifications for AI agents
- `SECURITY.md` - Security policies and credential handling

## Requirements

- Go 1.25+
- macOS (Keychain integration)
- `gh` CLI authenticated (`gh auth login`)
- Slack user token (`xoxp-*`) for Slack integration (optional)

## Security

- GitHub auth: CLI delegation via `gh` - Kora never sees tokens
- Slack auth: macOS Keychain with env var fallback
- Credentials always redacted in logs
- TLS verification always enabled
- See `SECURITY.md` for details

## License

Private - personal use only
