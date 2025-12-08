package storage

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewStore(t *testing.T) {
	t.Run("creates new database", func(t *testing.T) {
		dir := t.TempDir()
		dbPath := filepath.Join(dir, "test.db")

		store, err := NewStore(dbPath)
		if err != nil {
			t.Fatalf("NewStore() error = %v", err)
		}
		defer store.Close()

		// Verify file was created
		info, err := os.Stat(dbPath)
		if err != nil {
			t.Fatalf("database file not created: %v", err)
		}

		// Verify permissions (0600)
		perm := info.Mode().Perm()
		if perm != 0o600 {
			t.Errorf("file permissions = %o, want %o", perm, 0o600)
		}
	})

	t.Run("creates parent directories", func(t *testing.T) {
		dir := t.TempDir()
		dbPath := filepath.Join(dir, "nested", "dirs", "test.db")

		store, err := NewStore(dbPath)
		if err != nil {
			t.Fatalf("NewStore() error = %v", err)
		}
		defer store.Close()

		if _, err := os.Stat(dbPath); err != nil {
			t.Fatalf("database file not created: %v", err)
		}
	})

	t.Run("opens existing database", func(t *testing.T) {
		dir := t.TempDir()
		dbPath := filepath.Join(dir, "test.db")

		// Create first
		store1, err := NewStore(dbPath)
		if err != nil {
			t.Fatalf("first NewStore() error = %v", err)
		}
		store1.Close()

		// Reopen
		store2, err := NewStore(dbPath)
		if err != nil {
			t.Fatalf("second NewStore() error = %v", err)
		}
		defer store2.Close()

		// Verify schema exists
		ctx := context.Background()
		version, err := store2.SchemaVersion(ctx)
		if err != nil {
			t.Fatalf("SchemaVersion() error = %v", err)
		}
		if version != SchemaVersion {
			t.Errorf("SchemaVersion() = %d, want %d", version, SchemaVersion)
		}
	})
}

func TestStore_Close(t *testing.T) {
	t.Run("close is idempotent", func(t *testing.T) {
		dir := t.TempDir()
		dbPath := filepath.Join(dir, "test.db")

		store, err := NewStore(dbPath)
		if err != nil {
			t.Fatalf("NewStore() error = %v", err)
		}

		// Close multiple times should not error
		if err := store.Close(); err != nil {
			t.Errorf("first Close() error = %v", err)
		}
		// Note: sql.DB.Close() returns error on second call, but that's expected
	})
}

func TestStore_SchemaVersion(t *testing.T) {
	t.Run("returns version for initialized database", func(t *testing.T) {
		store := newTestStore(t)
		defer store.Close()

		ctx := context.Background()
		version, err := store.SchemaVersion(ctx)
		if err != nil {
			t.Fatalf("SchemaVersion() error = %v", err)
		}
		if version != SchemaVersion {
			t.Errorf("SchemaVersion() = %d, want %d", version, SchemaVersion)
		}
	})

	t.Run("respects context cancellation", func(t *testing.T) {
		store := newTestStore(t)
		defer store.Close()

		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		_, err := store.SchemaVersion(ctx)
		if err == nil {
			t.Error("SchemaVersion() with canceled context should error")
		}
	})
}

func TestStore_CheckVersion(t *testing.T) {
	t.Run("passes for current version", func(t *testing.T) {
		store := newTestStore(t)
		defer store.Close()

		ctx := context.Background()
		if err := store.CheckVersion(ctx); err != nil {
			t.Errorf("CheckVersion() error = %v", err)
		}
	})

	t.Run("fails for future version", func(t *testing.T) {
		store := newTestStore(t)
		defer store.Close()

		ctx := context.Background()

		// Set a future version
		_, err := store.db.ExecContext(ctx,
			"UPDATE _meta SET value = '999' WHERE key = 'schema_version'")
		if err != nil {
			t.Fatalf("failed to set future version: %v", err)
		}

		err = store.CheckVersion(ctx)
		if err == nil {
			t.Error("CheckVersion() with future version should error")
		}
		if err != nil && !isSchemaVersionError(err) {
			t.Errorf("CheckVersion() error = %v, want ErrSchemaVersionMismatch", err)
		}
	})
}

