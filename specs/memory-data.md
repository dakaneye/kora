# Kora Memory Store: SQLite Schema

## Purpose

Claude needs to understand your context as a personal assistant. The memory store captures the essential information Claude needs to:

1. **Understand what you've accomplished** - What you shipped, resolved, learned
2. **Know what you've committed to** - Promises made to others, deadlines
3. **Track your goals** - What you're working toward, priorities
4. **Have context about people and projects** - Who matters, what's important

This is NOT for storing Kora's event stream. This is for Claude's understanding of YOU.

## Design Approach

- **YAGNI** - Only tables that Claude actually needs to be useful as a personal assistant
- **Simple Structure** - Straightforward queries, no complex joins
- **Soft Deletes** - History matters; preserve everything
- **Audit Trail** - Know when things changed
- **Searchable** - Find context quickly with FTS5

## Core Tables

### goals

Your objectives and work priorities.

```sql
CREATE TABLE goals (
    id TEXT PRIMARY KEY,
    title TEXT NOT NULL,           -- "Ship auth redesign", "Reduce build time 50%"
    description TEXT,              -- Why this matters
    status TEXT DEFAULT 'active',  -- "active", "completed", "on_hold"
    priority INTEGER DEFAULT 3,    -- 1-5, lower is higher priority
    target_date TEXT,              -- RFC3339 when you want it done
    tags TEXT,                     -- JSON array: ["q4", "priority", "project:auth"]
    is_deleted INTEGER DEFAULT 0,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE INDEX idx_goals_status ON goals(status) WHERE is_deleted = 0;
CREATE INDEX idx_goals_priority ON goals(priority) WHERE is_deleted = 0;
```

### commitments

What you promised to do and to whom.

```sql
CREATE TABLE commitments (
    id TEXT PRIMARY KEY,
    title TEXT NOT NULL,           -- "Review auth PRs", "Ship dashboard by Friday"
    to_whom TEXT,                  -- "alice", "platform-team", or null for self
    status TEXT DEFAULT 'active',  -- "active", "in_progress", "completed"
    due_date TEXT NOT NULL,        -- RFC3339 deadline
    tags TEXT,                     -- JSON array
    is_deleted INTEGER DEFAULT 0,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE INDEX idx_commitments_status ON commitments(status) WHERE is_deleted = 0;
CREATE INDEX idx_commitments_due_date ON commitments(due_date) WHERE is_deleted = 0;
```

### accomplishments

What you shipped, resolved, and achieved. For retrospectives and Claude understanding your impact.

```sql
CREATE TABLE accomplishments (
    id TEXT PRIMARY KEY,
    title TEXT NOT NULL,           -- "Shipped secure rebuild for core-java"
    description TEXT,              -- Full context of what/how/impact
    impact TEXT,                   -- "Reduced build time from 15m to 8m", "Unblocked 3 teams"
    source_url TEXT,               -- GitHub PR, commit, issue link
    accomplished_at TEXT NOT NULL, -- RFC3339 when it happened
    tags TEXT,                     -- JSON array: ["shipped", "security", "product"]
    is_deleted INTEGER DEFAULT 0,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE INDEX idx_accomplishments_date ON accomplishments(accomplished_at);
```

### context

Knowledge about people, projects, and areas. Things Claude should know to be a better assistant.

```sql
CREATE TABLE context (
    id TEXT PRIMARY KEY,
    entity_type TEXT NOT NULL,     -- "person", "project", "repo", "team"
    entity_id TEXT NOT NULL,       -- username, project name, repo name, etc
    title TEXT NOT NULL,           -- Brief label
    body TEXT NOT NULL,            -- What Claude should know (markdown ok)
    urgency TEXT,                  -- "critical", "high", "normal" (for important stuff)
    source_url TEXT,               -- Where this came from (GitHub, etc)
    tags TEXT,                     -- JSON array
    is_deleted INTEGER DEFAULT 0,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE INDEX idx_context_entity ON context(entity_type, entity_id) WHERE is_deleted = 0;
CREATE INDEX idx_context_urgency ON context(urgency) WHERE is_deleted = 0;
```

### memory_search

Full-text search across all memory.

```sql
CREATE VIRTUAL TABLE memory_search USING fts5(
    content,                       -- "goal", "commitment", "accomplishment", "context"
    title,
    body,
    tags
);
```

## Example Data

