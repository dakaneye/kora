package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
	_ "modernc.org/sqlite" // Pure Go SQLite driver

	"github.com/dakaneye/kora/internal/storage"
)

// Init command flags
var (
	initPath  string
	initForce bool
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize the memory store",
	Long: `Initialize Kora's SQLite-based memory store.

The memory store persists goals, commitments, accomplishments, and context
that Claude uses to provide personalized assistance.

Default location: ~/.kora/data/kora.db

This command is idempotent:
  - If the database doesn't exist: creates it with the current schema
  - If the database exists at current version: reports status, no changes
  - If the database exists at older version: runs migrations
  - With --force: drops and recreates the database (destructive)

Examples:
  # Initialize at default location
  kora init

  # Initialize at custom path
  kora init --path /path/to/kora.db

  # Reinitialize (warning: destroys existing data)
  kora init --force`,
	RunE: runInit,
}

func init() {
	initCmd.Flags().StringVar(&initPath, "path", "", "database path (default: ~/.kora/data/kora.db)")
	initCmd.Flags().BoolVar(&initForce, "force", false, "reinitialize database (destroys existing data)")

	rootCmd.AddCommand(initCmd)
}

// runInit executes the init command.
func runInit(cmd *cobra.Command, _ []string) error {
	ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
	defer cancel()

	// Determine database path
	dbPath := initPath
	if dbPath == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("get home directory: %w", err)
		}
		dbPath = filepath.Join(home, ".kora", "data", "kora.db")
	}

	// Check if database already exists
	exists := fileExists(dbPath)

	// Handle --force flag
	if initForce && exists {
		if err := removeDatabase(dbPath); err != nil {
			return fmt.Errorf("remove existing database: %w", err)
		}
		exists = false
		if verbose {
			fmt.Fprintf(os.Stderr, "Removed existing database: %s\n", dbPath)
		}
	}

	// Check existing database version if it exists
	if exists {
		ver, err := checkSchemaVersion(ctx, dbPath)
		if err != nil {
			return fmt.Errorf("check schema version: %w", err)
		}

		if ver == storage.SchemaVersion {
			fmt.Printf("Already initialized (version %d)\n", ver)
			fmt.Printf("Path: %s\n", dbPath)
			return nil
		}

		if ver > storage.SchemaVersion {
			return fmt.Errorf("database schema (version %d) is newer than supported (version %d); upgrade kora", ver, storage.SchemaVersion)
		}

		// Database exists but needs migration
		// For now, we only support version 1, so migrations are not yet implemented
		fmt.Printf("Migrating from version %d to version %d...\n", ver, storage.SchemaVersion)
		// Future: call migration function here
		fmt.Printf("Migrated to version %d\n", storage.SchemaVersion)
		fmt.Printf("Path: %s\n", dbPath)
		return nil
	}

	// Create new database
	store, err := storage.NewStore(dbPath)
	if err != nil {
		return fmt.Errorf("create database: %w", err)
	}
	defer store.Close() //nolint:errcheck // Best effort cleanup

	fmt.Printf("Initialized memory store (version %d)\n", storage.SchemaVersion)
	fmt.Printf("Path: %s\n", dbPath)

	return nil
}

// fileExists checks if a file exists at the given path.
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// removeDatabase removes the database file and associated WAL/SHM files.
func removeDatabase(dbPath string) error {
	// Remove main database file
	if err := os.Remove(dbPath); err != nil && !os.IsNotExist(err) {
		return err
	}

	// Remove WAL file if it exists
	walPath := dbPath + "-wal"
	if err := os.Remove(walPath); err != nil && !os.IsNotExist(err) {
		return err
	}

	// Remove SHM file if it exists
	shmPath := dbPath + "-shm"
	if err := os.Remove(shmPath); err != nil && !os.IsNotExist(err) {
		return err
	}

	return nil
}

// checkSchemaVersion reads the schema version from an existing database.
func checkSchemaVersion(ctx context.Context, dbPath string) (int, error) {
	dsn := fmt.Sprintf("file:%s?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)", dbPath)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return 0, fmt.Errorf("open database: %w", err)
	}
	defer db.Close() //nolint:errcheck // Best effort cleanup

	var versionStr string
	err = db.QueryRowContext(ctx, "SELECT value FROM _meta WHERE key = 'schema_version'").Scan(&versionStr)
	if err != nil {
		if err == sql.ErrNoRows {
			return 0, fmt.Errorf("schema version not found in database")
		}
		return 0, fmt.Errorf("query schema version: %w", err)
	}

	var version int
	_, err = fmt.Sscanf(versionStr, "%d", &version)
	if err != nil {
		return 0, fmt.Errorf("parse schema version %q: %w", versionStr, err)
	}

	return version, nil
}
