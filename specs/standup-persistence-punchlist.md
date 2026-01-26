# Standup Persistence Implementation Punchlist

## Overview

This punchlist implements standup persistence in Kora's memory store through **three specialist agents**:

1. **sql-pro** - Database schema and migrations
2. **golang-pro** - Kora CLI implementation
3. **prompt-engineer** - Claude command updates

**CRITICAL**: Each task MUST be delegated to the specified agent. Claude Code should NOT attempt implementation directly.

The implementation adds:
- New `standups` table with FTS5 support
- Migration infrastructure (v1 → v2)
- Database commands (migrate, export, import)
- Enhanced standup command with persistence

All changes follow Kora's design principles:
- **Truth first** - Database is source of truth for standup history
- **Idempotent operations** - Safe to run commands multiple times
- **EFA compliance** - Respects existing EFAs (0001, 0004)
- **Graceful degradation** - Works even if persistence fails

## Prerequisites

Before starting implementation:
- [ ] Kora memory store initialized (`~/.kora/data/kora.db`)
- [ ] Current schema version is 1
- [ ] No pending migrations
- [ ] MCP server stopped (to avoid database locks)
- [ ] Tests passing: `go test ./...`

## Phase 1: Database Schema Migration (sql-pro)

**Agent**: `sql-pro`
**Context**: Kora uses SQLite with FTS5, soft deletes, and additive-only migrations

### Task 1.1: Update Schema Version Constant
**Agent**: `golang-pro`
**File**: `/Users/samueldacanay/dev/personal/kora/internal/storage/store.go`
**Changes**:
- Update `SchemaVersion` from `1` to `2` (line 27)

**Verification**:
```bash
grep "SchemaVersion = 2" internal/storage/store.go
```

**Dependencies**: None
**Risk**: Low - compile-time constant

---

### Task 1.2: Design Standups Table Schema
**Agent**: `sql-pro`
**Deliverable**: SQL schema for standups table with:
- All required columns (see context below)
- 3 indexes: date, status, created_at
- Proper constraints and defaults

**Context**:
- Required fields: id (PK), standup_text, date (YYYY-MM-DD), created_at, updated_at
- Optional fields: format, status, sources_used (JSON), sent_at, referenced_* (JSON), tags (JSON)
- Soft delete support: is_deleted INTEGER DEFAULT 0
- Follows pattern from existing tables (goals, commitments, accomplishments)

**Acceptance Criteria**:
- Schema is additive-only (no DROP statements)
- All timestamps use TEXT (RFC3339 format)
- JSON fields use TEXT (validated at application layer)
- Indexes use WHERE is_deleted = 0 for soft deletes

**Dependencies**: Task 1.1 complete
**Risk**: Low - follows established patterns

---

### Task 1.3: Design Update Timestamp Trigger
**Agent**: `sql-pro`
**Deliverable**: Trigger to auto-update `updated_at` on modification

**Pattern** (from existing triggers):
```sql
CREATE TRIGGER standups_update_timestamp
AFTER UPDATE ON standups
BEGIN
    UPDATE standups SET updated_at = datetime('now')
    WHERE id = NEW.id;
END;
```

**Acceptance Criteria**:
- Trigger fires on every UPDATE
- Uses datetime('now') for consistency
- Matches pattern from goals/commitments/accomplishments

**Dependencies**: Task 1.2 complete
**Risk**: Low - standard trigger pattern

---

### Task 1.4: Design FTS5 Synchronization Triggers
**Agent**: `sql-pro`
**Deliverable**: 3 triggers for FTS5 sync (INSERT, UPDATE, soft DELETE)

**Requirements**:
- Insert trigger: Add to memory_search with content='standup'
- Update trigger: Delete old + insert new (WHEN NEW.is_deleted = 0)
- Delete trigger: Remove from memory_search (WHEN NEW.is_deleted = 1 AND OLD.is_deleted = 0)
- Use title=date, body=standup_text for FTS5 fields

