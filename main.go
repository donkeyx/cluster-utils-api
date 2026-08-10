// @title Cluster Util API
// @version 2.0
// @description Drop-in HTTP util for testing probes, routing, headers, env/params and more in a cluster. Swagger "Try it out" uses the host you opened the UI on (or override with ?host=host:port&scheme=http on /api-docs/index.html). Authorize with Bearer token from logs or AUTH_TOKEN.
// @BasePath /
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description Paste: Bearer <token>  (token from container logs, or AUTH_TOKEN env). Example: Bearer dev

package main

import (
	"fmt"
	"math/rand"
	"os"
	"strconv"

	"github.com/donkeyx/cluster-utils-api/handlers"
	"github.com/donkeyx/cluster-utils-api/middleware"
	"github.com/donkeyx/cluster-utils-api/routes"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Set via -ldflags at build time (see Makefile).
var (
	Version = "dev"
	GitHash = "unknown"
)

var securityToken string

func main() {
	logger := setupLogger()
	defer logger.Sync()

	handlers.SetBuildInfo(Version, GitHash)
	handlers.InitProbesFromEnv()

	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(middleware.LoggerMiddleware(logger))
	r.Use(gin.Recovery())

	port := getEnvOrDefault("PORT", 8080)

	// Optional fixed token for automated tests; otherwise random each start.
	if t := os.Getenv("AUTH_TOKEN"); t != "" {
		securityToken = t
	} else {
		securityToken = generateRandomToken(32)
	}

	routes.SetupRouter(logger, securityToken, r)
	logger.Info("App started",
		zap.Int("port", port),
		zap.String("version", Version),
		zap.String("gitHash", GitHash),
	)
	// Greppable startup lines so people can find the bearer for /a/* endpoints.
	logger.Info("auth token for /a/* endpoints (Authorization: Bearer <token>)",
		zap.String("token", securityToken),
		zap.String("header", "Authorization: Bearer "+securityToken),
	)
	logger.Info("example curl with auth",
		zap.String("env", getCurlCommand(port, securityToken)),
		zap.String("probes", fmt.Sprintf("curl -sS -H 'Authorization: Bearer %s' http://localhost:%d/a/control/probes | jq", securityToken, port)),
	)

	r.Run(fmt.Sprintf(":%d", port))
}

func generateRandomToken(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	token := make([]byte, length)
	for i := 0; i < length; i++ {
		token[i] = charset[rand.Intn(len(charset))]
	}
	return string(token)
}

func getCurlCommand(port int, securityToken string) string {
	return fmt.Sprintf("curl -H 'Authorization: Bearer %s' http://localhost:%d/a/env | jq", securityToken, port)
}

func setupLogger() *zap.Logger {
	config := zap.NewProductionConfig()
	config.Encoding = "json"
	config.EncoderConfig.TimeKey = "timestamp"
	config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

	logger, err := config.Build()
	if err != nil {
		panic(err)
	}
	return logger
}

func getEnvOrDefault(key string, defaultValue int) int {
	if value, exists := os.LookupEnv(key); exists {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}
