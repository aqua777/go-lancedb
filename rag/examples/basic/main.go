package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"os"
	"time"

	"github.com/aqua777/go-lancedb/rag"
)

func main() {
	// Create a temporary database directory for this example
	dbPath := "/tmp/rag_example_db"

	// Clean up any existing database
	_ = os.RemoveAll(dbPath)
	defer os.RemoveAll(dbPath)

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Initialize the RAG store
	// Using 128-dimensional embeddings for this example
	fmt.Println("=== Creating RAG Store ===")
	embeddingDim := 128
	store, err := rag.NewRAGStore(dbPath, embeddingDim)
	if err != nil {
		log.Fatalf("Failed to create RAG store: %v", err)
	}
	defer store.Close()
	fmt.Println("✓ RAG store created successfully")

	userID := "demo_user"

	// Example 1: Add documents with embeddings
	fmt.Println("\n=== Adding Documents ===")
	docs := createSampleDocuments(embeddingDim)
	err = store.AddDocuments(ctx, userID, docs)
	if err != nil {
		log.Fatalf("Failed to add documents: %v", err)
	}
	fmt.Printf("✓ Added %d documents\n", len(docs))

	// Example 2: List all document names
	fmt.Println("\n=== Listing Documents ===")
	docNames, err := store.ListDocumentNames(ctx, userID)
	if err != nil {
		log.Fatalf("Failed to list documents: %v", err)
	}
	fmt.Printf("✓ Found %d unique documents:\n", len(docNames))
	for _, name := range docNames {
		fmt.Printf("  - %s\n", name)
	}

	// Example 3: Search for similar documents
	fmt.Println("\n=== Searching Documents ===")
	queryEmbedding := generateRandomEmbedding(embeddingDim)
	results, err := store.Search(ctx, userID, queryEmbedding, &rag.SearchOptions{
		Limit: 3,
	})
	if err != nil {
		log.Fatalf("Failed to search: %v", err)
	}
	fmt.Printf("✓ Found %d results:\n", len(results))
	for i, result := range results {
		fmt.Printf("  %d. %s (score: %.4f)\n", i+1, result.DocumentName, result.Score)
		fmt.Printf("     %s\n", truncate(result.Text, 80))
	}

	// Example 4: Search with filters (filter by document name)
	fmt.Println("\n=== Searching with Filters ===")
	results, err = store.Search(ctx, userID, queryEmbedding, &rag.SearchOptions{
		Limit: 2,
		Filters: map[string]interface{}{
			"document_name": "lancedb_overview.txt",
		},
	})
	if err != nil {
		log.Fatalf("Failed to search with filters: %v", err)
	}
	fmt.Printf("✓ Found %d results for lancedb_overview.txt:\n", len(results))
	for i, result := range results {
		fmt.Printf("  %d. %s (score: %.4f)\n", i+1, result.DocumentName, result.Score)
	}

	// Example 5: Document chunking
	fmt.Println("\n=== Document Chunking ===")
	demonstrateChunking()

	// Example 6: Update a document
	fmt.Println("\n=== Updating a Document ===")
	updatedDoc := rag.Document{
		ID:           "doc0_chunk0",
		Text:         "This is UPDATED content about LanceDB",
		DocumentName: "lancedb_overview.txt",
		Embedding:    generateRandomEmbedding(embeddingDim),
		Metadata: map[string]interface{}{
			"category": "updated",
			"version":  2,
		},
	}
	err = store.UpdateDocument(ctx, userID, updatedDoc)
	if err != nil {
		log.Fatalf("Failed to update document: %v", err)
	}
	fmt.Println("✓ Document updated successfully")

	// Example 7: Delete documents by name
	fmt.Println("\n=== Deleting Documents ===")
	err = store.DeleteByDocumentName(ctx, userID, "embeddings_guide.txt")
	if err != nil {
		log.Fatalf("Failed to delete document: %v", err)
	}
	fmt.Println("✓ All chunks of embeddings_guide.txt deleted successfully")

	// Verify deletion
	docNames, _ = store.ListDocumentNames(ctx, userID)
	fmt.Printf("✓ Remaining documents: %d\n", len(docNames))

	// Example 8: Paginated listing for large datasets
	fmt.Println("\n=== Paginated Listing ===")
	page, err := store.ListDocumentNamesPaginated(ctx, userID, 0, 2)
	if err != nil {
		log.Fatalf("Failed to paginate: %v", err)
	}
	fmt.Printf("✓ Page 1 (showing %d of %d total):\n", len(page.Names), page.TotalCount)
	for _, name := range page.Names {
		fmt.Printf("  - %s\n", name)
	}

	// Example 9: Get document count
	fmt.Println("\n=== Document Count ===")
	count, err := store.CountDocuments(ctx, userID)
	if err != nil {
		log.Fatalf("Failed to get document count: %v", err)
	}
	fmt.Printf("✓ Total document chunks in store: %d\n", count)

	// Example 10: Health check
	fmt.Println("\n=== Health Check ===")
	err = store.HealthCheck(ctx)
	if err != nil {
		log.Fatalf("Health check failed: %v", err)
	}
	fmt.Println("✓ Store is healthy")

	// Get detailed health status
	status := store.HealthCheckWithDetails(ctx)
	fmt.Printf("  - Tables: %d\n", status.TablesCount)
	if len(status.UserTableCount) > 0 {
		fmt.Printf("  - Sample user table document count: %v\n", status.UserTableCount)
	}

	fmt.Println("\n=== All Examples Complete ===")
}

