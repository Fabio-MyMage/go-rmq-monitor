package logger

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"go-rmq-monitor/internal/config"
)

// Level represents log levels
type Level int

const (
	LevelDebug Level = iota
	LevelInfo
	LevelWarn
	LevelError
)

// Logger handles application logging
type Logger struct {
	file   *os.File
	mu     sync.Mutex
	level  Level
	format string
}

// LogEntry represents a structured log entry
type LogEntry struct {
	Timestamp string                 `json:"timestamp"`
	Level     string                 `json:"level"`
	Message   string                 `json:"message"`
	Fields    map[string]interface{} `json:"fields,omitempty"`
	Error     string                 `json:"error,omitempty"`
}

// New creates a new logger instance
func New(cfg config.LoggingConfig) (*Logger, error) {
	// Create log directory if it doesn't exist
	logDir := filepath.Dir(cfg.FilePath)
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create log directory: %w", err)
	}

	// Open log file
	file, err := os.OpenFile(cfg.FilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file: %w", err)
	}

	// Parse log level
	level := parseLevel(cfg.Level)

	return &Logger{
		file:   file,
		level:  level,
		format: cfg.Format,
	}, nil
}

// parseLevel converts string level to Level type
func parseLevel(levelStr string) Level {
	switch levelStr {
	case "debug":
		return LevelDebug
	case "info":
		return LevelInfo
	case "warn":
		return LevelWarn
	case "error":
		return LevelError
	default:
		return LevelInfo
	}
}

// levelToString converts Level to string
func levelToString(level Level) string {
	switch level {
	case LevelDebug:
		return "debug"
	case LevelInfo:
		return "info"
	case LevelWarn:
		return "warn"
	case LevelError:
		return "error"
	default:
		return "info"
	}
}

// log writes a log entry
func (l *Logger) log(level Level, message string, err error, fields map[string]interface{}) {
	if level < l.level {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	entry := LogEntry{
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Level:     levelToString(level),
		Message:   message,
		Fields:    fields,
	}

	if err != nil {
		entry.Error = err.Error()
	}

	var output string
	if l.format == "json" {
		jsonBytes, _ := json.Marshal(entry)
		output = string(jsonBytes) + "\n"
	} else {
		// Text format
		output = l.formatText(entry)
	}

	// Write to file
	io.WriteString(l.file, output)
	
	// Also write to stdout for visibility
	io.WriteString(os.Stdout, output)
}

// formatText formats a log entry as text
func (l *Logger) formatText(entry LogEntry) string {
	output := fmt.Sprintf("[%s] %s: %s", entry.Timestamp, entry.Level, entry.Message)
	
	if len(entry.Fields) > 0 {
		fieldsJSON, _ := json.Marshal(entry.Fields)
		output += fmt.Sprintf(" %s", string(fieldsJSON))
	}
	
	if entry.Error != "" {
		output += fmt.Sprintf(" error=%s", entry.Error)
	}
	
	return output + "\n"
}

// Debug logs a debug message
func (l *Logger) Debug(message string, fields map[string]interface{}) {
	l.log(LevelDebug, message, nil, fields)
}

// Info logs an info message
func (l *Logger) Info(message string, fields map[string]interface{}) {
	l.log(LevelInfo, message, nil, fields)
}

// Warn logs a warning message
func (l *Logger) Warn(message string, fields map[string]interface{}) {
	l.log(LevelWarn, message, nil, fields)
}

// Error logs an error message
func (l *Logger) Error(message string, err error, fields map[string]interface{}) {
	l.log(LevelError, message, err, fields)
}

// Close closes the log file
func (l *Logger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	
	if l.file != nil {
		return l.file.Sync()
	}
	return nil
}
