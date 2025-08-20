package logger

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// LogLevel represents the logging level
type LogLevel int

const (
	LevelError LogLevel = iota
	LevelInfo
	LevelDebug
)

// LogEntry represents a single log entry in memory
type LogEntry struct {
	Timestamp time.Time
	Level     LogLevel
	Message   string
}

// Logger interface for logging messages
type Logger interface {
	Error(format string, args ...interface{})
	Info(format string, args ...interface{})
	Debug(format string, args ...interface{})
	GetLogs(minLevel LogLevel) string
}

// FileLogger implements Logger with file output and in-memory storage
type FileLogger struct {
	file        *os.File
	fileLevel   LogLevel      // Minimum level to write to file
	mu          sync.Mutex
	maxSize     int64
	filePath    string
	
	// In-memory storage for all logs
	memoryLogs  []LogEntry
	maxMemory   int           // Maximum number of entries to keep in memory
}

// NewFileLogger creates a new file logger
func NewFileLogger(logPath string, fileLevel LogLevel) (*FileLogger, error) {
	// Create log directory if needed
	logDir := filepath.Dir(logPath)
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create log directory: %v", err)
	}

	// Check if log file is too large and rotate if needed
	maxSize := int64(1024 * 1024) // 1MB
	if info, err := os.Stat(logPath); err == nil && info.Size() > maxSize {
		// Delete old log file if it's too large
		os.Remove(logPath)
	}

	// Open log file in append mode
	file, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file: %v", err)
	}

	return &FileLogger{
		file:        file,
		fileLevel:   fileLevel,
		maxSize:     maxSize,
		filePath:    logPath,
		memoryLogs:  make([]LogEntry, 0, 10000),
		maxMemory:   10000, // Keep last 10000 log entries in memory
	}, nil
}

// log adds an entry to memory and optionally to file
func (l *FileLogger) log(level LogLevel, format string, args ...interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()

	// Create log entry
	entry := LogEntry{
		Timestamp: time.Now(),
		Level:     level,
		Message:   fmt.Sprintf(format, args...),
	}

	// Always add to memory (ring buffer)
	if len(l.memoryLogs) >= l.maxMemory {
		// Remove oldest entry if at capacity
		l.memoryLogs = l.memoryLogs[1:]
	}
	l.memoryLogs = append(l.memoryLogs, entry)

	// Write to file if level meets threshold
	if level <= l.fileLevel {
		levelStr := "INFO"
		switch level {
		case LevelError:
			levelStr = "ERROR"
		case LevelDebug:
			levelStr = "DEBUG"
		}
		formatted := fmt.Sprintf("[%s] [%s] %s\n", 
			entry.Timestamp.Format("2006-01-02 15:04:05.000"),
			levelStr,
			entry.Message)
		l.file.WriteString(formatted)
	}
}

// Error logs an error message
func (l *FileLogger) Error(format string, args ...interface{}) {
	l.log(LevelError, format, args...)
}

// Info logs an info message
func (l *FileLogger) Info(format string, args ...interface{}) {
	l.log(LevelInfo, format, args...)
}

// Debug logs a debug message
func (l *FileLogger) Debug(format string, args ...interface{}) {
	l.log(LevelDebug, format, args...)
}

// Close closes the log file
func (l *FileLogger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.file.Close()
}

// GetLogs returns filtered logs from memory
func (l *FileLogger) GetLogs(minLevel LogLevel) string {
	l.mu.Lock()
	defer l.mu.Unlock()

	var result []string
	for _, entry := range l.memoryLogs {
		if entry.Level <= minLevel {
			levelStr := "INFO"
			switch entry.Level {
			case LevelError:
				levelStr = "ERROR"
			case LevelDebug:
				levelStr = "DEBUG"
			}
			formatted := fmt.Sprintf("[%s] [%s] %s",
				entry.Timestamp.Format("2006-01-02 15:04:05.000"),
				levelStr,
				entry.Message)
			result = append(result, formatted)
		}
	}
	return strings.Join(result, "\n")
}

// NullLogger is a logger that discards all messages
type NullLogger struct{}

func (n *NullLogger) Error(format string, args ...interface{}) {}
func (n *NullLogger) Info(format string, args ...interface{})  {}
func (n *NullLogger) Debug(format string, args ...interface{}) {}
func (n *NullLogger) GetLogs(minLevel LogLevel) string          { return "" }