**Pattern** (from existing triggers):
```sql
CREATE TRIGGER standups_fts_insert
AFTER INSERT ON standups
BEGIN
    INSERT INTO memory_search(content, title, body, tags)
    VALUES ('standup', NEW.date, NEW.standup_text, COALESCE(NEW.tags, ''));
END;
```

**Acceptance Criteria**:
- 3 triggers: insert, update, delete
- Matches pattern from goals/commitments/accomplishments/context
- Uses COALESCE for nullable fields
- Proper WHEN conditions for update/delete

**Dependencies**: Task 1.3 complete
**Risk**: Low - mirrors existing FTS patterns

---

### Task 1.5: Add Schema to schema.sql
**Agent**: `sql-pro`
**File**: `/Users/samueldacanay/dev/personal/kora/internal/storage/schema.sql`
**Changes**:
- Update schema version INSERT to `'2'` (line 14)
- Add standups table definition after context table
- Add all 4 triggers (update timestamp + 3 FTS)

**Verification**:
```bash
sqlite3 :memory: < internal/storage/schema.sql
sqlite3 :memory: ".schema standups"
```

**Dependencies**: Tasks 1.2, 1.3, 1.4 complete
**Risk**: Low - additive schema change

---

### Task 1.6: Create Migration Function
**Agent**: `golang-pro`
**File**: `/Users/samueldacanay/dev/personal/kora/internal/storage/migrations.go`
**Context**: Use sql-pro's schema from Tasks 1.2-1.5

**Changes**:
- Add migration entry to `migrations` slice (line 26):
  ```go
  {Version: 2, Name: "add_standups_table", Up: migrateV2AddStandups},
  ```
- Implement `migrateV2AddStandups` function
- Use `ExecMigrationSQL()` helper for validation
- Split into separate statements (table, indexes, triggers)

**Acceptance Criteria**:
- Migration is idempotent (safe to run twice)
- Uses ExecMigrationSQL for destructive pattern detection
- Returns descriptive errors
- Follows pattern from example migrations in file

**Verification**:
```bash
go build ./internal/storage
```

**Dependencies**: Task 1.5 complete
**Risk**: Medium - migration logic must be correct

---

### Task 1.7: Test Migration on Clean Database
**Agent**: `golang-pro`
**Test**: Create new database and verify schema version 2

```bash
rm -f /tmp/test-kora.db
./bin/kora init --path /tmp/test-kora.db

sqlite3 /tmp/test-kora.db "SELECT value FROM _meta WHERE key = 'schema_version';"
# Expected: 2

sqlite3 /tmp/test-kora.db ".schema standups"
# Expected: Full table definition

sqlite3 /tmp/test-kora.db "SELECT name FROM sqlite_master WHERE type='trigger' AND tbl_name='standups';"
# Expected: 4 triggers

rm -f /tmp/test-kora.db
```

**Dependencies**: Task 1.6 complete
**Risk**: Medium - end-to-end schema verification

---

### Task 1.8: Test Migration from Schema v1 to v2
**Agent**: `golang-pro`
**Test**: Migrate existing v1 database to v2

```bash
# Create v1 database
sqlite3 /tmp/test-kora-v1.db < <(head -n 285 internal/storage/schema.sql)

# Insert test data
sqlite3 /tmp/test-kora-v1.db <<EOF
INSERT INTO goals (id, title, status, priority, created_at, updated_at)
VALUES ('goal-1', 'Test', 'active', 1, datetime('now'), datetime('now'));
EOF

# Run migration (to be implemented in Phase 2)
./bin/kora db migrate --path /tmp/test-kora-v1.db

# Verify schema version
sqlite3 /tmp/test-kora-v1.db "SELECT value FROM _meta WHERE key = 'schema_version';"
# Expected: 2

# Verify existing data preserved
sqlite3 /tmp/test-kora-v1.db "SELECT COUNT(*) FROM goals;"
# Expected: 1

# Verify new table exists
sqlite3 /tmp/test-kora-v1.db ".schema standups"
# Expected: Full schema

rm -f /tmp/test-kora-v1.db
```

