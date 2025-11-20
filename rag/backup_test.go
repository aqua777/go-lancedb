package rag

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/suite"
)

// BackupTestSuite tests the backup and restore functionality
type BackupTestSuite struct {
	suite.Suite
	tmpDir  string
	store   *RAGStore
	userID  string
	ctx     context.Context
}

func TestBackupSuite(t *testing.T) {
	suite.Run(t, new(BackupTestSuite))
}

func (s *BackupTestSuite) SetupTest() {
	// Create temporary directory
	tmpDir, err := os.MkdirTemp("", "rag_backup_test_*")
	s.Require().NoError(err)
	s.tmpDir = tmpDir

	// Create RAG store
	dbPath := filepath.Join(tmpDir, "test.db")
	store, err := NewRAGStore(dbPath, 128)
	s.Require().NoError(err)
	s.store = store

	s.userID = "testuser"
	s.ctx = context.Background()
}

func (s *BackupTestSuite) TearDownTest() {
	if s.store != nil {
		s.store.Close()
	}
	if s.tmpDir != "" {
		os.RemoveAll(s.tmpDir)
	}
}

func (s *BackupTestSuite) TestExportUserDataJSON() {
	// Add some test documents (need 256+ for index to work)
	docs := make([]Document, 300)
	for i := 0; i < 300; i++ {
		docs[i] = Document{
			ID:           fmt.Sprintf("doc%d", i),
			Text:         fmt.Sprintf("test document %d", i),
			DocumentName: "test.txt",
			Embedding:    make([]float32, 128),
			Metadata:     map[string]interface{}{"key": fmt.Sprintf("value%d", i)},
		}
		// Fill embeddings with test values
		for j := range docs[i].Embedding {
			docs[i].Embedding[j] = float32(i + j)
		}
	}

	err := s.store.AddDocuments(s.ctx, s.userID, docs)
	s.Require().NoError(err)

	// Export to JSON
	backupPath := filepath.Join(s.tmpDir, "backup.json")
	err = s.store.ExportUserData(s.ctx, s.userID, backupPath, BackupFormatJSON)
	s.NoError(err)

	// Check that backup file exists
	stat, err := os.Stat(backupPath)
	s.NoError(err)
	s.True(stat.Size() > 0)
}

func (s *BackupTestSuite) TestExportUserDataJSONGzip() {
	// Add test documents (need 256+ for index to work)
	docs := make([]Document, 300)
	for i := 0; i < 300; i++ {
		docs[i] = Document{
			ID:           fmt.Sprintf("doc%d", i),
			Text:         fmt.Sprintf("test document %d", i),
			DocumentName: "test.txt",
			Embedding:    make([]float32, 128),
			Metadata:     map[string]interface{}{"key": fmt.Sprintf("value%d", i)},
		}
		for j := range docs[i].Embedding {
			docs[i].Embedding[j] = float32(i + j)
		}
	}

	err := s.store.AddDocuments(s.ctx, s.userID, docs)
	s.Require().NoError(err)

	// Export to compressed JSON
	backupPath := filepath.Join(s.tmpDir, "backup.json.gz")
	err = s.store.ExportUserData(s.ctx, s.userID, backupPath, BackupFormatJSONGzip)
	s.NoError(err)

	// Check that backup file exists and is compressed (smaller than uncompressed)
	stat, err := os.Stat(backupPath)
	s.NoError(err)
	s.True(stat.Size() > 0)

	// Verify it's actually gzipped by checking magic number
	file, err := os.Open(backupPath)
	s.NoError(err)
	defer file.Close()

	header := make([]byte, 2)
	_, err = file.Read(header)
	s.NoError(err)
	s.Equal(byte(0x1f), header[0])
	s.Equal(byte(0x8b), header[1])
}

