package log_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"testing"

	"github.com/argoproj-labs/argocd-ephemeral-access/pkg/log"
	"github.com/go-logr/logr"
	hclog "github.com/hashicorp/go-hclog"
	"github.com/go-logr/zapr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest"
)

// captureHCLogOutput redirects hclog's default output while fn runs and returns
// what was written. NewPluginLogger does not set an explicit Output, so hclog
// writes to hclog.DefaultOutput (which it snapshots at logger construction),
// not to the current os.Stderr. The logger must therefore be built inside fn.
func captureHCLogOutput(t *testing.T, fn func()) string {
	t.Helper()
	orig := hclog.DefaultOutput
	var buf bytes.Buffer
	hclog.DefaultOutput = &buf
	defer func() { hclog.DefaultOutput = orig }()

	fn()
	return buf.String()
}

func TestLoggerConfiguration(t *testing.T) {
	t.Run("will validate if default configurations are applied", func(t *testing.T) {
		// When
		zl := zaptest.NewLogger(t)
		logger, err := log.NewAppLogger(zl)

		// Then
		assert.NoError(t, err)
		assert.NotNil(t, logger)
	})
}

func TestPluginLogger(t *testing.T) {
	t.Run("will return error when logger is nil", func(t *testing.T) {
		// When
		logger, err := log.NewPluginLogger(nil)

		// Then
		assert.Error(t, err)
		assert.Nil(t, logger)
	})
	t.Run("will derive the level from the provided zap logger", func(t *testing.T) {
		// Given a zap logger enabled at debug level
		core := zapcore.NewCore(
			zapcore.NewJSONEncoder(zap.NewProductionEncoderConfig()),
			zapcore.AddSync(io.Discard),
			zapcore.DebugLevel,
		)
		zl := zap.New(core)

		// When
		logger, err := log.NewPluginLogger(zl)

		// Then
		require.NoError(t, err)
		assert.NotNil(t, logger)
		assert.True(t, logger.IsDebug())
		assert.Equal(t, "plugin", logger.Name())
	})
	t.Run("will honor a higher level from the provided zap logger", func(t *testing.T) {
		// Given a zap logger enabled only at info and above
		core := zapcore.NewCore(
			zapcore.NewJSONEncoder(zap.NewProductionEncoderConfig()),
			zapcore.AddSync(io.Discard),
			zapcore.InfoLevel,
		)
		zl := zap.New(core)

		// When
		logger, err := log.NewPluginLogger(zl)

		// Then
		require.NoError(t, err)
		assert.False(t, logger.IsDebug())
		assert.True(t, logger.IsInfo())
	})
}

func TestPluginLoggerWithOpts(t *testing.T) {
	t.Run("will validate if configs are applied without error", func(t *testing.T) {
		// When
		logger, err := log.NewPluginLoggerWithOpts(
			log.WithLevel(log.DebugLevel),
			log.WithFormat(log.JsonFormat),
		)

		// Then
		assert.NoError(t, err)
		assert.NotNil(t, logger)
		assert.True(t, logger.IsDebug())
		assert.Equal(t, "plugin", logger.Name())
	})
	t.Run("will validate if default configurations are applied", func(t *testing.T) {
		// When
		logger, err := log.NewPluginLoggerWithOpts()

		// Then
		assert.NoError(t, err)
		assert.NotNil(t, logger)
		assert.True(t, logger.IsInfo())
		assert.Equal(t, "plugin", logger.Name())
	})
	t.Run("will validate if provided level takes precedence", func(t *testing.T) {
		// When
		logger, err := log.NewPluginLoggerWithOpts(
			log.WithLevel(log.InfoLevel),
		)

		// Then
		assert.NoError(t, err)
		assert.NotNil(t, logger)
		assert.False(t, logger.IsDebug())
		assert.True(t, logger.IsInfo())
		assert.Equal(t, "plugin", logger.Name())
	})
	t.Run("will always emit hclog JSON so the host can parse the message", func(t *testing.T) {
		// When a logger built asking for text format logs an entry. hclog
		// snapshots its output at construction, so it must be built inside the
		// capture to observe its output.
		out := captureHCLogOutput(t, func() {
			logger, err := log.NewPluginLoggerWithOpts(
				log.WithName("some-plugin"),
				log.WithFormat(log.TextFormat),
			)
			require.NoError(t, err)
			logger.Info("granted access", "user", "leo")
		})

		// Then the output is hclog JSON, so @message carries only the message
		// and can be relayed into the host "msg" field verbatim.
		var entry map[string]interface{}
		require.NoError(t, json.Unmarshal([]byte(out), &entry),
			"expected JSON output regardless of the requested format, got: %s", out)
		assert.Equal(t, "granted access", entry["@message"])
		assert.Equal(t, "info", entry["@level"])
		assert.Equal(t, "some-plugin", entry["@module"])
		assert.Equal(t, "leo", entry["user"])
	})
	t.Run("will use the provided name", func(t *testing.T) {
		// When
		logger, err := log.NewPluginLoggerWithOpts(log.WithName("my-plugin"))

		// Then
		assert.NoError(t, err)
		assert.NotNil(t, logger)
		assert.Equal(t, "my-plugin", logger.Name())
	})
	t.Run("will default the name to plugin", func(t *testing.T) {
		// When
		logger, err := log.NewPluginLoggerWithOpts()

		// Then
		assert.NoError(t, err)
		assert.NotNil(t, logger)
		assert.Equal(t, "plugin", logger.Name())
	})
}

