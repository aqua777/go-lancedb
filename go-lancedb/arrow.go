// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: Copyright The LanceDB Authors

package lancedb

/*
#include <stdlib.h>

// Arrow C Data Interface structures
// See: https://arrow.apache.org/docs/format/CDataInterface.html

struct ArrowSchema {
    const char* format;
    const char* name;
    const char* metadata;
    int64_t flags;
    int64_t n_children;
    struct ArrowSchema** children;
    struct ArrowSchema* dictionary;
    void (*release)(struct ArrowSchema*);
    void* private_data;
};

struct ArrowArray {
    int64_t length;
    int64_t null_count;
    int64_t offset;
    int64_t n_buffers;
    int64_t n_children;
    const void** buffers;
    struct ArrowArray** children;
    struct ArrowArray* dictionary;
    void (*release)(struct ArrowArray*);
    void* private_data;
};

// Forward declarations of Rust functions
extern void lancedb_arrow_array_release(struct ArrowArray* array);
extern void lancedb_arrow_schema_release(struct ArrowSchema* schema);
*/
import "C"
import (
	"fmt"
	"runtime"
	"unsafe"

	"github.com/apache/arrow/go/v17/arrow"
	"github.com/apache/arrow/go/v17/arrow/array"
	"github.com/apache/arrow/go/v17/arrow/cdata"
	"github.com/apache/arrow/go/v17/arrow/memory"
)

// ArrowAllocator is the memory allocator used for Arrow operations
var ArrowAllocator = memory.NewGoAllocator()

// RecordToC exports a Go arrow.Record to C Data Interface structures
//
// The caller is responsible for calling ReleaseArrowArray and ReleaseArrowSchema
// when done with the data.
func RecordToC(record arrow.Record) (*C.struct_ArrowArray, *C.struct_ArrowSchema, error) {
	if record == nil {
		return nil, nil, fmt.Errorf("record cannot be nil")
	}

	// Allocate C structures
	cArray := (*C.struct_ArrowArray)(C.malloc(C.sizeof_struct_ArrowArray))
	cSchema := (*C.struct_ArrowSchema)(C.malloc(C.sizeof_struct_ArrowSchema))

	if cArray == nil || cSchema == nil {
		if cArray != nil {
			C.free(unsafe.Pointer(cArray))
		}
		if cSchema != nil {
			C.free(unsafe.Pointer(cSchema))
		}
		return nil, nil, fmt.Errorf("failed to allocate Arrow C structures")
	}

	// Export using cdata package
	// Cast our C structs to cdata's C structs (they have the same layout)
	cdata.ExportArrowRecordBatch(record, 
		(*cdata.CArrowArray)(unsafe.Pointer(cArray)), 
		(*cdata.CArrowSchema)(unsafe.Pointer(cSchema)))

	return cArray, cSchema, nil
}

// RecordFromC imports a Go arrow.Record from C Data Interface structures
//
// This function takes ownership of the C structures and will release them.
func RecordFromC(cArray *C.struct_ArrowArray, cSchema *C.struct_ArrowSchema) (arrow.Record, error) {
	if cArray == nil || cSchema == nil {
		return nil, fmt.Errorf("C array and schema cannot be nil")
	}

	// Import using cdata package
	// Cast our C structs to cdata's C structs (they have the same layout)
	record, err := cdata.ImportCRecordBatch(
		(*cdata.CArrowArray)(unsafe.Pointer(cArray)),
		(*cdata.CArrowSchema)(unsafe.Pointer(cSchema)))
	if err != nil {
		return nil, fmt.Errorf("failed to import record batch: %w", err)
	}

	return record, nil
}

