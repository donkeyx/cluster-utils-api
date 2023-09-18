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

	useJSON := true

	logger := setupLogger(useJSON)
	defer logger.Sync()

	gin.SetMode(gin.ReleaseMode)
	r := gin.New()                             // empty engine
	r.Use(middleware.LoggerMiddleware(logger)) // adds our new middleware
	r.Use(gin.Recovery())

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

	// var logger *zap.Logger
	// var err error
	// if useJSON {
	// 	logger, err = zap.NewProduction()
	// } else {
	// 	fmt.Print("Using Development Logger")
	// 	logger, err = zap.NewDevelopment()
	// }
	// if err != nil {
	// 	panic(err)
	// }

	config := zap.NewProductionConfig()
	config.Encoding = "json"
	config.EncoderConfig.TimeKey = "timestamp"
	config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder // Optional: Use ISO8601 time format

	// // config := zap.Config{
	// // 	Level:            zap.NewAtomicLevelAt(zapcore.InfoLevel),
	// // 	Development:      true, // Set this to false in production
	// // 	EncoderConfig:    encoderConfig,
	// // 	OutputPaths:      []string{"stdout"},
	// // 	ErrorOutputPaths: []string{"stderr"},
	// // }

	logger, err := config.Build()
	if err != nil {
		panic(err)
	}

	return logger
}
