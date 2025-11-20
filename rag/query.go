package rag

import (
	"context"
	"fmt"
	"strings"

	"github.com/apache/arrow/go/v17/arrow"
	"github.com/apache/arrow/go/v17/arrow/array"
	"github.com/aqua777/go-lancedb"
)

// SearchResult represents a single search result with document content and metadata
type SearchResult struct {
	ID           string
	Text         string
	DocumentName string
	Embedding    []float32
	Metadata     map[string]interface{}
	Score        float32 // Distance score (lower is better for L2, higher for cosine)
}

// SearchOptions configures search behavior
type SearchOptions struct {
	Limit        int                    // Maximum number of results (default: 10)
	Filters      map[string]interface{} // Metadata filters (applied as SQL predicates)
	DistanceType lancedb.DistanceType   // Distance metric (default: Cosine)
}

// Search performs vector similarity search on the user's documents
func (s *RAGStore) Search(ctx context.Context, userID string, queryEmbedding []float32, opts *SearchOptions) ([]SearchResult, error) {
	if len(queryEmbedding) != s.embeddingDim {
		return nil, fmt.Errorf("query embedding dimension mismatch: expected %d, got %d",
			s.embeddingDim, len(queryEmbedding))
	}

	// Check for context cancellation
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	// Set defaults
	if opts == nil {
		opts = &SearchOptions{
			Limit:        10,
			DistanceType: lancedb.DistanceTypeCosine,
		}
	}
	if opts.Limit <= 0 {
		opts.Limit = 10
	}

	// Check if table exists
	exists, err := s.TableExists(ctx, userID)
	if err != nil {
		return nil, err
	}
	if !exists {
		return []SearchResult{}, nil // No documents yet
	}

	table, err := s.conn.OpenTable(s.getTableName(userID))
	if err != nil {
		return nil, fmt.Errorf("failed to open table for user %s: %w", userID, err)
	}
	defer table.Close()

	// Build query
	query := table.Query()
	defer query.Close()

	query = query.
		NearestTo(queryEmbedding).
		SetDistanceType(opts.DistanceType).
		Limit(opts.Limit).
		Select("id", "text", "document_name", "embedding", "metadata", "_distance")

	// Apply filters if provided
	if len(opts.Filters) > 0 {
		predicate := buildPredicate(opts.Filters)
		query = query.Where(predicate)
	}

	// Execute query
	records, err := query.Execute()
	if err != nil {
		return nil, fmt.Errorf("failed to execute search: %w", err)
	}

	// Parse results
	results := make([]SearchResult, 0)
	for _, record := range records {
		recordResults, err := parseSearchResults(record, s.embeddingDim)
		if err != nil {
			// Clean up
			for _, r := range records {
				r.Release()
			}
			return nil, fmt.Errorf("failed to parse results: %w", err)
		}
		results = append(results, recordResults...)
		record.Release()
	}

	return results, nil
}

// SearchByDocument searches within a specific document's chunks
func (s *RAGStore) SearchByDocument(ctx context.Context, userID string, queryEmbedding []float32, documentName string, limit int) ([]SearchResult, error) {
	if documentName == "" {
		return nil, fmt.Errorf("document name cannot be empty")
	}

	opts := &SearchOptions{
		Limit: limit,
		Filters: map[string]interface{}{
			"document_name": documentName,
		},
		DistanceType: lancedb.DistanceTypeCosine,
	}

	return s.Search(ctx, userID, queryEmbedding, opts)
}

// validFilterKeys defines the allowed keys for filtering to prevent SQL injection
var validFilterKeys = map[string]bool{
	"id":            true,
	"text":          true,
	"document_name": true,
	"metadata":      true,
}

// isValidFilterKey checks if a filter key is in the allowed list
func isValidFilterKey(key string) bool {
	return validFilterKeys[key]
}

