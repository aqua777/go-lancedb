package rag

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/suite"
)

// StoreTestSuite tests RAGStore functionality
type StoreTestSuite struct {
	suite.Suite
	store  *RAGStore
	dbPath string
	ctx    context.Context
}

// SetupTest runs before each test
func (s *StoreTestSuite) SetupTest() {
	// Create temporary directory for test database
	tmpDir, err := os.MkdirTemp("", "rag_test_*")
	s.Require().NoError(err)
	s.dbPath = filepath.Join(tmpDir, "test.db")
	s.ctx = context.Background()

	// Create store with test configuration
	store, err := NewRAGStoreWithConfig(s.dbPath, 128, 100, &noopLogger{}, DefaultRetryConfig(), nil)
	s.Require().NoError(err)
	s.store = store
}

// TearDownTest runs after each test
func (s *StoreTestSuite) TearDownTest() {
	if s.store != nil {
		s.store.Close()
	}
	if s.dbPath != "" {
		os.RemoveAll(filepath.Dir(s.dbPath))
	}
}

// TestStoreTestSuite runs the store test suite
func TestStoreTestSuite(t *testing.T) {
	suite.Run(t, new(StoreTestSuite))
}

