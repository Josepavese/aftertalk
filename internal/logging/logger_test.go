package logging

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInit(t *testing.T) {
	tests := []struct {
		name      string
		level     string
		format    string
		expectErr bool
	}{
		{
			name:      "ValidDebugConsoleFormat",
			level:     "debug",
			format:    "console",
			expectErr: false,
		},
		{
			name:      "ValidInfoJSONFormat",
			level:     "info",
			format:    "json",
			expectErr: false,
		},
		{
			name:      "ValidWarnConsoleFormat",
			level:     "warn",
			format:    "console",
			expectErr: false,
		},
		{
			name:      "ValidErrorJSONFormat",
			level:     "error",
			format:    "json",
			expectErr: false,
		},
		{
			name:      "InvalidLevelDefaultsToInfo",
			level:     "invalid",
			format:    "console",
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.NoError(t, Init(tt.level, tt.format))
			defer Sync()

			assert.NotNil(t, Logger)
		})
	}
}

func TestInitWithInvalidFormat(t *testing.T) {
	err := Init("info", "invalid")
	require.Error(t, err)
	assert.Nil(t, Logger)
}

func TestInitTwice(t *testing.T) {
	require.NoError(t, Init("info", "console"))
	defer Sync()

	assert.NotNil(t, Logger)

	err := Init("debug", "json")
	assert.NoError(t, err)
	assert.NotNil(t, Logger)
}

func TestLevelSwitching(t *testing.T) {
	tests := []struct {
		validate func(t *testing.T)
		name     string
		initial  string
		second   string
	}{
		{
			name:    "FromDebugToInfo",
			initial: "debug",
			second:  "info",
			validate: func(t *testing.T) {
				assert.NotNil(t, Logger)
			},
		},
		{
			name:    "FromInfoToDebug",
			initial: "info",
			second:  "debug",
			validate: func(t *testing.T) {
				assert.NotNil(t, Logger)
			},
		},
		{
			name:    "FromErrorToWarn",
			initial: "error",
			second:  "warn",
			validate: func(t *testing.T) {
				assert.NotNil(t, Logger)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.NoError(t, Init(tt.initial, "console"))
			defer Sync()

			require.NoError(t, Init(tt.second, "console"))
			defer Sync()

			tt.validate(t)
		})
	}
}

func TestDebug(t *testing.T) {
	Init("debug", "console")
	defer Sync()

	assert.NotPanics(t, func() {
		Debug("test message")
	})
}

func TestInfo(t *testing.T) {
	Init("info", "console")
	defer Sync()

	assert.NotPanics(t, func() {
		Info("test message")
	})
}

func TestWarn(t *testing.T) {
	Init("info", "console")
	defer Sync()

	assert.NotPanics(t, func() {
		Warn("test warning")
	})
}

func TestError(t *testing.T) {
	Init("info", "console")
	defer Sync()

	assert.NotPanics(t, func() {
		Error("test error")
	})
}

// TestFatal is intentionally omitted: zap's Fatal calls os.Exit(1) which terminates the test binary.

func TestDebugf(t *testing.T) {
	Init("debug", "console")
	defer Sync()

	assert.NotPanics(t, func() {
		Debugf("test message %s", "formatted")
	})
}

func TestInfof(t *testing.T) {
	Init("info", "console")
	defer Sync()

	assert.NotPanics(t, func() {
		Infof("test message %s", "formatted")
	})
}

func TestWarnf(t *testing.T) {
	Init("info", "console")
	defer Sync()

	assert.NotPanics(t, func() {
		Warnf("test warning %s", "formatted")
	})
}

func TestErrorf(t *testing.T) {
	Init("info", "console")
	defer Sync()

	assert.NotPanics(t, func() {
		Errorf("test error %s", "formatted")
	})
}

// TestFatalf is intentionally omitted: zap's Fatalf calls os.Exit(1) which terminates the test binary.

func TestWithFields(t *testing.T) {
	Init("info", "console")
	defer Sync()

	logger := With("key1", "value1", "key2", "value2")
	assert.NotNil(t, logger)
}

func TestWithNoFields(t *testing.T) {
	Init("info", "console")
	defer Sync()

	logger := With()
	assert.NotNil(t, logger)
}

