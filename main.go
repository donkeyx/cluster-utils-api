package main

import (
	"cu-api/middleware"
	"fmt"
	"math/rand"
	"sync"

	"cu-api/routes"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

var (
	isReady       bool
	securityToken string
	readyMu       sync.RWMutex
	healthMu      sync.RWMutex
)

func main() {

	useJSON := true

	logger := setupLogger(useJSON)
	defer logger.Sync()

	securityToken = generateRandomToken(32)

	r := gin.Default()

	r.Use(middleware.SetupLoggerMiddleware(logger))
	r.Use(middleware.AuthMiddleware(logger, securityToken))

	routes.SetupRouter(r)

	logger.Info("Random Security Token", zap.String("token", securityToken))
	logger.Info("Curl Command", zap.String("command", getCurlCommand(8080, securityToken)))

	isReady = true

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

func setupLogger(useJSON bool) *zap.Logger {
	var logger *zap.Logger
	var err error

	if useJSON {
		logger, err = zap.NewProduction()
	} else {
		logger, err = zap.NewDevelopment()
	}

	if err != nil {
		panic(err)
	}
	return logger

}
