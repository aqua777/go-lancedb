// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: Copyright The LanceDB Authors

package lancedb

import (
	"testing"

	"github.com/apache/arrow/go/v17/arrow"
	"github.com/apache/arrow/go/v17/arrow/array"
	"github.com/apache/arrow/go/v17/arrow/memory"
)

func TestSchemaConversion(t *testing.T) {
	// Create a simple schema
	schema := arrow.NewSchema(
		[]arrow.Field{
			{Name: "id", Type: arrow.PrimitiveTypes.Int32, Nullable: false},
			{Name: "name", Type: arrow.BinaryTypes.String, Nullable: true},
		},
		nil,
	)

	// Export to C
	cSchema, err := SchemaToC(schema)
	if err != nil {
		t.Fatalf("Failed to export schema to C: %v", err)
	}
	defer ReleaseArrowSchema(cSchema)

	// Import back
	importedSchema, err := SchemaFromC(cSchema)
	if err != nil {
		t.Fatalf("Failed to import schema from C: %v", err)
	}

	// Verify fields match
	if len(importedSchema.Fields()) != len(schema.Fields()) {
		t.Errorf("Field count mismatch: expected %d, got %d", 
			len(schema.Fields()), len(importedSchema.Fields()))
	}

	for i, field := range schema.Fields() {
		importedField := importedSchema.Fields()[i]
		if field.Name != importedField.Name {
			t.Errorf("Field %d name mismatch: expected %s, got %s", 
				i, field.Name, importedField.Name)
		}
		if field.Type.ID() != importedField.Type.ID() {
			t.Errorf("Field %d type mismatch: expected %s, got %s",
				i, field.Type, importedField.Type)
		}
	}
}

func TestRecordBatchConversion(t *testing.T) {
	pool := memory.NewGoAllocator()

	// Create a simple schema
	schema := arrow.NewSchema(
		[]arrow.Field{
			{Name: "id", Type: arrow.PrimitiveTypes.Int32, Nullable: false},
			{Name: "value", Type: arrow.PrimitiveTypes.Float64, Nullable: true},
		},
		nil,
	)

	// Create arrays
	idBuilder := array.NewInt32Builder(pool)
	defer idBuilder.Release()
	idBuilder.AppendValues([]int32{1, 2, 3, 4, 5}, nil)
	idArray := idBuilder.NewArray()
	defer idArray.Release()

	valueBuilder := array.NewFloat64Builder(pool)
	defer valueBuilder.Release()
	valueBuilder.AppendValues([]float64{1.1, 2.2, 3.3, 4.4, 5.5}, nil)
	valueArray := valueBuilder.NewArray()
	defer valueArray.Release()

	// Create record
	record := array.NewRecord(schema, []arrow.Array{idArray, valueArray}, 5)
	defer record.Release()

	// Export to C
	cArray, cSchema, err := RecordToC(record)
	if err != nil {
		t.Fatalf("Failed to export record to C: %v", err)
	}
	defer ReleaseArrowArray(cArray)
	defer ReleaseArrowSchema(cSchema)

	// Import back
	importedRecord, err := RecordFromC(cArray, cSchema)
	if err != nil {
		t.Fatalf("Failed to import record from C: %v", err)
	}
	defer importedRecord.Release()

	// Verify the data
	if importedRecord.NumRows() != record.NumRows() {
		t.Errorf("Row count mismatch: expected %d, got %d",
			record.NumRows(), importedRecord.NumRows())
	}

	if importedRecord.NumCols() != record.NumCols() {
		t.Errorf("Column count mismatch: expected %d, got %d",
			record.NumCols(), importedRecord.NumCols())
	}

	// Verify the schema
	if !importedRecord.Schema().Equal(record.Schema()) {
		t.Errorf("Schema mismatch")
	}
}

func TestRecordBatchBuilder(t *testing.T) {
	pool := memory.NewGoAllocator()

	schema := arrow.NewSchema(
		[]arrow.Field{
			{Name: "x", Type: arrow.PrimitiveTypes.Int64, Nullable: false},
			{Name: "y", Type: arrow.PrimitiveTypes.Int64, Nullable: false},
		},
		nil,
	)

	builder := NewRecordBatchBuilder(schema)

	// Build first column
	xBuilder := array.NewInt64Builder(pool)
	defer xBuilder.Release()
	xBuilder.AppendValues([]int64{1, 2, 3}, nil)
	xArray := xBuilder.NewArray()
	defer xArray.Release()

	err := builder.AddColumn(xArray)
	if err != nil {
		t.Fatalf("Failed to add first column: %v", err)
	}

	// Build second column
	yBuilder := array.NewInt64Builder(pool)
	defer yBuilder.Release()
	yBuilder.AppendValues([]int64{4, 5, 6}, nil)
	yArray := yBuilder.NewArray()
	defer yArray.Release()

	err = builder.AddColumn(yArray)
	if err != nil {
		t.Fatalf("Failed to add second column: %v", err)
	}

	// Build the record
	record, err := builder.Build()
	if err != nil {
		t.Fatalf("Failed to build record: %v", err)
	}
	defer record.Release()

	if record.NumRows() != 3 {
		t.Errorf("Expected 3 rows, got %d", record.NumRows())
	}
	if record.NumCols() != 2 {
		t.Errorf("Expected 2 columns, got %d", record.NumCols())
	}
}

