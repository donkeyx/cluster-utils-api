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
	r.Use(handlers.MetricsMiddleware())

	// Redirect to swagger docs
	r.GET("/", func(c *gin.Context) {
		c.Redirect(http.StatusFound, "/api-docs/index.html")
	})

	r.GET("/api-docs/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	r.GET("/help", handlers.HelpHandler)
	r.GET("/version", handlers.VersionHandler)
	r.GET("/metrics", handlers.PrometheusMetricsHandler())

	// Kube-style probes (plus older aliases)
	// live  = liveness  → restart on fail
	// ready = readiness → leave Service endpoints on fail
	// startup = cold start latch → kube only until first success
	r.GET("/livez", handlers.LiveHandler)
	r.GET("/healthz", handlers.HealthzHandler)
	r.GET("/health", handlers.HealthHandler)

	r.GET("/readyz", handlers.ReadyzHandler)
	r.GET("/ready", handlers.ReadyHandler)

	r.GET("/startupz", handlers.StartupHandler)
	r.GET("/startup", handlers.StartupHandler)

	r.GET("/headers", handlers.HeadersHandler)
	r.GET("/debug", handlers.DebugHandler)
	r.GET("/ping", handlers.PingHandler)

	r.GET("/status/:code", handlers.StatusHandler)
	r.GET("/delay/:seconds", handlers.DelayHandler)
	r.Any("/echo", handlers.EchoHandler)

	authGroup := r.Group("/a")
	authGroup.Use(middleware.AuthMiddleware(logger, st))
	authGroup.GET("/env", handlers.EnvHandler)
	authGroup.GET("/control/probes", handlers.GetProbesHandler)
	authGroup.PUT("/control/probes", handlers.PutProbesHandler)
}
