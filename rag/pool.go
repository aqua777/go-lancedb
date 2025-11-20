package rag

import (
	"fmt"
	"sync"

	"github.com/aqua777/go-lancedb"
)

// ConnectionPool manages a pool of LanceDB connections for efficiency
type ConnectionPool struct {
	dbPath       string
	maxSize      int
	connections  []*lancedb.Connection
	available    chan *lancedb.Connection
	mu           sync.Mutex
	closed       bool
}

// NewConnectionPool creates a new connection pool
func NewConnectionPool(dbPath string, maxSize int) (*ConnectionPool, error) {
	if maxSize <= 0 {
		return nil, fmt.Errorf("max pool size must be positive, got %d", maxSize)
	}

	pool := &ConnectionPool{
		dbPath:      dbPath,
		maxSize:     maxSize,
		connections: make([]*lancedb.Connection, 0, maxSize),
		available:   make(chan *lancedb.Connection, maxSize),
	}

	// Pre-create connections up to maxSize
	for i := 0; i < maxSize; i++ {
		conn, err := lancedb.Connect(dbPath)
		if err != nil {
			// Close any connections we've already created
			pool.Close()
			return nil, fmt.Errorf("failed to create connection %d: %w", i, err)
		}
		pool.connections = append(pool.connections, conn)
		pool.available <- conn
	}

	return pool, nil
}

// Get retrieves a connection from the pool (blocks if none available)
func (p *ConnectionPool) Get() (*lancedb.Connection, error) {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return nil, fmt.Errorf("connection pool is closed")
	}
	p.mu.Unlock()

	conn := <-p.available
	return conn, nil
}

// Put returns a connection to the pool
func (p *ConnectionPool) Put(conn *lancedb.Connection) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return fmt.Errorf("connection pool is closed")
	}

	select {
	case p.available <- conn:
		return nil
	default:
		return fmt.Errorf("connection pool is full")
	}
}

// Close closes all connections in the pool
func (p *ConnectionPool) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return nil
	}

	p.closed = true
	close(p.available)

	// Close all connections
	for _, conn := range p.connections {
		conn.Close()
	}

	return nil
}

// Size returns the current number of connections in the pool
func (p *ConnectionPool) Size() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.connections)
}

// Available returns the number of available connections
func (p *ConnectionPool) Available() int {
	return len(p.available)
}

// PooledRAGStore is a RAGStore that uses a shared connection pool
type PooledRAGStore struct {
	*RAGStore
	pool *ConnectionPool
}

// NewPooledRAGStore creates a RAG store that uses a shared connection pool.
// This is more efficient when you have multiple RAGStore instances accessing the same database.
func NewPooledRAGStore(pool *ConnectionPool, embeddingDim int, maxBatchSize int, logger Logger, retryConfig *RetryConfig, metrics MetricsCollector) (*PooledRAGStore, error) {
	if embeddingDim <= 0 {
		return nil, fmt.Errorf("embedding dimension must be positive, got %d", embeddingDim)
	}
	if maxBatchSize <= 0 {
		return nil, fmt.Errorf("max batch size must be positive, got %d", maxBatchSize)
	}

	// Get a connection from the pool to validate
	conn, err := pool.Get()
	if err != nil {
		return nil, fmt.Errorf("failed to get connection from pool: %w", err)
	}

	if logger == nil {
		logger = &noopLogger{}
	}
	
	if metrics == nil {
		metrics = &noopMetrics{}
	}

	store := &RAGStore{
		conn:                conn,
		dbPath:              pool.dbPath,
		embeddingDim:        embeddingDim,
		maxBatchSize:        maxBatchSize,
		maxDocumentsForBM25: 10000, // default limit for BM25
		logger:              logger,
		retryConfig:         retryConfig,
		metrics:             metrics,
		indexConfigs:        make(map[string]*IndexConfig),
		indexCreated:        make(map[string]bool),
		userLocks:           make(map[string]*sync.Mutex),
	}

	return &PooledRAGStore{
		RAGStore: store,
		pool:     pool,
	}, nil
}

// Close returns the connection to the pool instead of closing it
func (s *PooledRAGStore) Close() error {
	if s.conn != nil {
		return s.pool.Put(s.conn)
	}
	return nil
}

// GlobalConnectionPool is a singleton connection pool for convenience
var (
	globalPool   *ConnectionPool
	globalPoolMu sync.Mutex
)

// InitGlobalPool initializes the global connection pool
func InitGlobalPool(dbPath string, maxSize int) error {
	globalPoolMu.Lock()
	defer globalPoolMu.Unlock()

	if globalPool != nil {
		return fmt.Errorf("global pool already initialized")
	}

	pool, err := NewConnectionPool(dbPath, maxSize)
	if err != nil {
		return fmt.Errorf("failed to create global pool: %w", err)
	}

	globalPool = pool
	return nil
}

// GetGlobalPool returns the global connection pool
func GetGlobalPool() (*ConnectionPool, error) {
	globalPoolMu.Lock()
	defer globalPoolMu.Unlock()

	if globalPool == nil {
		return nil, fmt.Errorf("global pool not initialized; call InitGlobalPool first")
	}

	return globalPool, nil
}

// CloseGlobalPool closes the global connection pool
func CloseGlobalPool() error {
	globalPoolMu.Lock()
	defer globalPoolMu.Unlock()

	if globalPool == nil {
		return nil
	}

	err := globalPool.Close()
	globalPool = nil
	return err
}

// HealthCheck performs a basic health check on the connection pool.
// Verifies that the pool is not closed and has available connections.
func (p *ConnectionPool) HealthCheck() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return fmt.Errorf("connection pool is closed")
	}

	if len(p.connections) == 0 {
		return fmt.Errorf("connection pool has no connections")
	}

	return nil
}

// HealthCheckWithConnection performs a health check that also tests a connection.
// This is more thorough but temporarily reserves a connection from the pool.
func (p *ConnectionPool) HealthCheckWithConnection() error {
	// First check pool status
	if err := p.HealthCheck(); err != nil {
		return err
	}

	// Try to get a connection
	conn, err := p.Get()
	if err != nil {
		return fmt.Errorf("failed to get connection from pool: %w", err)
	}
	defer p.Put(conn)

	// Try a simple operation to verify the connection works
	_, err = conn.TableNames()
	if err != nil {
		return fmt.Errorf("connection test failed: %w", err)
	}

	return nil
}

