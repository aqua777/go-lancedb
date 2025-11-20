package rag

import (
	"sync/atomic"
	"time"
)

// ProgressCallback is called during long-running operations to report progress.
// Implementations should be non-blocking and fast to avoid slowing down operations.
type ProgressCallback func(progress *Progress)

// Progress represents the current state of a long-running operation
type Progress struct {
	Stage       string    // Current operation stage (e.g., "chunking", "embedding", "inserting")
	Current     int64     // Number of items completed
	Total       int64     // Total number of items to process
	Message     string    // Optional status message
	StartTime   time.Time // When the operation started
	LastUpdated time.Time // Last progress update time
}

// Percent returns the completion percentage (0-100)
func (p *Progress) Percent() float64 {
	if p.Total == 0 {
		return 0
	}
	return (float64(p.Current) / float64(p.Total)) * 100
}

// Elapsed returns the time elapsed since the operation started
func (p *Progress) Elapsed() time.Duration {
	return time.Since(p.StartTime)
}

// EstimatedRemaining estimates the time remaining based on current progress.
// Returns 0 if unable to estimate (no progress yet, or operation just started).
func (p *Progress) EstimatedRemaining() time.Duration {
	if p.Current == 0 || p.Total == 0 {
		return 0
	}
	
	elapsed := p.Elapsed()
	rate := float64(p.Current) / elapsed.Seconds()
	if rate == 0 {
		return 0
	}
	
	remaining := p.Total - p.Current
	secondsRemaining := float64(remaining) / rate
	return time.Duration(secondsRemaining * float64(time.Second))
}

// IsComplete returns true if the operation is complete
func (p *Progress) IsComplete() bool {
	return p.Current >= p.Total && p.Total > 0
}

// ProgressTracker tracks progress for a single operation in a thread-safe manner.
// Use this to implement progress reporting in your functions.
type ProgressTracker struct {
	progress Progress
	callback ProgressCallback
	current  atomic.Int64
	total    atomic.Int64
}

// NewProgressTracker creates a new progress tracker
func NewProgressTracker(stage string, total int64, callback ProgressCallback) *ProgressTracker {
	now := time.Now()
	tracker := &ProgressTracker{
		progress: Progress{
			Stage:       stage,
			Current:     0,
			Total:       total,
			StartTime:   now,
			LastUpdated: now,
		},
		callback: callback,
	}
	tracker.current.Store(0)
	tracker.total.Store(total)
	
	// Report initial state
	if callback != nil {
		callback(&tracker.progress)
	}
	
	return tracker
}

// Increment increments the progress by 1 and reports if enough time has passed.
// This is optimized to avoid excessive callback invocations.
func (t *ProgressTracker) Increment() {
	t.Add(1)
}

// Add increments the progress by the specified amount
func (t *ProgressTracker) Add(delta int64) {
	current := t.current.Add(delta)
	
	// Update progress struct
	now := time.Now()
	t.progress.Current = current
	t.progress.LastUpdated = now
	
	// Report progress (with throttling to avoid excessive callbacks)
	if t.callback != nil {
		// Always report to ensure callbacks receive updates
		t.callback(&t.progress)
	}
}

// SetTotal updates the total count (useful for operations where total is unknown initially)
func (t *ProgressTracker) SetTotal(total int64) {
	t.total.Store(total)
	t.progress.Total = total
	
	if t.callback != nil {
		t.callback(&t.progress)
	}
}

// SetStage changes the current stage of the operation
func (t *ProgressTracker) SetStage(stage string) {
	t.progress.Stage = stage
	t.progress.LastUpdated = time.Now()
	
	if t.callback != nil {
		t.callback(&t.progress)
	}
}

// SetMessage sets a status message
func (t *ProgressTracker) SetMessage(message string) {
	t.progress.Message = message
	t.progress.LastUpdated = time.Now()
	
	if t.callback != nil {
		t.callback(&t.progress)
	}
}

// Complete marks the operation as complete and reports final progress
func (t *ProgressTracker) Complete() {
	t.progress.Current = t.progress.Total
	t.progress.LastUpdated = time.Now()
	
	if t.callback != nil {
		t.callback(&t.progress)
	}
}

// GetProgress returns a copy of the current progress
func (t *ProgressTracker) GetProgress() Progress {
	return Progress{
		Stage:       t.progress.Stage,
		Current:     t.current.Load(),
		Total:       t.total.Load(),
		Message:     t.progress.Message,
		StartTime:   t.progress.StartTime,
		LastUpdated: t.progress.LastUpdated,
	}
}