**Dependencies**: Task 1.7 complete, Phase 2 Task 2.4 complete
**Risk**: High - must not corrupt existing data

---

## Phase 2: Kora CLI Commands (golang-pro)

**Agent**: `golang-pro`
**Context**: Kora uses Cobra for CLI, SQLite for storage, follows Go best practices

### Task 2.1: Add "standups" to Tables List
**Agent**: `golang-pro`
**File**: `/Users/samueldacanay/dev/personal/kora/cmd/kora/db.go`
**Changes**:
- Update `tables` slice (line 31):
  ```go
  var tables = []string{"goals", "commitments", "accomplishments", "context", "standups"}
  ```

**Verification**:
```bash
./bin/kora db stats
# Should show "standups" in table list with 0 rows
```

**Dependencies**: Phase 1 complete
**Risk**: Low - read-only change

---

### Task 2.2: Add "db migrate" Command
**Agent**: `golang-pro`
**File**: `/Users/samueldacanay/dev/personal/kora/cmd/kora/db.go`

**Requirements**:
- Add `dbMigrateCmd` Cobra command
- Use `storage.Store.Migrate()` method
- Show current version, target version, pending migrations
- Report success/failure clearly
- Wire up in `init()` function

**Behavior**:
```
$ kora db migrate
Current version: 1
Target version: 2

Pending migrations:
  - [2] add_standups_table

Successfully migrated to version 2
```

**Acceptance Criteria**:
- Idempotent (safe to run multiple times)
- Shows "Database is up to date" if no pending migrations
- Returns error on migration failure
- Uses 60s context timeout

**Verification**:
```bash
./bin/kora db migrate --help
./bin/kora db migrate  # On up-to-date DB
# Expected: "Database is up to date (version 2)"
```

**Dependencies**: Phase 1 complete
**Risk**: Medium - migration orchestration

---

### Task 2.3: Update init Command to Call Migrations
**Agent**: `golang-pro`
**File**: `/Users/samueldacanay/dev/personal/kora/cmd/kora/init.go`

**Changes**: Replace TODO section (lines 104-110) with actual migration call:
```go
if ver < storage.SchemaVersion {
	fmt.Printf("Migrating from version %d to version %d...\n", ver, storage.SchemaVersion)

	store, err := storage.NewStore(dbPath)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer store.Close()

	if err := store.Migrate(ctx); err != nil {
		return fmt.Errorf("migration failed: %w", err)
	}

	fmt.Printf("Migrated to version %d\n", storage.SchemaVersion)
	fmt.Printf("Path: %s\n", dbPath)
	return nil
}
```

**Verification**:
```bash
# Create v1 database
sqlite3 /tmp/test-init.db < <(head -n 285 internal/storage/schema.sql)

# Run init (should migrate)
./bin/kora init --path /tmp/test-init.db
# Expected: "Migrating from version 1 to version 2..."

# Verify
sqlite3 /tmp/test-init.db "SELECT value FROM _meta WHERE key = 'schema_version';"
# Expected: 2

rm -f /tmp/test-init.db
```

**Dependencies**: Task 2.2 complete
**Risk**: Medium - critical path for upgrades

---

### Task 2.4: Add Export Support for Standups
**Agent**: `golang-pro`
**File**: `/Users/samueldacanay/dev/personal/kora/cmd/kora/db.go`

**Changes**:
1. Update `exportData` struct to add `Standups []map[string]any`
2. Update `exportJSON` to call `exportTable(ctx, db, "standups")`
3. Update `exportMarkdown` to add Standups section with fields: date, status, format, standup_text

