// Package rag provides high-level RAG (Retrieval-Augmented Generation) operations
// for LanceDB, with per-user table isolation and automatic vector indexing.
package rag

import (
	"context"
	"fmt"
	"log"
	"regexp"
	"sync"

	"github.com/apache/arrow/go/v17/arrow"
	"github.com/aqua777/go-lancedb"
)

// Logger interface for logging RAG operations
type Logger interface {
	Printf(format string, v ...interface{})
	Println(v ...interface{})
}

// defaultLogger wraps the standard log package
type defaultLogger struct{}

func (d *defaultLogger) Printf(format string, v ...interface{}) {
	log.Printf(format, v...)
}

func (d *defaultLogger) Println(v ...interface{}) {
	log.Println(v...)
}

// noopLogger discards all log messages
type noopLogger struct{}

func (n *noopLogger) Printf(format string, v ...interface{}) {}
func (n *noopLogger) Println(v ...interface{})              {}

// IndexConfig defines vector index configuration options
type IndexConfig struct {
	IndexType     lancedb.IndexType      // Type of index (IVFPQ, etc.)
	Metric        lancedb.DistanceMetric // Distance metric (Cosine, L2, etc.)
	NumPartitions int                    // Number of IVF partitions (0 = auto)
	NumSubVectors int                    // Number of PQ sub-vectors (0 = auto)
	Replace       bool                   // Replace existing index
}

// DefaultIndexConfig returns sensible default index configuration
func DefaultIndexConfig() *IndexConfig {
	return &IndexConfig{
		IndexType:     lancedb.IndexTypeIVFPQ,
		Metric:        lancedb.DistanceMetricCosine,
		NumPartitions: 0, // auto-calculate
		NumSubVectors: 0, // auto-calculate
		Replace:       true,
	}
}

// RAGStore manages RAG operations with per-user table isolation
type RAGStore struct {
	conn               *lancedb.Connection
	dbPath             string
	embeddingDim       int
	maxBatchSize       int                     // maximum number of documents per batch insert
	maxDocumentsForBM25 int                    // maximum documents for BM25 keyword search (default: 10000)
	logger             Logger                  // logger for RAG operations
	retryConfig        *RetryConfig            // retry configuration for transient failures
	metrics            MetricsCollector        // metrics collector for monitoring
	indexConfigs       map[string]*IndexConfig // per-user index configurations
	indexCreated       map[string]bool         // track per-user table index status
	mu                 sync.RWMutex            // protect indexCreated and indexConfigs maps
	userLocks          map[string]*sync.Mutex  // per-user locks for concurrent write protection
	locksMu            sync.RWMutex            // protect userLocks map
}

// NewRAGStore creates a new RAG store with the specified database path and embedding dimension.
// Uses default configuration: batch size 1000, standard logging, default retry behavior, no metrics.
func NewRAGStore(dbPath string, embeddingDim int) (*RAGStore, error) {
	return NewRAGStoreWithConfig(dbPath, embeddingDim, 1000, &defaultLogger{}, DefaultRetryConfig(), nil)
}

