# Kora Architecture Specification

**Version:** 1.0
**Status:** Living Document
**Last Updated:** 2025-12-07

## Overview

Kora is a **rich data layer** that aggregates work-related information from multiple external services, providing comprehensive, unfiltered data to Claude (or any AI assistant) acting as a personal assistant. Kora's role is to fetch, normalize, and structure data—not to make relevance judgments or filter based on importance. That intelligence layer is Claude's responsibility.

### Core Principle

> "It's not Kora's job to filter, it's Claude's. Kora is the rich data layer."

This fundamental separation of concerns defines the entire architecture:
- **Kora**: Fetch ALL relevant data, provide comprehensive metadata
- **Claude**: Filter, prioritize, synthesize, and present based on user context

## Architectural Vision

Kora evolves from a "morning digest" tool to a comprehensive data foundation for AI-powered personal assistance:

```
┌─────────────────────────────────────────────────────────────┐
│                        CLAUDE                                │
│                  (Personal Assistant)                        │
│                                                              │
│  Capabilities:                                               │
│  • "You have 3 things before your 10am..."                  │
│  • "Last week you shipped the auth refactor..."             │
│  • "Before your 1:1, here's context on Alice's projects..." │
│  • "Based on your goals, focus on X today..."               │
│                                                              │
│  Intelligence:                                               │
│  • Filter by relevance to user's work                       │
│  • Prioritize based on relationships & goals                │
│  • Synthesize patterns across sources                       │
│  • Remember context & decisions                             │
└──────────────────────────┬──────────────────────────────────┘
                           │
                           │ MCP / CLI
                           ▼
┌─────────────────────────────────────────────────────────────┐
│                         KORA                                 │
│                   (Rich Data Layer)                          │
│                                                              │
│  Commands:                                                   │
│  • digest      - What needs attention now                   │
│  • context     - Prepare me for X                           │
│  • history     - What happened with X                       │
│  • accomplish  - What did I do                              │
│  • query       - Tell me about entity X                     │
│  • remember    - Store context/decision/goal                │
│                                                              │
│  Responsibilities:                                           │
│  • Fetch ALL events from datasources                        │
│  • Provide comprehensive metadata                           │
│  • Deduplicate by URL (structural only)                     │
│  • Assign base priority (per EFA 0001)                      │
│  • Validate event structure                                 │
│  • Handle partial failures gracefully                       │
│  • Store local context (goals, decisions)                   │
└──────────────────┬──────────────────┬───────────────────────┘
                   │                  │
       ┌───────────┴──────┐      ┌───┴────────────┐
       ▼                  ▼      ▼                ▼
┌──────────────┐   ┌──────────────┐   ┌───────────────────┐
│ External API │   │ External API │   │   Local Store     │
│              │   │              │   │  ~/.kora/data/    │
│  • GitHub    │   │  • Linear    │   │                   │
│              │   │  • Calendar  │   │  • Goals & OKRs   │
│              │   │  • Gmail     │   │  • Decisions      │
│              │   │              │   │  • Commitments    │
│              │   │              │   │  • Context notes  │
└──────────────┘   └──────────────┘   └───────────────────┘
```

## System Architecture

### High-Level Components

```
┌─────────────────────────────────────────────────────────────┐
│                      CLI Interface                           │
│                    (cmd/kora/)                               │
│                                                              │
│  Commands: digest, context, accomplish, query, remember     │
└──────────────────────────┬──────────────────────────────────┘
                           │
                           ▼
┌─────────────────────────────────────────────────────────────┐
│                    Digest Engine                             │
│                  (internal/digest/)                          │
│                                                              │
│  • Orchestrates datasource fetching                         │
│  • Aggregates events                                        │
│  • Handles partial failures                                 │
│  • Deduplicates by URL                                      │
└──────────────────────────┬──────────────────────────────────┘
                           │
            ┌──────────────┼──────────────┐
            │              │              │
            ▼              ▼              ▼
┌──────────────────┐ ┌──────────────┐ ┌──────────────┐
│ DataSource       │ │ DataSource   │ │ Future       │
│ Runner           │ │ GitHub       │ │ Sources      │
│                  │ │              │ │              │
│ • Concurrent     │ │ • Search     │ │ • Linear     │
│   execution      │ │ • Full       │ │ • Calendar   │
│ • Timeout mgmt   │ │   context    │ │ • Gmail      │
│ • Result agg.    │ │ • CODEOWNERS │ │              │
└──────────────────┘ └──────┬───────┘ └──────────────┘
                            │
                            ▼
                    ┌──────────────┐
                    │ AuthProvider │
                    │   GitHub     │
                    │              │
                    │ • gh CLI     │
                    │   delegation │
                    └──────────────┘
```