// SchemaToC exports a Go arrow.Schema to C Data Interface structure
func SchemaToC(schema *arrow.Schema) (*C.struct_ArrowSchema, error) {
	if schema == nil {
		return nil, fmt.Errorf("schema cannot be nil")
	}

	cSchema := (*C.struct_ArrowSchema)(C.malloc(C.sizeof_struct_ArrowSchema))
	if cSchema == nil {
		return nil, fmt.Errorf("failed to allocate Arrow schema structure")
	}

	// Export schema
	// Cast our C struct to cdata's C struct (they have the same layout)
	cdata.ExportArrowSchema(schema, (*cdata.CArrowSchema)(unsafe.Pointer(cSchema)))

	return cSchema, nil
}

// SchemaFromC imports a Go arrow.Schema from C Data Interface structure
func SchemaFromC(cSchema *C.struct_ArrowSchema) (*arrow.Schema, error) {
	if cSchema == nil {
		return nil, fmt.Errorf("C schema cannot be nil")
	}

	// Cast our C struct to cdata's C struct (they have the same layout)
	schema, err := cdata.ImportCArrowSchema((*cdata.CArrowSchema)(unsafe.Pointer(cSchema)))
	if err != nil {
		return nil, fmt.Errorf("failed to import schema: %w", err)
	}

	return schema, nil
}

// ReleaseArrowArray releases an Arrow C Data Interface array structure
func ReleaseArrowArray(cArray *C.struct_ArrowArray) {
	if cArray != nil {
		C.lancedb_arrow_array_release(cArray)
		C.free(unsafe.Pointer(cArray))
	}
}

// ReleaseArrowSchema releases an Arrow C Data Interface schema structure
func ReleaseArrowSchema(cSchema *C.struct_ArrowSchema) {
	if cSchema != nil {
		C.lancedb_arrow_schema_release(cSchema)
		C.free(unsafe.Pointer(cSchema))
	}
}

// RecordBatchBuilder is a helper for building Arrow RecordBatches
type RecordBatchBuilder struct {
	schema  *arrow.Schema
	columns []arrow.Array
	nRows   int64
}

// NewRecordBatchBuilder creates a new RecordBatchBuilder
func NewRecordBatchBuilder(schema *arrow.Schema) *RecordBatchBuilder {
	return &RecordBatchBuilder{
		schema:  schema,
		columns: make([]arrow.Array, 0, len(schema.Fields())),
		nRows:   -1,
	}
}

// AddColumn adds a column to the record batch
func (b *RecordBatchBuilder) AddColumn(arr arrow.Array) error {
	if b.nRows == -1 {
		b.nRows = int64(arr.Len())
	} else if int64(arr.Len()) != b.nRows {
		return fmt.Errorf("column length mismatch: expected %d, got %d", b.nRows, arr.Len())
	}
	
	b.columns = append(b.columns, arr)
	return nil
}

// Build creates the RecordBatch
func (b *RecordBatchBuilder) Build() (arrow.Record, error) {
	if len(b.columns) != len(b.schema.Fields()) {
		return nil, fmt.Errorf("column count mismatch: expected %d, got %d",
			len(b.schema.Fields()), len(b.columns))
	}

	record := array.NewRecord(b.schema, b.columns, b.nRows)
	return record, nil
}

// RecordReader provides streaming access to record batches
type RecordReader struct {
	records []arrow.Record
	current int
}

// NewRecordReader creates a new RecordReader
func NewRecordReader(records []arrow.Record) *RecordReader {
	return &RecordReader{
		records: records,
		current: 0,
	}
}

// Next returns the next record batch, or nil if there are no more
func (r *RecordReader) Next() arrow.Record {
	if r.current >= len(r.records) {
		return nil
	}
	record := r.records[r.current]
	r.current++
	return record
}

// Close releases all record batches
func (r *RecordReader) Close() {
	for _, record := range r.records {
		record.Release()
	}
	r.records = nil
}

// Ensure RecordReader is properly finalized
func init() {
	// Any RecordReader that wasn't explicitly closed will be cleaned up by GC
	runtime.SetFinalizer(&RecordReader{}, func(r *RecordReader) {
		r.Close()
	})
}

