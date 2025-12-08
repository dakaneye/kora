# Kora Memory Store: MCP Configuration

## Overview

Claude accesses Kora's memory store directly via SQLite MCP server. This enables Claude to:
- Query your goals, commitments, accomplishments
- Search context about people and projects
- Store new information as you share it

**Architecture**: Claude → SQLite MCP Server → `~/.kora/data/kora.db`

## Setup

### 1. Initialize Kora

```bash
kora init
```

Creates database at `~/.kora/data/kora.db` with full schema.

### 2. Get Database Path

```bash
kora db path
```

Returns: `/Users/yourusername/.kora/data/kora.db`

### 3. Configure SQLite MCP Server

#### Claude Code CLI

```bash
# Add MCP server (user scope - works across all projects)
claude mcp add kora-memory -- npx -y @modelcontextprotocol/server-sqlite "$(kora db path)"

# Or project scope (creates .mcp.json for team sharing)
claude mcp add kora-memory --scope project -- npx -y @modelcontextprotocol/server-sqlite "$(kora db path)"
```

Verify with:
```bash
claude mcp list
claude mcp get kora-memory
```

#### Claude Desktop

Add to `~/Library/Application Support/Claude/claude_desktop_config.json`:

```json
{
  "mcpServers": {
    "kora-memory": {
      "type": "stdio",
      "command": "npx",
      "args": [
        "-y",
        "@modelcontextprotocol/server-sqlite",
        "/Users/yourusername/.kora/data/kora.db"
      ]
    }
  }
}
```

**Replace** the path with output from `kora db path`.

### 4. Restart Claude

- **Claude Code CLI**: Start new session (`claude`)
- **Claude Desktop**: Quit and relaunch the app

## SQL Query Patterns

### INSERT Examples

#### Store a Goal
```sql
INSERT INTO goals (id, title, description, status, priority, target_date, created_at, updated_at)
VALUES (
    'goal_' || lower(hex(randomblob(8))),
    'Ship auth redesign',
    'Migrate to OAuth2 with better error handling',
    'active',
    2,
    '2025-12-31T23:59:59Z',
    datetime('now'),
    datetime('now')
);
```

#### Record a Commitment
```sql
INSERT INTO commitments (id, title, to_whom, status, due_date, created_at, updated_at)
VALUES (
    'commit_' || lower(hex(randomblob(8))),
    'Review Alice PR #456',
    'alice',
    'active',
    '2025-12-10T17:00:00Z',
    datetime('now'),
    datetime('now')
);
```

#### Log an Accomplishment
```sql
INSERT INTO accomplishments (id, title, description, impact, source_url, accomplished_at, created_at, updated_at)
VALUES (
    'acc_' || lower(hex(randomblob(8))),
    'Shipped secure rebuild pipeline',
    'Automated security scanning for Maven Central rebuilds',
    'Reduced rebuild time from 2h to 30m',
    'https://github.com/org/repo/pull/1234',
    '2025-12-06T14:30:00Z',
    datetime('now'),
    datetime('now')
);
```

#### Save Context
```sql
INSERT INTO context (id, entity_type, entity_id, title, body, urgency, created_at, updated_at)
VALUES (
    'ctx_' || lower(hex(randomblob(8))),
    'person',
    'alice',
    'Platform team lead',
    'Owns infrastructure. Key stakeholder for auth. Available Wed-Thu.',
    'normal',
    datetime('now'),
    datetime('now')
);
```

### SELECT Examples

#### Active Commitments Due Soon
```sql
SELECT title, to_whom, due_date
FROM commitments
WHERE status = 'active' AND is_deleted = 0
AND due_date < datetime('now', '+3 days')
ORDER BY due_date ASC;
```

#### High-Priority Active Goals
```sql
SELECT title, description, target_date
FROM goals
WHERE status = 'active' AND is_deleted = 0
AND priority <= 2
ORDER BY priority ASC, target_date ASC;
```

#### Recent Accomplishments
```sql
SELECT title, impact, accomplished_at
FROM accomplishments
WHERE is_deleted = 0
AND accomplished_at > datetime('now', '-30 days')
ORDER BY accomplished_at DESC;
```

#### Context About a Person
```sql
SELECT title, body, urgency
FROM context
WHERE entity_type = 'person' AND entity_id = 'alice'
AND is_deleted = 0
ORDER BY urgency DESC;
```

