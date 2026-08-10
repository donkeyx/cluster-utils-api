package routes

import (
	"cu-api/docs"
	"cu-api/handlers"
	"cu-api/middleware"
	"net/http"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

// swaggerInfoMu guards docs.SwaggerInfo Host/Schemes when serving the UI for different origins.
var swaggerInfoMu sync.Mutex

func SetupRouter(logger *zap.Logger, st string, r *gin.Engine) {
	r.Use(handlers.MetricsMiddleware())

	// Redirect to swagger docs
	r.GET("/", func(c *gin.Context) {
		c.Redirect(http.StatusFound, "/api-docs/index.html")
	})

	// Swagger UI: persist Authorize token in the browser; host/scheme follow where you opened
	// the page (so port-forward / docker / cluster DNS all work). Optional overrides:
	//   /api-docs/index.html?host=my-svc:8080&scheme=http
	r.GET("/api-docs/*any", swaggerHandler())

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
	// east-west hop: north-south hits us, we call another svc (headers forwarded by default)
	r.GET("/proxy", handlers.ProxyHandler)
	r.POST("/proxy", handlers.ProxyHandler)

	authGroup := r.Group("/a")
	authGroup.Use(middleware.AuthMiddleware(logger, st))
	authGroup.GET("/env", handlers.EnvHandler)
	authGroup.GET("/control/probes", handlers.GetProbesHandler)
	authGroup.PUT("/control/probes", handlers.PutProbesHandler)
}

func swaggerHandler() gin.HandlerFunc {
	// Empty host in the generated spec would also work; we set Host from the request
	// so the Swagger top bar shows a real target and Try it out hits the right place.
	handler := ginSwagger.WrapHandler(
		swaggerFiles.Handler,
		ginSwagger.PersistAuthorization(true),
		ginSwagger.DefaultModelsExpandDepth(-1),
	)

	return func(c *gin.Context) {
		host := strings.TrimSpace(c.Query("host"))
		if host == "" {
			host = c.Request.Host
		}

		scheme := strings.ToLower(strings.TrimSpace(c.Query("scheme")))
		if scheme != "http" && scheme != "https" {
			if c.Request.TLS != nil || c.GetHeader("X-Forwarded-Proto") == "https" {
				scheme = "https"
			} else {
				scheme = "http"
			}
		}

		// Serialize updates to the global SwaggerInfo used when doc.json is generated.
		swaggerInfoMu.Lock()
		docs.SwaggerInfo.Host = host
		docs.SwaggerInfo.Schemes = []string{scheme}
		docs.SwaggerInfo.BasePath = "/"
		handler(c)
		swaggerInfoMu.Unlock()
	}
}
