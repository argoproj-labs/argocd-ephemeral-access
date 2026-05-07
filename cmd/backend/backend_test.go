package backend

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReadEnvConfigs(t *testing.T) {
	t.Run("will validate if default values are set properly", func(t *testing.T) {
		// Given: only the required namespace is set; all other vars rely on
		// envconfig defaults (or zero values for the tracing fields).
		t.Setenv("EPHEMERAL_BACKEND_NAMESPACE", "argocd")

		// When
		opts, err := readEnvConfigs()

		// Then
		require.NoError(t, err)
		assert.Equal(t, 8888, opts.Backend.Port)
		assert.Equal(t, 8091, opts.Backend.MetricsPort)
		assert.Equal(t, "argocd", opts.Backend.Namespace)
		assert.Equal(t, 4*time.Hour, opts.Backend.DefaultAccessDuration)

		assert.Empty(t, opts.Backend.Tracing.Endpoint)
		assert.False(t, opts.Backend.Tracing.Insecure)
		assert.Empty(t, opts.Backend.Tracing.Propagators)

		assert.Equal(t, "info", opts.Log.Level)
		assert.Equal(t, "text", opts.Log.Format)
	})

	t.Run("will validate if env vars are set properly", func(t *testing.T) {
		// Given
		t.Setenv("EPHEMERAL_BACKEND_PORT", "9000")
		t.Setenv("EPHEMERAL_BACKEND_METRICS_PORT", "9091")
		t.Setenv("KUBECONFIG", "/tmp/kube.cfg")
		t.Setenv("EPHEMERAL_BACKEND_NAMESPACE", "ephemeral")
		t.Setenv("EPHEMERAL_BACKEND_DEFAULT_ACCESS_DURATION", "30m")
		t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "http://collector:4318")
		t.Setenv("OTEL_EXPORTER_OTLP_INSECURE", "true")
		t.Setenv("OTEL_PROPAGATORS", "tracecontext,baggage,b3")
		t.Setenv("EPHEMERAL_LOG_LEVEL", "debug")
		t.Setenv("EPHEMERAL_LOG_FORMAT", "json")

		// When
		opts, err := readEnvConfigs()

		// Then
		require.NoError(t, err)
		assert.Equal(t, 9000, opts.Backend.Port)
		assert.Equal(t, 9091, opts.Backend.MetricsPort)
		assert.Equal(t, "/tmp/kube.cfg", opts.Backend.Kubeconfig)
		assert.Equal(t, "ephemeral", opts.Backend.Namespace)
		assert.Equal(t, 30*time.Minute, opts.Backend.DefaultAccessDuration)

		assert.Equal(t, "http://collector:4318", opts.Backend.Tracing.Endpoint)
		assert.True(t, opts.Backend.Tracing.Insecure)
		assert.Equal(t, "tracecontext,baggage,b3", opts.Backend.Tracing.Propagators)

		assert.Equal(t, "debug", opts.Log.Level)
		assert.Equal(t, "json", opts.Log.Format)
	})

	t.Run("will return error when tracing insecure is not a bool", func(t *testing.T) {
		// Given
		t.Setenv("EPHEMERAL_BACKEND_NAMESPACE", "argocd")
		t.Setenv("OTEL_EXPORTER_OTLP_INSECURE", "not-a-bool")

		// When
		opts, err := readEnvConfigs()

		// Then
		require.Error(t, err)
		assert.Nil(t, opts)
	})
}

