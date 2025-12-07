---
authors: Samuel Dacanay <samuel@dakaneye.com>
state: draft
agents: golang-pro, documentation-engineer, prompt-engineer
---

# EFA 0004: Tool Responsibility and Separation of Concerns

This EFA defines the fundamental separation of concerns between Kora (the data layer) and Claude (the intelligence layer). Kora is a **rich data layer** that fetches comprehensive, unfiltered data. Claude is the **personal assistant** that receives this data and makes relevance, priority, and presentation decisions.

## Motivation & Prior Art

Without explicit boundaries between Kora and Claude's responsibilities, Claude may:
- Ask Kora to filter or rank events (violates separation of concerns)
- Expect Kora to make relevance decisions (Kora has no user context)
- Treat Kora as an intelligent assistant rather than a data provider
- Request Kora to reduce data to improve response times (premature optimization)

**The core design principle:**
> "It's not Kora's job to filter, it's Claude's. Kora is the rich data layer."

**Goals:**
- Clear separation: Kora fetches ALL relevant data, Claude decides importance
- Speed: Kora must be fast - efficiency is critical for morning digest workflow
- Rich context: Comprehensive metadata enables Claude to make informed decisions
- Partial success: One datasource failure must not block others

**Non-goals:**
- Kora making relevance judgments (that's Claude's job)
- Kora filtering based on user preferences (Claude has user context)
- Kora summarizing or aggregating data (Claude does synthesis)
- Smart caching or prediction (v1 is stateless, single invocation)

## Detailed Design

### Architectural Responsibility Model

```
                    ┌──────────────────────────────────────────────────────────┐
                    │                        USER                              │
                    │         (Has context, preferences, relationships)        │
                    └─────────────────────────┬────────────────────────────────┘
                                              │
                                              ▼
┌─────────────────────────────────────────────────────────────────────────────────┐
│                                    CLAUDE                                        │
│                           (Intelligence Layer)                                   │
│                                                                                  │
│  Responsibilities:                                                               │
│  ├─ Understand user context and preferences                                     │
│  ├─ Filter events by relevance to user's current work                          │
│  ├─ Prioritize based on user's goals and relationships                         │
│  ├─ Synthesize patterns ("3 PRs blocking the release")                         │
│  ├─ Summarize and present in natural language                                  │
│  └─ Decide what to surface vs. omit                                            │
│                                                                                  │
│  Claude receives: Rich, comprehensive data firehose                             │
│  Claude outputs: Curated, relevant, actionable digest                          │
└─────────────────────────────────────────────────────────────────────────────────┘
                                              │
                                              │ MCP Tool Call / CLI Invocation
                                              ▼
┌─────────────────────────────────────────────────────────────────────────────────┐
│                                    KORA                                          │
│                             (Rich Data Layer)                                    │
│                                                                                  │
│  Responsibilities:                                                               │
│  ├─ Fetch ALL events from configured datasources                                │
│  ├─ Provide comprehensive metadata (EFA 0001)                                   │
│  ├─ Deduplicate events by URL (merge relationships)                             │
│  ├─ Assign base priority per EFA 0001 rules                                     │
│  ├─ Validate event structure                                                    │
│  └─ Handle partial failures gracefully                                          │
│                                                                                  │
│  Kora receives: Time range, optional event type filters                         │
│  Kora outputs: Complete, validated, rich event stream                           │
└─────────────────────────────────────────────────────────────────────────────────┘
                                              │
                          ┌───────────────────┼───────────────────┐
                          ▼                   ▼                   ▼
                   ┌────────────┐      ┌────────────┐      ┌────────────┐
                   │   GitHub   │      │ Future     │      │  Future    │
                   │    API     │      │    API     │      │  Sources   │
                   └────────────┘      └────────────┘      └────────────┘
```

### What Kora Provides (Rich Data)

Kora is a **firehose** of work-related data. It fetches everything that might be relevant and lets Claude decide what matters.

#### Complete Event Data

Every event includes ALL metadata defined in EFA 0001:

**GitHub PRs:**
- Review requests (user vs team, with team slugs)
- All reviews (author, state, timestamps)
- CI status (all checks, rollup state)
- Files changed (paths, additions, deletions)
- Labels, milestone, linked issues
- Comments, threads, unresolved discussions
- Draft status, mergeability
- Branch information
- Full timestamps

**GitHub Issues:**
- Assignees, labels, milestone
- Recent comments (up to 10)
- Linked PRs
- Reactions, timeline
- Full body text

