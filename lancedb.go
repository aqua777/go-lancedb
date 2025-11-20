// Package lancedb provides Go bindings for LanceDB using CGO.
//
// LanceDB is a serverless, low-latency vector database for AI applications.
// This package provides a high-level Go API for creating connections,
// managing tables, and performing vector searches.
//
// Basic usage:
//
//	import "github.com/lancedb/lancedb/golang/cgo"
//
//	// Connect to a database
//	db, err := lancedb.Connect("./my_database")
//	if err != nil {
//		log.Fatal(err)
//	}
//	defer db.Close()
//
//	// Create a table
//	table, err := db.CreateTable("my_table")
//	if err != nil {
//		log.Fatal(err)
//	}
//	defer table.Close()
package lancedb

/*
// Uncomment this to use the executable path
#cgo LDFLAGS: -L${SRCDIR}/libs/darwin-arm64 -L${SRCDIR}/rust-cgo/target/release -llancedb_cgo -lm -ldl -Wl,-rpath,${SRCDIR}/libs/darwin-arm64 -Wl,-rpath,@executable_path
// #cgo LDFLAGS: -L${SRCDIR}/libs/darwin-arm64 -L${SRCDIR}/rust-cgo/target/release -llancedb_cgo -lm -ldl -Wl,-rpath,${SRCDIR}/libs/darwin-arm64 
#cgo darwin LDFLAGS: -framework CoreFoundation -framework Security
#include <stdlib.h>
#include <stdint.h>
#include <stdbool.h>

// Arrow C Data Interface structures (forward declaration from arrow.go)
struct ArrowArray;
struct ArrowSchema;

// Forward declarations of C functions
typedef void* ConnectionHandle;
typedef void* TableHandle;

extern int lancedb_init();
extern void lancedb_cleanup();
extern const char* lancedb_get_last_error();
extern void lancedb_free_string(char*);

extern ConnectionHandle lancedb_connect(const char* dataset_uri);
extern void lancedb_connection_close(ConnectionHandle);
extern int lancedb_connection_table_names(ConnectionHandle, const char*, int, char***, int*);

extern TableHandle lancedb_table_open(ConnectionHandle, const char* name);
extern TableHandle lancedb_table_create(ConnectionHandle, const char* name);
extern void lancedb_table_close(TableHandle);
extern int64_t lancedb_table_count_rows(TableHandle);
extern int lancedb_table_add(TableHandle, struct ArrowArray*, struct ArrowSchema*, int);
extern int lancedb_table_schema(TableHandle, struct ArrowSchema*);
extern TableHandle lancedb_table_create_with_schema(ConnectionHandle, const char* name, struct ArrowSchema*);
extern int lancedb_table_to_arrow(TableHandle, int64_t, struct ArrowArray**, struct ArrowSchema**, int*);

// Index management functions
extern int lancedb_table_create_index(TableHandle, const char* column, const char* index_type, int metric, int num_partitions, int num_sub_vectors, bool replace);
extern int lancedb_table_list_indices(TableHandle, char**);

// Delete operations
extern int lancedb_table_delete(TableHandle, const char* predicate);
*/
import "C"
import (
	"encoding/json"
	"runtime"
	"unsafe"

	"github.com/apache/arrow/go/v17/arrow"
)

// Error represents a LanceDB error
type Error struct {
	Message string
}

func (e *Error) Error() string {
	return e.Message
}

// getLastError retrieves the last error message from the C library
func getLastError() error {
	cErr := C.lancedb_get_last_error()
	if cErr == nil {
		return nil
	}
	errStr := C.GoString(cErr)
	return &Error{Message: errStr}
}

// Connection represents a connection to a LanceDB database
type Connection struct {
	handle C.ConnectionHandle
}

// Connect creates a new connection to a LanceDB database
func Connect(uri string) (*Connection, error) {
	cURI := C.CString(uri)
	defer C.free(unsafe.Pointer(cURI))

	handle := C.lancedb_connect(cURI)
	if handle == nil {
		return nil, getLastError()
	}

	conn := &Connection{handle: handle}
	runtime.SetFinalizer(conn, (*Connection).Close)
	return conn, nil
}

// Close closes the database connection
func (c *Connection) Close() {
	if c.handle != nil {
		C.lancedb_connection_close(c.handle)
		c.handle = nil
	}
}