// createSampleDocuments generates sample documents with random embeddings
// Note: We generate 260 documents to meet the minimum requirement (256) for IVF_PQ indexing
func createSampleDocuments(dim int) []rag.Document {
	baseTexts := []struct {
		text     string
		category string
		docName  string
	}{
		{
			"LanceDB is a vector database built on Lance, an open-source columnar data format. It provides fast similarity search and filtering capabilities.",
			"technical",
			"lancedb_overview.txt",
		},
		{
			"The RAG (Retrieval-Augmented Generation) pattern combines information retrieval with language models to provide accurate, grounded responses.",
			"technical",
			"rag_patterns.txt",
		},
		{
			"Go is a statically typed, compiled programming language designed at Google. It is syntactically similar to C, but with memory safety and garbage collection.",
			"technical",
			"golang_intro.txt",
		},
		{
			"Vector embeddings represent text as numerical vectors in high-dimensional space, enabling semantic similarity comparisons.",
			"concepts",
			"embeddings_guide.txt",
		},
	}

	docs := make([]rag.Document, 0, 260)

	// Generate 260 documents (65 of each base text) to meet index training requirements
	for i := 0; i < 65; i++ {
		for j, base := range baseTexts {
			docID := fmt.Sprintf("doc%d_chunk%d", j, i)
			docs = append(docs, rag.Document{
				ID:           docID,
				Text:         fmt.Sprintf("[%d] %s", i, base.text),
				DocumentName: base.docName,
				Embedding:    generateRandomEmbedding(dim),
				Metadata: map[string]interface{}{
					"category": base.category,
					"chunk_id": i,
				},
			})
		}
	}

	return docs
}

// generateRandomEmbedding creates a random normalized embedding vector
// In a real application, you would use an embedding model (OpenAI, etc.)
func generateRandomEmbedding(dim int) []float32 {
	embedding := make([]float32, dim)
	var sum float32

	// Generate random values
	for i := 0; i < dim; i++ {
		embedding[i] = rand.Float32()
		sum += embedding[i] * embedding[i]
	}

	// Normalize the vector
	norm := float32(1.0) / float32(1e-8+sum)
	for i := 0; i < dim; i++ {
		embedding[i] *= norm
	}

	return embedding
}

// demonstrateChunking shows how to use different chunking strategies
func demonstrateChunking() {
	longText := `RAG systems combine the power of large language models with external knowledge retrieval. 
	
The basic workflow involves three steps: First, documents are split into chunks and converted to embeddings. 
These embeddings are stored in a vector database like LanceDB.

Second, when a user asks a question, it is converted to an embedding and used to search for similar document chunks.

Finally, the retrieved chunks are provided as context to the language model, which generates a response grounded in the retrieved information.`

	// Fixed-size chunking with overlap
	chunker, _ := rag.NewFixedSizeChunker(100, 20)
	chunks, _ := chunker.Chunk(longText)
	fmt.Printf("✓ Fixed-size chunker (100 chars, 20 overlap): %d chunks\n", len(chunks))

	// Sentence-based chunking
	sentenceChunker, _ := rag.NewSentenceChunker(2)
	sentenceChunks, _ := sentenceChunker.Chunk(longText)
	fmt.Printf("✓ Sentence chunker (2 sentences/chunk): %d chunks\n", len(sentenceChunks))

	// Token-aware chunking (4 chars ≈ 1 token)
	tokenChunker, _ := rag.NewTokenChunker(50, 10)
	tokenChunks, _ := tokenChunker.Chunk(longText)
	fmt.Printf("✓ Token chunker (~50 tokens/chunk): %d chunks\n", len(tokenChunks))
}

// truncate shortens a string to maxLen characters
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
