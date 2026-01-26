# Kora + Claude Code Integration Guide

This guide explains how to use Kora's memory store with Claude Code for personal productivity workflows.

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                      Claude Code                            │
├─────────────────────────────────────────────────────────────┤
│  Commands (User-triggered)     Skills (Auto-discovered)     │
│  ├─ /performance-review-seeder ├─ github-harvester          │
│  ├─ /github-harvest            ├─ standup-generator         │
│  ├─ /standup                   └─ quarterly-review-writer   │
│  ├─ /quarterly-review                                       │
│  └─ /memory                                                 │
├─────────────────────────────────────────────────────────────┤
│  Agent: kora-memory-manager (delegated DB operations)       │
├─────────────────────────────────────────────────────────────┤
│  MCP: SQLite server → ~/.kora/data/kora.db                  │
└─────────────────────────────────────────────────────────────┘
                              ↓
┌─────────────────────────────────────────────────────────────┐
│                    Kora CLI (Data Layer)                    │
├─────────────────────────────────────────────────────────────┤
│  kora init          Initialize database                     │
│  kora db stats      Show statistics                         │
│  kora db validate   Check integrity                         │
│  kora db import     Import JSON data                        │
│  kora db export     Export data                             │
│  kora db backup     Create backup                           │
│  kora digest        Fetch GitHub activity                   │
└─────────────────────────────────────────────────────────────┘
```

## Setup

### 1. Initialize Kora

```bash
# Build and install
cd ~/dev/personal/kora
make build
cp ./bin/kora /usr/local/bin/

# Initialize memory store
kora init
# Creates ~/.kora/data/kora.db
```

### 2. Configure SQLite MCP Server

Add to `~/.claude/settings.json`:

```json
{
  "mcpServers": {
    "sqlite": {
      "command": "npx",
      "args": ["-y", "@anthropic/mcp-server-sqlite", "--db-path", "/Users/samueldacanay/.kora/data/kora.db"]
    }
  }
}
```

Or use the Claude command:
```
/mcp add sqlite npx -y @anthropic/mcp-server-sqlite --db-path ~/.kora/data/kora.db
```

### 3. Verify Setup

```bash
kora db stats      # Check database exists
kora db validate   # Verify integrity
```

## Workflows

### Daily: Generate Standup

```
/standup
```

Or with options:
```
/standup --format slack
/standup --date 2025-12-07
```

Claude will:
1. Query recent accomplishments from memory store
2. Query active goals and upcoming commitments
3. Fetch GitHub activity via `kora digest`
4. Synthesize into standup format

### Weekly: Harvest GitHub Activity

```
/github-harvest --since 7d
```

Or let Claude auto-discover:
> "Add my PRs from last week to my accomplishments"

Claude will:
1. Query merged PRs and closed issues
2. Transform into accomplishment records
3. Insert into memory store with source URLs

### Quarterly: Seed from Performance Reviews

```
/performance-review-seeder
```

Claude will:
1. Read documents from `~/Downloads/performancereviews/`
2. Extract accomplishments, goals, feedback
3. Generate JSON for review
4. Import via `kora db import`

### Quarterly: Write Performance Review

```
/quarterly-review --period Q4 --year 2025
```

Or:
> "Help me write my Q4 self-assessment"

Claude will:
1. Query accomplishments for the quarter
2. Query goal status and commitments
3. Query manager feedback and growth areas
4. Synthesize into review document

## Commands Reference

| Command | Purpose | Arguments |
|---------|---------|-----------|
| `/memory` | Quick memory access | `search`, `goals`, `stats` |
| `/standup` | Generate standup | `--date`, `--format` |
| `/github-harvest` | Import GitHub work | `--since`, `--repo`, `--dry-run` |
| `/performance-review-seeder` | Seed from reviews | `--path`, `--dry-run` |
| `/quarterly-review` | Write review doc | `--period`, `--year`, `--type` |

## Skills Reference

| Skill | Auto-triggers on |
|-------|-----------------|
| `github-harvester` | "Add my GitHub work", "Import my PRs" |
| `standup-generator` | "Generate my standup", "What did I do?" |
| `quarterly-review-writer` | "Help write my review", "Prepare self-assessment" |

## Memory Store Schema

### Tables

| Table | Purpose | Key Fields |
|-------|---------|------------|
| `goals` | Objectives | title, status, priority, target_date |
| `accomplishments` | Completed work | title, impact, source_url, accomplished_at |
| `commitments` | Promises made | title, to_whom, due_date, status |
| `context` | Knowledge base | entity_type, entity_id, body, urgency |
| `memory_search` | FTS5 search | content, title, body, tags |

### Common Queries

```sql
-- Active goals by priority
SELECT * FROM goals WHERE status = 'active' AND is_deleted = 0
ORDER BY priority;

