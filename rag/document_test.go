package rag

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/suite"
)

// DocumentTestSuite tests document operations
type DocumentTestSuite struct {
	suite.Suite
	store  *RAGStore
	dbPath string
	ctx    context.Context
}

// SetupTest runs before each test
func (s *DocumentTestSuite) SetupTest() {
	tmpDir, err := os.MkdirTemp("", "rag_doc_test_*")
	s.Require().NoError(err)
	s.dbPath = filepath.Join(tmpDir, "test.db")
	s.ctx = context.Background()

	store, err := NewRAGStoreWithConfig(s.dbPath, 128, 100, &noopLogger{}, DefaultRetryConfig(), nil)
	s.Require().NoError(err)
	s.store = store
}

// TearDownTest runs after each test
func (s *DocumentTestSuite) TearDownTest() {
	if s.store != nil {
		s.store.Close()
	}
	if s.dbPath != "" {
		os.RemoveAll(filepath.Dir(s.dbPath))
	}
}

// TestDocumentTestSuite runs the document test suite
func TestDocumentTestSuite(t *testing.T) {
	suite.Run(t, new(DocumentTestSuite))
}

