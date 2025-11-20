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

## Desktop Application Best Practices

The RAG package is well-suited for desktop macOS applications. Here are recommended patterns for desktop use.

### Basic Desktop Setup

```go
package main

import (
    "context"
    "fmt"
    "os"
    "os/signal"
    "path/filepath"
    "syscall"
    
    "github.com/aqua777/go-lancedb/rag"
)

func main() {
    // Get application data directory
    homeDir, _ := os.UserHomeDir()
    appDataDir := filepath.Join(homeDir, "Library", "Application Support", "MyApp")
    os.MkdirAll(appDataDir, 0755)
    
    // Initialize file-based logging
    logger, err := rag.NewDefaultDesktopLogger("MyApp")
    if err != nil {
        fmt.Fprintf(os.Stderr, "Failed to create logger: %v\n", err)
        os.Exit(1)
    }
    defer logger.Close()
    
    // Create RAG store
    dbPath := filepath.Join(appDataDir, "vectordb")
    store, err := rag.NewRAGStoreWithConfig(
        dbPath,
        1536,  // embedding dimension
        1000,  // batch size
        logger,
        rag.DefaultRetryConfig(),
        nil,   // metrics (optional)
    )
    if err != nil {
        logger.Error("Failed to create RAG store: %v", err)
        os.Exit(1)
    }
    defer store.Close()
    
    // Validate database on startup
    ctx := context.Background()
    validation, err := store.ValidateDatabase(ctx, "default-user")
    if err != nil {
        logger.Warn("Database validation failed: %v", err)
    } else if !validation.Valid {
        logger.Warn("Database has issues: %v", validation.Issues)
        // Optionally attempt repair
        if err := store.RepairDatabase(ctx, "default-user"); err != nil {
            logger.Error("Failed to repair database: %v", err)
        }
    }
    
    // Setup graceful shutdown
    setupGracefulShutdown(store, logger)
    
    // Run your application...
    runApp(store, logger)
}

func setupGracefulShutdown(store *rag.RAGStore, logger *rag.FileLogger) {
    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
    
    go func() {
        <-sigChan
        logger.Info("Shutting down gracefully...")
        
        // Close with timeout
        ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
        defer cancel()
        
        if err := store.CloseWithContext(ctx); err != nil {
            logger.Error("Error during shutdown: %v", err)
        }
        
        logger.Close()
        os.Exit(0)
    }()
}

func runApp(store *rag.RAGStore, logger *rag.FileLogger) {
    // Your application logic here
}
```

### Progress Reporting for UI

Show progress to users during long operations:

```go
// Progress callback that updates your UI
progressCallback := func(progress *rag.Progress) {
    // Update progress bar
    percentage := progress.Percent()
    
    // Update status message
    message := fmt.Sprintf("%s: %d/%d (%.1f%%) - %s",
        progress.Stage,
        progress.Current,
        progress.Total,
        percentage,
        progress.Message,
    )
    
    // Estimate remaining time
    if remaining := progress.EstimatedRemaining(); remaining > 0 {
        message += fmt.Sprintf(" - %s remaining", remaining.Round(time.Second))
    }
    
    // Update your UI (NSProgressIndicator, SwiftUI ProgressView, etc.)
    updateUI(percentage, message)
}

// Adding documents with progress
texts := []string{"document 1", "document 2", /* ... */}
docNames := []string{"doc1.txt", "doc2.txt", /* ... */}

err := store.AddDocumentsWithEmbeddingProgress(
    ctx,
    "user123",
    texts,
    docNames,
    embeddingProvider,
    progressCallback,
)
```

### Data Backup and Export

Protect user data with regular backups:

```go
// Export user data to backup file
backupDir := filepath.Join(appDataDir, "backups")
os.MkdirAll(backupDir, 0755)

backupPath := filepath.Join(backupDir, fmt.Sprintf("backup_%s.json.gz", time.Now().Format("20060102_150405")))

// Export with progress
progressCallback := func(p *rag.Progress) {
    fmt.Printf("Backup progress: %.1f%%\n", p.Percent())
}

err := store.ExportUserDataWithProgress(
    ctx,
    "user123",
    backupPath,
    rag.BackupFormatJSONGzip, // Compressed format
    progressCallback,
)
if err != nil {
    logger.Error("Backup failed: %v", err)
} else {
    logger.Info("Backup saved to %s", backupPath)
}

// Restore from backup
err = store.ImportUserDataWithProgress(
    ctx,
    "user123",
    backupPath,
    true, // Clear existing data first
    progressCallback,
)
```

### Embedding Provider Setup

Configure embedding provider with rate limiting and caching:

