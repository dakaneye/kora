package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
	_ "modernc.org/sqlite"
)

// db command flags
var (
	dbPath       string
	dbDryRun     bool
	dbOlderThan  string
	dbFormat     string
	dbImportFile string
	dbMerge      bool
)

// tables is the list of core tables in the schema.
var tables = []string{"goals", "commitments", "accomplishments", "context"}

var dbCmd = &cobra.Command{
	Use:   "db",
	Short: "Database administration commands",
	Long: `Database administration commands for Kora's memory store.

Available subcommands:
  path      Print database file path
  stats     Show database statistics
  validate  Check database integrity
  prune     Remove soft-deleted records
  backup    Create database backup
  export    Export data to JSON or Markdown
  import    Import data from JSON file`,
}

// ============================================================================
// db path
// ============================================================================

var dbPathCmd = &cobra.Command{
	Use:   "path",
	Short: "Print database file path",
	Long: `Print the absolute path to the database file.

Useful for MCP configuration or scripting. Output is just the path,
suitable for piping to other commands.

Examples:
  kora db path
  kora db path --path /custom/path/kora.db
  cat $(kora db path)`,
	RunE: runDBPath,
}

func runDBPath(_ *cobra.Command, _ []string) error {
	path, err := resolveDBPath()
	if err != nil {
		return err
	}
	fmt.Println(path)
	return nil
}

// ============================================================================
// db stats
// ============================================================================

var dbStatsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Show database statistics",
	Long: `Show database statistics including row counts, file size,
and schema version.

Example:
  kora db stats`,
	RunE: runDBStats,
}

func runDBStats(cmd *cobra.Command, _ []string) error {
	ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
	defer cancel()

	path, err := resolveDBPath()
	if err != nil {
		return err
	}

	if !fileExists(path) {
		return fmt.Errorf("database not initialized; run 'kora init' first")
	}

	db, err := openDB(path)
	if err != nil {
		return err
	}
	defer db.Close() //nolint:errcheck // Best effort cleanup

	// Get file size
	stat, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("stat database: %w", err)
	}

	// Get schema version
	version, err := getSchemaVersion(ctx, db)
	if err != nil {
		return fmt.Errorf("get schema version: %w", err)
	}

	// Print header
	fmt.Printf("Database: %s\n", path)
	fmt.Printf("Size: %s (%d bytes)\n", humanBytes(stat.Size()), stat.Size())
	fmt.Printf("Schema Version: %d\n\n", version)

	// Print table stats
	fmt.Printf("%-20s %10s %10s %s\n", "TABLE", "ROWS", "DELETED", "LAST UPDATED")
	fmt.Printf("%-20s %10s %10s %s\n", "-----", "----", "-------", "------------")

	for _, table := range tables {
		total, deleted, lastUpdated, err := getTableStats(ctx, db, table)
		if err != nil {
			return fmt.Errorf("get stats for %s: %w", table, err)
		}

		lastStr := "-"
		if lastUpdated != nil {
			lastStr = lastUpdated.Format(time.RFC3339)
		}

		fmt.Printf("%-20s %10d %10d %s\n", table, total, deleted, lastStr)
	}

	// FTS stats
	ftsCount, err := getFTSCount(ctx, db)
	if err != nil {
		return fmt.Errorf("get FTS count: %w", err)
	}
	fmt.Printf("%-20s %10d %10s %s\n", "memory_search (FTS)", ftsCount, "-", "-")

	return nil
}

// ============================================================================
// db validate
// ============================================================================

var dbValidateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Check database integrity",
	Long: `Validate the database by running integrity checks:

  - SQLite PRAGMA integrity_check
  - FTS consistency (entries match source tables)
  - NULL checks on required fields
  - Orphaned FTS entry detection

Exit code 0 means all checks passed.

Example:
  kora db validate`,
	RunE: runDBValidate,
}

func runDBValidate(cmd *cobra.Command, _ []string) error {
	ctx, cancel := context.WithTimeout(cmd.Context(), 60*time.Second)
	defer cancel()

	path, err := resolveDBPath()
	if err != nil {
		return err
	}

	if !fileExists(path) {
		return fmt.Errorf("database not initialized; run 'kora init' first")
	}

	db, err := openDB(path)
	if err != nil {
		return err
	}
	defer db.Close() //nolint:errcheck // Best effort cleanup

	passed := true

	// 1. SQLite integrity check
	fmt.Print("SQLite integrity check... ")
	result, err := pragmaIntegrityCheck(ctx, db)
	if err != nil {
		fmt.Println("ERROR")
		fmt.Printf("  %v\n", err)
		passed = false
	} else if result != "ok" {
		fmt.Println("FAILED")
		fmt.Printf("  %s\n", result)
		passed = false
	} else {
		fmt.Println("OK")
	}

	// 2. Required field NULL checks
	fmt.Print("Required fields check... ")
	nullIssues, err := checkRequiredFields(ctx, db)
	if err != nil {
		fmt.Println("ERROR")
		fmt.Printf("  %v\n", err)
		passed = false
	} else if len(nullIssues) > 0 {
		fmt.Println("FAILED")
		for _, issue := range nullIssues {
			fmt.Printf("  %s\n", issue)
		}
		passed = false
	} else {
		fmt.Println("OK")
	}

	// 3. FTS consistency check
	fmt.Print("FTS consistency check... ")
	ftsIssues, err := checkFTSConsistency(ctx, db)
	if err != nil {
		fmt.Println("ERROR")
		fmt.Printf("  %v\n", err)
		passed = false
	} else if len(ftsIssues) > 0 {
		fmt.Println("FAILED")
		for _, issue := range ftsIssues {
			fmt.Printf("  %s\n", issue)
		}
		passed = false
	} else {
		fmt.Println("OK")
	}

	fmt.Println()
	if passed {
		fmt.Println("All checks passed.")
		return nil
	}
	return fmt.Errorf("validation failed")
}

// ============================================================================
// db prune
// ============================================================================

var dbPruneCmd = &cobra.Command{
	Use:   "prune",
	Short: "Remove soft-deleted records",
	Long: `Permanently delete records that have been soft-deleted and are
older than the specified duration.

Duration format: "30d" (days), "90d", "1y" (year)

FTS entries are automatically cleaned up via triggers.

Examples:
  kora db prune --older-than 30d
  kora db prune --older-than 90d --dry-run`,
	RunE: runDBPrune,
}

func runDBPrune(cmd *cobra.Command, _ []string) error {
	ctx, cancel := context.WithTimeout(cmd.Context(), 60*time.Second)
	defer cancel()

	if dbOlderThan == "" {
		return fmt.Errorf("--older-than is required (e.g., 30d, 90d, 1y)")
	}

	duration, err := parseDuration(dbOlderThan)
	if err != nil {
		return fmt.Errorf("invalid duration: %w", err)
	}

	path, err := resolveDBPath()
	if err != nil {
		return err
	}

	if !fileExists(path) {
		return fmt.Errorf("database not initialized; run 'kora init' first")
	}

	db, err := openDB(path)
	if err != nil {
		return err
	}
	defer db.Close() //nolint:errcheck // Best effort cleanup

	cutoff := time.Now().UTC().Add(-duration)
	cutoffStr := cutoff.Format("2006-01-02 15:04:05") // SQLite datetime format

	if dbDryRun {
		fmt.Printf("Dry run: would delete records with is_deleted=1 and updated_at < %s\n\n", cutoffStr)
	} else {
		fmt.Printf("Pruning records with is_deleted=1 and updated_at < %s\n\n", cutoffStr)
	}

	totalDeleted := int64(0)

	for _, table := range tables {
		count, err := pruneTable(ctx, db, table, cutoffStr, dbDryRun)
		if err != nil {
			return fmt.Errorf("prune %s: %w", table, err)
		}
		if count > 0 {
			action := "deleted"
			if dbDryRun {
				action = "would delete"
			}
			fmt.Printf("  %s: %s %d records\n", table, action, count)
		}
		totalDeleted += count
	}

	if totalDeleted == 0 {
		fmt.Println("No records to prune.")
	} else if dbDryRun {
		fmt.Printf("\nTotal: would delete %d records\n", totalDeleted)
	} else {
		fmt.Printf("\nTotal: deleted %d records\n", totalDeleted)
	}

	return nil
}

