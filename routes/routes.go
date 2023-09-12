package routes

import (
	"cu-api/middleware"

	"github.com/gin-gonic/gin"
)

func SetupRouter(r *gin.Engine) {

	gin.SetMode(gin.ReleaseMode)

	r.Use(middleware.AuthMiddleware(logger))

	r.GET("/health", HealthHandler)
	r.GET("/healthz", HealthHandler)
	r.GET("/ready", ReadyHandler)
	r.GET("/readyz", ReadyHandler)

	r.GET("/headers", HeadersHandler)
	r.GET("/env", authMiddleware(logger), EnvHandler)
	r.GET("/debug", DebugHandler)

	return r
}
