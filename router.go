package main

import (
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func setupRouter(logger *zap.SugaredLogger) *gin.Engine {

	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()

	r.Use(gin.LoggerWithConfig(gin.LoggerConfig{
		Output: logger.Writer(),
	}))

	r.Use(loggingMiddleware(logger))
	r.Use(authMiddleware(logger))

	r.GET("/health", healthHandler)
	r.GET("/healthz", healthHandler)
	r.GET("/ready", readyHandler)
	r.GET("/readyz", readyHandler)

	r.GET("/headers", headersHandler)
	r.GET("/env", authMiddleware(logger), envHandler)
	r.GET("/debug", debugHandler)

	return r
}
