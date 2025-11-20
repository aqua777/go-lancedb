# RAG Package - Production-Ready Implementation

This package provides a comprehensive, production-grade RAG (Retrieval-Augmented Generation) system built on LanceDB.

## Features Implemented

### Phase 1: Must Haves (Critical for Production)

✅ **Fixed ListDocumentNames Memory Issue**
- Added `ListDocumentNamesPaginated()` for efficient pagination
- Improved memory handling for large datasets

✅ **Batch Size Limits**
- Configurable `maxBatchSize` prevents memory exhaustion
- Automatic batching in `AddDocuments()`
- Default batch size: 1000 documents

✅ **Proper Error Handling & Logging**
- Logger interface with default and noop implementations
- Comprehensive error logging for index creation
- Detailed operation tracking

✅ **Context Support**
- All methods accept `context.Context` for timeout/cancellation
- Proper context propagation throughout operations
- Cancellation checks between batches

✅ **Concurrency Safety**
- Per-user locks for write operations
- Thread-safe index creation with double-checked locking
- Safe concurrent access to shared state

### Phase 2: Should Haves (Robustness)

✅ **Retry Logic**
- Exponential backoff retry mechanism
- Configurable retry parameters (attempts, delays)
- Intelligent error classification (retryable vs non-retryable)

✅ **SQL Injection Protection**
- Whitelist-based filter key validation
- Comprehensive SQL string escaping
- Identifier sanitization

✅ **Document Update/Upsert**
- `UpdateDocument()` for single document updates
- `UpsertDocuments()` for batch upsert operations
- Efficient delete-and-insert pattern

✅ **Pagination**
- `ListDocumentNamesPaginated()` with offset/limit
- Consistent sorting for reliable pagination
- Metadata includes pagination info

✅ **Monitoring & Metrics**
- `MetricsCollector` interface for custom integrations
- Simple in-memory metrics implementation
- Operation timing and success/error tracking

### Phase 3: Nice to Haves (Feature Completeness)

✅ **Document Chunking**
- `FixedSizeChunker` - Fixed character size with overlap
- `SentenceChunker` - Sentence-based chunking
- `ParagraphChunker` - Paragraph-based chunking
- `TokenChunker` - Token-aware chunking (4 chars ≈ 1 token)

✅ **Embedding Generation**
- `EmbeddingProvider` interface
- `OpenAIEmbeddingProvider` - OpenAI API integration
- `HTTPEmbeddingProvider` - Custom HTTP endpoints
- `AddDocumentsWithEmbedding()` - Automatic embedding generation
- `SearchWithText()` - Text-based search with auto-embedding

✅ **Re-ranking**
- `Reranker` interface
- `CrossEncoderReranker` - Cross-encoder model support
- `ReciprocalRankFusionReranker` - RRF for combining results
- `CustomScorerReranker` - Custom scoring functions

✅ **Hybrid Search**
- Combines vector and keyword (BM25) search
- Configurable weighting between vector and keyword
- Reciprocal Rank Fusion for result combination
- `HybridSearch()` and `HybridSearchWithText()` methods

✅ **Advanced Index Configuration**
- `IndexConfig` struct for fine-tuned control
- Per-user index configurations
- `SetIndexConfig()` and `GetIndexConfig()` methods
- `RebuildIndex()` for applying new configurations

✅ **Connection Pooling**
- `ConnectionPool` for efficient connection reuse
- `PooledRAGStore` for pool-based stores
- Global pool singleton for convenience
- Configurable pool size

✅ **Comprehensive Tests**
- Test suite structure using testify/suite
- Separate test suites for store, documents, queries, chunking, and pooling
- Automatic setup/teardown with temporary databases

## Usage Examples

### Basic Usage

