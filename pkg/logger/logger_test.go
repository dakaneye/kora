package logger

import (
	"log/slog"
	"testing"
)

func TestNew(t *testing.T) {
	l := New(slog.LevelInfo)
	if l == nil || l.Logger == nil {
		t.Fatal("New() returned nil or Logger with nil slog.Logger")
	}
}

func TestNewJSON(t *testing.T) {
	l := NewJSON(slog.LevelInfo)
	if l == nil {
		t.Fatal("NewJSON() returned nil")
	}
}

func TestRedact(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"", "[EMPTY]"},
		{"secret-token", "[REDACTED]"},
		{"xoxp-1234567890", "[REDACTED]"},
	}

	for _, tt := range tests {
		got := Redact(tt.input)
		if got != tt.want {
			t.Errorf("Redact(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestRedactPartial(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"", "[REDACTED]"},
		{"abc", "[REDACTED]"},
		{"abcd", "[REDACTED]"},
		{"abcde", "abcd..."},
		{"hello-world", "hell..."},
	}

	for _, tt := range tests {
		got := RedactPartial(tt.input)
		if got != tt.want {
			t.Errorf("RedactPartial(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestLogger_WithSource(t *testing.T) {
	l := New(slog.LevelInfo)
	l2 := l.WithSource("github")
	if l2 == nil {
		t.Fatal("WithSource() returned nil")
	}
}

func TestSafeAttr(t *testing.T) {
	tests := []struct {
		key       string
		value     string
		wantValue string
	}{
		// Sensitive keys should trigger redaction
		{"token", "secret123", "[REDACTED]"},
		{"api_token", "abc123", "[REDACTED]"},
		{"password", "hunter2", "[REDACTED]"},
		{"secret_key", "xyz", "[REDACTED]"},
		{"credential", "cred", "[REDACTED]"},
		// Sensitive values should trigger redaction
		{"data", "xoxp-12345", "[REDACTED]"},
		{"header", "ghp_abc123", "[REDACTED]"},
		// Non-sensitive pairs should pass through
		{"username", "alice", "alice"},
		{"status", "ok", "ok"},
		{"count", "42", "42"},
	}

	for _, tt := range tests {
		attr := SafeAttr(tt.key, tt.value)
		if attr.Key != tt.key {
			t.Errorf("SafeAttr(%q, %q).Key = %q, want %q", tt.key, tt.value, attr.Key, tt.key)
		}
		if attr.Value.String() != tt.wantValue {
			t.Errorf("SafeAttr(%q, %q).Value = %q, want %q", tt.key, tt.value, attr.Value.String(), tt.wantValue)
		}
	}
}
