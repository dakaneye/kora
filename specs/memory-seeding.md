# Memory Seeding: Importing Historical Context

> Populate the memory store with historical data from performance reviews and other sources.

## Overview

This specification defines how users can seed Kora's memory store with historical data. Following EFA 0004's separation of concerns:
- **Claude** (intelligence): Analyzes documents, extracts structured data
- **Kora** (data layer): Validates schema, imports records
- **User** (oversight): Reviews extracted data before import

## Data Flow

```
Performance Review Documents
         │
         ▼
    ┌─────────────┐
    │   Claude    │  Reads PDFs/TXT, extracts entities
    │  (analysis) │  Generates JSON matching export format
    └─────────────┘
         │
         ▼
    ┌─────────────┐
    │  JSON File  │  User reviews, edits if needed
    │  (review)   │  Version-controllable artifact
    └─────────────┘
         │
         ▼
    ┌─────────────┐
    │ kora db     │  Validates schema, inserts records
    │ import      │  FTS triggers sync automatically
    └─────────────┘
         │
         ▼
    ┌─────────────┐
    │   SQLite    │  Memory store populated
    │  (storage)  │
    └─────────────┘
```

**Why this flow?**
- User oversight on sensitive performance data
- JSON intermediate format enables audit trail
- Generic import command keeps implementation simple
- Symmetry: import is inverse of export

## Command Interface

### Export (Existing)
```bash
kora db export --format json > backup.json
```

### Import (New)
```bash
# Strict mode (default): Fail on duplicate IDs
kora db import seed-data.json

# Merge mode: Upsert based on ID
kora db import --merge seed-data.json

# Dry run: Validate without committing
kora db import --dry-run seed-data.json

# Combined
kora db import --dry-run --merge performance-review.json
```

## Import Format

Same as export format (symmetry principle):

```json
{
  "exported_at": "2025-12-08T10:00:00Z",
  "schema_version": 1,
  "goals": [
    {
      "id": "goal_fy26q2_001",
      "title": "Reduce rebuild failures across top 15K Maven projects",
      "description": "Systematically eliminate remaining major error categories...",
      "status": "active",
      "priority": 1,
      "target_date": "2025-09-30",
      "tags": "[\"engineering\", \"reliability\", \"q3_2025\", \"performance_review\"]",
      "is_deleted": 0,
      "created_at": "2025-12-08T10:00:00Z",
      "updated_at": "2025-12-08T10:00:00Z"
    }
  ],
  "accomplishments": [
    {
      "id": "acc_fy26q2_001",
      "title": "Built ChainVer from zero to production-ready",
      "description": "Designed and implemented cryptographic artifact verification tool...",
      "impact": "Created reusable verification tool for Chainguard artifacts",
      "source_url": null,
      "accomplished_at": "2025-08-06",
      "tags": "[\"engineering\", \"security\", \"tooling\", \"self_assessment\", \"q2_2025\"]",
      "is_deleted": 0,
      "created_at": "2025-12-08T10:00:00Z",
      "updated_at": "2025-12-08T10:00:00Z"
    }
  ],
  "commitments": [],
  "context": [
    {
      "id": "ctx_fy26q2_feedback_001",
      "entity_type": "general",
      "entity_id": "",
      "title": "AI tooling feedback from manager",
      "body": "While successful at leveraging AI tools, ensure only intended changes are made...",
      "urgency": "medium",
      "source_url": null,
      "tags": "[\"manager_feedback\", \"growth_area\", \"ai\", \"q2_2025\"]",
      "is_deleted": 0,
      "created_at": "2025-12-08T10:00:00Z",
      "updated_at": "2025-12-08T10:00:00Z"
    }
  ]
}
```

## Validation Rules

### Schema Validation (Required)
| Check | Error Message |
|-------|---------------|
| Schema version mismatch | `schema_version 2 does not match database version 1` |
| Missing required field | `goals[0]: missing required field 'title'` |
| Invalid JSON in tags | `goals[0].tags: invalid JSON array` |
| Invalid timestamp | `goals[0].created_at: invalid RFC3339 format` |
| Invalid ID format | `goals[0].id: ID cannot be empty` |

### Business Logic Validation (Required)
| Check | Error Message |
|-------|---------------|
| Invalid status | `goals[0].status: must be one of [active, completed, on_hold]` |
| Invalid priority | `goals[0].priority: must be 1-5, got 10` |
| Invalid entity_type | `context[0].entity_type: must be one of [person, project, repo, team, general]` |
| Duplicate ID in file | `goals: duplicate ID 'goal_001'` |

### Conflict Resolution

**Strict mode (default):**
- Error if ID already exists in database
- User must resolve manually (delete existing or change ID)

**Merge mode (`--merge`):**
- Update if import `updated_at` > database `updated_at`
- Insert if ID doesn't exist
- Skip if import `updated_at` <= database `updated_at`

## ID Generation Strategy

Claude generates IDs with this pattern:
```
{type}_{source}_{sequence}
```

Examples:
- `goal_fy26q2_001` - Goal from FY26 Q2 review, first item
- `acc_fy25_annual_003` - Accomplishment from FY25 annual review
- `ctx_manager_feedback_001` - Context from manager feedback

**Why not UUIDs?**
- Human-readable IDs aid debugging
- Source tracking built into ID
- Deterministic (same review = same IDs) enables re-runs

## Claude Extraction Workflow

### Single-Pass Prompt (For Most Documents)

Claude uses this prompt to extract data:

