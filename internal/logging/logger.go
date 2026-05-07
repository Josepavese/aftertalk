package logging

import (
	"errors"
	"fmt"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var errInvalidLogFormat = errors.New("invalid log format (must be 'json' or 'console')")

var (
	Logger     *zap.SugaredLogger //nolint:gochecknoglobals // global logger is the standard pattern for structured logging
	noopLogger = zap.NewNop().Sugar()
)

func Init(level, format string) error {
	var config zap.Config

	switch format {
	case "json":
		config = zap.NewProductionConfig()
	case "console":
		config = zap.NewDevelopmentConfig()
		config.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	default:
		Logger = nil
		return fmt.Errorf("%w: %s", errInvalidLogFormat, format)
	}

	switch level {
	case "debug":
		config.Level = zap.NewAtomicLevelAt(zap.DebugLevel)
	case "info":
		config.Level = zap.NewAtomicLevelAt(zap.InfoLevel)
	case "warn":
		config.Level = zap.NewAtomicLevelAt(zap.WarnLevel)
	case "error":
		config.Level = zap.NewAtomicLevelAt(zap.ErrorLevel)
	default:
		config.Level = zap.NewAtomicLevelAt(zap.InfoLevel)
	}

	config.OutputPaths = []string{"stdout"}
	config.ErrorOutputPaths = []string{"stderr"}

	logger, err := config.Build()
	if err != nil {
		return err
	}

	Logger = logger.Sugar()
	return nil
}

func Sync() {
	if Logger != nil {
		_ = Logger.Sync() //nolint:errcheck // Sync flushes buffered logs; error on stderr flush is not actionable
	}
}

func Debug(args ...interface{}) {
	activeLogger().Debug(args...)
}

func Info(args ...interface{}) {
	activeLogger().Info(args...)
}

func Warn(args ...interface{}) {
	activeLogger().Warn(args...)
}

func Error(args ...interface{}) {
	activeLogger().Error(args...)
}

func Fatal(args ...interface{}) {
	activeLogger().Fatal(args...)
}

func Debugf(template string, args ...interface{}) {
	activeLogger().Debugf(template, args...)
}

func Infof(template string, args ...interface{}) {
	activeLogger().Infof(template, args...)
}

func Warnf(template string, args ...interface{}) {
	activeLogger().Warnf(template, args...)
}

func Errorf(template string, args ...interface{}) {
	activeLogger().Errorf(template, args...)
}

func Fatalf(template string, args ...interface{}) {
	activeLogger().Fatalf(template, args...)
}

func With(fields ...interface{}) *zap.SugaredLogger {
	return activeLogger().With(fields...)
}

func activeLogger() *zap.SugaredLogger {
	if Logger != nil {
		return Logger
	}
	return noopLogger
}
