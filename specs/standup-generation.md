---
authors: Samuel Dacanay <samuel@dakaneye.com>
state: draft
created: 2025-12-08
---

# Standup Generation Architecture

> Automated generation of daily standup messages for Slack using Kora (data layer) and Claude (intelligence layer).

## Overview

This specification defines how Kora and Claude collaborate to generate daily standup messages for the #eng-ecosystems-standup Slack channel. Following EFA 0004's tool responsibility separation, Kora provides comprehensive work activity data while Claude synthesizes it into a concise, team-appropriate standup message.

## Goals

- **Automate routine standup posts** - Reduce manual effort in writing daily updates
- **Maintain consistency** - Follow established team standup patterns
- **Preserve voice** - Keep the user's communication style
- **Surface critical work** - Highlight completed work, current focus, blockers
- **Enable iteration** - Claude can refine standups based on feedback

## Non-Goals

- **Replace human judgment** - User reviews and approves before posting
- **Hallucinate work** - Only mention activities present in the data
- **Track time** - No task timing or estimation
- **Manage projects** - Standup generation only, not project tracking
- **Replace memory store** - Use existing Kora memory features, not new state

## Data Flow Architecture

```
┌────────────────────────────────────────────────────────────────────────┐
│                            USER                                         │
│  Requests: "Generate my standup for #eng-ecosystems-standup"          │
│  Reviews: Standup draft, provides feedback                             │
│  Posts: Approves and posts to Slack (manually or via Claude)          │
└──────────────────────────┬─────────────────────────────────────────────┘
                           │
                           ▼
┌────────────────────────────────────────────────────────────────────────┐
│                          CLAUDE (Intelligence Layer)                    │
│                                                                         │
│  1. Invoke Kora with time windows:                                     │
│     - kora digest --since 24h (yesterday's work)                       │
│                                                                        │
│                                                                         │
│  2. Query memory store for context:                                    │
│     - Current goals/commitments                                        │
│     - Recent accomplishments                                           │
│     - Team context (who's on the team, projects)                       │
│                                                                         │
│  3. Synthesize standup:                                                │
│     - Group related events (PRs from same project)                     │
│     - Identify completion vs. progress                                 │
│     - Format per team conventions                                      │
│     - Match user's voice/style                                         │
│                                                                         │
│  4. Present draft for review                                           │
│                                                                         │
└──────────────────────────┬─────────────────────────────────────────────┘
                           │
                           │ MCP Tool Call / CLI Invocation
                           ▼
┌────────────────────────────────────────────────────────────────────────┐
│                          KORA (Rich Data Layer)                         │
│                                                                         │
│  Input:                                                                 │
│  - --since 24h (yesterday's activity)                                  │
│  - --since 8h (today's activity)                                       │
│  - --format json                                                       │
│                                                                         │
│  Output (per EFA 0001):                                                │
│  - Complete event list with metadata:                                  │
│    * pr_closed (merged PRs)                                            │
│    * pr_review (reviews completed)                                     │
│    * pr_author (PRs in progress)                                       │
│    * pr_comment_mention (discussions)                                  │
│    * issue_assigned (tasks in progress)                                │
│                                                                         │
│  Memory Store (SQLite via MCP):                                        │
│  - Goals: Current sprint/week goals                                    │
│  - Commitments: Promises made in previous standups                     │
│  - Accomplishments: Completed work for metrics                         │
│  - Context: Team, projects, recent initiatives                         │
│                                                                         │
└──────────────────────────┬─────────────────────────────────────────────┘
                           │
                   ┌───────┴───────┐
                   ▼               ▼
           ┌───────────┐    ┌────────────┐
           │  GitHub   │    │   SQLite   │
           │    API    │    │ Memory DB  │
           └───────────┘    └────────────┘
```

## Standup Message Structure

Based on the team's established patterns (from `eng-ecosystems-standup-history-1-month.txt`):

### Standard Format

```
Previous/Yesterday/Monday:
- :white_check_mark: Completed task (merged PRs, shipped features)
- :white_check_mark: Completed task with details
- Progress on ongoing work (without completion marker)

Today/Next:
- Primary focus area
- Secondary tasks
- Meetings or support work

Blockers/Upcoming:
- Dependencies on others
- Technical blockers
- Upcoming priorities
```

### Team-Specific Patterns Observed

1. **Completion markers**: `:white_check_mark:` for completed work
2. **Progress indicators**: `:chart_with_upwards_trend:` for coverage increases
3. **Grouping**: Related work clustered together
4. **Specificity**: PR links, issue numbers, coverage percentages
5. **Forward-looking**: "Today" section with clear focus
6. **Brevity**: 3-5 items per section typical

