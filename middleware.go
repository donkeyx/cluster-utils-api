package main

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func loggingMiddleware(logger *zap.SugaredLogger) gin.HandlerFunc {
	return func(c *gin.Context) {

		logger.Infow("Request Information",
			"Method", c.Request.Method,
			"Path", c.Request.URL.Path,
			"Query", c.Request.URL.RawQuery,
			"UserAgent", c.Request.UserAgent(),
		)
		c.Set("logger", logger)

		c.Next()
	}
}

func authMiddleware(logger *zap.SugaredLogger) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Retrieve the logger from the Gin context
		c.Set("logger", logger)

		authHeader := c.GetHeader("Authorization")

		if authHeader != "Bearer "+securityToken {
			// Log unauthorized request
			logger.Info("Unauthorized request")

			c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
			c.Abort()
			return
		}

		// Continue with the next middleware or handler
		c.Next()
	}
}

func getOrCreateLogger(c *gin.Context) (*zap.SugaredLogger, bool) {
	// Retrieve the logger from the Gin context
	logger, exists := c.Get("logger")
	if !exists {
		return nil, false
	}

	// Cast the logger to *zap.SugaredLogger
	sugarLogger, ok := logger.(*zap.SugaredLogger)
	if !ok {
		return nil, false
	}

	return sugarLogger, true
}
