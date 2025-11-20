package rag

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
)

// HybridSearchOptions configures hybrid search behavior
type HybridSearchOptions struct {
	Limit          int                    // Maximum number of results
	VectorWeight   float32                // Weight for vector search (0-1, default: 0.5)
	KeywordWeight  float32                // Weight for keyword search (0-1, default: 0.5)
	Filters        map[string]interface{} // Metadata filters
	MinKeywordScore float32               // Minimum BM25 score to include (default: 0)
}

// HybridSearch performs both vector and keyword search, then combines results
func (s *RAGStore) HybridSearch(ctx context.Context, userID string, queryText string, queryEmbedding []float32, opts *HybridSearchOptions) ([]SearchResult, error) {
	if opts == nil {
		opts = &HybridSearchOptions{
			Limit:         10,
			VectorWeight:  0.5,
			KeywordWeight: 0.5,
		}
	}

	// Normalize weights
	totalWeight := opts.VectorWeight + opts.KeywordWeight
	if totalWeight == 0 {
		return nil, fmt.Errorf("at least one of VectorWeight or KeywordWeight must be non-zero")
	}
	opts.VectorWeight = opts.VectorWeight / totalWeight
	opts.KeywordWeight = opts.KeywordWeight / totalWeight

	// Check for context cancellation
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	// Perform vector search (fetch more results for better fusion)
	vectorLimit := opts.Limit * 3
	if vectorLimit > 100 {
		vectorLimit = 100
	}

	vectorSearchOpts := &SearchOptions{
		Limit:   vectorLimit,
		Filters: opts.Filters,
	}

	vectorResults, err := s.Search(ctx, userID, queryEmbedding, vectorSearchOpts)
	if err != nil {
		return nil, fmt.Errorf("vector search failed: %w", err)
	}

	// Perform keyword search
	keywordResults, err := s.keywordSearch(ctx, userID, queryText, vectorLimit, opts.Filters)
	if err != nil {
		return nil, fmt.Errorf("keyword search failed: %w", err)
	}

	// Combine results using RRF or weighted scoring
	combined := s.combineResults(vectorResults, keywordResults, opts)

	// Limit final results
	if len(combined) > opts.Limit {
		combined = combined[:opts.Limit]
	}

	return combined, nil
}

// keywordSearch performs BM25-based keyword search.
// WARNING: This loads ALL documents into memory to calculate BM25 scores.
// For large document collections, this can cause memory exhaustion.
// Use the MaxDocumentsForBM25 limit to prevent issues (default: 10,000).
func (s *RAGStore) keywordSearch(ctx context.Context, userID string, queryText string, limit int, filters map[string]interface{}) ([]SearchResult, error) {
	// Check if table exists
	exists, err := s.TableExists(ctx, userID)
	if err != nil {
		return nil, err
	}
	if !exists {
		return []SearchResult{}, nil
	}

	table, err := s.conn.OpenTable(s.getTableName(userID))
	if err != nil {
		return nil, fmt.Errorf("failed to open table: %w", err)
	}
	defer table.Close()

	// Check document count before loading all documents into memory
	// BM25 calculation requires all documents, which doesn't scale well
	if s.maxDocumentsForBM25 > 0 {
		count, err := table.CountRows()
		if err != nil {
			return nil, fmt.Errorf("failed to count documents: %w", err)
		}
		if count > int64(s.maxDocumentsForBM25) {
			return nil, fmt.Errorf("document count (%d) exceeds BM25 limit (%d); use pure vector search instead or increase limit with SetMaxDocumentsForBM25()", count, s.maxDocumentsForBM25)
		}
	}

	// Get all documents (for BM25 calculation)
	query := table.Query()
	defer query.Close()

	// Apply filters if provided
	if len(filters) > 0 {
		predicate := buildPredicate(filters)
		query = query.Where(predicate)
	}

	records, err := query.Select("id", "text", "document_name", "embedding", "metadata").Execute()
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}

	// Parse all results
	var allResults []SearchResult
	for _, record := range records {
		results, err := parseSearchResults(record, s.embeddingDim)
		if err != nil {
			for _, r := range records {
				r.Release()
			}
			return nil, fmt.Errorf("failed to parse results: %w", err)
		}
		allResults = append(allResults, results...)
		record.Release()
	}

	// Calculate BM25 scores
	queryTerms := tokenize(queryText)
	scoredResults := calculateBM25(allResults, queryTerms)

	// Sort by BM25 score descending
	sort.Slice(scoredResults, func(i, j int) bool {
		return scoredResults[i].Score > scoredResults[j].Score
	})

	// Limit results
	if len(scoredResults) > limit {
		scoredResults = scoredResults[:limit]
	}

	return scoredResults, nil
}

