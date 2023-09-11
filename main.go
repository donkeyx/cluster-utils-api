package main

import (
	"cu-api/middleware"
	"fmt"
	"math/rand"
	"sync"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"gorm.io/gorm/logger"
)

var (
	isReady       bool
	securityToken string
	readyMu       sync.RWMutex
	healthMu      sync.RWMutex
)

func main() {

	r := gin.Default()

	useJSONOutput := true // Set this to false for non-JSON output
	r.Use(middleware.SetupLoggerMiddleware(useJSONOutput))

	routes.setupRouter(r)

	securityToken = generateRandomToken(32)

	logger.Info("Random Security Token", zap.String("token", securityToken))
	logger.Info("Curl Command", zap.String("command", getCurlCommand(8080, securityToken)))

	isReady = true

	r := setupRouter(logger)
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
