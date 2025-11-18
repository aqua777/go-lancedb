// Package main is a starter template for using LanceDB Go bindings
//
// To use this template:
// 1. Copy this file to your project
// 2. Update the imports
// 3. Customize the schema and data for your use case
package main

import (
	"fmt"
	"log"

	"github.com/apache/arrow/go/v17/arrow"
	"github.com/apache/arrow/go/v17/arrow/array"
	"github.com/apache/arrow/go/v17/arrow/memory"
	"github.com/aqua777/go-lancedb"
)

// Config holds database configuration
type Config struct {
	DBPath        string
	TableName     string
	EmbeddingDims int
}

// Document represents a document with embedding
type Document struct {
	ID        int32
	Text      string
	Metadata  string
	Embedding []float32
}

func main() {
	// Configuration
	config := Config{
		DBPath:        "./my_lancedb",
		TableName:     "documents",
		EmbeddingDims: 128, // Adjust to your embedding model
	}

	// Initialize database
	db, err := initDatabase(config)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	// Create or open table
	table, err := setupTable(db, config)
	if err != nil {
		log.Fatalf("Failed to setup table: %v", err)
	}
	defer table.Close()

	// Example: Insert documents
	documents := []Document{
		{
			ID:        1,
			Text:      "LanceDB is a vector database",
			Metadata:  "category:tech",
			Embedding: generateMockEmbedding(config.EmbeddingDims, 1),
		},
		{
			ID:        2,
			Text:      "Go is a great programming language",
			Metadata:  "category:programming",
			Embedding: generateMockEmbedding(config.EmbeddingDims, 2),
		},
	}

	if err := insertDocuments(table, config, documents); err != nil {
		log.Fatalf("Failed to insert documents: %v", err)
	}
	fmt.Printf("✓ Inserted %d documents\n", len(documents))

	// Create index for fast search
	if err := createIndex(table); err != nil {
		log.Fatalf("Failed to create index: %v", err)
	}
	fmt.Println("✓ Created vector index")

	// Example: Search
	queryEmbedding := generateMockEmbedding(config.EmbeddingDims, 1)
	results, err := searchSimilar(table, queryEmbedding, 5)
	if err != nil {
		log.Fatalf("Failed to search: %v", err)
	}

	fmt.Println("\n📝 Search Results:")
	displayResults(results)

	// Clean up
	for _, result := range results {
		result.Release()
	}

	fmt.Println("\n✓ Starter template completed successfully!")
}

// initDatabase initializes a LanceDB connection
func initDatabase(config Config) (*lancedb.Connection, error) {
	db, err := lancedb.Connect(config.DBPath)
	if err != nil {
		return nil, fmt.Errorf("connect failed: %w", err)
	}
	return db, nil
}

// setupTable creates or opens a table with the appropriate schema
func setupTable(db *lancedb.Connection, config Config) (*lancedb.Table, error) {
	// Check if table exists
	tables, err := db.TableNames()
	if err != nil {
		return nil, fmt.Errorf("list tables failed: %w", err)
	}

	// Check if table already exists
	for _, name := range tables {
		if name == config.TableName {
			fmt.Printf("✓ Opening existing table '%s'\n", config.TableName)
			return db.OpenTable(config.TableName)
		}
	}

	// Create new table with schema
	fmt.Printf("✓ Creating new table '%s'\n", config.TableName)
	schema := arrow.NewSchema(
		[]arrow.Field{
			{Name: "id", Type: arrow.PrimitiveTypes.Int32, Nullable: false},
			{Name: "text", Type: arrow.BinaryTypes.String, Nullable: false},
			{Name: "metadata", Type: arrow.BinaryTypes.String, Nullable: true},
			{Name: "embedding", Type: arrow.FixedSizeListOf(int32(config.EmbeddingDims), arrow.PrimitiveTypes.Float32), Nullable: false},
		},
		nil,
	)

	return db.CreateTableWithSchema(config.TableName, schema)
}

