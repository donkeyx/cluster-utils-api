package middleware

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

func LoggerMiddleware(logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		// Process the request
		c.Next()

		// Process the request
		latency := time.Since(start)

		fields := []zap.Field{
			zap.String("method", c.Request.Method),
			zap.String("path", c.Request.URL.Path),
			zap.String("ip", c.ClientIP()),
			zap.String("user_agent", c.Request.UserAgent()),
			zap.Int("status", c.Writer.Status()),
			zap.Duration("latency", latency),
		}
		// join mesh access logs ↔ Tempo
		if rid := c.Writer.Header().Get("X-Request-Id"); rid != "" {
			fields = append(fields, zap.String("request_id", rid))
		} else if rid := c.GetHeader("X-Request-Id"); rid != "" {
			fields = append(fields, zap.String("request_id", rid))
		}
		if sc := trace.SpanFromContext(c.Request.Context()).SpanContext(); sc.IsValid() {
			fields = append(fields,
				zap.String("trace_id", sc.TraceID().String()),
				zap.String("span_id", sc.SpanID().String()),
			)
		}

		logger.Info("Request", fields...)
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
