package logging

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"gopkg.in/natefinch/lumberjack.v2"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var errInvalidLogFormat = errors.New("invalid log format (must be 'json' or 'console')")

var (
	Logger     *zap.SugaredLogger //nolint:gochecknoglobals // global logger is the standard pattern for structured logging
	noopLogger = zap.NewNop().Sugar()
	stateMu    sync.RWMutex
	redactor   = RedactionOptions{Enabled: true, Fields: DefaultRedactionFields()}
)

type Options struct {
	Level     string
	Format    string
	Service   string
	Env       string
	Version   string
	Release   string
	Output    OutputOptions
	Rotation  RotationOptions
	Retention RetentionOptions
	Redaction RedactionOptions
}

type OutputOptions struct {
	Stdout bool
	File   FileOutputOptions
}

type FileOutputOptions struct {
	Enabled   bool
	Path      string
	Mandatory bool
}

type RotationOptions struct {
	MaxSizeMB  int
	MaxAgeDays int
	MaxBackups int
	Compress   bool
}

type RetentionOptions struct {
	DeleteAfterDays       int
	EmergencyCutoffSizeMB int
}

type RedactionOptions struct {
	Enabled bool
	Fields  []string
}

func Init(level, format string) error {
	return InitWithOptions(Options{
		Level:  level,
		Format: format,
		Output: OutputOptions{Stdout: true},
	})
}

func InitWithOptions(opts Options) error {
	logger, err := buildLogger(opts)
	if err != nil {
		Logger = nil
		return err
	}

	stateMu.Lock()
	Logger = logger.Sugar()
	redactor = normalizeRedaction(opts.Redaction)
	stateMu.Unlock()
	return nil
}

func buildLogger(opts Options) (*zap.Logger, error) {
	var level zap.AtomicLevel
	switch opts.Level {
	case "debug":
		level = zap.NewAtomicLevelAt(zap.DebugLevel)
	case "info", "":
		level = zap.NewAtomicLevelAt(zap.InfoLevel)
	case "warn":
		level = zap.NewAtomicLevelAt(zap.WarnLevel)
	case "error":
		level = zap.NewAtomicLevelAt(zap.ErrorLevel)
	default:
		level = zap.NewAtomicLevelAt(zap.InfoLevel)
	}

	encoderCfg := zap.NewProductionEncoderConfig()
	encoderCfg.TimeKey = "ts"
	encoderCfg.MessageKey = "msg"
	encoderCfg.LevelKey = "level"
	encoderCfg.CallerKey = "caller"
	encoderCfg.EncodeTime = zapcore.ISO8601TimeEncoder
	encoderCfg.EncodeDuration = zapcore.MillisDurationEncoder
	var encoder zapcore.Encoder
	switch opts.Format {
	case "json":
		encoder = zapcore.NewJSONEncoder(encoderCfg)
	case "console":
		encoderCfg.EncodeLevel = zapcore.CapitalColorLevelEncoder
		encoder = zapcore.NewConsoleEncoder(encoderCfg)
	default:
		return nil, fmt.Errorf("%w: %s", errInvalidLogFormat, opts.Format)
	}

	syncers := make([]zapcore.WriteSyncer, 0, 2)
	if opts.Output.Stdout || (!opts.Output.Stdout && !opts.Output.File.Enabled) {
		syncers = append(syncers, zapcore.Lock(os.Stdout))
	}
	fileFallback := false
	if opts.Output.File.Enabled {
		fileSyncer, err := fileWriteSyncer(opts)
		if err != nil {
			if opts.Output.File.Mandatory {
				return nil, err
			}
			fileFallback = true
		} else {
			syncers = append(syncers, fileSyncer)
		}
	}
	if fileFallback && !opts.Output.Stdout {
		syncers = append(syncers, zapcore.Lock(os.Stdout))
	}
	if len(syncers) == 0 {
		syncers = append(syncers, zapcore.AddSync(io.Discard))
	}

	core := zapcore.NewCore(encoder, zapcore.NewMultiWriteSyncer(syncers...), level)
	fields := []zap.Field{}
	if opts.Service != "" {
		fields = append(fields, zap.String("service", opts.Service))
	}
	if opts.Env != "" {
		fields = append(fields, zap.String("env", opts.Env))
	}
	if opts.Version != "" {
		fields = append(fields, zap.String("version", opts.Version))
	}
	if opts.Release != "" {
		fields = append(fields, zap.String("release", opts.Release))
	}
	return zap.New(core, zap.AddCaller(), zap.AddStacktrace(zap.ErrorLevel), zap.Fields(fields...)), nil
}

func fileWriteSyncer(opts Options) (zapcore.WriteSyncer, error) {
	path := strings.TrimSpace(opts.Output.File.Path)
	if path == "" {
		return nil, errors.New("logging file output enabled but path is empty")
	}
	if dir := filepath.Dir(path); dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0o750); err != nil {
			return nil, fmt.Errorf("create log directory %s: %w", dir, err)
		}
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o640) //nolint:gosec // operator-configured log path
	if err != nil {
		return nil, fmt.Errorf("open log file %s: %w", path, err)
	}
	_ = f.Close() //nolint:errcheck // open above verifies writability; lumberjack owns runtime writes

	if opts.Retention.EmergencyCutoffSizeMB > 0 {
		enforceEmergencyCutoff(path, int64(opts.Retention.EmergencyCutoffSizeMB)*1024*1024)
	}

	maxAge := opts.Rotation.MaxAgeDays
	if opts.Retention.DeleteAfterDays > 0 && (maxAge <= 0 || opts.Retention.DeleteAfterDays < maxAge) {
		maxAge = opts.Retention.DeleteAfterDays
	}
	maxSize := opts.Rotation.MaxSizeMB
	if maxSize <= 0 {
		maxSize = 100
	}
	return zapcore.AddSync(&lumberjack.Logger{
		Filename:   path,
		MaxSize:    maxSize,
		MaxAge:     maxAge,
		MaxBackups: opts.Rotation.MaxBackups,
		Compress:   opts.Rotation.Compress,
	}), nil
}

