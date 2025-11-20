package rag

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

// ChunkingTestSuite tests document chunking utilities
type ChunkingTestSuite struct {
	suite.Suite
}

// TestChunkingTestSuite runs the chunking test suite
func TestChunkingTestSuite(t *testing.T) {
	suite.Run(t, new(ChunkingTestSuite))
}

