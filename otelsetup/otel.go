// Package otelsetup wires OpenTelemetry traces (OTLP push — not scraped).
//
// Traces are pushed to a collector (Grafana Alloy, OTel Collector, etc.) via OTLP.
// Metrics stay on GET /metrics (Prometheus scrape). Different pipelines.
//
// Standard env (SDK + exporter auto-config):
//
//	OTEL_SDK_DISABLED=true|false
//	OTEL_SERVICE_NAME=cluster-utils-api
//	OTEL_EXPORTER_OTLP_ENDPOINT=alloy.observability:4318   (or host:port)
//	OTEL_EXPORTER_OTLP_PROTOCOL=http/protobuf|grpc         (we default http)
//	OTEL_EXPORTER_OTLP_INSECURE=true                       (typical in-cluster)
//	OTEL_EXPORTER_OTLP_TRACES_ENDPOINT=...                 (optional override)
//	OTEL_TRACES_EXPORTER=otlp|none
//
// If no OTLP endpoint is set (and not forced), tracing is a no-op so local runs stay quiet.
package otelsetup

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.37.0"
	"go.uber.org/zap"
)

// Init sets the global TracerProvider + W3C propagators.
// Returns a shutdown func (flush spans). Safe to call when tracing is disabled (noop).
func Init(ctx context.Context, serviceName, serviceVersion string, log *zap.Logger) (shutdown func(context.Context) error, err error) {
	if serviceName == "" {
		serviceName = "cluster-utils-api"
	}
	if v := strings.TrimSpace(os.Getenv("OTEL_SERVICE_NAME")); v != "" {
		serviceName = v
	}

	noop := func(context.Context) error { return nil }

	if strings.EqualFold(os.Getenv("OTEL_SDK_DISABLED"), "true") {
		log.Info("otel traces disabled", zap.String("reason", "OTEL_SDK_DISABLED=true"))
		return noop, nil
	}
	if strings.EqualFold(os.Getenv("OTEL_TRACES_EXPORTER"), "none") {
		log.Info("otel traces disabled", zap.String("reason", "OTEL_TRACES_EXPORTER=none"))
		return noop, nil
	}

	if !endpointConfigured() {
		log.Info("otel traces disabled (no OTLP endpoint); set OTEL_EXPORTER_OTLP_ENDPOINT to push to Alloy/collector",
			zap.String("hint", "e.g. OTEL_EXPORTER_OTLP_ENDPOINT=alloy.observability.svc:4318 OTEL_EXPORTER_OTLP_INSECURE=true"),
		)
		return noop, nil
	}

	res, err := resource.New(ctx,
		resource.WithFromEnv(),
		resource.WithProcess(),
		resource.WithTelemetrySDK(),
		resource.WithHost(),
		resource.WithAttributes(
			semconv.ServiceName(serviceName),
			semconv.ServiceVersion(serviceVersion),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("otel resource: %w", err)
	}

	exp, protocol, err := newTraceExporter(ctx)
	if err != nil {
		return nil, fmt.Errorf("otel exporter: %w", err)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exp,
			sdktrace.WithMaxExportBatchSize(512),
			sdktrace.WithBatchTimeout(5*time.Second),
		),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.TraceIDRatioBased(sampleRatio()))),
	)

	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{}, // W3C traceparent / tracestate
		propagation.Baggage{},
	))

	log.Info("otel traces enabled (OTLP push — not scrape)",
		zap.String("service", serviceName),
		zap.String("protocol", protocol),
		zap.String("endpoint", endpointForLog()),
	)

	return tp.Shutdown, nil
}

func endpointConfigured() bool {
	return strings.TrimSpace(os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")) != "" ||
		strings.TrimSpace(os.Getenv("OTEL_EXPORTER_OTLP_TRACES_ENDPOINT")) != ""
}

func endpointForLog() string {
	if v := os.Getenv("OTEL_EXPORTER_OTLP_TRACES_ENDPOINT"); v != "" {
		return v
	}
	return os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
}

func sampleRatio() float64 {
	// optional OTEL_TRACE_SAMPLE_RATIO=0.0..1.0 (default 1.0 always on)
	v := strings.TrimSpace(os.Getenv("OTEL_TRACE_SAMPLE_RATIO"))
	if v == "" {
		return 1.0
	}
	var f float64
	if _, err := fmt.Sscanf(v, "%f", &f); err != nil || f < 0 || f > 1 {
		return 1.0
	}
	return f
}

func newTraceExporter(ctx context.Context) (*otlptrace.Exporter, string, error) {
	// Prefer explicit protocol; default http/protobuf (Alloy 4318) over grpc (4317).
	proto := strings.ToLower(strings.TrimSpace(os.Getenv("OTEL_EXPORTER_OTLP_PROTOCOL")))
	if proto == "" {
		proto = strings.ToLower(strings.TrimSpace(os.Getenv("OTEL_EXPORTER_OTLP_TRACES_PROTOCOL")))
	}
	if proto == "" {
		proto = "http/protobuf"
	}

	switch {
	case strings.Contains(proto, "grpc"):
		exp, err := otlptracegrpc.New(ctx) // env-based endpoint / TLS / headers
		return exp, "grpc", err
	default:
		exp, err := otlptracehttp.New(ctx)
		return exp, "http/protobuf", err
	}
}