```markdown
You are extracting structured data from a performance review document.

<document>
{{DOCUMENT_CONTENT}}
</document>

<metadata>
- Review Period: {{REVIEW_PERIOD}} (e.g., Q2 FY26)
- Document Type: {{self_assessment | manager_review}}
- Current Date: {{CURRENT_DATE}}
</metadata>

## Extraction Rules

**Accomplishments** (PAST - completed work):
- Past tense: "delivered", "completed", "achieved", "built"
- Have measurable outcomes or stated impact
- Extract: title, description, impact, accomplished_at, tags

**Goals** (FUTURE - planned work):
- Future tense: "will", "plan to", "target", "aim to"
- Have target dates or success criteria
- Extract: title, description, status, priority, target_date, tags

**Commitments** (PROMISES - specific agreements):
- Explicit promises to specific people
- Action items from feedback discussions
- Extract: title, to_whom, status, due_date, tags

**Context** (BACKGROUND - supporting information):
- Manager/peer feedback about performance patterns
- Growth areas or improvement needs
- Ratings and explanations
- Extract: entity_type, title, body, urgency, tags

## Tag Vocabulary

Domain: engineering, product, infrastructure, security, leadership
Source: self_assessment, manager_feedback, peer_feedback
Time: q1_2025, q2_2025, h1_2025, fy2025
Theme: career_growth, customer_impact, reliability, tooling

## Output Format

Return JSON matching Kora's export format exactly. Include:
- `_extraction` metadata for each item (confidence, source_quote)
- `warnings` array for ambiguous items
```

### Multi-Stage Prompt (For Complex Documents)

For 360 feedback or ambiguous documents, use three stages:
1. **Document Analysis**: Map sections to categories
2. **Entity Extraction**: Pull accomplishments, goals, etc.
3. **Schema Mapping**: Transform to Kora format

See Appendix A for full prompts.

## Implementation Phases

### Phase 1: Core Import Command
1. Parse JSON import file
2. Validate schema version
3. Validate required fields
4. Insert records (fail on duplicates)
5. Report success/failure counts

### Phase 2: Merge Support
1. Add `--merge` flag
2. Implement upsert logic (compare updated_at)
3. Track skipped records
4. Report merge statistics

### Phase 3: Enhanced Validation
1. Business logic validation (enums, ranges)
2. Detailed error reporting with line numbers
3. `--dry-run` mode
4. Warnings for stale data

## Testing Strategy

### Unit Tests
- JSON parsing edge cases
- Validation logic for all rules
- Merge conflict resolution

### Integration Tests
- Full import workflow
- FTS consistency after import
- `kora db validate` passes

### Sample Data
```bash
# Create test fixtures
tests/testdata/import/
  valid_seed.json      # Happy path
  missing_fields.json  # Validation errors
  duplicate_ids.json   # Conflict handling
  malformed.json       # Parse errors
```

## Security Considerations

- Validate JSON size limits (max 10MB)
- Sanitize string inputs
- Verify no SQL injection in tags field
- Check file permissions (0600) after import

## Example User Workflow

```bash
# 1. User asks Claude to analyze performance review
User: "Extract accomplishments and goals from my Q2 performance review"

# 2. Claude reads document, generates JSON
Claude: "I've extracted 5 accomplishments, 3 goals, and 2 context items.
         Here's the JSON for review: [shows JSON]

         Save to: ~/seed-q2-review.json"

# 3. User reviews JSON, makes any edits

# 4. User runs dry-run to validate
$ kora db import --dry-run ~/seed-q2-review.json
Validating: ~/seed-q2-review.json
  Schema version: 1 (OK)
  Goals: 3 valid
  Accomplishments: 5 valid
  Context: 2 valid

Dry run complete. No records imported.
Run without --dry-run to import.

# 5. User imports
$ kora db import ~/seed-q2-review.json
Importing: ~/seed-q2-review.json
  Goals: 3 imported
  Accomplishments: 5 imported
  Context: 2 imported

Import complete. 10 records added.

# 6. Verify
$ kora db stats
$ kora db validate
```

## Open Questions

1. **Stale data warning**: Should import warn if `updated_at` is > 30 days old?
2. **Force flag**: Add `--force` to override merge conflicts?
3. **Changelog**: Generate before/after diff on import?
4. **Batch imports**: Support importing multiple files in one command?

## References

- EFA 0004: Tool Responsibility and Separation of Concerns
- `specs/memory-data.md`: Memory store schema
- `cmd/kora/db.go`: Existing export implementation

---

## Appendix A: Multi-Stage Extraction Prompts

### Stage 1: Document Analysis

```markdown
Analyze this performance review document and identify its structure.

<document>
{{DOCUMENT_CONTENT}}
</document>

Output JSON:
{
  "metadata": {
    "review_period_start": "YYYY-MM-DD",
    "review_period_end": "YYYY-MM-DD",
    "review_type": "self_assessment | manager_review | combined"
  },
  "sections": [
    {
      "category": "ACCOMPLISHMENTS | GOALS | FEEDBACK | GROWTH_AREAS",
      "heading": "section heading",
      "temporal_context": "past | future | mixed"
    }
  ]
}
```

### Stage 2: Entity Extraction

```markdown
Extract entities from the identified sections.

<document>
{{DOCUMENT_CONTENT}}
</document>

<analysis>
{{STAGE_1_OUTPUT}}
</analysis>

For each section, extract the appropriate entities with:
- title, description, impact (accomplishments)
- title, description, target_date, priority (goals)
- title, body, urgency, source (context)
```

### Stage 3: Schema Mapping

```markdown
Transform extracted entities to Kora's memory store schema.

<extracted>
{{STAGE_2_OUTPUT}}
</extracted>

Apply:
- ID generation: {type}_{source}_{sequence}
- Tag vocabulary from approved list
- RFC3339 timestamp formatting
- is_deleted: 0 for all records
```

---

**Status:** Draft
**Author:** Claude (with prompt-engineer, project-task-planner, architect-review agents)
**Created:** 2025-12-08