## Kora Data Requirements

### Required Time Windows

1. **Yesterday's work** (`--since 24h`)
   - Completed PRs (pr_closed, merged)
   - Reviews completed (pr_review where reviewed)
   - Issues closed

2. **Today's focus** (`--since 8h`)
   - PRs authored (pr_author, open)
   - PRs awaiting review (pr_review, open)
   - Active issues (issue_assigned, open)
   - Recent discussions (pr_comment_mention)

### Event Metadata Used

Per EFA 0001, Claude will use:

**For PR Events:**
- `repo` - Project identification
- `number` - Reference linking
- `state` - Open/merged status
- `merged_at` - Completion timestamp
- `user_relationships` - Author/reviewer role
- `labels` - Feature type (customer, infrastructure, etc)
- `ci_rollup` - Build status
- `files_changed_count` - Scope of work

**For Issue Events:**
- `repo` - Project identification
- `number` - Reference
- `state` - Open/closed
- `labels` - Work category
- `assignees` - Ownership

### Memory Store Schema

Kora's memory store provides persistent context (accessed via SQLite MCP):

```sql
-- Current goals and commitments
CREATE TABLE goals (
    id INTEGER PRIMARY KEY,
    description TEXT NOT NULL,
    status TEXT CHECK(status IN ('active', 'completed', 'blocked')),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    completed_at TIMESTAMP,
    context TEXT -- JSON: project, customer, priority
);

-- Accomplishments for metrics
CREATE TABLE accomplishments (
    id INTEGER PRIMARY KEY,
    description TEXT NOT NULL,
    category TEXT, -- pr_merged, issue_closed, feature_shipped
    date DATE NOT NULL,
    metadata TEXT, -- JSON: url, repo, metrics
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Free-form context
CREATE TABLE context (
    id INTEGER PRIMARY KEY,
    key TEXT UNIQUE NOT NULL, -- current_project, team_focus, blockers
    value TEXT NOT NULL,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Full-text search
CREATE VIRTUAL TABLE memory_search USING fts5(
    content,
    content=goals,
    content_rowid=id
);
```

**Key context entries:**
- `current_project` - What you're focused on this week
- `team_focus` - Team-wide initiatives (customer work, tooling)
- `blockers` - Known dependencies or issues
- `voice_style` - Communication preferences

## Claude Prompt Template

```markdown
# Task: Generate Standup Message

You are helping generate a standup message for #eng-ecosystems-standup at Chainguard.

## Context

**User**: {{user_name}}
**Date**: {{current_date}}
**Team focus**: {{team_focus_from_memory}}

## Data Sources

### Yesterday's Activity (kora digest --since 24h)
{{kora_output_24h}}

### Today's Activity (kora digest --since 8h)
{{kora_output_8h}}

### Current Goals (from memory)
{{current_goals}}

### Recent Accomplishments (from memory)
{{recent_accomplishments}}

## Team Standup Conventions

Based on your team's history:

1. **Structure**: Previous/Yesterday → Today → Blockers/Upcoming
2. **Completion markers**: `:white_check_mark:` for completed work
3. **Specificity**: Include PR/issue numbers, coverage percentages, customer names
4. **Grouping**: Cluster related work (e.g., "worked on customer X" with sub-bullets)
5. **Brevity**: 3-5 items per section
6. **Forward-looking**: Clear statement of today's primary focus

## Instructions

1. **Analyze completed work** (24h window):
   - PRs merged (pr_closed with merged_at)
   - PRs reviewed (pr_review where you reviewed)
   - Issues closed
   - Group by project/customer

2. **Identify in-progress work** (8h window):
   - PRs authored and open
   - PRs awaiting your review
   - Active issues
   - Recent discussions

3. **Match user's voice**:
   - Review previous standups for style
   - Use similar phrasing and structure
   - Include metrics if user typically does

4. **Synthesize, don't list**:
   - "Worked on X customer coverage" (group related PRs)
   - NOT: "Merged PR #123, Merged PR #124, Merged PR #125"

5. **Highlight context from memory**:
   - If user has active goals, show progress
   - If blockers exist, mention them
   - If customer work is priority, emphasize that

## Output Format

Generate a standup message following the team's format:

```
Previous/Yesterday:
[Completed work with :white_check_mark:]
[Progress on ongoing work]

Today:
[Primary focus]
[Secondary tasks]

