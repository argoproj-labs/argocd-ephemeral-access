package tracing_test

import (
	"context"
	"testing"

	"github.com/argoproj-labs/argocd-ephemeral-access/internal/backend/tracing"
	"github.com/argoproj-labs/argocd-ephemeral-access/pkg/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestLogger(t *testing.T) *log.LogWrapper {
	t.Helper()
	logger, err := log.New()
	require.NoError(t, err)
	return logger
}

func TestInit_DisabledWhenNoEndpoint(t *testing.T) {
	shutdown, err := tracing.Init(context.Background(), tracing.Config{ServiceName: "test"}, newTestLogger(t))
	require.NoError(t, err)
	require.NotNil(t, shutdown)

	assert.NoError(t, shutdown(context.Background()))
}

func TestInit_EnabledWithEndpoint(t *testing.T) {
	shutdown, err := tracing.Init(context.Background(), tracing.Config{
		ServiceName:    "test",
		ServiceVersion: "0.0.0",
		Endpoint:       "http://localhost:4318/v1/traces",
		Protocol:       tracing.ProtocolHTTPProtobuf,
		Insecure:       true,
	}, newTestLogger(t))
	require.NoError(t, err)
	require.NotNil(t, shutdown)

	// Shutdown must succeed even though no collector is reachable — the batcher
	// will just drop spans on the configured retry/timeout.
	assert.NoError(t, shutdown(context.Background()))
}

func TestInit_ProtocolSelection(t *testing.T) {
	tests := map[string]struct {
		protocol string
		wantErr  bool
	}{
		"empty defaults to grpc": {protocol: "", wantErr: false},
		"explicit grpc":          {protocol: tracing.ProtocolGRPC, wantErr: false},
		"http/protobuf":          {protocol: tracing.ProtocolHTTPProtobuf, wantErr: false},
		"unknown":                {protocol: "http/json", wantErr: true},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			shutdown, err := tracing.Init(context.Background(), tracing.Config{
				ServiceName: "test",
				Endpoint:    "http://localhost:4317",
				Protocol:    tc.protocol,
				Insecure:    true,
			}, newTestLogger(t))
			if tc.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, shutdown)
			assert.NoError(t, shutdown(context.Background()))
		})
	}
}

func TestInit_PropagatorParsing(t *testing.T) {
	tests := map[string]struct {
		propagators string
		wantErr     bool
	}{
		"empty defaults":  {propagators: "", wantErr: false},
		"tracecontext":    {propagators: "tracecontext", wantErr: false},
		"all supported":   {propagators: "tracecontext,baggage,b3,b3multi,jaeger", wantErr: false},
		"with whitespace": {propagators: " tracecontext , baggage ", wantErr: false},
		"none sentinel":   {propagators: "none", wantErr: false},
		"unknown":         {propagators: "tracecontext,nope", wantErr: true},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			shutdown, err := tracing.Init(context.Background(), tracing.Config{
				ServiceName: "test",
				Endpoint:    "http://localhost:4318",
				Insecure:    true,
				Propagators: tc.propagators,
			}, newTestLogger(t))
			if tc.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, shutdown)
			assert.NoError(t, shutdown(context.Background()))
		})
	}
}
