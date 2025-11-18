// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: Copyright The LanceDB Authors

package lancedb

import (
	"os"
	"path/filepath"
	"testing"
)

// Helper to create temporary test directory
func createTempDB(t *testing.T) string {
	t.Helper()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_db")
	return dbPath
}

func TestConnect(t *testing.T) {
	dbPath := createTempDB(t)

	db, err := Connect(dbPath)
	if err != nil {
		t.Fatalf("Failed to connect to database: %v", err)
	}
	if db == nil {
		t.Fatal("Connection is nil")
	}
	if db.handle == nil {
		t.Fatal("Connection handle is nil")
	}

	db.Close()
}

func TestConnectInvalidPath(t *testing.T) {
	// Test with empty path - should still work (creates local db)
	dbPath := createTempDB(t)
	db, err := Connect(dbPath)
	if err != nil {
		t.Fatalf("Failed to connect with valid path: %v", err)
	}
	defer db.Close()
}

func TestConnectionClose(t *testing.T) {
	dbPath := createTempDB(t)
	db, err := Connect(dbPath)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}

	// Close once
	db.Close()
	if db.handle != nil {
		t.Error("Handle should be nil after Close()")
	}

	// Close again should be safe (idempotent)
	db.Close()
}

func TestTableNames(t *testing.T) {
	dbPath := createTempDB(t)
	db, err := Connect(dbPath)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer db.Close()

	// Initially should be empty
	tables, err := db.TableNames()
	if err != nil {
		t.Fatalf("Failed to get table names: %v", err)
	}
	if len(tables) != 0 {
		t.Errorf("Expected 0 tables, got %d", len(tables))
	}

	// Create a table
	table, err := db.CreateTable("test_table")
	if err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}
	defer table.Close()

	// Now should have one table
	tables, err = db.TableNames()
	if err != nil {
		t.Fatalf("Failed to get table names: %v", err)
	}
	if len(tables) != 1 {
		t.Errorf("Expected 1 table, got %d", len(tables))
	}
	if tables[0] != "test_table" {
		t.Errorf("Expected table name 'test_table', got '%s'", tables[0])
	}
}

func TestCreateTable(t *testing.T) {
	dbPath := createTempDB(t)
	db, err := Connect(dbPath)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer db.Close()

	table, err := db.CreateTable("test_table")
	if err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}
	if table == nil {
		t.Fatal("Table is nil")
	}
	if table.handle == nil {
		t.Fatal("Table handle is nil")
	}
	if table.conn != db {
		t.Error("Table's connection reference doesn't match")
	}

	table.Close()
}

func TestOpenTable(t *testing.T) {
	dbPath := createTempDB(t)
	db, err := Connect(dbPath)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer db.Close()

	// Create a table first
	table1, err := db.CreateTable("test_table")
	if err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}
	table1.Close()

	// Now open it
	table2, err := db.OpenTable("test_table")
	if err != nil {
		t.Fatalf("Failed to open table: %v", err)
	}
	if table2 == nil {
		t.Fatal("Opened table is nil")
	}
	if table2.handle == nil {
		t.Fatal("Opened table handle is nil")
	}

	table2.Close()
}

func TestOpenNonexistentTable(t *testing.T) {
	dbPath := createTempDB(t)
	db, err := Connect(dbPath)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer db.Close()

	_, err = db.OpenTable("nonexistent_table")
	if err == nil {
		t.Error("Expected error when opening nonexistent table, got nil")
	}
}

func TestTableClose(t *testing.T) {
	dbPath := createTempDB(t)
	db, err := Connect(dbPath)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer db.Close()

	table, err := db.CreateTable("test_table")
	if err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}

	// Close once
	table.Close()
	if table.handle != nil {
		t.Error("Table handle should be nil after Close()")
	}

	// Close again should be safe (idempotent)
	table.Close()
}

func TestCountRows(t *testing.T) {
	dbPath := createTempDB(t)
	db, err := Connect(dbPath)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer db.Close()

	table, err := db.CreateTable("test_table")
	if err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}
	defer table.Close()

	// New table should have 0 rows
	count, err := table.CountRows()
	if err != nil {
		t.Fatalf("Failed to count rows: %v", err)
	}
	if count != 0 {
		t.Errorf("Expected 0 rows, got %d", count)
	}
}