// TableNames returns a list of table names in the database
func (c *Connection) TableNames() ([]string, error) {
	var cNames **C.char
	var count C.int

	result := C.lancedb_connection_table_names(c.handle, nil, 0, &cNames, &count)
	if int(result) != 0 {
		return nil, getLastError()
	}

	names := make([]string, int(count))
	cNamesSlice := (*[1 << 30]*C.char)(unsafe.Pointer(cNames))[:count:count]

	for i := 0; i < int(count); i++ {
		names[i] = C.GoString(cNamesSlice[i])
	}

	// Free the C strings and array
	for i := 0; i < int(count); i++ {
		C.free(unsafe.Pointer(cNamesSlice[i]))
	}
	C.free(unsafe.Pointer(cNames))

	return names, nil
}

// Table represents a LanceDB table
type Table struct {
	handle C.TableHandle
	conn   *Connection // Keep reference to prevent GC
}

// OpenTable opens an existing table
func (c *Connection) OpenTable(name string) (*Table, error) {
	cName := C.CString(name)
	defer C.free(unsafe.Pointer(cName))

	handle := C.lancedb_table_open(c.handle, cName)
	if handle == nil {
		return nil, getLastError()
	}

	table := &Table{handle: handle, conn: c}
	runtime.SetFinalizer(table, (*Table).Close)
	return table, nil
}

// CreateTable creates a new table with a default schema
func (c *Connection) CreateTable(name string) (*Table, error) {
	cName := C.CString(name)
	defer C.free(unsafe.Pointer(cName))

	handle := C.lancedb_table_create(c.handle, cName)
	if handle == nil {
		return nil, getLastError()
	}

	table := &Table{handle: handle, conn: c}
	runtime.SetFinalizer(table, (*Table).Close)
	return table, nil
}

// CreateTableWithSchema creates a new table with a custom schema
func (c *Connection) CreateTableWithSchema(name string, schema *arrow.Schema) (*Table, error) {
	if schema == nil {
		return nil, &Error{Message: "schema cannot be nil"}
	}

	cName := C.CString(name)
	defer C.free(unsafe.Pointer(cName))

	// Export schema to C
	cSchema, err := SchemaToC(schema)
	if err != nil {
		return nil, err
	}
	defer ReleaseArrowSchema(cSchema)

	handle := C.lancedb_table_create_with_schema(c.handle, cName, cSchema)
	if handle == nil {
		return nil, getLastError()
	}

	table := &Table{handle: handle, conn: c}
	runtime.SetFinalizer(table, (*Table).Close)
	return table, nil
}

// Close closes the table
func (t *Table) Close() {
	if t.handle != nil {
		C.lancedb_table_close(t.handle)
		t.handle = nil
	}
}

// CountRows returns the number of rows in the table
func (t *Table) CountRows() (int64, error) {
	count := C.lancedb_table_count_rows(t.handle)
	if count == -1 {
		return 0, getLastError()
	}
	return int64(count), nil
}

// AddMode specifies how to add data to a table
type AddMode int

const (
	// AddModeAppend appends data to the table
	AddModeAppend AddMode = 0
	// AddModeOverwrite replaces the table contents
	AddModeOverwrite AddMode = 1
)

// Add inserts a RecordBatch into the table
func (t *Table) Add(record arrow.Record, mode AddMode) error {
	if record == nil {
		return &Error{Message: "record cannot be nil"}
	}

	// Export record to C
	cArray, cSchema, err := RecordToC(record)
	if err != nil {
		return err
	}
	defer ReleaseArrowArray(cArray)
	defer ReleaseArrowSchema(cSchema)

	result := C.lancedb_table_add(t.handle, cArray, cSchema, C.int(mode))
	if int(result) != 0 {
		return getLastError()
	}

	return nil
}

// Schema returns the Arrow schema of the table
func (t *Table) Schema() (*arrow.Schema, error) {
	// We need to use the struct type from the C import block in arrow.go
	// For now, we'll create a temp schema C structure
	cSchema := (*C.struct_ArrowSchema)(C.malloc(C.size_t(unsafe.Sizeof(C.struct_ArrowSchema{}))))
	if cSchema == nil {
		return nil, &Error{Message: "failed to allocate schema structure"}
	}
	defer C.free(unsafe.Pointer(cSchema))

	result := C.lancedb_table_schema(t.handle, cSchema)
	if int(result) != 0 {
		return nil, getLastError()
	}

	schema, err := SchemaFromC(cSchema)
	if err != nil {
		return nil, err
	}

	return schema, nil
}

