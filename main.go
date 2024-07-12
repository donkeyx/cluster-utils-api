// @title Cluster Util API
// @version 2.0
// @description This is a util api which lots of endpoints making it easy to test routing/ingress/egress
// @host localhost:8080
// @BasePath /

package main

import (
	"fmt"
	"math/rand"
	"os"
	"strconv"

	"cu-api/middleware"
	"cu-api/routes"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var securityToken string

func main() {

	logger := setupLogger()
	defer logger.Sync()

	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(middleware.LoggerMiddleware(logger))
	r.Use(gin.Recovery())

	port := getEnvOrDefault("PORT", 8080)

	securityToken = generateRandomToken(32)

	routes.SetupRouter(logger, securityToken, r)
	logger.Info("App started on port:", zap.Int("port", port))
	logger.Info("Random Security Token", zap.String("token", securityToken))
	logger.Info("Curl Command", zap.String("command", getCurlCommand(port, securityToken)))

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
	variable := fmt.Sprintf("curl -H 'Authorization: Bearer %s' http://localhost:%d/a/env | jq", securityToken, port)
	return variable
}

func setupLogger() *zap.Logger {

	config := zap.NewProductionConfig()
	config.Encoding = "json"
	config.EncoderConfig.TimeKey = "timestamp"
	config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder // Optional: Use ISO8601 time format

	logger, err := config.Build()
	if err != nil {
		panic(err)
	}

	return logger
}

// getEnvOrDefault retrieves the value of the environment variable named by the key
// or returns the default value if the environment variable is not set.
func getEnvOrDefault(key string, defaultValue int) int {
	if value, exists := os.LookupEnv(key); exists {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}
