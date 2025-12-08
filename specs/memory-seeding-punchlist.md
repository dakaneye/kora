# Memory Seeding Implementation Punchlist

> Concise, ordered tasks for implementing `kora db import` per `specs/memory-seeding.md`

## Principles

- **Truth-focused**: No simulated functionality, only what's needed
- **Hermeneutic**: Each task understood in context of the whole system
- **Agent-driven**: Each task specifies the implementing agent

## Phase 1: Core Import (Strict Mode)

| # | Task | Agent | Details |
|---|------|-------|---------|
| 1 | Add import command flags | `golang-pro` | Add `--merge`, `--dry-run` flags to `cmd/kora/db.go`, register `dbImportCmd` |
| 2 | JSON parsing + schema validation | `golang-pro` | Parse `exportData` struct, validate `schema_version` matches DB |
| 3 | Per-table import with duplicate detection | `golang-pro` | Insert into goals/accomplishments/commitments/context, fail on duplicate ID |
| 4 | Required fields + business logic validation | `golang-pro` | Validate: id, title, timestamps required; status/priority/entity_type enums |
| 5 | Import statistics reporting | `golang-pro` | Report: inserted count per table, validation errors with details |

**Acceptance**: `kora db import seed.json` inserts records, fails on duplicates, reports stats

## Phase 2: Merge Support

| # | Task | Agent | Details |
|---|------|-------|---------|
| 6 | `--merge` upsert logic | `golang-pro` | Compare `updated_at`: update if import newer, skip if older, insert if new |
| 7 | Merge statistics | `golang-pro` | Report: inserted, updated, skipped counts per table |

**Acceptance**: `kora db import --merge seed.json` upserts based on timestamp comparison

## Phase 3: Validation & Testing

| # | Task | Agent | Details |
|---|------|-------|---------|
| 8 | `--dry-run` mode | `golang-pro` | Validate all records without committing, report what would happen |
| 9 | Unit tests | `test-automator` | Test cases: valid import, missing fields, duplicate IDs, malformed JSON, schema mismatch |
| 10 | Test fixtures | `test-automator` | Create `tests/testdata/import/`: valid.json, missing_fields.json, duplicate_ids.json, malformed.json |
| 11 | FTS integration test | `test-automator` | Verify `kora db validate` passes after import, FTS entries match source tables |

**Acceptance**: All tests pass, `make test` green

## Dependencies

```
Phase 1: [1] → [2] → [3,4] → [5]
Phase 2: [5] → [6] → [7]
Phase 3: [7] → [8], [5] → [9,10,11]
```

## Files Modified

- `cmd/kora/db.go` - Add import command (tasks 1-8)
- `tests/unit/db_import_test.go` - Unit tests (task 9)
- `tests/testdata/import/*.json` - Fixtures (task 10)
- `tests/integration/memory_test.go` - FTS test (task 11)

## Reference

- Spec: `specs/memory-seeding.md`
- Schema: `internal/storage/schema.sql`
- Export format: `exportData` struct in `cmd/kora/db.go:750-760`
