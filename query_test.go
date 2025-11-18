// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: Copyright The LanceDB Authors

package lancedb

import (
	"math"
	"path/filepath"
	"testing"

	"github.com/apache/arrow/go/v17/arrow"
	"github.com/apache/arrow/go/v17/arrow/array"
	"github.com/apache/arrow/go/v17/arrow/memory"
)

func TestVectorSearch(t *testing.T) {
	pool := memory.NewGoAllocator()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_db")

	db, err := Connect(dbPath)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer db.Close()

	// Create schema with vector column
	vectorField := arrow.Field{
		Name: "vector",
		Type: arrow.FixedSizeListOf(2, arrow.PrimitiveTypes.Float32),
		Nullable: false,
	}
	schema := arrow.NewSchema(
		[]arrow.Field{
			{Name: "id", Type: arrow.PrimitiveTypes.Int32, Nullable: false},
			vectorField,
		},
		nil,
	)

	table, err := db.CreateTableWithSchema("vector_table", schema)
	if err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}
	defer table.Close()

	// Create test data with vectors
	idBuilder := array.NewInt32Builder(pool)
	vectorBuilder := array.NewFixedSizeListBuilder(pool, 2, arrow.PrimitiveTypes.Float32)
	float32Builder := vectorBuilder.ValueBuilder().(*array.Float32Builder)

	// Add 5 vectors
	vectors := [][2]float32{
		{1.0, 0.0},
		{0.9, 0.1},
		{0.0, 1.0},
		{0.5, 0.5},
		{-1.0, 0.0},
	}

	for i, vec := range vectors {
		idBuilder.Append(int32(i))
		vectorBuilder.Append(true)
		float32Builder.Append(vec[0])
		float32Builder.Append(vec[1])
	}

	idArray := idBuilder.NewArray()
	vectorArray := vectorBuilder.NewArray()
	record := array.NewRecord(schema, []arrow.Array{idArray, vectorArray}, 5)

	err = table.Add(record, AddModeAppend)
	if err != nil {
		t.Fatalf("Failed to add data: %v", err)
	}

	idBuilder.Release()
	vectorBuilder.Release()
	idArray.Release()
	vectorArray.Release()
	record.Release()

	// Search for nearest neighbors to [1.0, 0.0]
	query := table.Query().
		NearestTo([]float32{1.0, 0.0}).
		Limit(3)

	results, err := query.Execute()
	if err != nil {
		t.Fatalf("Failed to execute query: %v", err)
	}
	defer func() {
		for _, r := range results {
			r.Release()
		}
	}()

	// Verify we got results
	totalRows := int64(0)
	for _, r := range results {
		totalRows += r.NumRows()
	}

	if totalRows == 0 {
		t.Error("Expected at least 1 result from vector search")
	}
	if totalRows > 3 {
		t.Errorf("Expected at most 3 results due to limit, got %d", totalRows)
	}
}

func TestVectorSearchWithFilter(t *testing.T) {
	pool := memory.NewGoAllocator()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_db")

	db, err := Connect(dbPath)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer db.Close()

	// Create schema
	vectorField := arrow.Field{
		Name: "vector",
		Type: arrow.FixedSizeListOf(3, arrow.PrimitiveTypes.Float32),
		Nullable: false,
	}
	schema := arrow.NewSchema(
		[]arrow.Field{
			{Name: "id", Type: arrow.PrimitiveTypes.Int32, Nullable: false},
			{Name: "category", Type: arrow.BinaryTypes.String, Nullable: false},
			vectorField,
		},
		nil,
	)

	table, err := db.CreateTableWithSchema("filtered_table", schema)
	if err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}
	defer table.Close()

	// Add data
	idBuilder := array.NewInt32Builder(pool)
	categoryBuilder := array.NewStringBuilder(pool)
	vectorBuilder := array.NewFixedSizeListBuilder(pool, 3, arrow.PrimitiveTypes.Float32)
	float32Builder := vectorBuilder.ValueBuilder().(*array.Float32Builder)

	testData := []struct {
		id       int32
		category string
		vector   [3]float32
	}{
		{1, "A", [3]float32{1.0, 0.0, 0.0}},
		{2, "B", [3]float32{0.9, 0.1, 0.0}},
		{3, "A", [3]float32{0.8, 0.2, 0.0}},
		{4, "B", [3]float32{0.0, 1.0, 0.0}},
		{5, "A", [3]float32{0.0, 0.0, 1.0}},
	}

	for _, data := range testData {
		idBuilder.Append(data.id)
		categoryBuilder.Append(data.category)
		vectorBuilder.Append(true)
		for _, v := range data.vector {
			float32Builder.Append(v)
		}
	}

	idArray := idBuilder.NewArray()
	categoryArray := categoryBuilder.NewArray()
	vectorArray := vectorBuilder.NewArray()
	record := array.NewRecord(schema, []arrow.Array{idArray, categoryArray, vectorArray}, 5)

	err = table.Add(record, AddModeAppend)
	if err != nil {
		t.Fatalf("Failed to add data: %v", err)
	}

	idBuilder.Release()
	categoryBuilder.Release()
	vectorBuilder.Release()
	idArray.Release()
	categoryArray.Release()
	vectorArray.Release()
	record.Release()

	// Search with filter
	query := table.Query().
		NearestTo([]float32{1.0, 0.0, 0.0}).
		Where("category = 'A'").
		Limit(10)

	results, err := query.Execute()
	if err != nil {
		t.Fatalf("Failed to execute query: %v", err)
	}
	defer func() {
		for _, r := range results {
			r.Release()
		}
	}()

	totalRows := int64(0)
	for _, r := range results {
		totalRows += r.NumRows()
	}

	if totalRows == 0 {
		t.Error("Expected results from filtered vector search")
	}

	// All results should be from category A (3 total in dataset)
	if totalRows > 3 {
		t.Errorf("Expected at most 3 results (only 3 'A' category items), got %d", totalRows)
	}
}

