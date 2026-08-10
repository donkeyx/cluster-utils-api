// Package otelsetup wires OpenTelemetry traces (OTLP push — not scraped).
//
// Traces are pushed to a collector (Grafana Alloy, OTel Collector, etc.) via OTLP.
// Metrics stay on GET /metrics (Prometheus scrape). Different pipelines.
//
// End-to-end mesh tracing (Istio / Linkerd / etc.):
//
//   - W3C traceparent/tracestate — modern default (Envoy, OTEL, most stacks)
//   - B3 (x-b3-*) — still very common with Istio/Zipkin-style configs
//   - Jaeger uber-trace-id — older but still around
//   - x-request-id — Envoy/Istio correlation id (NOT a trace context header);
//     we attach it as a span attribute + echo on responses so you can join
//     access logs ↔ Tempo (same idea as x-correlation-id, x-amzn-trace-id, …)
//
// Standard env (and what we default when unset):
//
//	OTEL_SDK_DISABLED          default false (but no-op until endpoint set)
//	OTEL_SERVICE_NAME          default cluster-utils-api (or Init arg)
//	OTEL_SERVICE_VERSION       from binary Version when Init is called
//	OTEL_EXPORTER_OTLP_ENDPOINT          required to enable export
//	OTEL_EXPORTER_OTLP_TRACES_ENDPOINT    optional override
//	OTEL_EXPORTER_OTLP_PROTOCOL           default http/protobuf  (:4318)
//	OTEL_EXPORTER_OTLP_INSECURE           often true in-cluster (exporter reads env)
//	OTEL_TRACES_EXPORTER                 none disables; else otlp when endpoint set
//	OTEL_TRACE_SAMPLE_RATIO              default 1.0
//	OTEL_TRACE_PROBES                    default false (skip /livez etc.)
//	OTEL_PROPAGATORS                     optional; we always install W3C+B3+Jaeger+baggage
//
// If no OTLP endpoint is set, tracing is a no-op so local runs stay quiet.
package otelsetup

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"go.opentelemetry.io/contrib/propagators/b3"
	"go.opentelemetry.io/contrib/propagators/jaeger"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.37.0"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

// Init sets the global TracerProvider + multi-format propagators for mesh interop.
// Returns a shutdown func (flush spans). Safe when tracing is disabled (noop).
func Init(ctx context.Context, serviceName, serviceVersion string, log *zap.Logger) (shutdown func(context.Context) error, err error) {
	if serviceName == "" {
		serviceName = "cluster-utils-api"
	}
	if v := strings.TrimSpace(os.Getenv("OTEL_SERVICE_NAME")); v != "" {
		serviceName = v
	}

	noop := func(context.Context) error { return nil }

	// Always install propagators so inbound W3C/B3 still extract into context
	// even when we are not exporting (helps local middleware / future hops).
	installPropagators()

	cfg := effectiveConfig(serviceName, serviceVersion)
	logConfig(log, cfg)

	if !cfg.Enabled {
		return noop, nil
	}

	res, err := resource.New(ctx,
		resource.WithFromEnv(), // OTEL_RESOURCE_ATTRIBUTES, OTEL_SERVICE_*
		resource.WithProcess(),
		resource.WithTelemetrySDK(),
		resource.WithHost(),
		resource.WithAttributes(
			semconv.ServiceName(cfg.ServiceName),
			semconv.ServiceVersion(cfg.ServiceVersion),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("otel resource: %w", err)
	}

	exp, err := newTraceExporter(ctx, cfg.Protocol)
	if err != nil {
		return nil, fmt.Errorf("otel exporter: %w", err)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exp,
			sdktrace.WithMaxExportBatchSize(512),
			sdktrace.WithBatchTimeout(5*time.Second),
		),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.TraceIDRatioBased(cfg.SampleRatio))),
	)

	otel.SetTracerProvider(tp)

	log.Info("otel traces enabled (OTLP push — not scrape)",
		zap.String("service", cfg.ServiceName),
		zap.String("version", cfg.ServiceVersion),
		zap.String("protocol", cfg.Protocol),
		zap.String("endpoint", cfg.Endpoint),
		zap.Float64("sample_ratio", cfg.SampleRatio),
		zap.Strings("propagators", cfg.Propagators),
	)

	return tp.Shutdown, nil
}

// Config is the resolved startup picture (for logs / tests).
type Config struct {
	Enabled        bool
	DisabledReason string
	ServiceName    string
	ServiceVersion string
	Endpoint       string
	Protocol       string
	InsecureEnv    string
	SampleRatio    float64
	TraceProbes    bool
	Propagators    []string
}

func effectiveConfig(serviceName, serviceVersion string) Config {
	cfg := Config{
		ServiceName:    serviceName,
		ServiceVersion: serviceVersion,
		Endpoint:       endpointForLog(),
		Protocol:       protocol(),
		InsecureEnv:    firstEnv("OTEL_EXPORTER_OTLP_INSECURE", "OTEL_EXPORTER_OTLP_TRACES_INSECURE"),
		SampleRatio:    sampleRatio(),
		TraceProbes:    os.Getenv("OTEL_TRACE_PROBES") == "true",
		Propagators:    []string{"tracecontext", "baggage", "b3", "jaeger"},
	}

	switch {
	case strings.EqualFold(os.Getenv("OTEL_SDK_DISABLED"), "true"):
		cfg.DisabledReason = "OTEL_SDK_DISABLED=true"
	case strings.EqualFold(os.Getenv("OTEL_TRACES_EXPORTER"), "none"):
		cfg.DisabledReason = "OTEL_TRACES_EXPORTER=none"
	case !endpointConfigured():
		cfg.DisabledReason = "no OTEL_EXPORTER_OTLP_ENDPOINT (or _TRACES_ENDPOINT)"
	default:
		cfg.Enabled = true
	}
	return cfg
}

