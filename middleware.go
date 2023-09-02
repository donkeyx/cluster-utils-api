package main

import (
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func loggingMiddleware(logger *zap.SugaredLogger) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Log the request using the sugar logger
		logger.Infow("Request Information",
			"Method", c.Request.Method,
			"Path", c.Request.URL.Path,
			"Query", c.Request.URL.RawQuery,
			"UserAgent", c.Request.UserAgent(),
		)
		c.Set("logger", logger)

		// Continue handling the request
		c.Next()
	}
}
