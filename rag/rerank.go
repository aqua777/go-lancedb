package rag

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
)

// Reranker is an interface for re-ranking search results
type Reranker interface {
	// Rerank takes a query and search results, and returns re-ranked results with new scores
	Rerank(ctx context.Context, query string, results []SearchResult) ([]SearchResult, error)
}

// CrossEncoderReranker uses a cross-encoder model to re-rank results
// Cross-encoders jointly encode query and document for better relevance scoring
type CrossEncoderReranker struct {
	URL        string // HTTP endpoint for cross-encoder model
	httpClient *http.Client
}

// NewCrossEncoderReranker creates a new cross-encoder reranker
func NewCrossEncoderReranker(url string) *CrossEncoderReranker {
	return &CrossEncoderReranker{
		URL:        url,
		httpClient: &http.Client{},
	}
}

// Rerank re-ranks search results using a cross-encoder model
func (r *CrossEncoderReranker) Rerank(ctx context.Context, query string, results []SearchResult) ([]SearchResult, error) {
	if len(results) == 0 {
		return results, nil
	}

	// Prepare texts for scoring
	texts := make([]string, len(results))
	for i, result := range results {
		texts[i] = result.Text
	}

	// Call reranking service
	scores, err := r.scoreTexts(ctx, query, texts)
	if err != nil {
		return nil, fmt.Errorf("failed to get reranking scores: %w", err)
	}

	if len(scores) != len(results) {
		return nil, fmt.Errorf("expected %d scores, got %d", len(results), len(scores))
	}

	// Update scores and sort
	reranked := make([]SearchResult, len(results))
	copy(reranked, results)
	
	for i := range reranked {
		reranked[i].Score = scores[i]
	}

	// Sort by score descending (higher score = more relevant for cross-encoder)
	sort.Slice(reranked, func(i, j int) bool {
		return reranked[i].Score > reranked[j].Score
	})

	return reranked, nil
}

// scoreTexts calls the HTTP endpoint to score query-document pairs
func (r *CrossEncoderReranker) scoreTexts(ctx context.Context, query string, texts []string) ([]float32, error) {
	requestBody := map[string]interface{}{
		"query": query,
		"texts": texts,
	}

	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", r.URL, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := r.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("reranking service error (status %d): %s", resp.StatusCode, string(body))
	}

	var response struct {
		Scores []float32 `json:"scores"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return response.Scores, nil
}

// ReciprocRankFusionReranker implements Reciprocal Rank Fusion (RRF) for combining multiple result sets
// Useful when you have results from multiple sources (e.g., vector + keyword search)
type ReciprocalRankFusionReranker struct {
	K float32 // Constant for RRF formula (default: 60)
}

// NewReciprocalRankFusionReranker creates an RRF reranker
func NewReciprocalRankFusionReranker(k float32) *ReciprocalRankFusionReranker {
	if k <= 0 {
		k = 60 // default value from the RRF paper
	}
	return &ReciprocalRankFusionReranker{K: k}
}

// Rerank applies RRF to the results (assumes results are already in ranked order)
func (r *ReciprocalRankFusionReranker) Rerank(ctx context.Context, query string, results []SearchResult) ([]SearchResult, error) {
	if len(results) == 0 {
		return results, nil
	}

	// Calculate RRF scores based on rank
	reranked := make([]SearchResult, len(results))
	copy(reranked, results)

	for i := range reranked {
		rank := float32(i + 1)
		rrfScore := 1.0 / (r.K + rank)
		reranked[i].Score = rrfScore
	}

	return reranked, nil
}

// CombineRankedLists combines multiple ranked lists using RRF
func (r *ReciprocalRankFusionReranker) CombineRankedLists(ctx context.Context, resultSets [][]SearchResult) ([]SearchResult, error) {
	if len(resultSets) == 0 {
		return []SearchResult{}, nil
	}

	// Build a map of document ID to combined RRF score
	scoreMap := make(map[string]float32)
	resultMap := make(map[string]SearchResult)

	for _, resultSet := range resultSets {
		for rank, result := range resultSet {
			rrfScore := 1.0 / (r.K + float32(rank+1))
			
			if existing, exists := scoreMap[result.ID]; exists {
				scoreMap[result.ID] = existing + rrfScore
			} else {
				scoreMap[result.ID] = rrfScore
				resultMap[result.ID] = result
			}
		}
	}

	// Convert to sorted list
	combined := make([]SearchResult, 0, len(scoreMap))
	for id, score := range scoreMap {
		result := resultMap[id]
		result.Score = score
		combined = append(combined, result)
	}

	// Sort by combined score descending
	sort.Slice(combined, func(i, j int) bool {
		return combined[i].Score > combined[j].Score
	})

	return combined, nil
}

// CustomScorerReranker allows using a custom scoring function
type CustomScorerReranker struct {
	ScoreFunc func(query string, result SearchResult) float32
}

// NewCustomScorerReranker creates a reranker with a custom scoring function
func NewCustomScorerReranker(scoreFunc func(query string, result SearchResult) float32) *CustomScorerReranker {
	return &CustomScorerReranker{ScoreFunc: scoreFunc}
}

// Rerank re-ranks results using the custom scoring function
func (r *CustomScorerReranker) Rerank(ctx context.Context, query string, results []SearchResult) ([]SearchResult, error) {
	if len(results) == 0 {
		return results, nil
	}

	reranked := make([]SearchResult, len(results))
	copy(reranked, results)

	// Apply custom scoring
	for i := range reranked {
		reranked[i].Score = r.ScoreFunc(query, reranked[i])
	}

	// Sort by score descending
	sort.Slice(reranked, func(i, j int) bool {
		return reranked[i].Score > reranked[j].Score
	})

	return reranked, nil
}

// SearchWithRerank performs search and applies reranking
func (s *RAGStore) SearchWithRerank(ctx context.Context, userID string, queryText string, provider EmbeddingProvider, reranker Reranker, opts *SearchOptions) ([]SearchResult, error) {
	// First, perform regular vector search
	results, err := s.SearchWithText(ctx, userID, queryText, provider, opts)
	if err != nil {
		return nil, fmt.Errorf("search failed: %w", err)
	}

	// Apply reranking
	reranked, err := reranker.Rerank(ctx, queryText, results)
	if err != nil {
		return nil, fmt.Errorf("reranking failed: %w", err)
	}

	return reranked, nil
}

