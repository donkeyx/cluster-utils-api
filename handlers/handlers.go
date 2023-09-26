package handlers

import (
	"encoding/json"
	"net"
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
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

func PrometheusMetricsHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		handler := promhttp.Handler()
		handler.ServeHTTP(c.Writer, c.Request)
	}
}

// @Summary Get healthz
// @Description Get the health of the api
// @ID healthz
// @Produce json
// @Success 200 {string} string "OK"
// @Router /healthz [get]
// HealthHandler handles GET requests to /health endpoint and returns a 200 status code with "OK" message.
func HealthzHandler(c *gin.Context) {
	requestsTotal.WithLabelValues("GET", "/healthz", "200").Inc()
	HealthHandler(c)
}

// @Summary Get health
// @Description Get the health of the api
// @ID health
// @Produce json
// @Success 200 {string} string "OK"
// @Router /health [get]
// HealthHandler handles GET requests to /health endpoint and returns a 200 status code with "OK" message.
func HealthHandler(c *gin.Context) {
	requestsTotal.WithLabelValues("GET", "/health", "200").Inc()
	c.String(http.StatusOK, "OK")
}

// @Summary Get readyz
// @Description Get the readyness of the api
// @ID readyz
// @Produce json
// @Success 200 {string} string "OK"
// @Router /readyz [get]
func ReadyzHandler(c *gin.Context) {
	ReadyHandler(c)
}

// @Summary Get ready
// @Description Get the readyness of the api
// @ID ready
// @Produce json
// @Success 200 {string} string "OK"
// @Router /ready [get]
func ReadyHandler(c *gin.Context) {
	isReady := true // not sure what i will do here?

	if isReady {
		c.String(http.StatusOK, "Ready")
	} else {
		c.String(http.StatusServiceUnavailable, "Not Ready")
	}
}

// @Summary Get ping
// @Description Get the readyness of the api
// @ID ping
// @Produce json
// @Success 200 {string} string "PONG"
// @Router /ping [get]
// PingHandler handles the ping endpoint and returns a "PONG" response.
func PingHandler(c *gin.Context) {
	c.String(http.StatusOK, "PONG")
}

// @Summary Get headers
// @Description Get the headers recieved by the api
// @ID headers
// @Produce json
// @Success 200 {string} string "OK"
// @Router /headers [get]
func HeadersHandler(c *gin.Context) {
	headers := make(map[string]string)
	for key, values := range c.Request.Header {
		headers[key] = values[0]
	}

	// Convert headers to JSON
	headersJSON, err := json.Marshal(headers)
	if err != nil {
		c.String(http.StatusInternalServerError, "Error converting headers to JSON")
		return
	}

	c.Data(http.StatusOK, "application/json", headersJSON)
}

// @Summary Get environment variables
// @Description Get the env variables available to the api. This is behind auth
// @ID env
// @Produce json
// @Security BearerAuth
// @Success 200 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Router /env [get]
func EnvHandler(c *gin.Context) {
	envVariables, _ := json.Marshal(GetEnvironmentVariables())
	c.Data(http.StatusOK, "application/json", envVariables)
}

// @Summary Debug
// @Description Get lots of info from running container headers/ips
// @ID debug
// @Produce json
// @Success 200 {string} string "OK"
// @Router /debug [get]
func DebugHandler(c *gin.Context) {

	hostname, _ := os.Hostname()
	sourceIP := getClientIP(c.Request)
	headers := c.Request.Header

	debugInfo := gin.H{
		"Hostname":   hostname,
		"SourceIP":   sourceIP,
		"UserAgent":  headers.Get("User-Agent"),
		"Headers":    headers,
		"RequestURI": c.Request.RequestURI,
	}

	// Return the debug information in the response
	c.JSON(http.StatusOK, debugInfo)
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

// getClientIP extracts the client's IP address from the request.
func getClientIP(r *http.Request) string {
	xForwardedFor := r.Header.Get("X-Forwarded-For")
	if xForwardedFor != "" {
		return xForwardedFor
	}

	if ip, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return ip
	}

	return ""
}
