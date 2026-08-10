package handlers

import (
	"io"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Build info injected from main (ldflags).
var (
	AppVersion = "dev"
	AppGitHash = "unknown"
)

var (
	requestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests.",
		},
		[]string{"method", "path", "status"},
	)
)

func init() {
	prometheus.MustRegister(requestsTotal)
}

// SetBuildInfo lets main push version/git hash from ldflags.
func SetBuildInfo(version, gitHash string) {
	if version != "" {
		AppVersion = version
	}
	if gitHash != "" {
		AppGitHash = gitHash
	}
}

// MetricsMiddleware counts requests for /metrics.
func MetricsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
		path := c.FullPath()
		if path == "" {
			path = c.Request.URL.Path
		}
		requestsTotal.WithLabelValues(c.Request.Method, path, strconv.Itoa(c.Writer.Status())).Inc()
	}
}

// @Summary Prometheus metrics
// @Description Prometheus scrape endpoint
// @ID metrics
// @Produce plain
// @Success 200 {string} string "metrics"
// @Router /metrics [get]
func PrometheusMetricsHandler() gin.HandlerFunc {
	h := promhttp.Handler()
	return func(c *gin.Context) {
		h.ServeHTTP(c.Writer, c.Request)
	}
}

// @Summary Help
// @Description Quick map of useful routes
// @ID help
// @Produce json
// @Success 200 {object} map[string]string
// @Router /help [get]
func HelpHandler(c *gin.Context) {
	c.JSON(http.StatusOK, map[string]string{
		"/":                    "redirect to swagger docs",
		"/api-docs/*":          "swagger ui",
		"/help":                "GET this list",
		"/version":             "GET build version / git hash",
		"/livez|/healthz|/health": "GET liveness (LIVE_MODE / LIVE_DELAY)",
		"/readyz|/ready":       "GET readiness (READY_MODE / READY_DELAY)",
		"/startupz|/startup":   "GET startup latch (STARTUP_* / boot delay)",
		"/a/control/probes":    "GET/PUT probe state (bearer auth)",
		"/ping":                "GET PONG",
		"/headers":             "GET request headers",
		"/debug":               "GET hostname / ip / headers / uri",
		"/metrics":             "GET prometheus metrics",
		"/status/:code":        "GET respond with that http status",
		"/delay/:seconds":      "GET sleep then 200 (max 30s)",
		"/echo":                "GET/POST echo method, headers, query, body",
		"/a/env":               "GET env vars (bearer auth)",
	})
}

// @Summary Version / build info
// @Description What binary is running in this env
// @ID version
// @Produce json
// @Success 200 {object} map[string]string
// @Router /version [get]
func VersionHandler(c *gin.Context) {
	hostname, _ := os.Hostname()
	c.JSON(http.StatusOK, gin.H{
		"version":  AppVersion,
		"gitHash":  AppGitHash,
		"hostname": hostname,
	})
}

// @Summary Get ping
// @Description Simple alive check (not a kube probe — use /livez for that)
// @ID ping
// @Produce plain
// @Success 200 {string} string "PONG"
// @Router /ping [get]
func PingHandler(c *gin.Context) {
	c.String(http.StatusOK, "PONG")
}

// @Summary Get headers
// @Description Headers as seen by the app (handy behind ingress/ALB)
// @ID headers
// @Produce json
// @Success 200 {object} map[string]string
// @Router /headers [get]
func HeadersHandler(c *gin.Context) {
	headers := make(map[string]string)
	for key, values := range c.Request.Header {
		headers[key] = values[0]
	}
	c.JSON(http.StatusOK, headers)
}

// @Summary Get environment variables
// @Description Env dump so you can check secrets/configmaps/task params actually landed. Behind auth under /a/
// @ID env
// @Produce json
// @Security BearerAuth
// @Success 200 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Router /a/env [get]
// @Param Authorization header string true "Bearer token from app logs" default(Bearer )
func EnvHandler(c *gin.Context) {
	c.JSON(http.StatusOK, GetEnvironmentVariables())
}

// @Summary Debug
// @Description Hostname, client ip, headers, uri — good for routing tests
// @ID debug
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /debug [get]
func DebugHandler(c *gin.Context) {
	hostname, _ := os.Hostname()
	c.JSON(http.StatusOK, gin.H{
		"Hostname":   hostname,
		"SourceIP":   getClientIP(c.Request),
		"UserAgent":  c.Request.Header.Get("User-Agent"),
		"Headers":    c.Request.Header,
		"RequestURI": c.Request.RequestURI,
		"Method":     c.Request.Method,
	})
}

// @Summary Fixed status code
// @Description Respond with whatever http status you pass (100-599). Great for ingress/retry testing
// @ID status
// @Produce plain
// @Param code path int true "HTTP status code"
// @Success 200 {string} string "status body"
// @Router /status/{code} [get]
func StatusHandler(c *gin.Context) {
	code, err := strconv.Atoi(c.Param("code"))
	if err != nil || code < 100 || code > 599 {
		c.String(http.StatusBadRequest, "code must be an int between 100 and 599")
		return
	}
	c.String(code, "status=%d", code)
}

// @Summary Delay then OK
// @Description Sleep N seconds (max 30) then return 200. For probe timeouts prefer LIVE/READY/STARTUP delaySeconds instead
// @ID delay
// @Produce plain
// @Param seconds path number true "seconds to sleep (max 30)"
// @Success 200 {string} string "delayed"
// @Router /delay/{seconds} [get]
func DelayHandler(c *gin.Context) {
	raw := c.Param("seconds")
	secs, err := strconv.ParseFloat(raw, 64)
	if err != nil || secs < 0 {
		c.String(http.StatusBadRequest, "seconds must be a number >= 0")
		return
	}
	secs = clampDelay(secs)
	time.Sleep(time.Duration(secs * float64(time.Second)))
	c.String(http.StatusOK, "delayed=%.3fs", secs)
}

// @Summary Echo request
// @Description Bounce method, path, query, headers and body back as json
// @ID echo
// @Accept plain
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /echo [get]
// @Router /echo [post]
// @Router /echo [put]
// @Router /echo [patch]
// @Router /echo [delete]
func EchoHandler(c *gin.Context) {
	body, _ := io.ReadAll(io.LimitReader(c.Request.Body, 1<<20)) // 1MB cap
	headers := make(map[string][]string, len(c.Request.Header))
	for k, v := range c.Request.Header {
		headers[k] = v
	}
	c.JSON(http.StatusOK, gin.H{
		"method":  c.Request.Method,
		"path":    c.Request.URL.Path,
		"query":   c.Request.URL.Query(),
		"headers": headers,
		"body":    string(body),
	})
}

// GetEnvironmentVariables returns a map of all environment variables
func GetEnvironmentVariables() map[string]string {
	envVariables := make(map[string]string)
	for _, e := range os.Environ() {
		pair := strings.SplitN(e, "=", 2)
		if len(pair) == 2 {
			envVariables[pair[0]] = pair[1]
		}
	}
	return envVariables
}

func isTruthy(s string) bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "0", "false", "no", "off", "unhealthy", "notready", "not-ready", "not ready":
		return false
	default:
		return true
	}
}

func getClientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		return strings.TrimSpace(strings.Split(xff, ",")[0])
	}
	if ip, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return ip
	}
	return r.RemoteAddr
}
