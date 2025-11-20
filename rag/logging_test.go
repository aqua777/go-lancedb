package rag

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"
)

// LoggingTestSuite tests the file logging functionality
type LoggingTestSuite struct {
	suite.Suite
	tmpDir string
}

func TestLoggingSuite(t *testing.T) {
	suite.Run(t, new(LoggingTestSuite))
}

func (s *LoggingTestSuite) SetupTest() {
	// Create temporary directory for log files
	tmpDir, err := os.MkdirTemp("", "rag_logging_test_*")
	s.Require().NoError(err)
	s.tmpDir = tmpDir
}

func (s *LoggingTestSuite) TearDownTest() {
	// Clean up temporary directory
	if s.tmpDir != "" {
		os.RemoveAll(s.tmpDir)
	}
}

func (s *LoggingTestSuite) TestNewFileLogger() {
	logPath := filepath.Join(s.tmpDir, "test.log")
	logger, err := NewFileLogger(&FileLoggerConfig{
		Path:     logPath,
		MaxSize:  1024,
		MaxFiles: 3,
		MinLevel: LogLevelInfo,
	})

	s.NoError(err)
	s.NotNil(logger)
	s.Equal(logPath, logger.GetPath())

	// Clean up
	logger.Close()
}

func (s *LoggingTestSuite) TestNewFileLoggerNilConfig() {
	_, err := NewFileLogger(nil)
	s.Error(err)
}

func (s *LoggingTestSuite) TestNewFileLoggerEmptyPath() {
	_, err := NewFileLogger(&FileLoggerConfig{
		Path: "",
	})
	s.Error(err)
}

func (s *LoggingTestSuite) TestFileLoggerWriting() {
	logPath := filepath.Join(s.tmpDir, "test.log")
	logger, err := NewFileLogger(&FileLoggerConfig{
		Path:     logPath,
		MaxSize:  1024 * 1024,
		MaxFiles: 3,
		MinLevel: LogLevelDebug,
	})
	s.Require().NoError(err)
	defer logger.Close()

	// Write different log levels
	logger.Debug("debug message %d", 1)
	logger.Info("info message %d", 2)
	logger.Warn("warn message %d", 3)
	logger.Error("error message %d", 4)

	// Flush to ensure writes
	logger.Flush()

	// Read log file
	content, err := os.ReadFile(logPath)
	s.NoError(err)

	contentStr := string(content)
	s.Contains(contentStr, "DEBUG")
	s.Contains(contentStr, "debug message 1")
	s.Contains(contentStr, "INFO")
	s.Contains(contentStr, "info message 2")
	s.Contains(contentStr, "WARN")
	s.Contains(contentStr, "warn message 3")
	s.Contains(contentStr, "ERROR")
	s.Contains(contentStr, "error message 4")
}

func (s *LoggingTestSuite) TestFileLoggerMinLevel() {
	logPath := filepath.Join(s.tmpDir, "test.log")
	logger, err := NewFileLogger(&FileLoggerConfig{
		Path:     logPath,
		MaxSize:  1024 * 1024,
		MaxFiles: 3,
		MinLevel: LogLevelWarn, // Only warn and error should be logged
	})
	s.Require().NoError(err)
	defer logger.Close()

	// Write different log levels
	logger.Debug("debug message")
	logger.Info("info message")
	logger.Warn("warn message")
	logger.Error("error message")

	logger.Flush()

	// Read log file
	content, err := os.ReadFile(logPath)
	s.NoError(err)

	contentStr := string(content)
	s.NotContains(contentStr, "debug message")
	s.NotContains(contentStr, "info message")
	s.Contains(contentStr, "warn message")
	s.Contains(contentStr, "error message")
}

func (s *LoggingTestSuite) TestFileLoggerSetMinLevel() {
	logPath := filepath.Join(s.tmpDir, "test.log")
	logger, err := NewFileLogger(&FileLoggerConfig{
		Path:     logPath,
		MaxSize:  1024 * 1024,
		MaxFiles: 3,
		MinLevel: LogLevelInfo,
	})
	s.Require().NoError(err)
	defer logger.Close()

	logger.Debug("debug 1")
	logger.Info("info 1")

	// Change minimum level
	logger.SetMinLevel(LogLevelDebug)

	logger.Debug("debug 2")
	logger.Info("info 2")

	logger.Flush()

	content, err := os.ReadFile(logPath)
	s.NoError(err)

	contentStr := string(content)
	s.NotContains(contentStr, "debug 1")
	s.Contains(contentStr, "info 1")
	s.Contains(contentStr, "debug 2")
	s.Contains(contentStr, "info 2")
}

func (s *LoggingTestSuite) TestFileLoggerRotation() {
	logPath := filepath.Join(s.tmpDir, "test.log")
	logger, err := NewFileLogger(&FileLoggerConfig{
		Path:     logPath,
		MaxSize:  100, // Very small to trigger rotation
		MaxFiles: 3,
		MinLevel: LogLevelInfo,
	})
	s.Require().NoError(err)
	defer logger.Close()

	// Write enough to trigger rotation
	for i := 0; i < 50; i++ {
		logger.Info("log message number %d with some extra text to fill up space", i)
	}

	logger.Flush()

	// Check that rotated files exist
	_, err = os.Stat(logPath)
	s.NoError(err, "main log file should exist")

	// At least one rotated file should exist
	rotated1 := logPath + ".1"
	_, err = os.Stat(rotated1)
	s.NoError(err, "rotated log file should exist")
}

func (s *LoggingTestSuite) TestFileLoggerPrintfPrintln() {
	logPath := filepath.Join(s.tmpDir, "test.log")
	logger, err := NewFileLogger(&FileLoggerConfig{
		Path:     logPath,
		MaxSize:  1024 * 1024,
		MaxFiles: 3,
		MinLevel: LogLevelInfo,
	})
	s.Require().NoError(err)
	defer logger.Close()

	// Test Printf (from Logger interface)
	logger.Printf("formatted message: %s", "test")

	// Test Println (from Logger interface)
	logger.Println("simple", "message")

	logger.Flush()

	content, err := os.ReadFile(logPath)
	s.NoError(err)

	contentStr := string(content)
	s.Contains(contentStr, "formatted message: test")
	s.Contains(contentStr, "simple message")
}

func (s *LoggingTestSuite) TestLogLevelString() {
	s.Equal("DEBUG", LogLevelDebug.String())
	s.Equal("INFO", LogLevelInfo.String())
	s.Equal("WARN", LogLevelWarn.String())
	s.Equal("ERROR", LogLevelError.String())

	// Test unknown level
	s.Equal("UNKNOWN", LogLevel(999).String())
}

func (s *LoggingTestSuite) TestFileLoggerConcurrentWrites() {
	logPath := filepath.Join(s.tmpDir, "test.log")
	logger, err := NewFileLogger(&FileLoggerConfig{
		Path:     logPath,
		MaxSize:  1024 * 1024,
		MaxFiles: 3,
		MinLevel: LogLevelInfo,
	})
	s.Require().NoError(err)
	defer logger.Close()

	// Write concurrently from multiple goroutines
	done := make(chan bool)
	for i := 0; i < 5; i++ {
		go func(id int) {
			for j := 0; j < 10; j++ {
				logger.Info("goroutine %d message %d", id, j)
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 5; i++ {
		<-done
	}

	logger.Flush()

	// Read log file
	content, err := os.ReadFile(logPath)
	s.NoError(err)

	// Count messages (should have 50 total)
	contentStr := string(content)
	messageCount := strings.Count(contentStr, "goroutine")
	s.Equal(50, messageCount)
}

