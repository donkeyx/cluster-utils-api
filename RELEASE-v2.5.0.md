## cluster-utils-api v2.5.0 — OpenTelemetry traces

Adds OTLP **trace export** (push). Metrics are unchanged and still scraped.

### Push vs scrape (read this twice)

| Signal | How | Path |
|--------|-----|------|
| **Metrics** | **Scrape** (pull) | `GET /metrics` → Alloy/Prometheus → Mimir |
| **Traces** | **Push** | OTLP → Alloy → **Tempo** |

Tempo does **not** scrape `/metrics` for spans. The app exports OTLP when configured.

### Mesh / Istio — request id vs trace id

This is the detail that usually bites people:

- **`x-request-id`** (Envoy/Istio) is a **request correlation id** for access logs.
- **`traceparent` / OTEL trace id** (`X-Trace-Id` on our responses) is the **distributed trace** id in Tempo.
- They are **not the same value**. Both matter.

**What we do:**

- Propagate **W3C + B3 + Jaeger + baggage** so modern *and* Istio-era meshes join the same trace.
- Attach mesh ids as span attributes (`http.request_id`, correlation ids, …).
- Echo **`X-Trace-Id`** and **`X-Request-Id`** (when present) on responses.
- Put `trace_id` + `request_id` on JSON request logs so you can join Envoy logs ↔ Tempo.

**How to join:** Tempo search by `X-Trace-Id`, or by attribute `http.request_id` = Envoy’s id.

Other meshes (Linkerd, etc.) follow the same pattern: W3C is the normal default; B3 still shows up; product-specific headers are extra correlation.

### Enable (defaults)

Tracing is a **no-op** until an OTLP endpoint is set.

| Variable | Default |
|----------|---------|
| export | off until `OTEL_EXPORTER_OTLP_ENDPOINT` (or traces endpoint) |
| protocol | `http/protobuf` (Alloy **:4318**) |
| service name | `cluster-utils-api` |
| sample ratio | `1.0` |
| probe spans | off (`OTEL_TRACE_PROBES=true` to include `/livez` etc.) |
| propagators | W3C + baggage + B3 + Jaeger |

```yaml
OTEL_SERVICE_NAME=cluster-utils-api
OTEL_EXPORTER_OTLP_ENDPOINT=alloy.observability.svc:4318
OTEL_EXPORTER_OTLP_PROTOCOL=http/protobuf
OTEL_EXPORTER_OTLP_INSECURE=true
```

On **every startup** logs include `otel config (effective)` — grep pod logs for `otel config` to see exactly what it started with.

### What is instrumented

- Inbound HTTP (Gin), with mesh header attributes
- Outbound `/a/proxy` (linked east-west spans + header inject)
- Probe/metrics/ping paths skipped unless `OTEL_TRACE_PROBES=true`

### Images

- `donkeyx/cluster-utils-api:2.5.0` / `:latest`
- `ghcr.io/donkeyx/cluster-utils-api:2.5.0` / `:latest`
