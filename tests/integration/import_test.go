//go:build integration

package integration

import (
	"context"
	"database/sql"
	"os/exec"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

// TestImport_ValidFile tests importing a valid JSON file
func TestImport_ValidFile(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "kora.db")

	// Initialize database
	cmd := exec.Command("go", "run", "../../cmd/kora", "init", "--path", dbPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("kora init failed: %v\nOutput: %s", err, output)
	}

	// Import valid file
	importFile := "../../tests/testdata/import/valid.json"
	cmd = exec.Command("go", "run", "../../cmd/kora", "db", "import", "--path", dbPath, importFile)
	output, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("kora db import failed: %v\nOutput: %s", err, output)
	}

	outputStr := string(output)
	t.Logf("Import output:\n%s", outputStr)

	// Verify records were imported
	db, err := openDBImport(dbPath)
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	ctx := context.Background()

	// Check goals count
	var goalCount int
	err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM goals WHERE is_deleted = 0").Scan(&goalCount)
	if err != nil {
		t.Fatalf("Failed to count goals: %v", err)
	}
	if goalCount != 2 {
		t.Errorf("goals count = %d, want 2", goalCount)
	}

	// Check commitments count
	var commitmentCount int
	err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM commitments WHERE is_deleted = 0").Scan(&commitmentCount)
	if err != nil {
		t.Fatalf("Failed to count commitments: %v", err)
	}
	if commitmentCount != 2 {
		t.Errorf("commitments count = %d, want 2", commitmentCount)
	}

	// Check accomplishments count
	var accomplishmentCount int
	err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM accomplishments WHERE is_deleted = 0").Scan(&accomplishmentCount)
	if err != nil {
		t.Fatalf("Failed to count accomplishments: %v", err)
	}
	if accomplishmentCount != 2 {
		t.Errorf("accomplishments count = %d, want 2", accomplishmentCount)
	}

	// Check context count
	var contextCount int
	err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM context WHERE is_deleted = 0").Scan(&contextCount)
	if err != nil {
		t.Fatalf("Failed to count context: %v", err)
	}
	if contextCount != 2 {
		t.Errorf("context count = %d, want 2", contextCount)
	}

	t.Log("All record counts verified")
}

// TestImport_FTSConsistency tests that FTS entries are created after import
func TestImport_FTSConsistency(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "kora.db")

	// Initialize database
	cmd := exec.Command("go", "run", "../../cmd/kora", "init", "--path", dbPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("kora init failed: %v\nOutput: %s", err, output)
	}

	// Import valid file
	importFile := "../../tests/testdata/import/valid.json"
	cmd = exec.Command("go", "run", "../../cmd/kora", "db", "import", "--path", dbPath, importFile)
	output, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("kora db import failed: %v\nOutput: %s", err, output)
	}

	// Run validation to check FTS consistency
	cmd = exec.Command("go", "run", "../../cmd/kora", "db", "validate", "--path", dbPath)
	output, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("kora db validate failed: %v\nOutput: %s", err, output)
	}

	outputStr := string(output)
	if !containsString(outputStr, "All checks passed") {
		t.Errorf("Validation did not pass after import: %s", outputStr)
	}

	// Verify FTS entry counts
	db, err := openDBImport(dbPath)
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	ctx := context.Background()

	// Count FTS entries by content type
	ftsCountByType := map[string]int{
		"goal":           0,
		"commitment":     0,
		"accomplishment": 0,
		"context":        0,
	}

	rows, err := db.QueryContext(ctx, "SELECT content, COUNT(*) FROM memory_search GROUP BY content")
	if err != nil {
		t.Fatalf("Failed to query FTS counts: %v", err)
	}
	defer rows.Close()

	for rows.Next() {
		var contentType sql.NullString
		var count int
		if err := rows.Scan(&contentType, &count); err != nil {
			t.Fatalf("Failed to scan FTS count: %v", err)
		}
		if contentType.Valid {
			ftsCountByType[contentType.String] = count
		}
	}

	// Verify FTS counts match source tables
	expectedCounts := map[string]int{
		"goal":           2,
		"commitment":     2,
		"accomplishment": 2,
		"context":        2,
	}

	for contentType, expected := range expectedCounts {
		if ftsCountByType[contentType] != expected {
			t.Errorf("FTS count for %s = %d, want %d", contentType, ftsCountByType[contentType], expected)
		}
	}

	t.Log("FTS consistency verified")
}

// TestImport_MissingFields tests validation of missing required fields
func TestImport_MissingFields(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "kora.db")

	// Initialize database
	cmd := exec.Command("go", "run", "../../cmd/kora", "init", "--path", dbPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("kora init failed: %v\nOutput: %s", err, output)
	}

	// Try to import file with missing fields
	importFile := "../../tests/testdata/import/missing_fields.json"
	cmd = exec.Command("go", "run", "../../cmd/kora", "db", "import", "--path", dbPath, importFile)
	output, err = cmd.CombinedOutput()
	if err == nil {
		t.Fatal("Expected import to fail with missing fields, but it succeeded")
	}

	outputStr := string(output)
	if !containsString(outputStr, "Validation errors") {
		t.Errorf("Expected 'Validation errors' in output, got: %s", outputStr)
	}

	// Check for specific missing field errors
	expectedErrors := []string{
		"missing required field \"title\"",
		"missing required field \"due_date\"",
		"missing required field \"accomplished_at\"",
		"missing required field \"entity_type\"",
		"missing required field \"body\"",
	}

	for _, expected := range expectedErrors {
		if !containsString(outputStr, expected) {
			t.Errorf("Expected error containing %q, not found in output:\n%s", expected, outputStr)
		}
	}

	t.Log("Missing fields validation errors detected correctly")
}

