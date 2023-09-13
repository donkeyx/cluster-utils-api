package main

import (
	"fmt"
	"math/rand"

	"cu-api/routes"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var securityToken string

func main() {
	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()

	useJSON := true

	logger := setupLogger(useJSON)

	// r.Use(middleware.SetupLoggerMiddleware(logger))
	// r.Use(ginzap.Ginzap(logger, time.RFC3339, true))
	// r.Use(ginzap.RecoveryWithZap(logger, true))

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

func setupLogger(useJSON bool) *zap.Logger {

	config := zap.Config{
		Level:            zap.NewAtomicLevelAt(zapcore.InfoLevel),
		Development:      true, // Set this to false in production
		EncoderConfig:    zap.NewProductionEncoderConfig(),
		OutputPaths:      []string{"stdout"},
		ErrorOutputPaths: []string{"stderr"},
	}

	logger, err := config.Build()
	if err != nil {
		panic(err)
	}
	defer logger.Sync()

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
