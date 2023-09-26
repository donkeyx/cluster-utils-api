package routes

import (
	"cu-api/handlers"
	"cu-api/middleware"
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	_ "cu-api/docs"

	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

func SetupRouter(logger *zap.Logger, st string, r *gin.Engine) {

	// Redirect to swagger docs
	r.GET("/", func(c *gin.Context) {
		c.Redirect(http.StatusFound, "/api-docs/index.html")
	})

	r.GET("/api-docs/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	r.GET("/health", handlers.HealthHandler)
	r.GET("/healthz", handlers.HealthzHandler)
	r.GET("/ready", handlers.ReadyHandler)
	r.GET("/readyz", handlers.ReadyzHandler)

	r.GET("/headers", handlers.HeadersHandler)
	r.GET("/debug", handlers.DebugHandler)
	r.GET("/ping", handlers.PingHandler)

	authGroup := r.Group("/a")
	authGroup.Use(middleware.AuthMiddleware(logger, st))
	authGroup.GET("/env", handlers.EnvHandler)
}
