// @title Cluster Util API
// @version 2.0
// @description Drop-in HTTP util for testing probes, routing, headers, env/params and more in a cluster. Swagger "Try it out" uses the host you opened the UI on (or override with ?host=host:port&scheme=http on /api-docs/index.html). Authorize with Bearer token from logs or AUTH_TOKEN.
// @BasePath /
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description Use the top-right Authorize lock only (not a per-operation Authorization field). Value MUST be exactly: Bearer <token>  e.g. local: Bearer dev

package main

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"strconv"
	"time"

	"github.com/donkeyx/cluster-utils-api/handlers"
	"github.com/donkeyx/cluster-utils-api/middleware"
	"github.com/donkeyx/cluster-utils-api/otelsetup"
	"github.com/donkeyx/cluster-utils-api/routes"

	"net/http"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Set via -ldflags at build time (see Makefile).
var (
	Version = "dev"
	GitHash = "unknown"
)

var securityToken string

func main() {
	logger := setupLogger()
	defer logger.Sync()

	handlers.SetBuildInfo(Version, GitHash)
	handlers.InitProbesFromEnv()

	// Traces: OTLP *push* to Alloy/collector (not scraped). No-op if endpoint unset.
	ctx := context.Background()
	otelShutdown, err := otelsetup.Init(ctx, "cluster-utils-api", Version, logger)
	if err != nil {
		logger.Fatal("otel init failed", zap.Error(err))
	}
	defer func() {
		shCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := otelShutdown(shCtx); err != nil {
			logger.Warn("otel shutdown", zap.Error(err))
		}
	}()

	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	// Order: create span → mesh attrs + echo ids → metrics → logs → recover
	r.Use(otelgin.Middleware("cluster-utils-api", otelgin.WithFilter(otelPathFilter)))
	r.Use(traceAndRequestIDHeaders())
	r.Use(handlers.MetricsMiddleware())
	r.Use(middleware.LoggerMiddleware(logger))
	r.Use(gin.Recovery())

	port := getEnvOrDefault("PORT", 8080)

	// Optional fixed token for automated tests; otherwise random each start.
	if t := os.Getenv("AUTH_TOKEN"); t != "" {
		securityToken = t
	} else {
		securityToken = generateRandomToken(32)
	}

	routes.SetupRouter(logger, securityToken, r)
	logger.Info("App started",
		zap.Int("port", port),
		zap.String("version", Version),
		zap.String("gitHash", GitHash),
	)
	// Greppable startup lines so people can find the bearer for /a/* endpoints.
	logger.Info("auth token for /a/* endpoints (Authorization: Bearer <token>)",
		zap.String("token", securityToken),
		zap.String("header", "Authorization: Bearer "+securityToken),
	)
	logger.Info("example curl with auth",
		zap.String("env", getCurlCommand(port, securityToken)),
		zap.String("probes", fmt.Sprintf("curl -sS -H 'Authorization: Bearer %s' http://localhost:%d/a/control/probes | jq", securityToken, port)),
	)

	r.Run(fmt.Sprintf(":%d", port))
}

func generateRandomToken(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	token := make([]byte, length)
	for i := 0; i < length; i++ {
		token[i] = charset[rand.Intn(len(charset))]
	}
	return string(token)
}

func getCurlCommand(port int, securityToken string) string {
	return fmt.Sprintf("curl -H 'Authorization: Bearer %s' http://localhost:%d/a/env | jq", securityToken, port)
}

func setupLogger() *zap.Logger {
	config := zap.NewProductionConfig()
	config.Encoding = "json"
	config.EncoderConfig.TimeKey = "timestamp"
	config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

	logger, err := config.Build()
	if err != nil {
		panic(err)
	}
	return logger
}

func getEnvOrDefault(key string, defaultValue int) int {
	if value, exists := os.LookupEnv(key); exists {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

// otelPathFilter skips ultra-noisy probes/metrics by default so Tempo isn't flooded.
// Set OTEL_TRACE_PROBES=true to include them when you're debugging probe behaviour itself.
func otelPathFilter(r *http.Request) bool {
	if os.Getenv("OTEL_TRACE_PROBES") == "true" {
		return true
	}
	switch r.URL.Path {
	case "/livez", "/readyz", "/startupz",
		"/health", "/healthz", "/ready", "/startup",
		"/metrics", "/ping":
		return false
	default:
		return true
	}
}

// traceAndRequestIDHeaders:
//   - attaches Istio/Envoy x-request-id (and friends) onto the span
//   - echoes X-Trace-Id (OTEL) and X-Request-Id (mesh) on the response for easy curl ↔ Tempo/access-log joins
func traceAndRequestIDHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		reqID := otelsetup.AnnotateMeshHeaders(c.Request.Context(), c.GetHeader)
		c.Next()

		sc := trace.SpanFromContext(c.Request.Context()).SpanContext()
		if sc.IsValid() {
			c.Writer.Header().Set("X-Trace-Id", sc.TraceID().String())
		}
		// Prefer inbound mesh id; otherwise leave empty (don't invent — Envoy owns that space)
		if reqID != "" {
			c.Writer.Header().Set("X-Request-Id", reqID)
		} else if inbound := c.GetHeader("X-Request-Id"); inbound != "" {
			c.Writer.Header().Set("X-Request-Id", inbound)
		}
	}
}
