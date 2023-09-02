package main

import (
	"fmt"
	"log"
	"math/rand"
	"sync"

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

	securityToken = generateRandomToken(32)

	logger, err := newSugarLogger(false)
	if err != nil {
		log.Fatal("Error creating logger:", err)
	}

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

func newSugarLogger(useJSON bool) (*zap.SugaredLogger, error) {
	var config zap.Config

	if useJSON {
		config = zap.NewProductionConfig()
	} else {
		config = zap.NewDevelopmentConfig()
	}

	config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

	// Configure the logger to write logs to the terminal
	config.OutputPaths = []string{"stdout"}

	// Create the logger
	logger, err := config.Build()
	if err != nil {
		return nil, err
	}

	// Create a sugar logger from the base logger
	sugarLogger := logger.Sugar()

	return sugarLogger, nil
}