### Core Modules

| Module | Location | Governed By | Purpose |
|--------|----------|-------------|---------|
| Event Model | `internal/models/` | EFA 0001 | Canonical event structure |
| Auth Providers | `internal/auth/` | EFA 0002 | Credential management, secure delegation |
| DataSources | `internal/datasources/` | EFA 0003 | External service integration |
| Digest Engine | `internal/digest/` | EFA 0004 | Orchestration, aggregation |
| Output Formatters | `internal/output/` | - | Terminal, JSON, Markdown output |
| CLI | `cmd/kora/` | - | Command-line interface |
| Storage | `internal/storage/` | (Future EFA) | Local context persistence |

### EFA Governance

All core modules are governed by Explainer For Agents (EFA) documents that define ground truth:

- **EFA 0001**: Event Model - Structure, validation, metadata, priority rules
- **EFA 0002**: Auth Provider - Credential security, CLI delegation
- **EFA 0003**: DataSource Interface - Fetching, concurrency, partial success
- **EFA 0004**: Tool Responsibility - Kora vs Claude separation of concerns

**Critical for AI agents**: These EFAs contain "AI Agent Rules" sections that MUST be followed. Code protected by EFAs has explicit comments marking them as forbidden to change without EFA updates.

**Future EFAs needed**:
- Storage interface and persistence layer
- Query language for context retrieval
- Webhook/real-time update handling
- Multi-user isolation and security

## Data Flow

### 1. Digest Command Flow

```
User/Claude
    │
    ├─> kora digest --since 16h --format json
    │
    ▼
CLI Parser
    │
    ├─> Parse flags
    ├─> Validate time range
    │
    ▼
Digest Engine
    │
    ├─> Initialize configured datasources
    ├─> Create DataSourceRunner
    │
    ▼
DataSourceRunner (concurrent execution)
    │
    ├──> GitHub DataSource ────┐
    │    │                      │ (parallel)
    │    ├─> Authenticate       │
    │    ├─> Phase 1: Search    │
    │    │   • review-requested │
    │    │   • mentions         │
    │    │   • authored PRs     │
    │    │   • assigned issues  │
    │    │                      │
    │    ├─> Phase 2: Fetch     │
    │    │   Full Context       │
    │    │   • Reviews          │
    │    │   • CI status        │
    │    │   • Files changed    │
    │    │   • Comments         │
    │    │                      │
    │    ├─> Phase 3: CODEOWNERS│
    │    │   • Check ownership  │
    │    │   • Create events    │
    │    │                      │
    │    └─> Validate events    │
    │                           │
    └──────────────────────────┘
         │
         ├─> Wait for all (errgroup)
         │
         ▼
    Aggregate Results
         │
         ├─> Deduplicate by URL
         ├─> Merge relationships
         ├─> Sort by timestamp
         │
         ▼
    Output Formatter
         │
         ├─> JSON / Terminal / Markdown
         │
         ▼
    Claude / User
```

### 2. Event Lifecycle

```
External Service
    │
    ▼
DataSource
    │
    ├─> Fetch raw data
    ├─> Transform to Event
    │   • Set Type
    │   • Set Priority (EFA 0001 rules)
    │   • Populate Metadata
    │   • Set user_relationships
    │
    ├─> Validate (Event.Validate())
    │   • Check Type is valid
    │   • Verify Title length
    │   • Validate URL format
    │   • Check metadata keys
    │
    └─> Return Event
         │
         ▼
    DataSourceRunner
         │
         ├─> Deduplicate by URL
         ├─> Merge relationships
         │
         ▼
    Output
```

### 3. Authentication Flow

