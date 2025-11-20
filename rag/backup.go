package rag

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"
)

// BackupFormat specifies the format for backup files
type BackupFormat string

const (
	// BackupFormatJSON exports data as human-readable JSON
	BackupFormatJSON BackupFormat = "json"
	// BackupFormatJSONGzip exports data as compressed JSON (smaller files)
	BackupFormatJSONGzip BackupFormat = "json.gz"
)

// BackupMetadata contains metadata about a backup file
type BackupMetadata struct {
	Version       string    `json:"version"`        // Backup format version
	UserID        string    `json:"user_id"`        // User ID for this backup
	Created       time.Time `json:"created"`        // When the backup was created
	DocumentCount int       `json:"document_count"` // Number of documents in backup
	EmbeddingDim  int       `json:"embedding_dim"`  // Embedding dimension
	Format        string    `json:"format"`         // Backup format (json, json.gz)
}

// BackupDocument represents a document in the backup format
type BackupDocument struct {
	ID           string                 `json:"id"`
	Text         string                 `json:"text"`
	DocumentName string                 `json:"document_name"`
	Embedding    []float32              `json:"embedding"`
	Metadata     map[string]interface{} `json:"metadata"`
}

// BackupData represents the complete backup data structure
type BackupData struct {
	Metadata  BackupMetadata   `json:"metadata"`
	Documents []BackupDocument `json:"documents"`
}

// ExportUserData exports all data for a user to a backup file.
// The format parameter determines the output format (JSON or compressed JSON).
func (s *RAGStore) ExportUserData(ctx context.Context, userID string, outputPath string, format BackupFormat) error {
	// Validate user ID
	if err := validateUserID(userID); err != nil {
		return err
	}

	// Check if table exists
	exists, err := s.TableExists(ctx, userID)
	if err != nil {
		return fmt.Errorf("failed to check if table exists: %w", err)
	}
	if !exists {
		return fmt.Errorf("no data exists for user %s", userID)
	}

	// Open table
	table, err := s.conn.OpenTable(s.getTableName(userID))
	if err != nil {
		return fmt.Errorf("failed to open table: %w", err)
	}
	defer table.Close()

	// Get document count
	count, err := table.CountRows()
	if err != nil {
		return fmt.Errorf("failed to count documents: %w", err)
	}

	// Build metadata
	metadata := BackupMetadata{
		Version:       "1.0",
		UserID:        userID,
		Created:       time.Now(),
		DocumentCount: int(count),
		EmbeddingDim:  s.embeddingDim,
		Format:        string(format),
	}

	// Query all documents
	query := table.Query()
	defer query.Close()

	records, err := query.Select("id", "text", "document_name", "embedding", "metadata").Execute()
	if err != nil {
		return fmt.Errorf("failed to query documents: %w", err)
	}

	// Convert to backup format
	var documents []BackupDocument
	for _, record := range records {
		results, err := parseSearchResults(record, s.embeddingDim)
		if err != nil {
			// Clean up
			for _, r := range records {
				r.Release()
			}
			return fmt.Errorf("failed to parse documents: %w", err)
		}

		for _, result := range results {
			documents = append(documents, BackupDocument{
				ID:           result.ID,
				Text:         result.Text,
				DocumentName: result.DocumentName,
				Embedding:    result.Embedding,
				Metadata:     result.Metadata,
			})
		}
		record.Release()
	}

	// Create backup data structure
	backupData := BackupData{
		Metadata:  metadata,
		Documents: documents,
	}

	// Write to file
	return writeBackupFile(outputPath, backupData, format)
}

// ExportUserDataWithProgress exports user data with progress reporting
func (s *RAGStore) ExportUserDataWithProgress(ctx context.Context, userID string, outputPath string, format BackupFormat, callback ProgressCallback) error {
	// Validate user ID
	if err := validateUserID(userID); err != nil {
		return err
	}

	var tracker *ProgressTracker
	if callback != nil {
		tracker = NewProgressTracker("preparing", 100, callback)
	}

	// Check if table exists
	exists, err := s.TableExists(ctx, userID)
	if err != nil {
		return fmt.Errorf("failed to check if table exists: %w", err)
	}
	if !exists {
		return fmt.Errorf("no data exists for user %s", userID)
	}

	// Open table
	table, err := s.conn.OpenTable(s.getTableName(userID))
	if err != nil {
		return fmt.Errorf("failed to open table: %w", err)
	}
	defer table.Close()

	if tracker != nil {
		tracker.Add(10)
		tracker.SetStage("counting")
	}

	// Get document count
	count, err := table.CountRows()
	if err != nil {
		return fmt.Errorf("failed to count documents: %w", err)
	}

	if tracker != nil {
		tracker.Add(10)
		tracker.SetStage("reading")
		tracker.SetTotal(count + 20) // 20 for prep, rest for documents
	}

	// Build metadata
	metadata := BackupMetadata{
		Version:       "1.0",
		UserID:        userID,
		Created:       time.Now(),
		DocumentCount: int(count),
		EmbeddingDim:  s.embeddingDim,
		Format:        string(format),
	}

	// Query all documents
	query := table.Query()
	defer query.Close()

	records, err := query.Select("id", "text", "document_name", "embedding", "metadata").Execute()
	if err != nil {
		return fmt.Errorf("failed to query documents: %w", err)
	}

	// Convert to backup format
	var documents []BackupDocument
	for _, record := range records {
		// Check for cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		results, err := parseSearchResults(record, s.embeddingDim)
		if err != nil {
			// Clean up
			for _, r := range records {
				r.Release()
			}
			return fmt.Errorf("failed to parse documents: %w", err)
		}

		for _, result := range results {
			documents = append(documents, BackupDocument{
				ID:           result.ID,
				Text:         result.Text,
				DocumentName: result.DocumentName,
				Embedding:    result.Embedding,
				Metadata:     result.Metadata,
			})

			if tracker != nil {
				tracker.Increment()
			}
		}
		record.Release()
	}

	if tracker != nil {
		tracker.SetStage("writing")
		tracker.SetMessage("Writing backup file")
	}

	// Create backup data structure
	backupData := BackupData{
		Metadata:  metadata,
		Documents: documents,
	}

	// Write to file
	if err := writeBackupFile(outputPath, backupData, format); err != nil {
		return err
	}

	if tracker != nil {
		tracker.Complete()
	}

	return nil
}