**Verification**:
```bash
# Create test standup
sqlite3 ~/.kora/data/kora.db <<EOF
INSERT INTO standups (id, standup_text, date, format, status, created_at, updated_at)
VALUES ('test-1', 'Test standup', '2025-12-09', 'markdown', 'sent', datetime('now'), datetime('now'));
EOF

# Export to JSON
./bin/kora db export --format json | jq '.standups'
# Expected: Array with 1 standup

# Export to markdown
./bin/kora db export --format md | grep -A5 "## Standups"
# Expected: Standups section with test standup

# Cleanup
sqlite3 ~/.kora/data/kora.db "DELETE FROM standups WHERE id = 'test-1';"
```

**Dependencies**: Task 2.1 complete
**Risk**: Low - mirrors existing export patterns

---

### Task 2.5: Add Import Support for Standups
**Agent**: `golang-pro`
**File**: `/Users/samueldacanay/dev/personal/kora/cmd/kora/db.go`

**Changes**:
1. Update `importStats` struct with Standups{Inserted,Updated,Skipped}
2. Update helper methods (tableInserted, tableUpdated, tableSkipped, totals)
3. Add `validateStandupRecord` function
4. Add `importStandups` function
5. Add `upsertStandups` function
6. Update `validateImportData` to validate standups
7. Update `checkDuplicateIDs` to check standups
8. Update `runDBImport` to import/upsert standups

**Validation rules**:
- Required fields: id, standup_text, date, created_at, updated_at
- Valid formats: "terminal", "markdown", "slack"
- Valid statuses: "draft", "sent", "archived"
- Date format: YYYY-MM-DD (use parseTimestamp helper)

**Verification**:
```bash
# Create test import file
cat > /tmp/test-standups.json <<EOF
{
  "exported_at": "2025-12-09T10:00:00Z",
  "schema_version": 2,
  "goals": [],
  "commitments": [],
  "accomplishments": [],
  "context": [],
  "standups": [
    {
      "id": "test-import-1",
      "standup_text": "Test import",
      "date": "2025-12-09",
      "format": "markdown",
      "status": "draft",
      "is_deleted": 0,
      "created_at": "2025-12-09T10:00:00Z",
      "updated_at": "2025-12-09T10:00:00Z"
    }
  ]
}
EOF

# Test import
./bin/kora db import /tmp/test-standups.json
# Expected: "Standups: 1 imported"

# Verify
sqlite3 ~/.kora/data/kora.db "SELECT COUNT(*) FROM standups WHERE id = 'test-import-1';"
# Expected: 1

# Test merge (no changes)
./bin/kora db import --merge /tmp/test-standups.json
# Expected: "Standups: 0 inserted, 0 updated, 1 skipped"

# Cleanup
sqlite3 ~/.kora/data/kora.db "DELETE FROM standups WHERE id = 'test-import-1';"
rm -f /tmp/test-standups.json
```

**Dependencies**: Task 2.4 complete
**Risk**: High - complex validation and merge logic

---

### Task 2.6: Update db validate Command for Standups
**Agent**: `golang-pro`
**File**: `/Users/samueldacanay/dev/personal/kora/cmd/kora/db.go`

**Changes**:
1. Update `checkRequiredFields` to add standup checks:
   - standups.standup_text
   - standups.date
   - standups.created_at
   - standups.updated_at
2. Update `checkFTSConsistency` to add "standup": "standups" mapping

**Verification**:
```bash
# Run validate (should pass)
./bin/kora db validate
# Expected: All checks passed

# Insert invalid standup (NULL date)
sqlite3 ~/.kora/data/kora.db <<EOF
INSERT INTO standups (id, standup_text, created_at, updated_at)
VALUES ('bad', 'test', datetime('now'), datetime('now'));
EOF

# Run validate (should fail)
./bin/kora db validate
# Expected: "standups.date: N rows with NULL"

# Cleanup
sqlite3 ~/.kora/data/kora.db "DELETE FROM standups WHERE id = 'bad';"
```

**Dependencies**: Task 2.1 complete
**Risk**: Low - validation enhancement

---

## Phase 3: Claude Command Updates (prompt-engineer)

**Agent**: `prompt-engineer`
**Context**: Claude command uses MCP for database access, Kora CLI for GitHub data