// ============================================================================
// db backup
// ============================================================================

var dbBackupCmd = &cobra.Command{
	Use:   "backup",
	Short: "Create database backup",
	Long: `Create a backup of the database file.

Backups are stored in ~/.kora/backups/ with timestamp names:
  kora-YYYYMMDD-HHMMSS.db

The command checks that the source database is not locked before copying.

Example:
  kora db backup`,
	RunE: runDBBackup,
}

func runDBBackup(cmd *cobra.Command, _ []string) error {
	ctx, cancel := context.WithTimeout(cmd.Context(), 60*time.Second)
	defer cancel()

	path, err := resolveDBPath()
	if err != nil {
		return err
	}

	if !fileExists(path) {
		return fmt.Errorf("database not initialized; run 'kora init' first")
	}

	// Check database is accessible (not locked)
	db, err := openDB(path)
	if err != nil {
		return fmt.Errorf("database may be locked: %w", err)
	}

	// Verify we can query
	if err := db.PingContext(ctx); err != nil {
		db.Close() //nolint:errcheck // Best effort cleanup on error path
		return fmt.Errorf("database ping failed: %w", err)
	}
	db.Close() //nolint:errcheck // Best effort cleanup

	// Create backup directory
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("get home directory: %w", err)
	}
	backupDir := filepath.Join(home, ".kora", "backups")
	if err := os.MkdirAll(backupDir, 0o700); err != nil {
		return fmt.Errorf("create backup directory: %w", err)
	}

	// Generate backup filename
	timestamp := time.Now().Format("20060102-150405")
	backupPath := filepath.Join(backupDir, fmt.Sprintf("kora-%s.db", timestamp))

	// Copy database file
	if err := copyFile(path, backupPath); err != nil {
		return fmt.Errorf("copy database: %w", err)
	}

	// Set permissions
	if err := os.Chmod(backupPath, 0o600); err != nil {
		return fmt.Errorf("chmod backup: %w", err)
	}

	fmt.Println(backupPath)
	return nil
}

// ============================================================================
// db export
// ============================================================================

var dbExportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export data to JSON or Markdown",
	Long: `Export all database content to JSON or Markdown format.

Output goes to stdout and can be piped to a file.

Formats:
  json  - Complete dump with all fields, pretty-printed
  md    - Human-readable grouped by type

Examples:
  kora db export --format json > backup.json
  kora db export --format md > memory.md`,
	RunE: runDBExport,
}

func runDBExport(cmd *cobra.Command, _ []string) error {
	ctx, cancel := context.WithTimeout(cmd.Context(), 60*time.Second)
	defer cancel()

	if dbFormat != "json" && dbFormat != "md" {
		return fmt.Errorf("invalid format %q; use 'json' or 'md'", dbFormat)
	}

	path, err := resolveDBPath()
	if err != nil {
		return err
	}

	if !fileExists(path) {
		return fmt.Errorf("database not initialized; run 'kora init' first")
	}

	db, err := openDB(path)
	if err != nil {
		return err
	}
	defer db.Close() //nolint:errcheck // Best effort cleanup

	switch dbFormat {
	case "json":
		return exportJSON(ctx, db, os.Stdout)
	case "md":
		return exportMarkdown(ctx, db, os.Stdout)
	}

	return nil
}

// ============================================================================
// Initialization
// ============================================================================

func init() {
	// Add persistent flag for db path
	dbCmd.PersistentFlags().StringVar(&dbPath, "path", "", "database path (default: ~/.kora/data/kora.db)")

	// prune flags
	dbPruneCmd.Flags().BoolVar(&dbDryRun, "dry-run", false, "show what would be deleted without deleting")
	dbPruneCmd.Flags().StringVar(&dbOlderThan, "older-than", "", "delete records older than duration (e.g., 30d, 90d, 1y)")

	// export flags
	dbExportCmd.Flags().StringVar(&dbFormat, "format", "json", "output format: json or md")

	// import flags
	dbImportCmd.Flags().BoolVar(&dbDryRun, "dry-run", false, "validate without importing")
	dbImportCmd.Flags().BoolVar(&dbMerge, "merge", false, "upsert: insert new, update if import is newer, skip otherwise")

	// Add subcommands
	dbCmd.AddCommand(dbPathCmd)
	dbCmd.AddCommand(dbStatsCmd)
	dbCmd.AddCommand(dbValidateCmd)
	dbCmd.AddCommand(dbPruneCmd)
	dbCmd.AddCommand(dbBackupCmd)
	dbCmd.AddCommand(dbExportCmd)
	dbCmd.AddCommand(dbImportCmd)

	rootCmd.AddCommand(dbCmd)
}

// ============================================================================
// Helper Functions
// ============================================================================

// resolveDBPath returns the database path from flags or default location.
func resolveDBPath() (string, error) {
	if dbPath != "" {
		return filepath.Abs(dbPath)
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("get home directory: %w", err)
	}
	return filepath.Join(home, ".kora", "data", "kora.db"), nil
}

// openDB opens a database connection with standard pragmas.
func openDB(path string) (*sql.DB, error) {
	dsn := fmt.Sprintf("file:%s?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)", path)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	return db, nil
}

// getSchemaVersion reads the schema version from the database.
func getSchemaVersion(ctx context.Context, db *sql.DB) (int, error) {
	var versionStr string
	err := db.QueryRowContext(ctx, "SELECT value FROM _meta WHERE key = 'schema_version'").Scan(&versionStr)
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(versionStr)
}