**Future datasources (Linear, Calendar):**
- DMs and mentions
- Thread context
- Workspace and channel info

#### No Relevance Filtering

Kora does NOT filter based on:
- User's current project focus
- Relationship strength with authors
- Historical engagement patterns
- Predicted importance

These are Claude's decisions to make with user context.

#### Structural Deduplication Only

Kora performs only **structural deduplication**, not **semantic deduplication**:

```go
// Structural deduplication (Kora does this)
// Same PR appears in multiple searches -> merge into one event
pr1 := Event{URL: "github.com/org/repo/pull/123", Type: "pr_review"}
pr2 := Event{URL: "github.com/org/repo/pull/123", Type: "pr_mention"}
// Result: One event with merged user_relationships

// Semantic deduplication (Claude does this)
// PR 123 and Issue 456 are about the same feature -> group them
// Kora returns both; Claude decides they're related
```

### What Claude Provides (Intelligence)

Claude is the **brain** that processes Kora's data firehose and produces a curated digest.

#### Relevance Filtering

Claude filters based on user context:
- "I'm focused on the auth refactor" -> prioritize auth-related PRs
- "I'm off frontend this sprint" -> deprioritize UI changes
- "Alice is my tech lead" -> her reviews matter more

#### Priority Adjustment

Claude adjusts Kora's base priorities:
- Base priority 3 (Medium) from Kora
- Claude knows user is release captain -> elevate to priority 1
- Claude knows it's a minor typo fix -> demote to priority 5

#### Synthesis and Grouping

Claude identifies patterns:
- "3 PRs from the same author need your review"
- "CI is failing on your 2 open PRs"
- "The auth team is waiting on your input"

#### Presentation

Claude decides how to present:
- What goes in the summary vs. details
- How to phrase notifications
- What links to include
- When to suggest actions

### Performance Requirements

Speed is critical. The morning digest workflow requires fast execution.

#### Timeout Constraints

| Operation | Timeout | Rationale |
|-----------|---------|-----------|
| Total execution | 60 seconds | User won't wait longer |
| Per-datasource | 30 seconds | One slow source shouldn't block others |
| Per-API call | 10 seconds | Detect hung connections early |
| Auth verification | 5 seconds | Fast fail on auth issues |

#### Concurrency Model

Datasources run **concurrently** via errgroup (EFA 0003):

```go
// All datasources run in parallel
g, ctx := errgroup.WithContext(ctx)
for _, ds := range datasources {
    ds := ds
    g.Go(func() error {
        return ds.Fetch(ctx, opts)
    })
}
```

**No sequential dependencies** between datasources. Multiple datasources fetch in parallel.

#### Efficiency Guidelines

1. **GraphQL over REST**: Fetch all metadata in single queries
2. **Two-phase approach**: Search first (lightweight), then fetch context
3. **No pagination by default**: Fetch reasonable limits (100 events)
4. **Early termination**: Respect context cancellation immediately

### Partial Success Model

One datasource failure MUST NOT block others:

```go
type RunResult struct {
    Events       []Event           // All events from successful sources
    SourceErrors map[string]error  // Which sources failed
}

// Example: GitHub succeeds, another fails
result := &RunResult{
    Events: githubEvents,  // User still gets GitHub data
    SourceErrors: map[string]error{
        "calendar": ErrServiceUnavailable,  // Reported but not blocking
    },
}
```

Claude can decide how to handle partial results:
- Show available data with warning
- Retry failed sources
- Report to user

### The Contract

#### Kora's Guarantees

1. **Completeness**: Return ALL events matching time range and type filters
2. **Richness**: Include ALL EFA 0001 metadata for each event
3. **Validity**: Every event passes `Event.Validate()`
4. **Speed**: Complete within timeout constraints
5. **Resilience**: Partial success on datasource failures
6. **Deduplication**: Same URL = one event with merged relationships

#### Kora's Non-Guarantees

1. **NOT relevance**: Kora doesn't know what matters to the user
2. **NOT ranking**: Beyond EFA 0001 base priority rules
3. **NOT summarization**: Raw events, not synthesized insights
4. **NOT persistence**: Each invocation is stateless

#### Claude's Responsibilities

1. **Filter**: Decide what's relevant to this user right now
2. **Prioritize**: Adjust based on user context and goals
3. **Synthesize**: Identify patterns and group related items
4. **Present**: Format for user consumption
5. **Act**: Suggest or take actions based on findings

