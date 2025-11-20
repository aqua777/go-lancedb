package rag

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	
	"golang.org/x/time/rate"
)

// EmbeddingProvider is an interface for generating text embeddings
type EmbeddingProvider interface {
	// GenerateEmbedding generates an embedding for a single text
	GenerateEmbedding(ctx context.Context, text string) ([]float32, error)
	
	// GenerateEmbeddings generates embeddings for multiple texts (batch operation)
	GenerateEmbeddings(ctx context.Context, texts []string) ([][]float32, error)
	
	// Dimensions returns the dimensionality of the embeddings
	Dimensions() int
}

// OpenAIEmbeddingProvider generates embeddings using OpenAI's API
type OpenAIEmbeddingProvider struct {
	APIKey     string
	Model      string // e.g., "text-embedding-ada-002", "text-embedding-3-small"
	BaseURL    string
	dimensions int
	httpClient *http.Client
}

// NewOpenAIEmbeddingProvider creates a new OpenAI embedding provider
func NewOpenAIEmbeddingProvider(apiKey, model string, dimensions int) *OpenAIEmbeddingProvider {
	return &OpenAIEmbeddingProvider{
		APIKey:     apiKey,
		Model:      model,
		BaseURL:    "https://api.openai.com/v1",
		dimensions: dimensions,
		httpClient: &http.Client{},
	}
}

// Dimensions returns the embedding dimensionality
func (p *OpenAIEmbeddingProvider) Dimensions() int {
	return p.dimensions
}

// GenerateEmbedding generates a single embedding
func (p *OpenAIEmbeddingProvider) GenerateEmbedding(ctx context.Context, text string) ([]float32, error) {
	embeddings, err := p.GenerateEmbeddings(ctx, []string{text})
	if err != nil {
		return nil, err
	}
	if len(embeddings) == 0 {
		return nil, fmt.Errorf("no embeddings returned")
	}
	return embeddings[0], nil
}

