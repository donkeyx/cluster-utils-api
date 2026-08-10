package routes

import (
	"bytes"
	_ "embed"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"

	"github.com/donkeyx/cluster-utils-api/docs"
	"github.com/donkeyx/cluster-utils-api/handlers"
	"github.com/donkeyx/cluster-utils-api/middleware"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

// swaggerInfoMu guards docs.SwaggerInfo Host/Schemes/Description when serving the UI.
var swaggerInfoMu sync.Mutex

// swaggerBaseDescription is the static @description from swag (no per-request target line).
var (
	swaggerBaseDescription     string
	swaggerBaseDescriptionOnce sync.Once
)

// darkCSS is injected into Swagger UI (gin-swagger has no first-class dark mode).
//
//go:embed swagger-dark.css
var darkCSS []byte

func SetupRouter(logger *zap.Logger, st string, r *gin.Engine) {
	// Middleware (otel / metrics / log / recover) is registered in main before this.

	// Redirect to swagger docs
	r.GET("/", func(c *gin.Context) {
		c.Redirect(http.StatusFound, "/api-docs/index.html")
	})

	// Swagger UI: persist Authorize token; host/scheme for Try-it-out.
	// Priority: ?host=&scheme= query > SWAGGER_HOST / SWAGGER_SCHEME env > request Host / X-Forwarded-Proto.
	// Dark theme by default; ?theme=light for stock Swagger look.
	r.GET("/api-docs/*any", swaggerHandler())

	r.GET("/help", handlers.HelpHandler)
	r.GET("/version", handlers.VersionHandler)
	r.GET("/metrics", handlers.PrometheusMetricsHandler())

	// Kube-style probes (plus older aliases)
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

	// Sensitive / abusable — bearer auth required (see README security section)
	authGroup := r.Group("/a")
	authGroup.Use(middleware.AuthMiddleware(logger, st))
	authGroup.GET("/env", handlers.EnvHandler)
	authGroup.GET("/control/probes", handlers.GetProbesHandler)
	authGroup.PUT("/control/probes", handlers.PutProbesHandler)
	authGroup.GET("/proxy", handlers.ProxyGetHandler)
	authGroup.POST("/proxy", handlers.ProxyPostHandler)
}

func swaggerHandler() gin.HandlerFunc {
	handler := ginSwagger.WrapHandler(
		swaggerFiles.Handler,
		ginSwagger.PersistAuthorization(true),
		ginSwagger.DefaultModelsExpandDepth(-1),
		ginSwagger.DocExpansion("list"),
	)

	return func(c *gin.Context) {
		// Serve our dark stylesheet (relative to /api-docs/)
		any := c.Param("any")
		if any == "/swagger-dark.css" || any == "swagger-dark.css" || strings.HasSuffix(any, "swagger-dark.css") {
			c.Data(http.StatusOK, "text/css; charset=utf-8", darkCSS)
			return
		}

		// Resolve Try-it-out server URL (not the listen bind — that's PORT).
		// 1) query  2) SWAGGER_* env  3) request
		hostSource := "request Host"
		host := strings.TrimSpace(c.Query("host"))
		if host != "" {
			hostSource = "URL ?host="
		} else {
			host = strings.TrimSpace(os.Getenv("SWAGGER_HOST"))
			if host != "" {
				hostSource = "SWAGGER_HOST env"
			} else {
				host = c.Request.Host
			}
		}

		schemeSource := "request / X-Forwarded-Proto"
		scheme := strings.ToLower(strings.TrimSpace(c.Query("scheme")))
		if scheme == "http" || scheme == "https" {
			schemeSource = "URL ?scheme="
		} else {
			scheme = strings.ToLower(strings.TrimSpace(os.Getenv("SWAGGER_SCHEME")))
			if scheme == "http" || scheme == "https" {
				schemeSource = "SWAGGER_SCHEME env"
			} else {
				if c.Request.TLS != nil || strings.EqualFold(c.GetHeader("X-Forwarded-Proto"), "https") {
					scheme = "https"
				} else {
					scheme = "http"
				}
			}
		}

		light := strings.EqualFold(c.Query("theme"), "light")

		// Serialize updates to the global SwaggerInfo used when doc.json is generated.
		swaggerInfoMu.Lock()
		swaggerBaseDescriptionOnce.Do(func() {
			swaggerBaseDescription = docs.SwaggerInfo.Description
		})
		docs.SwaggerInfo.Host = host
		docs.SwaggerInfo.Schemes = []string{scheme}
		docs.SwaggerInfo.BasePath = "/"
		// Live target + how to change it (shows in the Swagger info panel).
		docs.SwaggerInfo.Description = swaggerBaseDescription + fmt.Sprintf(
			"\n\n---\n\n**Try-it-out target (now):** `%s://%s/`  \n"+
				"_host from %s · scheme from %s_  \n\n"+
				"Change for this tab: append query params, e.g.  \n"+
				"`/api-docs/index.html?host=my-svc.ns.svc:8080&scheme=http`  \n"+
				"Or set env `SWAGGER_HOST` / `SWAGGER_SCHEME` once at startup.  \n"+
				"Priority: **query → env → request Host**.",
			scheme, host, hostSource, schemeSource,
		)

		// Capture HTML for index so we can inject dark CSS (stock swagger is bright white).
		if !light && isSwaggerIndex(any) {
			buf := &responseCapture{ResponseWriter: c.Writer, body: &bytes.Buffer{}, status: http.StatusOK}
			c.Writer = buf
			handler(c)
			html := buf.body.String()
			inject := `<link rel="stylesheet" type="text/css" href="./swagger-dark.css">` +
				`<meta name="color-scheme" content="dark">`
			if strings.Contains(html, "</head>") {
				html = strings.Replace(html, "</head>", inject+"</head>", 1)
			} else {
				html = inject + html
			}
			c.Writer = buf.ResponseWriter
			// Drop content-length from capture; write fresh body
			for k, vv := range buf.Header() {
				if strings.EqualFold(k, "Content-Length") {
					continue
				}
				for _, v := range vv {
					c.Writer.Header().Add(k, v)
				}
			}
			c.Writer.Header().Set("Content-Type", "text/html; charset=utf-8")
			c.Writer.WriteHeader(buf.status)
			_, _ = c.Writer.Write([]byte(html))
			swaggerInfoMu.Unlock()
			return
		}

		handler(c)
		swaggerInfoMu.Unlock()
	}
}

func isSwaggerIndex(any string) bool {
	any = strings.TrimPrefix(any, "/")
	return any == "" || any == "index.html" || strings.HasSuffix(any, "/index.html")
}

// responseCapture buffers the handler response so we can rewrite HTML.
type responseCapture struct {
	gin.ResponseWriter
	body   *bytes.Buffer
	status int
}

func (w *responseCapture) Write(b []byte) (int, error) {
	return w.body.Write(b)
}

func (w *responseCapture) WriteString(s string) (int, error) {
	return w.body.WriteString(s)
}

func (w *responseCapture) WriteHeader(statusCode int) {
	w.status = statusCode
}
