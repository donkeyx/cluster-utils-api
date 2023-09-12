package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func SetupLoggerMiddleware(logger *zap.Logger) gin.HandlerFunc {

	return func(c *gin.Context) {
		// Attach the logger to the context for access in handlers
		c.Set("logger", logger)

		// Continue with the request
		c.Next()
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
