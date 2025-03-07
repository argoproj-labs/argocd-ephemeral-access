package log

import (
	"context"
	"fmt"
	"os"

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

// LogWrapper provides more expressive methods than the ones provided
// by the logr.Logger interface abstracting away the usage of numeric
// log levels.
type LogWrapper struct {
	Logger *logr.Logger
}

// New will initialize a new log wrapper with the provided logger.
func New(opts ...Opts) (*LogWrapper, error) {
	logger, err := NewLogger(opts...)
	if err != nil {
		return nil, fmt.Errorf("error creating logger: %s", err)
	}
	return &LogWrapper{
		Logger: &logger,
	}, nil
}

// FromContext will return a new log wrapper with the extracted logger
// from the given context.
func FromContext(ctx context.Context, keysAndValues ...interface{}) *LogWrapper {
	l := k8slog.FromContext(ctx, keysAndValues...)
	return &LogWrapper{
		Logger: &l,
	}
}

// Logger defines the main logger contract used by this project.
type Logger interface {
	Info(msg string, keysAndValues ...any)
	Debug(msg string, keysAndValues ...any)
	Error(err error, msg string, keysAndValues ...any)
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

// NewPluginLogger will initialize a logger to be used in ephemeral-access plugins.
func NewPluginLogger(opts ...Opts) (hclog.Logger, error) {
	envOpts := []Opts{}
	logFormat := os.Getenv(EphemeralLogFormat)
	if logFormat != "" {
		envOpts = append(envOpts, WithFormat(LogFormat(logFormat)))
	}
	logLevel := os.Getenv(EphemeralLogLevel)
	if logLevel != "" {
		envOpts = append(envOpts, WithLevel(LogLevel(logLevel)))
	}
	options := append(envOpts, opts...)
	cfg := logConfig(options...)
	jsonFormat := false
	if cfg.logFormat == JsonFormat {
		jsonFormat = true
	}
	return hclog.New(&hclog.LoggerOptions{
		Name:            "plugin",
		Level:           hclog.LevelFromString(string(cfg.logLevel)),
		JSONFormat:      jsonFormat,
		IncludeLocation: false,
	}), nil
}

// NewLogger will use the given opts to build a new logr.Logger instance.
// It will use zap and the underlying Logger implementation.
// This function should be called only during the service initialization.
func NewLogger(opts ...Opts) (logr.Logger, error) {
	zapLogger, err := NewZapLogger(opts...)
	if err != nil {
		return logr.Logger{}, fmt.Errorf("error creating zap logger: %s", err)
	}
	return zapr.NewLogger(zapLogger), nil
}