func TestQueryWithLimit(t *testing.T) {
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

	table, err := db.CreateTableWithSchema("limit_test", schema)
	if err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}
	defer table.Close()

	// Add 100 rows
	idBuilder := array.NewInt32Builder(pool)
	valueBuilder := array.NewFloat64Builder(pool)
	for i := 0; i < 100; i++ {
		idBuilder.Append(int32(i))
		valueBuilder.Append(float64(i) * 1.5)
	}

	idArray := idBuilder.NewArray()
	valueArray := valueBuilder.NewArray()
	record := array.NewRecord(schema, []arrow.Array{idArray, valueArray}, 100)

	err = table.Add(record, AddModeAppend)
	if err != nil {
		t.Fatalf("Failed to add data: %v", err)
	}

	idBuilder.Release()
	valueBuilder.Release()
	idArray.Release()
	valueArray.Release()
	record.Release()

	// Query with limit of 10
	query := table.Query().Limit(10)
	results, err := query.Execute()
	if err != nil {
		t.Fatalf("Failed to execute query: %v", err)
	}
	defer func() {
		for _, r := range results {
			r.Release()
		}
	}()

	totalRows := int64(0)
	for _, r := range results {
		totalRows += r.NumRows()
	}

	if totalRows != 10 {
		t.Errorf("Expected exactly 10 rows with limit, got %d", totalRows)
	}
}

func TestQueryWithOffset(t *testing.T) {
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
		},
		nil,
	)

	table, err := db.CreateTableWithSchema("offset_test", schema)
	if err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}
	defer table.Close()

	// Add 20 rows
	builder := array.NewInt32Builder(pool)
	for i := 0; i < 20; i++ {
		builder.Append(int32(i))
	}

	arr := builder.NewArray()
	record := array.NewRecord(schema, []arrow.Array{arr}, 20)

	err = table.Add(record, AddModeAppend)
	if err != nil {
		t.Fatalf("Failed to add data: %v", err)
	}

	builder.Release()
	arr.Release()
	record.Release()

	// Query with offset
	query := table.Query().Offset(10).Limit(5)
	results, err := query.Execute()
	if err != nil {
		t.Fatalf("Failed to execute query: %v", err)
	}
	defer func() {
		for _, r := range results {
			r.Release()
		}
	}()

	totalRows := int64(0)
	for _, r := range results {
		totalRows += r.NumRows()
	}

	if totalRows != 5 {
		t.Errorf("Expected 5 rows with offset+limit, got %d", totalRows)
	}
}

func TestQuerySelect(t *testing.T) {
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
			{Name: "name", Type: arrow.BinaryTypes.String, Nullable: false},
			{Name: "value", Type: arrow.PrimitiveTypes.Float64, Nullable: false},
		},
		nil,
	)

	table, err := db.CreateTableWithSchema("select_test", schema)
	if err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}
	defer table.Close()

	// Add data
	idBuilder := array.NewInt32Builder(pool)
	nameBuilder := array.NewStringBuilder(pool)
	valueBuilder := array.NewFloat64Builder(pool)

	idBuilder.Append(1)
	nameBuilder.Append("test")
	valueBuilder.Append(1.5)

	idArray := idBuilder.NewArray()
	nameArray := nameBuilder.NewArray()
	valueArray := valueBuilder.NewArray()
	record := array.NewRecord(schema, []arrow.Array{idArray, nameArray, valueArray}, 1)

	err = table.Add(record, AddModeAppend)
	if err != nil {
		t.Fatalf("Failed to add data: %v", err)
	}

	idBuilder.Release()
	nameBuilder.Release()
	valueBuilder.Release()
	idArray.Release()
	nameArray.Release()
	valueArray.Release()
	record.Release()

	// Query selecting only specific columns
	query := table.Query().Select("id", "name")
	results, err := query.Execute()
	if err != nil {
		t.Fatalf("Failed to execute query: %v", err)
	}
	defer func() {
		for _, r := range results {
			r.Release()
		}
	}()

	if len(results) == 0 {
		t.Fatal("Expected at least one result batch")
	}

	// Verify only 2 columns are returned
	if results[0].NumCols() != 2 {
		t.Errorf("Expected 2 columns in result, got %d", results[0].NumCols())
	}
}

