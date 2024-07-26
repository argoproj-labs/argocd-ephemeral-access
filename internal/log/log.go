package log

import (
	"context"

	logr "github.com/go-logr/logr"
	k8slog "sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	INFO  = 0
	WARN  = 1
	DEBUG = 2
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

// NewFromContext will initialize a new log wrapper extracting the logger
// from the given context.
func NewFromContext(ctx context.Context, keysAndValues ...interface{}) *logWrapper {
	l := k8slog.FromContext(ctx, keysAndValues...)
	return &logWrapper{
		Logger: l,
	}
}

// Info logs a non-error message with info level. If provided, the given
// key/value pairs are added in the log entry context.
func (l *logWrapper) Info(msg string, keysAndValues ...any) {
	l.Logger.V(INFO).Info(msg, keysAndValues...)
}

// Warn logs a non-error message with warn level. If provided, the given
// key/value pairs are added in the log entry context.
func (l *logWrapper) Warn(msg string, keysAndValues ...any) {
	l.Logger.V(WARN).Info(msg, keysAndValues...)
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
