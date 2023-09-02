package main

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func setupRouter(logger *zap.SugaredLogger) *gin.Engine {

	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()

	r.Use(loggingMiddleware(logger))

	r.GET("/", helloHandler)
	r.GET("/health", healthHandler)
	r.GET("/healthz", healthHandler)
	r.GET("/ready", readyHandler)
	r.GET("/readyz", readyHandler)

	r.GET("/headers", headersHandler)
	r.GET("/env", authMiddleware, func(c *gin.Context) {
		envVariables := getEnvironmentVariables()
		c.JSON(http.StatusOK, envVariables)
	})
	r.GET("/debug", debugHandler)

	return r
}