### Task 3.1: Add Command Arguments Documentation
**Agent**: `prompt-engineer`
**File**: `/Users/samueldacanay/.claude/commands/standup.md`

**Changes**: Update arguments section (lines 3-9) to add:
```yaml
arguments:
  - name: history
    description: Show past standups (number of days, e.g., --history 7)
    required: false
  - name: view-date
    description: View specific standup by date (YYYY-MM-DD)
    required: false
  - name: overwrite
    description: Allow overwriting existing standup for date
    required: false
  - name: status
    description: Filter by status - draft, sent, archived
    required: false
  - name: search
    description: Search standup history using FTS5
    required: false
```

**Dependencies**: Phase 2 complete
**Risk**: None - documentation only

---

### Task 3.2: Add Pre-Generation Check Section
**Agent**: `prompt-engineer`
**File**: `/Users/samueldacanay/.claude/commands/standup.md`

**Changes**: Add new section after "Workflow" header, before Step 1:

```markdown
### Pre-Generation Check

Before generating a standup, check if one already exists for the target date:

```sql
SELECT id, status, created_at, standup_text
FROM standups
WHERE date = ? AND is_deleted = 0
LIMIT 1;
```

**Handling existing standups**:

1. **If standup exists AND --overwrite NOT specified**:
   ```
   A standup already exists for {date} (status: {status}).
   Created: {created_at}

   {standup_text preview (first 200 chars)}

   Options:
   - View full standup: /standup --view-date {date}
   - Overwrite: /standup --date {date} --overwrite
   - Edit in place: I can help you update specific sections
   ```

2. **If standup exists AND --overwrite specified**:
   - Proceed with generation
   - Save with same ID (UPDATE, not INSERT)
   - Preserve original created_at

3. **If standup does NOT exist**:
   - Proceed with normal generation workflow
```

**Dependencies**: Task 3.1 complete
**Risk**: Low - read-only check

---

### Task 3.3: Update Save Workflow in Output Actions
**Agent**: `prompt-engineer`
**File**: `/Users/samueldacanay/.claude/commands/standup.md`

**Changes**: Replace "Output Actions" section (line 238) with enhanced save logic:

**Key additions**:
- SQL INSERT statement with all fields
- Field value mappings (sources_used, referenced_*, tags)
- Save confirmation message
- Error handling (save fails → still copy to clipboard)
- Instructions for marking as sent

**Acceptance Criteria**:
- Clear SQL query for INSERT
- Documents all field values and how to derive them
- Handles errors gracefully
- Preserves existing clipboard behavior

**Dependencies**: Task 3.2 complete
**Risk**: Medium - must handle database errors gracefully

---

### Task 3.4: Add History Mode Section
**Agent**: `prompt-engineer`
**File**: `/Users/samueldacanay/.claude/commands/standup.md`

**Changes**: Add new section after "Output Actions":

**Requirements**:
- SQL query to fetch standups for last N days
- Display format showing date, status, preview
- View mode for specific date (--view-date)
- Clear instructions for full text retrieval

**Queries needed**:
1. History listing: SELECT with date filter, ORDER BY date DESC
2. Single standup view: SELECT WHERE date = ?
3. Show referenced counts (goals, accomplishments, commitments)

**Dependencies**: Task 3.3 complete
**Risk**: Low - read-only queries

---

### Task 3.5: Add Search Mode Section
**Agent**: `prompt-engineer`
**File**: `/Users/samueldacanay/.claude/commands/standup.md`

**Changes**: Add new section after "History Mode":

**Requirements**:
- FTS5 search query using memory_search JOIN
- Display format with snippets (use snippet() function)
- FTS5 syntax examples (AND, OR, NOT, phrase)
- Ranked results (ORDER BY rank)

**Query pattern**:
```sql
SELECT s.id, s.date, s.status,
       snippet(memory_search, 2, '<mark>', '</mark>', '...', 32) AS snippet
FROM standups s
JOIN memory_search ms ON ms.content = 'standup' AND ms.title = s.date
WHERE ms.body MATCH ?
  AND s.is_deleted = 0
ORDER BY rank
LIMIT 20;
```

