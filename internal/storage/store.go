// Package storage provides SQLite-based persistence for Kora's memory store.
//
// The storage layer uses modernc.org/sqlite (pure Go, no CGO) and supports:
//   - Schema versioning via _meta table
//   - Forward-compatible migrations (additive only)
//   - Lock detection for safe concurrent access
package storage

import (
	"context"
	"database/sql"
	_ "embed"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite" // Pure Go SQLite driver (no CGO)
)

//go:embed schema.sql
var schemaSQL string

// SchemaVersion is the current schema version supported by this code.
// Increment when adding new migrations.
const SchemaVersion = 1

// Default timeouts for database operations.
const (
	defaultQueryTimeout = 5 * time.Second
	defaultLockTimeout  = 1 * time.Second
)

// Sentinel errors for storage operations.
var (
	// ErrDatabaseLocked indicates another process holds the database lock.
	ErrDatabaseLocked = errors.New("storage: database locked")

	// ErrSchemaVersionMismatch indicates the database schema is newer than this code.
	ErrSchemaVersionMismatch = errors.New("storage: schema version mismatch")

	// ErrDestructiveMigration indicates a migration attempted a destructive operation.
	ErrDestructiveMigration = errors.New("storage: destructive migration detected")
)

// Store manages SQLite database connections for Kora's memory store.
type Store struct {
	db     *sql.DB
	dbPath string
}

// NewStore opens or creates a SQLite database at the given path.
// Creates parent directories and sets file permissions to 0600.
//
// The database location is typically ~/.kora/data/kora.db
func NewStore(dbPath string) (*Store, error) {
	// Ensure parent directory exists
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("storage: create directory %s: %w", dir, err)
	}

	// Check if database already exists
	isNew := false
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		isNew = true
	}

	// Open database with WAL mode for better concurrency
	dsn := fmt.Sprintf("file:%s?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)", dbPath)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("storage: open database: %w", err)
	}

	// Verify connection
	ctx, cancel := context.WithTimeout(context.Background(), defaultLockTimeout)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		db.Close() //nolint:errcheck // Best effort cleanup on error path
		if isLockError(err) {
			return nil, fmt.Errorf("%w: close other connections (e.g., MCP server) and retry", ErrDatabaseLocked)
		}
		return nil, fmt.Errorf("storage: ping database: %w", err)
	}

	// Set file permissions to 0600 (owner read/write only)
	if err := os.Chmod(dbPath, 0o600); err != nil {
		db.Close() //nolint:errcheck // Best effort cleanup on error path
		return nil, fmt.Errorf("storage: chmod %s: %w", dbPath, err)
	}

	s := &Store{
		db:     db,
		dbPath: dbPath,
	}

	// Initialize schema for new databases
	if isNew {
		if err := s.initSchema(ctx); err != nil {
			db.Close() //nolint:errcheck // Best effort cleanup on error path
			return nil, fmt.Errorf("storage: init schema: %w", err)
		}
	}

	return s, nil
}

// Close closes the database connection.
func (s *Store) Close() error {
	if s.db == nil {
		return nil
	}
	if err := s.db.Close(); err != nil {
		return fmt.Errorf("storage: close database: %w", err)
	}
	return nil
}

// DB returns the underlying database connection for advanced queries.
// Use with caution; prefer typed methods when available.
func (s *Store) DB() *sql.DB {
	return s.db
}

// Path returns the database file path.
func (s *Store) Path() string {
	return s.dbPath
}

// initSchema creates the initial database schema.
func (s *Store) initSchema(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, schemaSQL)
	if err != nil {
		return fmt.Errorf("execute schema: %w", err)
	}
	return nil
}

// isLockError checks if an error indicates a database lock.
func isLockError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	// modernc.org/sqlite returns "database is locked" for SQLITE_BUSY
	return contains(errStr, "database is locked") ||
		contains(errStr, "SQLITE_BUSY") ||
		contains(errStr, "database table is locked")
}

// contains checks if s contains substr (simple helper to avoid strings import).
func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr) >= 0
}

// searchString finds the first index of substr in s, or -1 if not found.
func searchString(s, substr string) int {
	n := len(substr)
	for i := 0; i <= len(s)-n; i++ {
		if s[i:i+n] == substr {
			return i
		}
	}
	return -1
}