```
DataSource
    │
    ├─> Request credentials
    │
    ▼
AuthProvider
    │
    ├─> Check credential cache (memory)
    │
    ├─> If expired/missing:
    │   │
    │   └─> GitHub: Delegate to `gh` CLI
    │       • Execute: gh auth status
    │       • Return: delegation token
    │
    └─> Return credential
         │
         ▼
    DataSource uses credential
         │
         ├─> NEVER logs credential
         ├─> NEVER includes in errors
         │
         └─> API Call
```

## Data Categories & Sources

### Current Implementation (v1)

| Category | Sources | Data Types | Priority |
|----------|---------|------------|----------|
| Work Queue | GitHub | PRs, Issues | Tier 1 |
| Schedule | (Future: Google Calendar) | Meetings, events | Tier 1 |

### Planned Implementation

| Category | Sources | Data Types | Use Cases |
|----------|---------|------------|-----------|
| Accomplishments | GitHub, Linear | Merged PRs, Completed Issues | "What did I ship this week?" |
| Context | GitHub, Linear | PR discussions, Issue threads | "Prepare me for meeting X" |
| Commitments | Local Store | Promises made | "What did I commit to?" |
| Goals | Local Store | OKRs, Personal goals | "Am I on track?" |
| Decisions | Local Store | Technical decisions | "Why did we choose X?" |

## Datasource Architecture

### Tier 1 Datasources (Current + Next)

#### 1. GitHub (Implemented)

**Purpose**: Development work visibility - PRs, issues, code reviews

**Event Types**:
- `pr_review` - Review requested (direct or team)
- `pr_author` - User's own PRs (status tracking)
- `pr_codeowner` - User owns changed files
- `pr_mention` - @mentioned in PR body/title
- `pr_comment_mention` - @mentioned in comments/reviews
- `pr_closed` - User's PR merged/closed (informational)
- `issue_assigned` - Assigned issues
- `issue_mention` - @mentioned in issues

**Implementation Pattern**:
- **Phase 1**: Search queries (lightweight)
  - `review-requested:@me is:open -draft:true type:pr`
  - `mentions:@me type:pr`
  - `assignee:@me is:open type:issue`
  - `author:@me is:open type:pr`
- **Phase 2**: GraphQL queries for full context (per item)
  - Reviews, CI status, files changed
  - Comments, labels, milestones
  - All EFA 0001 metadata fields
- **Phase 3**: CODEOWNERS integration
  - Check file ownership
  - Create events for owned files (if not already reviewer)

**Authentication**: `gh` CLI delegation (EFA 0002)

**Rich Metadata** (see EFA 0001 for complete list):
- Review requests (user vs team, team slugs)
- Reviews (author, state, timestamps)
- CI checks (all checks + rollup state)
- Files changed (paths, additions, deletions)
- Labels, milestones, linked issues
- Comments, threads, unresolved discussions
- Draft status, mergeability
- Branch information

#### 2. Linear (Planned - Tier 1)

**Purpose**: Issue tracking complementary to GitHub

**Why Linear**: Many engineering teams use Linear for sprint planning, roadmap tracking, and customer-facing issues. It provides context GitHub doesn't have:
- Sprint cycles and velocity
- Customer impact priorities
- Cross-team dependencies
- Time estimates and actuals

**Event Types** (proposed):
- `linear_assigned` - Assigned issues
- `linear_mention` - @mentions
- `linear_blocked` - Blocking issues
- `linear_due_soon` - Issues with approaching deadlines

**Implementation Pattern**:
- GraphQL API queries
- Filter by assignee, mentions
- Include cycle, project, labels

**Rich Metadata**:
- Project, team
- Cycle, sprint
- Status, priority
- Estimate, actual time
- Blocked by, blocking
- Labels, attachments

**Future EFA**: Will require new EFA for Linear-specific metadata keys

#### 3. Google Calendar (Planned - Tier 1)

**Purpose**: Meeting context, schedule awareness

**Why Calendar**: Claude needs situational awareness:
- "You have a 1:1 in 30 minutes"
- "Before your planning meeting, here's what's happening"
- "You're overbooked today—3 meetings back-to-back"

**Event Types** (proposed):
- `calendar_meeting` - Upcoming meetings
- `calendar_all_day` - All-day events
- `calendar_reminder` - Reminders

