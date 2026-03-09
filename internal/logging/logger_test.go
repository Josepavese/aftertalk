package logging

import (
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
	assert.Error(t, err)
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
		name     string
		initial  string
		second   string
		validate func(t *testing.T)
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
	assert.NotPanics(t, func() {
		Sync()
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
