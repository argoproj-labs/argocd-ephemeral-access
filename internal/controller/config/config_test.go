package config_test

import (
	"testing"
	"time"

	"github.com/argoproj-labs/argocd-ephemeral-access/internal/controller/config"
	"github.com/stretchr/testify/assert"
)

func TestConfiguration(t *testing.T) {
	t.Run("will validate if default values are set properly", func(t *testing.T) {
		// When
		config, err := config.ReadEnvConfigs()

		// Then
		assert.NoError(t, err, "NewConfiguration error")
		assert.Equal(t, "info", config.LogLevel())
		assert.Equal(t, "text", config.LogFormat())
		assert.Equal(t, ":8090", config.MetricsAddress())
		assert.Equal(t, false, config.MetricsSecure())
		assert.Equal(t, 8081, config.ControllerPort())
		assert.Equal(t, false, config.EnableLeaderElection())
		assert.Equal(t, ":8082", config.ControllerHealthProbeAddr())
		assert.Equal(t, false, config.ControllerEnableHTTP2())
		assert.Equal(t, time.Minute*3, config.ControllerRequeueInterval())
		assert.Equal(t, "", config.PluginPath())
		assert.Equal(t, time.Hour*4, config.ControllerRequestTimeout())
		assert.Equal(t, time.Nanosecond*0, config.ControllerAccessRequestTTL())
	})
	t.Run("will validate if env vars are set properly", func(t *testing.T) {
		// Given
		t.Setenv("EPHEMERAL_LOG_LEVEL", "debug")
		t.Setenv("EPHEMERAL_LOG_FORMAT", "json")
		t.Setenv("EPHEMERAL_METRICS_ADDR", ":9093")
		t.Setenv("EPHEMERAL_METRICS_SECURE", "true")
		t.Setenv("EPHEMERAL_CONTROLLER_PORT", "9091")
		t.Setenv("EPHEMERAL_CONTROLLER_ENABLE_LEADER_ELECTION", "true")
		t.Setenv("EPHEMERAL_CONTROLLER_HEALTH_PROBE_ADDR", ":1313")
		t.Setenv("EPHEMERAL_CONTROLLER_ENABLE_HTTP2", "true")
		t.Setenv("EPHEMERAL_CONTROLLER_REQUEUE_INTERVAL", "1s")
		t.Setenv("EPHEMERAL_CONTROLLER_REQUEST_TIMEOUT", "1h")
		t.Setenv("EPHEMERAL_CONTROLLER_ACCESS_REQUEST_TTL", "10h")
		t.Setenv("EPHEMERAL_PLUGIN_PATH", "/usr/local/bin/plugin")

		// When
		config, err := config.ReadEnvConfigs()

		// Then
		assert.NoError(t, err, "NewConfiguration error")
		assert.Equal(t, "debug", config.LogLevel())
		assert.Equal(t, "json", config.LogFormat())
		assert.Equal(t, ":9093", config.MetricsAddress())
		assert.Equal(t, true, config.MetricsSecure())
		assert.Equal(t, 9091, config.ControllerPort())
		assert.Equal(t, true, config.EnableLeaderElection())
		assert.Equal(t, ":1313", config.ControllerHealthProbeAddr())
		assert.Equal(t, true, config.ControllerEnableHTTP2())
		assert.Equal(t, time.Second, config.ControllerRequeueInterval())
		assert.Equal(t, "/usr/local/bin/plugin", config.PluginPath())
		assert.Equal(t, time.Hour*1, config.ControllerRequestTimeout())
		assert.Equal(t, time.Hour*10, config.ControllerAccessRequestTTL())
	})
}
