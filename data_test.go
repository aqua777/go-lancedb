// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: Copyright The LanceDB Authors

package lancedb

import (
	"path/filepath"
	"testing"

	"github.com/apache/arrow/go/v17/arrow"
	"github.com/apache/arrow/go/v17/arrow/array"
	"github.com/apache/arrow/go/v17/arrow/memory"
)

func TestCreateTableWithSchema(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_db")

	db, err := Connect(dbPath)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer db.Close()

	// Create a custom schema
	schema := arrow.NewSchema(
		[]arrow.Field{
			{Name: "id", Type: arrow.PrimitiveTypes.Int64, Nullable: false},
			{Name: "name", Type: arrow.BinaryTypes.String, Nullable: true},
			{Name: "score", Type: arrow.PrimitiveTypes.Float64, Nullable: false},
		},
		nil,
	)

	table, err := db.CreateTableWithSchema("custom_table", schema)
	if err != nil {
		t.Fatalf("Failed to create table with schema: %v", err)
	}
	defer table.Close()

	// Verify the schema
	retrievedSchema, err := table.Schema()
	if err != nil {
		t.Fatalf("Failed to get schema: %v", err)
	}

	if len(retrievedSchema.Fields()) != len(schema.Fields()) {
		t.Errorf("Schema field count mismatch: expected %d, got %d",
			len(schema.Fields()), len(retrievedSchema.Fields()))
	}
}

func TestAddData(t *testing.T) {
	pool := memory.NewGoAllocator()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_db")

	db, err := Connect(dbPath)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer db.Close()

	// Create schema
	schema := arrow.NewSchema(
		[]arrow.Field{
			{Name: "id", Type: arrow.PrimitiveTypes.Int32, Nullable: false},
			{Name: "value", Type: arrow.PrimitiveTypes.Float64, Nullable: false},
		},
		nil,
	)

	// Create table
	table, err := db.CreateTableWithSchema("data_table", schema)
	if err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}
	defer table.Close()

	// Create data
	idBuilder := array.NewInt32Builder(pool)
	defer idBuilder.Release()
	valueBuilder := array.NewFloat64Builder(pool)
	defer valueBuilder.Release()

	idBuilder.AppendValues([]int32{1, 2, 3}, nil)
	valueBuilder.AppendValues([]float64{1.1, 2.2, 3.3}, nil)

	idArray := idBuilder.NewArray()
	defer idArray.Release()
	valueArray := valueBuilder.NewArray()
	defer valueArray.Release()

	record := array.NewRecord(schema, []arrow.Array{idArray, valueArray}, 3)
	defer record.Release()

	// Add data
	err = table.Add(record, AddModeAppend)
	if err != nil {
		t.Fatalf("Failed to add data: %v", err)
	}

	// Verify row count
	count, err := table.CountRows()
	if err != nil {
		t.Fatalf("Failed to count rows: %v", err)
	}

	if count != 3 {
		t.Errorf("Expected 3 rows, got %d", count)
	}
}

func TestAddDataAppendMode(t *testing.T) {
	pool := memory.NewGoAllocator()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_db")

	db, err := Connect(dbPath)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer db.Close()

	schema := arrow.NewSchema(
		[]arrow.Field{
			{Name: "x", Type: arrow.PrimitiveTypes.Int32, Nullable: false},
		},
		nil,
	)

	table, err := db.CreateTableWithSchema("append_table", schema)
	if err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}
	defer table.Close()

	// Add first batch
	builder1 := array.NewInt32Builder(pool)
	builder1.AppendValues([]int32{1, 2, 3}, nil)
	arr1 := builder1.NewArray()
	record1 := array.NewRecord(schema, []arrow.Array{arr1}, 3)
	builder1.Release()
	arr1.Release()
	defer record1.Release()

	err = table.Add(record1, AddModeAppend)
	if err != nil {
		t.Fatalf("Failed to add first batch: %v", err)
	}

	// Add second batch
	builder2 := array.NewInt32Builder(pool)
	builder2.AppendValues([]int32{4, 5}, nil)
	arr2 := builder2.NewArray()
	record2 := array.NewRecord(schema, []arrow.Array{arr2}, 2)
	builder2.Release()
	arr2.Release()
	defer record2.Release()

	err = table.Add(record2, AddModeAppend)
	if err != nil {
		t.Fatalf("Failed to add second batch: %v", err)
	}

	// Verify total row count
	count, err := table.CountRows()
	if err != nil {
		t.Fatalf("Failed to count rows: %v", err)
	}

	if count != 5 {
		t.Errorf("Expected 5 rows after append, got %d", count)
	}
}

