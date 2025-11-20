package rag

import (
	"context"
	"fmt"

	"github.com/apache/arrow/go/v17/arrow"
	"github.com/apache/arrow/go/v17/arrow/array"
	"github.com/apache/arrow/go/v17/arrow/memory"
	"github.com/aqua777/go-lancedb"
)

// Document represents a document chunk with embedding and metadata
type Document struct {
	ID           string
	Text         string
	DocumentName string
	Embedding    []float32
	Metadata     map[string]interface{}
}

// AddDocuments adds documents to the user's table with automatic indexing.
// Large document sets are automatically batched to prevent memory exhaustion.
func (s *RAGStore) AddDocuments(ctx context.Context, userID string, docs []Document) error {
	return s.AddDocumentsWithProgress(ctx, userID, docs, nil)
}

// AddDocumentsWithProgress adds documents with progress reporting.
// The callback receives progress updates during the operation.
// Pass nil for callback to disable progress reporting (equivalent to AddDocuments).
func (s *RAGStore) AddDocumentsWithProgress(ctx context.Context, userID string, docs []Document, callback ProgressCallback) error {
	if len(docs) == 0 {
		return fmt.Errorf("no documents to add")
	}

	// Check for context cancellation
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// Validate embeddings
	for i, doc := range docs {
		if len(doc.Embedding) != s.embeddingDim {
			return fmt.Errorf("document %d: embedding dimension mismatch: expected %d, got %d",
				i, s.embeddingDim, len(doc.Embedding))
		}
	}

	// Initialize progress tracker
	var tracker *ProgressTracker
	if callback != nil {
		tracker = NewProgressTracker("validating", int64(len(docs)), callback)
		tracker.SetStage("inserting")
	}

	// Acquire per-user lock for concurrent write protection
	lock := s.getUserLock(userID)
	lock.Lock()
	defer lock.Unlock()

	// Get or create table
	table, err := s.getOrCreateTable(userID)
	if err != nil {
		return err
	}
	defer table.Close()

	// Process documents in batches to prevent memory exhaustion
	for batchStart := 0; batchStart < len(docs); batchStart += s.maxBatchSize {
		// Check for context cancellation between batches
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		batchEnd := batchStart + s.maxBatchSize
		if batchEnd > len(docs) {
			batchEnd = len(docs)
		}
		batch := docs[batchStart:batchEnd]

		if err := s.addDocumentsBatch(table, batch); err != nil {
			return fmt.Errorf("failed to add batch [%d:%d]: %w", batchStart, batchEnd, err)
		}

		// Update progress
		if tracker != nil {
			tracker.Add(int64(len(batch)))
		}
	}

	// Ensure index exists after first insert
	if tracker != nil {
		tracker.SetStage("indexing")
	}
	if err := s.ensureIndex(table, userID); err != nil {
		return fmt.Errorf("failed to create index: %w", err)
	}

	// Mark as complete
	if tracker != nil {
		tracker.Complete()
	}

	return nil
}

// addDocumentsBatch inserts a single batch of documents
func (s *RAGStore) addDocumentsBatch(table *lancedb.Table, docs []Document) error {
	// Build Arrow record
	schema := arrow.NewSchema(
		[]arrow.Field{
			{Name: "id", Type: arrow.BinaryTypes.String, Nullable: false},
			{Name: "text", Type: arrow.BinaryTypes.String, Nullable: false},
			{Name: "document_name", Type: arrow.BinaryTypes.String, Nullable: false},
			{Name: "embedding", Type: arrow.FixedSizeListOf(int32(s.embeddingDim), arrow.PrimitiveTypes.Float32), Nullable: false},
			{Name: "metadata", Type: arrow.BinaryTypes.String, Nullable: true},
		},
		nil,
	)

	mem := memory.NewGoAllocator()
	recordBuilder := array.NewRecordBuilder(mem, schema)
	defer recordBuilder.Release()

	idBuilder := recordBuilder.Field(0).(*array.StringBuilder)
	textBuilder := recordBuilder.Field(1).(*array.StringBuilder)
	docNameBuilder := recordBuilder.Field(2).(*array.StringBuilder)
	embeddingBuilder := recordBuilder.Field(3).(*array.FixedSizeListBuilder)
	embeddingValueBuilder := embeddingBuilder.ValueBuilder().(*array.Float32Builder)
	metadataBuilder := recordBuilder.Field(4).(*array.StringBuilder)

	for _, doc := range docs {
		idBuilder.Append(doc.ID)
		textBuilder.Append(doc.Text)
		docNameBuilder.Append(doc.DocumentName)

		// Append embedding
		embeddingBuilder.Append(true)
		for _, val := range doc.Embedding {
			embeddingValueBuilder.Append(val)
		}

		// Encode and append metadata
		metaJSON, err := encodeMetadata(doc.Metadata)
		if err != nil {
			return fmt.Errorf("failed to encode metadata for document %s: %w", doc.ID, err)
		}
		metadataBuilder.Append(metaJSON)
	}

	record := recordBuilder.NewRecord()
	defer record.Release()

	// Insert data
	if err := table.Add(record, lancedb.AddModeAppend); err != nil {
		return fmt.Errorf("failed to add documents: %w", err)
	}

	return nil
}

