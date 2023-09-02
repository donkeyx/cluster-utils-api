package main

import (
	"fmt"
	"math/rand"
	"sync"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	isReady       bool
	securityToken string
	readyMu       sync.RWMutex
	healthMu      sync.RWMutex
)

func main() {

	rand.Seed(time.Now().UnixNano())
	securityToken = generateRandomToken(32)

	// Initialize the logger with JSON output
	config := zap.NewProductionConfig()
	config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	logger, _ := config.Build()
	defer logger.Sync()

	logger.Info("Random Security Token", zap.String("token", securityToken))
	logCurlCommand(logger)

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

func logCurlCommand(logger *zap.Logger) {
	// Log the curl command for the /env endpoint
	curlCommand := fmt.Sprintf("curl -H 'Authorization: Bearer %s' http://localhost:8080/env", securityToken)
	logger.Info("Curl Command", zap.String("command", curlCommand))
}