**Dependencies**: Task 3.4 complete
**Risk**: Low - leverages existing FTS5 infrastructure

---

### Task 3.6: Add Status Filter Documentation
**Agent**: `prompt-engineer`
**File**: `/Users/samueldacanay/.claude/commands/standup.md`

**Changes**: Update "History Mode" section to add status filtering:

**Requirements**:
- Document --status flag usage
- SQL query with status WHERE clause
- Example usage with --history

**Dependencies**: Task 3.5 complete
**Risk**: None - simple WHERE clause addition

---

## Phase 4: Integration Testing (golang-pro + prompt-engineer)

### Task 4.1: End-to-End Standup Workflow Test
**Agent**: `prompt-engineer` (for test script), `golang-pro` (for any fixes)

**Test complete standup generation, save, and retrieval**:

```bash
# 1. Generate standup (in Claude)
/standup --date 2025-12-09 --format markdown

# 2. Approve (type "good")
# Should save to database and copy to clipboard

# 3. Verify save
sqlite3 ~/.kora/data/kora.db <<EOF
SELECT id, date, status, length(standup_text) as text_length
FROM standups
WHERE date = '2025-12-09' AND is_deleted = 0;
EOF
# Expected: 1 record with status='draft'

# 4. View saved standup
/standup --view-date 2025-12-09
# Expected: Full standup text

# 5. Test overwrite protection
/standup --date 2025-12-09
# Expected: Warning message

# 6. Test overwrite
/standup --date 2025-12-09 --overwrite
# Expected: New standup generated and saved

# 7. Verify history
/standup --history 1
# Expected: Standup for 2025-12-09

# 8. Test search
/standup --search "alf3"
# Expected: Find standup if it mentions alf3

# 9. Cleanup
sqlite3 ~/.kora/data/kora.db "UPDATE standups SET is_deleted = 1 WHERE date = '2025-12-09';"
```

**Dependencies**: Phase 3 complete
**Risk**: High - validates entire feature

---

### Task 4.2: Test Migration Path with Data Preservation
**Agent**: `golang-pro`

**Test verify upgrade from v1 to v2 preserves data**:

```bash
# 1. Create v1 database with test data
sqlite3 /tmp/test-upgrade.db < <(head -n 285 internal/storage/schema.sql)

sqlite3 /tmp/test-upgrade.db <<EOF
INSERT INTO goals (id, title, status, priority, created_at, updated_at)
VALUES ('goal-1', 'Test goal', 'active', 1, datetime('now'), datetime('now'));

INSERT INTO accomplishments (id, title, accomplished_at, created_at, updated_at)
VALUES ('acc-1', 'Test accomplishment', date('now'), datetime('now'), datetime('now'));
EOF

# 2. Verify v1 data
sqlite3 /tmp/test-upgrade.db "SELECT COUNT(*) FROM goals;"
# Expected: 1

# 3. Run migration
./bin/kora db migrate --path /tmp/test-upgrade.db

# 4. Verify schema version upgraded
sqlite3 /tmp/test-upgrade.db "SELECT value FROM _meta WHERE key = 'schema_version';"
# Expected: 2

# 5. Verify existing data preserved
sqlite3 /tmp/test-upgrade.db "SELECT title FROM goals WHERE id = 'goal-1';"
# Expected: Test goal

# 6. Verify new table exists
sqlite3 /tmp/test-upgrade.db ".schema standups"
# Expected: Full schema

# 7. Test insert to new table
sqlite3 /tmp/test-upgrade.db <<EOF
INSERT INTO standups (id, standup_text, date, created_at, updated_at)
VALUES ('test-1', 'Migration test', '2025-12-09', datetime('now'), datetime('now'));
EOF

# 8. Verify FTS trigger fired
sqlite3 /tmp/test-upgrade.db "SELECT COUNT(*) FROM memory_search WHERE content = 'standup';"
# Expected: 1

# 9. Cleanup
rm -f /tmp/test-upgrade.db
```