**Implementation Pattern**:
- Calendar API queries
- Filter by time range
- Include attendees, location

**Rich Metadata**:
- Attendees (with response status)
- Meeting description
- Location (physical/virtual)
- Conference link
- Organizer
- Recurrence pattern

**Future EFA**: Will require new EFA for Calendar-specific metadata keys

### Tier 2 Datasources (Future)

#### 4. Gmail (Planned - Tier 2)

**Purpose**: High-signal email detection (filtered, not all email)

**Why Gmail**: Catch high-priority communications that don't happen in GitHub:
- Direct emails from execs or customers
- Important threads (flagged/starred)
- Action items in email

**Event Types** (proposed):
- `email_direct` - Direct emails (not lists)
- `email_flagged` - Flagged/starred
- `email_thread_reply` - Replies in active threads

**Filters** (important—not all email):
- **Skip**: Mailing lists, newsletters, notifications
- **Include**: Direct messages, flagged, active threads
- Use Gmail labels/filters for signal

**Rich Metadata**:
- From, To, Cc
- Thread ID
- Labels
- Snippet (preview)
- Attachments (list only)

**Challenge**: Email volume requires aggressive filtering. Kora should fetch selectively based on labels, stars, importance markers.

**Future EFA**: Will require new EFA defining email filtering rules and metadata

### Tier 3 Datasources (Exploration)

**Potential future sources** (to be evaluated based on user needs):
- **Jira** (if team uses it instead of/alongside GitHub/Linear)
- **Notion** (for documentation context, meeting notes)
- **Confluence** (for wiki/documentation context)
- **Figma** (for design reviews, if applicable)
- **PagerDuty** (for on-call incidents)

**Evaluation criteria**:
- Does it provide unique work context?
- Is the API rich enough for comprehensive metadata?
- Can it be queried efficiently (GraphQL preferred)?
- Does it fit the "work visibility" scope?

### Local Memory Store (Planned)

**Purpose**: Store user-provided context that external services don't have

**Data Types**:
- **Goals & OKRs**: Personal/team objectives
- **Decisions**: Technical and project decisions
- **Commitments**: Promises made to others
- **Context Notes**: Background on people/projects
- **Accomplishments**: Journal of completed work

**Storage**: SQLite database at `~/.kora/data/kora.db`

**Design**: See `specs/memory-data.md` for detailed schema and query patterns

**Commands**:
```bash
# Store data
kora remember goal "Ship auth refactor by Q4"
kora remember decision "Use OAuth2" --rationale "Better security"
kora remember commitment "Review Alice's PR by Friday" --to alice

# Query data
kora query --type goal
kora query --type person --id alice
kora accomplish --since 7d
```

**Future EFA**: Storage layer will require comprehensive EFA covering:
- Schema design and evolution
- Query interface
- Transaction boundaries
- Backup/restore
- Privacy/encryption

## Command Interface

### Current Commands (v1)

#### `kora digest`

**Purpose**: Get work queue events from external sources

**Usage**:
```bash
kora digest --since 16h [--format json|terminal|markdown] [--types pr_review]
```

**Output**: List of events with full metadata

**Example**:
```bash
# Morning digest
kora digest --since 16h --format terminal

# JSON for Claude
kora digest --since 16h --format json
```

### Planned Commands

#### `kora context`

**Purpose**: Prepare context for a specific activity (meeting, code review, planning)

**Usage**:
```bash
kora context --for "meeting:1:1-alice"
kora context --for "pr-review:123"
kora context --for "planning:sprint"
```

**Output**: Aggregated context relevant to the activity:
- Recent interactions with people involved
- Related PRs/issues/decisions
- Goals/commitments relevant to topic
- Recent accomplishments

**Implementation**: Combines external events + local store queries

#### `kora accomplish`

**Purpose**: Show what was accomplished in a time period

**Usage**:
```bash
kora accomplish --since 7d [--format markdown]
kora accomplish --since 2024-12-01 --category shipped
```

**Output**: Merged PRs, completed issues, tagged accomplishments

**Use case**: Weekly standup prep, performance reviews, status updates

#### `kora query`

**Purpose**: Look up context about an entity (person, project, team)

