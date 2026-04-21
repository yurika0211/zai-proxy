package logger

import (
	"bytes"
	"strings"
	"testing"
)

func captureOutput(f func()) string {
	var buf bytes.Buffer
	old := output
	output = &buf
	defer func() { output = old }()
	f()
	return buf.String()
}

func TestInitLogger_Levels(t *testing.T) {
	tests := []struct {
		input    string
		expected LogLevel
	}{
		{"debug", DEBUG},
		{"DEBUG", DEBUG},
		{"warn", WARN},
		{"WARN", WARN},
		{"error", ERROR},
		{"ERROR", ERROR},
		{"info", INFO},
		{"INFO", INFO},
		{"unknown", INFO},
		{"", INFO},
	}

	for _, tt := range tests {
		InitLogger(tt.input)
		got := LogLevel(currentLevel.Load())
		if got != tt.expected {
			t.Errorf("InitLogger(%q): expected %d, got %d", tt.input, tt.expected, got)
		}
	}
}

func TestLogDebug_VisibleAtDebugLevel(t *testing.T) {
	InitLogger("debug")
	output := captureOutput(func() {
		LogDebug("test debug message")
	})
	if !strings.Contains(output, "test debug message") {
		t.Errorf("debug message should be visible at debug level, got: %s", output)
	}
}

func TestLogDebug_HiddenAtInfoLevel(t *testing.T) {
	InitLogger("info")
	output := captureOutput(func() {
		LogDebug("should not appear")
	})
	if strings.Contains(output, "should not appear") {
		t.Error("debug message should be hidden at info level")
	}
}

func TestLogInfo_VisibleAtInfoLevel(t *testing.T) {
	InitLogger("info")
	output := captureOutput(func() {
		LogInfo("test info message")
	})
	if !strings.Contains(output, "test info message") {
		t.Errorf("info message should be visible, got: %s", output)
	}
}

func TestLogWarn_VisibleAtWarnLevel(t *testing.T) {
	InitLogger("warn")
	output := captureOutput(func() {
		LogInfo("should not appear")
		LogWarn("test warn message")
	})
	if strings.Contains(output, "should not appear") {
		t.Error("info should be hidden at warn level")
	}
	if !strings.Contains(output, "test warn message") {
		t.Errorf("warn message should be visible, got: %s", output)
	}
}

func TestLogError_VisibleAtErrorLevel(t *testing.T) {
	InitLogger("error")
	output := captureOutput(func() {
		LogWarn("should not appear")
		LogError("test error message")
	})
	if strings.Contains(output, "should not appear") {
		t.Error("warn should be hidden at error level")
	}
	if !strings.Contains(output, "test error message") {
		t.Errorf("error message should be visible, got: %s", output)
	}
}

func TestLogFormat(t *testing.T) {
	InitLogger("info")
	output := captureOutput(func() {
		LogInfo("hello %s", "world")
	})
	if !strings.Contains(output, "hello world") {
		t.Errorf("format args should work, got: %s", output)
	}
}

func TestLogLevelNames(t *testing.T) {
	InitLogger("debug")
	output := captureOutput(func() {
		LogDebug("d")
		LogInfo("i")
		LogWarn("w")
		LogError("e")
	})
	if !strings.Contains(output, "DEBUG") {
		t.Error("should contain DEBUG level name")
	}
	if !strings.Contains(output, "INFO") {
		t.Error("should contain INFO level name")
	}
	if !strings.Contains(output, "WARN") {
		t.Error("should contain WARN level name")
	}
	if !strings.Contains(output, "ERROR") {
		t.Error("should contain ERROR level name")
	}
}

func TestConcurrentLogging(t *testing.T) {
	InitLogger("info")
	done := make(chan struct{})
	go func() {
		for i := 0; i < 100; i++ {
			LogInfo("goroutine 1: %d", i)
		}
		close(done)
	}()
	for i := 0; i < 100; i++ {
		LogInfo("goroutine 2: %d", i)
	}
	<-done
}