func TestAddDataOverwriteMode(t *testing.T) {
	pool := memory.NewGoAllocator()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_db")

	db, err := Connect(dbPath)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer db.Close()

	schema := arrow.NewSchema(
		[]arrow.Field{
			{Name: "x", Type: arrow.PrimitiveTypes.Int32, Nullable: false},
		},
		nil,
	)

	table, err := db.CreateTableWithSchema("overwrite_table", schema)
	if err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}
	defer table.Close()

	// Add first batch
	builder1 := array.NewInt32Builder(pool)
	builder1.AppendValues([]int32{1, 2, 3}, nil)
	arr1 := builder1.NewArray()
	record1 := array.NewRecord(schema, []arrow.Array{arr1}, 3)
	builder1.Release()
	arr1.Release()
	defer record1.Release()

	err = table.Add(record1, AddModeAppend)
	if err != nil {
		t.Fatalf("Failed to add first batch: %v", err)
	}

	// Overwrite with second batch
	builder2 := array.NewInt32Builder(pool)
	builder2.AppendValues([]int32{10, 20}, nil)
	arr2 := builder2.NewArray()
	record2 := array.NewRecord(schema, []arrow.Array{arr2}, 2)
	builder2.Release()
	arr2.Release()
	defer record2.Release()

	err = table.Add(record2, AddModeOverwrite)
	if err != nil {
		t.Fatalf("Failed to overwrite data: %v", err)
	}

	// Verify row count (should be 2, not 5)
	count, err := table.CountRows()
	if err != nil {
		t.Fatalf("Failed to count rows: %v", err)
	}

	if count != 2 {
		t.Errorf("Expected 2 rows after overwrite, got %d", count)
	}
}

func TestAddDataMultipleTypes(t *testing.T) {
	pool := memory.NewGoAllocator()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_db")

	db, err := Connect(dbPath)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer db.Close()

	// Create schema with multiple types
	schema := arrow.NewSchema(
		[]arrow.Field{
			{Name: "int32_col", Type: arrow.PrimitiveTypes.Int32, Nullable: false},
			{Name: "int64_col", Type: arrow.PrimitiveTypes.Int64, Nullable: false},
			{Name: "float32_col", Type: arrow.PrimitiveTypes.Float32, Nullable: false},
			{Name: "float64_col", Type: arrow.PrimitiveTypes.Float64, Nullable: false},
			{Name: "string_col", Type: arrow.BinaryTypes.String, Nullable: true},
		},
		nil,
	)

	table, err := db.CreateTableWithSchema("multi_type_table", schema)
	if err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}
	defer table.Close()

	// Build arrays
	int32Builder := array.NewInt32Builder(pool)
	int64Builder := array.NewInt64Builder(pool)
	float32Builder := array.NewFloat32Builder(pool)
	float64Builder := array.NewFloat64Builder(pool)
	stringBuilder := array.NewStringBuilder(pool)

	int32Builder.AppendValues([]int32{1, 2, 3}, nil)
	int64Builder.AppendValues([]int64{100, 200, 300}, nil)
	float32Builder.AppendValues([]float32{1.1, 2.2, 3.3}, nil)
	float64Builder.AppendValues([]float64{10.5, 20.5, 30.5}, nil)
	stringBuilder.AppendValues([]string{"a", "b", "c"}, nil)

	int32Array := int32Builder.NewArray()
	int64Array := int64Builder.NewArray()
	float32Array := float32Builder.NewArray()
	float64Array := float64Builder.NewArray()
	stringArray := stringBuilder.NewArray()

	int32Builder.Release()
	int64Builder.Release()
	float32Builder.Release()
	float64Builder.Release()
	stringBuilder.Release()

	defer int32Array.Release()
	defer int64Array.Release()
	defer float32Array.Release()
	defer float64Array.Release()
	defer stringArray.Release()

	record := array.NewRecord(schema,
		[]arrow.Array{int32Array, int64Array, float32Array, float64Array, stringArray}, 3)
	defer record.Release()

	// Add data
	err = table.Add(record, AddModeAppend)
	if err != nil {
		t.Fatalf("Failed to add data: %v", err)
	}

	// Verify row count
	count, err := table.CountRows()
	if err != nil {
		t.Fatalf("Failed to count rows: %v", err)
	}

	if count != 3 {
		t.Errorf("Expected 3 rows, got %d", count)
	}
}