**Usage**:
```bash
kora query --type person --id alice
kora query --type project --id auth-refactor
```

**Output**: Aggregated info from all sources + local notes

**Implementation**: Cross-reference external events with local context

#### `kora remember`

**Purpose**: Store context, decisions, goals, commitments

**Usage**:
```bash
kora remember goal "Ship feature X by Q4"
kora remember decision "Use OAuth2" --rationale "Better security"
kora remember commitment "Review Alice's PR by Friday" --to alice
kora remember note "Alice is tech lead for auth team" --entity person:alice
```

**Output**: Confirmation + ID for later reference

**Storage**: Local SQLite database (see `specs/memory-data.md`)

#### `kora history`

**Purpose**: Timeline view of what happened with an entity

**Usage**:
```bash
kora history --entity pr:123
kora history --entity person:alice --since 30d
```

**Output**: Chronological timeline of events related to entity

**Implementation**: Query events + local store, merge by timestamp

## Personal Assistant Capabilities

### What Claude Can Do With Kora

| Capability | Data Required | Example |
|------------|--------------|---------|
| **Situational Awareness** | Calendar + work queue | "You have a 1:1 with Alice in 30min. Last interaction: She requested review on PR #123" |
| **Memory** | Decisions + commitments | "Last time you talked to Bob, you agreed to use OAuth2. Here's the rationale..." |
| **Accomplishment Tracking** | GitHub PRs + Linear issues | "This week you shipped: auth refactor, improved test coverage, fixed 3 bugs" |
| **Preparation** | Calendar + context + goals | "Before your security review, here's what's happening: 2 PRs pending, 1 blocker" |
| **Planning** | Goals + workload + deadlines | "Based on your Q4 goals and current capacity, focus on auth first, then monitoring" |
| **Follow-up** | Commitments + timestamps | "You said you'd get back to Alice by Friday about OAuth design" |
| **Relationship Context** | Communications + interactions | "Alice is your tech lead. Recent topics: auth, testing, deployment" |
| **Pattern Recognition** | Cross-source synthesis | "3 PRs from the auth team are waiting on your review. Common theme: OAuth integration" |

### Example Interaction Flow

**User**: "What should I focus on this morning?"

**Claude** (internally):
1. Invokes: `kora digest --since 16h --format json`
2. Receives: 15 events from GitHub
3. Filters: Relevant to user's current projects (auth refactor)
4. Prioritizes: Based on relationships, deadlines, goals
5. Synthesizes: Groups related items, identifies patterns

**Claude** (to user):
> Good morning! Here's what needs your attention:
>
> **Critical** (1 item):
> - Your "Fix OAuth redirect" PR has failing CI. Tests show 2 failures in `auth_test.go`.
>
> **High Priority** (2 items):
> - Alice's OAuth flow PR is blocked on your review (15 files, CI passing)
> - Bob (your manager) needs input on Q4 planning
>
> **Context**: You're working toward your goal "Ship auth refactor by Q4". Alice's PR is part of that work.
>
> **Also on your radar**: 3 older PR mentions, 2 issues
>
> Want me to look at the CI failures?

## Design Principles

### 1. Local-First Storage

**Rationale**: Privacy, speed, offline capability

**Implementation**:
- SQLite at `~/.kora/data/kora.db`
- No cloud sync in v1
- Backup strategy: User responsibility

**Benefits**:
- No network calls for local data
- User owns their data
- Fast queries
- Works offline

**Future**: Optional encrypted cloud backup

### 2. Speed Through Parallelism

**Rationale**: Morning digest must be fast (<60s total)

**Implementation**:
- Concurrent datasource fetching (errgroup)
- Per-datasource timeout (30s)
- Per-API-call timeout (10s)
- GraphQL for efficient queries

**Targets**:
| Operation | Timeout | Rationale |
|-----------|---------|-----------|
| Total execution | 60s | User won't wait longer |
| Per-datasource | 30s | One slow source doesn't block others |
| Per-API call | 10s | Detect hung connections early |
| Auth verification | 5s | Fast fail on auth issues |

### 3. Partial Success Model

**Rationale**: One datasource failure shouldn't block all results (EFA 0004)