// writeBackupFile writes backup data to a file in the specified format
func writeBackupFile(path string, data BackupData, format BackupFormat) error {
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("failed to create backup file: %w", err)
	}
	defer file.Close()

	var writer io.Writer = file

	// Add compression if requested
	if format == BackupFormatJSONGzip {
		gzipWriter := gzip.NewWriter(file)
		defer gzipWriter.Close()
		writer = gzipWriter
	}

	// Encode as JSON
	encoder := json.NewEncoder(writer)
	encoder.SetIndent("", "  ") // Pretty print for readability

	if err := encoder.Encode(data); err != nil {
		return fmt.Errorf("failed to encode backup data: %w", err)
	}

	return nil
}

// ValidateBackupFile validates a backup file and returns its metadata
func ValidateBackupFile(path string) (*BackupMetadata, error) {
	// Open file
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open backup file: %w", err)
	}
	defer file.Close()

	// Detect if file is gzipped
	var reader io.Reader = file
	
	// Read first two bytes to check for gzip magic number
	header := make([]byte, 2)
	if _, err := file.Read(header); err != nil {
		return nil, fmt.Errorf("failed to read file header: %w", err)
	}
	
	// Reset to beginning
	if _, err := file.Seek(0, 0); err != nil {
		return nil, fmt.Errorf("failed to seek file: %w", err)
	}

	// Check for gzip magic number (0x1f 0x8b)
	if header[0] == 0x1f && header[1] == 0x8b {
		gzipReader, err := gzip.NewReader(file)
		if err != nil {
			return nil, fmt.Errorf("failed to create gzip reader: %w", err)
		}
		defer gzipReader.Close()
		reader = gzipReader
	}

	// Decode just the metadata (we only need to read the beginning)
	var backupData struct {
		Metadata BackupMetadata `json:"metadata"`
	}

	decoder := json.NewDecoder(reader)
	if err := decoder.Decode(&backupData); err != nil {
		return nil, fmt.Errorf("failed to decode backup metadata: %w", err)
	}

	// Validate version
	if backupData.Metadata.Version != "1.0" {
		return nil, fmt.Errorf("unsupported backup version: %s", backupData.Metadata.Version)
	}

	return &backupData.Metadata, nil
}

// readBackupFile reads and parses a backup file
func readBackupFile(path string) (*BackupData, error) {
	// Open file
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open backup file: %w", err)
	}
	defer file.Close()

	// Detect if file is gzipped
	var reader io.Reader = file
	
	// Read first two bytes to check for gzip magic number
	header := make([]byte, 2)
	if _, err := file.Read(header); err != nil {
		return nil, fmt.Errorf("failed to read file header: %w", err)
	}
	
	// Reset to beginning
	if _, err := file.Seek(0, 0); err != nil {
		return nil, fmt.Errorf("failed to seek file: %w", err)
	}

	// Check for gzip magic number (0x1f 0x8b)
	if header[0] == 0x1f && header[1] == 0x8b {
		gzipReader, err := gzip.NewReader(file)
		if err != nil {
			return nil, fmt.Errorf("failed to create gzip reader: %w", err)
		}
		defer gzipReader.Close()
		reader = gzipReader
	}

	// Decode full backup data
	var backupData BackupData
	decoder := json.NewDecoder(reader)
	if err := decoder.Decode(&backupData); err != nil {
		return nil, fmt.Errorf("failed to decode backup data: %w", err)
	}

	return &backupData, nil
}