func enforceEmergencyCutoff(path string, cutoffBytes int64) {
	if cutoffBytes <= 0 {
		return
	}
	dir := filepath.Dir(path)
	base := filepath.Base(path)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	type logFile struct {
		path    string
		modTime time.Time
		size    int64
	}
	files := []logFile{}
	var total int64
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasPrefix(entry.Name(), base) {
			continue
		}
		info, statErr := entry.Info()
		if statErr != nil {
			continue
		}
		size := info.Size()
		total += size
		files = append(files, logFile{
			path:    filepath.Join(dir, entry.Name()),
			modTime: info.ModTime(),
			size:    size,
		})
	}
	if total <= cutoffBytes {
		return
	}
	sort.Slice(files, func(i, j int) bool {
		return files[i].modTime.Before(files[j].modTime)
	})
	for _, file := range files {
		if filepath.Clean(file.path) == filepath.Clean(path) {
			continue
		}
		if total <= cutoffBytes {
			return
		}
		if err := os.Remove(file.path); err == nil {
			total -= file.size
		}
	}
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

func DebugEvent(event string, fields ...interface{}) {
	activeLogger().Debugw(event, append([]interface{}{"event", event}, sanitizeFields(fields)...)...)
}

func InfoEvent(event string, fields ...interface{}) {
	activeLogger().Infow(event, append([]interface{}{"event", event}, sanitizeFields(fields)...)...)
}

func WarnEvent(event string, fields ...interface{}) {
	activeLogger().Warnw(event, append([]interface{}{"event", event}, sanitizeFields(fields)...)...)
}

func ErrorEvent(event string, fields ...interface{}) {
	activeLogger().Errorw(event, append([]interface{}{"event", event}, sanitizeFields(fields)...)...)
}

func With(fields ...interface{}) *zap.SugaredLogger {
	return activeLogger().With(sanitizeFields(fields)...)
}

func DefaultRedactionFields() []string {
	return []string{
		"api_key",
		"token",
		"authorization",
		"secret",
		"password",
		"webhook_payload",
		"transcript_text",
		"minutes",
		"raw_provider_payload",
		"provider_payload",
	}
}

func SanitizeError(err error) string {
	if err == nil {
		return ""
	}
	return sanitizeString(err.Error(), 2048)
}

func SanitizeMessage(msg string) string {
	return sanitizeString(msg, 2048)
}

func activeLogger() *zap.SugaredLogger {
	if Logger != nil {
		return Logger
	}
	return noopLogger
}

func sanitizeFields(fields []interface{}) []interface{} {
	if len(fields) == 0 {
		return fields
	}
	out := make([]interface{}, len(fields))
	copy(out, fields)

	stateMu.RLock()
	r := redactor
	stateMu.RUnlock()
	if !r.Enabled {
		return out
	}
	sensitive := redactionSet(r.Fields)
	for i := 0; i+1 < len(out); i += 2 {
		key, ok := out[i].(string)
		if !ok {
			continue
		}
		if isSensitiveKey(key, sensitive) {
			out[i+1] = "[REDACTED]"
			continue
		}
		if err, ok := out[i+1].(error); ok {
			out[i+1] = SanitizeError(err)
			continue
		}
		if s, ok := out[i+1].(string); ok {
			out[i+1] = sanitizeString(s, 4096)
		}
	}
	return out
}

func normalizeRedaction(opts RedactionOptions) RedactionOptions {
	if len(opts.Fields) == 0 {
		opts.Fields = DefaultRedactionFields()
	}
	return opts
}

func redactionSet(fields []string) map[string]struct{} {
	out := make(map[string]struct{}, len(fields))
	for _, field := range fields {
		field = strings.ToLower(strings.TrimSpace(field))
		if field != "" {
			out[field] = struct{}{}
		}
	}
	return out
}

func isSensitiveKey(key string, sensitive map[string]struct{}) bool {
	key = strings.ToLower(strings.TrimSpace(key))
	if _, ok := sensitive[key]; ok {
		return true
	}
	for field := range sensitive {
		if field == "" {
			continue
		}
		if matchesSensitiveField(key, field) {
			return true
		}
	}
	return false
}

func matchesSensitiveField(key, field string) bool {
	switch field {
	case "token":
		return key == field || strings.HasSuffix(key, "_token")
	case "minutes":
		return key == field || key == "full_minutes" || key == "minutes_payload"
	case "transcript_text", "webhook_payload", "raw_provider_payload", "provider_payload":
		return key == field || strings.HasSuffix(key, "_"+field)
	default:
		return key == field || strings.HasSuffix(key, "_"+field) || strings.HasPrefix(key, field+"_")
	}
}

func sanitizeString(msg string, limit int) string {
	msg = strings.ReplaceAll(msg, "\n", " ")
	msg = strings.ReplaceAll(msg, "\r", " ")
	msg = strings.TrimSpace(msg)
	if limit > 0 && len(msg) > limit {
		return msg[:limit] + "...[truncated]"
	}
	return msg
}
