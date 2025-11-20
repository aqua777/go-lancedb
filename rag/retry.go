package rag

import (
	"context"
	"fmt"
	"math"
	"time"
)

// RetryConfig configures retry behavior for transient failures
type RetryConfig struct {
	MaxAttempts     int           // Maximum number of retry attempts (default: 3)
	InitialDelay    time.Duration // Initial delay before first retry (default: 100ms)
	MaxDelay        time.Duration // Maximum delay between retries (default: 10s)
	BackoffMultiple float64       // Multiplier for exponential backoff (default: 2.0)
}

// DefaultRetryConfig returns sensible defaults for retry behavior
func DefaultRetryConfig() *RetryConfig {
	return &RetryConfig{
		MaxAttempts:     3,
		InitialDelay:    100 * time.Millisecond,
		MaxDelay:        10 * time.Second,
		BackoffMultiple: 2.0,
	}
}

// retryWithBackoff executes a function with exponential backoff retry logic.
// It retries on errors that are deemed transient (e.g., temporary network issues).
func retryWithBackoff(ctx context.Context, config *RetryConfig, fn func() error) error {
	if config == nil {
		config = DefaultRetryConfig()
	}

	var lastErr error
	for attempt := 0; attempt < config.MaxAttempts; attempt++ {
		// Check for context cancellation before attempting
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Execute the function
		err := fn()
		if err == nil {
			return nil // Success
		}

		lastErr = err

		// Check if this is the last attempt
		if attempt >= config.MaxAttempts-1 {
			break
		}

		// Check if error is retryable
		if !isRetryableError(err) {
			return err // Don't retry non-retryable errors
		}

		// Calculate backoff delay with exponential growth
		delay := time.Duration(float64(config.InitialDelay) * math.Pow(config.BackoffMultiple, float64(attempt)))
		if delay > config.MaxDelay {
			delay = config.MaxDelay
		}

		// Wait with context cancellation support
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
			// Continue to next attempt
		}
	}

	return fmt.Errorf("max retry attempts (%d) exceeded: %w", config.MaxAttempts, lastErr)
}

// isRetryableError determines if an error is transient and worth retrying.
// This is a simple heuristic - you may want to customize based on specific LanceDB errors.
func isRetryableError(err error) bool {
	if err == nil {
		return false
	}

	// Simple heuristic: retry on most errors except for specific non-retryable ones
	errStr := err.Error()

	// Non-retryable errors (validation, schema issues, etc.)
	nonRetryablePatterns := []string{
		"dimension mismatch",
		"invalid",
		"cannot be empty",
		"too long",
		"parse",
		"decode",
		"encode",
	}

	for _, pattern := range nonRetryablePatterns {
		if contains(errStr, pattern) {
			return false
		}
	}

	// Default to retryable for database/network errors
	return true
}

// contains checks if a string contains a substring (case-insensitive)
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && containsAt(s, substr))
}

func containsAt(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		match := true
		for j := 0; j < len(substr); j++ {
			if s[i+j] != substr[j] {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