func (s *BackupTestSuite) TestValidateBackupFile() {
	// Add and export documents (need 256+ for index to work)
	docs := make([]Document, 300)
	for i := 0; i < 300; i++ {
		docs[i] = Document{
			ID:           fmt.Sprintf("doc%d", i),
			Text:         fmt.Sprintf("test document %d", i),
			DocumentName: "test.txt",
			Embedding:    make([]float32, 128),
			Metadata:     map[string]interface{}{"key": "value"},
		}
		for j := range docs[i].Embedding {
			docs[i].Embedding[j] = float32(i + j)
		}
	}

	err := s.store.AddDocuments(s.ctx, s.userID, docs)
	s.Require().NoError(err)

	backupPath := filepath.Join(s.tmpDir, "backup.json")
	err = s.store.ExportUserData(s.ctx, s.userID, backupPath, BackupFormatJSON)
	s.Require().NoError(err)

	// Validate backup file
	metadata, err := ValidateBackupFile(backupPath)
	s.NoError(err)
	s.NotNil(metadata)
	s.Equal("1.0", metadata.Version)
	s.Equal(s.userID, metadata.UserID)
	s.Equal(300, metadata.DocumentCount)
	s.Equal(128, metadata.EmbeddingDim)
}

func (s *BackupTestSuite) TestValidateBackupFileGzipped() {
	// Add and export documents (need 256+ for index to work)
	docs := make([]Document, 300)
	for i := 0; i < 300; i++ {
		docs[i] = Document{
			ID:           fmt.Sprintf("doc%d", i),
			Text:         fmt.Sprintf("test document %d", i),
			DocumentName: "test.txt",
			Embedding:    make([]float32, 128),
			Metadata:     map[string]interface{}{"key": "value"},
		}
		for j := range docs[i].Embedding {
			docs[i].Embedding[j] = float32(i + j)
		}
	}

	err := s.store.AddDocuments(s.ctx, s.userID, docs)
	s.Require().NoError(err)

	backupPath := filepath.Join(s.tmpDir, "backup.json.gz")
	err = s.store.ExportUserData(s.ctx, s.userID, backupPath, BackupFormatJSONGzip)
	s.Require().NoError(err)

	// Validate compressed backup file
	metadata, err := ValidateBackupFile(backupPath)
	s.NoError(err)
	s.NotNil(metadata)
	s.Equal("1.0", metadata.Version)
	s.Equal(s.userID, metadata.UserID)
}

func (s *BackupTestSuite) TestImportUserData() {
	// Add and export documents (need 256+ for index to work)
	originalDocs := make([]Document, 300)
	for i := 0; i < 300; i++ {
		originalDocs[i] = Document{
			ID:           fmt.Sprintf("doc%d", i),
			Text:         fmt.Sprintf("test document %d", i),
			DocumentName: "test.txt",
			Embedding:    make([]float32, 128),
			Metadata:     map[string]interface{}{"key": fmt.Sprintf("value%d", i)},
		}
		for j := range originalDocs[i].Embedding {
			originalDocs[i].Embedding[j] = float32(i + j)
		}
	}

	err := s.store.AddDocuments(s.ctx, s.userID, originalDocs)
	s.Require().NoError(err)

	backupPath := filepath.Join(s.tmpDir, "backup.json")
	err = s.store.ExportUserData(s.ctx, s.userID, backupPath, BackupFormatJSON)
	s.Require().NoError(err)

	// Clear existing data
	err = s.store.ClearUserData(s.ctx, s.userID)
	s.Require().NoError(err)

	// Verify data is cleared
	count, err := s.store.CountDocuments(s.ctx, s.userID)
	s.NoError(err)
	s.Equal(int64(0), count)

	// Import from backup
	err = s.store.ImportUserData(s.ctx, s.userID, backupPath, true)
	s.NoError(err)

	// Verify data is restored
	count, err = s.store.CountDocuments(s.ctx, s.userID)
	s.NoError(err)
	s.Equal(int64(300), count)

	// Verify document content by searching
	// Create a query embedding
	queryEmb := make([]float32, 128)
	for i := range queryEmb {
		queryEmb[i] = float32(i)
	}

	results, err := s.store.Search(s.ctx, s.userID, queryEmb, &SearchOptions{
		Limit: 10,
	})
	s.NoError(err)
	s.Len(results, 10)
}