#### Full-Text Search
```sql
SELECT content, title, body
FROM memory_search
WHERE memory_search MATCH 'auth security'
LIMIT 10;
```

### UPDATE Examples

#### Complete a Commitment
```sql
UPDATE commitments
SET status = 'completed'
WHERE id = 'commit_abc123';
```

#### Mark Goal On Hold
```sql
UPDATE goals
SET status = 'on_hold'
WHERE id = 'goal_xyz789';
```

### Soft Delete

```sql
UPDATE goals
SET is_deleted = 1
WHERE id = 'goal_xyz789';
```

## FTS5 Search Syntax

Full-text search supports:

```sql
-- Phrase match
WHERE memory_search MATCH '"auth redesign"'

-- AND operator (implicit)
WHERE memory_search MATCH 'auth security'

-- OR operator
WHERE memory_search MATCH 'auth OR oauth'

-- NOT operator
WHERE memory_search MATCH 'auth NOT deprecated'

-- Column-specific
WHERE memory_search MATCH 'title:auth'

-- Prefix match
WHERE memory_search MATCH 'secur*'
```

## Schema Reference

| Table | Purpose | Key Columns |
|-------|---------|-------------|
| **goals** | Your objectives and work priorities | `status`, `priority`, `target_date` |
| **commitments** | Promises made to others | `to_whom`, `status`, `due_date` |
| **accomplishments** | What you shipped | `accomplished_at`, `impact`, `source_url` |
| **context** | Knowledge about people/projects | `entity_type`, `entity_id`, `urgency` |
| **memory_search** | FTS5 search across all tables | `content`, `title`, `body`, `tags` |

### Status Values

- **goals**: `active`, `completed`, `on_hold`
- **commitments**: `active`, `in_progress`, `completed`

### Entity Types (context)

- `person` - People you work with
- `project` - Active projects
- `repo` - Repositories
- `team` - Teams

### Priority (goals)

1-5 scale where 1 = highest priority, 5 = lowest

### Urgency (context)

- `critical` - Needs immediate attention
- `high` - Important context
- `normal` - Standard information

## Admin Commands

```bash
# Get database path
kora db path

# Show statistics (record counts, schema version)
kora db stats

# Verify database integrity and FTS consistency
kora db validate

# Backup database
kora db backup ~/backups/kora-$(date +%Y%m%d).db

# Export to JSON
kora db export > kora-export.json

# Prune soft-deleted records older than 30 days
kora db prune --older-than 30d
```

## Security

- Database file: `~/.kora/data/kora.db`
- Permissions: `0600` (user read/write only)
- Location: User home directory (local-first, no cloud sync)
- No credentials stored (auth handled by `kora digest` via `gh` CLI)

## Troubleshooting

### MCP Server Not Loading (Claude Code CLI)

```bash
# Verify server is registered
claude mcp list

# Check server details
claude mcp get kora-memory

# Remove and re-add if needed
claude mcp remove kora-memory
claude mcp add kora-memory -- npx -y @modelcontextprotocol/server-sqlite "$(kora db path)"
```

### MCP Server Not Found

```bash
# Test SQLite MCP server installation
npx -y @modelcontextprotocol/server-sqlite --help
```

### Database Not Found

```bash
# Initialize if missing
kora init

# Verify database exists
kora db path
ls -l ~/.kora/data/kora.db
```

### Permission Denied

```bash
# Fix permissions
chmod 0600 ~/.kora/data/kora.db
```

### Schema/Integrity Errors

```bash
# Verify schema and FTS consistency
kora db validate

# Check stats for schema version
kora db stats
```

## Example Claude Usage

**You:** "Remember that I committed to review Alice's auth PR by Friday"

**Claude:** *(executes SQL INSERT into commitments)*

**You:** "What are my high-priority goals?"

**Claude:** *(executes SQL SELECT from goals WHERE priority <= 2)*

**You:** "What do I know about Alice?"

**Claude:** *(executes SQL SELECT from context WHERE entity_id = 'alice')*

**You:** "What did I ship last week?"

**Claude:** *(executes SQL SELECT from accomplishments WHERE accomplished_at > ...)*

---

**Status:** Ready for implementation
**Last Updated:** 2025-12-07