func TestStore_Migrate(t *testing.T) {
	t.Run("no-op when up to date", func(t *testing.T) {
		store := newTestStore(t)
		defer store.Close()

		ctx := context.Background()
		if err := store.Migrate(ctx); err != nil {
			t.Errorf("Migrate() error = %v", err)
		}

		// Version should still be current
		version, err := store.SchemaVersion(ctx)
		if err != nil {
			t.Fatalf("SchemaVersion() error = %v", err)
		}
		if version != SchemaVersion {
			t.Errorf("SchemaVersion() = %d, want %d", version, SchemaVersion)
		}
	})

	t.Run("fails for future version", func(t *testing.T) {
		store := newTestStore(t)
		defer store.Close()

		ctx := context.Background()

		// Set a future version
		_, err := store.db.ExecContext(ctx,
			"UPDATE _meta SET value = '999' WHERE key = 'schema_version'")
		if err != nil {
			t.Fatalf("failed to set future version: %v", err)
		}

		err = store.Migrate(ctx)
		if err == nil {
			t.Error("Migrate() with future version should error")
		}
	})
}

func TestStore_PendingMigrations(t *testing.T) {
	t.Run("returns empty for up to date", func(t *testing.T) {
		store := newTestStore(t)
		defer store.Close()

		ctx := context.Background()
		pending, err := store.PendingMigrations(ctx)
		if err != nil {
			t.Fatalf("PendingMigrations() error = %v", err)
		}
		if len(pending) != 0 {
			t.Errorf("PendingMigrations() = %d migrations, want 0", len(pending))
		}
	})
}

