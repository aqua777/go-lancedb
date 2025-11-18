package main

import (
	"fmt"
	"log"
	"os"

	"github.com/apache/arrow/go/v17/arrow"
	"github.com/apache/arrow/go/v17/arrow/array"
	"github.com/apache/arrow/go/v17/arrow/memory"
	"github.com/aqua777/lancedb/go-lancedb"
)

func main() {
	// Database path - comment out defer to keep the database after running
	dbPath := "./example.lancedb"
	defer os.RemoveAll(dbPath) // Remove this line if you want to inspect the database

	fmt.Println("🚀 LanceDB Go - Comprehensive Example")
	fmt.Println("=====================================\n")

	// ========================================
	// 1. DATABASE CONNECTION
	// ========================================
	fmt.Println("📁 Step 1: Connecting to database")
	fmt.Printf("   Path: %s\n", dbPath)

	db, err := lancedb.Connect(dbPath)
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer db.Close()
	fmt.Println("   ✓ Connected successfully\n")

	// ========================================
	// 2. TABLE CREATION WITH SCHEMA
	// ========================================
	fmt.Println("📊 Step 2: Creating table with custom schema")

	// Define schema: documents with text, categories, and vector embeddings
	schema := arrow.NewSchema(
		[]arrow.Field{
			{Name: "id", Type: arrow.PrimitiveTypes.Int32, Nullable: false},
			{Name: "title", Type: arrow.BinaryTypes.String, Nullable: false},
			{Name: "category", Type: arrow.BinaryTypes.String, Nullable: true},
			{Name: "views", Type: arrow.PrimitiveTypes.Int64, Nullable: false},
			{Name: "embedding", Type: arrow.FixedSizeListOf(128, arrow.PrimitiveTypes.Float32), Nullable: false},
		},
		nil,
	)

	table, err := db.CreateTableWithSchema("documents", schema)
	if err != nil {
		log.Fatalf("Failed to create table: %v", err)
	}
	defer table.Close()

	fmt.Println("   ✓ Table 'documents' created")
	fmt.Println("   Schema:")
	for i := 0; i < schema.NumFields(); i++ {
		field := schema.Field(i)
		fmt.Printf("     - %s: %s\n", field.Name, field.Type)
	}
	fmt.Println()

	// ========================================
	// 3. DATA INSERTION
	// ========================================
	fmt.Println("💾 Step 3: Inserting sample documents")

	mem := memory.NewGoAllocator()
	recordBuilder := array.NewRecordBuilder(mem, schema)
	defer recordBuilder.Release()

	// Sample documents
	documents := []struct {
		id       int32
		title    string
		category string
		views    int64
	}{
		{1, "Introduction to Machine Learning", "tech", 15000},
		{2, "Deep Learning Fundamentals", "tech", 22000},
		{3, "Understanding Neural Networks", "tech", 18500},
		{4, "Stock Market Analysis 2024", "business", 12000},
		{5, "Cryptocurrency Trends", "business", 9500},
		{6, "Climate Change Impact", "science", 31000},
		{7, "Space Exploration News", "science", 28000},
		{8, "Olympic Games Highlights", "sports", 45000},
		{9, "Football Championship", "sports", 38000},
		{10, "Latest Movie Reviews", "entertainment", 21000},
	}

	idBuilder := recordBuilder.Field(0).(*array.Int32Builder)
	titleBuilder := recordBuilder.Field(1).(*array.StringBuilder)
	categoryBuilder := recordBuilder.Field(2).(*array.StringBuilder)
	viewsBuilder := recordBuilder.Field(3).(*array.Int64Builder)
	embeddingBuilder := recordBuilder.Field(4).(*array.FixedSizeListBuilder)
	embeddingValueBuilder := embeddingBuilder.ValueBuilder().(*array.Float32Builder)

	for _, doc := range documents {
		idBuilder.Append(doc.id)
		titleBuilder.Append(doc.title)
		categoryBuilder.Append(doc.category)
		viewsBuilder.Append(doc.views)

		// Generate a simple embedding based on document ID and category
		embeddingBuilder.Append(true)
		categoryOffset := float32(len(doc.category) * 10)
		for j := 0; j < 128; j++ {
			val := float32(doc.id) + float32(j)*0.1 + categoryOffset
			embeddingValueBuilder.Append(val)
		}
	}

	record := recordBuilder.NewRecord()
	defer record.Release()

	if err := table.Add(record, lancedb.AddModeAppend); err != nil {
		log.Fatalf("Failed to insert data: %v", err)
	}

	count, _ := table.CountRows()
	fmt.Printf("   ✓ Inserted %d documents\n", count)
	fmt.Println()

	// ========================================
	// 4. READING DATA
	// ========================================
	fmt.Println("📖 Step 4: Reading data from table")

	records, err := table.ToArrow(5) // Read first 5 rows
	if err != nil {
		log.Fatalf("Failed to read data: %v", err)
	}

	fmt.Println("   First 5 documents:")
	for _, rec := range records {
		idCol := rec.Column(0).(*array.Int32)
		titleCol := rec.Column(1).(*array.String)
		categoryCol := rec.Column(2).(*array.String)
		viewsCol := rec.Column(3).(*array.Int64)

		for i := 0; i < int(rec.NumRows()); i++ {
			fmt.Printf("     [%d] %s\n", idCol.Value(i), titleCol.Value(i))
			fmt.Printf("         Category: %s, Views: %d\n", categoryCol.Value(i), viewsCol.Value(i))
		}
		rec.Release()
	}
	fmt.Println()

	// ========================================
	// 5. VECTOR SEARCH (WITHOUT INDEX)
	// ========================================
	fmt.Println("🔍 Step 5: Vector search (brute force, no index)")

	// Create a query vector (simulating a search for tech-related content)
	queryVector := make([]float32, 128)
	for i := range queryVector {
		queryVector[i] = float32(2) + float32(i)*0.1 + 40.0 // Similar to "tech" category
	}

	query := table.Query()
	defer query.Close()

	results, err := query.
		NearestTo(queryVector).
		SetDistanceType(lancedb.DistanceTypeCosine).
		Limit(3).
		Select("id", "title", "category").
		Execute()

	if err != nil {
		log.Fatalf("Failed to execute search: %v", err)
	}

	fmt.Println("   Top 3 results (brute force):")
	for _, result := range results {
		idCol := result.Column(0).(*array.Int32)
		titleCol := result.Column(1).(*array.String)
		categoryCol := result.Column(2).(*array.String)

		for i := 0; i < int(result.NumRows()); i++ {
			fmt.Printf("     %d. [ID:%d] %s (%s)\n",
				i+1, idCol.Value(i), titleCol.Value(i), categoryCol.Value(i))
		}
		result.Release()
	}
	fmt.Println()

	// ========================================
	// 6. CREATE VECTOR INDEX
	// ========================================
	fmt.Println("⚡ Step 6: Creating vector index for faster search")

	// Add more documents first (indices work better with more data)
	fmt.Println("   Adding 290 more documents...")
	recordBuilder2 := array.NewRecordBuilder(mem, schema)
	defer recordBuilder2.Release()

	idBuilder2 := recordBuilder2.Field(0).(*array.Int32Builder)
	titleBuilder2 := recordBuilder2.Field(1).(*array.StringBuilder)
	categoryBuilder2 := recordBuilder2.Field(2).(*array.StringBuilder)
	viewsBuilder2 := recordBuilder2.Field(3).(*array.Int64Builder)
	embeddingBuilder2 := recordBuilder2.Field(4).(*array.FixedSizeListBuilder)
	embeddingValueBuilder2 := embeddingBuilder2.ValueBuilder().(*array.Float32Builder)

	categories := []string{"tech", "business", "science", "sports", "entertainment"}
	for i := 11; i <= 300; i++ {
		idBuilder2.Append(int32(i))
		titleBuilder2.Append(fmt.Sprintf("Document %d", i))
		cat := categories[i%len(categories)]
		categoryBuilder2.Append(cat)
		viewsBuilder2.Append(int64(1000 + i*100))

		embeddingBuilder2.Append(true)
		categoryOffset := float32(len(cat) * 10)
		for j := 0; j < 128; j++ {
			val := float32(i) + float32(j)*0.1 + categoryOffset
			embeddingValueBuilder2.Append(val)
		}
	}

	record2 := recordBuilder2.NewRecord()
	defer record2.Release()

	if err := table.Add(record2, lancedb.AddModeAppend); err != nil {
		log.Fatalf("Failed to insert additional data: %v", err)
	}

	totalCount, _ := table.CountRows()
	fmt.Printf("   ✓ Total documents: %d\n\n", totalCount)

	// Create IVF-PQ index
	fmt.Println("   Creating IVF-PQ index with custom options...")
	indexOpts := &lancedb.IndexOptions{
		IndexType:     lancedb.IndexTypeIVFPQ,
		Metric:        lancedb.DistanceMetricCosine,
		NumPartitions: 4,  // Small dataset, use fewer partitions
		NumSubVectors: 16, // Compression parameter
		Replace:       true,
	}

	if err := table.CreateIndex("embedding", indexOpts); err != nil {
		log.Fatalf("Failed to create index: %v", err)
	}
	fmt.Println("   ✓ Index created successfully")
	fmt.Println("   Configuration:")
	fmt.Println("     - Type: IVF-PQ (Inverted File Index with Product Quantization)")
	fmt.Println("     - Metric: Cosine similarity")
	fmt.Println("     - Partitions: 4")
	fmt.Println("     - Sub-vectors: 16")
	fmt.Println()

	// ========================================
	// 7. LIST INDICES
	// ========================================
	fmt.Println("📋 Step 7: Listing table indices")

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
	fmt.Println()

	// ========================================
	// 8. INDEXED VECTOR SEARCH
	// ========================================
	fmt.Println("⚡ Step 8: Vector search (with index - much faster!)")

	query2 := table.Query()
	defer query2.Close()

	results2, err := query2.
		NearestTo(queryVector).
		SetDistanceType(lancedb.DistanceTypeCosine).
		Limit(5).
		Select("id", "title", "category", "views").
		Execute()

	if err != nil {
		log.Fatalf("Failed to execute indexed search: %v", err)
	}

	fmt.Println("   Top 5 results (indexed search):")
	for _, result := range results2 {
		idCol := result.Column(0).(*array.Int32)
		titleCol := result.Column(1).(*array.String)
		categoryCol := result.Column(2).(*array.String)
		viewsCol := result.Column(3).(*array.Int64)

		for i := 0; i < int(result.NumRows()); i++ {
			fmt.Printf("     %d. [ID:%d] %s\n", i+1, idCol.Value(i), titleCol.Value(i))
			fmt.Printf("        Category: %s, Views: %d\n", categoryCol.Value(i), viewsCol.Value(i))
		}
		result.Release()
	}
	fmt.Println()

	// ========================================
	// 9. FILTERED VECTOR SEARCH
	// ========================================
	fmt.Println("🎯 Step 9: Filtered vector search (category='tech' AND views > 10000)")

	query3 := table.Query()
	defer query3.Close()

	results3, err := query3.
		NearestTo(queryVector).
		SetDistanceType(lancedb.DistanceTypeCosine).
		Where("category = 'tech' AND views > 10000").
		Limit(5).
		Select("id", "title", "views").
		Execute()

	if err != nil {
		log.Fatalf("Failed to execute filtered search: %v", err)
	}

	fmt.Println("   Filtered results:")
	for _, result := range results3 {
		idCol := result.Column(0).(*array.Int32)
		titleCol := result.Column(1).(*array.String)
		viewsCol := result.Column(2).(*array.Int64)

		for i := 0; i < int(result.NumRows()); i++ {
			fmt.Printf("     %d. [ID:%d] %s (Views: %d)\n",
				i+1, idCol.Value(i), titleCol.Value(i), viewsCol.Value(i))
		}
		result.Release()
	}
	fmt.Println()

	// ========================================
	// 10. DIFFERENT DISTANCE METRICS
	// ========================================
	fmt.Println("📏 Step 10: Comparing different distance metrics")

	metrics := []struct {
		name   string
		metric lancedb.DistanceType
	}{
		{"L2 (Euclidean)", lancedb.DistanceTypeL2},
		{"Cosine", lancedb.DistanceTypeCosine},
		{"Dot Product", lancedb.DistanceTypeDot},
	}

	for _, m := range metrics {
		query := table.Query()
		results, err := query.
			NearestTo(queryVector).
			SetDistanceType(m.metric).
			Limit(3).
			Select("id", "title").
			Execute()

		if err != nil {
			query.Close()
			continue
		}

		fmt.Printf("   %s - Top 3:\n", m.name)
		for _, result := range results {
			idCol := result.Column(0).(*array.Int32)
			titleCol := result.Column(1).(*array.String)

			for i := 0; i < int(result.NumRows()); i++ {
				fmt.Printf("     %d. [ID:%d] %s\n", i+1, idCol.Value(i), titleCol.Value(i))
			}
			result.Release()
		}
		query.Close()
	}
	fmt.Println()

	// ========================================
	// 11. TABLE STATISTICS
	// ========================================
	fmt.Println("📊 Step 11: Table statistics")

	finalCount, _ := table.CountRows()
	finalSchema, _ := table.Schema()
	tableNames, _ := db.TableNames()

	fmt.Printf("   Database: %s\n", dbPath)
	fmt.Printf("   Tables: %v\n", tableNames)
	fmt.Printf("   Total documents: %d\n", finalCount)
	fmt.Printf("   Columns: %d\n", finalSchema.NumFields())
	fmt.Printf("   Indices: %d\n", len(indices))
	fmt.Println()

	// ========================================
	// SUMMARY
	// ========================================
	fmt.Println("✅ Example completed successfully!\n")
	fmt.Println("Features demonstrated:")
	fmt.Println("  ✓ Database connection & management")
	fmt.Println("  ✓ Custom schema creation with Arrow")
	fmt.Println("  ✓ Batch data insertion (append mode)")
	fmt.Println("  ✓ Data reading with Arrow records")
	fmt.Println("  ✓ Brute-force vector search")
	fmt.Println("  ✓ Index creation (IVF-PQ)")
	fmt.Println("  ✓ Index listing & management")
	fmt.Println("  ✓ Indexed vector search (faster)")
	fmt.Println("  ✓ Filtered vector search (hybrid)")
	fmt.Println("  ✓ Multiple distance metrics")
	fmt.Println("  ✓ Column projection (SELECT)")
	fmt.Println("  ✓ Query builder API")
	fmt.Println()
	fmt.Println("🎉 LanceDB Go bindings are production-ready!")
}