```go
// Get API key from keychain or environment
apiKey := os.Getenv("OPENAI_API_KEY")

// Create base provider
baseProvider := rag.NewOpenAIEmbeddingProvider(
    apiKey,
    "text-embedding-3-small",
    1536,
)

// Add rate limiting (10 requests/sec, burst of 20)
rateLimited := rag.NewRateLimitedEmbeddingProvider(baseProvider, 10.0, 20)

// Add caching to save API costs
cache := rag.NewLRUEmbeddingCache(1000) // Cache 1000 queries
cachedProvider := rag.NewCachedEmbeddingProvider(rateLimited, cache, nil)

// Use the cached provider for all operations
results, err := store.SearchWithText(ctx, "user123", "search query", cachedProvider, nil)
```

### Error Handling

Handle errors gracefully for desktop users:

```go
func handleRAGError(err error, logger *rag.FileLogger) {
    if err == nil {
        return
    }
    
    logger.Error("RAG operation failed: %v", err)
    
    // Check for specific error types
    switch {
    case strings.Contains(err.Error(), "dimension mismatch"):
        showUserAlert("Error", "Invalid embedding dimensions. Please check your model configuration.")
        
    case strings.Contains(err.Error(), "rate limit"):
        showUserAlert("Warning", "API rate limit reached. Please wait a moment and try again.")
        
    case strings.Contains(err.Error(), "context canceled"):
        showUserAlert("Info", "Operation was cancelled.")
        
    case strings.Contains(err.Error(), "failed to open"):
        showUserAlert("Error", "Could not access the database. Please check file permissions.")
        
    default:
        showUserAlert("Error", fmt.Sprintf("An error occurred: %v", err))
    }
}

func showUserAlert(title, message string) {
    // Show native alert dialog (NSAlert on macOS, etc.)
}
```

### Offline Mode Handling

Detect and handle offline mode gracefully:

```go
func processDocumentsWithFallback(store *rag.RAGStore, texts []string, provider rag.EmbeddingProvider) error {
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()
    
    err := store.AddDocumentsWithEmbedding(ctx, "user123", texts, docNames, provider)
    
    // Check for network errors
    if err != nil && isNetworkError(err) {
        logger.Warn("Network error detected, queuing for later processing")
        
        // Queue documents for processing when online
        queueForLaterProcessing(texts)
        
        showUserAlert("Offline", "Documents will be processed when internet connection is restored.")
        return nil
    }
    
    return err
}

func isNetworkError(err error) bool {
    return strings.Contains(err.Error(), "network") ||
           strings.Contains(err.Error(), "connection") ||
           strings.Contains(err.Error(), "timeout")
}
```

### Performance Tips for Desktop

1. **Increase BM25 limit for desktop** (Macs typically have more RAM):
```go
store.SetMaxDocumentsForBM25(25000) // Up from default 10,000
```

2. **Use appropriate batch sizes** for your use case:
```go
// For large imports, use larger batches
store, _ := rag.NewRAGStoreWithConfig(dbPath, embeddingDim, 5000, logger, nil, nil)
```

3. **Enable debug logging during development**:
```go
logger.SetMinLevel(rag.LogLevelDebug)
```

4. **Monitor cache effectiveness**:
```go
// Check cache hit rate periodically
cachedProvider := rag.NewCachedEmbeddingProvider(provider, cache, nil)

// After some operations
cacheSize := cachedProvider.CacheSize()
logger.Info("Embedding cache contains %d entries", cacheSize)
```

### Troubleshooting

#### Database Corruption

If the database becomes corrupted:

```go
validation, err := store.ValidateDatabase(ctx, "user123")
if err != nil || !validation.Valid {
    // Try automatic repair
    if err := store.RepairDatabase(ctx, "user123"); err != nil {
        logger.Error("Auto-repair failed: %v", err)
        
        // Last resort: restore from backup
        if backupPath := findLatestBackup(); backupPath != "" {
            store.ImportUserData(ctx, "user123", backupPath, true)
        }
    }
}
```

#### High Memory Usage

If memory usage is high with hybrid search:

```go
// Check document count
count, _ := store.CountDocuments(ctx, "user123")
if count > 25000 {
    logger.Warn("High document count (%d), consider using vector-only search", count)
    
    // Use regular search instead of hybrid
    results, err := store.SearchWithText(ctx, "user123", query, provider, nil)
}
```

#### Log File Location

Logs are stored in standard macOS locations:
- macOS: `~/Library/Logs/YourApp/rag.log`
- Linux: `~/.local/share/YourApp/logs/rag.log`

Access with:
```go
logger, _ := rag.NewDefaultDesktopLogger("MyApp")
fmt.Printf("Logs: %s\n", logger.GetPath())
```

## License

Same as parent project.

