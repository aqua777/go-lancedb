package rag

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/suite"
)

// QueryTestSuite tests query and search operations
type QueryTestSuite struct {
	suite.Suite
	store  *RAGStore
	dbPath string
	ctx    context.Context
}

// SetupTest runs before each test
func (s *QueryTestSuite) SetupTest() {
	tmpDir, err := os.MkdirTemp("", "rag_query_test_*")
	s.Require().NoError(err)
	s.dbPath = filepath.Join(tmpDir, "test.db")
	s.ctx = context.Background()

	store, err := NewRAGStoreWithConfig(s.dbPath, 128, 100, &noopLogger{}, DefaultRetryConfig(), nil)
	s.Require().NoError(err)
	s.store = store
}

// TearDownTest runs after each test
func (s *QueryTestSuite) TearDownTest() {
	if s.store != nil {
		s.store.Close()
	}
	if s.dbPath != "" {
		os.RemoveAll(filepath.Dir(s.dbPath))
	}
}

// TestQueryTestSuite runs the query test suite
func TestQueryTestSuite(t *testing.T) {
	suite.Run(t, new(QueryTestSuite))
}

