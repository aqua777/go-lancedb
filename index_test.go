package lancedb

import (
	"os"
	"testing"

	"github.com/apache/arrow/go/v17/arrow"
	"github.com/apache/arrow/go/v17/arrow/array"
	"github.com/apache/arrow/go/v17/arrow/memory"
)

// TestCreateIndex tests basic index creation
func TestCreateIndex(t *testing.T) {
	dbPath := t.TempDir() + "/test_create_index.db"
	defer os.RemoveAll(dbPath)

	db, err := Connect(dbPath)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer db.Close()

	// Create table with vector data
	schema := arrow.NewSchema(
		[]arrow.Field{
			{Name: "id", Type: arrow.PrimitiveTypes.Int32},
			{Name: "vector", Type: arrow.FixedSizeListOf(128, arrow.PrimitiveTypes.Float32)},
		},
		nil,
	)

	table, err := db.CreateTableWithSchema("vectors", schema)
	if err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}
	defer table.Close()

	// Add some test data (need at least 256 rows for IVF-PQ index)
	mem := memory.NewGoAllocator()
	recordBuilder := array.NewRecordBuilder(mem, schema)
	defer recordBuilder.Release()

	idBuilder := recordBuilder.Field(0).(*array.Int32Builder)
	vecBuilder := recordBuilder.Field(1).(*array.FixedSizeListBuilder)
	vecValueBuilder := vecBuilder.ValueBuilder().(*array.Float32Builder)

	for i := 0; i < 300; i++ {
		idBuilder.Append(int32(i))
		vecBuilder.Append(true)
		for j := 0; j < 128; j++ {
			vecValueBuilder.Append(float32(i%10 + j))
		}
	}

	record := recordBuilder.NewRecord()
	defer record.Release()

	if err := table.Add(record, AddModeAppend); err != nil {
		t.Fatalf("Failed to add data: %v", err)
	}

	// Create index with default options
	err = table.CreateIndex("vector", nil)
	if err != nil {
		t.Fatalf("Failed to create index: %v", err)
	}

	// Verify index was created by listing indices
	indices, err := table.ListIndices()
	if err != nil {
		t.Fatalf("Failed to list indices: %v", err)
	}

	if len(indices) == 0 {
		t.Fatalf("Expected at least one index, got none")
	}

	found := false
	for _, idx := range indices {
		if len(idx.Columns) > 0 && idx.Columns[0] == "vector" {
			found = true
			break
		}
	}

	if !found {
		t.Errorf("Expected to find index on 'vector' column")
	}
}

// TestCreateIndexWithOptions tests index creation with custom options
func TestCreateIndexWithOptions(t *testing.T) {
	dbPath := t.TempDir() + "/test_index_options.db"
	defer os.RemoveAll(dbPath)

	db, err := Connect(dbPath)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer db.Close()

	schema := arrow.NewSchema(
		[]arrow.Field{
			{Name: "id", Type: arrow.PrimitiveTypes.Int32},
			{Name: "embedding", Type: arrow.FixedSizeListOf(64, arrow.PrimitiveTypes.Float32)},
		},
		nil,
	)

	table, err := db.CreateTableWithSchema("embeddings", schema)
	if err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}
	defer table.Close()

	// Add test data
	mem := memory.NewGoAllocator()
	recordBuilder := array.NewRecordBuilder(mem, schema)
	defer recordBuilder.Release()

	idBuilder := recordBuilder.Field(0).(*array.Int32Builder)
	vecBuilder := recordBuilder.Field(1).(*array.FixedSizeListBuilder)
	vecValueBuilder := vecBuilder.ValueBuilder().(*array.Float32Builder)

	for i := 0; i < 300; i++ {
		idBuilder.Append(int32(i))
		vecBuilder.Append(true)
		for j := 0; j < 64; j++ {
			vecValueBuilder.Append(float32(i + j))
		}
	}

	record := recordBuilder.NewRecord()
	defer record.Release()

	if err := table.Add(record, AddModeAppend); err != nil {
		t.Fatalf("Failed to add data: %v", err)
	}

	// Create index with custom options
	opts := &IndexOptions{
		IndexType:     IndexTypeIVFPQ,
		Metric:        DistanceMetricCosine,
		NumPartitions: 4,
		NumSubVectors: 8,
		Replace:       true,
	}

	err = table.CreateIndex("embedding", opts)
	if err != nil {
		t.Fatalf("Failed to create index with options: %v", err)
	}

	// List indices
	indices, err := table.ListIndices()
	if err != nil {
		t.Fatalf("Failed to list indices: %v", err)
	}

	if len(indices) == 0 {
		t.Fatalf("Expected at least one index")
	}
}