func TestValidateMigrationSQL(t *testing.T) {
	tests := []struct {
		name    string
		sql     string
		wantErr bool
	}{
		{
			name:    "CREATE TABLE allowed",
			sql:     "CREATE TABLE foo (id TEXT PRIMARY KEY)",
			wantErr: false,
		},
		{
			name:    "CREATE INDEX allowed",
			sql:     "CREATE INDEX idx_foo ON foo(bar)",
			wantErr: false,
		},
		{
			name:    "ALTER TABLE ADD COLUMN allowed",
			sql:     "ALTER TABLE foo ADD COLUMN bar TEXT",
			wantErr: false,
		},
		{
			name:    "INSERT allowed",
			sql:     "INSERT INTO foo (id) VALUES ('test')",
			wantErr: false,
		},
		{
			name:    "DROP TABLE blocked",
			sql:     "DROP TABLE foo",
			wantErr: true,
		},
		{
			name:    "DROP INDEX blocked",
			sql:     "DROP INDEX idx_foo",
			wantErr: true,
		},
		{
			name:    "DROP TRIGGER blocked",
			sql:     "DROP TRIGGER trigger_foo",
			wantErr: true,
		},
		{
			name:    "DROP VIEW blocked",
			sql:     "DROP VIEW view_foo",
			wantErr: true,
		},
		{
			name:    "ALTER TABLE DROP blocked",
			sql:     "ALTER TABLE foo DROP COLUMN bar",
			wantErr: true,
		},
		{
			name:    "ALTER TABLE RENAME blocked",
			sql:     "ALTER TABLE foo RENAME COLUMN bar TO baz",
			wantErr: true,
		},
		{
			name:    "case insensitive DROP TABLE",
			sql:     "drop table foo",
			wantErr: true,
		},
		{
			name:    "case insensitive ALTER DROP",
			sql:     "alter table foo drop column bar",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateMigrationSQL(tt.sql)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateMigrationSQL() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestExecMigrationSQL(t *testing.T) {
	t.Run("executes valid SQL", func(t *testing.T) {
		store := newTestStore(t)
		defer store.Close()

		ctx := context.Background()
		tx, err := store.db.BeginTx(ctx, nil)
		if err != nil {
			t.Fatalf("BeginTx() error = %v", err)
		}
		defer func() { _ = tx.Rollback() }()

		err = ExecMigrationSQL(ctx, tx, "CREATE TABLE test_table (id TEXT PRIMARY KEY)")
		if err != nil {
			t.Errorf("ExecMigrationSQL() error = %v", err)
		}
	})

	t.Run("blocks destructive SQL", func(t *testing.T) {
		store := newTestStore(t)
		defer store.Close()

		ctx := context.Background()
		tx, err := store.db.BeginTx(ctx, nil)
		if err != nil {
			t.Fatalf("BeginTx() error = %v", err)
		}
		defer func() { _ = tx.Rollback() }()

		err = ExecMigrationSQL(ctx, tx, "DROP TABLE goals")
		if err == nil {
			t.Error("ExecMigrationSQL() should block DROP TABLE")
		}
	})
}

func TestStore_Tables(t *testing.T) {
	t.Run("schema creates all tables", func(t *testing.T) {
		store := newTestStore(t)
		defer store.Close()

		ctx := context.Background()
		expectedTables := []string{
			"_meta",
			"goals",
			"commitments",
			"accomplishments",
			"context",
		}

		for _, table := range expectedTables {
			var name string
			err := store.db.QueryRowContext(ctx,
				"SELECT name FROM sqlite_master WHERE type='table' AND name=?",
				table,
			).Scan(&name)
			if err != nil {
				t.Errorf("table %q not found: %v", table, err)
			}
		}
	})

	t.Run("schema creates FTS5 virtual table", func(t *testing.T) {
		store := newTestStore(t)
		defer store.Close()

		ctx := context.Background()
		var name string
		err := store.db.QueryRowContext(ctx,
			"SELECT name FROM sqlite_master WHERE type='table' AND name='memory_search'",
		).Scan(&name)
		if err != nil {
			t.Errorf("FTS5 table memory_search not found: %v", err)
		}
	})
}

func TestStore_BasicOperations(t *testing.T) {
	t.Run("can insert and query goals", func(t *testing.T) {
		store := newTestStore(t)
		defer store.Close()

		ctx := context.Background()
		now := time.Now().UTC().Format(time.RFC3339)

		// Insert a goal
		_, err := store.db.ExecContext(ctx, `
			INSERT INTO goals (id, title, description, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?)
		`, "goal-1", "Test Goal", "A test goal", now, now)
		if err != nil {
			t.Fatalf("insert goal: %v", err)
		}

		// Query it back
		var title string
		err = store.db.QueryRowContext(ctx,
			"SELECT title FROM goals WHERE id = ?", "goal-1",
		).Scan(&title)
		if err != nil {
			t.Fatalf("query goal: %v", err)
		}
		if title != "Test Goal" {
			t.Errorf("title = %q, want %q", title, "Test Goal")
		}
	})

	t.Run("FTS5 syncs on insert", func(t *testing.T) {
		store := newTestStore(t)
		defer store.Close()

		ctx := context.Background()
		now := time.Now().UTC().Format(time.RFC3339)

		// Insert a goal
		_, err := store.db.ExecContext(ctx, `
			INSERT INTO goals (id, title, description, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?)
		`, "goal-fts", "Searchable Goal", "Contains unique keyword", now, now)
		if err != nil {
			t.Fatalf("insert goal: %v", err)
		}

		// Search via FTS5
		var count int
		err = store.db.QueryRowContext(ctx,
			"SELECT COUNT(*) FROM memory_search WHERE memory_search MATCH 'Searchable'",
		).Scan(&count)
		if err != nil {
			t.Fatalf("FTS5 search: %v", err)
		}
		if count != 1 {
			t.Errorf("FTS5 match count = %d, want 1", count)
		}
	})
}

// newTestStore creates a new Store with a temporary database for testing.
func newTestStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	store, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}
	return store
}

// isSchemaVersionError checks if err is or wraps ErrSchemaVersionMismatch.
func isSchemaVersionError(err error) bool {
	return err != nil && contains(err.Error(), "schema version mismatch")
}
