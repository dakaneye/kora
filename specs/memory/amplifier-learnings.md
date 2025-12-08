# Memory Schema Learnings

Research from [Microsoft Amplifier](https://github.com/microsoft/amplifier) and [MCP Memory Server](https://github.com/modelcontextprotocol/servers/tree/main/src/memory).

## MCP Memory Server: Knowledge Graph Approach

### Schema

```
Entity:
  - name (unique identifier)
  - entityType ("person", "organization", "event")
  - observations[] (atomic facts as strings)

Relation:
  - from (source entity)
  - to (target entity)
  - relationType (active voice: "manages", "owns", "blocks")
```

### Key Patterns

1. **Atomic observations** - One fact per observation, not compound info
2. **Active voice relations** - "Alice manages Auth" not "Auth is managed by Alice"
3. **Three degrees of separation** - Track relationships up to 3 hops

### What to Remember (MCP guidance)

- Identity: demographics, roles, education
- Behaviors: interests, habits
- Preferences: communication style, language
- Goals: aspirations, objectives
- Relationships: professional and personal connections

## Microsoft Amplifier: File-Based Approach

### Storage Mechanisms

| Type | Location | Purpose |
|------|----------|---------|
| Transcripts | `.data/transcripts/` | Full conversation history before compaction |
| Project context | `AGENTS.md` | Persistent guidance per project |
| Decisions | `ai_working/decisions/` | Architectural rationale with review triggers |
| AI context | `.ai/docs/` | Documented knowledge |

### Key Patterns

1. **PreCompact hooks** - Auto-capture before token limit compaction
2. **Decision records** - Context, rationale, alternatives, review triggers
3. **Zero-BS code** - Never ship stubs or placeholders
4. **Incremental saves** - Save after each item, not at intervals

## Comparison: Kora vs MCP Memory

| Aspect | MCP Memory | Kora |
|--------|------------|------|
| Model | Knowledge graph | Structured tables |
| Storage | JSONL | SQLite + FTS5 |
| Entities | Generic + type field | Separate tables |
| Relations | Explicit links | **Missing** |
| Facts | Atomic observations | Text blobs |
| Search | Name/type scan | Full-text search |

## Gaps in Kora's Current Schema

### 1. No Relations

Can't express:
- Goal → Commitment ("goal X requires commitment Y")
- Person → Project ("Alice owns auth")
- Accomplishment → Goal ("shipped X, completing goal Y")

### 2. No Atomic Observations

`context.body` is monolithic. Can't update single facts like "prefers async" without rewriting entire body.

### 3. No Storage Guidance

No explicit rules for when Claude should create entries.

## Recommended Additions

### Relations Table

```sql
CREATE TABLE relations (
    id TEXT PRIMARY KEY,
    from_type TEXT NOT NULL,  -- "goal", "commitment", "context"
    from_id TEXT NOT NULL,
    to_type TEXT NOT NULL,
    to_id TEXT NOT NULL,
    relation TEXT NOT NULL,   -- "drives", "blocks", "owns", "relates_to"
    created_at TEXT NOT NULL
);

CREATE INDEX idx_relations_from ON relations(from_type, from_id);
CREATE INDEX idx_relations_to ON relations(to_type, to_id);
```

### Observations Table (Optional)

```sql
CREATE TABLE observations (
    id TEXT PRIMARY KEY,
    entity_type TEXT NOT NULL,
    entity_id TEXT NOT NULL,
    fact TEXT NOT NULL,
    is_deleted INTEGER DEFAULT 0,
    created_at TEXT NOT NULL
);

CREATE INDEX idx_observations_entity ON observations(entity_type, entity_id)
    WHERE is_deleted = 0;
```

### Storage Guidance for Claude

| Store as | When |
|----------|------|
| **Goal** | User states objective or priority |
| **Commitment** | User promises something to someone with deadline |
| **Accomplishment** | User ships, resolves, or achieves something |
| **Context** | User shares info about people/projects to remember |
| **Relation** | Entities connect (person owns project, goal drives commitment) |
| **Observation** | Atomic fact about entity that may change independently |

### Relation Types (Active Voice)

- `owns` - Person/team owns project/repo
- `drives` - Goal drives commitment
- `completes` - Accomplishment completes goal
- `blocks` - Entity blocks another
- `relates_to` - General association
- `stakeholder_for` - Person is stakeholder for project
- `reports_to` - Person reports to person

## Decision

**Keep structured tables** (better for work queries) but consider adding:

1. `relations` table for entity linking
2. Storage guidance in `specs/memory-mcp.md`
3. Optional: `observations` table for flexible facts

**Don't adopt:**
- Full knowledge graph (overkill for work context)
- JSONL storage (SQLite + FTS5 is better for our queries)
- Generic entity table (structured tables are clearer)

---

## Kora Digest Issues (2025-12-08)

### Issue 1: Missing PR Review Request

**Symptom**: PR #658 (ecosystems-java-rebuilder) didn't appear in terminal digest this morning, but DOES appear in JSON output now.

**Analysis**: PR is being fetched correctly with `user_relationships: ["direct_reviewer"]`. The discrepancy was likely timing - the user ran the digest before the review request was made.

**Status**: Working as designed

### Issue 2: Missing Merged PRs from Repos User Cares About

**Symptom**: PRs #382, #383 (ecosystems-rebuilder.js) were recently merged but didn't appear in digest.

**Root Cause**: Kora only fetches closed/merged PRs where user is the **author**:
```go
// graphql_fetcher.go:511
searchQuery := fmt.Sprintf("author:%s is:closed type:pr updated:>=%s", currentUser, ...)
```

**Current Fetch Strategy**:
| Fetch Type | Query | Result |
|------------|-------|--------|
| PRs authored | `author:USER` | Shows merged own PRs |
| PRs reviewing | `review-requested:USER` | Only open PRs |
| PRs mentioned | `mentions:USER` | Only if mentioned |
| Issues assigned | `assignee:USER` | Works |
| Issues mentioned | `mentions:USER` | Works |

**Gap**: No way to track merged PRs in repos user cares about (unless directly involved).

### Proposed Solution: Watched Repos Config

Add `watched_repos` config option:
```yaml
datasources:
  github:
    enabled: true
    watched_repos:
      - chainguard-dev/ecosystems-rebuilder.js
      - chainguard-dev/ecosystems-java-rebuilder
```

This would fetch recently merged PRs from watched repos regardless of user involvement.

**Implementation**:
```go
// New fetch: recent merged PRs in watched repos
// Query: repo:ORG/REPO is:merged type:pr updated:>=DATE
func (d *DataSource) fetchWatchedRepoMergedPRs(...)
```

**New Event Type**: `pr_merged_watched` or reuse `pr_closed` with metadata `watched: true`

### Priority

- **High**: Watched repos feature (users miss important team PRs)
- **Medium**: Relations table for memory store
- **Low**: Observations table

---

**Date**: 2025-12-08
**Status**: Research complete, pending implementation decision