func logConfig(log *zap.Logger, cfg Config) {
	// One greppable block so "what did it start with?" is obvious in pod logs.
	log.Info("otel config (effective)",
		zap.Bool("enabled", cfg.Enabled),
		zap.String("disabled_reason", cfg.DisabledReason),
		zap.String("OTEL_SERVICE_NAME", cfg.ServiceName),
		zap.String("service_version", cfg.ServiceVersion),
		zap.String("OTEL_EXPORTER_OTLP_ENDPOINT", envOr("(unset)", "OTEL_EXPORTER_OTLP_ENDPOINT")),
		zap.String("OTEL_EXPORTER_OTLP_TRACES_ENDPOINT", envOr("(unset)", "OTEL_EXPORTER_OTLP_TRACES_ENDPOINT")),
		zap.String("OTEL_EXPORTER_OTLP_PROTOCOL", cfg.Protocol+" (default http/protobuf if unset)"),
		zap.String("OTEL_EXPORTER_OTLP_INSECURE", envOr("(unset)", "OTEL_EXPORTER_OTLP_INSECURE", "OTEL_EXPORTER_OTLP_TRACES_INSECURE")),
		zap.String("OTEL_SDK_DISABLED", envOr("false", "OTEL_SDK_DISABLED")),
		zap.String("OTEL_TRACES_EXPORTER", envOr("(auto)", "OTEL_TRACES_EXPORTER")),
		zap.Float64("OTEL_TRACE_SAMPLE_RATIO", cfg.SampleRatio),
		zap.Bool("OTEL_TRACE_PROBES", cfg.TraceProbes),
		zap.Strings("propagators", cfg.Propagators),
		zap.String("mesh_headers", "traceparent, x-b3-*, uber-trace-id, x-request-id (attr)"),
	)
}

func installPropagators() {
	// Extract/inject in priority order: first match wins on extract for some fields.
	// Composite tries all; W3C first is the modern default, B3/Jaeger for Istio-era traffic.
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
		b3.New(b3.WithInjectEncoding(b3.B3MultipleHeader|b3.B3SingleHeader)),
		jaeger.Jaeger{},
	))
}

// AnnotateMeshHeaders copies common mesh / gateway correlation headers onto the active span
// and returns the request id (if any) for response headers.
func AnnotateMeshHeaders(ctx context.Context, header func(string) string) (requestID string) {
	span := trace.SpanFromContext(ctx)
	if !span.IsRecording() {
		// still surface request id for response even without export
		return firstNonEmpty(
			header("X-Request-Id"),
			header("X-Request-ID"),
			header("X-Correlation-Id"),
			header("X-Correlation-ID"),
		)
	}

	var attrs []attribute.KeyValue
	add := func(hdr, key string) {
		if v := strings.TrimSpace(header(hdr)); v != "" {
			attrs = append(attrs, attribute.String(key, v))
		}
	}

	// Envoy / Istio request id (access log correlation — not the same as OTEL trace id)
	for _, h := range []string{"X-Request-Id", "X-Request-ID"} {
		if v := strings.TrimSpace(header(h)); v != "" {
			attrs = append(attrs, attribute.String("http.request_id", v))
			requestID = v
			break
		}
	}
	for _, h := range []string{"X-Correlation-Id", "X-Correlation-ID"} {
		if v := strings.TrimSpace(header(h)); v != "" {
			attrs = append(attrs, attribute.String("http.correlation_id", v))
			if requestID == "" {
				requestID = v
			}
			break
		}
	}

	// Cloud / product variants (attach if present; propagation may already handle some)
	add("X-Amzn-Trace-Id", "aws.xray.trace_id")
	add("X-Cloud-Trace-Context", "gcp.cloud_trace_context")
	add("X-B3-TraceId", "messaging.b3.trace_id") // also in B3 propagator; useful as attr for search

	if len(attrs) > 0 {
		span.SetAttributes(attrs...)
	}
	return requestID
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

func protocol() string {
	proto := strings.ToLower(strings.TrimSpace(os.Getenv("OTEL_EXPORTER_OTLP_PROTOCOL")))
	if proto == "" {
		proto = strings.ToLower(strings.TrimSpace(os.Getenv("OTEL_EXPORTER_OTLP_TRACES_PROTOCOL")))
	}
	if proto == "" {
		return "http/protobuf"
	}
	if strings.Contains(proto, "grpc") {
		return "grpc"
	}
	return "http/protobuf"
}

func sampleRatio() float64 {
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

func newTraceExporter(ctx context.Context, proto string) (*otlptrace.Exporter, error) {
	if proto == "grpc" {
		return otlptracegrpc.New(ctx)
	}
	return otlptracehttp.New(ctx)
}

func envOr(fallback string, keys ...string) string {
	for _, k := range keys {
		if v, ok := os.LookupEnv(k); ok && strings.TrimSpace(v) != "" {
			return v
		}
	}
	return fallback
}

func firstEnv(keys ...string) string {
	return envOr("", keys...)
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}