func TestGetSchema(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_db")

	db, err := Connect(dbPath)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer db.Close()

	// Create schema
	schema := arrow.NewSchema(
		[]arrow.Field{
			{Name: "id", Type: arrow.PrimitiveTypes.Int32, Nullable: false},
			{Name: "name", Type: arrow.BinaryTypes.String, Nullable: true},
		},
		nil,
	)

	table, err := db.CreateTableWithSchema("schema_table", schema)
	if err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}
	defer table.Close()

	// Get schema
	retrievedSchema, err := table.Schema()
	if err != nil {
		t.Fatalf("Failed to get schema: %v", err)
	}

	// Verify fields
	if len(retrievedSchema.Fields()) != 2 {
		t.Errorf("Expected 2 fields, got %d", len(retrievedSchema.Fields()))
	}

	if retrievedSchema.Field(0).Name != "id" {
		t.Errorf("Expected first field to be 'id', got '%s'", retrievedSchema.Field(0).Name)
	}

	if retrievedSchema.Field(1).Name != "name" {
		t.Errorf("Expected second field to be 'name', got '%s'", retrievedSchema.Field(1).Name)
	}
}

func TestReadData(t *testing.T) {
	pool := memory.NewGoAllocator()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_db")

	db, err := Connect(dbPath)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer db.Close()

	schema := arrow.NewSchema(
		[]arrow.Field{
			{Name: "id", Type: arrow.PrimitiveTypes.Int32, Nullable: false},
			{Name: "value", Type: arrow.PrimitiveTypes.Float64, Nullable: false},
		},
		nil,
	)

	table, err := db.CreateTableWithSchema("read_table", schema)
	if err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}
	defer table.Close()

	// Add some data
	idBuilder := array.NewInt32Builder(pool)
	valueBuilder := array.NewFloat64Builder(pool)
	idBuilder.AppendValues([]int32{1, 2, 3}, nil)
	valueBuilder.AppendValues([]float64{1.1, 2.2, 3.3}, nil)
	idArray := idBuilder.NewArray()
	valueArray := valueBuilder.NewArray()
	record := array.NewRecord(schema, []arrow.Array{idArray, valueArray}, 3)

	err = table.Add(record, AddModeAppend)
	if err != nil {
		t.Fatalf("Failed to add data: %v", err)
	}

	idBuilder.Release()
	valueBuilder.Release()
	idArray.Release()
	valueArray.Release()
	record.Release()

	// Read the data back
	records, err := table.ToArrow(-1)
	if err != nil {
		t.Fatalf("Failed to read data: %v", err)
	}
	defer func() {
		for _, r := range records {
			r.Release()
		}
	}()

	// Verify we got at least one batch
	if len(records) == 0 {
		t.Fatal("Expected at least one record batch")
	}

	// Calculate total rows
	totalRows := int64(0)
	for _, r := range records {
		totalRows += r.NumRows()
	}

	if totalRows != 3 {
		t.Errorf("Expected 3 rows total, got %d", totalRows)
	}

	// Verify first batch has the right schema
	if len(records[0].Schema().Fields()) != 2 {
		t.Errorf("Expected 2 columns, got %d", len(records[0].Schema().Fields()))
	}
}

func TestReadDataWithLimit(t *testing.T) {
	pool := memory.NewGoAllocator()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_db")

	db, err := Connect(dbPath)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer db.Close()

	schema := arrow.NewSchema(
		[]arrow.Field{
			{Name: "x", Type: arrow.PrimitiveTypes.Int32, Nullable: false},
		},
		nil,
	)

	table, err := db.CreateTableWithSchema("limit_table", schema)
	if err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}
	defer table.Close()

	// Add 10 rows
	builder := array.NewInt32Builder(pool)
	builder.AppendValues([]int32{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}, nil)
	arr := builder.NewArray()
	record := array.NewRecord(schema, []arrow.Array{arr}, 10)
	builder.Release()
	arr.Release()

	err = table.Add(record, AddModeAppend)
	if err != nil {
		t.Fatalf("Failed to add data: %v", err)
	}
	record.Release()

	// Read with limit of 5
	records, err := table.ToArrow(5)
	if err != nil {
		t.Fatalf("Failed to read data: %v", err)
	}
	defer func() {
		for _, r := range records {
			r.Release()
		}
	}()

	// Calculate total rows
	totalRows := int64(0)
	for _, r := range records {
		totalRows += r.NumRows()
	}

	if totalRows > 5 {
		t.Errorf("Expected at most 5 rows, got %d", totalRows)
	}
}

