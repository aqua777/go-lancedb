package rag

import (
	"container/list"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"sync"
)

// EmbeddingCache is an interface for caching query embeddings.
// Implementations should be thread-safe.
type EmbeddingCache interface {
	// Get retrieves a cached embedding for the given query text
	Get(query string) ([]float32, bool)
	
	// Set stores an embedding for the given query text
	Set(query string, embedding []float32)
	
	// Clear removes all cached entries
	Clear()
	
	// Size returns the current number of cached entries
	Size() int
}

// LRUEmbeddingCache implements a thread-safe LRU (Least Recently Used) cache for embeddings.
// When the cache is full, the least recently used entry is evicted.
type LRUEmbeddingCache struct {
	mu       sync.RWMutex
	capacity int
	cache    map[string]*list.Element
	lru      *list.List
}

// cacheEntry represents a cached embedding with its query key
type cacheEntry struct {
	key       string
	embedding []float32
}

// NewLRUEmbeddingCache creates a new LRU cache with the specified capacity.
// capacity determines the maximum number of embeddings to cache.
// Typical values: 100-1000 for development, 1000-10000 for production.
func NewLRUEmbeddingCache(capacity int) *LRUEmbeddingCache {
	if capacity <= 0 {
		capacity = 1000 // default capacity
	}
	
	return &LRUEmbeddingCache{
		capacity: capacity,
		cache:    make(map[string]*list.Element),
		lru:      list.New(),
	}
}

// Get retrieves a cached embedding for the given query text.
// Returns the embedding and true if found, nil and false otherwise.
// Accessing an entry marks it as recently used.
func (c *LRUEmbeddingCache) Get(query string) ([]float32, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	key := hashQuery(query)
	
	if elem, found := c.cache[key]; found {
		// Move to front (most recently used)
		c.lru.MoveToFront(elem)
		entry := elem.Value.(*cacheEntry)
		return entry.embedding, true
	}
	
	return nil, false
}

// Set stores an embedding for the given query text.
// If the cache is full, the least recently used entry is evicted.
func (c *LRUEmbeddingCache) Set(query string, embedding []float32) {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	key := hashQuery(query)
	
	// Check if already exists
	if elem, found := c.cache[key]; found {
		// Update existing entry and move to front
		c.lru.MoveToFront(elem)
		elem.Value.(*cacheEntry).embedding = embedding
		return
	}
	
	// Add new entry
	entry := &cacheEntry{
		key:       key,
		embedding: embedding,
	}
	elem := c.lru.PushFront(entry)
	c.cache[key] = elem
	
	// Evict oldest if over capacity
	if c.lru.Len() > c.capacity {
		c.evictOldest()
	}
}

// Clear removes all cached entries
func (c *LRUEmbeddingCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	c.cache = make(map[string]*list.Element)
	c.lru = list.New()
}

// Size returns the current number of cached entries
func (c *LRUEmbeddingCache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	return c.lru.Len()
}

// evictOldest removes the least recently used entry (must be called with lock held)
func (c *LRUEmbeddingCache) evictOldest() {
	elem := c.lru.Back()
	if elem != nil {
		c.lru.Remove(elem)
		entry := elem.Value.(*cacheEntry)
		delete(c.cache, entry.key)
	}
}

// hashQuery creates a consistent hash key for a query string.
// Using SHA256 to avoid collision issues with map keys.
func hashQuery(query string) string {
	hash := sha256.Sum256([]byte(query))
	return hex.EncodeToString(hash[:])
}

// CachedEmbeddingProvider wraps an embedding provider with caching.
// Identical queries will return cached embeddings instead of calling the provider.
type CachedEmbeddingProvider struct {
	provider EmbeddingProvider
	cache    EmbeddingCache
	metrics  MetricsCollector // optional metrics collector for cache hit/miss tracking
}

// NewCachedEmbeddingProvider creates a cached wrapper around an embedding provider.
// cache is the cache implementation to use (typically LRUEmbeddingCache).
// metrics is optional; pass nil to disable cache metrics.
func NewCachedEmbeddingProvider(provider EmbeddingProvider, cache EmbeddingCache, metrics MetricsCollector) *CachedEmbeddingProvider {
	if metrics == nil {
		metrics = &noopMetrics{}
	}
	
	return &CachedEmbeddingProvider{
		provider: provider,
		cache:    cache,
		metrics:  metrics,
	}
}

// Dimensions returns the embedding dimensionality from the wrapped provider
func (p *CachedEmbeddingProvider) Dimensions() int {
	return p.provider.Dimensions()
}

// GenerateEmbedding generates a single embedding with caching.
// Returns cached result if available, otherwise calls the provider and caches the result.
func (p *CachedEmbeddingProvider) GenerateEmbedding(ctx context.Context, text string) ([]float32, error) {
	// Check cache first
	if embedding, found := p.cache.Get(text); found {
		p.metrics.RecordOperation("embedding_cache_hit", 0, true)
		return embedding, nil
	}
	
	p.metrics.RecordOperation("embedding_cache_miss", 0, true)
	
	// Generate embedding
	embedding, err := p.provider.GenerateEmbedding(ctx, text)
	if err != nil {
		return nil, err
	}
	
	// Cache the result
	p.cache.Set(text, embedding)
	
	return embedding, nil
}

// GenerateEmbeddings generates multiple embeddings.
// Note: Batch operations are not cached individually; use GenerateEmbedding for cacheable queries.
func (p *CachedEmbeddingProvider) GenerateEmbeddings(ctx context.Context, texts []string) ([][]float32, error) {
	// For batch operations, check each text individually
	embeddings := make([][]float32, len(texts))
	uncachedIndices := make([]int, 0)
	uncachedTexts := make([]string, 0)
	
	for i, text := range texts {
		if embedding, found := p.cache.Get(text); found {
			embeddings[i] = embedding
			p.metrics.RecordOperation("embedding_cache_hit", 0, true)
		} else {
			uncachedIndices = append(uncachedIndices, i)
			uncachedTexts = append(uncachedTexts, text)
			p.metrics.RecordOperation("embedding_cache_miss", 0, true)
		}
	}
	
	// If all were cached, return immediately
	if len(uncachedTexts) == 0 {
		return embeddings, nil
	}
	
	// Generate embeddings for uncached texts
	newEmbeddings, err := p.provider.GenerateEmbeddings(ctx, uncachedTexts)
	if err != nil {
		return nil, err
	}
	
	// Fill in the results and cache them
	for i, embedding := range newEmbeddings {
		idx := uncachedIndices[i]
		embeddings[idx] = embedding
		p.cache.Set(uncachedTexts[i], embedding)
	}
	
	return embeddings, nil
}

// ClearCache clears all cached embeddings
func (p *CachedEmbeddingProvider) ClearCache() {
	p.cache.Clear()
}

// CacheSize returns the current number of cached embeddings
func (p *CachedEmbeddingProvider) CacheSize() int {
	return p.cache.Size()
}

