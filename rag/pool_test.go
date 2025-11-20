package rag

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/suite"
)

// PoolTestSuite tests connection pooling
type PoolTestSuite struct {
	suite.Suite
	pool   *ConnectionPool
	dbPath string
}

// SetupTest runs before each test
func (s *PoolTestSuite) SetupTest() {
	tmpDir, err := os.MkdirTemp("", "rag_pool_test_*")
	s.Require().NoError(err)
	s.dbPath = filepath.Join(tmpDir, "test.db")
}

// TearDownTest runs after each test
func (s *PoolTestSuite) TearDownTest() {
	if s.pool != nil {
		s.pool.Close()
	}
	if s.dbPath != "" {
		os.RemoveAll(filepath.Dir(s.dbPath))
	}
}

// TestPoolTestSuite runs the pool test suite
func TestPoolTestSuite(t *testing.T) {
	suite.Run(t, new(PoolTestSuite))
}