func TestReadEmptyTable(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_db")

	db, err := Connect(dbPath)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer db.Close()

	schema := arrow.NewSchema(
		[]arrow.Field{
			{Name: "x", Type: arrow.PrimitiveTypes.Int32, Nullable: false},
		},
		nil,
	)

	table, err := db.CreateTableWithSchema("empty_table", schema)
	if err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}
	defer table.Close()

	// Read from empty table
	records, err := table.ToArrow(-1)
	if err != nil {
		t.Fatalf("Failed to read data: %v", err)
	}

	if len(records) != 0 {
		t.Errorf("Expected 0 record batches from empty table, got %d", len(records))
	}
}

func TestDataRoundtrip(t *testing.T) {
	pool := memory.NewGoAllocator()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_db")

	db, err := Connect(dbPath)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer db.Close()

	schema := arrow.NewSchema(
		[]arrow.Field{
			{Name: "id", Type: arrow.PrimitiveTypes.Int32, Nullable: false},
			{Name: "name", Type: arrow.BinaryTypes.String, Nullable: true},
			{Name: "score", Type: arrow.PrimitiveTypes.Float64, Nullable: false},
		},
		nil,
	)

	table, err := db.CreateTableWithSchema("roundtrip_table", schema)
	if err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}
	defer table.Close()

	// Create data
	idBuilder := array.NewInt32Builder(pool)
	nameBuilder := array.NewStringBuilder(pool)
	scoreBuilder := array.NewFloat64Builder(pool)

	originalIDs := []int32{1, 2, 3}
	originalNames := []string{"Alice", "Bob", "Charlie"}
	originalScores := []float64{95.5, 87.3, 92.1}

	idBuilder.AppendValues(originalIDs, nil)
	nameBuilder.AppendValues(originalNames, nil)
	scoreBuilder.AppendValues(originalScores, nil)

	idArray := idBuilder.NewArray()
	nameArray := nameBuilder.NewArray()
	scoreArray := scoreBuilder.NewArray()
	record := array.NewRecord(schema, []arrow.Array{idArray, nameArray, scoreArray}, 3)

	// Write data
	err = table.Add(record, AddModeAppend)
	if err != nil {
		t.Fatalf("Failed to add data: %v", err)
	}

	idBuilder.Release()
	nameBuilder.Release()
	scoreBuilder.Release()
	idArray.Release()
	nameArray.Release()
	scoreArray.Release()
	record.Release()

	// Read data back
	records, err := table.ToArrow(-1)
	if err != nil {
		t.Fatalf("Failed to read data: %v", err)
	}
	defer func() {
		for _, r := range records {
			r.Release()
		}
	}()

	// Verify data
	totalRows := int64(0)
	for _, r := range records {
		totalRows += r.NumRows()
	}

	if totalRows != 3 {
		t.Errorf("Expected 3 rows, got %d", totalRows)
	}

	// Verify schema
	if len(records[0].Schema().Fields()) != 3 {
		t.Errorf("Expected 3 fields, got %d", len(records[0].Schema().Fields()))
	}

	// Verify first column is int32
	if records[0].Schema().Field(0).Type.ID() != arrow.INT32 {
		t.Errorf("Expected first column to be INT32, got %s", records[0].Schema().Field(0).Type)
	}
}

// BenchmarkDataInsertion measures insertion performance
func BenchmarkDataInsertion(b *testing.B) {
	pool := memory.NewGoAllocator()
	tmpDir := b.TempDir()
	dbPath := filepath.Join(tmpDir, "bench_db")

	db, err := Connect(dbPath)
	if err != nil {
		b.Fatalf("Failed to connect: %v", err)
	}
	defer db.Close()

	schema := arrow.NewSchema(
		[]arrow.Field{
			{Name: "id", Type: arrow.PrimitiveTypes.Int32, Nullable: false},
			{Name: "value", Type: arrow.PrimitiveTypes.Float64, Nullable: false},
		},
		nil,
	)

	table, err := db.CreateTableWithSchema("bench_table", schema)
	if err != nil {
		b.Fatalf("Failed to create table: %v", err)
	}
	defer table.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		idBuilder := array.NewInt32Builder(pool)
		valueBuilder := array.NewFloat64Builder(pool)

		for j := 0; j < 100; j++ {
			idBuilder.Append(int32(j))
			valueBuilder.Append(float64(j) * 1.5)
		}

		idArray := idBuilder.NewArray()
		valueArray := valueBuilder.NewArray()
		record := array.NewRecord(schema, []arrow.Array{idArray, valueArray}, 100)

		err = table.Add(record, AddModeAppend)
		if err != nil {
			b.Fatalf("Failed to add data: %v", err)
		}

		idBuilder.Release()
		valueBuilder.Release()
		idArray.Release()
		valueArray.Release()
		record.Release()
	}
}

