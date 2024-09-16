package log_test

import (
	"testing"

	"github.com/argoproj-labs/ephemeral-access/pkg/log"
	"github.com/stretchr/testify/assert"
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
}
