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

type log struct {
	Logger logr.Logger
}

func New(l logr.Logger) *log {
	return &log{
		Logger: l,
	}
}

func NewFromContext(ctx context.Context, keysAndValues ...interface{}) *log {
	l := k8slog.FromContext(ctx, keysAndValues...)
	return &log{
		Logger: l,
	}
}

func (l *log) Info(msg string, keysAndValues ...any) {
	l.Logger.V(INFO).Info(msg, keysAndValues...)
}

func (l *log) Warn(msg string, keysAndValues ...any) {
	l.Logger.V(WARN).Info(msg, keysAndValues...)
}

func (l *log) Debug(msg string, keysAndValues ...any) {
	l.Logger.V(DEBUG).Info(msg, keysAndValues...)
}

func (l *log) Error(err error, msg string, keysAndValues ...any) {
	l.Logger.Error(err, msg, keysAndValues...)
}