func TestPluginHostLogger(t *testing.T) {
	t.Run("will return error when logger is nil", func(t *testing.T) {
		// When
		logger, err := log.NewPluginHostLogger(nil)

		// Then
		assert.Error(t, err)
		assert.Nil(t, logger)
	})
	t.Run("will not name the logger so the host can name it after the plugin", func(t *testing.T) {
		// Given
		zl := zaptest.NewLogger(t)

		// When
		logger, err := log.NewPluginHostLogger(zl)

		// Then the logger is unnamed; the go-plugin host names it after the
		// plugin binary, so naming it here would produce a redundant segment.
		assert.NoError(t, err)
		assert.NotNil(t, logger)
		assert.Equal(t, "", logger.Name())
	})
	t.Run("will report level from the wrapped zap logger", func(t *testing.T) {
		// Given a zap logger enabled at debug level
		core := zapcore.NewCore(
			zapcore.NewJSONEncoder(zap.NewProductionEncoderConfig()),
			zapcore.AddSync(io.Discard),
			zapcore.DebugLevel,
		)
		zl := zap.New(core)

		// When
		logger, err := log.NewPluginHostLogger(zl)

		// Then
		require.NoError(t, err)
		assert.True(t, logger.IsDebug())
		assert.True(t, logger.IsInfo())
	})
	t.Run("will honor a higher level from the wrapped zap logger", func(t *testing.T) {
		// Given a zap logger enabled only at warn and above
		core := zapcore.NewCore(
			zapcore.NewJSONEncoder(zap.NewProductionEncoderConfig()),
			zapcore.AddSync(io.Discard),
			zapcore.WarnLevel,
		)
		zl := zap.New(core)

		// When
		logger, err := log.NewPluginHostLogger(zl)

		// Then
		require.NoError(t, err)
		assert.False(t, logger.IsDebug())
		assert.False(t, logger.IsInfo())
		assert.True(t, logger.IsWarn())
		assert.True(t, logger.IsError())
	})
	t.Run("will relay entries through the wrapped zap logger", func(t *testing.T) {
		// Given a zap logger writing JSON to a buffer
		var buf bytes.Buffer
		core := zapcore.NewCore(
			zapcore.NewJSONEncoder(zap.NewProductionEncoderConfig()),
			zapcore.AddSync(&buf),
			zapcore.InfoLevel,
		)
		zl := zap.New(core)
		logger, err := log.NewPluginHostLogger(zl)
		require.NoError(t, err)

		// When
		logger.Info("relayed message", "key", "value")

		// Then the entry is encoded by zap with the message and key/value pair
		var entry map[string]interface{}
		require.NoError(t, json.Unmarshal(buf.Bytes(), &entry))
		assert.Equal(t, "relayed message", entry["msg"])
		assert.Equal(t, "value", entry["key"])
	})
	t.Run("will drop the timestamp re-injected by the go-plugin host", func(t *testing.T) {
		// Given a zap logger writing JSON to a buffer
		var buf bytes.Buffer
		core := zapcore.NewCore(
			zapcore.NewJSONEncoder(zap.NewProductionEncoderConfig()),
			zapcore.AddSync(&buf),
			zapcore.InfoLevel,
		)
		zl := zap.New(core)
		logger, err := log.NewPluginHostLogger(zl)
		require.NoError(t, err)

		// When the host relays an entry, the go-plugin host appends a
		// "timestamp" key/value pair carrying the plugin's original time.
		logger.Info("relayed message", "timestamp", "2026-07-28T14:20:28.843Z", "key", "value")

		// Then the relayed timestamp is dropped (zap already emits its own "ts"
		// time field), while other key/value pairs are preserved.
		var entry map[string]interface{}
		require.NoError(t, json.Unmarshal(buf.Bytes(), &entry))
		assert.Equal(t, "relayed message", entry["msg"])
		assert.Equal(t, "value", entry["key"])
		assert.NotContains(t, entry, "timestamp")
		assert.Contains(t, entry, "ts")
	})
}