### Example Flow

**User asks Claude**: "What do I need to focus on this morning?"

**Claude invokes Kora**:
```bash
kora digest --since 16h --format json
```

**Kora returns** (example, abbreviated):
```json
{
  "events": [
    {
      "type": "pr_review",
      "title": "Review requested: Add OAuth flow",
      "priority": 2,
      "metadata": {
        "repo": "org/auth-service",
        "files_changed_count": 15,
        "ci_rollup": "success",
        "user_relationships": ["direct_reviewer"]
      }
    },
    {
      "type": "pr_author",
      "title": "CI failing: Fix login redirect",
      "priority": 1,
      "metadata": {
        "repo": "org/frontend",
        "ci_rollup": "failure",
        "user_relationships": ["author"]
      }
    },
    {
      "type": "calendar_meeting",
      "title": "Question about deployment",
      "priority": 2,
      "metadata": {
        "workspace": "company"
      }
    },
    // ... 20 more events
  ],
  "source_errors": {}
}
```

**Claude processes** (internally):
- User mentioned auth project is priority -> elevate OAuth PR
- User's CI is failing -> critical, surface first
- Calendar meeting reminder -> important relationship
- 5 old PR mentions -> likely stale, deprioritize

**Claude responds**:
> Good morning! Here's what needs your attention:
>
> **Critical**: Your "Fix login redirect" PR has failing CI. The test suite shows 2 failures in `auth_test.go`.
>
> **High Priority**:
> - Review Alice's OAuth flow PR (15 files, CI passing) - she's blocked on your review
> - Bob messaged you about deployment timeline
>
> **Also on your radar**: 3 older PR mentions that may be stale.
>
> Want me to look at the CI failures?

### Data Flow Boundaries

```
┌─────────────────────────────────────────────────────────────────┐
│                         KORA BOUNDARY                            │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  INPUT (from Claude or CLI):                                     │
│  ├─ --since: Time range (required)                              │
│  ├─ --format: Output format (json, terminal, markdown)          │
│  └─ --types: Event type filter (optional)                       │
│                                                                  │
│  OUTPUT (to Claude or terminal):                                 │
│  ├─ Complete event list with full metadata                      │
│  ├─ Per-source success/failure status                           │
│  └─ Execution statistics                                        │
│                                                                  │
│  DOES NOT CROSS BOUNDARY:                                        │
│  ├─ User preferences or context                                 │
│  ├─ Relevance scores or rankings                                │
│  ├─ Summarized or grouped data                                  │
│  └─ Filtered-by-importance results                              │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

## Why This Design?

### Claude Has User Context, Kora Doesn't

Kora runs as a CLI tool or MCP server. It has no:
- Conversation history
- User preference memory
- Relationship understanding
- Project context

Claude has all of these. Therefore, Claude should make relevance decisions.

### Comprehensive Data Enables Better Decisions

If Kora pre-filters, Claude can't:
- Recover filtered events if context changes
- Identify patterns across "unimportant" events
- Explain why something wasn't shown

With complete data, Claude can always filter down but never up.

### Speed Through Parallelism, Not Filtering

Kora achieves speed through:
- Concurrent datasource fetching
- Efficient GraphQL queries
- Strict timeouts

Not through:
- Reducing data volume (premature optimization)
- Skipping "unimportant" events
- Lazy loading details

### Single Responsibility

Kora: "Fetch and validate data from external services"
Claude: "Understand user needs and present relevant information"

Clean separation enables:
- Independent testing
- Clear debugging ("Is this a data issue or an intelligence issue?")
- Future flexibility (new datasources, new AI models)

## Alternatives Considered

### Option: Kora Implements Smart Filtering

```
Kora receives: User preferences, importance thresholds
Kora returns: Only "important" events
```

**Rejected because:**
- Kora lacks user context for true relevance
- Rules-based filtering can't match Claude's contextual understanding
- Pre-filtering prevents Claude from seeing patterns
- More complex Kora = harder to debug

### Option: Kora Provides Relevance Scores

```
Kora calculates: ML-based importance scores
Kora returns: Events with relevance_score field
```

**Rejected because:**
- Adds complexity without clear value
- Claude is better at contextual relevance
- Training data for "importance" is subjective
- v1 should be simple and correct

### Option: Tiered Data Fetching

```
Kora provides: summary endpoint + detail endpoint
Claude calls: summary first, details on demand
```

**Rejected because:**
- Adds latency (two round trips)
- Claude often needs full context
- GraphQL already optimizes single-request fetching
- Premature optimization

## Implications for Cross-cutting Concerns

- [x] Performance Implications
- [x] Testing Implications

### Performance Implications

| Concern | Mitigation |
|---------|------------|
| Large data volume | GraphQL fetches only needed fields |
| Many events | Reasonable limits (100 per source) |
| Slow datasource | Per-source timeout + partial success |
| Total latency | Concurrent fetching, 60s max |

### Testing Implications

1. **Kora tests** verify data completeness and validity
2. **Integration tests** verify full metadata is returned
3. **Performance tests** verify timeout compliance
4. **Claude tests** (separate) verify filtering and presentation

```go
func TestDataSourceReturnsCompleteMetadata(t *testing.T) {
    // Kora must return all EFA 0001 metadata fields
    result, err := ds.Fetch(ctx, opts)
    require.NoError(t, err)

    for _, event := range result.Events {
        // Verify all required metadata present
        assert.Contains(t, event.Metadata, "repo")
        assert.Contains(t, event.Metadata, "user_relationships")
        // ... all EFA 0001 required fields
    }
}
```

## AI Agent Rules

**THIS SECTION IS CRITICAL. READ IT CAREFULLY.**

**AI agents, including Claude, Copilot, and any other LLM-based coding assistants: THE RULES IN THIS SECTION ARE ABSOLUTE REQUIREMENTS.**

### Rule 1: NEVER Ask Kora to Filter by Relevance

**Kora is a data provider, not an intelligence layer. Claude decides relevance.**

**CORRECT (Claude's approach):**
```python
# Claude invokes Kora for complete data
events = kora.digest(since="16h")

