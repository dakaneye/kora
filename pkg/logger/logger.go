// Package logger provides structured logging with credential redaction.
package logger

import (
	"context"
	"log/slog"
	"os"
	"strings"
)

// Logger wraps slog with credential redaction support.
type Logger struct {
	*slog.Logger
}

// New creates a new Logger with the specified level.
func New(level slog.Level) *Logger {
	opts := &slog.HandlerOptions{
		Level: level,
	}
	handler := slog.NewTextHandler(os.Stderr, opts)
	return &Logger{Logger: slog.New(handler)}
}

// NewJSON creates a new Logger that outputs JSON.
func NewJSON(level slog.Level) *Logger {
	opts := &slog.HandlerOptions{
		Level: level,
	}
	handler := slog.NewJSONHandler(os.Stderr, opts)
	return &Logger{Logger: slog.New(handler)}
}

// Redact replaces sensitive values with a redacted placeholder.
// Use this for any credential or token values.
func Redact(value string) string {
	if len(value) == 0 {
		return "[EMPTY]"
	}
	return "[REDACTED]"
}

// RedactPartial shows first 4 chars and redacts the rest.
// Only use for non-sensitive identifiers, NEVER for credentials.
func RedactPartial(value string) string {
	if len(value) <= 4 {
		return "[REDACTED]"
	}
	return value[:4] + "..."
}

// WithContext returns a Logger with context values.
// Currently returns the same logger; will extract trace IDs in future.
func (l *Logger) WithContext(_ context.Context) *Logger {
	return l
}

// WithSource adds a source identifier to log entries.
func (l *Logger) WithSource(source string) *Logger {
	return &Logger{Logger: l.With("source", source)}
}

// containsSensitive checks if a string might contain sensitive data.
func containsSensitive(s string) bool {
	lower := strings.ToLower(s)
	return strings.Contains(lower, "token") ||
		strings.Contains(lower, "secret") ||
		strings.Contains(lower, "password") ||
		strings.Contains(lower, "credential") ||
		strings.Contains(lower, "xoxp-") ||
		strings.Contains(lower, "ghp_")
}

// SafeAttr creates a slog attribute that automatically redacts values
// if the key name suggests it contains sensitive data.
// Use this when logging user-provided or dynamic key-value pairs.
func SafeAttr(key, value string) slog.Attr {
	if containsSensitive(key) || containsSensitive(value) {
		return slog.String(key, Redact(value))
	}
	return slog.String(key, value)
}
