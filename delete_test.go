package lancedb

import (
	"fmt"
	"os"
	"testing"

	"github.com/apache/arrow/go/v17/arrow"
	"github.com/apache/arrow/go/v17/arrow/array"
	"github.com/apache/arrow/go/v17/arrow/memory"
)

// Helper function to create a test table with sample data
func createTestTableWithData(t *testing.T, dbPath string, tableName string) (*Connection, *Table) {
	// Clean up any existing database
	os.RemoveAll(dbPath)

	// Connect to database
	db, err := Connect(dbPath)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}

	// Create schema
	schema := arrow.NewSchema([]arrow.Field{
		{Name: "id", Type: arrow.PrimitiveTypes.Int32},
		{Name: "name", Type: arrow.BinaryTypes.String},
		{Name: "category", Type: arrow.BinaryTypes.String},
	}, nil)

	// Create table
	table, err := db.CreateTableWithSchema(tableName, schema)
	if err != nil {
		db.Close()
		t.Fatalf("Failed to create table: %v", err)
	}

	// Insert test data
	mem := memory.NewGoAllocator()
	builder := array.NewRecordBuilder(mem, schema)
	defer builder.Release()

	idBuilder := builder.Field(0).(*array.Int32Builder)
	nameBuilder := builder.Field(1).(*array.StringBuilder)
	categoryBuilder := builder.Field(2).(*array.StringBuilder)

	// Add 100 rows
	for i := 0; i < 100; i++ {
		idBuilder.Append(int32(i))
		nameBuilder.Append(fmt.Sprintf("doc_%d", i))
		if i < 50 {
			categoryBuilder.Append("old")
		} else {
			categoryBuilder.Append("new")
		}
	}

	record := builder.NewRecord()
	defer record.Release()

	err = table.Add(record, AddModeAppend)
	if err != nil {
		table.Close()
		db.Close()
		t.Fatalf("Failed to add data: %v", err)
	}

	return db, table
}

// TestDeleteSimple tests simple predicate-based deletion
func TestDeleteSimple(t *testing.T) {
	dbPath := "./test_delete_simple_db"
	defer os.RemoveAll(dbPath)

	db, table := createTestTableWithData(t, dbPath, "test_table")
	defer db.Close()
	defer table.Close()

	// Verify initial count
	count, err := table.CountRows()
	if err != nil {
		t.Fatalf("Failed to count rows: %v", err)
	}
	if count != 100 {
		t.Fatalf("Expected 100 rows, got %d", count)
	}

	// Delete rows with id > 50
	err = table.Delete("id > 50")
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	// Verify count after deletion
	count, err = table.CountRows()
	if err != nil {
		t.Fatalf("Failed to count rows after delete: %v", err)
	}
	if count != 51 {
		t.Fatalf("Expected 51 rows after delete, got %d", count)
	}
}

// TestDeleteBuilder tests builder pattern deletion
func TestDeleteBuilder(t *testing.T) {
	dbPath := "./test_delete_builder_db"
	defer os.RemoveAll(dbPath)

	db, table := createTestTableWithData(t, dbPath, "test_table")
	defer db.Close()
	defer table.Close()

	// Delete using builder pattern
	err := table.DeleteBuilder().Where("id < 20").Execute()
	if err != nil {
		t.Fatalf("Delete builder failed: %v", err)
	}

	// Verify count after deletion
	count, err := table.CountRows()
	if err != nil {
		t.Fatalf("Failed to count rows: %v", err)
	}
	if count != 80 {
		t.Fatalf("Expected 80 rows after delete, got %d", count)
	}
}

// TestDeleteWithStringPredicate tests deletion with string-based predicates
func TestDeleteWithStringPredicate(t *testing.T) {
	dbPath := "./test_delete_string_db"
	defer os.RemoveAll(dbPath)

	db, table := createTestTableWithData(t, dbPath, "test_table")
	defer db.Close()
	defer table.Close()

	// Delete rows with category = 'old'
	err := table.Delete("category = 'old'")
	if err != nil {
		t.Fatalf("Delete with string predicate failed: %v", err)
	}

	// Verify count after deletion
	count, err := table.CountRows()
	if err != nil {
		t.Fatalf("Failed to count rows: %v", err)
	}
	if count != 50 {
		t.Fatalf("Expected 50 rows after delete, got %d", count)
	}
}