func TestLogWrapper(t *testing.T) {
	type fixture struct {
		logger *log.LogWrapper
		logr   logr.Logger
	}
	setup := func(writter io.Writer) *fixture {
		mycore := NewZapCore(writter)
		l := zap.New(mycore)
		zaplogger := zapr.NewLogger(l)
		logger := &log.LogWrapper{
			Logger: &zaplogger,
		}
		return &fixture{
			logger: logger,
			logr:   zaplogger,
		}
	}
	type entry struct {
		Level    string `json:"level"`
		Msg      string `json:"msg"`
		Error    string `json:"error"`
		TestBool bool   `json:"testBool"`
	}

	t.Run("will send info logs successfully", func(t *testing.T) {
		// Given
		b := &bytes.Buffer{}
		f := setup(b)
		var logEntry entry

		// When
		f.logger.WithValues("testBool", true).Info("hi")

		// Then
		json.Unmarshal(b.Bytes(), &logEntry)
		assert.Equal(t, "info", logEntry.Level)
		assert.Equal(t, "hi", logEntry.Msg)
		assert.True(t, logEntry.TestBool)
	})
	t.Run("will send debug logs successfully", func(t *testing.T) {
		// Given
		b := &bytes.Buffer{}
		f := setup(b)
		var logEntry entry

		// When
		f.logger.WithValues("testBool", true).Debug("hi")

		// Then
		err := json.Unmarshal(b.Bytes(), &logEntry)
		require.NoError(t, err)
		assert.Equal(t, "debug", logEntry.Level)
		assert.Equal(t, "hi", logEntry.Msg)
		assert.True(t, logEntry.TestBool)
	})
	t.Run("will send error logs successfully", func(t *testing.T) {
		// Given
		b := &bytes.Buffer{}
		f := setup(b)
		var logEntry entry
		e := errors.New("some error")

		// When
		f.logger.WithValues("testBool", true).Error(e, "This is an error")

		// Then
		err := json.Unmarshal(b.Bytes(), &logEntry)
		require.NoError(t, err)
		assert.Equal(t, "error", logEntry.Level)
		assert.Equal(t, "This is an error", logEntry.Msg)
		assert.True(t, logEntry.TestBool)
		assert.Equal(t, "some error", logEntry.Error)
	})
	t.Run("will retrieve logger from context", func(t *testing.T) {
		// Given
		b := &bytes.Buffer{}
		f := setup(b)
		var logEntry entry
		ctx := log.IntoContext(context.Background(), f.logger)

		// When
		l := log.FromContext(ctx, "testBool", true)
		l.Info("from context")

		// Then
		err := json.Unmarshal(b.Bytes(), &logEntry)
		require.NoError(t, err)
		assert.Equal(t, "info", logEntry.Level)
		assert.Equal(t, "from context", logEntry.Msg)
		assert.True(t, logEntry.TestBool)
	})
}

func NewZapCore(pipeTo io.Writer) zapcore.Core {
	return zapcore.NewCore(
		zapcore.NewJSONEncoder(zap.NewProductionEncoderConfig()),
		zap.CombineWriteSyncers(os.Stderr, zapcore.AddSync(pipeTo)),
		zapcore.DebugLevel,
	)
}