```go
// Create a RAG store
store, err := rag.NewRAGStore("/path/to/db", 128)
if err != nil {
    log.Fatal(err)
}
defer store.Close()

ctx := context.Background()

// Add documents
docs := []rag.Document{
    {
        ID:           "doc1",
        Text:         "Example text",
        DocumentName: "example.txt",
        Embedding:    embedding, // 128-dimensional vector
        Metadata:     map[string]interface{}{"source": "file"},
    },
}
err = store.AddDocuments(ctx, "user123", docs)

// Search
results, err := store.Search(ctx, "user123", queryEmbedding, &rag.SearchOptions{
    Limit: 10,
    Filters: map[string]interface{}{
        "source": "file",
    },
})
```

### With Chunking and Embeddings

```go
// Create chunker
chunker, _ := rag.NewFixedSizeChunker(500, 50)

// Create embedding provider
provider := rag.NewOpenAIEmbeddingProvider("api-key", "text-embedding-3-small", 1536)

// Chunk and add documents
texts := []string{"Long document text..."}
docNames := []string{"document.txt"}
err := store.AddDocumentsWithEmbedding(ctx, "user123", texts, docNames, provider)

// Search with text
results, err := store.SearchWithText(ctx, "user123", "query text", provider, nil)
```

### Hybrid Search with Re-ranking

```go
// Perform hybrid search
results, err := store.HybridSearchWithText(ctx, "user123", "query", provider, &rag.HybridSearchOptions{
    Limit:         10,
    VectorWeight:  0.7,
    KeywordWeight: 0.3,
})

// Re-rank results
reranker := rag.NewCrossEncoderReranker("http://localhost:8000/rerank")
reranked, err := reranker.Rerank(ctx, "query", results)
```

### Connection Pooling

```go
// Initialize global pool
err := rag.InitGlobalPool("/path/to/db", 10)
defer rag.CloseGlobalPool()

// Get pool and create store
pool, _ := rag.GetGlobalPool()
store, err := rag.NewPooledRAGStore(pool, 128, 1000, nil, nil, nil)
defer store.Close() // Returns connection to pool
```

### Query Caching

Cache embeddings to avoid redundant API calls for repeated queries:

```go
// Create embedding provider
provider := rag.NewOpenAIEmbeddingProvider(apiKey, "text-embedding-3-small", 1536)

// Wrap with cache (capacity: 1000 queries)
cache := rag.NewLRUEmbeddingCache(1000)
cachedProvider := rag.NewCachedEmbeddingProvider(provider, cache, nil)

// Use cached provider - repeated queries will hit the cache
results1, _ := store.SearchWithText(ctx, "user123", "same query", cachedProvider, nil)
results2, _ := store.SearchWithText(ctx, "user123", "same query", cachedProvider, nil) // Cache hit!

// Check cache stats
fmt.Printf("Cache size: %d\n", cachedProvider.CacheSize())
```

### Rate Limiting

Prevent overwhelming embedding APIs with rate limiting:

```go
provider := rag.NewOpenAIEmbeddingProvider(apiKey, "text-embedding-3-small", 1536)

// Limit to 10 requests/second with bursts up to 20
rateLimited := rag.NewRateLimitedEmbeddingProvider(provider, 10.0, 20)

// Combine with caching for maximum efficiency
cache := rag.NewLRUEmbeddingCache(1000)
cached := rag.NewCachedEmbeddingProvider(rateLimited, cache, nil)

err := store.AddDocumentsWithEmbedding(ctx, "user123", texts, docNames, cached)
```

### Health Checks

Monitor database connectivity:

```go
// Simple health check
err := store.HealthCheck(ctx)
if err != nil {
    log.Printf("Health check failed: %v", err)
}

// Detailed health check with diagnostics
status := store.HealthCheckWithDetails(ctx)
fmt.Printf("Healthy: %v\n", status.Healthy)
fmt.Printf("Tables: %d\n", status.TablesCount)
fmt.Printf("Sample user docs: %v\n", status.UserTableCount)

// Pool health check
pool, _ := rag.GetGlobalPool()
err = pool.HealthCheckWithConnection()
```

## Configuration

