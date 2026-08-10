package handlers

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Prometheus setup — client_golang is still the k8s scrape standard.
// We use a dedicated registry + standard RED-ish HTTP metrics + Go/process collectors.
// (promhttp.InstrumentHandler* is the other common pattern; middleware fits Gin better.)

var (
	metricsRegistry = prometheus.NewRegistry()

	httpRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total HTTP requests processed.",
		},
		[]string{"method", "path", "status"},
	)

	httpRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request latency in seconds.",
			Buckets: prometheus.DefBuckets, // 5ms … 10s — fine for an API util
		},
		[]string{"method", "path", "status"},
	)

	httpRequestsInFlight = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "http_requests_in_flight",
			Help: "Number of HTTP requests currently being processed.",
		},
	)
)

func init() {
	metricsRegistry.MustRegister(
		httpRequestsTotal,
		httpRequestDuration,
		httpRequestsInFlight,
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	)
}

// MetricsMiddleware records request count, in-flight gauge, and duration.
// Path label uses gin FullPath (route template) so cardinality stays bounded.
func MetricsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// don't instrument the scrape endpoint itself
		if c.Request.URL.Path == "/metrics" {
			c.Next()
			return
		}

		start := time.Now()
		httpRequestsInFlight.Inc()
		c.Next()
		httpRequestsInFlight.Dec()

		path := c.FullPath()
		if path == "" {
			path = "unmatched"
		}
		status := strconv.Itoa(c.Writer.Status())
		method := c.Request.Method

		httpRequestsTotal.WithLabelValues(method, path, status).Inc()
		httpRequestDuration.WithLabelValues(method, path, status).Observe(time.Since(start).Seconds())
	}
}

// @Summary Prometheus metrics
// @Description Prometheus scrape endpoint (Go + process + HTTP RED metrics)
// @ID metrics
// @Produce plain
// @Success 200 {string} string "metrics"
// @Router /metrics [get]
func PrometheusMetricsHandler() gin.HandlerFunc {
	h := promhttp.HandlerFor(metricsRegistry, promhttp.HandlerOpts{
		EnableOpenMetrics: true,
	})
	return func(c *gin.Context) {
		h.ServeHTTP(c.Writer, c.Request)
	}
}