// TestDeleteAllRows tests deleting all rows
func TestDeleteAllRows(t *testing.T) {
	dbPath := "./test_delete_all_db"
	defer os.RemoveAll(dbPath)

	db, table := createTestTableWithData(t, dbPath, "test_table")
	defer db.Close()
	defer table.Close()

	// Delete all rows
	err := table.Delete("id >= 0")
	if err != nil {
		t.Fatalf("Delete all rows failed: %v", err)
	}

	// Verify count after deletion
	count, err := table.CountRows()
	if err != nil {
		t.Fatalf("Failed to count rows: %v", err)
	}
	if count != 0 {
		t.Fatalf("Expected 0 rows after delete, got %d", count)
	}
}

// TestDeleteInvalidPredicate tests error handling for invalid predicates
func TestDeleteInvalidPredicate(t *testing.T) {
	dbPath := "./test_delete_invalid_db"
	defer os.RemoveAll(dbPath)

	db, table := createTestTableWithData(t, dbPath, "test_table")
	defer db.Close()
	defer table.Close()

	// Try to delete with empty predicate
	err := table.Delete("")
	if err == nil {
		t.Fatal("Expected error for empty predicate, got nil")
	}

	// Try to delete with invalid column
	err = table.Delete("nonexistent_column > 10")
	if err == nil {
		t.Fatal("Expected error for invalid column, got nil")
	}
}

// TestDeleteBuilderEmptyPredicate tests builder without Where clause
func TestDeleteBuilderEmptyPredicate(t *testing.T) {
	dbPath := "./test_delete_builder_empty_db"
	defer os.RemoveAll(dbPath)

	db, table := createTestTableWithData(t, dbPath, "test_table")
	defer db.Close()
	defer table.Close()

	// Try to execute delete without setting predicate
	err := table.DeleteBuilder().Execute()
	if err == nil {
		t.Fatal("Expected error when executing delete without predicate, got nil")
	}
}

// TestDeleteVerifyRowCount tests that CountRows reflects deletions
func TestDeleteVerifyRowCount(t *testing.T) {
	dbPath := "./test_delete_verify_count_db"
	defer os.RemoveAll(dbPath)

	db, table := createTestTableWithData(t, dbPath, "test_table")
	defer db.Close()
	defer table.Close()

	// Initial count
	initialCount, err := table.CountRows()
	if err != nil {
		t.Fatalf("Failed to get initial count: %v", err)
	}

	// Delete some rows
	err = table.Delete("id >= 25 AND id <= 74")
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	// Count after delete
	afterCount, err := table.CountRows()
	if err != nil {
		t.Fatalf("Failed to get count after delete: %v", err)
	}

	expectedAfter := initialCount - 50
	if afterCount != expectedAfter {
		t.Fatalf("Expected %d rows after delete, got %d", expectedAfter, afterCount)
	}
}

// TestDeleteVerifyData tests that deleted rows are actually removed
func TestDeleteVerifyData(t *testing.T) {
	dbPath := "./test_delete_verify_data_db"
	defer os.RemoveAll(dbPath)

	db, table := createTestTableWithData(t, dbPath, "test_table")
	defer db.Close()
	defer table.Close()

	// Delete rows with id < 30
	err := table.Delete("id < 30")
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	// Read all remaining data
	records, err := table.ToArrow(-1)
	if err != nil {
		t.Fatalf("Failed to read data: %v", err)
	}

	// Verify that all remaining rows have id >= 30
	for _, record := range records {
		idCol := record.Column(0).(*array.Int32)
		for i := 0; i < int(record.NumRows()); i++ {
			id := idCol.Value(i)
			if id < 30 {
				t.Fatalf("Found row with id %d, which should have been deleted", id)
			}
		}
		record.Release()
	}
}

// TestDeleteMultipleOperations tests multiple sequential delete operations
func TestDeleteMultipleOperations(t *testing.T) {
	dbPath := "./test_delete_multiple_db"
	defer os.RemoveAll(dbPath)

	db, table := createTestTableWithData(t, dbPath, "test_table")
	defer db.Close()
	defer table.Close()

	// First delete
	err := table.Delete("id < 20")
	if err != nil {
		t.Fatalf("First delete failed: %v", err)
	}

	count, err := table.CountRows()
	if err != nil {
		t.Fatalf("Failed to count rows: %v", err)
	}
	if count != 80 {
		t.Fatalf("Expected 80 rows after first delete, got %d", count)
	}

	// Second delete
	err = table.DeleteBuilder().Where("id > 80").Execute()
	if err != nil {
		t.Fatalf("Second delete failed: %v", err)
	}

	count, err = table.CountRows()
	if err != nil {
		t.Fatalf("Failed to count rows: %v", err)
	}
	if count != 61 {
		t.Fatalf("Expected 61 rows after second delete, got %d", count)
	}
}