func TestMultipleWithCalls(t *testing.T) {
	Init("info", "console")
	defer Sync()

	logger1 := With("key1", "value1")
	logger2 := logger1.With("key2", "value2")
	logger3 := logger2.With("key3", "value3")

	assert.NotNil(t, logger1)
	assert.NotNil(t, logger2)
	assert.NotNil(t, logger3)
}

func TestSync(t *testing.T) {
	Init("info", "console")
	defer Sync()

	assert.NotPanics(t, func() {
		Sync()
	})
}

func TestSyncWhenLoggerNil(t *testing.T) {
	Logger = nil
	assert.NotPanics(t, func() {
		Sync()
	})
}

func TestLoggingWhenLoggerNil(t *testing.T) {
	Logger = nil

	assert.NotPanics(t, func() {
		Debug("debug message")
		Info("info message")
		Warn("warn message")
		Error("error message")
		Debugf("debug %s", "message")
		Infof("info %s", "message")
		Warnf("warn %s", "message")
		Errorf("error %s", "message")
		assert.NotNil(t, With("key", "value"))
	})
}

func TestLoggerAllLevels(t *testing.T) {
	Init("debug", "console")
	defer Sync()

	levels := []string{"debug", "info", "warn", "error"}

	for _, level := range levels {
		t.Run(level, func(t *testing.T) {
			defer Sync()
			Init(level, "console")

			switch level {
			case "debug":
				Debug("debug message")
				Debugf("debug format %s", "message")
			case "info":
				Info("info message")
				Infof("info format %s", "message")
			case "warn":
				Warn("warn message")
				Warnf("warn format %s", "message")
			case "error":
				Error("error message")
				Errorf("error format %s", "message")
			}
		})
	}
}

func TestInitWithOptions_FileSinkAndRedaction(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "aftertalk.jsonl")
	err := InitWithOptions(Options{
		Level:   "info",
		Format:  "json",
		Service: "aftertalk-test",
		Env:     "test",
		Version: "1.2.3",
		Release: "v1.2.3",
		Output: OutputOptions{
			Stdout: false,
			File: FileOutputOptions{
				Enabled: true,
				Path:    logPath,
			},
		},
		Rotation: RotationOptions{MaxSizeMB: 1, MaxBackups: 1},
		Redaction: RedactionOptions{
			Enabled: true,
			Fields:  []string{"api_key", "authorization"},
		},
	})
	if err != nil {
		t.Fatalf("InitWithOptions failed: %v", err)
	}
	InfoEvent("llm.request.completed", "api_key", "sk-secret", "authorization", "Bearer token", "session_id", "session-1")
	Sync()

	raw, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read log file: %v", err)
	}
	line := string(raw)
	if !strings.Contains(line, `"event":"llm.request.completed"`) || !strings.Contains(line, `"service":"aftertalk-test"`) {
		t.Fatalf("structured event fields missing: %s", line)
	}
	if strings.Contains(line, "sk-secret") || strings.Contains(line, "Bearer token") {
		t.Fatalf("sensitive fields were not redacted: %s", line)
	}
	if strings.Count(line, "[REDACTED]") != 2 {
		t.Fatalf("expected redacted placeholders, got: %s", line)
	}
}

func TestStructuredRedactionDoesNotHideOperationalIDsOrTokenCounts(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "aftertalk.jsonl")
	require.NoError(t, InitWithOptions(Options{
		Level:  "info",
		Format: "json",
		Output: OutputOptions{
			Stdout: false,
			File:   FileOutputOptions{Enabled: true, Path: logPath},
		},
		Redaction: RedactionOptions{
			Enabled: true,
			Fields:  []string{"token", "secret", "minutes"},
		},
	}))
	InfoEvent("llm.request.completed",
		"minutes_id", "minutes-123",
		"prompt_tokens", 42,
		"max_tokens", 128,
		"access_token", "secret-token",
		"minutes", "full sensitive payload",
	)
	Sync()

	raw, err := os.ReadFile(logPath)
	require.NoError(t, err)
	line := string(raw)
	assert.Contains(t, line, `"minutes_id":"minutes-123"`)
	assert.Contains(t, line, `"prompt_tokens":42`)
	assert.Contains(t, line, `"max_tokens":128`)
	assert.NotContains(t, line, "secret-token")
	assert.NotContains(t, line, "full sensitive payload")
	assert.Equal(t, 2, strings.Count(line, "[REDACTED]"))
}