func TestMultipleTables(t *testing.T) {
	dbPath := createTempDB(t)
	db, err := Connect(dbPath)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer db.Close()

	// Create multiple tables
	table1, err := db.CreateTable("table1")
	if err != nil {
		t.Fatalf("Failed to create table1: %v", err)
	}
	defer table1.Close()

	table2, err := db.CreateTable("table2")
	if err != nil {
		t.Fatalf("Failed to create table2: %v", err)
	}
	defer table2.Close()

	table3, err := db.CreateTable("table3")
	if err != nil {
		t.Fatalf("Failed to create table3: %v", err)
	}
	defer table3.Close()

	// Check table names
	tables, err := db.TableNames()
	if err != nil {
		t.Fatalf("Failed to get table names: %v", err)
	}
	if len(tables) != 3 {
		t.Errorf("Expected 3 tables, got %d", len(tables))
	}

	// Verify all table names are present
	tableMap := make(map[string]bool)
	for _, name := range tables {
		tableMap[name] = true
	}
	if !tableMap["table1"] || !tableMap["table2"] || !tableMap["table3"] {
		t.Errorf("Missing expected table names. Got: %v", tables)
	}
}

func TestErrorHandling(t *testing.T) {
	dbPath := createTempDB(t)
	db, err := Connect(dbPath)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer db.Close()

	// Try to open a table that doesn't exist
	_, err = db.OpenTable("nonexistent")
	if err == nil {
		t.Error("Expected error opening nonexistent table")
	}

	// Error should be of type *Error
	if _, ok := err.(*Error); !ok {
		t.Errorf("Expected *lancedb.Error, got %T", err)
	}

	// Error message should be non-empty
	if err.Error() == "" {
		t.Error("Error message is empty")
	}
}

// TestDatabasePersistence verifies that data persists across connections
func TestDatabasePersistence(t *testing.T) {
	dbPath := createTempDB(t)

	// Create a table in first connection
	{
		db, err := Connect(dbPath)
		if err != nil {
			t.Fatalf("Failed to connect: %v", err)
		}
		_, err = db.CreateTable("persistent_table")
		if err != nil {
			t.Fatalf("Failed to create table: %v", err)
		}
		db.Close()
	}

	// Verify table exists in second connection
	{
		db, err := Connect(dbPath)
		if err != nil {
			t.Fatalf("Failed to reconnect: %v", err)
		}
		defer db.Close()

		tables, err := db.TableNames()
		if err != nil {
			t.Fatalf("Failed to get table names: %v", err)
		}
		if len(tables) != 1 || tables[0] != "persistent_table" {
			t.Errorf("Expected persistent_table, got %v", tables)
		}

		// Should be able to open the table
		table, err := db.OpenTable("persistent_table")
		if err != nil {
			t.Fatalf("Failed to open persistent table: %v", err)
		}
		table.Close()
	}
}

// TestConcurrentTableOperations tests working with multiple tables simultaneously
func TestConcurrentTableOperations(t *testing.T) {
	dbPath := createTempDB(t)
	db, err := Connect(dbPath)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer db.Close()

	// Create and open multiple tables
	table1, err := db.CreateTable("table1")
	if err != nil {
		t.Fatalf("Failed to create table1: %v", err)
	}
	defer table1.Close()

	table2, err := db.CreateTable("table2")
	if err != nil {
		t.Fatalf("Failed to create table2: %v", err)
	}
	defer table2.Close()

	// Both tables should be usable
	count1, err := table1.CountRows()
	if err != nil {
		t.Fatalf("Failed to count rows in table1: %v", err)
	}
	if count1 != 0 {
		t.Errorf("Expected 0 rows in table1, got %d", count1)
	}

	count2, err := table2.CountRows()
	if err != nil {
		t.Fatalf("Failed to count rows in table2: %v", err)
	}
	if count2 != 0 {
		t.Errorf("Expected 0 rows in table2, got %d", count2)
	}
}

// BenchmarkConnect measures connection performance
func BenchmarkConnect(b *testing.B) {
	tmpDir := b.TempDir()
	dbPath := filepath.Join(tmpDir, "bench_db")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		db, err := Connect(dbPath)
		if err != nil {
			b.Fatalf("Failed to connect: %v", err)
		}
		db.Close()
	}
}

// BenchmarkTableCreate measures table creation performance
func BenchmarkTableCreate(b *testing.B) {
	tmpDir := b.TempDir()
	dbPath := filepath.Join(tmpDir, "bench_db")
	db, err := Connect(dbPath)
	if err != nil {
		b.Fatalf("Failed to connect: %v", err)
	}
	defer db.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tableName := filepath.Join("table", string(rune(i)))
		table, err := db.CreateTable(tableName)
		if err != nil {
			b.Fatalf("Failed to create table: %v", err)
		}
		table.Close()
	}
}

// TestMain ensures cleanup happens even if tests panic
func TestMain(m *testing.M) {
	code := m.Run()
	os.Exit(code)
}

