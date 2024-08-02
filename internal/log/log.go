package log

import (
	"context"
	"fmt"

	logr "github.com/go-logr/logr"
	"go.uber.org/zap/zapcore"
	k8slog "sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	// we can only have info, debug and error log levels when using
	// logr/zapr: https://github.com/go-logr/logr/issues/258
	INFO  = 0
	DEBUG = 1
)

// logWrapper provides more expressive methods than the ones provided
// by the logr.Logger interface abstracting away the usage of numeric
// log levels.
type logWrapper struct {
	Logger logr.Logger
}

// New will initialize a new log wrapper with the provided logger.
func New(l logr.Logger) *logWrapper {
	return &logWrapper{
		Logger: l,
	}
}

// FromContext will return a new log wrapper with the extracted logger
// from the given context.
func FromContext(ctx context.Context, keysAndValues ...interface{}) *logWrapper {
	l := k8slog.FromContext(ctx, keysAndValues...)
	return &logWrapper{
		Logger: l,
	}
}

func ZapLevel(level string) (zapcore.Level, error) {
	var l zapcore.Level
	if err := l.UnmarshalText([]byte(level)); err != nil {
		return zapcore.InfoLevel, fmt.Errorf("unable to determine log level: %w", err)
	}
	return l, nil
}

// Info logs a non-error message with info level. If provided, the given
// key/value pairs are added in the log entry context.
func (l *logWrapper) Info(msg string, keysAndValues ...any) {
	l.Logger.V(INFO).Info(msg, keysAndValues...)
}

// Debug logs a non-error message with debug level. If provided, the given
// key/value pairs are added in the log entry context.
func (l *logWrapper) Debug(msg string, keysAndValues ...any) {
	l.Logger.V(DEBUG).Info(msg, keysAndValues...)
}

// Error logs an error message. If provided, the given
// key/value pairs are added in the log entry context.
func (l *logWrapper) Error(err error, msg string, keysAndValues ...any) {
	l.Logger.Error(err, msg, keysAndValues...)
}