func TestDistanceTypes(t *testing.T) {
	pool := memory.NewGoAllocator()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_db")

	db, err := Connect(dbPath)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer db.Close()

	// Create schema with vector column
	vectorField := arrow.Field{
		Name: "vector",
		Type: arrow.FixedSizeListOf(2, arrow.PrimitiveTypes.Float32),
		Nullable: false,
	}
	schema := arrow.NewSchema(
		[]arrow.Field{
			{Name: "id", Type: arrow.PrimitiveTypes.Int32, Nullable: false},
			vectorField,
		},
		nil,
	)

	table, err := db.CreateTableWithSchema("distance_test", schema)
	if err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}
	defer table.Close()

	// Add test vectors
	idBuilder := array.NewInt32Builder(pool)
	vectorBuilder := array.NewFixedSizeListBuilder(pool, 2, arrow.PrimitiveTypes.Float32)
	float32Builder := vectorBuilder.ValueBuilder().(*array.Float32Builder)

	vectors := [][2]float32{
		{1.0, 0.0},
		{0.0, 1.0},
		{math.Sqrt2 / 2, math.Sqrt2 / 2},
	}

	for i, vec := range vectors {
		idBuilder.Append(int32(i))
		vectorBuilder.Append(true)
		float32Builder.Append(vec[0])
		float32Builder.Append(vec[1])
	}

	idArray := idBuilder.NewArray()
	vectorArray := vectorBuilder.NewArray()
	record := array.NewRecord(schema, []arrow.Array{idArray, vectorArray}, 3)

	err = table.Add(record, AddModeAppend)
	if err != nil {
		t.Fatalf("Failed to add data: %v", err)
	}

	idBuilder.Release()
	vectorBuilder.Release()
	idArray.Release()
	vectorArray.Release()
	record.Release()

	// Test different distance metrics
	distanceTypes := []DistanceType{
		DistanceTypeL2,
		DistanceTypeCosine,
		DistanceTypeDot,
	}

	for _, dt := range distanceTypes {
		query := table.Query().
			NearestTo([]float32{1.0, 0.0}).
			SetDistanceType(dt).
			Limit(3)

		results, err := query.Execute()
		if err != nil {
			t.Errorf("Failed to execute query with distance type %d: %v", dt, err)
			continue
		}

		totalRows := int64(0)
		for _, r := range results {
			totalRows += r.NumRows()
			r.Release()
		}

		if totalRows == 0 {
			t.Errorf("Expected results with distance type %d", dt)
		}
	}
}

// BenchmarkVectorSearch measures vector search performance
func BenchmarkVectorSearch(b *testing.B) {
	pool := memory.NewGoAllocator()
	tmpDir := b.TempDir()
	dbPath := filepath.Join(tmpDir, "bench_db")

	db, _ := Connect(dbPath)
	defer db.Close()

	// Create table with 1000 vectors
	vectorField := arrow.Field{
		Name: "vector",
		Type: arrow.FixedSizeListOf(128, arrow.PrimitiveTypes.Float32),
		Nullable: false,
	}
	schema := arrow.NewSchema(
		[]arrow.Field{
			{Name: "id", Type: arrow.PrimitiveTypes.Int32, Nullable: false},
			vectorField,
		},
		nil,
	)

	table, _ := db.CreateTableWithSchema("bench_table", schema)
	defer table.Close()

	// Add test data
	idBuilder := array.NewInt32Builder(pool)
	vectorBuilder := array.NewFixedSizeListBuilder(pool, 128, arrow.PrimitiveTypes.Float32)
	float32Builder := vectorBuilder.ValueBuilder().(*array.Float32Builder)

	for i := 0; i < 1000; i++ {
		idBuilder.Append(int32(i))
		vectorBuilder.Append(true)
		for j := 0; j < 128; j++ {
			float32Builder.Append(float32(i+j) * 0.01)
		}
	}

	idArray := idBuilder.NewArray()
	vectorArray := vectorBuilder.NewArray()
	record := array.NewRecord(schema, []arrow.Array{idArray, vectorArray}, 1000)
	table.Add(record, AddModeAppend)

	idBuilder.Release()
	vectorBuilder.Release()
	idArray.Release()
	vectorArray.Release()
	record.Release()

	// Benchmark query
	queryVector := make([]float32, 128)
	for i := range queryVector {
		queryVector[i] = 0.5
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		query := table.Query().NearestTo(queryVector).Limit(10)
		results, _ := query.Execute()
		for _, r := range results {
			r.Release()
		}
	}
}

