// internal/logger/logger.go
package logger

import (
	"log"
	"os"
)

// Logger wraps the standard logger with service context
type Logger struct {
	service string
	logger  *log.Logger
}

// New creates a new logger instance for a service
func New(service string) *Logger {
	return &Logger{
		service: service,
		logger:  log.New(os.Stdout, "["+service+"] ", log.LstdFlags),
	}
}

// Info logs an info message
func (l *Logger) Info(message string, keyvals ...interface{}) {
	l.logger.Printf("INFO: %s %v", message, formatKeyVals(keyvals...))
}

// Error logs an error message
func (l *Logger) Error(message string, keyvals ...interface{}) {
	l.logger.Printf("ERROR: %s %v", message, formatKeyVals(keyvals...))
}

// Warn logs a warning message
func (l *Logger) Warn(message string, keyvals ...interface{}) {
	l.logger.Printf("WARN: %s %v", message, formatKeyVals(keyvals...))
}

// Debug logs a debug message
func (l *Logger) Debug(message string, keyvals ...interface{}) {
	l.logger.Printf("DEBUG: %s %v", message, formatKeyVals(keyvals...))
}

// Fatal logs a fatal message and exits
func (l *Logger) Fatal(message string, keyvals ...interface{}) {
	l.logger.Printf("FATAL: %s %v", message, formatKeyVals(keyvals...))
	os.Exit(1)
}

// formatKeyVals formats key-value pairs for logging
func formatKeyVals(keyvals ...interface{}) string {
	if len(keyvals) == 0 {
		return ""
	}

	result := ""
	for i := 0; i < len(keyvals); i += 2 {
		if i+1 < len(keyvals) {
			result += " " + keyvals[i].(string) + "=" + formatValue(keyvals[i+1])
		}
	}
	return result
}

// formatValue formats a value for logging
func formatValue(v interface{}) string {
	switch val := v.(type) {
	case string:
		return val
	case int, int64, float64:
		return log.Sprint(val)
	default:
		return log.Sprint(val)
	}
}
