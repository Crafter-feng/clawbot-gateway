package log

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func TestMain(m *testing.M) {
	os.Unsetenv("CLAWBOT_LOGIN_PASSWORD")
	m.Run()
}

func TestNew(t *testing.T) {
	l := New("info")
	if l == nil {
		t.Fatal("New returned nil")
	}
}

func TestNewLevels(t *testing.T) {
	levels := []string{"debug", "info", "warn", "error"}
	for _, level := range levels {
		t.Run(level, func(t *testing.T) {
			l := New(level)
			if l == nil {
				t.Errorf("New(%q) returned nil", level)
			}
		})
	}
}

func TestNewWriter(t *testing.T) {
	var buf bytes.Buffer
	l := NewWriter(&buf, "info")
	if l == nil {
		t.Fatal("NewWriter returned nil")
	}
	l.Info("test message")
	output := buf.String()
	if !strings.Contains(output, "test message") {
		t.Errorf("NewWriter output should contain 'test message', got '%s'", output)
	}
}

func TestNewWriterLevelFiltering(t *testing.T) {
	tests := []struct {
		level    string
		logFunc  func(l *Logger, msg string)
		msg      string
		expected bool
	}{
		// Messages at or above the logger level should appear
		{"debug", func(l *Logger, msg string) { l.Debug(msg) }, "debug msg", true},
		{"info", func(l *Logger, msg string) { l.Info(msg) }, "info msg", true},
		{"warn", func(l *Logger, msg string) { l.Warn(msg) }, "warn msg", true},
		{"error", func(l *Logger, msg string) { l.Error(msg) }, "error msg", true},
		// Info logger should filter out debug
		{"info", func(l *Logger, msg string) { l.Debug(msg) }, "debug filtered", false},
		// Warn logger should filter out info and debug
		{"warn", func(l *Logger, msg string) { l.Info(msg) }, "info filtered", false},
		{"warn", func(l *Logger, msg string) { l.Debug(msg) }, "debug filtered", false},
		// Error logger should filter out warn, info, debug
		{"error", func(l *Logger, msg string) { l.Warn(msg) }, "warn filtered", false},
		{"error", func(l *Logger, msg string) { l.Info(msg) }, "info filtered", false},
		{"error", func(l *Logger, msg string) { l.Debug(msg) }, "debug filtered", false},
	}
	for _, tt := range tests {
		name := tt.level + "/" + tt.msg
		t.Run(name, func(t *testing.T) {
			var buf bytes.Buffer
			l := NewWriter(&buf, tt.level)
			tt.logFunc(l, tt.msg)
			output := buf.String()
			hasMsg := strings.Contains(output, tt.msg)
			if tt.expected && !hasMsg {
				t.Errorf("Expected output to contain '%s', got '%s'", tt.msg, output)
			}
			if !tt.expected && hasMsg {
				t.Errorf("Expected output to NOT contain '%s', got '%s'", tt.msg, output)
			}
		})
	}
}

func TestDefault(t *testing.T) {
	l := Default()
	if l == nil {
		t.Fatal("Default returned nil")
	}
}

func TestSetDefault(t *testing.T) {
	original := Default()

	var buf bytes.Buffer
	newLogger := NewWriter(&buf, "info")
	SetDefault(newLogger)

	if Default() != newLogger {
		t.Error("Default() should return the logger set by SetDefault")
	}

	// Restore original
	SetDefault(original)
	if Default() != original {
		t.Error("Default() should be restored to original")
	}
}

func TestWithComponent(t *testing.T) {
	var buf bytes.Buffer
	l := NewWriter(&buf, "info")
	cl := l.WithComponent("test")
	if cl == nil {
		t.Fatal("WithComponent returned nil")
	}

	cl.Info("component test")
	output := buf.String()
	if !strings.Contains(output, "test") {
		t.Errorf("WithComponent output should contain component name, got '%s'", output)
	}
}

func TestWithField(t *testing.T) {
	var buf bytes.Buffer
	l := NewWriter(&buf, "info")
	fl := l.WithField("key1", "value1")
	if fl == nil {
		t.Fatal("WithField returned nil")
	}

	fl.Info("field test")
	output := buf.String()
	if !strings.Contains(output, "key1") {
		t.Errorf("WithField output should contain field key, got '%s'", output)
	}
	if !strings.Contains(output, "value1") {
		t.Errorf("WithField output should contain field value, got '%s'", output)
	}
}

func TestWithComponentAndField(t *testing.T) {
	var buf bytes.Buffer
	l := NewWriter(&buf, "info")
	cl := l.WithComponent("api").WithField("request_id", "abc123")
	if cl == nil {
		t.Fatal("Chained WithComponent/WithField returned nil")
	}

	cl.Info("request started")
	output := buf.String()
	if !strings.Contains(output, "api") {
		t.Errorf("Output should contain component 'api', got '%s'", output)
	}
	if !strings.Contains(output, "abc123") {
		t.Errorf("Output should contain field value 'abc123', got '%s'", output)
	}
}

func TestLogLevelConstants(t *testing.T) {
	if LevelDebug != -4 {
		t.Errorf("LevelDebug want -4, got %d", LevelDebug)
	}
	if LevelInfo != 0 {
		t.Errorf("LevelInfo want 0, got %d", LevelInfo)
	}
	if LevelWarn != 4 {
		t.Errorf("LevelWarn want 4, got %d", LevelWarn)
	}
	if LevelError != 8 {
		t.Errorf("LevelError want 8, got %d", LevelError)
	}
}

func TestLoggerMethods(t *testing.T) {
	var buf bytes.Buffer
	l := NewWriter(&buf, "debug")

	// Test all log methods don't panic
	l.Debug("debug", "arg1", "arg2")
	l.Info("info", "arg1", "arg2")
	l.Warn("warn", "arg1", "arg2")
	l.Error("error", "arg1", "arg2")
}

func TestNewWriterError(t *testing.T) {
	// Test with invalid level string - should not panic
	var buf bytes.Buffer
	l := NewWriter(&buf, "invalid")
	if l == nil {
		t.Fatal("NewWriter with invalid level returned nil")
	}
	l.Info("test")
}

func TestLoggerChaining(t *testing.T) {
	var buf bytes.Buffer
	l := NewWriter(&buf, "info")
	// Chain multiple WithField calls
	fl := l.WithField("a", "1").WithField("b", "2")
	fl.Info("chained")
	output := buf.String()
	if !strings.Contains(output, "a") || !strings.Contains(output, "1") {
		t.Errorf("Output should contain field 'a=1', got '%s'", output)
	}
	if !strings.Contains(output, "b") || !strings.Contains(output, "2") {
		t.Errorf("Output should contain field 'b=2', got '%s'", output)
	}
}

func TestNewWriterDefaultLevel(t *testing.T) {
	// Test that empty level defaults to info
	var buf bytes.Buffer
	l := NewWriter(&buf, "")
	if l == nil {
		t.Fatal("NewWriter with empty level returned nil")
	}
	l.Info("test")
}