func (s *BackupTestSuite) TestImportUserDataGzipped() {
	// Add and export documents (need 256+ for index to work)
	docs := make([]Document, 300)
	for i := 0; i < 300; i++ {
		docs[i] = Document{
			ID:           fmt.Sprintf("doc%d", i),
			Text:         fmt.Sprintf("test document %d", i),
			DocumentName: "test.txt",
			Embedding:    make([]float32, 128),
			Metadata:     map[string]interface{}{"key": "value"},
		}
		for j := range docs[i].Embedding {
			docs[i].Embedding[j] = float32(i + j)
		}
	}

	err := s.store.AddDocuments(s.ctx, s.userID, docs)
	s.Require().NoError(err)

	backupPath := filepath.Join(s.tmpDir, "backup.json.gz")
	err = s.store.ExportUserData(s.ctx, s.userID, backupPath, BackupFormatJSONGzip)
	s.Require().NoError(err)

	// Clear and reimport
	err = s.store.ClearUserData(s.ctx, s.userID)
	s.Require().NoError(err)

	err = s.store.ImportUserData(s.ctx, s.userID, backupPath, true)
	s.NoError(err)

	// Verify restoration
	count, err := s.store.CountDocuments(s.ctx, s.userID)
	s.NoError(err)
	s.Equal(int64(300), count)
}

func (s *BackupTestSuite) TestImportUserDataWithProgress() {
	// Add documents (need 256+ for index to work)
	docs := make([]Document, 300)
	for i := 0; i < 300; i++ {
		docs[i] = Document{
			ID:           fmt.Sprintf("doc%d", i),
			Text:         fmt.Sprintf("test document %d", i),
			DocumentName: "test.txt",
			Embedding:    make([]float32, 128),
			Metadata:     map[string]interface{}{"key": "value"},
		}
		for j := range docs[i].Embedding {
			docs[i].Embedding[j] = float32(i + j)
		}
	}

	err := s.store.AddDocuments(s.ctx, s.userID, docs)
	s.Require().NoError(err)

	backupPath := filepath.Join(s.tmpDir, "backup.json")
	err = s.store.ExportUserData(s.ctx, s.userID, backupPath, BackupFormatJSON)
	s.Require().NoError(err)

	// Clear data
	err = s.store.ClearUserData(s.ctx, s.userID)
	s.Require().NoError(err)

	// Import with progress callback
	progressCalled := false
	callback := func(p *Progress) {
		progressCalled = true
	}

	err = s.store.ImportUserDataWithProgress(s.ctx, s.userID, backupPath, true, callback)
	s.NoError(err)
	s.True(progressCalled)

	// Verify restoration
	count, err := s.store.CountDocuments(s.ctx, s.userID)
	s.NoError(err)
	s.Equal(int64(300), count)
}

func (s *BackupTestSuite) TestExportUserDataNonExistentUser() {
	backupPath := filepath.Join(s.tmpDir, "backup.json")
	err := s.store.ExportUserData(s.ctx, "nonexistentuser", backupPath, BackupFormatJSON)
	s.Error(err)
}

func (s *BackupTestSuite) TestImportUserDataDimensionMismatch() {
	// Create a backup with different embedding dimension
	// This would require manually creating a backup file or using a different store
	// For simplicity, we'll test with validation

	backupPath := filepath.Join(s.tmpDir, "invalid_backup.json")
	
	// Create a fake backup with wrong dimension
	backupData := BackupData{
		Metadata: BackupMetadata{
			Version:       "1.0",
			UserID:        s.userID,
			DocumentCount: 1,
			EmbeddingDim:  256, // Wrong dimension
		},
		Documents: []BackupDocument{
			{
				ID:           "doc1",
				Text:         "test",
				DocumentName: "test.txt",
				Embedding:    make([]float32, 256),
			},
		},
	}

	err := writeBackupFile(backupPath, backupData, BackupFormatJSON)
	s.Require().NoError(err)

	// Try to import - should fail due to dimension mismatch
	err = s.store.ImportUserData(s.ctx, s.userID, backupPath, true)
	s.Error(err)
	s.Contains(err.Error(), "dimension")
}