// getTableStats returns row counts and last updated time for a table.
func getTableStats(ctx context.Context, db *sql.DB, table string) (total, deleted int64, lastUpdated *time.Time, err error) {
	// Total rows
	row := db.QueryRowContext(ctx, fmt.Sprintf("SELECT COUNT(*) FROM %s", table))
	if err = row.Scan(&total); err != nil {
		return
	}

	// Deleted rows
	row = db.QueryRowContext(ctx, fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE is_deleted = 1", table))
	if err = row.Scan(&deleted); err != nil {
		return
	}

	// Most recent updated_at
	var lastStr sql.NullString
	row = db.QueryRowContext(ctx, fmt.Sprintf("SELECT MAX(updated_at) FROM %s", table))
	if err = row.Scan(&lastStr); err != nil {
		return
	}

	if lastStr.Valid && lastStr.String != "" {
		t, parseErr := time.Parse(time.RFC3339, lastStr.String)
		if parseErr == nil {
			lastUpdated = &t
		} else {
			// Try datetime format without timezone
			t, parseErr = time.Parse("2006-01-02 15:04:05", lastStr.String)
			if parseErr == nil {
				lastUpdated = &t
			}
		}
	}

	return
}

// getFTSCount returns the row count in the FTS table.
func getFTSCount(ctx context.Context, db *sql.DB) (int64, error) {
	var count int64
	err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM memory_search").Scan(&count)
	return count, err
}

// humanBytes formats bytes as human-readable string.
func humanBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// pragmaIntegrityCheck runs SQLite integrity check.
func pragmaIntegrityCheck(ctx context.Context, db *sql.DB) (string, error) {
	var result string
	err := db.QueryRowContext(ctx, "PRAGMA integrity_check").Scan(&result)
	return result, err
}

// checkRequiredFields checks for NULL values in required fields.
func checkRequiredFields(ctx context.Context, db *sql.DB) ([]string, error) {
	var issues []string

	checks := []struct {
		table string
		field string
	}{
		{"goals", "title"},
		{"goals", "created_at"},
		{"goals", "updated_at"},
		{"commitments", "title"},
		{"commitments", "due_date"},
		{"commitments", "created_at"},
		{"commitments", "updated_at"},
		{"accomplishments", "title"},
		{"accomplishments", "accomplished_at"},
		{"accomplishments", "created_at"},
		{"accomplishments", "updated_at"},
		{"context", "entity_type"},
		{"context", "entity_id"},
		{"context", "title"},
		{"context", "body"},
		{"context", "created_at"},
		{"context", "updated_at"},
	}

	for _, check := range checks {
		var count int64
		query := fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE %s IS NULL", check.table, check.field)
		if err := db.QueryRowContext(ctx, query).Scan(&count); err != nil {
			return nil, fmt.Errorf("check %s.%s: %w", check.table, check.field, err)
		}
		if count > 0 {
			issues = append(issues, fmt.Sprintf("%s.%s: %d rows with NULL", check.table, check.field, count))
		}
	}

	return issues, nil
}

// checkFTSConsistency checks that FTS entries match source tables.
func checkFTSConsistency(ctx context.Context, db *sql.DB) ([]string, error) {
	var issues []string

	// Count FTS entries by content type
	ftsCountByType := make(map[string]int64)
	rows, err := db.QueryContext(ctx, "SELECT content, COUNT(*) FROM memory_search GROUP BY content")
	if err != nil {
		return nil, fmt.Errorf("count FTS entries: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var contentType sql.NullString
		var count int64
		if err := rows.Scan(&contentType, &count); err != nil {
			return nil, fmt.Errorf("scan FTS count: %w", err)
		}
		if contentType.Valid {
			ftsCountByType[contentType.String] = count
		}
	}

	// Map content types to tables
	typeToTable := map[string]string{
		"goal":           "goals",
		"commitment":     "commitments",
		"accomplishment": "accomplishments",
		"context":        "context",
	}

	// Check each type
	for contentType, table := range typeToTable {
		var sourceCount int64
		query := fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE is_deleted = 0", table)
		if err := db.QueryRowContext(ctx, query).Scan(&sourceCount); err != nil {
			return nil, fmt.Errorf("count %s: %w", table, err)
		}

		ftsCount := ftsCountByType[contentType]
		if ftsCount != sourceCount {
			issues = append(issues, fmt.Sprintf("%s: FTS has %d entries, source has %d", table, ftsCount, sourceCount))
		}
	}

	return issues, nil
}

// parseDuration parses a duration string like "30d", "90d", "1y".
func parseDuration(s string) (time.Duration, error) {
	re := regexp.MustCompile(`^(\d+)([dDyY])$`)
	matches := re.FindStringSubmatch(s)
	if matches == nil {
		return 0, fmt.Errorf("invalid format; use '30d', '90d', or '1y'")
	}

	num, _ := strconv.Atoi(matches[1])
	unit := strings.ToLower(matches[2])

	switch unit {
	case "d":
		return time.Duration(num) * 24 * time.Hour, nil
	case "y":
		return time.Duration(num) * 365 * 24 * time.Hour, nil
	}

	return 0, fmt.Errorf("invalid unit; use 'd' for days or 'y' for years")
}

// pruneTable deletes soft-deleted records older than cutoff.
func pruneTable(ctx context.Context, db *sql.DB, table, cutoff string, dryRun bool) (int64, error) {
	// Count first
	var count int64
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE is_deleted = 1 AND updated_at < ?", table)
	if err := db.QueryRowContext(ctx, countQuery, cutoff).Scan(&count); err != nil {
		return 0, err
	}

	if dryRun || count == 0 {
		return count, nil
	}

	// Delete
	deleteQuery := fmt.Sprintf("DELETE FROM %s WHERE is_deleted = 1 AND updated_at < ?", table)
	result, err := db.ExecContext(ctx, deleteQuery, cutoff)
	if err != nil {
		return 0, err
	}

	return result.RowsAffected()
}

// copyFile copies a file from src to dst.
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}

	return out.Sync()
}

// ============================================================================
// Export Functions
// ============================================================================

// exportData holds all exportable data.
type exportData struct {
	ExportedAt       string               `json:"exported_at"`
	SchemaVersion    int                  `json:"schema_version"`
	Goals            []map[string]any     `json:"goals"`
	Commitments      []map[string]any     `json:"commitments"`
	Accomplishments  []map[string]any     `json:"accomplishments"`
	Context          []map[string]any     `json:"context"`
}

func exportJSON(ctx context.Context, db *sql.DB, w io.Writer) error {
	version, err := getSchemaVersion(ctx, db)
	if err != nil {
		return fmt.Errorf("get schema version: %w", err)
	}

	data := exportData{
		ExportedAt:    time.Now().Format(time.RFC3339),
		SchemaVersion: version,
	}

	// Export each table
	data.Goals, err = exportTable(ctx, db, "goals")
	if err != nil {
		return fmt.Errorf("export goals: %w", err)
	}

	data.Commitments, err = exportTable(ctx, db, "commitments")
	if err != nil {
		return fmt.Errorf("export commitments: %w", err)
	}

	data.Accomplishments, err = exportTable(ctx, db, "accomplishments")
	if err != nil {
		return fmt.Errorf("export accomplishments: %w", err)
	}

	data.Context, err = exportTable(ctx, db, "context")
	if err != nil {
		return fmt.Errorf("export context: %w", err)
	}

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(data)
}

func exportTable(ctx context.Context, db *sql.DB, table string) ([]map[string]any, error) {
	query := fmt.Sprintf("SELECT * FROM %s WHERE is_deleted = 0", table)
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	var results []map[string]any

	for rows.Next() {
		values := make([]any, len(columns))
		valuePtrs := make([]any, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return nil, err
		}

		row := make(map[string]any)
		for i, col := range columns {
			val := values[i]
			// Convert []byte to string for JSON
			if b, ok := val.([]byte); ok {
				val = string(b)
			}
			row[col] = val
		}
		results = append(results, row)
	}

	return results, rows.Err()
}

func exportMarkdown(ctx context.Context, db *sql.DB, w io.Writer) error {
	version, err := getSchemaVersion(ctx, db)
	if err != nil {
		return fmt.Errorf("get schema version: %w", err)
	}

	fmt.Fprintf(w, "# Kora Memory Export\n\n")
	fmt.Fprintf(w, "Exported: %s\n", time.Now().Format(time.RFC3339))
	fmt.Fprintf(w, "Schema Version: %d\n\n", version)

	// Goals
	fmt.Fprintf(w, "## Goals\n\n")
	if err := exportTableMarkdown(ctx, db, w, "goals", []string{"title", "description", "status", "priority", "target_date", "tags"}); err != nil {
		return err
	}

	// Commitments
	fmt.Fprintf(w, "## Commitments\n\n")
	if err := exportTableMarkdown(ctx, db, w, "commitments", []string{"title", "to_whom", "status", "due_date", "tags"}); err != nil {
		return err
	}

	// Accomplishments
	fmt.Fprintf(w, "## Accomplishments\n\n")
	if err := exportTableMarkdown(ctx, db, w, "accomplishments", []string{"title", "description", "impact", "source_url", "accomplished_at", "tags"}); err != nil {
		return err
	}

	// Context
	fmt.Fprintf(w, "## Context\n\n")
	if err := exportTableMarkdown(ctx, db, w, "context", []string{"entity_type", "entity_id", "title", "body", "urgency", "tags"}); err != nil {
		return err
	}

	return nil
}

func exportTableMarkdown(ctx context.Context, db *sql.DB, w io.Writer, table string, fields []string) error {
	query := fmt.Sprintf("SELECT %s FROM %s WHERE is_deleted = 0", strings.Join(fields, ", "), table)
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return err
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		values := make([]sql.NullString, len(fields))
		valuePtrs := make([]any, len(fields))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return err
		}

		count++

		// Use title/first field as header
		title := values[0].String
		if title == "" {
			title = "(untitled)"
		}
		fmt.Fprintf(w, "### %s\n\n", title)

		for i := 1; i < len(fields); i++ {
			if values[i].Valid && values[i].String != "" {
				fmt.Fprintf(w, "- **%s**: %s\n", fields[i], values[i].String)
			}
		}
		fmt.Fprintln(w)
	}

	if count == 0 {
		fmt.Fprintf(w, "_No records_\n\n")
	}

	return rows.Err()
}

// ============================================================================
// db import
// ============================================================================