Blockers/Upcoming:
[Dependencies or blockers if any]
```

## Critical Rules

- **NEVER hallucinate work** - Only mention events in the Kora data
- **NEVER guess metrics** - Only cite coverage/stats from metadata
- **NEVER mention missing data** - If Calendar/Glean unavailable, omit meetings
- **ALWAYS include PR/issue links** - Use URLs from events
- **ALWAYS group related work** - Cluster PRs by project/customer

## Example

User: "Generate my standup"

Claude analyzes:
- Kora data: 3 PRs merged yesterday (all customer X coverage)
- Kora data: 2 PRs in review today (tooling improvements)
- Memory: Current goal = "Increase customer X coverage to 80%"

Output:
```
Previous:
:white_check_mark: Increased customer X coverage from 65% to 78%
- Merged 3 PRs adding configs for numpy, scipy, pandas
:white_check_mark: Reviewed 2 PRs for alf3 tooling improvements

Today:
Finish customer X coverage (target 80%)
- Working on remaining scikit-learn dependencies
Help onboard @new_teammate with rebuilder workflow

Upcoming:
Start work on customer Y analysis next week
```
```

## Implementation Workflow

### Step 1: User Invokes

```bash
# Via Claude Code conversation
User: "Generate my standup for #eng-ecosystems-standup"
```

### Step 2: Claude Gathers Data

Claude executes in parallel:

1. **Fetch recent activity**:
   ```bash
   kora digest --since 24h --format json
   kora digest --since 8h --format json
   ```

2. **Query memory store** (via SQLite MCP):
   ```sql
   -- Current goals
   SELECT description, status FROM goals WHERE status = 'active';

   -- Recent accomplishments
   SELECT description, category, metadata
   FROM accomplishments
   WHERE date >= date('now', '-7 days');

   -- Context
   SELECT key, value FROM context
   WHERE key IN ('current_project', 'team_focus', 'blockers');
   ```

### Step 3: Claude Synthesizes

Claude applies the prompt template:

1. **Identify completions**: PR events with `merged_at`, `state=merged`
2. **Group related work**: PRs with same `labels` or `repo`
3. **Match voice**: Reference user's previous standups from memory
4. **Format output**: Follow team conventions

### Step 4: Present Draft

```
Claude: Here's your standup draft. I found 3 merged PRs from yesterday
(all customer X coverage) and 2 PRs in review today. Does this look right?

---
[Standup message]
---

Would you like me to:
1. Post this to Slack
2. Revise anything
3. Save key items to your goals
```

### Step 5: User Reviews and Posts

User can:
- **Approve**: Claude posts to Slack (if Slack integration available)
- **Edit**: User refines and posts manually
- **Iterate**: Provide feedback for Claude to revise

## Handling Missing Data

Following EFA 0004, Kora returns partial success when datasources fail:

```json
{
  "events": [...],
  "source_errors": {
    "glean": "authentication failed",
    "calendar": "service unavailable"
  }
}
```

**Claude's response:**
- **DO**: Use available GitHub data
- **DO**: Note gaps to user ("I don't have your calendar, so meetings aren't included")
- **DON'T**: Guess or invent meeting data
- **DON'T**: Fail entirely due to partial data

Example:
```
Claude: I generated your standup based on GitHub activity. Note: I don't
have your calendar data, so meetings aren't included. You may want to add
those manually.

Previous:
[GitHub-based content]

Today:
[GitHub-based content]
- Meetings: [you may want to add these]
```

## Standup Command (Optional Future Enhancement)

If Kora adds a dedicated standup command:

```bash
# Kora provides standup-specific data
kora standup --date today --format json

# Output optimized for standup generation
{
  "completed_yesterday": [
    {
      "type": "pr_merged",
      "repo": "org/repo",
      "number": 123,
      "title": "Add customer X coverage",
      "merged_at": "2025-12-07T18:30:00Z",
      "labels": ["customer-x", "python"]
    }
  ],
  "in_progress_today": [
    {
      "type": "pr_open",
      "repo": "org/repo",
      "number": 456,
      "title": "Improve alf3 tooling",
      "ci_rollup": "success"
    }
  ],
  "reviews_completed": [...],
  "reviews_pending": [...]
}
```