// DeleteByDocumentName removes all chunks associated with a document name
func (s *RAGStore) DeleteByDocumentName(ctx context.Context, userID string, documentName string) error {
	if documentName == "" {
		return fmt.Errorf("document name cannot be empty")
	}

	// Check for context cancellation
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// Acquire per-user lock for write protection
	lock := s.getUserLock(userID)
	lock.Lock()
	defer lock.Unlock()

	table, err := s.conn.OpenTable(s.getTableName(userID))
	if err != nil {
		return fmt.Errorf("failed to open table for user %s: %w", userID, err)
	}
	defer table.Close()

	// Delete rows matching the document name
	predicate := fmt.Sprintf("document_name = '%s'", escapeSQLString(documentName))
	if err := table.Delete(predicate); err != nil {
		return fmt.Errorf("failed to delete documents with name %s: %w", documentName, err)
	}

	return nil
}

// ClearUserData deletes all rows from the user's table but keeps the table structure
func (s *RAGStore) ClearUserData(ctx context.Context, userID string) error {
	exists, err := s.TableExists(ctx, userID)
	if err != nil {
		return err
	}
	if !exists {
		return nil // Nothing to clear
	}

	// Check for context cancellation
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// Acquire per-user lock for write protection
	lock := s.getUserLock(userID)
	lock.Lock()
	defer lock.Unlock()

	table, err := s.conn.OpenTable(s.getTableName(userID))
	if err != nil {
		return fmt.Errorf("failed to open table for user %s: %w", userID, err)
	}
	defer table.Close()

	// Delete all rows (using a predicate that's always true)
	// LanceDB requires a predicate, so we use a simple one
	if err := table.Delete("id != ''"); err != nil {
		return fmt.Errorf("failed to clear data for user %s: %w", userID, err)
	}

	// Reset index tracking
	s.mu.Lock()
	delete(s.indexCreated, userID)
	s.mu.Unlock()

	return nil
}

// ClearUserTable removes all data from the user's table but keeps the table structure.
// Note: This does not drop the table itself, only clears all rows.
func (s *RAGStore) ClearUserTable(ctx context.Context, userID string) error {
	return s.ClearUserData(ctx, userID)
}

// CountDocuments returns the total number of document chunks for a user
func (s *RAGStore) CountDocuments(ctx context.Context, userID string) (int64, error) {
	exists, err := s.TableExists(ctx, userID)
	if err != nil {
		return 0, err
	}
	if !exists {
		return 0, nil
	}

	// Check for context cancellation
	select {
	case <-ctx.Done():
		return 0, ctx.Err()
	default:
	}

	table, err := s.conn.OpenTable(s.getTableName(userID))
	if err != nil {
		return 0, fmt.Errorf("failed to open table for user %s: %w", userID, err)
	}
	defer table.Close()

	count, err := table.CountRows()
	if err != nil {
		return 0, fmt.Errorf("failed to count documents: %w", err)
	}

	return count, nil
}

