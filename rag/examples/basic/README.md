# RAG Package - Basic Example

This example demonstrates the core functionality of the `rag` package for building production-ready RAG (Retrieval-Augmented Generation) systems with LanceDB.

## What This Example Shows

1. **Store Creation** - Initialize a RAG store with vector embeddings
2. **Document Management**
   - Adding documents with metadata
   - Listing document names
   - Updating documents
   - Deleting documents
3. **Vector Search** - Finding similar documents using embeddings
4. **Filtered Search** - Searching with metadata filters
5. **Document Chunking** - Three chunking strategies (fixed-size, sentence-based, token-aware)
6. **Pagination** - Efficient listing for large datasets
7. **Document Counting** - Getting total document statistics
8. **Health Checks** - Monitoring database connectivity

## Running the Example

```bash
go run main.go
```

## Key Features Demonstrated

### Automatic Indexing
The example generates 260 documents to meet the minimum requirement (256) for IVF_PQ vector index training. In production, indices are created automatically after the first batch of documents is added.

### Real-World Patterns
- Context handling with timeouts
- Proper error handling
- Resource cleanup with defer
- Structured logging

## Note on Embeddings

This example uses **random embeddings** for demonstration purposes. In a real RAG application, you would:
- Use an embedding model (OpenAI, Cohere, local models, etc.)
- Leverage the `EmbeddingProvider` interface from the rag package
- Use `AddDocumentsWithEmbedding()` and `SearchWithText()` for automatic embedding generation

See the main [RAG README](../../README.md) for examples with actual embedding providers.

## Output

The example produces clean, structured output showing:
- Document names and IDs
- Search scores (lower is better for cosine distance)
- Truncated text previews
- Metadata from documents
- Pagination information
- Health status

## Next Steps

After understanding this basic example, explore:
- **Hybrid Search** - Combining vector and keyword (BM25) search
- **Re-ranking** - Improving search results with cross-encoder models
- **Connection Pooling** - Efficient resource management for multiple stores
- **Query Caching** - Reducing API calls for repeated queries
- **Rate Limiting** - Preventing API throttling

See the [RAG Package README](../../README.md) for advanced usage patterns.

