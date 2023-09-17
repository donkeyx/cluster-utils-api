package middleware

import (
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func LoggerMiddleware(logger *zap.Logger) gin.HandlerFunc {

	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.TimeKey = "timestamp"
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

	// Create a Zap logger with a JSON encoder
	logger = zap.New(zapcore.NewCore(
		zapcore.NewJSONEncoder(encoderConfig),
		zapcore.AddSync(os.Stdout),
		zap.NewAtomicLevelAt(zap.InfoLevel),
	), zap.AddCaller())

	return func(c *gin.Context) {
		// Attach the logger to the context for access in handlers
		c.Set("zapLogger", logger)
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