// UpdateDocument updates a single document by ID. If the document doesn't exist, returns an error.
// Use UpsertDocuments if you want automatic insert-or-update behavior.
func (s *RAGStore) UpdateDocument(ctx context.Context, userID string, doc Document) error {
	if doc.ID == "" {
		return fmt.Errorf("document ID cannot be empty")
	}

	// Check for context cancellation
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// Validate embedding
	if len(doc.Embedding) != s.embeddingDim {
		return fmt.Errorf("embedding dimension mismatch: expected %d, got %d",
			s.embeddingDim, len(doc.Embedding))
	}

	// Acquire per-user lock for write protection
	lock := s.getUserLock(userID)
	lock.Lock()
	defer lock.Unlock()

	// Delete old document with this ID
	table, err := s.conn.OpenTable(s.getTableName(userID))
	if err != nil {
		return fmt.Errorf("failed to open table for user %s: %w", userID, err)
	}
	defer table.Close()

	predicate := fmt.Sprintf("id = '%s'", escapeSQLString(doc.ID))
	if err := table.Delete(predicate); err != nil {
		return fmt.Errorf("failed to delete old document: %w", err)
	}

	// Insert new version
	if err := s.addDocumentsBatch(table, []Document{doc}); err != nil {
		return fmt.Errorf("failed to insert updated document: %w", err)
	}

	return nil
}

// UpsertDocuments inserts or updates documents. If a document with the same ID exists, it's updated.
// Otherwise, it's inserted. This is more efficient than calling UpdateDocument multiple times.
func (s *RAGStore) UpsertDocuments(ctx context.Context, userID string, docs []Document) error {
	return s.UpsertDocumentsWithProgress(ctx, userID, docs, nil)
}

// UpsertDocumentsWithProgress upserts documents with progress reporting.
// The callback receives progress updates during the operation.
func (s *RAGStore) UpsertDocumentsWithProgress(ctx context.Context, userID string, docs []Document, callback ProgressCallback) error {
	if len(docs) == 0 {
		return fmt.Errorf("no documents to upsert")
	}

	// Check for context cancellation
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// Validate embeddings
	for i, doc := range docs {
		if len(doc.Embedding) != s.embeddingDim {
			return fmt.Errorf("document %d: embedding dimension mismatch: expected %d, got %d",
				i, s.embeddingDim, len(doc.Embedding))
		}
	}

	// Initialize progress tracker
	var tracker *ProgressTracker
	if callback != nil {
		// Total phases: deleting + inserting + indexing
		tracker = NewProgressTracker("deleting", int64(len(docs)), callback)
	}

	// Acquire per-user lock for write protection
	lock := s.getUserLock(userID)
	lock.Lock()
	defer lock.Unlock()

	table, err := s.getOrCreateTable(userID)
	if err != nil {
		return err
	}
	defer table.Close()

	// Delete all documents with IDs that match the incoming documents
	// Batch delete by building a predicate with OR conditions
	if len(docs) > 0 {
		idPredicates := make([]string, 0, len(docs))
		for _, doc := range docs {
			if doc.ID != "" {
				idPredicates = append(idPredicates, fmt.Sprintf("id = '%s'", escapeSQLString(doc.ID)))
			}
		}
		
		if len(idPredicates) > 0 {
			predicate := idPredicates[0]
			for i := 1; i < len(idPredicates); i++ {
				predicate += " OR " + idPredicates[i]
			}
			
			// Delete existing documents (ignore errors if documents don't exist)
			_ = table.Delete(predicate)
		}
	}

	if tracker != nil {
		tracker.SetStage("inserting")
	}

	// Insert all documents in batches
	for batchStart := 0; batchStart < len(docs); batchStart += s.maxBatchSize {
		// Check for context cancellation between batches
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		batchEnd := batchStart + s.maxBatchSize
		if batchEnd > len(docs) {
			batchEnd = len(docs)
		}
		batch := docs[batchStart:batchEnd]

		if err := s.addDocumentsBatch(table, batch); err != nil {
			return fmt.Errorf("failed to upsert batch [%d:%d]: %w", batchStart, batchEnd, err)
		}

		// Update progress
		if tracker != nil {
			tracker.Add(int64(len(batch)))
		}
	}

	// Ensure index exists
	if tracker != nil {
		tracker.SetStage("indexing")
	}
	if err := s.ensureIndex(table, userID); err != nil {
		return fmt.Errorf("failed to create index: %w", err)
	}

	if tracker != nil {
		tracker.Complete()
	}

	return nil
}