// TestListIndicesEmpty tests listing indices on a table with no indices
func TestListIndicesEmpty(t *testing.T) {
	dbPath := t.TempDir() + "/test_list_empty.db"
	defer os.RemoveAll(dbPath)

	db, err := Connect(dbPath)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer db.Close()

	schema := arrow.NewSchema(
		[]arrow.Field{
			{Name: "id", Type: arrow.PrimitiveTypes.Int32},
		},
		nil,
	)

	table, err := db.CreateTableWithSchema("simple", schema)
	if err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}
	defer table.Close()

	// List indices on empty table
	indices, err := table.ListIndices()
	if err != nil {
		t.Fatalf("Failed to list indices: %v", err)
	}

	// Should return empty list, not error
	if indices == nil {
		t.Errorf("Expected empty slice, got nil")
	}
}

// TestReplaceIndex tests replacing an existing index
func TestReplaceIndex(t *testing.T) {
	dbPath := t.TempDir() + "/test_replace_index.db"
	defer os.RemoveAll(dbPath)

	db, err := Connect(dbPath)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer db.Close()

	schema := arrow.NewSchema(
		[]arrow.Field{
			{Name: "id", Type: arrow.PrimitiveTypes.Int32},
			{Name: "vec", Type: arrow.FixedSizeListOf(32, arrow.PrimitiveTypes.Float32)},
		},
		nil,
	)

	table, err := db.CreateTableWithSchema("test_replace", schema)
	if err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}
	defer table.Close()

	// Add data
	mem := memory.NewGoAllocator()
	recordBuilder := array.NewRecordBuilder(mem, schema)
	defer recordBuilder.Release()

	idBuilder := recordBuilder.Field(0).(*array.Int32Builder)
	vecBuilder := recordBuilder.Field(1).(*array.FixedSizeListBuilder)
	vecValueBuilder := vecBuilder.ValueBuilder().(*array.Float32Builder)

	for i := 0; i < 300; i++ {
		idBuilder.Append(int32(i))
		vecBuilder.Append(true)
		for j := 0; j < 32; j++ {
			vecValueBuilder.Append(float32(i*j))
		}
	}

	record := recordBuilder.NewRecord()
	defer record.Release()

	if err := table.Add(record, AddModeAppend); err != nil {
		t.Fatalf("Failed to add data: %v", err)
	}

	// Create first index
	opts1 := &IndexOptions{
		Metric:  DistanceMetricL2,
		Replace: true,
	}
	if err := table.CreateIndex("vec", opts1); err != nil {
		t.Fatalf("Failed to create first index: %v", err)
	}

	// Replace with different metric
	opts2 := &IndexOptions{
		Metric:  DistanceMetricCosine,
		Replace: true,
	}
	if err := table.CreateIndex("vec", opts2); err != nil {
		t.Fatalf("Failed to replace index: %v", err)
	}

	// Should still have indices
	indices, err := table.ListIndices()
	if err != nil {
		t.Fatalf("Failed to list indices: %v", err)
	}

	if len(indices) == 0 {
		t.Errorf("Expected indices after replace")
	}
}

