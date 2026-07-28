package log

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"github.com/go-logr/zapr"
	hclog "github.com/hashicorp/go-hclog"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	k8slog "sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	// we can only have info, debug and error log levels when using
	// logr/zapr: https://github.com/go-logr/logr/issues/258
	INFO                 = 0
	DEBUG                = 1
	DebugLevel LogLevel  = "debug"
	InfoLevel  LogLevel  = "info"
	TextFormat LogFormat = "text"
	JsonFormat LogFormat = "json"
)

// LogLevel can be DebugLevel or InfoLevel
type LogLevel string

// LogFormat can be TextFormat or JsonFormat
type LogFormat string

// String will return the string representation for this LogLevel
func (l LogLevel) String() string {
	return string(l)
}

// String will return the string representation for this LogFormat
func (l LogFormat) String() string {
	return string(l)
}

// LogConfig is a LogConfigurer implementation
type LogConfig struct {
	logLevel  LogLevel
	logFormat LogFormat
	logName   string
}

type Opts func(*LogConfig)

func WithLevel(level LogLevel) Opts {
	return func(c *LogConfig) {
		c.logLevel = level
	}
}

func WithFormat(format LogFormat) Opts {
	return func(c *LogConfig) {
		c.logFormat = format
	}
}

// WithName sets the logger name. For plugin loggers this name is emitted as the
// hclog "@module" field of each entry, which the go-plugin host relays into the
// controller log stream, allowing a plugin to identify itself in that stream.
func WithName(name string) Opts {
	return func(c *LogConfig) {
		c.logName = name
	}
}

// LogWrapper provides more expressive methods than the ones provided
// by the logr.Logger interface abstracting away the usage of numeric
// log levels.
type LogWrapper struct {
	Logger *logr.Logger
}

// New will initialize a new log wrapper with the provided opts.
func New(opts ...Opts) (*LogWrapper, error) {

	zaplogger, err := NewZapLogger(opts...)
	if err != nil {
		return nil, fmt.Errorf("error creating zap logger: %s", err)
	}
	logger, err := NewAppLogger(zaplogger)
	if err != nil {
		return nil, fmt.Errorf("error creating logger: %s", err)
	}
	return &LogWrapper{
		Logger: &logger,
	}, nil
}

// FromContext will return a new log wrapper with the extracted logger
// from the given context.
func FromContext(ctx context.Context, keysAndValues ...any) *LogWrapper {
	l := k8slog.FromContext(ctx, keysAndValues...)
	return &LogWrapper{
		Logger: &l,
	}
}

// IntoContext takes a context and sets the logger as one of its values.
func IntoContext(ctx context.Context, logger *LogWrapper) context.Context {
	return k8slog.IntoContext(ctx, *logger.Logger)
}

// Logger defines the main logger contract used by this project.
type Logger interface {
	Info(msg string, keysAndValues ...any)
	Debug(msg string, keysAndValues ...any)
	Error(err error, msg string, keysAndValues ...any)
	WithValues(keysAndValues ...any) *LogWrapper
}

// Info logs a non-error message with info level. If provided, the given
// key/value pairs are added in the log entry context.
func (l *LogWrapper) Info(msg string, keysAndValues ...any) {
	l.Logger.V(INFO).Info(msg, keysAndValues...)
}

// Debug logs a non-error message with debug level. If provided, the given
// key/value pairs are added in the log entry context.
func (l *LogWrapper) Debug(msg string, keysAndValues ...any) {
	l.Logger.V(DEBUG).Info(msg, keysAndValues...)
}

// Error logs an error message. If provided, the given key/value pairs are added
// in the log entry context.
func (l *LogWrapper) Error(err error, msg string, keysAndValues ...any) {
	l.Logger.Error(err, msg, keysAndValues...)
}

// WithValues returns a new LogWrapper instance with additional key/value pairs.
// keysAndValues should be provided as alternating keys and values.
func (l *LogWrapper) WithValues(keysAndValues ...any) *LogWrapper {
	logger := l.Logger.WithValues(keysAndValues...)
	return &LogWrapper{
		Logger: &logger,
	}
}

// Fake logger implementation to be used in tests
type Fake struct{}

// Info noop
func (l *Fake) Info(msg string, keysAndValues ...any) {
}

// Debug noop
func (l *Fake) Debug(msg string, keysAndValues ...any) {
}

// Error noop
func (l *Fake) Error(err error, msg string, keysAndValues ...any) {
}

// WithValues noop
func (l *Fake) WithValues(keysAndValues ...any) *LogWrapper {
	return nil
}

// NewFake will instantiate a new fake logger to be used in tests
func NewFake() *Fake {
	return &Fake{}
}

// NewZapLogger will initialize and return a new zap.Logger
func NewZapLogger(opts ...Opts) (*zap.Logger, error) {
	cfg := logConfig(opts...)
	logLevel, err := zapcore.ParseLevel(cfg.logLevel.String())
	if err != nil {
		return nil, fmt.Errorf("error parsing log level from configuration: %s", err)
	}

	zapConfig := zap.Config{
		Level:             zap.NewAtomicLevelAt(logLevel),
		Development:       false,
		DisableCaller:     true,
		DisableStacktrace: true,
		OutputPaths:       []string{"stderr"},
		ErrorOutputPaths:  []string{"stderr"},
	}
	switch cfg.logFormat {
	case JsonFormat:
		encoderConfig := zap.NewProductionEncoderConfig()
		encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
		zapConfig.Encoding = "json"
		zapConfig.EncoderConfig = encoderConfig
	case TextFormat:
		encoderConfig := zap.NewDevelopmentEncoderConfig()
		encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
		zapConfig.Encoding = "console"
		zapConfig.EncoderConfig = encoderConfig
	default:
		return nil, fmt.Errorf("unsupported log format: %s", string(cfg.logFormat))
	}
	logger, err := zapConfig.Build()
	if err != nil {
		return nil, fmt.Errorf("error building logger: %s", err)
	}
	return logger, nil
}

