package middleware

import (
	"net/http"

	"github.com/getsentry/sugar"
	"github.com/getsentry/sugar/console"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func SetupLoggerMiddleware(useJSON bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Create a new Sugar logger
		logger, _ := sugar.New()

		if useJSON {
			// Configure the logger output to use JSON format
			logger.SetFormatter(sugar.NewJSONFormatter())
		} else {
			// Configure the logger output to use the console writer
			logger.SetWriter(console.New())
		}

		// You can customize other logger settings here, such as log level, etc.
		// For example:
		// logger.SetLevel(sugar.InfoLevel)

		// Attach the logger to the context for access in handlers
		c.Set("logger", logger)

		// Continue with the request
		c.Next()
	}
}

func AuthMiddleware(logger *zap.SugaredLogger) gin.HandlerFunc {
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