var dbImportCmd = &cobra.Command{
	Use:   "import [file]",
	Short: "Import data from JSON file",
	Long: `Import records from a JSON file into the database.

The import format must match the export format (kora db export --format json).

Modes:
  strict (default) - Fail if any ID already exists in the database
  merge (--merge)  - Upsert based on updated_at timestamp comparison:
                     - INSERT if ID doesn't exist
                     - UPDATE if import updated_at > database updated_at
                     - SKIP if import updated_at <= database updated_at

Examples:
  kora db import seed-data.json
  kora db import --dry-run seed-data.json
  kora db import --merge seed-data.json`,
	Args: cobra.ExactArgs(1),
	RunE: runDBImport,
}

// importStats tracks import operation results.
type importStats struct {
	Errors []string
	// Per-table statistics
	GoalsInserted           int
	GoalsUpdated            int
	GoalsSkipped            int
	CommitmentsInserted     int
	CommitmentsUpdated      int
	CommitmentsSkipped      int
	AccomplishmentsInserted int
	AccomplishmentsUpdated  int
	AccomplishmentsSkipped  int
	ContextInserted         int
	ContextUpdated          int
	ContextSkipped          int
}

// tableStats returns inserted/updated/skipped for a table (for backward compat reporting).
func (s *importStats) tableInserted(table string) int {
	switch table {
	case "goals":
		return s.GoalsInserted
	case "commitments":
		return s.CommitmentsInserted
	case "accomplishments":
		return s.AccomplishmentsInserted
	case "context":
		return s.ContextInserted
	}
	return 0
}

func (s *importStats) tableUpdated(table string) int {
	switch table {
	case "goals":
		return s.GoalsUpdated
	case "commitments":
		return s.CommitmentsUpdated
	case "accomplishments":
		return s.AccomplishmentsUpdated
	case "context":
		return s.ContextUpdated
	}
	return 0
}

func (s *importStats) tableSkipped(table string) int {
	switch table {
	case "goals":
		return s.GoalsSkipped
	case "commitments":
		return s.CommitmentsSkipped
	case "accomplishments":
		return s.AccomplishmentsSkipped
	case "context":
		return s.ContextSkipped
	}
	return 0
}

func (s *importStats) totalInserted() int {
	return s.GoalsInserted + s.CommitmentsInserted + s.AccomplishmentsInserted + s.ContextInserted
}

func (s *importStats) totalUpdated() int {
	return s.GoalsUpdated + s.CommitmentsUpdated + s.AccomplishmentsUpdated + s.ContextUpdated
}

func (s *importStats) totalSkipped() int {
	return s.GoalsSkipped + s.CommitmentsSkipped + s.AccomplishmentsSkipped + s.ContextSkipped
}

// validStatuses defines valid status values per table.
var validStatuses = map[string][]string{
	"goals":       {"active", "completed", "on_hold"},
	"commitments": {"active", "in_progress", "completed"},
}

// validEntityTypes defines valid entity_type values for context table.
var validEntityTypes = []string{"person", "project", "repo", "team", "general"}

// validUrgencies defines valid urgency values for context table.
var validUrgencies = []string{"critical", "high", "medium", "normal", "low", ""}

