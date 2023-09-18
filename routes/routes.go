package routes

import (
	"cu-api/handlers"
	"cu-api/middleware"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func SetupRouter(logger *zap.Logger, st string, r *gin.Engine) {

	r.GET("/health", handlers.HealthHandler)
	r.GET("/healthz", handlers.HealthHandler)
	r.GET("/ready", handlers.ReadyHandler)
	r.GET("/readyz", handlers.ReadyHandler)

	r.GET("/headers", handlers.HeadersHandler)
	r.GET("/debug", handlers.DebugHandler)

	authGroup := r.Group("/a")
	authGroup.Use(middleware.AuthMiddleware(logger, st))
	authGroup.GET("/env", handlers.EnvHandler)

}