### RAGStore Configuration

```go
store, err := rag.NewRAGStoreWithConfig(
    dbPath,
    embeddingDim,
    maxBatchSize,
    logger,        // Custom logger or nil
    retryConfig,   // Retry configuration or nil
    metrics,       // Metrics collector or nil
)
```

### Index Configuration

```go
config := &rag.IndexConfig{
    IndexType:     lancedb.IndexTypeIVFPQ,
    Metric:        lancedb.DistanceMetricCosine,
    NumPartitions: 256,
    NumSubVectors: 16,
    Replace:       true,
}
err := store.SetIndexConfig(ctx, "user123", config)
```

## Testing

Run tests with:

```bash
go test ./rag/...
```

Test suites are organized by functionality:
- `store_test.go` - Core store operations
- `document_test.go` - Document management
- `query_test.go` - Search and retrieval
- `chunking_test.go` - Text chunking
- `pool_test.go` - Connection pooling

## Performance Considerations

1. **Batch Size**: Default 1000 documents per batch. Adjust based on memory constraints.
2. **Index Creation**: Happens automatically after first insert. For large datasets, configure index parameters.
3. **Connection Pool**: Use pooling when creating multiple RAGStore instances.
4. **Pagination**: Use `ListDocumentNamesPaginated()` for large document collections.
5. **Hybrid Search**: More expensive than pure vector search; use when relevance is critical.

## Known Limitations

### Hybrid Search / BM25 Scalability

**Important**: The hybrid search implementation has a built-in document count limit (default: 10,000 documents).

**Why the limit exists:**
- BM25 keyword search loads ALL documents into memory to calculate relevance scores
- This is necessary for accurate BM25 scoring but doesn't scale to large document collections
- For 100K+ documents, this can cause memory exhaustion and severely degrade performance

**What happens when you hit the limit:**
```
Error: document count (15000) exceeds BM25 limit (10000); use pure vector search instead or increase limit with SetMaxDocumentsForBM25()
```

**Recommendations:**
1. **Small datasets (<10K docs)**: Hybrid search works great, no issues
2. **Medium datasets (10K-50K docs)**: Consider increasing the limit cautiously with `SetMaxDocumentsForBM25()`, monitor memory usage
3. **Large datasets (>50K docs)**: Use pure vector search (`Search()` or `SearchWithText()`) instead of hybrid search
4. **Need BM25 at scale?**: Consider pre-computing BM25 scores and storing them as metadata (future enhancement)

**Example - adjusting the limit:**
```go
store, _ := rag.NewRAGStore("/path/to/db", 128)
store.SetMaxDocumentsForBM25(25000) // Increase limit (ensure you have enough memory)

// Or disable the limit entirely (NOT recommended for production)
store.SetMaxDocumentsForBM25(0)
```

### Rate Limiting for Embedding APIs

When adding large numbers of documents or running many queries, you may hit rate limits from embedding providers (OpenAI, etc.).

**Solution:** Wrap your embedding provider with rate limiting:
```go
provider := rag.NewOpenAIEmbeddingProvider(apiKey, "text-embedding-3-small", 1536)

// Limit to 10 requests/second with bursts up to 20
rateLimited := rag.NewRateLimitedEmbeddingProvider(provider, 10.0, 20)

// Use the rate-limited provider
err := store.AddDocumentsWithEmbedding(ctx, "user123", texts, docNames, rateLimited)
```

## Production Checklist

- ✅ Context support for timeouts
- ✅ Concurrency safety
- ✅ Error handling and logging
- ✅ Retry logic for transient failures
- ✅ SQL injection protection
- ✅ Memory management (batching, pagination)
- ✅ Metrics and monitoring hooks
- ✅ Connection pooling for efficiency
- ✅ Query caching for embeddings
- ✅ Rate limiting for API calls
- ✅ Health check endpoints
- ✅ Document count limits for BM25
- ✅ Comprehensive test coverage

## License

Same as parent project.