-- Recent accomplishments
SELECT * FROM accomplishments WHERE is_deleted = 0
AND accomplished_at > datetime('now', '-30 days');

-- Upcoming commitments
SELECT * FROM commitments WHERE status = 'active' AND is_deleted = 0
AND due_date < datetime('now', '+7 days');

-- Full-text search
SELECT * FROM memory_search WHERE memory_search MATCH 'authentication';
```

## Data Flow

### Performance Reviews → Memory Store

```
PDF/TXT Documents
      ↓
/performance-review-seeder
      ↓
Claude extracts structured data
      ↓
JSON file (user reviews)
      ↓
kora db import --dry-run (validate)
      ↓
kora db import (commit)
      ↓
Memory Store
```

### GitHub → Memory Store → Standup

```
GitHub (PRs, Issues)
      ↓
kora digest --format json
      ↓
/github-harvest transforms to accomplishments
      ↓
Memory Store (accomplishments)
      ↓
/standup queries + synthesizes
      ↓
Markdown Standup
```

### Memory Store → Quarterly Review

```
Memory Store
├── accomplishments (Q4 filter)
├── goals (status changes)
├── commitments (met/missed)
└── context (feedback)
      ↓
/quarterly-review synthesizes
      ↓
Review Document
```

## EFA 0004 Compliance

This integration follows the Tool Responsibility Separation principle:

| Layer | Responsibility |
|-------|---------------|
| **Kora** | Schema owner, admin commands, data validation |
| **Claude** | Intelligence, extraction, synthesis, relevance |
| **User** | Oversight, approval before imports, review artifacts |

**Key rules:**
- Claude uses SQLite MCP for CRUD, not Kora wrappers
- Schema changes go through Kora migrations
- User reviews JSON before `kora db import`
- Soft deletes only (`is_deleted = 1`)

## Files Reference

### Claude Code Configuration

```
~/.claude/
├── settings.json                    # MCP server config
├── agents/
│   └── kora-memory-manager.md       # DB operations agent
├── commands/
│   ├── memory.md                    # Quick access
│   ├── standup.md                   # Daily standup
│   ├── github-harvest.md            # Import GitHub
│   ├── performance-review-seeder.md # Seed from reviews
│   └── quarterly-review.md          # Write reviews
└── skills/
    ├── kora-memory/SKILL.md         # Schema reference
    ├── github-harvester/SKILL.md    # Auto-harvest
    ├── standup-generator/SKILL.md   # Auto-standup
    └── quarterly-review-writer/
        ├── SKILL.md                 # Auto-review
        └── TEMPLATES.md             # Review templates
```

### Kora Project

```
~/dev/personal/kora/
├── cmd/kora/
│   ├── db.go                        # Database commands
│   └── init.go                      # Initialize command
├── internal/storage/
│   ├── schema.sql                   # Database schema
│   └── store.go                     # Storage layer
├── specs/
│   ├── memory-seeding.md            # Import spec
│   ├── standup-generation.md        # Standup spec
│   └── efas/0004-tool-responsibility.md
└── docs/
    └── claude-integration.md        # This file
```

## Troubleshooting

### Database locked
```
Error: database is locked
```
Close other SQLite connections or restart Claude Code MCP server.

### FTS out of sync
```bash
kora db validate --rebuild-fts
```

### Schema mismatch
```bash
kora db validate  # Check version
kora migrate      # Apply pending migrations
```

### Import validation failed
```bash
kora db import --dry-run seed.json  # See errors
```

## Quick Start Checklist

- [ ] `kora init` - Initialize database
- [ ] Add SQLite MCP to `~/.claude/settings.json`
- [ ] Restart Claude Code
- [ ] `/memory stats` - Verify connection
- [ ] `/performance-review-seeder` - Seed historical data
- [ ] `/github-harvest --since 30d` - Import recent work
- [ ] `/standup` - Generate first standup