// escapeSQLString escapes characters in SQL string literals to prevent injection.
// Handles single quotes, backslashes, and other special characters.
func escapeSQLString(s string) string {
	// Escape backslashes first
	s = strings.ReplaceAll(s, "\\", "\\\\")
	// Escape single quotes
	s = strings.ReplaceAll(s, "'", "''")
	// Remove null bytes
	s = strings.ReplaceAll(s, "\x00", "")
	return s
}

// sanitizeIdentifier ensures a SQL identifier (column name) is safe.
// Only allows alphanumeric characters and underscores.
func sanitizeIdentifier(identifier string) (string, error) {
	if identifier == "" {
		return "", fmt.Errorf("identifier cannot be empty")
	}
	
	// Check for valid characters only
	for _, ch := range identifier {
		if !((ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || 
			 (ch >= '0' && ch <= '9') || ch == '_') {
			return "", fmt.Errorf("invalid identifier: contains illegal character '%c'", ch)
		}
	}
	
	return identifier, nil
}

// buildPredicate converts a filter map to a SQL-like predicate string with injection protection
func buildPredicate(filters map[string]interface{}) string {
	if len(filters) == 0 {
		return ""
	}

	predicates := make([]string, 0, len(filters))
	for key, value := range filters {
		// Validate filter key against whitelist
		if !isValidFilterKey(key) {
			// Skip invalid keys silently to prevent errors, but log would be good
			continue
		}
		
		// Sanitize the key
		safeKey, err := sanitizeIdentifier(key)
		if err != nil {
			continue
		}

		switch v := value.(type) {
		case string:
			predicates = append(predicates, fmt.Sprintf("%s = '%s'", safeKey, escapeSQLString(v)))
		case int:
			predicates = append(predicates, fmt.Sprintf("%s = %d", safeKey, v))
		case int32:
			predicates = append(predicates, fmt.Sprintf("%s = %d", safeKey, v))
		case int64:
			predicates = append(predicates, fmt.Sprintf("%s = %d", safeKey, v))
		case float32:
			predicates = append(predicates, fmt.Sprintf("%s = %f", safeKey, v))
		case float64:
			predicates = append(predicates, fmt.Sprintf("%s = %f", safeKey, v))
		case bool:
			predicates = append(predicates, fmt.Sprintf("%s = %t", safeKey, v))
		default:
			// Skip unsupported types
			continue
		}
	}

	if len(predicates) == 0 {
		return ""
	}

	// Join with AND
	result := predicates[0]
	for i := 1; i < len(predicates); i++ {
		result += " AND " + predicates[i]
	}
	return result
}

// parseSearchResults parses Arrow records into SearchResult structs
func parseSearchResults(record arrow.Record, embeddingDim int) ([]SearchResult, error) {
	numRows := int(record.NumRows())
	results := make([]SearchResult, numRows)

	idCol := record.Column(0).(*array.String)
	textCol := record.Column(1).(*array.String)
	docNameCol := record.Column(2).(*array.String)
	embeddingCol := record.Column(3).(*array.FixedSizeList)
	metadataCol := record.Column(4).(*array.String)
	
	// Distance column is optional (column 5 if present)
	var distanceCol *array.Float32
	if record.NumCols() > 5 {
		distanceCol = record.Column(5).(*array.Float32)
	}

	// Get the underlying float32 values array
	embeddingValues := embeddingCol.ListValues().(*array.Float32)

	for i := 0; i < numRows; i++ {
		// IMPORTANT: Copy strings explicitly to avoid referencing freed Arrow memory
		// Arrow string columns point to the record's buffer, which gets freed on Release()
		results[i].ID = string([]byte(idCol.Value(i)))
		results[i].Text = string([]byte(textCol.Value(i)))
		results[i].DocumentName = string([]byte(docNameCol.Value(i)))

		// Extract embedding for this row
		start := i * embeddingDim
		results[i].Embedding = make([]float32, embeddingDim)
		for j := 0; j < embeddingDim; j++ {
			results[i].Embedding[j] = embeddingValues.Value(start + j)
		}

		// Parse metadata (JSON unmarshal already creates new strings, so no copy needed)
		metaJSON := metadataCol.Value(i)
		meta, err := decodeMetadata(metaJSON)
		if err != nil {
			return nil, fmt.Errorf("failed to decode metadata for row %d: %w", i, err)
		}
		results[i].Metadata = meta
		
		// Extract distance score if available
		if distanceCol != nil {
			results[i].Score = distanceCol.Value(i)
		}
	}

	return results, nil
}

// DocumentNamePage represents a page of document names with pagination info
type DocumentNamePage struct {
	Names      []string
	TotalCount int64
	Offset     int
	Limit      int
	HasMore    bool
}

// ListDocumentNamesPaginated returns unique document names for a user with pagination support.
// This is more memory-efficient for large datasets than ListDocumentNames.
func (s *RAGStore) ListDocumentNamesPaginated(ctx context.Context, userID string, offset, limit int) (*DocumentNamePage, error) {
	if offset < 0 {
		return nil, fmt.Errorf("offset must be non-negative, got %d", offset)
	}
	if limit <= 0 {
		limit = 100 // default page size
	}

	// Check for context cancellation
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	exists, err := s.TableExists(ctx, userID)
	if err != nil {
		return nil, err
	}
	if !exists {
		return &DocumentNamePage{Names: []string{}, TotalCount: 0, Offset: offset, Limit: limit, HasMore: false}, nil
	}

	table, err := s.conn.OpenTable(s.getTableName(userID))
	if err != nil {
		return nil, fmt.Errorf("failed to open table for user %s: %w", userID, err)
	}
	defer table.Close()

	// Get total count
	totalCount, err := table.CountRows()
	if err != nil {
		return nil, fmt.Errorf("failed to count rows: %w", err)
	}

	// Create a query that only selects the document_name column
	query := table.Query()
	defer query.Close()
	
	records, err := query.Select("document_name").Execute()
	if err != nil {
		return nil, fmt.Errorf("failed to query table: %w", err)
	}

	// Collect unique document names in a streaming fashion
	docNameSet := make(map[string]bool)
	
	for _, record := range records {
		docNameCol := record.Column(0).(*array.String)
		for i := 0; i < int(record.NumRows()); i++ {
			// Copy string to avoid referencing freed Arrow memory
			name := string([]byte(docNameCol.Value(i)))
			if !docNameSet[name] {
				docNameSet[name] = true
			}
		}
		record.Release()
	}

	// Convert set to sorted slice for consistent pagination
	allNames := make([]string, 0, len(docNameSet))
	for name := range docNameSet {
		allNames = append(allNames, name)
	}
	
	// Sort for consistent results across pages
	sortStrings(allNames)
	
	// Apply pagination
	start := offset
	if start > len(allNames) {
		start = len(allNames)
	}
	end := start + limit
	hasMore := false
	if end > len(allNames) {
		end = len(allNames)
	} else {
		hasMore = true
	}
	
	pageNames := allNames[start:end]

	return &DocumentNamePage{
		Names:      pageNames,
		TotalCount: totalCount,
		Offset:     offset,
		Limit:      limit,
		HasMore:    hasMore,
	}, nil
}

// ListDocumentNames returns all unique document names for a user.
// For large datasets with many documents, consider using ListDocumentNamesPaginated instead.
func (s *RAGStore) ListDocumentNames(ctx context.Context, userID string) ([]string, error) {
	page, err := s.ListDocumentNamesPaginated(ctx, userID, 0, 10000)
	if err != nil {
		return nil, err
	}
	return page.Names, nil
}

// sortStrings is a simple insertion sort for small slices, good enough for document names
func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		key := s[i]
		j := i - 1
		for j >= 0 && s[j] > key {
			s[j+1] = s[j]
			j--
		}
		s[j+1] = key
	}
}

