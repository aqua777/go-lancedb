package rag

import (
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
)

// ProgressTestSuite tests the progress tracking functionality
type ProgressTestSuite struct {
	suite.Suite
}

func TestProgressSuite(t *testing.T) {
	suite.Run(t, new(ProgressTestSuite))
}

func (s *ProgressTestSuite) TestProgressPercent() {
	progress := &Progress{
		Current: 50,
		Total:   100,
	}

	s.Equal(50.0, progress.Percent())

	// Test zero total
	progress.Total = 0
	s.Equal(0.0, progress.Percent())
}

func (s *ProgressTestSuite) TestProgressElapsed() {
	start := time.Now().Add(-5 * time.Second)
	progress := &Progress{
		StartTime: start,
	}

	elapsed := progress.Elapsed()
	s.True(elapsed >= 5*time.Second)
	s.True(elapsed < 6*time.Second)
}

func (s *ProgressTestSuite) TestProgressEstimatedRemaining() {
	start := time.Now().Add(-10 * time.Second)
	progress := &Progress{
		StartTime: start,
		Current:   25,
		Total:     100,
	}

	// With 25% done in 10 seconds, should take ~30 more seconds
	remaining := progress.EstimatedRemaining()
	s.True(remaining >= 25*time.Second)
	s.True(remaining <= 35*time.Second)

	// Test zero progress
	progress.Current = 0
	s.Equal(time.Duration(0), progress.EstimatedRemaining())
}

func (s *ProgressTestSuite) TestProgressIsComplete() {
	progress := &Progress{
		Current: 100,
		Total:   100,
	}
	s.True(progress.IsComplete())

	progress.Current = 50
	s.False(progress.IsComplete())

	progress.Total = 0
	s.False(progress.IsComplete())
}

func (s *ProgressTestSuite) TestProgressTracker() {
	callCount := 0
	var lastProgress *Progress

	callback := func(p *Progress) {
		callCount++
		lastProgress = &Progress{
			Stage:   p.Stage,
			Current: p.Current,
			Total:   p.Total,
			Message: p.Message,
		}
	}

	tracker := NewProgressTracker("testing", 100, callback)
	s.Equal(1, callCount) // Initial callback

	// Increment progress
	tracker.Increment()
	s.Equal(int64(1), lastProgress.Current)

	// Add multiple
	tracker.Add(50)
	s.Equal(int64(51), lastProgress.Current)

	// Set stage
	tracker.SetStage("processing")
	s.Equal("processing", lastProgress.Stage)

	// Set message
	tracker.SetMessage("test message")
	s.Equal("test message", lastProgress.Message)

	// Complete
	tracker.Complete()
	s.Equal(int64(100), lastProgress.Current)
	s.True(lastProgress.IsComplete())
}

func (s *ProgressTestSuite) TestProgressTrackerNilCallback() {
	// Should not panic with nil callback
	tracker := NewProgressTracker("testing", 100, nil)
	s.NotNil(tracker)

	tracker.Increment()
	tracker.Add(10)
	tracker.SetStage("test")
	tracker.SetMessage("test")
	tracker.Complete()
}

func (s *ProgressTestSuite) TestProgressTrackerSetTotal() {
	var lastProgress *Progress
	callback := func(p *Progress) {
		lastProgress = &Progress{
			Total: p.Total,
		}
	}

	tracker := NewProgressTracker("testing", 50, callback)
	s.Equal(int64(50), lastProgress.Total)

	tracker.SetTotal(100)
	s.Equal(int64(100), lastProgress.Total)
}

func (s *ProgressTestSuite) TestProgressTrackerGetProgress() {
	tracker := NewProgressTracker("testing", 100, nil)
	tracker.Add(25)

	progress := tracker.GetProgress()
	s.Equal(int64(25), progress.Current)
	s.Equal(int64(100), progress.Total)
	s.Equal("testing", progress.Stage)
}