// ImportUserData imports data from a backup file into the specified user's table.
// This will REPLACE all existing data for the user. Use with caution.
// Set clearExisting to false to append rather than replace (documents with duplicate IDs will be updated).
func (s *RAGStore) ImportUserData(ctx context.Context, userID string, inputPath string, clearExisting bool) error {
	return s.ImportUserDataWithProgress(ctx, userID, inputPath, clearExisting, nil)
}

// ImportUserDataWithProgress imports data with progress reporting
func (s *RAGStore) ImportUserDataWithProgress(ctx context.Context, userID string, inputPath string, clearExisting bool, callback ProgressCallback) error {
	// Validate user ID
	if err := validateUserID(userID); err != nil {
		return err
	}

	var tracker *ProgressTracker
	if callback != nil {
		tracker = NewProgressTracker("validating", 100, callback)
	}

	// Validate backup file first
	metadata, err := ValidateBackupFile(inputPath)
	if err != nil {
		return fmt.Errorf("backup file validation failed: %w", err)
	}

	// Check embedding dimensions match
	if metadata.EmbeddingDim != s.embeddingDim {
		return fmt.Errorf("backup embedding dimension (%d) does not match store dimension (%d)",
			metadata.EmbeddingDim, s.embeddingDim)
	}

	if tracker != nil {
		tracker.Add(10)
		tracker.SetStage("reading")
		tracker.SetTotal(int64(metadata.DocumentCount + 20))
	}

	// Read backup file
	backupData, err := readBackupFile(inputPath)
	if err != nil {
		return fmt.Errorf("failed to read backup file: %w", err)
	}

	if tracker != nil {
		tracker.Add(10)
		tracker.SetStage("preparing")
	}

	// Clear existing data if requested
	if clearExisting {
		if tracker != nil {
			tracker.SetMessage("Clearing existing data")
		}

		exists, err := s.TableExists(ctx, userID)
		if err != nil {
			return fmt.Errorf("failed to check table existence: %w", err)
		}

		if exists {
			if err := s.ClearUserData(ctx, userID); err != nil {
				return fmt.Errorf("failed to clear existing data: %w", err)
			}
		}
	}

	// Convert backup documents to regular documents
	documents := make([]Document, len(backupData.Documents))
	for i, backupDoc := range backupData.Documents {
		documents[i] = Document{
			ID:           backupDoc.ID,
			Text:         backupDoc.Text,
			DocumentName: backupDoc.DocumentName,
			Embedding:    backupDoc.Embedding,
			Metadata:     backupDoc.Metadata,
		}
	}

	if tracker != nil {
		tracker.SetStage("importing")
		tracker.SetMessage(fmt.Sprintf("Importing %d documents", len(documents)))
	}

	// Import documents
	var importErr error
	if clearExisting {
		// Use AddDocuments for fresh import
		if callback != nil {
			// Create a wrapper callback that updates our main tracker
			importCallback := func(p *Progress) {
				if tracker != nil {
					// Map the insert progress to our overall progress
					if p.Stage == "inserting" {
						tracker.progress.Current = 20 + p.Current
						tracker.progress.Stage = "importing"
						callback(&tracker.progress)
					}
				}
			}
			importErr = s.AddDocumentsWithProgress(ctx, userID, documents, importCallback)
		} else {
			importErr = s.AddDocuments(ctx, userID, documents)
		}
	} else {
		// Use Upsert to merge with existing data
		if callback != nil {
			importCallback := func(p *Progress) {
				if tracker != nil {
					tracker.progress.Current = 20 + p.Current
					tracker.progress.Stage = "importing"
					callback(&tracker.progress)
				}
			}
			importErr = s.UpsertDocumentsWithProgress(ctx, userID, documents, importCallback)
		} else {
			importErr = s.UpsertDocuments(ctx, userID, documents)
		}
	}

	if importErr != nil {
		return fmt.Errorf("failed to import documents: %w", importErr)
	}

	if tracker != nil {
		tracker.Complete()
		tracker.SetMessage("Import complete")
	}

	s.logger.Printf("Successfully imported %d documents for user %s from %s", len(documents), userID, inputPath)
	return nil
}

// ImportOptions configures the import behavior
type ImportOptions struct {
	ClearExisting bool // If true, clear existing data before import
	ValidateOnly  bool // If true, only validate the backup file without importing
	SkipErrors    bool // If true, skip documents that fail validation and continue
}

// ImportUserDataWithOptions provides advanced import options
func (s *RAGStore) ImportUserDataWithOptions(ctx context.Context, userID string, inputPath string, opts *ImportOptions, callback ProgressCallback) error {
	if opts == nil {
		opts = &ImportOptions{ClearExisting: true}
	}

	// Validate only mode
	if opts.ValidateOnly {
		metadata, err := ValidateBackupFile(inputPath)
		if err != nil {
			return err
		}
		s.logger.Printf("Backup file is valid: %d documents, embedding dim: %d", metadata.DocumentCount, metadata.EmbeddingDim)
		return nil
	}

	// Regular import
	return s.ImportUserDataWithProgress(ctx, userID, inputPath, opts.ClearExisting, callback)
}