**However**, per EFA 0004, this is **NOT RECOMMENDED**:
- Violates separation of concerns (Kora shouldn't understand "standup")
- Duplicates logic Claude can already perform
- Reduces flexibility (hard-coded standup format)

**Instead**, keep Kora generic:
- Use existing `digest` command with appropriate time windows
- Let Claude determine what's "completed" vs "in progress"
- Claude can evolve standup format without Kora changes

## Example End-to-End Flow

### Scenario

**Date**: Monday, December 9, 2025
**User**: Sam Dacanay
**Context**: Working on JS customer submissions and alf3 tooling

### Step 1: User Invokes

```
Sam: "Generate my standup"
```

### Step 2: Claude Executes

```bash
# Fetch weekend/Friday activity
kora digest --since 72h --format json

# Fetch today's activity
kora digest --since 8h --format json
```

**Kora returns** (abbreviated):
```json
{
  "events": [
    {
      "type": "pr_closed",
      "title": "Merged: Improve alf3 submission tooling",
      "url": "https://github.com/org/rebuilder.js/pull/345",
      "timestamp": "2025-12-06T20:00:00Z",
      "metadata": {
        "repo": "chainguard-dev/ecosystems-rebuilder.js",
        "state": "merged",
        "merged_at": "2025-12-06T20:00:00Z",
        "labels": ["tooling", "alf3"]
      }
    },
    {
      "type": "pr_author",
      "title": "Add alf3 Argo workflow",
      "url": "https://github.com/org/rebuilder.js/pull/350",
      "timestamp": "2025-12-08T10:00:00Z",
      "metadata": {
        "repo": "chainguard-dev/ecosystems-rebuilder.js",
        "state": "open",
        "ci_rollup": "success"
      }
    }
  ],
  "source_errors": {}
}
```

**Memory query**:
```sql
SELECT description FROM goals WHERE status = 'active';
-- Returns: "Complete alf3 workflow for automated customer submissions"
```

### Step 3: Claude Synthesizes

```
Claude: Here's your standup draft based on Friday/weekend activity:

---
Friday/This weekend:
:white_check_mark: Improved alf3 submission tooling (merged PR #345)
Continued work on alf3 Argo workflow (PR #350 in review)
Preparation for automated customer submissions

Today:
Finalize alf3 Argo workflow and test with sample customers
Work with Charlie on infrastructure setup
---

Does this capture your work? I noticed you're close to completing your
goal of automating customer submissions.
```

### Step 4: Sam Reviews

```
Sam: "Looks good, but add that I submitted several customers last week"
```

### Step 5: Claude Refines

```
Claude: Updated:

---
Friday/This weekend:
:white_check_mark: Improved alf3 submission tooling (merged PR #345)
:white_check_mark: Submitted several JS customers for coverage analysis
Continued work on alf3 Argo workflow (PR #350 in review)

Today:
Finalize alf3 Argo workflow and test with sample customers
Work with Charlie on infrastructure setup
Analyze results from weekend submissions
---

Ready to post?
```

### Step 6: Sam Approves

```
Sam: "Post it"

Claude: Posted to #eng-ecosystems-standup ✓

I've also saved "Analyze results from weekend submissions" to your
active goals for tracking.
```

## Limitations & Mitigations

### Limitation 1: No Calendar Data (v1)

**Impact**: Can't mention meetings automatically

**Mitigation**:
- Claude prompts: "Any meetings today?"
- User adds manually or skips
- Memory store can track recurring meetings

**Future**: Add Google Calendar datasource per `specs/google-datasources.md`

### Limitation 2: No Slack Data (v1)

**Impact**: Can't mention DMs, channel discussions

**Mitigation**:
- Focus on GitHub activity (PRs, issues)
- User adds communication context manually
- Memory store can track important discussions

**Future**: Add Glean datasource per `specs/glean-datasource.md`

### Limitation 3: Coverage Metrics Not in Kora

**Impact**: Can't auto-report "Coverage increased from X% to Y%"

**Mitigation**:
- Claude asks: "Any coverage metrics to report?"
- User provides or skips
- Memory store can track customer coverage history

**Future**: Consider adding coverage tracking to memory store

### Limitation 4: Team Context Not Automatic

**Impact**: Claude doesn't know current team priorities

**Mitigation**:
- User populates memory store with team context
- Claude prompts for context first time
- Update memory store during sprint planning

**Example memory entry**:
```sql
INSERT INTO context (key, value) VALUES
('team_focus', 'Customer coverage increases for P1 customers'),
('current_sprint', 'Q4 2025 - Focus on automation tooling');
```

### Limitation 5: Voice Consistency Requires Training

**Impact**: Claude's first standup may not match user's style

**Mitigation**:
- Provide example standups in prompt template
- User corrects first few iterations
- Memory store can save "voice preferences"

**Example voice preferences**:
```sql
INSERT INTO context (key, value) VALUES
('voice_style', 'Brief, technical, use emoji sparingly. Always include PR links. Group work by customer/project.');
```

## Memory Store Integration

### Required Memory Store Commands

Following EFA 0004, Kora provides schema management, Claude accesses directly:

```bash
# Initialize memory store (if not exists)
kora init

# User can inspect
kora db stats
kora db path  # ~/.kora/data/kora.db
```

### Claude Access via SQLite MCP

Claude uses the SQLite MCP server configured in `specs/memory-mcp.md`:

```json
{
  "mcpServers": {
    "kora-memory": {
      "command": "mcp-sqlite",
      "args": ["--db-path", "/Users/{{user}}/.kora/data/kora.db"]
    }
  }
}
```

**Claude queries**:
```sql
-- Get active goals
SELECT description, status FROM goals WHERE status = 'active';

-- Add accomplishment
INSERT INTO accomplishments (description, category, date, metadata)
VALUES (
  'Increased customer X coverage to 80%',
  'coverage_milestone',
  date('now'),
  json_object('customer', 'customer-x', 'coverage', '80%')
);

-- Update context
INSERT OR REPLACE INTO context (key, value)
VALUES ('current_project', 'alf3 automation tooling');
```

### Memory Store Workflow

1. **First-time setup** (user-initiated):
   ```bash
   kora init
   ```

2. **Populate initial context** (Claude helps):
   ```
   Claude: To generate better standups, let me ask a few questions:

   1. What's your current focus area?
   2. Are there team-wide priorities I should know?
   3. Do you have any active blockers?

   I'll save these to your memory store.
   ```

3. **Ongoing updates** (automatic):
   - Claude saves accomplishments after each standup
   - User can manually update goals: "Remember I'm focused on customer Y now"
   - Weekly reviews: "What goals did I complete this week?"

4. **Validation** (user-initiated):
   ```bash
   kora db validate  # Check integrity
   kora db export > backup.sql  # Backup
   ```

## Success Metrics

### Effectiveness
- **Time saved**: <5 minutes to generate and post standup (vs. 10-15 manual)
- **Accuracy**: >90% of standups require no manual edits
- **Coverage**: Captures all work visible in GitHub events
- **Consistency**: Follows team format 100% of time

### Adoption
- **Usage rate**: User generates standup 3+ times per week
- **Satisfaction**: User prefers automated vs. manual after 1 week
- **Iteration**: User refines voice/style within first 5 generations

### Quality
- **No hallucinations**: 0 instances of invented work
- **Complete data**: <10% of standups missing expected items
- **Team feedback**: Teammates find standups clear and consistent

## Future Enhancements

### Phase 2: Calendar Integration
- Add Google Calendar datasource (see `specs/google-datasources.md`)
- Include meetings in "Today" section
- Suggest meeting prep based on calendar events

### Phase 3: Slack Integration
- Post directly to Slack (no copy-paste)
- Read previous standups for voice consistency
- Notify team of blockers automatically

### Phase 4: Metrics Tracking
- Store coverage percentages in memory
- Auto-report progress on goals
- Generate weekly summaries

### Phase 5: Glean Integration
- Include Slack discussions (see `specs/glean-datasource.md`)
- Mention important DMs or threads
- Surface cross-team collaborations

## Open Questions

1. **Should Kora provide a `standup` command?**
   - Recommendation: **No** - Violates EFA 0004 separation
   - Use existing `digest` command with appropriate time windows

2. **How to handle multi-day gaps (weekends, holidays)?**
   - Recommendation: Claude detects gap, adjusts language ("Last week" vs. "Yesterday")
   - User can specify: "Generate standup for Monday after long weekend"

3. **Should standups auto-post or require approval?**
   - Recommendation: **Require approval** - User reviews before posting
   - Future: Optional auto-post with user preference

4. **How to track coverage metrics not in Kora?**
   - Recommendation: Memory store for coverage tracking
   - Future: Consider Kora integration with coverage database

5. **How to handle team-specific formats?**
   - Recommendation: Store format preferences in memory
   - Claude adapts prompt template per team conventions

## Conclusion

This architecture follows EFA 0004's principle: **Kora provides rich data, Claude provides intelligence**. By keeping Kora generic (digest command) and letting Claude understand standup conventions, we maintain clean separation while enabling flexible, team-appropriate standup generation.

The system starts simple (GitHub events only) and can grow (Calendar, Slack, Glean) without requiring Kora to understand "standups". This approach maximizes flexibility while maintaining architectural integrity.

## References

- **EFA 0004**: Tool Responsibility and Separation of Concerns
- **EFA 0001**: Event Model Ground Truth
- `specs/memory-data.md`: Memory store schema and usage
- `specs/memory-mcp.md`: SQLite MCP configuration
- `specs/glean-datasource.md`: Future Slack/message integration
- `specs/google-datasources.md`: Future Calendar integration
- `eng-ecosystems-standup-history-1-month.txt`: Team standup patterns