func runDBImport(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(cmd.Context(), 120*time.Second)
	defer cancel()

	importFile := args[0]

	// Read and parse import file
	// #nosec G304 -- CLI tool intentionally reads user-specified files
	data, err := os.ReadFile(importFile)
	if err != nil {
		return fmt.Errorf("read import file: %w", err)
	}

	// Check file size (max 10MB)
	const maxFileSize = 10 * 1024 * 1024
	if len(data) > maxFileSize {
		return fmt.Errorf("import file too large: %d bytes (max %d)", len(data), maxFileSize)
	}

	var importData exportData
	if err := json.Unmarshal(data, &importData); err != nil {
		return fmt.Errorf("parse import file: %w", err)
	}

	// Resolve and open database
	path, err := resolveDBPath()
	if err != nil {
		return err
	}

	if !fileExists(path) {
		return fmt.Errorf("database not initialized; run 'kora init' first")
	}

	db, err := openDB(path)
	if err != nil {
		return err
	}
	defer db.Close() //nolint:errcheck // Best effort cleanup

	// Validate schema version
	dbVersion, err := getSchemaVersion(ctx, db)
	if err != nil {
		return fmt.Errorf("get schema version: %w", err)
	}

	if importData.SchemaVersion != dbVersion {
		return fmt.Errorf("schema_version %d does not match database version %d", importData.SchemaVersion, dbVersion)
	}

	// Validate all records before importing
	stats := &importStats{}

	if err := validateImportData(&importData, stats); err != nil {
		return err
	}

	if len(stats.Errors) > 0 {
		fmt.Fprintf(os.Stderr, "Validation errors:\n")
		for _, e := range stats.Errors {
			fmt.Fprintf(os.Stderr, "  %s\n", e)
		}
		return fmt.Errorf("validation failed with %d errors", len(stats.Errors))
	}

	// Check for duplicate IDs in database (strict mode)
	if !dbMerge {
		if err := checkDuplicateIDs(ctx, db, &importData, stats); err != nil {
			return err
		}
		if len(stats.Errors) > 0 {
			fmt.Fprintf(os.Stderr, "Duplicate ID errors (use --merge to update existing):\n")
			for _, e := range stats.Errors {
				fmt.Fprintf(os.Stderr, "  %s\n", e)
			}
			return fmt.Errorf("import failed: %d duplicate IDs found", len(stats.Errors))
		}
	}

	// Dry run mode
	if dbDryRun {
		fmt.Printf("Dry run: validating %s\n", importFile)
		fmt.Printf("  Schema version: %d (OK)\n", importData.SchemaVersion)
		fmt.Printf("  Goals: %d valid\n", len(importData.Goals))
		fmt.Printf("  Commitments: %d valid\n", len(importData.Commitments))
		fmt.Printf("  Accomplishments: %d valid\n", len(importData.Accomplishments))
		fmt.Printf("  Context: %d valid\n", len(importData.Context))
		fmt.Println("\nDry run complete. No records imported.")
		fmt.Println("Run without --dry-run to import.")
		return nil
	}

	// Import within transaction
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck // Rollback is no-op after commit

	// Import each table (merge mode uses upsert functions)
	if dbMerge {
		if err := upsertGoals(ctx, tx, importData.Goals, stats); err != nil {
			return fmt.Errorf("upsert goals: %w", err)
		}
		if err := upsertCommitments(ctx, tx, importData.Commitments, stats); err != nil {
			return fmt.Errorf("upsert commitments: %w", err)
		}
		if err := upsertAccomplishments(ctx, tx, importData.Accomplishments, stats); err != nil {
			return fmt.Errorf("upsert accomplishments: %w", err)
		}
		if err := upsertContext(ctx, tx, importData.Context, stats); err != nil {
			return fmt.Errorf("upsert context: %w", err)
		}
	} else {
		// Strict mode: insert only
		inserted, err := importGoals(ctx, tx, importData.Goals)
		if err != nil {
			return fmt.Errorf("import goals: %w", err)
		}
		stats.GoalsInserted = inserted

		inserted, err = importCommitments(ctx, tx, importData.Commitments)
		if err != nil {
			return fmt.Errorf("import commitments: %w", err)
		}
		stats.CommitmentsInserted = inserted

		inserted, err = importAccomplishments(ctx, tx, importData.Accomplishments)
		if err != nil {
			return fmt.Errorf("import accomplishments: %w", err)
		}
		stats.AccomplishmentsInserted = inserted

		inserted, err = importContext(ctx, tx, importData.Context)
		if err != nil {
			return fmt.Errorf("import context: %w", err)
		}
		stats.ContextInserted = inserted
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	// Report results
	fmt.Printf("Importing: %s\n", importFile)
	if dbMerge {
		// Merge mode: show inserted/updated/skipped
		fmt.Printf("  Goals: %d inserted, %d updated, %d skipped\n",
			stats.GoalsInserted, stats.GoalsUpdated, stats.GoalsSkipped)
		fmt.Printf("  Commitments: %d inserted, %d updated, %d skipped\n",
			stats.CommitmentsInserted, stats.CommitmentsUpdated, stats.CommitmentsSkipped)
		fmt.Printf("  Accomplishments: %d inserted, %d updated, %d skipped\n",
			stats.AccomplishmentsInserted, stats.AccomplishmentsUpdated, stats.AccomplishmentsSkipped)
		fmt.Printf("  Context: %d inserted, %d updated, %d skipped\n",
			stats.ContextInserted, stats.ContextUpdated, stats.ContextSkipped)

		fmt.Printf("\nMerge complete. %d inserted, %d updated, %d skipped.\n",
			stats.totalInserted(), stats.totalUpdated(), stats.totalSkipped())
	} else {
		// Strict mode: show imported counts
		fmt.Printf("  Goals: %d imported\n", stats.GoalsInserted)
		fmt.Printf("  Commitments: %d imported\n", stats.CommitmentsInserted)
		fmt.Printf("  Accomplishments: %d imported\n", stats.AccomplishmentsInserted)
		fmt.Printf("  Context: %d imported\n", stats.ContextInserted)

		fmt.Printf("\nImport complete. %d records added.\n", stats.totalInserted())
	}

	return nil
}

// validateImportData validates all records in the import data.
func validateImportData(data *exportData, stats *importStats) error {
	// Track IDs within the file for duplicate detection
	seenIDs := make(map[string]map[string]bool)
	seenIDs["goals"] = make(map[string]bool)
	seenIDs["commitments"] = make(map[string]bool)
	seenIDs["accomplishments"] = make(map[string]bool)
	seenIDs["context"] = make(map[string]bool)

	// Validate goals
	for i, goal := range data.Goals {
		prefix := fmt.Sprintf("goals[%d]", i)
		validateGoalRecord(goal, prefix, seenIDs["goals"], stats)
	}

	// Validate commitments
	for i, commitment := range data.Commitments {
		prefix := fmt.Sprintf("commitments[%d]", i)
		validateCommitmentRecord(commitment, prefix, seenIDs["commitments"], stats)
	}

	// Validate accomplishments
	for i, accomplishment := range data.Accomplishments {
		prefix := fmt.Sprintf("accomplishments[%d]", i)
		validateAccomplishmentRecord(accomplishment, prefix, seenIDs["accomplishments"], stats)
	}

	// Validate context
	for i, ctx := range data.Context {
		prefix := fmt.Sprintf("context[%d]", i)
		validateContextRecord(ctx, prefix, seenIDs["context"], stats)
	}

	return nil
}

// validateGoalRecord validates a single goal record.
func validateGoalRecord(record map[string]any, prefix string, seenIDs map[string]bool, stats *importStats) {
	// Required fields
	id := validateRequiredString(record, "id", prefix, stats)
	validateRequiredString(record, "title", prefix, stats)
	validateRequiredString(record, "created_at", prefix, stats)
	validateRequiredString(record, "updated_at", prefix, stats)

	// Duplicate ID check within file
	if id != "" {
		if seenIDs[id] {
			stats.Errors = append(stats.Errors, fmt.Sprintf("goals: duplicate ID %q", id))
		}
		seenIDs[id] = true
	}

	// Validate status enum
	if status, ok := record["status"].(string); ok && status != "" {
		if !contains(validStatuses["goals"], status) {
			stats.Errors = append(stats.Errors, fmt.Sprintf("%s.status: must be one of %v, got %q", prefix, validStatuses["goals"], status))
		}
	}

	// Validate priority range
	if priority := getNumericValue(record, "priority"); priority != nil {
		p := int(*priority)
		if p < 1 || p > 5 {
			stats.Errors = append(stats.Errors, fmt.Sprintf("%s.priority: must be 1-5, got %d", prefix, p))
		}
	}

	// Validate timestamps
	validateTimestamp(record, "created_at", prefix, stats)
	validateTimestamp(record, "updated_at", prefix, stats)
	validateOptionalTimestamp(record, "target_date", prefix, stats)

	// Validate tags JSON
	validateTagsJSON(record, "tags", prefix, stats)
}

// validateCommitmentRecord validates a single commitment record.
func validateCommitmentRecord(record map[string]any, prefix string, seenIDs map[string]bool, stats *importStats) {
	// Required fields
	id := validateRequiredString(record, "id", prefix, stats)
	validateRequiredString(record, "title", prefix, stats)
	validateRequiredString(record, "due_date", prefix, stats)
	validateRequiredString(record, "created_at", prefix, stats)
	validateRequiredString(record, "updated_at", prefix, stats)

	// Duplicate ID check within file
	if id != "" {
		if seenIDs[id] {
			stats.Errors = append(stats.Errors, fmt.Sprintf("commitments: duplicate ID %q", id))
		}
		seenIDs[id] = true
	}

	// Validate status enum
	if status, ok := record["status"].(string); ok && status != "" {
		if !contains(validStatuses["commitments"], status) {
			stats.Errors = append(stats.Errors, fmt.Sprintf("%s.status: must be one of %v, got %q", prefix, validStatuses["commitments"], status))
		}
	}

	// Validate timestamps
	validateTimestamp(record, "created_at", prefix, stats)
	validateTimestamp(record, "updated_at", prefix, stats)
	validateOptionalTimestamp(record, "due_date", prefix, stats)

	// Validate tags JSON
	validateTagsJSON(record, "tags", prefix, stats)
}

// validateAccomplishmentRecord validates a single accomplishment record.
func validateAccomplishmentRecord(record map[string]any, prefix string, seenIDs map[string]bool, stats *importStats) {
	// Required fields
	id := validateRequiredString(record, "id", prefix, stats)
	validateRequiredString(record, "title", prefix, stats)
	validateRequiredString(record, "accomplished_at", prefix, stats)
	validateRequiredString(record, "created_at", prefix, stats)
	validateRequiredString(record, "updated_at", prefix, stats)

	// Duplicate ID check within file
	if id != "" {
		if seenIDs[id] {
			stats.Errors = append(stats.Errors, fmt.Sprintf("accomplishments: duplicate ID %q", id))
		}
		seenIDs[id] = true
	}

	// Validate timestamps
	validateTimestamp(record, "created_at", prefix, stats)
	validateTimestamp(record, "updated_at", prefix, stats)
	validateOptionalTimestamp(record, "accomplished_at", prefix, stats)

	// Validate tags JSON
	validateTagsJSON(record, "tags", prefix, stats)
}

// validateContextRecord validates a single context record.
func validateContextRecord(record map[string]any, prefix string, seenIDs map[string]bool, stats *importStats) {
	// Required fields
	id := validateRequiredString(record, "id", prefix, stats)
	validateRequiredString(record, "entity_type", prefix, stats)
	validateRequiredString(record, "entity_id", prefix, stats)
	validateRequiredString(record, "title", prefix, stats)
	validateRequiredString(record, "body", prefix, stats)
	validateRequiredString(record, "created_at", prefix, stats)
	validateRequiredString(record, "updated_at", prefix, stats)

	// Duplicate ID check within file
	if id != "" {
		if seenIDs[id] {
			stats.Errors = append(stats.Errors, fmt.Sprintf("context: duplicate ID %q", id))
		}
		seenIDs[id] = true
	}

	// Validate entity_type enum
	if entityType, ok := record["entity_type"].(string); ok && entityType != "" {
		if !contains(validEntityTypes, entityType) {
			stats.Errors = append(stats.Errors, fmt.Sprintf("%s.entity_type: must be one of %v, got %q", prefix, validEntityTypes, entityType))
		}
	}

	// Validate urgency enum (optional)
	if urgency, ok := record["urgency"].(string); ok {
		if !contains(validUrgencies, urgency) {
			stats.Errors = append(stats.Errors, fmt.Sprintf("%s.urgency: must be one of %v, got %q", prefix, validUrgencies, urgency))
		}
	}

	// Validate timestamps
	validateTimestamp(record, "created_at", prefix, stats)
	validateTimestamp(record, "updated_at", prefix, stats)

	// Validate tags JSON
	validateTagsJSON(record, "tags", prefix, stats)
}

// validateRequiredString checks that a required string field exists and is non-empty.
func validateRequiredString(record map[string]any, field, prefix string, stats *importStats) string {
	val, ok := record[field]
	if !ok || val == nil {
		stats.Errors = append(stats.Errors, fmt.Sprintf("%s: missing required field %q", prefix, field))
		return ""
	}

	str, ok := val.(string)
	if !ok {
		stats.Errors = append(stats.Errors, fmt.Sprintf("%s.%s: expected string, got %T", prefix, field, val))
		return ""
	}

	if str == "" {
		stats.Errors = append(stats.Errors, fmt.Sprintf("%s.%s: cannot be empty", prefix, field))
		return ""
	}

	return str
}

// validateTimestamp checks that a timestamp field is valid RFC3339 or datetime format.
func validateTimestamp(record map[string]any, field, prefix string, stats *importStats) {
	val, ok := record[field]
	if !ok || val == nil {
		return
	}

	str, ok := val.(string)
	if !ok {
		stats.Errors = append(stats.Errors, fmt.Sprintf("%s.%s: expected string, got %T", prefix, field, val))
		return
	}

	if str == "" {
		return
	}

	// Try RFC3339 first, then datetime format
	if _, err := time.Parse(time.RFC3339, str); err == nil {
		return
	}
	if _, err := time.Parse("2006-01-02 15:04:05", str); err == nil {
		return
	}
	if _, err := time.Parse("2006-01-02", str); err == nil {
		return
	}

	stats.Errors = append(stats.Errors, fmt.Sprintf("%s.%s: invalid timestamp format %q", prefix, field, str))
}

// validateOptionalTimestamp checks an optional timestamp field.
func validateOptionalTimestamp(record map[string]any, field, prefix string, stats *importStats) {
	val, ok := record[field]
	if !ok || val == nil {
		return
	}

	str, ok := val.(string)
	if !ok || str == "" {
		return
	}

	// Try RFC3339 first, then datetime format
	if _, err := time.Parse(time.RFC3339, str); err == nil {
		return
	}
	if _, err := time.Parse("2006-01-02 15:04:05", str); err == nil {
		return
	}
	if _, err := time.Parse("2006-01-02", str); err == nil {
		return
	}

	stats.Errors = append(stats.Errors, fmt.Sprintf("%s.%s: invalid timestamp format %q", prefix, field, str))
}

// validateTagsJSON checks that tags field contains valid JSON array.
func validateTagsJSON(record map[string]any, field, prefix string, stats *importStats) {
	val, ok := record[field]
	if !ok || val == nil {
		return
	}

	str, ok := val.(string)
	if !ok || str == "" {
		return
	}

	var arr []any
	if err := json.Unmarshal([]byte(str), &arr); err != nil {
		stats.Errors = append(stats.Errors, fmt.Sprintf("%s.%s: invalid JSON array: %v", prefix, field, err))
	}
}

// getNumericValue extracts a numeric value from a record field.
func getNumericValue(record map[string]any, field string) *float64 {
	val, ok := record[field]
	if !ok || val == nil {
		return nil
	}

	switch v := val.(type) {
	case float64:
		return &v
	case int:
		f := float64(v)
		return &f
	case int64:
		f := float64(v)
		return &f
	}

	return nil
}

// contains checks if a string slice contains a value.
func contains(slice []string, val string) bool {
	for _, s := range slice {
		if s == val {
			return true
		}
	}
	return false
}

// checkDuplicateIDs checks if any IDs in import data already exist in database.
func checkDuplicateIDs(ctx context.Context, db *sql.DB, data *exportData, stats *importStats) error {
	// Check goals
	for i, goal := range data.Goals {
		id, ok := goal["id"].(string)
		if !ok || id == "" {
			continue
		}
		exists, err := idExists(ctx, db, "goals", id)
		if err != nil {
			return fmt.Errorf("check goals[%d].id: %w", i, err)
		}
		if exists {
			stats.Errors = append(stats.Errors, fmt.Sprintf("goals[%d].id: %q already exists in database", i, id))
		}
	}

	// Check commitments
	for i, commitment := range data.Commitments {
		id, ok := commitment["id"].(string)
		if !ok || id == "" {
			continue
		}
		exists, err := idExists(ctx, db, "commitments", id)
		if err != nil {
			return fmt.Errorf("check commitments[%d].id: %w", i, err)
		}
		if exists {
			stats.Errors = append(stats.Errors, fmt.Sprintf("commitments[%d].id: %q already exists in database", i, id))
		}
	}

	// Check accomplishments
	for i, accomplishment := range data.Accomplishments {
		id, ok := accomplishment["id"].(string)
		if !ok || id == "" {
			continue
		}
		exists, err := idExists(ctx, db, "accomplishments", id)
		if err != nil {
			return fmt.Errorf("check accomplishments[%d].id: %w", i, err)
		}
		if exists {
			stats.Errors = append(stats.Errors, fmt.Sprintf("accomplishments[%d].id: %q already exists in database", i, id))
		}
	}

	// Check context
	for i, ctxRecord := range data.Context {
		id, ok := ctxRecord["id"].(string)
		if !ok || id == "" {
			continue
		}
		exists, err := idExists(ctx, db, "context", id)
		if err != nil {
			return fmt.Errorf("check context[%d].id: %w", i, err)
		}
		if exists {
			stats.Errors = append(stats.Errors, fmt.Sprintf("context[%d].id: %q already exists in database", i, id))
		}
	}

	return nil
}

// idExists checks if an ID exists in a table.
// Note: table parameter comes from controlled internal list, not user input.
func idExists(ctx context.Context, db *sql.DB, table, id string) (bool, error) {
	var count int
	// #nosec G201 -- table name is from controlled internal list (goals, commitments, accomplishments, context)
	query := fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE id = ?", table)
	if err := db.QueryRowContext(ctx, query, id).Scan(&count); err != nil {
		return false, err
	}
	return count > 0, nil
}

// importGoals inserts goal records into the database.
func importGoals(ctx context.Context, tx *sql.Tx, goals []map[string]any) (int, error) {
	if len(goals) == 0 {
		return 0, nil
	}

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO goals (id, title, description, status, priority, target_date, tags, is_deleted, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return 0, fmt.Errorf("prepare statement: %w", err)
	}
	defer stmt.Close() //nolint:errcheck // Best effort cleanup

	count := 0
	for i, goal := range goals {
		_, err := stmt.ExecContext(ctx,
			getString(goal, "id"),
			getString(goal, "title"),
			getStringPtr(goal, "description"),
			getStringOrDefault(goal, "status", "active"),
			getIntOrDefault(goal, "priority", 3),
			getStringPtr(goal, "target_date"),
			getStringPtr(goal, "tags"),
			getIntOrDefault(goal, "is_deleted", 0),
			getString(goal, "created_at"),
			getString(goal, "updated_at"),
		)
		if err != nil {
			return count, fmt.Errorf("insert goals[%d]: %w", i, err)
		}
		count++
	}

	return count, nil
}

// importCommitments inserts commitment records into the database.
func importCommitments(ctx context.Context, tx *sql.Tx, commitments []map[string]any) (int, error) {
	if len(commitments) == 0 {
		return 0, nil
	}

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO commitments (id, title, to_whom, status, due_date, tags, is_deleted, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return 0, fmt.Errorf("prepare statement: %w", err)
	}
	defer stmt.Close() //nolint:errcheck // Best effort cleanup

	count := 0
	for i, commitment := range commitments {
		_, err := stmt.ExecContext(ctx,
			getString(commitment, "id"),
			getString(commitment, "title"),
			getStringPtr(commitment, "to_whom"),
			getStringOrDefault(commitment, "status", "active"),
			getString(commitment, "due_date"),
			getStringPtr(commitment, "tags"),
			getIntOrDefault(commitment, "is_deleted", 0),
			getString(commitment, "created_at"),
			getString(commitment, "updated_at"),
		)
		if err != nil {
			return count, fmt.Errorf("insert commitments[%d]: %w", i, err)
		}
		count++
	}

	return count, nil
}

// importAccomplishments inserts accomplishment records into the database.
func importAccomplishments(ctx context.Context, tx *sql.Tx, accomplishments []map[string]any) (int, error) {
	if len(accomplishments) == 0 {
		return 0, nil
	}

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO accomplishments (id, title, description, impact, source_url, accomplished_at, tags, is_deleted, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return 0, fmt.Errorf("prepare statement: %w", err)
	}
	defer stmt.Close() //nolint:errcheck // Best effort cleanup

	count := 0
	for i, accomplishment := range accomplishments {
		_, err := stmt.ExecContext(ctx,
			getString(accomplishment, "id"),
			getString(accomplishment, "title"),
			getStringPtr(accomplishment, "description"),
			getStringPtr(accomplishment, "impact"),
			getStringPtr(accomplishment, "source_url"),
			getString(accomplishment, "accomplished_at"),
			getStringPtr(accomplishment, "tags"),
			getIntOrDefault(accomplishment, "is_deleted", 0),
			getString(accomplishment, "created_at"),
			getString(accomplishment, "updated_at"),
		)
		if err != nil {
			return count, fmt.Errorf("insert accomplishments[%d]: %w", i, err)
		}
		count++
	}

	return count, nil
}

// importContext inserts context records into the database.
func importContext(ctx context.Context, tx *sql.Tx, contextRecords []map[string]any) (int, error) {
	if len(contextRecords) == 0 {
		return 0, nil
	}

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO context (id, entity_type, entity_id, title, body, urgency, source_url, tags, is_deleted, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return 0, fmt.Errorf("prepare statement: %w", err)
	}
	defer stmt.Close() //nolint:errcheck // Best effort cleanup

	count := 0
	for i, record := range contextRecords {
		_, err := stmt.ExecContext(ctx,
			getString(record, "id"),
			getString(record, "entity_type"),
			getString(record, "entity_id"),
			getString(record, "title"),
			getString(record, "body"),
			getStringPtr(record, "urgency"),
			getStringPtr(record, "source_url"),
			getStringPtr(record, "tags"),
			getIntOrDefault(record, "is_deleted", 0),
			getString(record, "created_at"),
			getString(record, "updated_at"),
		)
		if err != nil {
			return count, fmt.Errorf("insert context[%d]: %w", i, err)
		}
		count++
	}

	return count, nil
}

// getString extracts a string value from a record.
func getString(record map[string]any, field string) string {
	val, ok := record[field]
	if !ok || val == nil {
		return ""
	}
	str, ok := val.(string)
	if !ok {
		return fmt.Sprintf("%v", val)
	}
	return str
}

// getStringPtr extracts a nullable string value from a record.
func getStringPtr(record map[string]any, field string) *string {
	val, ok := record[field]
	if !ok || val == nil {
		return nil
	}
	str, ok := val.(string)
	if !ok {
		s := fmt.Sprintf("%v", val)
		return &s
	}
	if str == "" {
		return nil
	}
	return &str
}

// getStringOrDefault extracts a string value with a default.
func getStringOrDefault(record map[string]any, field, defaultVal string) string {
	val, ok := record[field]
	if !ok || val == nil {
		return defaultVal
	}
	str, ok := val.(string)
	if !ok {
		return defaultVal
	}
	if str == "" {
		return defaultVal
	}
	return str
}

// getIntOrDefault extracts an integer value with a default.
func getIntOrDefault(record map[string]any, field string, defaultVal int) int {
	val, ok := record[field]
	if !ok || val == nil {
		return defaultVal
	}

	switch v := val.(type) {
	case float64:
		return int(v)
	case int:
		return v
	case int64:
		return int(v)
	}

	return defaultVal
}

// ============================================================================
// Upsert Functions (Merge Mode)
// ============================================================================

// parseTimestamp parses a timestamp string in RFC3339 or datetime format.
func parseTimestamp(s string) (time.Time, error) {
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, nil
	}
	if t, err := time.Parse("2006-01-02 15:04:05", s); err == nil {
		return t, nil
	}
	if t, err := time.Parse("2006-01-02", s); err == nil {
		return t, nil
	}
	return time.Time{}, fmt.Errorf("invalid timestamp format: %s", s)
}

