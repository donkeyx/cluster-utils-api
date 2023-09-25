package middleware

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func LoggerMiddleware(logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		// Process the request
		c.Next()

		// Log request and response details
		end := time.Now()
		latency := end.Sub(start)

		logger.Info("Request",
			zap.String("method", c.Request.Method),
			zap.String("path", c.Request.URL.Path),
			zap.String("ip", c.ClientIP()),
			zap.String("user_agent", c.Request.UserAgent()),
			zap.Int("status", c.Writer.Status()),
			zap.Duration("latency", latency),
		)
	}
}

func AuthMiddleware(logger *zap.Logger, st string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Retrieve the logger from the Gin context
		authHeader := c.GetHeader("Authorization")

		if authHeader != "Bearer "+st {
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