// logConfig will build a new LogConfig based on the given opts.
func logConfig(opts ...Opts) *LogConfig {
	// set the default values
	cfg := &LogConfig{
		logLevel:  InfoLevel,
		logFormat: TextFormat,
		logName:   "plugin",
	}
	for _, opt := range opts {
		opt(cfg)
	}
	return cfg
}

const (
	EphemeralLogLevel  = "EPHEMERAL_LOG_LEVEL"
	EphemeralLogFormat = "EPHEMERAL_LOG_FORMAT"
)

// NewPluginLogger will initialize a native hclog.Logger to be used in
// ephemeral-access plugins, deriving the log level from the provided zap.Logger
// and emitting hclog JSON.
//
// Deprecated: use NewPluginLoggerWithOpts, which does not require a zap.Logger
// and allows configuring the logger name (relayed to the controller as the
// hclog "@module" field). This function is retained for backward compatibility;
// it only reads the level from the given logger and otherwise ignores it (in
// particular its encoder/format), because the plugin->host log channel requires
// hclog JSON. See NewPluginLoggerWithOpts for details. Returns an error if the
// provided logger is nil.
func NewPluginLogger(logger *zap.Logger) (hclog.Logger, error) {
	if logger == nil {
		return nil, fmt.Errorf("no logger provided to NewPluginLogger")
	}
	return newPluginLogger(WithLevel(LogLevel(logger.Level().String()))), nil
}

// NewPluginLoggerWithOpts will initialize a native hclog.Logger to be used in
// ephemeral-access plugins. The log level is defined by the provided opts,
// defaulting to InfoLevel when not set. Callers running in the plugin
// subprocess (where the controller configs are not available) should derive the
// level from the EPHEMERAL_LOG_LEVEL environment variable, which the controller
// propagates to the plugin process.
//
// The output is always encoded as hclog JSON regardless of any WithFormat
// option. The plugin->host log channel is an internal protocol that only the
// hclog JSON format can carry losslessly: it lets the go-plugin host parse the
// level and message of each relayed entry and re-emit them through the
// controller logger. With text output the host cannot parse the entry and
// relays the whole line into the host "msg" field. The format the operator sees
// is therefore defined by the controller logger, not by this plugin logger.
//
// The logger name defaults to "plugin" and can be set with WithName. The name
// is emitted as the hclog "@module" field of each entry, which the go-plugin
// host relays into the controller log stream, allowing a plugin to identify
// itself in that stream.
func NewPluginLoggerWithOpts(opts ...Opts) (hclog.Logger, error) {
	return newPluginLogger(opts...), nil
}

// newPluginLogger builds the native hclog.Logger shared by the exported plugin
// logger constructors.
func newPluginLogger(opts ...Opts) hclog.Logger {
	cfg := logConfig(opts...)
	return hclog.New(&hclog.LoggerOptions{
		Name:  cfg.logName,
		Level: hclog.LevelFromString(string(cfg.logLevel)),
		// Always JSON: the go-plugin host requires hclog JSON to parse each
		// relayed entry. See NewPluginLoggerWithOpts for details.
		JSONFormat: true,
		// Do not include the source location. When true, hclog adds an
		// "@caller" file:line field resolved via runtime.Caller. That location
		// would point at this logging package rather than the plugin call site,
		// so it is misleading, and it adds a per-log runtime.Caller cost for no
		// benefit once the entry is relayed to the controller.
		IncludeLocation: false,
	})
}

// NewPluginHostLogger builds the host side hclog.Logger that the go-plugin host
// uses to relay the plugin subprocess logs into the controller's log stream. It
// wraps the provided zap.Logger so that the relayed entries are encoded exactly
// like the controller's own logs, keeping the aggregated output consistent.
//
// This logger only produces output on the host and never crosses the plugin RPC
// boundary, so wrapping zap here is safe. This is in contrast to NewPluginLogger
// (the plugin's own emitter), whose output must remain native hclog so the host
// can correctly parse the level and message of each relayed entry.
//
// The returned logger is intentionally not named: the go-plugin host already
// names the relay logger after the plugin binary (filepath.Base of the plugin
// path). Naming it here too would prepend a redundant segment (e.g.
// "plugin.plugin") to every relayed entry.
//
// It returns an error if the provided logger is nil.
func NewPluginHostLogger(logger *zap.Logger) (hclog.Logger, error) {
	if logger == nil {
		return nil, fmt.Errorf("no logger provided to NewPluginHostLogger")
	}
	return newZapHCLogAdapter(logger), nil
}

// NewAppLogger creates a new logr.Logger instance using the provided zap.Logger.
// It returns an error if the provided logger is nil.
//
// Parameters:
//   - logger: A *zap.Logger instance to be wrapped.
//
// Returns:
//   - logr.Logger: The wrapped logger instance.
//   - error: An error if the logger is nil.
func NewAppLogger(logger *zap.Logger) (logr.Logger, error) {
	if logger == nil {
		return logr.Logger{}, fmt.Errorf("No logger provided to NewAppLogger")
	}
	return zapr.NewLogger(logger), nil
}