// insertDocuments inserts documents into the table
func insertDocuments(table *lancedb.Table, config Config, documents []Document) error {
	schema, err := table.Schema()
	if err != nil {
		return fmt.Errorf("get schema failed: %w", err)
	}

	mem := memory.NewGoAllocator()
	builder := array.NewRecordBuilder(mem, schema)
	defer builder.Release()

	idBuilder := builder.Field(0).(*array.Int32Builder)
	textBuilder := builder.Field(1).(*array.StringBuilder)
	metadataBuilder := builder.Field(2).(*array.StringBuilder)
	embeddingBuilder := builder.Field(3).(*array.FixedSizeListBuilder)
	embeddingValueBuilder := embeddingBuilder.ValueBuilder().(*array.Float32Builder)

	for _, doc := range documents {
		if len(doc.Embedding) != config.EmbeddingDims {
			return fmt.Errorf("document %d has wrong embedding dimension: got %d, expected %d",
				doc.ID, len(doc.Embedding), config.EmbeddingDims)
		}

		idBuilder.Append(doc.ID)
		textBuilder.Append(doc.Text)
		metadataBuilder.Append(doc.Metadata)

		embeddingBuilder.Append(true)
		for _, val := range doc.Embedding {
			embeddingValueBuilder.Append(val)
		}
	}

	record := builder.NewRecord()
	defer record.Release()

	return table.Add(record, lancedb.AddModeAppend)
}

// createIndex creates a vector index on the embedding column
func createIndex(table *lancedb.Table) error {
	opts := &lancedb.IndexOptions{
		IndexType:     lancedb.IndexTypeIVFPQ,
		Metric:        lancedb.DistanceMetricCosine,
		NumPartitions: 0, // Auto-calculate
		NumSubVectors: 0, // Auto-calculate
		Replace:       true,
	}

	return table.CreateIndex("embedding", opts)
}

// searchSimilar performs vector similarity search
func searchSimilar(table *lancedb.Table, queryEmbedding []float32, limit int) ([]arrow.Record, error) {
	query := table.Query()
	defer query.Close()

	return query.
		NearestTo(queryEmbedding).
		SetDistanceType(lancedb.DistanceTypeCosine).
		Limit(limit).
		Select("id", "text", "metadata").
		Execute()
}

// displayResults prints search results
func displayResults(results []arrow.Record) {
	for i, result := range results {
		fmt.Printf("\nBatch %d (%d rows):\n", i+1, result.NumRows())

		idCol := result.Column(0).(*array.Int32)
		textCol := result.Column(1).(*array.String)
		metadataCol := result.Column(2).(*array.String)

		for row := 0; row < int(result.NumRows()); row++ {
			fmt.Printf("  [%d] %s\n", idCol.Value(row), textCol.Value(row))
			fmt.Printf("      Metadata: %s\n", metadataCol.Value(row))
		}
	}
}

// generateMockEmbedding generates a mock embedding vector for testing
// Replace this with your actual embedding model (OpenAI, Cohere, etc.)
func generateMockEmbedding(dims int, seed int32) []float32 {
	embedding := make([]float32, dims)
	for i := range embedding {
		embedding[i] = float32(seed + int32(i))
	}
	return embedding
}

// Example of integrating with an embedding API:
//
// func getEmbedding(text string) ([]float32, error) {
//     // OpenAI example
//     client := openai.NewClient(os.Getenv("OPENAI_API_KEY"))
//     resp, err := client.CreateEmbeddings(context.Background(), openai.EmbeddingRequest{
//         Model: openai.AdaEmbeddingV2,
//         Input: []string{text},
//     })
//     if err != nil {
//         return nil, err
//     }
//     return resp.Data[0].Embedding, nil
// }

// Example of filtering search results:
//
// func searchWithFilter(table *lancedb.Table, queryEmbedding []float32, filter string, limit int) ([]arrow.Record, error) {
//     query := table.Query()
//     defer query.Close()
//
//     return query.
//         NearestTo(queryEmbedding).
//         SetDistanceType(lancedb.DistanceTypeCosine).
//         Where(filter).  // e.g., "metadata = 'category:tech'"
//         Limit(limit).
//         Execute()
// }

// Example of pagination:
//
// func searchWithPagination(table *lancedb.Table, queryEmbedding []float32, page, pageSize int) ([]arrow.Record, error) {
//     query := table.Query()
//     defer query.Close()
//
//     offset := (page - 1) * pageSize
//     return query.
//         NearestTo(queryEmbedding).
//         SetDistanceType(lancedb.DistanceTypeCosine).
//         Offset(offset).
//         Limit(pageSize).
//         Execute()
// }
