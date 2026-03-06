package logger

import (
	"time"

	"go.uber.org/zap"
)

// Field is an alias for zap.Field so callers need not import zap.
type Field = zap.Field

// New creates a production logger with JSON encoding.
// Logs go to stdout so Railway (and similar platforms) parse the "level" field
// from JSON and display correctly, rather than treating stderr as errors.
func New() *zap.Logger {
	cfg := zap.NewProductionConfig()
	cfg.OutputPaths = []string{"stdout"}
	return zap.Must(cfg.Build())
}

// Init creates a new logger and installs it as the global. Call once at startup.
// After Init, use Info, Warn, Error, etc. for logging.
func Init() {
	zap.ReplaceGlobals(New())
}

// Sync flushes buffered log entries. Call before process exit (e.g. defer logger.Sync()).
func Sync() {
	_ = zap.L().Sync()
}

// Info logs at info level.
func Info(msg string, fields ...Field) {
	zap.L().Info(msg, fields...)
}

// Warn logs at warn level.
func Warn(msg string, fields ...Field) {
	zap.L().Warn(msg, fields...)
}

// Error logs at error level.
func Error(msg string, fields ...Field) {
	zap.L().Error(msg, fields...)
}

// Debug logs at debug level.
func Debug(msg string, fields ...Field) {
	zap.L().Debug(msg, fields...)
}

// Fatal logs at fatal level and exits the process.
func Fatal(msg string, fields ...Field) {
	zap.L().Fatal(msg, fields...)
}

// Field constructors — use these instead of zap.String, zap.Int, etc.

func String(key, val string) Field  { return zap.String(key, val) }
func Int(key string, val int) Field { return zap.Int(key, val) }
func Int64(key string, val int64) Field {
	return zap.Int64(key, val)
}
func Bool(key string, val bool) Field { return zap.Bool(key, val) }
func Duration(key string, val time.Duration) Field {
	return zap.Duration(key, val)
}
func Time(key string, val time.Time) Field { return zap.Time(key, val) }

// Err returns a field for an error. Use as logger.Err(err) to avoid shadowing the Error function.
func Err(err error) Field { return zap.Error(err) }