**Implementation**:
- DataSources return `(events, error)` tuple
- Runner collects both successes and failures
- Output includes `source_errors` map
- Claude decides how to handle partial data

**Example**:
```json
{
  "events": [...], // GitHub events (successful)
  "source_errors": {}
}
```

### 4. Rich Metadata (No Filtering)

**Rationale**: Claude has context to make relevance decisions, Kora doesn't (EFA 0004)

**Implementation**:
- Every event includes ALL EFA 0001 metadata
- No "importance" filtering
- No "relevance" scoring
- Comprehensive data enables better AI decisions

**Benefits**:
- Claude can always filter down, never up
- Patterns visible across "unimportant" events
- Explanations for why something was/wasn't shown
- Future-proof for new AI capabilities

### 5. Base Priority Only

**Rationale**: Kora assigns structural priority, Claude adjusts for context (EFA 0004)

**Kora's Priority Rules** (EFA 0001):
- PR author + CI failing → 1 (Critical)
- Direct review request → 2 (High)
- Team review request → 3 (Medium)
- Mentions → 3 (Medium)
- PR closed → 5 (Info)

**Claude's Adjustments** (examples):
- "Alice is tech lead" → Elevate her PRs
- "User is on vacation" → Demote work items
- "Release deadline tomorrow" → Elevate related PRs
- "Minor typo fix" → Demote despite high base priority

### 6. Stateless Execution (v1)

**Rationale**: Simplicity, testability, reliability

**Implementation**:
- Each invocation is independent
- No state between calls
- No "seen" tracking
- No "last check" memory

**Future**: Local store will add state for goals/decisions/commitments, but datasource fetching remains stateless.

### 7. Security First

**Rationale**: Credentials are sensitive, must be protected (EFA 0002)

**Implementation**:
- **Never log credentials** (not even debug level)
- **Never include in errors**
- **GitHub**: Delegate to `gh` CLI (no token in Kora)
- **TLS 1.2+** for all network calls
- **Timeout on all operations**

## Performance Characteristics

### Current Performance (v1)

| Metric | Target | Actual | Notes |
|--------|--------|--------|-------|
| Total execution | <60s | ~5-10s | GitHub concurrent |
| GitHub fetch | <30s | ~5-10s | 5 searches + context queries |
| Memory usage | <100MB | ~50MB | In-memory event aggregation |

### Scalability Considerations

**Current Limits**:
- ~100 events per datasource (reasonable for daily digest)
- ~5 datasources concurrent (IO-bound, not CPU)
- No persistence (memory-only)

**Future Scaling**:
- Add pagination for large result sets
- Add caching layer for repeated queries
- Add incremental updates (webhooks)
- Add database persistence for historical queries

## Error Handling

### Error Categories

| Category | Example | Handling |
|----------|---------|----------|
| **Authentication** | GitHub token expired | Fail fast, clear message |
| **Rate Limiting** | GitHub API limit hit | Report, include partial results |
| **Network** | Service unreachable | Report, include other sources |
| **Validation** | Malformed event | Log, skip event |
| **Timeout** | Slow API call | Cancel, return partial |

### Partial Success Example

```json
{
  "events": [
    {"source": "github", "type": "pr_review", ...},
    {"source": "github", "type": "pr_author", ...}
  ],
  "source_errors": {},
  "stats": {
    "total_sources": 1,
    "successful_sources": 1,
    "failed_sources": 0,
    "total_events": 2
  }
}
```

Claude can then decide:
- Show available data with warning
- Retry failed source
- Proceed without failed datasource

## Testing Strategy

### Unit Tests

**Event Model** (EFA 0001):
- Validation rules
- Priority calculation
- Metadata key validation
- Deduplication logic

**DataSources** (EFA 0003):
- Mock HTTP responses (use `testdata/`)
- Partial success scenarios
- Rate limiting detection
- Event validation

**Auth Providers** (EFA 0002):
- Credential handling
- No logging of secrets
- CLI delegation (mock)

### Integration Tests

**Auth Flow**:
- GitHub `gh` CLI delegation (requires `gh` installed)

**DataSource Fetching**:
- Real API calls (tagged `//go:build integration`)
- Rate limiting behavior
- Timeout handling
- Full context fetching