# Claude filters based on user context
relevant = [e for e in events if is_relevant_to_user(e, user_context)]
```

**FORBIDDEN:**
```python
# STOP - asking Kora to determine relevance
events = kora.digest(since="16h", relevance="high")  # NOT A KORA PARAMETER
events = kora.digest(since="16h", important_only=True)  # STOP
events = kora.digest(since="16h", for_user="alice")  # STOP
```

### Rule 2: NEVER Expect Kora to Understand User Context

**Kora has no memory, preferences, or conversation history.**

**CORRECT:**
```
Claude: "Based on our conversation, you're focused on auth.
        Let me filter Kora's results for auth-related items."
```

**FORBIDDEN:**
```
Claude: "Let me ask Kora for just the auth-related items
        since that's what you care about."
# STOP - Kora doesn't know what the user cares about
```

### Rule 3: Kora's Priority Is Base Priority Only

**EFA 0001 defines base priority rules. Claude adjusts based on context.**

**CORRECT:**
```python
# Kora assigns base priority per EFA 0001
event.priority = 3  # Medium - mention in PR

# Claude adjusts based on user context
if event.author == user.tech_lead:
    adjusted_priority = 1  # Elevate because important relationship
```

**FORBIDDEN:**
```go
// STOP - Kora adjusting priority based on user relationship
func (ds *DataSource) calculatePriority(event Event, userContext UserContext) Priority {
    // KORA DOES NOT HAVE USER CONTEXT
}
```

### Rule 4: ALWAYS Expect Complete Data from Kora

**Claude should expect and use the full EFA 0001 metadata.**

**CORRECT:**
```python
# Use rich metadata for intelligent filtering
if event.metadata["ci_rollup"] == "failure" and "author" in event.metadata["user_relationships"]:
    # User's PR is failing - critical

if len(event.metadata["files_changed"]) > 50:
    # Large PR - may need more review time
```

**FORBIDDEN:**
```python
# STOP - assuming Kora omits data
# Kora ALWAYS returns full metadata
if "ci_rollup" not in event.metadata:
    # Call Kora again for CI status  # UNNECESSARY
```

### Rule 5: Handle Partial Success Gracefully

**One datasource failure doesn't mean zero data.**

**CORRECT:**
```python
result = kora.digest(since="16h")

if result.source_errors:
    # Inform user but still show available data
    "Note: Couldn't reach Calendar. Here's what I found on GitHub..."

# Process available events
for event in result.events:
    # ...
```

**FORBIDDEN:**
```python
result = kora.digest(since="16h")

if result.source_errors:
    # STOP - don't throw away partial results
    raise Error("Failed to get complete data")