```sql
-- A goal you're working on
INSERT INTO goals (id, title, description, status, priority, target_date, created_at, updated_at)
VALUES (
    'goal_1',
    'Ship customer insights module',
    'Enable customers to understand how they use our platform',
    'active',
    2,
    '2025-12-31T23:59:59Z',
    datetime('now'),
    datetime('now')
);

-- A commitment to someone else
INSERT INTO commitments (id, title, to_whom, status, due_date, created_at, updated_at)
VALUES (
    'commit_1',
    'Review architecture proposal',
    'alice',
    'active',
    '2025-12-10T17:00:00Z',
    datetime('now'),
    datetime('now')
);

-- Something you accomplished
INSERT INTO accomplishments (
    id, title, description, impact, source_url, accomplished_at, created_at, updated_at
)
VALUES (
    'acc_1',
    'Shipped secure rebuild for core-java',
    'Implemented automated security scanning for Maven Central rebuilds',
    'Reduced rebuild time from 2h to 30m, enabled 3 critical patches',
    'https://github.com/chainguard-dev/internal-dev/pull/1234',
    '2025-12-06T14:30:00Z',
    datetime('now'),
    datetime('now')
);

-- Context about a person
INSERT INTO context (id, entity_type, entity_id, title, body, created_at, updated_at)
VALUES (
    'ctx_1',
    'person',
    'alice',
    'Lead of platform team',
    'Owns all platform infrastructure. Key stakeholder for auth changes. Usually available Wed-Thu.',
    datetime('now'),
    datetime('now')
);
```

## Common Queries

### What are your active commitments?
```sql
SELECT title, to_whom, due_date
FROM commitments
WHERE status = 'active' AND is_deleted = 0
ORDER BY due_date ASC;
```

### Who is this person?
```sql
SELECT body, urgency
FROM context
WHERE entity_type = 'person' AND entity_id = ?
AND is_deleted = 0
ORDER BY urgency;
```

### Recent accomplishments (for Claude to understand your impact)
```sql
SELECT title, impact, accomplished_at
FROM accomplishments
WHERE is_deleted = 0
AND accomplished_at > datetime('now', '-30 days')
ORDER BY accomplished_at DESC;
```

### Search for context
```sql
SELECT content, title, body
FROM memory_search
WHERE memory_search MATCH ?
LIMIT 10;
```

### What's critical right now?
```sql
SELECT 'goal' as type, title FROM goals
WHERE status = 'active' AND is_deleted = 0 AND priority <= 2
UNION ALL
SELECT 'commitment', title FROM commitments
WHERE status = 'active' AND is_deleted = 0
AND due_date < datetime('now', '+3 days')
UNION ALL
SELECT 'context', title FROM context
WHERE urgency = 'critical' AND is_deleted = 0;
```

## Go Types

```go
package memory

import (
    "encoding/json"
    "time"
)

type Goal struct {
    ID          string    `json:"id"`
    Title       string    `json:"title"`
    Description string    `json:"description,omitempty"`
    Status      string    `json:"status"` // "active", "completed", "on_hold"
    Priority    int       `json:"priority"` // 1-5
    TargetDate  *time.Time `json:"target_date,omitempty"`
    Tags        []string  `json:"tags,omitempty"`
    CreatedAt   time.Time `json:"created_at"`
    UpdatedAt   time.Time `json:"updated_at"`
}

type Commitment struct {
    ID      string     `json:"id"`
    Title   string     `json:"title"`
    ToWhom  string     `json:"to_whom,omitempty"` // empty = personal
    Status  string     `json:"status"` // "active", "in_progress", "completed"
    DueDate time.Time  `json:"due_date"`
    Tags    []string   `json:"tags,omitempty"`
    CreatedAt time.Time `json:"created_at"`
    UpdatedAt time.Time `json:"updated_at"`
}

type Accomplishment struct {
    ID            string     `json:"id"`
    Title         string     `json:"title"`
    Description   string     `json:"description,omitempty"`
    Impact        string     `json:"impact,omitempty"` // Quantified result
    SourceURL     string     `json:"source_url,omitempty"` // GitHub PR, commit
    AccomplishedAt time.Time  `json:"accomplished_at"`
    Tags          []string   `json:"tags,omitempty"`
    CreatedAt     time.Time  `json:"created_at"`
    UpdatedAt     time.Time  `json:"updated_at"`
}

type Context struct {
    ID        string    `json:"id"`
    Type      string    `json:"type"` // "person", "project", "repo", "team"
    EntityID  string    `json:"entity_id"`
    Title     string    `json:"title"`
    Body      string    `json:"body"`
    Urgency   string    `json:"urgency,omitempty"` // "critical", "high", "normal"
    SourceURL string    `json:"source_url,omitempty"`
    Tags      []string  `json:"tags,omitempty"`
    CreatedAt time.Time `json:"created_at"`
    UpdatedAt time.Time `json:"updated_at"`
}
```

## Implementation Notes

1. **No migrations for v1** - Single initialization script
2. **No transactions** - v1 is single-threaded CLI
3. **Soft deletes only** - Never hard-delete; preserve history
4. **JSON tags** - Parse in application layer for flexibility
5. **UTC timestamps** - Always RFC3339 format
6. **File security** - `chmod 0600 ~/.kora/data/kora.db`

## Why This Structure

**Goals** → Claude understands what matters to you right now
**Commitments** → Claude sees your obligations and can help prioritize
**Accomplishments** → Claude knows your impact; useful for 1-1s, retrospectives, context for decisions
**Context** → Claude has knowledge about people and projects to be smarter about relevance

Each table is independent. Each query is simple. No complex joins needed.

---

**Status:** Ready for implementation
**Last Updated:** 2025-12-07
