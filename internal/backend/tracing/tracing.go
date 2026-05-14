// Package tracing provides OpenTelemetry tracing setup for the backend service.
//
// Tracing is opt-in: the SDK is only initialized when Config.Endpoint is set.
// When disabled, Init returns a no-op shutdown function and the global tracer
// provider remains the default no-op implementation, so otelhttp instrumentation
// is effectively a passthrough.
package tracing

import (
	"context"
	"fmt"
	"strings"

	"github.com/argoproj-labs/argocd-ephemeral-access/pkg/log"

	"go.opentelemetry.io/contrib/propagators/autoprop"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

// Supported values for Config.Protocol. Names match the OpenTelemetry
// specification env var OTEL_EXPORTER_OTLP_PROTOCOL.
const (
	ProtocolGRPC         = "grpc"
	ProtocolHTTPProtobuf = "http/protobuf"
)

// ShutdownFunc flushes any buffered spans and releases exporter resources.
// It is safe to call when tracing was not enabled — it will be a no-op.
type ShutdownFunc func(context.Context) error

// Config controls tracing initialization.
type Config struct {
	// ServiceName is reported as service.name on all spans.
	ServiceName string
	// ServiceVersion is reported as service.version on all spans. Optional.
	ServiceVersion string
	// Endpoint is the resolved OTLP endpoint. When empty, tracing is disabled
	// and Init returns a no-op shutdown. The endpoint must be compatible with
	// the chosen Protocol (gRPC collectors typically listen on :4317,
	// HTTP/protobuf collectors on :4318).
	Endpoint string
	// Protocol selects the OTLP wire protocol. Supported values: "grpc"
	// (default) and "http/protobuf". An unknown value causes Init to error.
	Protocol string
	// Insecure disables TLS on the OTLP exporter.
	Insecure bool
	// Propagators is a comma-separated list of propagator names. Empty means
	// the default ("tracecontext,baggage"). Supported names: tracecontext,
	// baggage, b3, b3multi, jaeger, none. Unknown names cause Init to error.
	Propagators string
}

// Init configures the global OpenTelemetry tracer provider with an OTLP
// exporter when Config.Endpoint is set. The wire protocol is selected by
// Config.Protocol (grpc or http/protobuf). When Endpoint is empty, Init logs
// that tracing is disabled and returns a no-op ShutdownFunc.
//
// All other OTLP exporter env vars (headers, TLS, compression, timeouts) per
// the OTel specification are honored automatically by the selected exporter.
func Init(ctx context.Context, cfg Config, logger log.Logger) (ShutdownFunc, error) {
	if cfg.Endpoint == "" {
		logger.Info("OpenTelemetry tracing disabled: no OTLP endpoint configured")
		return noopShutdown, nil
	}

	propagator, err := buildPropagator(cfg.Propagators)
	if err != nil {
		return noopShutdown, err
	}

	client, err := newExporterClient(cfg)
	if err != nil {
		return noopShutdown, err
	}
	exporter, err := otlptrace.New(ctx, client)
	if err != nil {
		return noopShutdown, fmt.Errorf("error creating OTLP trace exporter: %w", err)
	}

	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(cfg.ServiceName),
			semconv.ServiceVersion(cfg.ServiceVersion),
		),
		resource.WithFromEnv(),
		resource.WithProcess(),
		resource.WithTelemetrySDK(),
	)
	if err != nil {
		return noopShutdown, fmt.Errorf("error creating tracing resource: %w", err)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagator)

	logger.Info("OpenTelemetry tracing enabled",
		"endpoint", cfg.Endpoint,
		"protocol", normalizeProtocol(cfg.Protocol),
		"service", cfg.ServiceName,
		"insecure", cfg.Insecure,
	)
	return tp.Shutdown, nil
}

// newExporterClient builds the OTLP exporter client matching cfg.Protocol.
// An empty Protocol defaults to gRPC, which matches the OpenTelemetry SDK
// default and the in-cluster collector convention.
func newExporterClient(cfg Config) (otlptrace.Client, error) {
	switch normalizeProtocol(cfg.Protocol) {
	case ProtocolGRPC:
		opts := []otlptracegrpc.Option{}
		if cfg.Insecure {
			opts = append(opts, otlptracegrpc.WithInsecure())
		}
		return otlptracegrpc.NewClient(opts...), nil
	case ProtocolHTTPProtobuf:
		opts := []otlptracehttp.Option{}
		if cfg.Insecure {
			opts = append(opts, otlptracehttp.WithInsecure())
		}
		return otlptracehttp.NewClient(opts...), nil
	default:
		return nil, fmt.Errorf("unsupported OTLP protocol %q: must be %q or %q", cfg.Protocol, ProtocolGRPC, ProtocolHTTPProtobuf)
	}
}

func normalizeProtocol(p string) string {
	if p == "" {
		return ProtocolGRPC
	}
	return p
}

// buildPropagator parses a comma-separated list of propagator names into a
// composite TextMapPropagator using autoprop. An empty list yields autoprop's
// default (W3C tracecontext + baggage). Supported names include tracecontext,
// baggage, b3, b3multi, jaeger, ottrace, xray, and the "none" sentinel.
// Unknown names cause an error so misconfigurations surface at startup.
func buildPropagator(names string) (propagation.TextMapPropagator, error) {
	trimmed := strings.TrimSpace(names)
	if trimmed == "" {
		return autoprop.NewTextMapPropagator(), nil
	}

	parts := strings.Split(trimmed, ",")
	cleaned := make([]string, 0, len(parts))
	for _, raw := range parts {
		name := strings.TrimSpace(raw)
		if name != "" {
			cleaned = append(cleaned, name)
		}
	}
	p, err := autoprop.TextMapPropagator(cleaned...)
	if err != nil {
		return nil, fmt.Errorf("error building OTel propagator: %w", err)
	}
	return p, nil
}

func noopShutdown(context.Context) error { return nil }