// ToArrow reads all data from the table and returns it as Arrow RecordBatch slices
// limit: maximum number of rows to read (-1 for no limit)
func (t *Table) ToArrow(limit int64) ([]arrow.Record, error) {
	var cArrays *C.struct_ArrowArray
	var cSchemas *C.struct_ArrowSchema
	var count C.int

	result := C.lancedb_table_to_arrow(t.handle, C.int64_t(limit), &cArrays, &cSchemas, &count)
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

// DistanceMetric represents the distance metric for vector indices
type DistanceMetric int

const (
	// DistanceMetricL2 is Euclidean (L2) distance
	DistanceMetricL2 DistanceMetric = 0
	// DistanceMetricCosine is cosine similarity
	DistanceMetricCosine DistanceMetric = 1
	// DistanceMetricDot is dot product
	DistanceMetricDot DistanceMetric = 2
)

// IndexType represents the type of index
type IndexType string

const (
	// IndexTypeIVFPQ is IVF with Product Quantization (most common for vector search)
	IndexTypeIVFPQ IndexType = "IVF_PQ"
	// IndexTypeAuto automatically chooses the best index type
	IndexTypeAuto IndexType = "AUTO"
)

// IndexOptions contains options for creating an index
type IndexOptions struct {
	// IndexType specifies the type of index to create (default: IVF_PQ)
	IndexType IndexType
	// Metric specifies the distance metric (default: L2)
	Metric DistanceMetric
	// NumPartitions specifies the number of IVF partitions (default: auto-calculated)
	NumPartitions int
	// NumSubVectors specifies the number of PQ sub-vectors (default: auto-calculated)
	NumSubVectors int
	// Replace specifies whether to replace an existing index (default: true)
	Replace bool
}

// IndexInfo contains information about an index
type IndexInfo struct {
	Name    string   `json:"name"`
	Type    string   `json:"type"`
	Columns []string `json:"columns"`
}

// CreateIndex creates an index on the specified column
func (t *Table) CreateIndex(column string, opts *IndexOptions) error {
	if opts == nil {
		opts = &IndexOptions{
			IndexType:     IndexTypeIVFPQ,
			Metric:        DistanceMetricL2,
			NumPartitions: 0, // 0 means use default
			NumSubVectors: 0, // 0 means use default
			Replace:       true,
		}
	}

	// Set default index type if not specified
	if opts.IndexType == "" {
		opts.IndexType = IndexTypeIVFPQ
	}

	cColumn := C.CString(column)
	defer C.free(unsafe.Pointer(cColumn))

	cIndexType := C.CString(string(opts.IndexType))
	defer C.free(unsafe.Pointer(cIndexType))

	result := C.lancedb_table_create_index(
		t.handle,
		cColumn,
		cIndexType,
		C.int(opts.Metric),
		C.int(opts.NumPartitions),
		C.int(opts.NumSubVectors),
		C.bool(opts.Replace),
	)

	if int(result) != 0 {
		return getLastError()
	}

	return nil
}

// ListIndices returns a list of all indices on the table
func (t *Table) ListIndices() ([]IndexInfo, error) {
	var cJSON *C.char
	result := C.lancedb_table_list_indices(t.handle, &cJSON)
	if int(result) < 0 {
		return nil, getLastError()
	}

	if cJSON == nil {
		return []IndexInfo{}, nil
	}
	defer C.lancedb_free_string(cJSON)

	jsonStr := C.GoString(cJSON)

	// Parse JSON manually (simple approach)
	var indices []IndexInfo
	if err := parseIndicesJSON(jsonStr, &indices); err != nil {
		return nil, err
	}

	return indices, nil
}

// parseIndicesJSON parses a JSON string into a slice of IndexInfo
func parseIndicesJSON(jsonStr string, indices *[]IndexInfo) error {
	return json.Unmarshal([]byte(jsonStr), indices)
}

// Delete removes rows from the table that match the given predicate.
// The predicate is a SQL-like expression (e.g., "id > 100" or "name = 'doc1'").
// This method automatically compacts the table to reclaim disk space.
func (t *Table) Delete(predicate string) error {
	if predicate == "" {
		return &Error{Message: "predicate cannot be empty"}
	}

	cPredicate := C.CString(predicate)
	defer C.free(unsafe.Pointer(cPredicate))

	result := C.lancedb_table_delete(t.handle, cPredicate)
	if int(result) != 0 {
		return getLastError()
	}

	return nil
}

// Initialize the LanceDB runtime
func init() {
	result := C.lancedb_init()
	if int(result) != 0 {
		panic("Failed to initialize LanceDB")
	}
}
