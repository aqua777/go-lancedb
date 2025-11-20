package rag

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// LogLevel represents the severity of a log message
type LogLevel int

const (
	LogLevelDebug LogLevel = iota
	LogLevelInfo
	LogLevelWarn
	LogLevelError
)

func (l LogLevel) String() string {
	switch l {
	case LogLevelDebug:
		return "DEBUG"
	case LogLevelInfo:
		return "INFO"
	case LogLevelWarn:
		return "WARN"
	case LogLevelError:
		return "ERROR"
	default:
		return "UNKNOWN"
	}
}

// FileLogger implements Logger interface with file output, log rotation, and levels
type FileLogger struct {
	mu          sync.Mutex
	file        *os.File
	path        string
	maxSize     int64 // Maximum size in bytes before rotation
	maxFiles    int   // Maximum number of rotated log files to keep
	currentSize int64
	minLevel    LogLevel // Minimum level to log
}

// FileLoggerConfig configures the file logger
type FileLoggerConfig struct {
	Path     string   // Path to log file
	MaxSize  int64    // Max size before rotation (default: 10MB)
	MaxFiles int      // Max rotated files to keep (default: 5)
	MinLevel LogLevel // Minimum level to log (default: Info)
}

// NewFileLogger creates a new file logger with the specified configuration
func NewFileLogger(config *FileLoggerConfig) (*FileLogger, error) {
	if config == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}

	if config.Path == "" {
		return nil, fmt.Errorf("log path cannot be empty")
	}

	// Set defaults
	if config.MaxSize == 0 {
		config.MaxSize = 10 * 1024 * 1024 // 10MB
	}
	if config.MaxFiles == 0 {
		config.MaxFiles = 5
	}

	// Create directory if it doesn't exist
	dir := filepath.Dir(config.Path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create log directory: %w", err)
	}

	// Open or create log file
	file, err := os.OpenFile(config.Path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file: %w", err)
	}

	// Get current file size
	stat, err := file.Stat()
	if err != nil {
		file.Close()
		return nil, fmt.Errorf("failed to stat log file: %w", err)
	}

	return &FileLogger{
		file:        file,
		path:        config.Path,
		maxSize:     config.MaxSize,
		maxFiles:    config.MaxFiles,
		currentSize: stat.Size(),
		minLevel:    config.MinLevel,
	}, nil
}

// NewDefaultDesktopLogger creates a logger configured for desktop applications.
// On macOS, logs to ~/Library/Logs/[appName]/rag.log
// On Linux/other, logs to ~/.local/share/[appName]/logs/rag.log
func NewDefaultDesktopLogger(appName string) (*FileLogger, error) {
	if appName == "" {
		return nil, fmt.Errorf("appName cannot be empty")
	}

	// Get home directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	// Determine log path based on OS
	var logPath string
	if runtime := os.Getenv("GOOS"); runtime == "" {
		// Detect at runtime
		// Try macOS location first
		macOSPath := filepath.Join(homeDir, "Library", "Logs", appName, "rag.log")
		macOSDir := filepath.Dir(macOSPath)
		if _, err := os.Stat(filepath.Join(homeDir, "Library", "Logs")); err == nil {
			// We're likely on macOS
			logPath = macOSPath
			os.MkdirAll(macOSDir, 0755)
		} else {
			// Default to XDG-style location
			logPath = filepath.Join(homeDir, ".local", "share", appName, "logs", "rag.log")
		}
	} else {
		// macOS
		logPath = filepath.Join(homeDir, "Library", "Logs", appName, "rag.log")
	}

	return NewFileLogger(&FileLoggerConfig{
		Path:     logPath,
		MaxSize:  10 * 1024 * 1024, // 10MB
		MaxFiles: 5,
		MinLevel: LogLevelInfo,
	})
}

// Printf implements Logger interface
func (l *FileLogger) Printf(format string, v ...interface{}) {
	l.log(LogLevelInfo, format, v)
}

// Println implements Logger interface
func (l *FileLogger) Println(v ...interface{}) {
	l.log(LogLevelInfo, fmt.Sprintln(v...), nil)
}

// Debug logs a debug message
func (l *FileLogger) Debug(format string, v ...interface{}) {
	l.log(LogLevelDebug, format, v)
}

// Info logs an info message
func (l *FileLogger) Info(format string, v ...interface{}) {
	l.log(LogLevelInfo, format, v)
}

// Warn logs a warning message
func (l *FileLogger) Warn(format string, v ...interface{}) {
	l.log(LogLevelWarn, format, v)
}

// Error logs an error message
func (l *FileLogger) Error(format string, v ...interface{}) {
	l.log(LogLevelError, format, v)
}

// log writes a log message with the specified level
func (l *FileLogger) log(level LogLevel, format string, v []interface{}) {
	// Check if we should log this level
	if level < l.minLevel {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	// Format message
	timestamp := time.Now().Format("2006-01-02 15:04:05.000")
	var message string
	if v == nil {
		message = format
	} else {
		message = fmt.Sprintf(format, v...)
	}
	
	logLine := fmt.Sprintf("[%s] [%s] %s\n", timestamp, level.String(), message)

	// Write to file
	n, err := l.file.WriteString(logLine)
	if err != nil {
		// Can't really log this, but at least try stderr
		fmt.Fprintf(os.Stderr, "Failed to write to log file: %v\n", err)
		return
	}

	l.currentSize += int64(n)

	// Check if rotation is needed
	if l.currentSize >= l.maxSize {
		l.rotate()
	}
}

// rotate rotates the log file
func (l *FileLogger) rotate() {
	// Close current file
	l.file.Close()

	// Rotate existing files
	for i := l.maxFiles - 1; i > 0; i-- {
		oldPath := fmt.Sprintf("%s.%d", l.path, i)
		newPath := fmt.Sprintf("%s.%d", l.path, i+1)
		
		// Remove oldest file if it exists
		if i == l.maxFiles-1 {
			os.Remove(newPath)
		}
		
		// Rename if exists
		if _, err := os.Stat(oldPath); err == nil {
			os.Rename(oldPath, newPath)
		}
	}

	// Rename current log to .1
	if _, err := os.Stat(l.path); err == nil {
		os.Rename(l.path, fmt.Sprintf("%s.1", l.path))
	}

	// Create new log file
	file, err := os.OpenFile(l.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create new log file after rotation: %v\n", err)
		return
	}

	l.file = file
	l.currentSize = 0
}

// Close closes the log file
func (l *FileLogger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.file != nil {
		return l.file.Close()
	}
	return nil
}

// SetMinLevel sets the minimum log level
func (l *FileLogger) SetMinLevel(level LogLevel) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.minLevel = level
}

// GetPath returns the current log file path
func (l *FileLogger) GetPath() string {
	return l.path
}

// Flush ensures all buffered log data is written to disk
func (l *FileLogger) Flush() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.file != nil {
		return l.file.Sync()
	}
	return nil
}

