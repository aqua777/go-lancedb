// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: Copyright The LanceDB Authors

package lancedb

/*
#include <stdlib.h>
#include <stdint.h>

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

typedef void* QueryStreamHandle;
extern QueryStreamHandle lancedb_query_execute_stream(QueryHandle);
extern int lancedb_stream_next(QueryStreamHandle, struct ArrowArray*, struct ArrowSchema*);
extern void lancedb_stream_close(QueryStreamHandle);
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
	err    error  // Capture errors during builder chain
}

// Query creates a new query for the table
func (t *Table) Query() *Query {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if t.handle == nil {
		return &Query{err: &Error{Message: "table is closed"}}
	}

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	handle := C.lancedb_query_new(C.TableHandle(t.handle))
	if handle == nil {
		return &Query{err: getLastError()}
	}

	q := &Query{handle: handle, table: t}
	runtime.SetFinalizer(q, (*Query).Close)
	return q
}

// Close releases the query resources
func (q *Query) Close() {
	if q.handle != nil {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		C.lancedb_query_close(q.handle)
		q.handle = nil
	}
}

// NearestTo sets the query vector for nearest neighbor search
// vector: the query vector to search for
// NearestTo sets the query vector for nearest neighbor search
// vector: the query vector to search for
func (q *Query) NearestTo(vector []float32) *Query {
	if q.err != nil {
		return q
	}
	if len(vector) == 0 {
		return q
	}

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	result := C.lancedb_query_nearest_to(q.handle, (*C.float)(unsafe.Pointer(&vector[0])), C.int(len(vector)))
	if int(result) != 0 {
		q.err = getLastError()
	}
	return q
}

// DistanceType sets the distance metric for the query
func (q *Query) SetDistanceType(dt DistanceType) *Query {
	if q.err != nil {
		return q
	}

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	result := C.lancedb_query_distance_type(q.handle, C.int(dt))
	if int(result) != 0 {
		q.err = getLastError()
	}
	return q
}

// Limit sets the maximum number of results to return
func (q *Query) Limit(limit int) *Query {
	if q.err != nil {
		return q
	}

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	result := C.lancedb_query_limit(q.handle, C.int(limit))
	if int(result) != 0 {
		q.err = getLastError()
	}
	return q
}

// Offset sets the number of results to skip
func (q *Query) Offset(offset int) *Query {
	if q.err != nil {
		return q
	}

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	result := C.lancedb_query_offset(q.handle, C.int(offset))
	if int(result) != 0 {
		q.err = getLastError()
	}
	return q
}

// Where sets a filter predicate for the query
// filter: SQL-like filter expression, e.g., "price > 100 AND category = 'electronics'"
func (q *Query) Where(filter string) *Query {
	if q.err != nil {
		return q
	}

	cFilter := C.CString(filter)
	defer C.free(unsafe.Pointer(cFilter))

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	result := C.lancedb_query_filter(q.handle, cFilter)
	if int(result) != 0 {
		q.err = getLastError()
	}
	return q
}

// Select specifies which columns to return in the results
func (q *Query) Select(columns ...string) *Query {
	if q.err != nil {
		return q
	}
	if len(columns) == 0 {
		return q
	}

	// Convert Go strings to C array of strings
	cColumns := make([]*C.char, len(columns))
	for i, col := range columns {
		cColumns[i] = C.CString(col)
		defer C.free(unsafe.Pointer(cColumns[i]))
	}

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	result := C.lancedb_query_select(q.handle, &cColumns[0], C.int(len(columns)))
	if int(result) != 0 {
		q.err = getLastError()
	}
	return q
}

// Execute runs the query and returns the results
func (q *Query) Execute() ([]arrow.Record, error) {
	if q.err != nil {
		return nil, q.err
	}

	var cArrays *C.struct_ArrowArray
	var cSchemas *C.struct_ArrowSchema
	var count C.int

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

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

// RecordIterator iterates over query results
type RecordIterator interface {
	Next() (arrow.Record, error)
	Close()
}

type streamIterator struct {
	handle C.QueryStreamHandle
	conn   *Connection // Keep reference
}

func (it *streamIterator) Next() (arrow.Record, error) {
	if it.handle == nil {
		return nil, nil
	}

	// Allocate C structures for one batch
	cArray := (*C.struct_ArrowArray)(C.malloc(C.sizeof_struct_ArrowArray))
	cSchema := (*C.struct_ArrowSchema)(C.malloc(C.sizeof_struct_ArrowSchema))

	if cArray == nil || cSchema == nil {
		if cArray != nil {
			C.free(unsafe.Pointer(cArray))
		}
		if cSchema != nil {
			C.free(unsafe.Pointer(cSchema))
		}
		return nil, &Error{Message: "failed to allocate Arrow C structures"}
	}

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	result := C.lancedb_stream_next(it.handle, cArray, cSchema)
	if int(result) < 0 {
		C.free(unsafe.Pointer(cArray))
		C.free(unsafe.Pointer(cSchema))
		return nil, getLastError()
	}

	if int(result) == 0 {
		// End of stream
		C.free(unsafe.Pointer(cArray))
		C.free(unsafe.Pointer(cSchema))
		return nil, nil
	}

	record, err := RecordFromC(cArray, cSchema)
	if err != nil {
		return nil, err
	}

	return record, nil
}

func (it *streamIterator) Close() {
	if it.handle != nil {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		C.lancedb_stream_close(it.handle)
		it.handle = nil
	}
}

// ExecuteStreaming runs the query and returns an iterator
func (q *Query) ExecuteStreaming() (RecordIterator, error) {
	if q.err != nil {
		return nil, q.err
	}

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	handle := C.lancedb_query_execute_stream(q.handle)
	if handle == nil {
		return nil, getLastError()
	}

	it := &streamIterator{handle: handle, conn: q.table.conn}
	runtime.SetFinalizer(it, (*streamIterator).Close)
	return it, nil
}
