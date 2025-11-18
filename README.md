# LanceDB Go Bindings

Production-ready Go bindings for [LanceDB](https://github.com/lancedb/lancedb) - a fast, serverless vector database for AI applications.

## Features

- 🚀 **High Performance**: Zero-copy Arrow integration with Rust-powered core
- 🔍 **Vector Search**: Fast k-NN search with multiple distance metrics (L2, Cosine, Dot Product)
- 📊 **Hybrid Queries**: Combine vector similarity with SQL-like filtering
- 🗂️ **Index Support**: IVF-PQ indexing for scaling to millions of vectors
- 🔧 **Flexible Schema**: Full Apache Arrow schema support with custom types
- 💾 **Serverless**: Embedded database, no separate server required
- 🎯 **RAG-Ready**: Perfect for Retrieval Augmented Generation systems

## Installation

### Prerequisites

- Go 1.21 or later
- Pre-built Rust library (included) or Rust 1.70+ for building from source

### Quick Install

```bash
# Clone the repository
git clone https://github.com/aqua777/lancedb.git
cd lancedb

# Build the Rust component (one-time)
cd rust-cgo && cargo build --release && cd ..

# Use in your Go project
cd go-lancedb
go get github.com/apache/arrow/go/v17
```

For development builds and cross-compilation, see [DEVELOPMENT.md](DEVELOPMENT.md).

## Quick Start

### Basic Example

```go
package main

import (
    "fmt"
    "log"

    "github.com/apache/arrow/go/v17/arrow"
    "github.com/apache/arrow/go/v17/arrow/array"
    "github.com/apache/arrow/go/v17/arrow/memory"
    lancedb "github.com/yourusername/lancedb-go/go-lancedb"
)

func main() {
    // Connect to database
    db, err := lancedb.Connect("./my_vector_db")
    if err != nil {
        log.Fatal(err)
    }
    defer db.Close()

    // Define schema with vector embeddings
    schema := arrow.NewSchema([]arrow.Field{
        {Name: "id", Type: arrow.PrimitiveTypes.Int32},
        {Name: "text", Type: arrow.BinaryTypes.String},
        {Name: "embedding", Type: arrow.FixedSizeListOf(128, arrow.PrimitiveTypes.Float32)},
    }, nil)

    // Create table
    table, err := db.CreateTableWithSchema("documents", schema)
    if err != nil {
        log.Fatal(err)
    }
    defer table.Close()

    // Insert data (see full example below)
    // ...

    // Vector search
    queryVector := []float32{0.1, 0.2, 0.3, /* ... 128 dimensions */}
    results, err := table.Query().
        NearestTo(queryVector).
        SetDistanceType(lancedb.DistanceTypeCosine).
        Limit(5).
        Execute()

    if err != nil {
        log.Fatal(err)
    }

    for _, result := range results {
        // Process results...
        result.Release()
    }
}
```

### Complete Working Example

See [examples/basic/main.go](go-lancedb/examples/basic/main.go) for a comprehensive example demonstrating:

- Database connection and table management
- Custom Arrow schemas with vector embeddings
- Batch data insertion (300 documents)
- Brute-force vector search
- IVF-PQ index creation
- Indexed vector search (10-100x faster)
- Filtered hybrid search (SQL WHERE + vectors)
- Multiple distance metrics comparison
- Column projection and pagination

Run it:

```bash
cd go-lancedb/examples/basic
go run main.go
```

## Usage Guide

### 1. Connection Management

```go
// Connect to local database (creates if doesn't exist)
db, err := lancedb.Connect("./my_database")
if err != nil {
    log.Fatal(err)
}
defer db.Close()

// List tables
tables, err := db.TableNames()
fmt.Println("Tables:", tables)

// Open existing table
table, err := db.OpenTable("documents")
defer table.Close()
```

### 2. Creating Tables with Schema

```go
// Define schema with vector column
schema := arrow.NewSchema([]arrow.Field{
    {Name: "id", Type: arrow.PrimitiveTypes.Int32},
    {Name: "title", Type: arrow.BinaryTypes.String},
    {Name: "category", Type: arrow.BinaryTypes.String},
    {Name: "score", Type: arrow.PrimitiveTypes.Float32},
    // Vector embedding: fixed-size list of 128 floats
    {Name: "embedding", Type: arrow.FixedSizeListOf(128, arrow.PrimitiveTypes.Float32)},
}, nil)

table, err := db.CreateTableWithSchema("documents", schema)
if err != nil {
    log.Fatal(err)
}
defer table.Close()
```

### 3. Inserting Data

```go
// Create record builder
mem := memory.NewGoAllocator()
builder := array.NewRecordBuilder(mem, schema)
defer builder.Release()

// Build data
idBuilder := builder.Field(0).(*array.Int32Builder)
titleBuilder := builder.Field(1).(*array.StringBuilder)
embeddingBuilder := builder.Field(4).(*array.FixedSizeListBuilder)
embeddingValues := embeddingBuilder.ValueBuilder().(*array.Float32Builder)

// Add rows
for i := 0; i < 100; i++ {
    idBuilder.Append(int32(i))
    titleBuilder.Append(fmt.Sprintf("Document %d", i))
    
    // Add 128-dimensional vector
    embeddingBuilder.Append(true)
    for j := 0; j < 128; j++ {
        embeddingValues.Append(float32(i) * 0.1)
    }
}

// Create record and insert
record := builder.NewRecord()
defer record.Release()

err = table.Add(record, lancedb.AddModeAppend)
if err != nil {
    log.Fatal(err)
}

fmt.Println("Rows inserted:", record.NumRows())
```

### 4. Vector Search

#### Basic Vector Search

```go
// Create query vector (same dimensions as schema)
queryVector := make([]float32, 128)
for i := range queryVector {
    queryVector[i] = float32(i) * 0.05
}

// Search for nearest neighbors
results, err := table.Query().
    NearestTo(queryVector).
    SetDistanceType(lancedb.DistanceTypeCosine).
    Limit(10).
    Execute()

if err != nil {
    log.Fatal(err)
}

// Process results
for _, record := range results {
    idCol := record.Column(0).(*array.Int32)
    titleCol := record.Column(1).(*array.String)
    
    for i := 0; i < int(record.NumRows()); i++ {
        fmt.Printf("ID: %d, Title: %s\n", idCol.Value(i), titleCol.Value(i))
    }
    record.Release()
}
```

#### Hybrid Search with Filters

```go
// Vector search with SQL-like filtering
results, err := table.Query().
    NearestTo(queryVector).
    SetDistanceType(lancedb.DistanceTypeCosine).
    Where("category = 'technology' AND score > 0.5").
    Limit(10).
    Select("id", "title", "category").  // Column projection
    Execute()
```

### 5. Creating Indices

For datasets with >10K vectors, create an index for faster search:

```go
// Create IVF-PQ index
opts := &lancedb.IndexOptions{
    IndexType:     lancedb.IndexTypeIVFPQ,
    Metric:        lancedb.DistanceMetricCosine,
    NumPartitions: 256,  // Number of clusters (default: auto)
    NumSubVectors: 16,   // Compression factor (default: auto)
    Replace:       true, // Replace existing index
}

err = table.CreateIndex("embedding", opts)
if err != nil {
    log.Fatal(err)
}

// List indices
indices, err := table.ListIndices()
for _, idx := range indices {
    fmt.Printf("Index: %s, Type: %s, Columns: %v\n", 
        idx.Name, idx.Type, idx.Columns)
}
```

**Index Performance**:
- **Without index**: O(N) - scans all vectors
- **With IVF-PQ**: O(sqrt(N)) - 10-100x faster on large datasets
- Trade-off: Slight recall reduction (typically 95-99% accuracy)

### 6. Distance Metrics

```go
// L2 (Euclidean) - good for absolute distances
query.SetDistanceType(lancedb.DistanceTypeL2)

// Cosine similarity - good for normalized vectors
query.SetDistanceType(lancedb.DistanceTypeCosine)

// Dot product - fast, for pre-normalized vectors
query.SetDistanceType(lancedb.DistanceTypeDot)
```

### 7. Query Options

```go
results, err := table.Query().
    NearestTo(queryVector).
    SetDistanceType(lancedb.DistanceTypeCosine).
    Where("category IN ('tech', 'science')").  // SQL filter
    Limit(20).                                   // Max results
    Offset(10).                                  // Skip first 10
    Select("id", "title", "score").             // Column projection
    Execute()
```

## API Reference

### Connection

```go
type Connection struct { /* ... */ }

// Create/open database
func Connect(uri string) (*Connection, error)

// Lifecycle
func (c *Connection) Close()
func (c *Connection) TableNames() ([]string, error)

// Table operations
func (c *Connection) OpenTable(name string) (*Table, error)
func (c *Connection) CreateTable(name string) (*Table, error)
func (c *Connection) CreateTableWithSchema(name string, schema *arrow.Schema) (*Table, error)
```

### Table

```go
type Table struct { /* ... */ }

// Data operations
func (t *Table) Add(record arrow.Record, mode AddMode) error
func (t *Table) CountRows() (int64, error)
func (t *Table) Schema() (*arrow.Schema, error)
func (t *Table) ToArrow(limit int64) ([]arrow.Record, error)

// Indexing
func (t *Table) CreateIndex(column string, opts *IndexOptions) error
func (t *Table) ListIndices() ([]IndexInfo, error)

// Querying
func (t *Table) Query() *Query

// Lifecycle
func (t *Table) Close()
```

### Query Builder

```go
type Query struct { /* ... */ }

// Vector search
func (q *Query) NearestTo(vector []float32) *Query
func (q *Query) SetDistanceType(dt DistanceType) *Query

// Filtering and pagination
func (q *Query) Where(filter string) *Query
func (q *Query) Limit(n int) *Query
func (q *Query) Offset(n int) *Query
func (q *Query) Select(columns ...string) *Query

// Execute
func (q *Query) Execute() ([]arrow.Record, error)
func (q *Query) Close()
```

### Types

```go
// Distance metrics
type DistanceType int
const (
    DistanceTypeL2     DistanceType = 0  // Euclidean distance
    DistanceTypeCosine DistanceType = 1  // Cosine similarity
    DistanceTypeDot    DistanceType = 2  // Dot product
)

// Index options
type IndexOptions struct {
    IndexType     IndexType      // IVFPq, Auto
    Metric        DistanceMetric // L2, Cosine, Dot
    NumPartitions int            // IVF partitions (0 = auto)
    NumSubVectors int            // PQ sub-vectors (0 = auto)
    Replace       bool           // Replace existing index
}

type DistanceMetric int
const (
    DistanceMetricL2     DistanceMetric = 0
    DistanceMetricCosine DistanceMetric = 1
    DistanceMetricDot    DistanceMetric = 2
)

// Add modes
type AddMode int
const (
    AddModeAppend    AddMode = 0  // Append to existing data
    AddModeOverwrite AddMode = 1  // Replace all data
)
```

## Feature Status

| Feature | Status | Notes |
|---------|--------|-------|
| Database connection | ✅ | Local databases |
| Table management | ✅ | Create, open, list |
| Custom schemas | ✅ | Full Arrow schema support |
| Data insertion | ✅ | Batch append/overwrite |
| Data reading | ✅ | Full table or limited reads |
| Vector search (k-NN) | ✅ | L2, Cosine, Dot metrics |
| Query filters | ✅ | SQL WHERE clauses |
| Pagination | ✅ | Limit & offset |
| Column selection | ✅ | SELECT specific fields |
| Vector indices | ✅ | IVF-PQ indexing |
| Index management | ✅ | Create and list indices |
| Arrow C FFI | ✅ | Zero-copy data transfer |
| Update operations | ❌ | Not implemented yet |
| Delete operations | ❌ | Not implemented yet |
| Full-text search | ❌ | Not implemented yet |
| Remote databases | ❌ | LanceDB Cloud support pending |

See [IMPLEMENTATION_STATUS.md](IMPLEMENTATION_STATUS.md) for detailed feature tracking.

## Performance Tips

1. **Use Indices**: Create IVF-PQ indices for datasets >10K vectors
2. **Batch Inserts**: Insert data in batches (1K-10K rows) rather than row-by-row
3. **Release Records**: Always call `record.Release()` to free memory
4. **Column Projection**: Use `Select()` to fetch only needed columns
5. **Appropriate Metrics**: Use Cosine for normalized vectors, L2 for absolute distances
6. **Tune Index Parameters**: Increase `NumPartitions` for larger datasets

## Common Patterns

### RAG (Retrieval Augmented Generation)

```go
// 1. Embed and store documents
embedding := embedText(document.Text)  // Your embedding model
record := createRecord(document.ID, document.Text, embedding)
table.Add(record, lancedb.AddModeAppend)

// 2. Create index after loading documents
table.CreateIndex("embedding", &lancedb.IndexOptions{
    IndexType: lancedb.IndexTypeIVFPQ,
    Metric:    lancedb.DistanceMetricCosine,
})

// 3. Query with context
queryEmbedding := embedText(userQuestion)
results, _ := table.Query().
    NearestTo(queryEmbedding).
    SetDistanceType(lancedb.DistanceTypeCosine).
    Limit(5).
    Execute()

// 4. Use results as context for LLM
context := extractTextFromResults(results)
answer := callLLM(userQuestion, context)
```

### Semantic Search

```go
// Store documents with metadata
schema := arrow.NewSchema([]arrow.Field{
    {Name: "id", Type: arrow.PrimitiveTypes.Int64},
    {Name: "title", Type: arrow.BinaryTypes.String},
    {Name: "content", Type: arrow.BinaryTypes.String},
    {Name: "category", Type: arrow.BinaryTypes.String},
    {Name: "timestamp", Type: arrow.PrimitiveTypes.Int64},
    {Name: "embedding", Type: arrow.FixedSizeListOf(384, arrow.PrimitiveTypes.Float32)},
}, nil)

// Search with filters
results, _ := table.Query().
    NearestTo(searchEmbedding).
    Where("category = 'news' AND timestamp > 1640000000").
    Limit(10).
    Execute()
```

## Troubleshooting

### "cannot find -llancedb_cgo"

The Rust library isn't built. Run:

```bash
cd rust-cgo && cargo build --release
```

### Memory Leaks

Always call `Close()` and `Release()` methods:

```go
db, _ := lancedb.Connect("./db")
defer db.Close()  // Important!

table, _ := db.OpenTable("docs")
defer table.Close()  // Important!

results, _ := query.Execute()
for _, record := range results {
    // Use record...
    record.Release()  // Important!
}
```

### Performance Issues

- Use release builds of Rust library (not debug)
- Create indices for large datasets
- Batch your inserts
- Use column projection to fetch less data

For more help, see [DEVELOPMENT.md](DEVELOPMENT.md).

## Contributing

Contributions welcome! See [DEVELOPMENT.md](DEVELOPMENT.md) for build instructions and contribution guidelines.

## License

Apache License 2.0

## Links

- [LanceDB Documentation](https://lancedb.github.io/lancedb/)
- [Apache Arrow Go](https://github.com/apache/arrow/tree/main/go)
- [Development Guide](DEVELOPMENT.md)
- [Implementation Status](IMPLEMENTATION_STATUS.md)
