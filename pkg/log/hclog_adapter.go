package log

import (
	"io"
	stdlog "log"

	hclog "github.com/hashicorp/go-hclog"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// zapHCLogAdapter adapts a *zap.Logger to the hclog.Logger interface. It is used
// on the controller (host) side to relay the plugin subprocess logs into the
// controller's log stream so that the relayed entries are encoded exactly like
// the controller's own logs.
//
// Unlike a generic wrapper, the level query methods (IsTrace, IsDebug, ...) and
// GetLevel consult the underlying zap logger so that the go-plugin host observes
// the level that is actually in effect. SetLevel is intentionally a no-op: the
// zap logger owns the level and it is configured once from the controller
// settings.
type zapHCLogAdapter struct {
	zap  *zap.Logger
	name string
}

// newZapHCLogAdapter wraps the given zap logger as an hclog.Logger.
func newZapHCLogAdapter(z *zap.Logger) hclog.Logger {
	return &zapHCLogAdapter{zap: z}
}

// relayedTimestampKey is the key under which the go-plugin host re-injects the
// plugin entry's original timestamp as a key/value pair when relaying it to the
// host logger (see go-plugin client.go logStderr:
// out = append(out, "timestamp", entry.Timestamp...)). The zap encoder already
// stamps its own time field, so this relayed key is dropped to avoid a
// duplicate timestamp in the controller log stream.
const relayedTimestampKey = "timestamp"

// hclogToZapLevel maps an hclog level to the corresponding zap level. hclog has
// a Trace level that zap does not, so it is mapped to DebugLevel.
func hclogToZapLevel(level hclog.Level) zapcore.Level {
	switch level {
	case hclog.Trace, hclog.Debug:
		return zapcore.DebugLevel
	case hclog.Info, hclog.NoLevel, hclog.DefaultLevel:
		return zapcore.InfoLevel
	case hclog.Warn:
		return zapcore.WarnLevel
	case hclog.Error:
		return zapcore.ErrorLevel
	default:
		return zapcore.InfoLevel
	}
}

// zapToHCLogLevel maps a zap level to the corresponding hclog level.
func zapToHCLogLevel(level zapcore.Level) hclog.Level {
	switch level {
	case zapcore.DebugLevel:
		return hclog.Debug
	case zapcore.InfoLevel:
		return hclog.Info
	case zapcore.WarnLevel:
		return hclog.Warn
	case zapcore.ErrorLevel, zapcore.DPanicLevel, zapcore.PanicLevel, zapcore.FatalLevel:
		return hclog.Error
	default:
		return hclog.Info
	}
}

// toZapFields converts hclog's variadic key/value args into zap fields. Keys
// that are not strings fall back to a positional field name so no argument is
// silently dropped.
func toZapFields(args ...interface{}) []zapcore.Field {
	fields := make([]zapcore.Field, 0, (len(args)+1)/2)
	for i := 0; i < len(args); i += 2 {
		if i+1 >= len(args) {
			// odd trailing arg with no value
			fields = append(fields, zap.Any(hclog.MissingKey, args[i]))
			break
		}
		key, ok := args[i].(string)
		if !ok {
			key = hclog.MissingKey
			fields = append(fields, zap.Any(key, args[i]))
			continue
		}
		// Drop the timestamp the go-plugin host re-injects; the zap encoder
		// already emits its own time field. See relayedTimestampKey.
		if key == relayedTimestampKey {
			continue
		}
		fields = append(fields, zap.Any(key, args[i+1]))
	}
	return fields
}

func (a *zapHCLogAdapter) Log(level hclog.Level, msg string, args ...interface{}) {
	switch level {
	case hclog.Trace, hclog.Debug:
		a.Debug(msg, args...)
	case hclog.Warn:
		a.Warn(msg, args...)
	case hclog.Error:
		a.Error(msg, args...)
	default:
		a.Info(msg, args...)
	}
}

func (a *zapHCLogAdapter) Trace(msg string, args ...interface{}) {
	a.zap.Debug(msg, toZapFields(args...)...)
}

func (a *zapHCLogAdapter) Debug(msg string, args ...interface{}) {
	a.zap.Debug(msg, toZapFields(args...)...)
}

func (a *zapHCLogAdapter) Info(msg string, args ...interface{}) {
	a.zap.Info(msg, toZapFields(args...)...)
}

func (a *zapHCLogAdapter) Warn(msg string, args ...interface{}) {
	a.zap.Warn(msg, toZapFields(args...)...)
}

func (a *zapHCLogAdapter) Error(msg string, args ...interface{}) {
	a.zap.Error(msg, toZapFields(args...)...)
}

// enabled reports whether the underlying zap logger emits at the given hclog
// level.
func (a *zapHCLogAdapter) enabled(level hclog.Level) bool {
	return a.zap.Core().Enabled(hclogToZapLevel(level))
}

func (a *zapHCLogAdapter) IsTrace() bool { return a.enabled(hclog.Trace) }
func (a *zapHCLogAdapter) IsDebug() bool { return a.enabled(hclog.Debug) }
func (a *zapHCLogAdapter) IsInfo() bool  { return a.enabled(hclog.Info) }
func (a *zapHCLogAdapter) IsWarn() bool  { return a.enabled(hclog.Warn) }
func (a *zapHCLogAdapter) IsError() bool { return a.enabled(hclog.Error) }

func (a *zapHCLogAdapter) ImpliedArgs() []interface{} { return nil }

func (a *zapHCLogAdapter) With(args ...interface{}) hclog.Logger {
	return &zapHCLogAdapter{zap: a.zap.With(toZapFields(args...)...), name: a.name}
}

func (a *zapHCLogAdapter) Name() string { return a.name }

func (a *zapHCLogAdapter) Named(name string) hclog.Logger {
	newName := name
	if a.name != "" {
		newName = a.name + "." + name
	}
	return &zapHCLogAdapter{zap: a.zap.Named(name), name: newName}
}

func (a *zapHCLogAdapter) ResetNamed(name string) hclog.Logger {
	return &zapHCLogAdapter{zap: a.zap.Named(name), name: name}
}

// SetLevel is a no-op. The level is owned by the underlying zap logger, which is
// configured once from the controller settings.
func (a *zapHCLogAdapter) SetLevel(level hclog.Level) {}

func (a *zapHCLogAdapter) GetLevel() hclog.Level {
	return zapToHCLogLevel(a.zap.Level())
}

func (a *zapHCLogAdapter) StandardLogger(opts *hclog.StandardLoggerOptions) *stdlog.Logger {
	return stdlog.New(a.StandardWriter(opts), "", 0)
}

func (a *zapHCLogAdapter) StandardWriter(opts *hclog.StandardLoggerOptions) io.Writer {
	return hclog.DefaultOutput
}