func TestRecordBatchBuilderMismatchedLengths(t *testing.T) {
	pool := memory.NewGoAllocator()

	schema := arrow.NewSchema(
		[]arrow.Field{
			{Name: "x", Type: arrow.PrimitiveTypes.Int64, Nullable: false},
			{Name: "y", Type: arrow.PrimitiveTypes.Int64, Nullable: false},
		},
		nil,
	)

	builder := NewRecordBatchBuilder(schema)

	// Build first column with 3 elements
	xBuilder := array.NewInt64Builder(pool)
	defer xBuilder.Release()
	xBuilder.AppendValues([]int64{1, 2, 3}, nil)
	xArray := xBuilder.NewArray()
	defer xArray.Release()

	err := builder.AddColumn(xArray)
	if err != nil {
		t.Fatalf("Failed to add first column: %v", err)
	}

	// Try to build second column with 2 elements (should fail)
	yBuilder := array.NewInt64Builder(pool)
	defer yBuilder.Release()
	yBuilder.AppendValues([]int64{4, 5}, nil)
	yArray := yBuilder.NewArray()
	defer yArray.Release()

	err = builder.AddColumn(yArray)
	if err == nil {
		t.Error("Expected error for mismatched column lengths, got nil")
	}
}

func TestRecordReader(t *testing.T) {
	pool := memory.NewGoAllocator()

	schema := arrow.NewSchema(
		[]arrow.Field{
			{Name: "id", Type: arrow.PrimitiveTypes.Int32, Nullable: false},
		},
		nil,
	)

	// Create multiple records
	var records []arrow.Record
	for i := 0; i < 3; i++ {
		builder := array.NewInt32Builder(pool)
		builder.AppendValues([]int32{int32(i * 10), int32(i*10 + 1)}, nil)
		arr := builder.NewArray()
		record := array.NewRecord(schema, []arrow.Array{arr}, 2)
		records = append(records, record)
		builder.Release()
		arr.Release()
	}

	// Create reader
	reader := NewRecordReader(records)

	// Read all records
	count := 0
	for {
		record := reader.Next()
		if record == nil {
			break
		}
		count++
	}

	if count != 3 {
		t.Errorf("Expected to read 3 records, got %d", count)
	}

	// Close should be idempotent
	reader.Close()
	reader.Close()
}

// BenchmarkRecordConversion measures the performance of record conversion
func BenchmarkRecordConversion(b *testing.B) {
	pool := memory.NewGoAllocator()

	schema := arrow.NewSchema(
		[]arrow.Field{
			{Name: "id", Type: arrow.PrimitiveTypes.Int32, Nullable: false},
			{Name: "value", Type: arrow.PrimitiveTypes.Float64, Nullable: false},
		},
		nil,
	)

	// Create a record with 1000 rows
	idBuilder := array.NewInt32Builder(pool)
	valueBuilder := array.NewFloat64Builder(pool)
	
	for i := 0; i < 1000; i++ {
		idBuilder.Append(int32(i))
		valueBuilder.Append(float64(i) * 1.5)
	}
	
	idArray := idBuilder.NewArray()
	valueArray := valueBuilder.NewArray()
	record := array.NewRecord(schema, []arrow.Array{idArray, valueArray}, 1000)
	
	defer idBuilder.Release()
	defer valueBuilder.Release()
	defer idArray.Release()
	defer valueArray.Release()
	defer record.Release()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cArray, cSchema, err := RecordToC(record)
		if err != nil {
			b.Fatalf("Failed to export: %v", err)
		}
		
		importedRecord, err := RecordFromC(cArray, cSchema)
		if err != nil {
			b.Fatalf("Failed to import: %v", err)
		}
		
		importedRecord.Release()
		ReleaseArrowArray(cArray)
		ReleaseArrowSchema(cSchema)
	}
}

