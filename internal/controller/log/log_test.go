package log_test

import (
	"testing"

	"github.com/argoproj-labs/ephemeral-access/internal/controller/log"
	"github.com/argoproj-labs/ephemeral-access/test/mocks"
	"github.com/stretchr/testify/assert"
)

func TestConfiguration(t *testing.T) {
	t.Run("will validate if configs are applied without error", func(t *testing.T) {
		// Given
		logConfigMock := mocks.NewMockConfigurer(t)
		logConfigMock.EXPECT().LogLevel().Return("debug")
		logConfigMock.EXPECT().LogFormat().Return("json")

		// When
		logger, err := log.NewLogger(logConfigMock)

		// Then
		assert.NoError(t, err)
		assert.NotNil(t, logger)
		logConfigMock.AssertNumberOfCalls(t, "LogLevel", 1)
		logConfigMock.AssertNumberOfCalls(t, "LogFormat", 1)
	})
	t.Run("will return error if provided invalid log level", func(t *testing.T) {
		// Given
		logConfigMock := mocks.NewMockConfigurer(t)
		logConfigMock.EXPECT().LogLevel().Return("invalid_log_level")

		// When
		_, err := log.NewLogger(logConfigMock)

		// Then
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid_log_level")
		logConfigMock.AssertNumberOfCalls(t, "LogLevel", 1)
	})
	t.Run("will return error if provided invalid log format", func(t *testing.T) {
		// Given
		logConfigMock := mocks.NewMockConfigurer(t)
		logConfigMock.EXPECT().LogLevel().Return("debug")
		logConfigMock.EXPECT().LogFormat().Return("invalid_log_format")

		// When
		_, err := log.NewLogger(logConfigMock)

		// Then
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid_log_format")
		logConfigMock.AssertNumberOfCalls(t, "LogLevel", 1)
		logConfigMock.AssertNumberOfCalls(t, "LogFormat", 2)
	})
}