// combineResults merges vector and keyword results with weighted scoring
func (s *RAGStore) combineResults(vectorResults, keywordResults []SearchResult, opts *HybridSearchOptions) []SearchResult {
	// Build maps for quick lookup
	resultMap := make(map[string]SearchResult)
	scoreMap := make(map[string]float32)

	// Normalize vector scores (lower distance = higher score for cosine)
	maxVectorScore := float32(0.0)
	for _, result := range vectorResults {
		// For cosine similarity, distance is 1 - similarity, so invert
		score := 1.0 - result.Score
		if score > maxVectorScore {
			maxVectorScore = score
		}
	}

	for i, result := range vectorResults {
		normalizedScore := float32(0.0)
		if maxVectorScore > 0 {
			normalizedScore = (1.0 - result.Score) / maxVectorScore
		}
		
		weightedScore := normalizedScore * opts.VectorWeight
		
		resultMap[result.ID] = result
		scoreMap[result.ID] = weightedScore
		
		// Adjust rank in metadata
		if vectorResults[i].Metadata == nil {
			vectorResults[i].Metadata = make(map[string]interface{})
		}
		vectorResults[i].Metadata["vector_rank"] = i
	}

	// Normalize keyword scores
	maxKeywordScore := float32(0.0)
	for _, result := range keywordResults {
		if result.Score > maxKeywordScore {
			maxKeywordScore = result.Score
		}
	}

	for i, result := range keywordResults {
		normalizedScore := float32(0.0)
		if maxKeywordScore > 0 {
			normalizedScore = result.Score / maxKeywordScore
		}
		
		weightedScore := normalizedScore * opts.KeywordWeight
		
		if existing, exists := scoreMap[result.ID]; exists {
			scoreMap[result.ID] = existing + weightedScore
		} else {
			resultMap[result.ID] = result
			scoreMap[result.ID] = weightedScore
		}
		
		// Adjust rank in metadata
		if existingResult, exists := resultMap[result.ID]; exists {
			if existingResult.Metadata == nil {
				existingResult.Metadata = make(map[string]interface{})
			}
			existingResult.Metadata["keyword_rank"] = i
			resultMap[result.ID] = existingResult
		}
	}

	// Convert to sorted list
	combined := make([]SearchResult, 0, len(resultMap))
	for id, result := range resultMap {
		result.Score = scoreMap[id]
		combined = append(combined, result)
	}

	// Sort by combined score descending
	sort.Slice(combined, func(i, j int) bool {
		return combined[i].Score > combined[j].Score
	})

	return combined
}

// tokenize splits text into lowercase tokens
func tokenize(text string) []string {
	text = strings.ToLower(text)
	tokens := strings.FieldsFunc(text, func(r rune) bool {
		return !((r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'))
	})
	return tokens
}

// calculateBM25 computes BM25 scores for documents given query terms
// BM25 parameters: k1=1.5, b=0.75
func calculateBM25(documents []SearchResult, queryTerms []string) []SearchResult {
	if len(documents) == 0 || len(queryTerms) == 0 {
		return documents
	}

	k1 := float32(1.5)
	b := float32(0.75)

	// Calculate average document length
	totalLength := 0
	docLengths := make([]int, len(documents))
	for i, doc := range documents {
		tokens := tokenize(doc.Text)
		docLengths[i] = len(tokens)
		totalLength += len(tokens)
	}
	avgDocLength := float32(totalLength) / float32(len(documents))

	// Calculate IDF for each query term
	idf := make(map[string]float32)
	for _, term := range queryTerms {
		docCount := 0
		for _, doc := range documents {
			if containsTerm(tokenize(doc.Text), term) {
				docCount++
			}
		}
		if docCount > 0 {
			numerator := float64(len(documents)-docCount) + 0.5
			denominator := float64(docCount) + 0.5
			idf[term] = float32(math.Log(numerator / denominator))
		}
	}

	// Calculate BM25 score for each document
	scored := make([]SearchResult, len(documents))
	copy(scored, documents)

	for i, doc := range documents {
		docTokens := tokenize(doc.Text)
		termFreq := make(map[string]int)
		for _, token := range docTokens {
			termFreq[token]++
		}

		score := float32(0.0)
		for _, term := range queryTerms {
			if termIDF, ok := idf[term]; ok {
				tf := float32(termFreq[term])
				docLen := float32(docLengths[i])
				
				numerator := tf * (k1 + 1)
				denominator := tf + k1*(1-b+b*(docLen/avgDocLength))
				
				score += termIDF * (numerator / denominator)
			}
		}

		scored[i].Score = score
	}

	return scored
}

// containsTerm checks if a token list contains a specific term
func containsTerm(tokens []string, term string) bool {
	for _, token := range tokens {
		if token == term {
			return true
		}
	}
	return false
}

// HybridSearchWithText performs hybrid search using text query (generates embedding automatically)
func (s *RAGStore) HybridSearchWithText(ctx context.Context, userID string, queryText string, provider EmbeddingProvider, opts *HybridSearchOptions) ([]SearchResult, error) {
	// Generate embedding for query
	embedding, err := provider.GenerateEmbedding(ctx, queryText)
	if err != nil {
		return nil, fmt.Errorf("failed to generate query embedding: %w", err)
	}

	return s.HybridSearch(ctx, userID, queryText, embedding, opts)
}

