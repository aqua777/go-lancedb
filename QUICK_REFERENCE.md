# LanceDB Go Bindings - Quick Reference Card

## Installation

### Option 1: Automated Setup (Recommended)
```bash
cd your-project
curl -sSL https://raw.githubusercontent.com/lancedb/lancedb/main/golang/scripts/setup-project.sh | bash
```

### Option 2: Manual Setup
```bash
# Add as submodule
git submodule add https://github.com/lancedb/lancedb.git vendor/lancedb

# Update go.mod
echo 'replace github.com/lancedb/lancedb/golang/cgo => ./vendor/lancedb/golang/cgo' >> go.mod
go get github.com/lancedb/lancedb/golang/cgo

# Build Rust library
cd vendor/lancedb/golang/rust-cgo && cargo build --release
```

---

## Basic Usage

### Connect
```go
db, err := lancedb.Connect("./my_database")
defer db.Close()
```

### Create Table
```go
schema := arrow.NewSchema([]arrow.Field{
    {Name: "id", Type: arrow.PrimitiveTypes.Int32},
    {Name: "text", Type: arrow.BinaryTypes.String},
    {Name: "embedding", Type: arrow.FixedSizeListOf(128, arrow.PrimitiveTypes.Float32)},
}, nil)

table, err := db.CreateTableWithSchema("docs", schema)
defer table.Close()
```

### Insert Data
```go
// Build record (see examples for complete code)
record := buildRecord(data)
defer record.Release()

err = table.Add(record, lancedb.AddModeAppend)
```

### Create Index
```go
opts := &lancedb.IndexOptions{
    IndexType: lancedb.IndexTypeIVFPQ,
    Metric:    lancedb.DistanceMetricCosine,
}
err = table.CreateIndex("embedding", opts)
```

### Vector Search
```go
results, err := table.Query().
    NearestTo(queryVector).
    SetDistanceType(lancedb.DistanceTypeCosine).
    Where("category = 'tech'").
    Limit(10).
    Execute()

for _, result := range results {
    // Process results
    result.Release()
}
```

---

## Common Patterns

### RAG System
```go
// 1. Setup
db, _ := lancedb.Connect("./rag_db")
table, _ := db.CreateTableWithSchema("knowledge", schema)

// 2. Ingest
table.Add(documents, lancedb.AddModeAppend)
table.CreateIndex("embedding", &lancedb.IndexOptions{
    IndexType: lancedb.IndexTypeIVFPQ,
    Metric:    lancedb.DistanceMetricCosine,
})

// 3. Retrieve
results, _ := table.Query().
    NearestTo(queryEmbedding).
    Limit(5).
    Execute()
```

### Semantic Search
```go
// Index your documents
docs := []Document{ /* ... */ }
table.Add(buildRecords(docs), lancedb.AddModeAppend)

// Search by meaning
results, _ := table.Query().
    NearestTo(getEmbedding("machine learning")).
    SetDistanceType(lancedb.DistanceMetricCosine).
    Limit(10).
    Execute()
```

### Filtered Search
```go
results, _ := table.Query().
    NearestTo(queryVector).
    Where("category = 'tech' AND date > '2024-01-01'").
    Limit(10).
    Select("id", "title", "summary").
    Execute()
```

---

## API Cheat Sheet

### Connection
| Method | Description |
|--------|-------------|
| `Connect(path)` | Open/create database |
| `db.Close()` | Close connection |
| `db.TableNames()` | List tables |
| `db.CreateTableWithSchema()` | Create table |
| `db.OpenTable(name)` | Open existing table |

### Table
| Method | Description |
|--------|-------------|
| `table.Close()` | Close table |
| `table.Add(record, mode)` | Insert data |
| `table.CountRows()` | Get row count |
| `table.Schema()` | Get schema |
| `table.ToArrow(limit)` | Read data |
| `table.CreateIndex(col, opts)` | Create index |
| `table.ListIndices()` | List indices |
| `table.Query()` | Start query |

### Query
| Method | Description |
|--------|-------------|
| `query.NearestTo(vec)` | Vector search |
| `query.SetDistanceType(dt)` | Set metric (L2/Cosine/Dot) |
| `query.Where(filter)` | SQL-like filter |
| `query.Limit(n)` | Top-K results |
| `query.Offset(n)` | Skip N results |
| `query.Select(cols...)` | Choose columns |
| `query.Execute()` | Run query |

### Types
```go
// Add modes
lancedb.AddModeAppend    // Append data
lancedb.AddModeOverwrite // Replace all data

// Distance metrics
lancedb.DistanceMetricL2      // Euclidean
lancedb.DistanceMetricCosine  // Cosine similarity
lancedb.DistanceMetricDot     // Dot product

// Index types
lancedb.IndexTypeIVFPQ  // IVF with Product Quantization
lancedb.IndexTypeAuto   // Auto-select
```

---

## Performance Tips

1. **Use Indices**: 5-10x faster searches
   ```go
   table.CreateIndex("embedding", nil) // Use defaults
   ```

2. **Batch Inserts**: Insert 1000s of rows at once
   ```go
   // Build large record with many rows
   table.Add(bigRecord, lancedb.AddModeAppend)
   ```

3. **Select Only Needed Columns**:
   ```go
   query.Select("id", "text") // Not all columns
   ```

4. **Use Cosine for Normalized Embeddings**:
   ```go
   query.SetDistanceType(lancedb.DistanceMetricCosine)
   ```

---

## Common Errors

### Library Not Found
```bash
# macOS
export DYLD_LIBRARY_PATH=/path/to/rust-cgo/target/release:$DYLD_LIBRARY_PATH

# Linux
export LD_LIBRARY_PATH=/path/to/rust-cgo/target/release:$LD_LIBRARY_PATH
```

### CGO Not Enabled
```bash
export CGO_ENABLED=1
go build
```

### Rebuild Rust Library
```bash
cd vendor/lancedb/golang/rust-cgo
cargo build --release
```

---

## Examples

### Minimal Example
```go
package main

import (
    "log"
    lancedb "github.com/lancedb/lancedb/golang/cgo"
    "github.com/apache/arrow/go/v17/arrow"
)

func main() {
    db, _ := lancedb.Connect("./db")
    defer db.Close()
    
    schema := arrow.NewSchema([]arrow.Field{
        {Name: "id", Type: arrow.PrimitiveTypes.Int32},
        {Name: "vec", Type: arrow.FixedSizeListOf(128, arrow.PrimitiveTypes.Float32)},
    }, nil)
    
    table, _ := db.CreateTableWithSchema("t", schema)
    defer table.Close()
    
    log.Println("✓ LanceDB working!")
}
```

### Complete Examples
See `vendor/lancedb/golang/cgo/example/`:
- `main.go` - Full walkthrough
- `starter_template.go` - Copy/paste template
- `vector_search_with_index.go` - RAG demo

---

## Resources

- [Full Documentation](USAGE_AS_DEPENDENCY.md)
- [Features](FEATURES.md)
- [Implementation Status](IMPLEMENTATION_STATUS.md)
- [README](README.md)

---

## Need Help?

1. Check [Troubleshooting](USAGE_AS_DEPENDENCY.md#troubleshooting)
2. Run examples: `go run vendor/lancedb/golang/cgo/example/main.go`
3. Open an issue on GitHub

---

**Version**: 0.10.0  
**Status**: Production-Ready for RAG Systems ✅

