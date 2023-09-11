package routes

import (
	"github.com/gin-gonic/gin"
)

func setupRouter(r *gin.Engine) {

	gin.SetMode(gin.ReleaseMode)

	r.Use(authMiddleware(logger))

	r.GET("/health", healthHandler)
	r.GET("/healthz", healthHandler)
	r.GET("/ready", readyHandler)
	r.GET("/readyz", readyHandler)

	r.GET("/headers", headersHandler)
	r.GET("/env", authMiddleware(logger), envHandler)
	r.GET("/debug", debugHandler)

	return r
}