**Dependencies**: Task 4.1 complete
**Risk**: Critical - data integrity validation

---

### Task 4.3: Test Export/Import Round-Trip
**Agent**: `golang-pro`

**Verify standups survive export/import cycle**:

```bash
# 1. Create standups
sqlite3 ~/.kora/data/kora.db <<EOF
INSERT INTO standups (id, standup_text, date, format, status, sources_used, tags, created_at, updated_at)
VALUES
  ('exp-1', 'Export test 1', '2025-12-09', 'markdown', 'sent', '["kora","memory"]', '["monday"]', datetime('now'), datetime('now')),
  ('exp-2', 'Export test 2', '2025-12-10', 'slack', 'draft', '["kora"]', '[]', datetime('now'), datetime('now'));
EOF

# 2. Export to JSON
./bin/kora db export --format json > /tmp/export-test.json

# 3. Verify standups in export
cat /tmp/export-test.json | jq '.standups | length'
# Expected: 2

# 4. Delete standups
sqlite3 ~/.kora/data/kora.db "DELETE FROM standups WHERE id IN ('exp-1', 'exp-2');"

# 5. Import back
./bin/kora db import /tmp/export-test.json
# Expected: "Standups: 2 imported"

# 6. Verify restored
sqlite3 ~/.kora/data/kora.db <<EOF
SELECT id, standup_text, format, status
FROM standups
WHERE id IN ('exp-1', 'exp-2') AND is_deleted = 0
ORDER BY id;
EOF
# Expected: Both records with correct data

# 7. Test merge mode
./bin/kora db import --merge /tmp/export-test.json
# Expected: "Standups: 0 inserted, 0 updated, 2 skipped"

# 8. Cleanup
sqlite3 ~/.kora/data/kora.db "DELETE FROM standups WHERE id IN ('exp-1', 'exp-2');"
rm -f /tmp/export-test.json
```

**Dependencies**: Task 4.2 complete
**Risk**: High - validates data portability

---

## Verification Checklist

After completing all tasks, verify:

### Database Schema (sql-pro + golang-pro)
- [ ] Schema version is 2
- [ ] `standups` table exists with all columns
- [ ] All 3 indexes exist (date, status, created)
- [ ] All 4 triggers exist (update timestamp, 3 FTS triggers)
- [ ] FTS5 sync works (insert standup → appears in memory_search)
- [ ] Soft delete works (is_deleted=1 removes from FTS)

### Migrations (golang-pro)
- [ ] Clean v2 database initialization works
- [ ] v1 to v2 migration preserves existing data
- [ ] Migration is idempotent (safe to run multiple times)
- [ ] `kora init` detects and runs migrations
- [ ] `kora db migrate` shows pending migrations
- [ ] `kora db migrate` applies migrations successfully

### CLI Commands (golang-pro)
- [ ] `kora db stats` includes standups
- [ ] `kora db validate` checks standups table
- [ ] `kora db export --format json` includes standups
- [ ] `kora db export --format md` includes standups
- [ ] `kora db import` validates standup records
- [ ] `kora db import` inserts standups (strict mode)
- [ ] `kora db import --merge` upserts standups correctly
- [ ] `kora db prune` removes old soft-deleted standups
- [ ] `kora db backup` includes standups table

### Claude Command (prompt-engineer)
- [ ] Pre-generation check detects existing standups
- [ ] Overwrite protection works (warns without --overwrite)
- [ ] Overwrite mode works (--overwrite flag)
- [ ] Save workflow persists standup to database
- [ ] Save generates correct ID and timestamps
- [ ] Save captures referenced IDs (goals, accomplishments, commitments)
- [ ] Save records sources_used array
- [ ] Save adds appropriate tags (e.g., "monday")
- [ ] History mode lists past standups (--history N)
- [ ] View mode displays specific standup (--view-date)
- [ ] Search mode uses FTS5 (--search "query")
- [ ] Status filter works (--status draft|sent|archived)
- [ ] Clipboard copy works on save (pbcopy)
- [ ] Error handling graceful (save fails → still copies to clipboard)