### Test Coverage Targets

| Module | Target | Rationale |
|--------|--------|-----------|
| Event Model | >95% | Core data structure |
| DataSources | >85% | Complex API integration |
| Auth Providers | >90% | Security-critical |
| CLI | >70% | UI layer |
| Output Formatters | >80% | Data presentation |

## Future Architecture Considerations

### Webhooks for Real-Time Updates

**Current**: Poll-based (digest command)
**Future**: Webhook-based push notifications

**Design**:
```
GitHub Webhook
    │
    └─> Kora Server (daemon)
         │
         ├─> Store event in local DB
         ├─> Notify Claude via MCP
         │
         ▼
    Claude responds immediately
```

**Benefits**:
- Real-time awareness
- No polling delay
- Reduced API calls

**Challenges**:
- Requires daemon process
- Authentication for webhooks
- Duplicate event handling

**Future EFA**: Webhook handling will require new EFA for event deduplication, persistence, and notification patterns

### Multi-User Support

**Current**: Single-user macOS CLI
**Future**: Multi-user deployment

**Considerations**:
- User isolation in database
- Per-user credential storage
- Rate limit pooling
- Privacy boundaries

**Future EFA**: Multi-user isolation will require new EFA for security boundaries

### Cloud Sync

**Current**: Local-only storage
**Future**: Optional cloud backup

**Design**:
- Encrypted backups
- User-controlled sync
- Privacy-preserving (goals/decisions only, not credentials)

**Future EFA**: Cloud sync will require new EFA for encryption, privacy, and sync conflict resolution

## Appendix: Diagrams

### Event Deduplication Flow

```
Search Results:
┌─────────────────────────────────────┐
│ PR #123: review-requested search    │
│ • user_relationships: [reviewer]    │
└─────────────────────────────────────┘

┌─────────────────────────────────────┐
│ PR #123: mentions search            │
│ • user_relationships: [mentioned]   │
└─────────────────────────────────────┘

         │
         ├─> Deduplicate by URL
         │
         ▼

Merged Event:
┌─────────────────────────────────────┐
│ PR #123: single event               │
│ • user_relationships: [reviewer,    │
│                        mentioned]   │
│ • EventType: pr_review (higher pri) │
└─────────────────────────────────────┘
```

### Priority Calculation Decision Tree

```
                    Event Type?
                         │
        ┌────────────────┼────────────────┐
        │                │                │
    pr_author        pr_review       pr_mention
        │                │                │
        ├─> CI status?   ├─> Request      └─> Priority 3
        │   │            │   type?            (Medium)
        │   ├─ failing   │   │
        │   │  → P1      │   ├─ user
        │   │            │   │  → P2
        │   ├─ changes   │   │
        │   │  requested │   └─ team
        │   │  → P2      │      → P3
        │   │            │
        │   └─ else      │
        │      → P3      │
```

### Relationship Priority Hierarchy

```
When deduplicating by URL, select EventType by highest-priority relationship:

1. direct_reviewer  → pr_review (P2)
   │
2. author          → pr_author (P1/P2/P3 based on CI/reviews)
   │
3. codeowner       → pr_codeowner (P3)
   │
4. mentioned       → pr_comment_mention (P3)
   (in comments)
   │
5. mentioned       → pr_mention (P3)
   (in body/title)
```

## References

- **EFA 0001**: Event Model Ground Truth
- **EFA 0002**: Auth Provider Interface
- **EFA 0003**: DataSource Interface
- **EFA 0004**: Tool Responsibility and Separation of Concerns
- **specs/memory-data.md**: Local storage schema and query patterns
- **CLAUDE.md**: Project development guide
- **specs/repository-layout.md**: Repository structure

---

**Document Status**: This is a living document that will evolve as Kora's architecture matures. All changes to core architecture must be reflected here and in relevant EFA documents.

**Future EFAs Needed**:
1. Storage interface and persistence layer (references `memory-data.md`)
2. Linear datasource metadata and event types
3. Google Calendar datasource metadata and event types
4. Gmail datasource filtering rules and metadata
5. Webhook handling and real-time updates
6. Multi-user isolation and security boundaries
7. Cloud sync encryption and privacy
