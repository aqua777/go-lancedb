// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: Copyright The LanceDB Authors

package lancedb

/*
#include <stdlib.h>

// Forward declarations from other CGO files
struct ArrowArray;
struct ArrowSchema;

typedef void* QueryHandle;
typedef void* TableHandle;

extern QueryHandle lancedb_query_new(TableHandle table);
extern void lancedb_query_close(QueryHandle);
extern int lancedb_query_nearest_to(QueryHandle, float*, int);
extern int lancedb_query_distance_type(QueryHandle, int);
extern int lancedb_query_limit(QueryHandle, int);
extern int lancedb_query_offset(QueryHandle, int);
extern int lancedb_query_filter(QueryHandle, const char*);
extern int lancedb_query_select(QueryHandle, char**, int);
extern int lancedb_query_execute(QueryHandle, struct ArrowArray**, struct ArrowSchema**, int*);
*/
import "C"
import (
	"runtime"
	"unsafe"

	"github.com/apache/arrow/go/v17/arrow"
)

// DistanceType specifies the distance metric for vector search
type DistanceType int

const (
	// DistanceTypeL2 uses L2 (Euclidean) distance
	DistanceTypeL2 DistanceType = 0
	// DistanceTypeCosine uses cosine distance
	DistanceTypeCosine DistanceType = 1
	// DistanceTypeDot uses dot product distance
	DistanceTypeDot DistanceType = 2
)

// Query represents a query on a LanceDB table
type Query struct {
	handle C.QueryHandle
	table  *Table // Keep reference to prevent GC
}

// Query creates a new query for the table
func (t *Table) Query() *Query {
	handle := C.lancedb_query_new(C.TableHandle(t.handle))
	if handle == nil {
		return nil
	}

	q := &Query{handle: handle, table: t}
	runtime.SetFinalizer(q, (*Query).Close)
	return q
}

// Close releases the query resources
func (q *Query) Close() {
	if q.handle != nil {
		C.lancedb_query_close(q.handle)
		q.handle = nil
	}
}

// NearestTo sets the query vector for nearest neighbor search
// vector: the query vector to search for
func (q *Query) NearestTo(vector []float32) *Query {
	if len(vector) == 0 {
		return q
	}

	result := C.lancedb_query_nearest_to(q.handle, (*C.float)(unsafe.Pointer(&vector[0])), C.int(len(vector)))
	if int(result) != 0 {
		// Error occurred, but we can't return it here in fluent API
		// The error will be caught on Execute()
	}
	return q
}

// DistanceType sets the distance metric for the query
func (q *Query) SetDistanceType(dt DistanceType) *Query {
	result := C.lancedb_query_distance_type(q.handle, C.int(dt))
	if int(result) != 0 {
		// Error occurred, but we can't return it here in fluent API
	}
	return q
}

// Limit sets the maximum number of results to return
func (q *Query) Limit(limit int) *Query {
	result := C.lancedb_query_limit(q.handle, C.int(limit))
	if int(result) != 0 {
		// Error occurred, but we can't return it here in fluent API
	}
	return q
}

// Offset sets the number of results to skip
func (q *Query) Offset(offset int) *Query {
	result := C.lancedb_query_offset(q.handle, C.int(offset))
	if int(result) != 0 {
		// Error occurred, but we can't return it here in fluent API
	}
	return q
}

// Where sets a filter predicate for the query
// filter: SQL-like filter expression, e.g., "price > 100 AND category = 'electronics'"
func (q *Query) Where(filter string) *Query {
	cFilter := C.CString(filter)
	defer C.free(unsafe.Pointer(cFilter))

	result := C.lancedb_query_filter(q.handle, cFilter)
	if int(result) != 0 {
		// Error occurred, but we can't return it here in fluent API
	}
	return q
}

// Select specifies which columns to return in the results
func (q *Query) Select(columns ...string) *Query {
	if len(columns) == 0 {
		return q
	}

	// Convert Go strings to C array of strings
	cColumns := make([]*C.char, len(columns))
	for i, col := range columns {
		cColumns[i] = C.CString(col)
		defer C.free(unsafe.Pointer(cColumns[i]))
	}

	result := C.lancedb_query_select(q.handle, &cColumns[0], C.int(len(columns)))
	if int(result) != 0 {
		// Error occurred, but we can't return it here in fluent API
	}
	return q
}

// Execute runs the query and returns the results
func (q *Query) Execute() ([]arrow.Record, error) {
	var cArrays *C.struct_ArrowArray
	var cSchemas *C.struct_ArrowSchema
	var count C.int

	result := C.lancedb_query_execute(q.handle, &cArrays, &cSchemas, &count)
	if int(result) != 0 {
		return nil, getLastError()
	}

	numBatches := int(count)
	if numBatches == 0 {
		return []arrow.Record{}, nil
	}

	// Convert C arrays to Go slices
	cArraySlice := (*[1 << 30]C.struct_ArrowArray)(unsafe.Pointer(cArrays))[:numBatches:numBatches]
	cSchemaSlice := (*[1 << 30]C.struct_ArrowSchema)(unsafe.Pointer(cSchemas))[:numBatches:numBatches]

	records := make([]arrow.Record, numBatches)
	for i := 0; i < numBatches; i++ {
		record, err := RecordFromC(&cArraySlice[i], &cSchemaSlice[i])
		if err != nil {
			// Clean up any already-converted records
			for j := 0; j < i; j++ {
				records[j].Release()
			}
			C.free(unsafe.Pointer(cArrays))
			C.free(unsafe.Pointer(cSchemas))
			return nil, err
		}
		records[i] = record
	}

	// Free the C arrays (the individual records are now owned by Go)
	C.free(unsafe.Pointer(cArrays))
	C.free(unsafe.Pointer(cSchemas))

	return records, nil
}

