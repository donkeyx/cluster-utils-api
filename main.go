package main

import (
	"encoding/json"
	"github.com/gin-gonic/gin"
	"net/http"
	"sync"
	"time"
)

var (
	isReady  bool
	readyMu  sync.RWMutex
	healthMu sync.RWMutex
)

func main() {
	// Create a new Gin router
	router := gin.Default()

	// Set initial readiness state to true
	isReady = true

	// Define routes and handlers
	router.GET("/", helloHandler)
	router.GET("/health", healthHandler)
	router.GET("/ready", readyHandler)
	router.GET("/headers", headersHandler)

	// Start the server
	port := "8080"
	router.Run(":" + port)
}

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