// TestImport_InvalidEnums tests validation of invalid enum values
func TestImport_InvalidEnums(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "kora.db")

	// Initialize database
	cmd := exec.Command("go", "run", "../../cmd/kora", "init", "--path", dbPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("kora init failed: %v\nOutput: %s", err, output)
	}

	// Try to import file with invalid enums
	importFile := "../../tests/testdata/import/invalid_enums.json"
	cmd = exec.Command("go", "run", "../../cmd/kora", "db", "import", "--path", dbPath, importFile)
	output, err = cmd.CombinedOutput()
	if err == nil {
		t.Fatal("Expected import to fail with invalid enums, but it succeeded")
	}

	outputStr := string(output)
	if !containsString(outputStr, "Validation errors") {
		t.Errorf("Expected 'Validation errors' in output, got: %s", outputStr)
	}

	// Check for specific enum validation errors
	expectedErrors := []string{
		"must be one of",
		"must be 1-5",
	}

	for _, expected := range expectedErrors {
		if !containsString(outputStr, expected) {
			t.Errorf("Expected error containing %q, not found in output:\n%s", expected, outputStr)
		}
	}

	t.Log("Invalid enum validation errors detected correctly")
}

// TestImport_DuplicateIDs tests duplicate ID detection within file
func TestImport_DuplicateIDs(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "kora.db")

	// Initialize database
	cmd := exec.Command("go", "run", "../../cmd/kora", "init", "--path", dbPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("kora init failed: %v\nOutput: %s", err, output)
	}

	// Try to import file with duplicate IDs
	importFile := "../../tests/testdata/import/duplicate_ids.json"
	cmd = exec.Command("go", "run", "../../cmd/kora", "db", "import", "--path", dbPath, importFile)
	output, err = cmd.CombinedOutput()
	if err == nil {
		t.Fatal("Expected import to fail with duplicate IDs, but it succeeded")
	}

	outputStr := string(output)
	if !containsString(outputStr, "duplicate ID") {
		t.Errorf("Expected 'duplicate ID' in output, got: %s", outputStr)
	}

	t.Log("Duplicate ID validation errors detected correctly")
}

// TestImport_DryRun tests dry run mode
func TestImport_DryRun(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "kora.db")

	// Initialize database
	cmd := exec.Command("go", "run", "../../cmd/kora", "init", "--path", dbPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("kora init failed: %v\nOutput: %s", err, output)
	}

	// Dry run import
	importFile := "../../tests/testdata/import/valid.json"
	cmd = exec.Command("go", "run", "../../cmd/kora", "db", "import", "--path", dbPath, "--dry-run", importFile)
	output, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("kora db import --dry-run failed: %v\nOutput: %s", err, output)
	}

	outputStr := string(output)
	if !containsString(outputStr, "Dry run") {
		t.Errorf("Expected 'Dry run' in output, got: %s", outputStr)
	}

	// Verify no records were imported
	db, err := openDBImport(dbPath)
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	ctx := context.Background()

	var goalCount int
	err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM goals").Scan(&goalCount)
	if err != nil {
		t.Fatalf("Failed to count goals: %v", err)
	}
	if goalCount != 0 {
		t.Errorf("goals count = %d, want 0 (dry run should not import)", goalCount)
	}

	t.Log("Dry run mode verified - no records imported")
}

// TestImport_MergeMode tests merge mode (upsert behavior)
func TestImport_MergeMode(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "kora.db")

	// Initialize database
	cmd := exec.Command("go", "run", "../../cmd/kora", "init", "--path", dbPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("kora init failed: %v\nOutput: %s", err, output)
	}

	// First import
	importFile := "../../tests/testdata/import/valid.json"
	cmd = exec.Command("go", "run", "../../cmd/kora", "db", "import", "--path", dbPath, importFile)
	output, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("First import failed: %v\nOutput: %s", err, output)
	}

	// Second import without merge should fail
	cmd = exec.Command("go", "run", "../../cmd/kora", "db", "import", "--path", dbPath, importFile)
	output, err = cmd.CombinedOutput()
	if err == nil {
		t.Fatal("Expected second import to fail without --merge, but it succeeded")
	}

	outputStr := string(output)
	if !containsString(outputStr, "already exists") {
		t.Errorf("Expected 'already exists' in output, got: %s", outputStr)
	}

	// Second import with merge should succeed
	cmd = exec.Command("go", "run", "../../cmd/kora", "db", "import", "--path", dbPath, "--merge", importFile)
	output, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Merge import failed: %v\nOutput: %s", err, output)
	}

	outputStr = string(output)
	if !containsString(outputStr, "skipped") {
		t.Errorf("Expected 'skipped' in merge output (records are same timestamp), got: %s", outputStr)
	}

	// Verify record counts unchanged
	db, err := openDBImport(dbPath)
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	ctx := context.Background()

	var goalCount int
	err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM goals WHERE is_deleted = 0").Scan(&goalCount)
	if err != nil {
		t.Fatalf("Failed to count goals: %v", err)
	}
	if goalCount != 2 {
		t.Errorf("goals count = %d, want 2 (merge should not duplicate)", goalCount)
	}

	t.Log("Merge mode verified - duplicate detection and upsert working")
}

// Helper functions

// openDB opens a database connection with standard pragmas.
func openDBImport(path string) (*sql.DB, error) {
	dsn := "file:" + path + "?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)"
	return sql.Open("sqlite", dsn)
}
