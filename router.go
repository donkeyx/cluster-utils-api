package main

import (
	"net/http"
	"time"

	ginzap "github.com/gin-contrib/zap"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func setupRouter(logger *zap.Logger) *gin.Engine {

	r := gin.Default()
	r.Use(ginzap.Ginzap(logger, time.RFC3339, true))

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

	return r
}
