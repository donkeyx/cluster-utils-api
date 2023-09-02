package main

import (
	"encoding/json"
	"net"
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func helloHandler(c *gin.Context) {
	c.String(http.StatusOK, "Hello, World!")
}

func healthHandler(c *gin.Context) {
	// You can add more complex health checks here
	c.String(http.StatusOK, "OK")
}

func readyHandler(c *gin.Context) {
	readyMu.RLock()
	defer readyMu.RUnlock()

	if isReady {
		c.String(http.StatusOK, "Ready")
	} else {
		c.String(http.StatusServiceUnavailable, "Not Ready")
	}
}

func headersHandler(c *gin.Context) {
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
func authMiddleware(c *gin.Context) {
	authHeader := c.GetHeader("Authorization")

	if authHeader != "Bearer "+securityToken {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		c.Abort()
		return
	}

	c.Next()
}

func getEnvironmentVariables() map[string]string {
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

func debugHandler(c *gin.Context) {
	logger, _ := c.Get("logger")
	sugarLogger, _ := logger.(*zap.SugaredLogger)

	hostname, _ := os.Hostname()
	sourceIP := getClientIP(c.Request)
	headers := c.Request.Header

	sugarLogger.Infow("Debug Information",
		"Hostname", hostname,
		"SourceIP", sourceIP,
		"UserAgent", headers.Get("User-Agent"),
		"Headers", headers,
		"RequestURI", c.Request.RequestURI,
	)

	c.JSON(http.StatusOK, gin.H{"message": "Debug information logged"})
}