// GenerateEmbeddings generates multiple embeddings in a batch
func (p *OpenAIEmbeddingProvider) GenerateEmbeddings(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return [][]float32{}, nil
	}

	requestBody := map[string]interface{}{
		"input": texts,
		"model": p.Model,
	}

	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", p.BaseURL+"/embeddings", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.APIKey)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("OpenAI API error (status %d): %s", resp.StatusCode, string(body))
	}

	var response struct {
		Data []struct {
			Embedding []float32 `json:"embedding"`
			Index     int       `json:"index"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(response.Data) != len(texts) {
		return nil, fmt.Errorf("expected %d embeddings, got %d", len(texts), len(response.Data))
	}

	embeddings := make([][]float32, len(texts))
	for _, item := range response.Data {
		if item.Index >= len(embeddings) {
			return nil, fmt.Errorf("invalid embedding index: %d", item.Index)
		}
		embeddings[item.Index] = item.Embedding
	}

	return embeddings, nil
}

// HTTPEmbeddingProvider calls a custom HTTP endpoint for embeddings
// Useful for local models served via HTTP (e.g., sentence-transformers, FastEmbed)
type HTTPEmbeddingProvider struct {
	URL        string
	dimensions int
	httpClient *http.Client
}

// NewHTTPEmbeddingProvider creates a provider for a custom HTTP embedding endpoint
func NewHTTPEmbeddingProvider(url string, dimensions int) *HTTPEmbeddingProvider {
	return &HTTPEmbeddingProvider{
		URL:        url,
		dimensions: dimensions,
		httpClient: &http.Client{},
	}
}

// Dimensions returns the embedding dimensionality
func (p *HTTPEmbeddingProvider) Dimensions() int {
	return p.dimensions
}

// GenerateEmbedding generates a single embedding
func (p *HTTPEmbeddingProvider) GenerateEmbedding(ctx context.Context, text string) ([]float32, error) {
	embeddings, err := p.GenerateEmbeddings(ctx, []string{text})
	if err != nil {
		return nil, err
	}
	if len(embeddings) == 0 {
		return nil, fmt.Errorf("no embeddings returned")
	}
	return embeddings[0], nil
}

// GenerateEmbeddings generates multiple embeddings
func (p *HTTPEmbeddingProvider) GenerateEmbeddings(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return [][]float32{}, nil
	}

	requestBody := map[string]interface{}{
		"texts": texts,
	}

	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", p.URL, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("HTTP embedding service error (status %d): %s", resp.StatusCode, string(body))
	}

	var response struct {
		Embeddings [][]float32 `json:"embeddings"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(response.Embeddings) != len(texts) {
		return nil, fmt.Errorf("expected %d embeddings, got %d", len(texts), len(response.Embeddings))
	}

	return response.Embeddings, nil
}

// AddDocumentsWithEmbedding adds documents to the store, generating embeddings automatically
func (s *RAGStore) AddDocumentsWithEmbedding(ctx context.Context, userID string, texts []string, documentNames []string, provider EmbeddingProvider) error {
	if len(texts) == 0 {
		return fmt.Errorf("no texts to add")
	}
	if len(documentNames) != len(texts) {
		return fmt.Errorf("number of document names (%d) must match number of texts (%d)", len(documentNames), len(texts))
	}

	// Check embedding dimensions match
	if provider.Dimensions() != s.embeddingDim {
		return fmt.Errorf("provider embedding dimension (%d) does not match store dimension (%d)",
			provider.Dimensions(), s.embeddingDim)
	}

	// Generate embeddings in batches to avoid overwhelming the provider
	batchSize := 100 // Most providers support batches of 100+
	docs := make([]Document, 0, len(texts))

	for i := 0; i < len(texts); i += batchSize {
		// Check for context cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		end := i + batchSize
		if end > len(texts) {
			end = len(texts)
		}

		batch := texts[i:end]
		embeddings, err := provider.GenerateEmbeddings(ctx, batch)
		if err != nil {
			return fmt.Errorf("failed to generate embeddings for batch [%d:%d]: %w", i, end, err)
		}

		for j, embedding := range embeddings {
			idx := i + j
			docs = append(docs, Document{
				ID:           fmt.Sprintf("%s_%d", documentNames[idx], idx),
				Text:         texts[idx],
				DocumentName: documentNames[idx],
				Embedding:    embedding,
				Metadata:     map[string]interface{}{},
			})
		}
	}

	// Add all documents
	return s.AddDocuments(ctx, userID, docs)
}

// SearchWithText performs a search using text query instead of pre-computed embedding
func (s *RAGStore) SearchWithText(ctx context.Context, userID string, queryText string, provider EmbeddingProvider, opts *SearchOptions) ([]SearchResult, error) {
	// Generate embedding for query text
	embedding, err := provider.GenerateEmbedding(ctx, queryText)
	if err != nil {
		return nil, fmt.Errorf("failed to generate query embedding: %w", err)
	}

	// Perform regular search with the embedding
	return s.Search(ctx, userID, embedding, opts)
}

// RateLimitedEmbeddingProvider wraps an embedding provider with rate limiting.
// This prevents overwhelming external APIs and helps avoid rate limit errors.
// Useful for production deployments with high request volumes.
type RateLimitedEmbeddingProvider struct {
	provider EmbeddingProvider
	limiter  *rate.Limiter
}

// NewRateLimitedEmbeddingProvider creates a rate-limited wrapper around an embedding provider.
// requestsPerSecond controls the sustained request rate.
// burstSize allows short bursts above the sustained rate.
// Example: requestsPerSecond=10, burstSize=20 allows bursts up to 20 requests/sec, settling at 10/sec.
func NewRateLimitedEmbeddingProvider(provider EmbeddingProvider, requestsPerSecond float64, burstSize int) *RateLimitedEmbeddingProvider {
	return &RateLimitedEmbeddingProvider{
		provider: provider,
		limiter:  rate.NewLimiter(rate.Limit(requestsPerSecond), burstSize),
	}
}

// Dimensions returns the embedding dimensionality from the wrapped provider
func (p *RateLimitedEmbeddingProvider) Dimensions() int {
	return p.provider.Dimensions()
}

// GenerateEmbedding generates a single embedding with rate limiting
func (p *RateLimitedEmbeddingProvider) GenerateEmbedding(ctx context.Context, text string) ([]float32, error) {
	// Wait for rate limiter (respects context cancellation)
	if err := p.limiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("rate limiter error: %w", err)
	}
	
	return p.provider.GenerateEmbedding(ctx, text)
}

// GenerateEmbeddings generates multiple embeddings with rate limiting.
// Note: This applies rate limiting per batch call, not per individual text.
func (p *RateLimitedEmbeddingProvider) GenerateEmbeddings(ctx context.Context, texts []string) ([][]float32, error) {
	// Wait for rate limiter (respects context cancellation)
	if err := p.limiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("rate limiter error: %w", err)
	}
	
	return p.provider.GenerateEmbeddings(ctx, texts)
}

