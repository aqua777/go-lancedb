package rag

import (
	"sync"
	"time"
)

// MetricsCollector is an interface for collecting metrics from RAG operations.
// Implement this interface to integrate with your monitoring system (Prometheus, Datadog, etc.)
type MetricsCollector interface {
	// RecordOperation records the duration and outcome of an operation
	RecordOperation(operation string, duration time.Duration, success bool)
	
	// RecordDocumentCount records the number of documents in an operation
	RecordDocumentCount(operation string, count int)
	
	// RecordSearchResults records the number of results returned by a search
	RecordSearchResults(count int)
	
	// RecordError records an error occurrence
	RecordError(operation string, errorType string)
}

// noopMetrics is a no-op implementation of MetricsCollector
type noopMetrics struct{}

func (n *noopMetrics) RecordOperation(operation string, duration time.Duration, success bool) {}
func (n *noopMetrics) RecordDocumentCount(operation string, count int)                        {}
func (n *noopMetrics) RecordSearchResults(count int)                                          {}
func (n *noopMetrics) RecordError(operation string, errorType string)                         {}

// simpleMetrics is a basic in-memory metrics collector for development/debugging.
// Thread-safe for concurrent use.
type simpleMetrics struct {
	mu         sync.RWMutex
	operations map[string]*operationStats
}

type operationStats struct {
	count         int64
	totalDuration time.Duration
	successCount  int64
	errorCount    int64
}

// NewSimpleMetrics creates a basic in-memory metrics collector
func NewSimpleMetrics() MetricsCollector {
	return &simpleMetrics{
		operations: make(map[string]*operationStats),
	}
}

func (m *simpleMetrics) RecordOperation(operation string, duration time.Duration, success bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	stats, exists := m.operations[operation]
	if !exists {
		stats = &operationStats{}
		m.operations[operation] = stats
	}
	
	stats.count++
	stats.totalDuration += duration
	if success {
		stats.successCount++
	} else {
		stats.errorCount++
	}
}

func (m *simpleMetrics) RecordDocumentCount(operation string, count int) {
	// Simple implementation - just track as operation
	m.RecordOperation(operation+"_docs", 0, true)
}

func (m *simpleMetrics) RecordSearchResults(count int) {
	// Track search result counts
	m.RecordOperation("search_results", time.Duration(count), true)
}

func (m *simpleMetrics) RecordError(operation string, errorType string) {
	m.RecordOperation(operation+"_error_"+errorType, 0, false)
}

// GetStats returns a copy of the collected statistics (for debugging).
// Returns a copy to prevent external modifications and race conditions.
func (m *simpleMetrics) GetStats() map[string]*operationStats {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	// Return a deep copy to prevent races
	statsCopy := make(map[string]*operationStats, len(m.operations))
	for key, stats := range m.operations {
		statsCopy[key] = &operationStats{
			count:         stats.count,
			totalDuration: stats.totalDuration,
			successCount:  stats.successCount,
			errorCount:    stats.errorCount,
		}
	}
	return statsCopy
}

// metricsTimer is a helper to track operation duration
type metricsTimer struct {
	collector MetricsCollector
	operation string
	startTime time.Time
}

// newMetricsTimer creates a new timer for an operation
func newMetricsTimer(collector MetricsCollector, operation string) *metricsTimer {
	return &metricsTimer{
		collector: collector,
		operation: operation,
		startTime: time.Now(),
	}
}

// recordSuccess records a successful operation completion
func (t *metricsTimer) recordSuccess() {
	if t.collector != nil {
		t.collector.RecordOperation(t.operation, time.Since(t.startTime), true)
	}
}

// recordError records a failed operation completion
func (t *metricsTimer) recordError(errorType string) {
	if t.collector != nil {
		t.collector.RecordOperation(t.operation, time.Since(t.startTime), false)
		t.collector.RecordError(t.operation, errorType)
	}
}