### Integration (golang-pro + prompt-engineer)
- [ ] End-to-end workflow: generate → save → retrieve → search
- [ ] Migration preserves existing memory data
- [ ] Export/import round-trip preserves standups
- [ ] FTS search finds standups correctly
- [ ] Multiple standups for different dates work
- [ ] Same-date overwrites work correctly

### Performance (golang-pro)
- [ ] Query performance acceptable (<100ms for history/search)
- [ ] FTS index size reasonable
- [ ] Migration completes quickly (<5s on populated DB)
- [ ] No database locks during normal operations

## Agent Workflow Summary

### Phase 1: sql-pro → golang-pro
1. **sql-pro** designs schema, triggers (Tasks 1.2-1.5)
2. **golang-pro** implements schema version constant (Task 1.1)
3. **golang-pro** implements migration function (Task 1.6)
4. **golang-pro** tests migration (Tasks 1.7-1.8)

### Phase 2: golang-pro only
5. **golang-pro** implements all CLI commands (Tasks 2.1-2.6)
6. Each task builds on previous (sequential dependencies)

### Phase 3: prompt-engineer only
7. **prompt-engineer** updates Claude command documentation (Tasks 3.1-3.6)
8. Each task adds new section to standup command (sequential)

### Phase 4: golang-pro + prompt-engineer
9. **prompt-engineer** writes E2E test script (Task 4.1)
10. **golang-pro** writes migration test (Task 4.2)
11. **golang-pro** writes export/import test (Task 4.3)

## Success Criteria

The implementation is complete when:

1. **All 30 tasks executed by appropriate agents** - No direct implementation by Claude Code
2. **All agent deliverables accepted** - Each task's acceptance criteria met
3. **All integration tests pass** - End-to-end workflows complete successfully
4. **All 41 verification checklist items pass** - Comprehensive feature validation
5. **Zero data loss** - Existing memory data preserved through migration
6. **Graceful degradation** - Feature works even with partial failures

## Rollback Plan

If critical issues discovered after deployment:

### Rollback Schema (golang-pro)
```bash
# Stop MCP server
killall mcp-server-kora

# Downgrade schema version
sqlite3 ~/.kora/data/kora.db <<EOF
UPDATE _meta SET value = '1' WHERE key = 'schema_version';

DROP TRIGGER IF EXISTS standups_update_timestamp;
DROP TRIGGER IF EXISTS standups_fts_insert;
DROP TRIGGER IF EXISTS standups_fts_update;
DROP TRIGGER IF EXISTS standups_fts_delete;
DROP TABLE IF EXISTS standups;

DELETE FROM memory_search WHERE content = 'standup';
EOF

# Rebuild binary from previous commit
git checkout HEAD~1
make build
```

### Preserve Standups Before Rollback (golang-pro)
```bash
./bin/kora db export --format json > ~/.kora/backups/standups-backup-$(date +%Y%m%d).json
```

## Notes

- **Agent separation is mandatory** - Each task specifies required agent
- Schema migrations are **additive only** - No DROP statements
- FTS5 triggers **automatically maintain** search index
- All timestamps use **RFC3339 format** for consistency
- Soft deletes (`is_deleted=1`) preserve history
- Export/import uses same format as existing tables
- Database operations use **contexts with timeouts**
- All SQL uses **prepared statements** (no injection risk)

## References

- Database design: `specs/standup-persistence.md` (sql-pro)
- Kora integration: `specs/standup-persistence-kora.md` (golang-pro)
- Command design: `specs/standup-persistence-claude.md` (prompt-engineer)
- Memory schema: `internal/storage/schema.sql`
- Migration framework: `internal/storage/migrations.go`
- Existing patterns: `cmd/kora/db.go` (import/export/validate)
