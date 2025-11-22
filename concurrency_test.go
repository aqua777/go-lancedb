package lancedb_test

import (
	"math/rand"
	"os"
	"sync"
	"testing"

	"github.com/apache/arrow/go/v17/arrow"
	"github.com/apache/arrow/go/v17/arrow/array"
	"github.com/apache/arrow/go/v17/arrow/memory"
	lancedb "github.com/aqua777/go-lancedb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConcurrency(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "lancedb_concurrency_test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	db, err := lancedb.Connect(tmpDir)
	require.NoError(t, err)
	defer db.Close()

	schema := arrow.NewSchema([]arrow.Field{
		{Name: "id", Type: arrow.PrimitiveTypes.Int32},
		{Name: "vector", Type: arrow.FixedSizeListOf(128, arrow.PrimitiveTypes.Float32)},
	}, nil)

	tableName := "concurrent_table"
	table, err := db.CreateTableWithSchema(tableName, schema)
	require.NoError(t, err)
	defer table.Close()

	// Insert initial data
	mem := memory.NewGoAllocator()
	builder := array.NewRecordBuilder(mem, schema)
	defer builder.Release()

	idBuilder := builder.Field(0).(*array.Int32Builder)
	vecBuilder := builder.Field(1).(*array.FixedSizeListBuilder)
	vecValues := vecBuilder.ValueBuilder().(*array.Float32Builder)

	for i := 0; i < 100; i++ {
		idBuilder.Append(int32(i))
		vecBuilder.Append(true)
		for j := 0; j < 128; j++ {
			vecValues.Append(rand.Float32())
		}
	}

	record := builder.NewRecord()
	defer record.Release()
	err = table.Add(record, lancedb.AddModeAppend)
	require.NoError(t, err)

	// Run concurrent operations
	var wg sync.WaitGroup
	numGoroutines := 10

	// Concurrent Readers
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 5; j++ {
				count, err := table.CountRows()
				assert.NoError(t, err)
				assert.GreaterOrEqual(t, count, int64(100))

				// Vector search
				queryVec := make([]float32, 128)
				for k := 0; k < 128; k++ {
					queryVec[k] = rand.Float32()
				}
				results, err := table.Query().NearestTo(queryVec).Limit(5).Execute()
				assert.NoError(t, err)
				for _, r := range results {
					r.Release()
				}
			}
		}()
	}

	// Concurrent Writers (Insert)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			// Create a new record for each insert to avoid race on the record itself
			localBuilder := array.NewRecordBuilder(mem, schema)
			defer localBuilder.Release()

			lIdBuilder := localBuilder.Field(0).(*array.Int32Builder)
			lVecBuilder := localBuilder.Field(1).(*array.FixedSizeListBuilder)
			lVecValues := lVecBuilder.ValueBuilder().(*array.Float32Builder)

			lIdBuilder.Append(999)
			lVecBuilder.Append(true)
			for k := 0; k < 128; k++ {
				lVecValues.Append(rand.Float32())
			}

			rec := localBuilder.NewRecord()
			defer rec.Release()

			err := table.Add(rec, lancedb.AddModeAppend)
			assert.NoError(t, err)
		}()
	}

	wg.Wait()
}

func TestStreaming(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "lancedb_streaming_test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	db, err := lancedb.Connect(tmpDir)
	require.NoError(t, err)
	defer db.Close()

	schema := arrow.NewSchema([]arrow.Field{
		{Name: "id", Type: arrow.PrimitiveTypes.Int32},
	}, nil)

	table, err := db.CreateTableWithSchema("streaming_table", schema)
	require.NoError(t, err)
	defer table.Close()

	// Insert 1000 rows
	mem := memory.NewGoAllocator()
	builder := array.NewRecordBuilder(mem, schema)
	defer builder.Release()
	idBuilder := builder.Field(0).(*array.Int32Builder)

	for i := 0; i < 1000; i++ {
		idBuilder.Append(int32(i))
	}
	record := builder.NewRecord()
	defer record.Release()
	err = table.Add(record, lancedb.AddModeAppend)
	require.NoError(t, err)

	// Test Streaming
	iter, err := table.Query().ExecuteStreaming()
	require.NoError(t, err)
	defer iter.Close()

	rowCount := 0
	for {
		batch, err := iter.Next()
		require.NoError(t, err)
		if batch == nil {
			break
		}
		rowCount += int(batch.NumRows())
		batch.Release()
	}

	assert.Equal(t, 1000, rowCount)
}
