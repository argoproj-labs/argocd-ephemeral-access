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
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
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
	// Endpoint is the resolved OTLP/HTTP endpoint. When empty, tracing is
	// disabled and Init returns a no-op shutdown.
	Endpoint string
	// Insecure disables TLS on the OTLP/HTTP exporter.
	Insecure bool
	// Propagators is a comma-separated list of propagator names. Empty means
	// the default ("tracecontext,baggage"). Supported names: tracecontext,
	// baggage, b3, b3multi, jaeger, none. Unknown names cause Init to error.
	Propagators string
}

// Init configures the global OpenTelemetry tracer provider with an OTLP/HTTP
// exporter when Config.Endpoint is set. When Endpoint is empty, Init logs that
// tracing is disabled and returns a no-op ShutdownFunc.
//
// All other OTLP/HTTP exporter env vars (headers, TLS, compression, timeouts)
// per the OTel specification are honored automatically by otlptracehttp.
func Init(ctx context.Context, cfg Config, logger log.Logger) (ShutdownFunc, error) {
	if cfg.Endpoint == "" {
		logger.Info("OpenTelemetry tracing disabled: no OTLP endpoint configured")
		return noopShutdown, nil
	}

	propagator, err := buildPropagator(cfg.Propagators)
	if err != nil {
		return noopShutdown, err
	}

	clientOpts := []otlptracehttp.Option{}
	if cfg.Insecure {
		clientOpts = append(clientOpts, otlptracehttp.WithInsecure())
	}
	exporter, err := otlptrace.New(ctx, otlptracehttp.NewClient(clientOpts...))
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

	logger.Info("OpenTelemetry tracing enabled", "endpoint", cfg.Endpoint, "service", cfg.ServiceName, "insecure", cfg.Insecure)
	return tp.Shutdown, nil
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
