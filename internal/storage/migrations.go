package storage

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"strings"
)

// Migration represents a schema migration.
// Migrations are Go functions that modify the database schema.
//
//nolint:govet // Field order prioritizes semantic grouping over memory alignment
type Migration struct {
	// Version is the schema version this migration upgrades to.
	Version int
	// Name is a human-readable description of the migration.
	Name string
	// Up applies the migration.
	Up func(ctx context.Context, tx *sql.Tx) error
}

// migrations is the list of all migrations in order.
// Each migration upgrades from Version-1 to Version.
var migrations = []Migration{
	// Schema version 1 is created by schema.sql, no migration needed.
	// Future migrations go here, e.g.:
	// {Version: 2, Name: "add_notes_table", Up: migrateV2},
}

// destructivePatterns matches SQL that would delete or alter existing schema.
// Additive-only migrations are enforced to prevent data loss.
var destructivePatterns = regexp.MustCompile(`(?i)\b(DROP\s+(TABLE|INDEX|TRIGGER|VIEW)|ALTER\s+TABLE\s+\w+\s+(DROP|RENAME|MODIFY))\b`)

// SchemaVersion returns the current schema version from the database.
// Returns 0 if the _meta table doesn't exist (uninitialized database).
func (s *Store) SchemaVersion(ctx context.Context) (int, error) {
	var version int
	err := s.db.QueryRowContext(ctx,
		"SELECT CAST(value AS INTEGER) FROM _meta WHERE key = 'schema_version'",
	).Scan(&version)
	if err != nil {
		if err == sql.ErrNoRows {
			return 0, nil
		}
		// Table might not exist
		if isTableNotExist(err) {
			return 0, nil
		}
		return 0, fmt.Errorf("storage: query schema version: %w", err)
	}
	return version, nil
}

// CheckVersion verifies the database schema is compatible with this code.
// Returns ErrSchemaVersionMismatch if the database is newer than the code.
func (s *Store) CheckVersion(ctx context.Context) error {
	version, err := s.SchemaVersion(ctx)
	if err != nil {
		return err
	}

	if version > SchemaVersion {
		return fmt.Errorf("%w: database is version %d, code supports up to %d",
			ErrSchemaVersionMismatch, version, SchemaVersion)
	}

	return nil
}

// Migrate applies all pending migrations to the database.
// Migrations are applied in order, each in its own transaction.
// Returns nil if no migrations are needed.
func (s *Store) Migrate(ctx context.Context) error {
	// Check for lock before starting
	if err := s.checkLock(ctx); err != nil {
		return err
	}

	// Get current version
	currentVersion, err := s.SchemaVersion(ctx)
	if err != nil {
		return fmt.Errorf("storage: get schema version: %w", err)
	}

	// Check forward compatibility
	if currentVersion > SchemaVersion {
		return fmt.Errorf("%w: database is version %d, code supports up to %d",
			ErrSchemaVersionMismatch, currentVersion, SchemaVersion)
	}

	// Apply pending migrations
	for _, m := range migrations {
		if m.Version <= currentVersion {
			continue
		}

		if err := s.applyMigration(ctx, m); err != nil {
			return fmt.Errorf("storage: migration %d (%s): %w", m.Version, m.Name, err)
		}
	}

	return nil
}

// PendingMigrations returns the list of migrations that haven't been applied.
func (s *Store) PendingMigrations(ctx context.Context) ([]Migration, error) {
	currentVersion, err := s.SchemaVersion(ctx)
	if err != nil {
		return nil, err
	}

	var pending []Migration
	for _, m := range migrations {
		if m.Version > currentVersion {
			pending = append(pending, m)
		}
	}
	return pending, nil
}

// applyMigration runs a single migration in a transaction.
func (s *Store) applyMigration(ctx context.Context, m Migration) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck // Best effort on rollback

	// Run the migration
	if upErr := m.Up(ctx, tx); upErr != nil {
		return fmt.Errorf("apply: %w", upErr)
	}

	// Update schema version
	_, err = tx.ExecContext(ctx,
		"UPDATE _meta SET value = ? WHERE key = 'schema_version'",
		fmt.Sprintf("%d", m.Version),
	)
	if err != nil {
		return fmt.Errorf("update version: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit: %w", err)
	}

	return nil
}

// checkLock verifies the database isn't locked by another process.
func (s *Store) checkLock(ctx context.Context) error {
	// Try to acquire an exclusive lock briefly
	ctx, cancel := context.WithTimeout(ctx, defaultLockTimeout)
	defer cancel()

	// Execute a simple write to test for locks
	_, err := s.db.ExecContext(ctx, "BEGIN EXCLUSIVE; ROLLBACK;")
	if err != nil {
		if isLockError(err) {
			return fmt.Errorf("%w: close other connections (e.g., MCP server) and retry", ErrDatabaseLocked)
		}
		return fmt.Errorf("storage: check lock: %w", err)
	}
	return nil
}

// ValidateMigrationSQL checks that SQL doesn't contain destructive operations.
// Use this in migration functions to catch accidental DROP/ALTER statements.
func ValidateMigrationSQL(stmt string) error {
	if destructivePatterns.MatchString(stmt) {
		matches := destructivePatterns.FindStringSubmatch(stmt)
		return fmt.Errorf("%w: found %q in migration SQL", ErrDestructiveMigration, matches[0])
	}
	return nil
}

// ExecMigrationSQL is a helper for migrations that executes SQL after validation.
// Returns ErrDestructiveMigration if the SQL contains DROP or destructive ALTER.
func ExecMigrationSQL(ctx context.Context, tx *sql.Tx, stmt string) error {
	if err := ValidateMigrationSQL(stmt); err != nil {
		return err
	}
	_, err := tx.ExecContext(ctx, stmt)
	return err
}

// isTableNotExist checks if an error indicates a missing table.
func isTableNotExist(err error) bool {
	if err == nil {
		return false
	}
	errStr := strings.ToLower(err.Error())
	return strings.Contains(errStr, "no such table") ||
		strings.Contains(errStr, "table") && strings.Contains(errStr, "not exist")
}