// TestDifferentDistanceMetrics tests creating indices with different distance metrics
func TestDifferentDistanceMetrics(t *testing.T) {
	dbPath := t.TempDir() + "/test_metrics.db"
	defer os.RemoveAll(dbPath)

	db, err := Connect(dbPath)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer db.Close()

	metrics := []struct {
		name   string
		metric DistanceMetric
	}{
		{"L2", DistanceMetricL2},
		{"Cosine", DistanceMetricCosine},
		{"Dot", DistanceMetricDot},
	}

	for _, tc := range metrics {
		t.Run(tc.name, func(t *testing.T) {
			schema := arrow.NewSchema(
				[]arrow.Field{
					{Name: "id", Type: arrow.PrimitiveTypes.Int32},
					{Name: "vec", Type: arrow.FixedSizeListOf(16, arrow.PrimitiveTypes.Float32)},
				},
				nil,
			)

			table, err := db.CreateTableWithSchema("test_"+tc.name, schema)
			if err != nil {
				t.Fatalf("Failed to create table: %v", err)
			}
			defer table.Close()

			// Add minimal data
			mem := memory.NewGoAllocator()
			recordBuilder := array.NewRecordBuilder(mem, schema)
			defer recordBuilder.Release()

			idBuilder := recordBuilder.Field(0).(*array.Int32Builder)
			vecBuilder := recordBuilder.Field(1).(*array.FixedSizeListBuilder)
			vecValueBuilder := vecBuilder.ValueBuilder().(*array.Float32Builder)

			for i := 0; i < 300; i++ {
				idBuilder.Append(int32(i))
				vecBuilder.Append(true)
				for j := 0; j < 16; j++ {
					vecValueBuilder.Append(float32(i + j))
				}
			}

			record := recordBuilder.NewRecord()
			defer record.Release()

			if err := table.Add(record, AddModeAppend); err != nil {
				t.Fatalf("Failed to add data: %v", err)
			}

			// Create index with this metric
			opts := &IndexOptions{
				Metric:  tc.metric,
				Replace: true,
			}

			if err := table.CreateIndex("vec", opts); err != nil {
				t.Fatalf("Failed to create %s index: %v", tc.name, err)
			}
		})
	}
}

// BenchmarkCreateIndex benchmarks index creation performance
func BenchmarkCreateIndex(b *testing.B) {
	dbPath := b.TempDir() + "/bench_index.db"
	defer os.RemoveAll(dbPath)

	db, err := Connect(dbPath)
	if err != nil {
		b.Fatalf("Failed to connect: %v", err)
	}
	defer db.Close()

	schema := arrow.NewSchema(
		[]arrow.Field{
			{Name: "id", Type: arrow.PrimitiveTypes.Int32},
			{Name: "vec", Type: arrow.FixedSizeListOf(128, arrow.PrimitiveTypes.Float32)},
		},
		nil,
	)

	table, err := db.CreateTableWithSchema("bench", schema)
	if err != nil {
		b.Fatalf("Failed to create table: %v", err)
	}
	defer table.Close()

	// Add data once
	mem := memory.NewGoAllocator()
	recordBuilder := array.NewRecordBuilder(mem, schema)
	defer recordBuilder.Release()

	idBuilder := recordBuilder.Field(0).(*array.Int32Builder)
	vecBuilder := recordBuilder.Field(1).(*array.FixedSizeListBuilder)
	vecValueBuilder := vecBuilder.ValueBuilder().(*array.Float32Builder)

	for i := 0; i < 1000; i++ {
		idBuilder.Append(int32(i))
		vecBuilder.Append(true)
		for j := 0; j < 128; j++ {
			vecValueBuilder.Append(float32(i + j))
		}
	}

	record := recordBuilder.NewRecord()
	defer record.Release()

	if err := table.Add(record, AddModeAppend); err != nil {
		b.Fatalf("Failed to add data: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		opts := &IndexOptions{
			Replace: true,
		}
		if err := table.CreateIndex("vec", opts); err != nil {
			b.Fatalf("Failed to create index: %v", err)
		}
	}
}