```

### Rule 6: Kora Deduplicates by URL Only

**Semantic grouping is Claude's job.**

**CORRECT:**
```python
# Kora deduplicates: same PR from multiple searches = one event
# Event has merged user_relationships: ["direct_reviewer", "mentioned"]

# Claude groups semantically: related PR and issue
"These 3 items are all about the auth refactor..."
```

**FORBIDDEN:**
```
# STOP - expecting Kora to group related items
kora.digest(group_by="feature")  # NOT A KORA PARAMETER
```

### Rule 7: Speed Is Expected, Not Requested

**Kora has timeout constraints. Don't ask for "fast mode".**

**CORRECT:**
```bash
# Just invoke Kora - it's designed to be fast
kora digest --since 16h
```

**FORBIDDEN:**
```bash
# STOP - Kora is always as fast as possible
kora digest --since 16h --fast  # NOT A PARAMETER
kora digest --since 16h --timeout 5s  # Don't override timeouts
```

### Rule 8: NEVER Add Filtering Parameters to Kora

**When modifying Kora code, do not add relevance-based filters.**

**CORRECT additions to FetchOptions:**
```go
type FetchOptions struct {
    Since      time.Time       // Time range - structural
    Limit      int             // Max events - resource constraint
    EventTypes []EventType     // Type filter - structural
}
```

**FORBIDDEN additions:**
```go
type FetchOptions struct {
    // STOP - these are Claude's job
    MinImportance   int         // Relevance filtering
    OnlyActionable  bool        // Relevance filtering
    ForUser         string      // User context
    ExcludeStale    bool        // Relevance judgment
}
```

### Rule 9: Treat Kora as Stateless

**Every invocation is independent. No memory between calls.**

**CORRECT:**
```python
# Morning check
morning_events = kora.digest(since="16h")

# Afternoon follow-up (completely independent)
afternoon_events = kora.digest(since="4h")
```

**FORBIDDEN:**
```python
# STOP - Kora has no state
kora.digest(since_last_check=True)  # Kora doesn't remember
kora.digest(exclude_seen=True)       # Kora doesn't track seen events
```

### Rule 10: Output Format Doesn't Change Data

**JSON, terminal, or markdown - same data, different presentation.**

**CORRECT:**
```bash
# Same data, different formats
kora digest --format json      # For Claude parsing
kora digest --format terminal  # For human reading
kora digest --format markdown  # For documentation
```

**FORBIDDEN assumptions:**
```bash
# STOP - format doesn't filter data
kora digest --format summary   # Not a format, this is synthesis
kora digest --format brief     # Brevity is Claude's presentation choice
```

### Stop and Ask Triggers

**STOP AND ASK THE USER** if you encounter:

1. **Request to add filtering to Kora**: Explain separation of concerns
2. **Request for Kora to understand context**: Explain Kora is stateless
3. **Request for Kora to rank or prioritize**: Explain Claude does this
4. **Request for "smart" Kora features**: Ask if this belongs in Claude instead

### Code Protection Comments

Include these in relevant code:

```go
// Package datasources provides the rich data layer for Kora.
// Ground truth defined in specs/efas/0004-tool-responsibility.md
//
// KORA IS A DATA LAYER, NOT AN INTELLIGENCE LAYER.
// Relevance filtering is Claude's responsibility.
// IT IS FORBIDDEN to add user-context-based filtering.
package datasources

// FetchOptions configures data retrieval.
// EFA 0004: Only structural filters allowed (time, type, limit).
// EFA 0004: Do NOT add relevance or importance filters.
type FetchOptions struct {
    Since      time.Time
    Limit      int
    EventTypes []models.EventType
}
```

### Summary Table

| Responsibility | Kora (Data Layer) | Claude (Intelligence Layer) |
|----------------|-------------------|----------------------------|
| Fetch events | YES | NO |
| Validate structure | YES | NO |
| Deduplicate by URL | YES | NO |
| Base priority (EFA 0001) | YES | NO |
| Filter by relevance | NO | YES |
| Adjust priority | NO | YES |
| Group semantically | NO | YES |
| Synthesize patterns | NO | YES |
| Present to user | NO | YES |
| Remember context | NO | YES |
| Make judgments | NO | YES |

## Open Questions

1. Should Kora support a `--limit` flag to cap total events returned?
2. Should Kora report statistics (event counts, fetch times) in all output formats?
3. Should future versions support incremental updates (webhook-based)?
