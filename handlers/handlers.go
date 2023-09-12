package handlers

import (
	"encoding/json"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
)

func HealthHandler(c *gin.Context) {
	c.String(http.StatusOK, "OK")
}

func ReadyHandler(c *gin.Context) {
	var readyMu sync.RWMutex

	readyMu.RLock()
	defer readyMu.RUnlock()

	if readyMu {
		c.String(http.StatusOK, "Ready")
	} else {
		c.String(http.StatusServiceUnavailable, "Not Ready")
	}
}

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

// functions for the routers

func EnvHandler(c *gin.Context) {
	envVariables, _ := json.Marshal(GetEnvironmentVariables())
	c.Data(http.StatusOK, "application/json", envVariables)
}

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
func GetClientIP(r *http.Request) string {
	xForwardedFor := r.Header.Get("X-Forwarded-For")
	if xForwardedFor != "" {
		return xForwardedFor
	}

	if ip, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return ip
	}

	return ""
}

func DebugHandler(c *gin.Context) {
	// Retrieve the logger from the Gin context
	logger := c.MustGet("logger").(*sugar.Logger)

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

	// Log the debug information
	logger.Infow("Debug Information", debugInfo)

	// Return the debug information in the response
	c.JSON(http.StatusOK, debugInfo)
}