// NewRAGStoreWithConfig creates a new RAG store with custom configuration.
// maxBatchSize controls how many documents are inserted in a single operation (prevents memory exhaustion).
// logger is used for logging operations; pass nil to disable logging.
// retryConfig controls retry behavior for transient failures; pass nil to disable retries.
// metrics is used for collecting operation metrics; pass nil to disable metrics collection.
func NewRAGStoreWithConfig(dbPath string, embeddingDim int, maxBatchSize int, logger Logger, retryConfig *RetryConfig, metrics MetricsCollector) (*RAGStore, error) {
	if embeddingDim <= 0 {
		return nil, fmt.Errorf("embedding dimension must be positive, got %d", embeddingDim)
	}
	if maxBatchSize <= 0 {
		return nil, fmt.Errorf("max batch size must be positive, got %d", maxBatchSize)
	}

	conn, err := lancedb.Connect(dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	if logger == nil {
		logger = &noopLogger{}
	}
	
	if metrics == nil {
		metrics = &noopMetrics{}
	}

	return &RAGStore{
		conn:                conn,
		dbPath:              dbPath,
		embeddingDim:        embeddingDim,
		maxBatchSize:        maxBatchSize,
		maxDocumentsForBM25: 10000, // default limit for BM25 to prevent memory exhaustion
		logger:              logger,
		retryConfig:         retryConfig,
		metrics:             metrics,
		indexConfigs:        make(map[string]*IndexConfig),
		indexCreated:        make(map[string]bool),
		userLocks:           make(map[string]*sync.Mutex),
	}, nil
}

// Close closes the database connection
func (s *RAGStore) Close() error {
	s.conn.Close()
	return nil
}

// userIDPattern defines valid characters for user IDs (alphanumeric, underscores, hyphens)
var userIDPattern = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

// validateUserID checks if a user ID is valid
func validateUserID(userID string) error {
	if userID == "" {
		return fmt.Errorf("user ID cannot be empty")
	}
	if len(userID) > 100 {
		return fmt.Errorf("user ID too long (max 100 characters)")
	}
	if !userIDPattern.MatchString(userID) {
		return fmt.Errorf("user ID contains invalid characters (only alphanumeric, underscores, and hyphens allowed)")
	}
	return nil
}

// getTableName returns the table name for a given user ID
func (s *RAGStore) getTableName(userID string) string {
	return fmt.Sprintf("rag_user_%s", userID)
}

// getOrCreateTable returns the table for a user, creating it if it doesn't exist
func (s *RAGStore) getOrCreateTable(userID string) (*lancedb.Table, error) {
	if err := validateUserID(userID); err != nil {
		return nil, err
	}
	
	tableName := s.getTableName(userID)

	// Try to open existing table first
	table, err := s.conn.OpenTable(tableName)
	if err == nil {
		return table, nil
	}

	// Table doesn't exist, create it with schema
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

	table, err = s.conn.CreateTableWithSchema(tableName, schema)
	if err != nil {
		return nil, fmt.Errorf("failed to create table %s: %w", tableName, err)
	}

	return table, nil
}

// ensureIndex creates a vector index on the embedding column if not already created.
// This uses double-checked locking for thread-safety and logs the operation.
func (s *RAGStore) ensureIndex(table *lancedb.Table, userID string) error {
	s.mu.RLock()
	if s.indexCreated[userID] {
		s.mu.RUnlock()
		return nil
	}
	config := s.indexConfigs[userID]
	s.mu.RUnlock()

	s.mu.Lock()
	defer s.mu.Unlock()

	// Double-check after acquiring write lock
	if s.indexCreated[userID] {
		return nil
	}

	// Use default config if none specified
	if config == nil {
		config = DefaultIndexConfig()
	}

	s.logger.Printf("Creating vector index for user %s with config: %+v", userID, config)

	// Create index with user's configuration
	indexOpts := &lancedb.IndexOptions{
		IndexType:     config.IndexType,
		Metric:        config.Metric,
		Replace:       config.Replace,
		NumPartitions: config.NumPartitions,
		NumSubVectors: config.NumSubVectors,
	}

	if err := table.CreateIndex("embedding", indexOpts); err != nil {
		s.logger.Printf("Failed to create index for user %s: %v", userID, err)
		return fmt.Errorf("failed to create index: %w", err)
	}

	s.indexCreated[userID] = true
	s.logger.Printf("Successfully created vector index for user %s", userID)
	return nil
}

// SetIndexConfig sets the index configuration for a specific user.
// This must be called before adding documents; it won't affect existing indexes.
// To rebuild with new config, clear the user's data first.
func (s *RAGStore) SetIndexConfig(userID string, config *IndexConfig) error {
	if err := validateUserID(userID); err != nil {
		return err
	}
	if config == nil {
		return fmt.Errorf("index config cannot be nil")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Check if index already exists
	if s.indexCreated[userID] {
		return fmt.Errorf("index already created for user %s; clear data and recreate to apply new config", userID)
	}

	s.indexConfigs[userID] = config
	s.logger.Printf("Set index config for user %s: %+v", userID, config)
	return nil
}

// GetIndexConfig returns the current index configuration for a user
func (s *RAGStore) GetIndexConfig(userID string) (*IndexConfig, error) {
	if err := validateUserID(userID); err != nil {
		return nil, err
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	if config, exists := s.indexConfigs[userID]; exists {
		return config, nil
	}

	return DefaultIndexConfig(), nil
}

// RebuildIndex rebuilds the index for a user with new configuration.
// This is an expensive operation that requires re-indexing all documents.
func (s *RAGStore) RebuildIndex(ctx context.Context, userID string, config *IndexConfig) error {
	if err := validateUserID(userID); err != nil {
		return err
	}

	// Check for context cancellation
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// Acquire user lock
	lock := s.getUserLock(userID)
	lock.Lock()
	defer lock.Unlock()

	table, err := s.conn.OpenTable(s.getTableName(userID))
	if err != nil {
		return fmt.Errorf("failed to open table: %w", err)
	}
	defer table.Close()

	s.mu.Lock()
	// Update config and mark index as not created
	s.indexConfigs[userID] = config
	s.indexCreated[userID] = false
	s.mu.Unlock()

	s.logger.Printf("Rebuilding index for user %s with new config: %+v", userID, config)

	// Create new index
	if err := s.ensureIndex(table, userID); err != nil {
		return fmt.Errorf("failed to rebuild index: %w", err)
	}

	s.logger.Printf("Successfully rebuilt index for user %s", userID)
	return nil
}

// TableExists checks if a table exists for the given user
func (s *RAGStore) TableExists(ctx context.Context, userID string) (bool, error) {
	if err := validateUserID(userID); err != nil {
		return false, err
	}
	
	// Check for context cancellation
	select {
	case <-ctx.Done():
		return false, ctx.Err()
	default:
	}
	
	tableNames, err := s.conn.TableNames()
	if err != nil {
		return false, fmt.Errorf("failed to list tables: %w", err)
	}

	targetName := s.getTableName(userID)
	for _, name := range tableNames {
		if name == targetName {
			return true, nil
		}
	}
	return false, nil
}

// GetEmbeddingDim returns the configured embedding dimension
func (s *RAGStore) GetEmbeddingDim() int {
	return s.embeddingDim
}

// SetMaxDocumentsForBM25 sets the maximum number of documents for BM25 keyword search.
// This limit prevents memory exhaustion when loading all documents for BM25 scoring.
// Default is 10,000. Set to 0 to disable the limit (not recommended for production).
func (s *RAGStore) SetMaxDocumentsForBM25(max int) {
	s.maxDocumentsForBM25 = max
}

// GetMaxDocumentsForBM25 returns the current BM25 document limit
func (s *RAGStore) GetMaxDocumentsForBM25() int {
	return s.maxDocumentsForBM25
}

// getUserLock returns the lock for a specific user, creating it if needed.
// This ensures concurrent writes to the same user's table are serialized.
func (s *RAGStore) getUserLock(userID string) *sync.Mutex {
	s.locksMu.RLock()
	lock, exists := s.userLocks[userID]
	s.locksMu.RUnlock()
	
	if exists {
		return lock
	}
	
	s.locksMu.Lock()
	defer s.locksMu.Unlock()
	
	// Double-check after acquiring write lock
	if lock, exists := s.userLocks[userID]; exists {
		return lock
	}
	
	lock = &sync.Mutex{}
	s.userLocks[userID] = lock
	return lock
}

// HealthStatus contains detailed health check information
type HealthStatus struct {
	Healthy        bool              // Overall health status
	DatabasePath   string            // Path to the database
	TablesCount    int               // Number of tables in the database
	Error          string            // Error message if unhealthy
	UserTableCount map[string]int64  // Document counts per user (sample)
}

// HealthCheck performs a lightweight health check on the database connection.
// Returns nil if healthy, error otherwise.
func (s *RAGStore) HealthCheck(ctx context.Context) error {
	// Check for context cancellation
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// Try to list tables (lightweight operation)
	_, err := s.conn.TableNames()
	if err != nil {
		return fmt.Errorf("health check failed: unable to list tables: %w", err)
	}

	return nil
}

// HealthCheckWithDetails performs a comprehensive health check with detailed diagnostics.
// This is more expensive than HealthCheck() but provides useful debugging information.
func (s *RAGStore) HealthCheckWithDetails(ctx context.Context) *HealthStatus {
	status := &HealthStatus{
		Healthy:        true,
		DatabasePath:   s.dbPath,
		UserTableCount: make(map[string]int64),
	}

	// Check for context cancellation
	select {
	case <-ctx.Done():
		status.Healthy = false
		status.Error = ctx.Err().Error()
		return status
	default:
	}

	// List all tables
	tableNames, err := s.conn.TableNames()
	if err != nil {
		status.Healthy = false
		status.Error = fmt.Sprintf("failed to list tables: %v", err)
		return status
	}

	status.TablesCount = len(tableNames)

	// Get document counts for user tables (sample up to 10 users)
	userTablePrefix := "rag_user_"
	sampleCount := 0
	for _, tableName := range tableNames {
		if len(tableName) > len(userTablePrefix) && tableName[:len(userTablePrefix)] == userTablePrefix {
			if sampleCount >= 10 {
				break // Limit sampling to avoid expensive operations
			}
			
			table, err := s.conn.OpenTable(tableName)
			if err != nil {
				continue // Skip tables that can't be opened
			}
			
			count, err := table.CountRows()
			table.Close()
			
			if err == nil {
				userID := tableName[len(userTablePrefix):]
				status.UserTableCount[userID] = count
				sampleCount++
			}
		}
	}

	return status
}

