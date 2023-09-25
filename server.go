package main

import (
	"fmt"
	"math/rand"

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
	r := gin.New()                             // empty engine
	r.Use(middleware.LoggerMiddleware(logger)) // adds our new middleware
	r.Use(gin.Recovery())

	securityToken = generateRandomToken(32)

	routes.SetupRouter(logger, securityToken, r)

	logger.Info("Random Security Token", zap.String("token", securityToken))
	logger.Info("Curl Command", zap.String("command", getCurlCommand(8080, securityToken)))

	r.Run(":8080")
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
	variable := fmt.Sprintf("curl -H 'Authorization: Bearer %s' http://localhost:8080/env", securityToken)
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
