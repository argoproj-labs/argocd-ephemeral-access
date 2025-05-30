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
	"github.com/go-logr/zapr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func TestLoggerConfiguration(t *testing.T) {
	t.Run("will validate if default configurations are applied", func(t *testing.T) {
		// When
		logger, err := log.NewLogger()

		// Then
		assert.NoError(t, err)
		assert.NotNil(t, logger)
	})
	t.Run("will validate if configs are applied without error", func(t *testing.T) {
		// When
		logger, err := log.NewLogger(
			log.WithLevel(log.DebugLevel),
			log.WithFormat(log.JsonFormat),
		)

		// Then
		assert.NoError(t, err)
		assert.NotNil(t, logger)
	})
	t.Run("will return error if provided invalid log level", func(t *testing.T) {

		// When
		_, err := log.NewLogger(log.WithLevel(log.LogLevel("invalid_log_level")))

		// Then
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid_log_level")
	})
	t.Run("will return error if provided invalid log format", func(t *testing.T) {
		// When
		_, err := log.NewLogger(log.WithFormat(log.LogFormat("invalid_log_format")))

		// Then
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid_log_format")
	})
}

func TestPluginLogger(t *testing.T) {
	t.Run("will validate if configs are applied without error", func(t *testing.T) {
		// When
		logger, err := log.NewPluginLogger(
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
		logger, err := log.NewPluginLogger()

		// Then
		assert.NoError(t, err)
		assert.NotNil(t, logger)
		assert.True(t, logger.IsInfo())
		assert.Equal(t, "plugin", logger.Name())
	})
	t.Run("will validate if env var are applied automatically", func(t *testing.T) {
		// Given
		t.Setenv(log.EphemeralLogLevel, "debug")
		t.Setenv(log.EphemeralLogFormat, "json")

		// When
		logger, err := log.NewPluginLogger()

		// Then
		assert.NoError(t, err)
		assert.NotNil(t, logger)
		assert.True(t, logger.IsDebug())
		assert.Equal(t, "plugin", logger.Name())
	})
	t.Run("will validate if provided configs takes precedence", func(t *testing.T) {
		// Given
		t.Setenv(log.EphemeralLogLevel, "info")
		t.Setenv(log.EphemeralLogFormat, "text")

		// When
		logger, err := log.NewPluginLogger(
			log.WithLevel(log.DebugLevel),
			log.WithFormat(log.JsonFormat),
		)

		// Then
		assert.NoError(t, err)
		assert.NotNil(t, logger)
		assert.True(t, logger.IsDebug())
		assert.Equal(t, "plugin", logger.Name())
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