// getExistingUpdatedAt fetches the updated_at timestamp for an existing record.
// Returns nil if the record doesn't exist.
func getExistingUpdatedAt(ctx context.Context, tx *sql.Tx, table, id string) (*time.Time, error) {
	var updatedAtStr sql.NullString
	// #nosec G201 -- table name is from controlled internal list
	query := fmt.Sprintf("SELECT updated_at FROM %s WHERE id = ?", table)
	err := tx.QueryRowContext(ctx, query, id).Scan(&updatedAtStr)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if !updatedAtStr.Valid || updatedAtStr.String == "" {
		return nil, nil
	}
	t, err := parseTimestamp(updatedAtStr.String)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// upsertGoals performs insert/update/skip logic for goals in merge mode.
func upsertGoals(ctx context.Context, tx *sql.Tx, goals []map[string]any, stats *importStats) error {
	if len(goals) == 0 {
		return nil
	}

	insertStmt, err := tx.PrepareContext(ctx, `
		INSERT INTO goals (id, title, description, status, priority, target_date, tags, is_deleted, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return fmt.Errorf("prepare insert statement: %w", err)
	}
	defer insertStmt.Close() //nolint:errcheck

	updateStmt, err := tx.PrepareContext(ctx, `
		UPDATE goals SET title=?, description=?, status=?, priority=?, target_date=?, tags=?, is_deleted=?, created_at=?, updated_at=?
		WHERE id=?
	`)
	if err != nil {
		return fmt.Errorf("prepare update statement: %w", err)
	}
	defer updateStmt.Close() //nolint:errcheck

	for i, goal := range goals {
		id := getString(goal, "id")
		importUpdatedAtStr := getString(goal, "updated_at")
		importUpdatedAt, err := parseTimestamp(importUpdatedAtStr)
		if err != nil {
			return fmt.Errorf("goals[%d]: parse updated_at: %w", i, err)
		}

		existingUpdatedAt, err := getExistingUpdatedAt(ctx, tx, "goals", id)
		if err != nil {
			return fmt.Errorf("goals[%d]: check existing: %w", i, err)
		}

		if existingUpdatedAt == nil {
			// INSERT: record doesn't exist
			_, err := insertStmt.ExecContext(ctx,
				id,
				getString(goal, "title"),
				getStringPtr(goal, "description"),
				getStringOrDefault(goal, "status", "active"),
				getIntOrDefault(goal, "priority", 3),
				getStringPtr(goal, "target_date"),
				getStringPtr(goal, "tags"),
				getIntOrDefault(goal, "is_deleted", 0),
				getString(goal, "created_at"),
				importUpdatedAtStr,
			)
			if err != nil {
				return fmt.Errorf("insert goals[%d]: %w", i, err)
			}
			stats.GoalsInserted++
		} else if importUpdatedAt.After(*existingUpdatedAt) {
			// UPDATE: import is newer
			_, err := updateStmt.ExecContext(ctx,
				getString(goal, "title"),
				getStringPtr(goal, "description"),
				getStringOrDefault(goal, "status", "active"),
				getIntOrDefault(goal, "priority", 3),
				getStringPtr(goal, "target_date"),
				getStringPtr(goal, "tags"),
				getIntOrDefault(goal, "is_deleted", 0),
				getString(goal, "created_at"),
				importUpdatedAtStr,
				id,
			)
			if err != nil {
				return fmt.Errorf("update goals[%d]: %w", i, err)
			}
			stats.GoalsUpdated++
		} else {
			// SKIP: existing is same or newer
			stats.GoalsSkipped++
		}
	}

	return nil
}

// upsertCommitments performs insert/update/skip logic for commitments in merge mode.
func upsertCommitments(ctx context.Context, tx *sql.Tx, commitments []map[string]any, stats *importStats) error {
	if len(commitments) == 0 {
		return nil
	}

	insertStmt, err := tx.PrepareContext(ctx, `
		INSERT INTO commitments (id, title, to_whom, status, due_date, tags, is_deleted, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return fmt.Errorf("prepare insert statement: %w", err)
	}
	defer insertStmt.Close() //nolint:errcheck

	updateStmt, err := tx.PrepareContext(ctx, `
		UPDATE commitments SET title=?, to_whom=?, status=?, due_date=?, tags=?, is_deleted=?, created_at=?, updated_at=?
		WHERE id=?
	`)
	if err != nil {
		return fmt.Errorf("prepare update statement: %w", err)
	}
	defer updateStmt.Close() //nolint:errcheck

	for i, commitment := range commitments {
		id := getString(commitment, "id")
		importUpdatedAtStr := getString(commitment, "updated_at")
		importUpdatedAt, err := parseTimestamp(importUpdatedAtStr)
		if err != nil {
			return fmt.Errorf("commitments[%d]: parse updated_at: %w", i, err)
		}

		existingUpdatedAt, err := getExistingUpdatedAt(ctx, tx, "commitments", id)
		if err != nil {
			return fmt.Errorf("commitments[%d]: check existing: %w", i, err)
		}

		if existingUpdatedAt == nil {
			// INSERT: record doesn't exist
			_, err := insertStmt.ExecContext(ctx,
				id,
				getString(commitment, "title"),
				getStringPtr(commitment, "to_whom"),
				getStringOrDefault(commitment, "status", "active"),
				getString(commitment, "due_date"),
				getStringPtr(commitment, "tags"),
				getIntOrDefault(commitment, "is_deleted", 0),
				getString(commitment, "created_at"),
				importUpdatedAtStr,
			)
			if err != nil {
				return fmt.Errorf("insert commitments[%d]: %w", i, err)
			}
			stats.CommitmentsInserted++
		} else if importUpdatedAt.After(*existingUpdatedAt) {
			// UPDATE: import is newer
			_, err := updateStmt.ExecContext(ctx,
				getString(commitment, "title"),
				getStringPtr(commitment, "to_whom"),
				getStringOrDefault(commitment, "status", "active"),
				getString(commitment, "due_date"),
				getStringPtr(commitment, "tags"),
				getIntOrDefault(commitment, "is_deleted", 0),
				getString(commitment, "created_at"),
				importUpdatedAtStr,
				id,
			)
			if err != nil {
				return fmt.Errorf("update commitments[%d]: %w", i, err)
			}
			stats.CommitmentsUpdated++
		} else {
			// SKIP: existing is same or newer
			stats.CommitmentsSkipped++
		}
	}

	return nil
}

// upsertAccomplishments performs insert/update/skip logic for accomplishments in merge mode.
func upsertAccomplishments(ctx context.Context, tx *sql.Tx, accomplishments []map[string]any, stats *importStats) error {
	if len(accomplishments) == 0 {
		return nil
	}

	insertStmt, err := tx.PrepareContext(ctx, `
		INSERT INTO accomplishments (id, title, description, impact, source_url, accomplished_at, tags, is_deleted, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return fmt.Errorf("prepare insert statement: %w", err)
	}
	defer insertStmt.Close() //nolint:errcheck

	updateStmt, err := tx.PrepareContext(ctx, `
		UPDATE accomplishments SET title=?, description=?, impact=?, source_url=?, accomplished_at=?, tags=?, is_deleted=?, created_at=?, updated_at=?
		WHERE id=?
	`)
	if err != nil {
		return fmt.Errorf("prepare update statement: %w", err)
	}
	defer updateStmt.Close() //nolint:errcheck

	for i, accomplishment := range accomplishments {
		id := getString(accomplishment, "id")
		importUpdatedAtStr := getString(accomplishment, "updated_at")
		importUpdatedAt, err := parseTimestamp(importUpdatedAtStr)
		if err != nil {
			return fmt.Errorf("accomplishments[%d]: parse updated_at: %w", i, err)
		}

		existingUpdatedAt, err := getExistingUpdatedAt(ctx, tx, "accomplishments", id)
		if err != nil {
			return fmt.Errorf("accomplishments[%d]: check existing: %w", i, err)
		}

		if existingUpdatedAt == nil {
			// INSERT: record doesn't exist
			_, err := insertStmt.ExecContext(ctx,
				id,
				getString(accomplishment, "title"),
				getStringPtr(accomplishment, "description"),
				getStringPtr(accomplishment, "impact"),
				getStringPtr(accomplishment, "source_url"),
				getString(accomplishment, "accomplished_at"),
				getStringPtr(accomplishment, "tags"),
				getIntOrDefault(accomplishment, "is_deleted", 0),
				getString(accomplishment, "created_at"),
				importUpdatedAtStr,
			)
			if err != nil {
				return fmt.Errorf("insert accomplishments[%d]: %w", i, err)
			}
			stats.AccomplishmentsInserted++
		} else if importUpdatedAt.After(*existingUpdatedAt) {
			// UPDATE: import is newer
			_, err := updateStmt.ExecContext(ctx,
				getString(accomplishment, "title"),
				getStringPtr(accomplishment, "description"),
				getStringPtr(accomplishment, "impact"),
				getStringPtr(accomplishment, "source_url"),
				getString(accomplishment, "accomplished_at"),
				getStringPtr(accomplishment, "tags"),
				getIntOrDefault(accomplishment, "is_deleted", 0),
				getString(accomplishment, "created_at"),
				importUpdatedAtStr,
				id,
			)
			if err != nil {
				return fmt.Errorf("update accomplishments[%d]: %w", i, err)
			}
			stats.AccomplishmentsUpdated++
		} else {
			// SKIP: existing is same or newer
			stats.AccomplishmentsSkipped++
		}
	}

	return nil
}

// upsertContext performs insert/update/skip logic for context records in merge mode.
func upsertContext(ctx context.Context, tx *sql.Tx, contextRecords []map[string]any, stats *importStats) error {
	if len(contextRecords) == 0 {
		return nil
	}

	insertStmt, err := tx.PrepareContext(ctx, `
		INSERT INTO context (id, entity_type, entity_id, title, body, urgency, source_url, tags, is_deleted, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return fmt.Errorf("prepare insert statement: %w", err)
	}
	defer insertStmt.Close() //nolint:errcheck

	updateStmt, err := tx.PrepareContext(ctx, `
		UPDATE context SET entity_type=?, entity_id=?, title=?, body=?, urgency=?, source_url=?, tags=?, is_deleted=?, created_at=?, updated_at=?
		WHERE id=?
	`)
	if err != nil {
		return fmt.Errorf("prepare update statement: %w", err)
	}
	defer updateStmt.Close() //nolint:errcheck

	for i, record := range contextRecords {
		id := getString(record, "id")
		importUpdatedAtStr := getString(record, "updated_at")
		importUpdatedAt, err := parseTimestamp(importUpdatedAtStr)
		if err != nil {
			return fmt.Errorf("context[%d]: parse updated_at: %w", i, err)
		}

		existingUpdatedAt, err := getExistingUpdatedAt(ctx, tx, "context", id)
		if err != nil {
			return fmt.Errorf("context[%d]: check existing: %w", i, err)
		}

		if existingUpdatedAt == nil {
			// INSERT: record doesn't exist
			_, err := insertStmt.ExecContext(ctx,
				id,
				getString(record, "entity_type"),
				getString(record, "entity_id"),
				getString(record, "title"),
				getString(record, "body"),
				getStringPtr(record, "urgency"),
				getStringPtr(record, "source_url"),
				getStringPtr(record, "tags"),
				getIntOrDefault(record, "is_deleted", 0),
				getString(record, "created_at"),
				importUpdatedAtStr,
			)
			if err != nil {
				return fmt.Errorf("insert context[%d]: %w", i, err)
			}
			stats.ContextInserted++
		} else if importUpdatedAt.After(*existingUpdatedAt) {
			// UPDATE: import is newer
			_, err := updateStmt.ExecContext(ctx,
				getString(record, "entity_type"),
				getString(record, "entity_id"),
				getString(record, "title"),
				getString(record, "body"),
				getStringPtr(record, "urgency"),
				getStringPtr(record, "source_url"),
				getStringPtr(record, "tags"),
				getIntOrDefault(record, "is_deleted", 0),
				getString(record, "created_at"),
				importUpdatedAtStr,
				id,
			)
			if err != nil {
				return fmt.Errorf("update context[%d]: %w", i, err)
			}
			stats.ContextUpdated++
		} else {
			// SKIP: existing is same or newer
			stats.ContextSkipped++
		}
	}

	return nil
}
