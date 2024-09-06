package config_test

import (
	"os"
	"testing"
	"time"

	"github.com/argoproj-labs/ephemeral-access/internal/controller/config"
	"github.com/stretchr/testify/assert"
)

func TestConfiguration(t *testing.T) {
	t.Run("will validate if default values are set properly", func(t *testing.T) {
		// When
		config, err := config.NewConfiguration()

		// Then
		assert.NoError(t, err, "NewConfiguration error")
		assert.Equal(t, "info", config.LogLevel())
		assert.Equal(t, "console", config.LogFormat())
		assert.Equal(t, "0", config.MetricsAddress())
		assert.Equal(t, false, config.MetricsSecure())
		assert.Equal(t, false, config.EnableLeaderElection())
		assert.Equal(t, ":8081", config.ControllerHealthProbeAddr())
		assert.Equal(t, false, config.ControllerEnableHTTP2())
		assert.Equal(t, time.Minute*3, config.ControllerRequeueInterval())
	})
	t.Run("will validate if env vars are set properly", func(t *testing.T) {
		// Given
		os.Setenv("EPHEMERAL_LOG_LEVEL", "debug")
		os.Setenv("EPHEMERAL_LOG_FORMAT", "json")
		os.Setenv("EPHEMERAL_METRICS_ADDR", ":9091")
		os.Setenv("EPHEMERAL_METRICS_SECURE", "true")
		os.Setenv("EPHEMERAL_CONTROLLER_ENABLE_LEADER_ELECTION", "true")
		os.Setenv("EPHEMERAL_CONTROLLER_HEALTH_PROBE_ADDR", ":1313")
		os.Setenv("EPHEMERAL_CONTROLLER_ENABLE_HTTP2", "true")
		os.Setenv("EPHEMERAL_CONTROLLER_REQUEUE_INTERVAL", "1s")

		// When
		config, err := config.NewConfiguration()

		// Then
		assert.NoError(t, err, "NewConfiguration error")
		assert.Equal(t, "debug", config.LogLevel())
		assert.Equal(t, "json", config.LogFormat())
		assert.Equal(t, ":9091", config.MetricsAddress())
		assert.Equal(t, true, config.MetricsSecure())
		assert.Equal(t, true, config.EnableLeaderElection())
		assert.Equal(t, ":1313", config.ControllerHealthProbeAddr())
		assert.Equal(t, true, config.ControllerEnableHTTP2())
		assert.Equal(t, time.Second, config.ControllerRequeueInterval())
	})
}
