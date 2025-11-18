package main

import (
	"fmt"
	"log"
	"os"

	"github.com/apache/arrow/go/v17/arrow"
	"github.com/apache/arrow/go/v17/arrow/array"
	"github.com/apache/arrow/go/v17/arrow/memory"
	"github.com/aqua777/go-lancedb"
)

func main() {
	dbPath := "./vector_search_demo.db"
	// Clean up at the end
	defer os.RemoveAll(dbPath)

	fmt.Println("🚀 LanceDB Go - Complete Vector Search Demo")
	fmt.Println("=" + string(make([]byte, 48)))

	// 1. Connect to database
	fmt.Println("\n1️⃣  Connecting to database...")
	db, err := lancedb.Connect(dbPath)
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer db.Close()
	fmt.Println("   ✓ Connected")

	// 2. Create table with vector column
	fmt.Println("\n2️⃣  Creating table with vector embeddings...")
	schema := arrow.NewSchema(
		[]arrow.Field{
			{Name: "id", Type: arrow.PrimitiveTypes.Int32},
			{Name: "text", Type: arrow.BinaryTypes.String},
			{Name: "category", Type: arrow.BinaryTypes.String},
			{Name: "embedding", Type: arrow.FixedSizeListOf(128, arrow.PrimitiveTypes.Float32)},
		},
		nil,
	)

	table, err := db.CreateTableWithSchema("documents", schema)
	if err != nil {
		log.Fatalf("Failed to create table: %v", err)
	}
	defer table.Close()
	fmt.Println("   ✓ Table created")

	// 3. Insert data
	fmt.Println("\n3️⃣  Inserting 500 document embeddings...")
	mem := memory.NewGoAllocator()
	recordBuilder := array.NewRecordBuilder(mem, schema)
	defer recordBuilder.Release()

	idBuilder := recordBuilder.Field(0).(*array.Int32Builder)
	textBuilder := recordBuilder.Field(1).(*array.StringBuilder)
	categoryBuilder := recordBuilder.Field(2).(*array.StringBuilder)
	embeddingBuilder := recordBuilder.Field(3).(*array.FixedSizeListBuilder)
	embeddingValueBuilder := embeddingBuilder.ValueBuilder().(*array.Float32Builder)

	categories := []string{"tech", "science", "business", "sports", "entertainment"}

	for i := 0; i < 500; i++ {
		idBuilder.Append(int32(i))
		textBuilder.Append(fmt.Sprintf("Document %d with interesting content", i))
		categoryBuilder.Append(categories[i%len(categories)])

		embeddingBuilder.Append(true)
		for j := 0; j < 128; j++ {
			// Simulate embedding vectors
			val := float32(i%10 + j%10)
			embeddingValueBuilder.Append(val)
		}
	}

	record := recordBuilder.NewRecord()
	defer record.Release()

	if err := table.Add(record, lancedb.AddModeAppend); err != nil {
		log.Fatalf("Failed to add data: %v", err)
	}
	fmt.Printf("   ✓ Inserted %d documents\n", record.NumRows())

	// 4. Create index for fast vector search
	fmt.Println("\n4️⃣  Creating IVF-PQ index on embeddings...")
	indexOpts := &lancedb.IndexOptions{
		IndexType:     lancedb.IndexTypeIVFPQ,
		Metric:        lancedb.DistanceMetricCosine,
		NumPartitions: 8,
		NumSubVectors: 16,
		Replace:       true,
	}

	if err := table.CreateIndex("embedding", indexOpts); err != nil {
		log.Fatalf("Failed to create index: %v", err)
	}
	fmt.Println("   ✓ Index created (IVF-PQ with Cosine similarity)")

	// 5. List indices
	fmt.Println("\n5️⃣  Listing table indices...")
	indices, err := table.ListIndices()
	if err != nil {
		log.Fatalf("Failed to list indices: %v", err)
	}
	fmt.Printf("   Found %d index(es):\n", len(indices))
	for _, idx := range indices {
		fmt.Printf("   - Name: %s\n", idx.Name)
		fmt.Printf("     Type: %s\n", idx.Type)
		fmt.Printf("     Columns: %v\n", idx.Columns)
	}

	// 6. Perform vector search
	fmt.Println("\n6️⃣  Performing vector search (top 10 nearest neighbors)...")
	queryVector := make([]float32, 128)
	for i := range queryVector {
		queryVector[i] = float32(5 + i%10)
	}

	query := table.Query()
	defer query.Close()

	results, err := query.
		NearestTo(queryVector).
		SetDistanceType(lancedb.DistanceTypeCosine).
		Limit(10).
		Select("id", "text", "category").
		Execute()

	if err != nil {
		log.Fatalf("Failed to execute search: %v", err)
	}

	fmt.Println("   ✓ Search results:")
	for i, result := range results {
		fmt.Printf("\n   Batch %d: %d rows\n", i+1, result.NumRows())

		idCol := result.Column(0).(*array.Int32)
		textCol := result.Column(1).(*array.String)
		categoryCol := result.Column(2).(*array.String)

		for row := 0; row < int(result.NumRows()); row++ {
			fmt.Printf("      [%d] ID=%d, Category=%s, Text=%s\n",
				row+1,
				idCol.Value(row),
				categoryCol.Value(row),
				textCol.Value(row))
		}
		result.Release()
	}

	// 7. Filtered vector search
	fmt.Println("\n7️⃣  Performing filtered search (category='tech', top 5)...")
	query2 := table.Query()
	defer query2.Close()

	results2, err := query2.
		NearestTo(queryVector).
		SetDistanceType(lancedb.DistanceTypeCosine).
		Where("category = 'tech'").
		Limit(5).
		Select("id", "text", "category").
		Execute()

	if err != nil {
		log.Fatalf("Failed to execute filtered search: %v", err)
	}

	fmt.Println("   ✓ Filtered results:")
	for i, result := range results2 {
		fmt.Printf("\n   Batch %d: %d rows\n", i+1, result.NumRows())

		idCol := result.Column(0).(*array.Int32)
		textCol := result.Column(1).(*array.String)
		categoryCol := result.Column(2).(*array.String)

		for row := 0; row < int(result.NumRows()); row++ {
			fmt.Printf("      [%d] ID=%d, Category=%s, Text=%s\n",
				row+1,
				idCol.Value(row),
				categoryCol.Value(row),
				textCol.Value(row))
		}
		result.Release()
	}

	// 8. Show table stats
	fmt.Println("\n8️⃣  Table statistics...")
	count, err := table.CountRows()
	if err != nil {
		log.Fatalf("Failed to count rows: %v", err)
	}
	fmt.Printf("   Total documents: %d\n", count)

	tableSchema, err := table.Schema()
	if err != nil {
		log.Fatalf("Failed to get schema: %v", err)
	}
	fmt.Printf("   Columns: %d\n", tableSchema.NumFields())
	for i := 0; i < tableSchema.NumFields(); i++ {
		field := tableSchema.Field(i)
		fmt.Printf("      - %s (%s)\n", field.Name, field.Type)
	}

	fmt.Println("\n✅ Demo completed successfully!")
	fmt.Println("\nKey features demonstrated:")
	fmt.Println("  ✓ Table creation with custom schema")
	fmt.Println("  ✓ Batch data insertion")
	fmt.Println("  ✓ Vector index creation (IVF-PQ)")
	fmt.Println("  ✓ Index management (list indices)")
	fmt.Println("  ✓ Vector search with k-NN")
	fmt.Println("  ✓ Filtered vector search")
	fmt.Println("  ✓ Multiple distance metrics")
	fmt.Println("  ✓ Column projection")
}

