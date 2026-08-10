# Variables
BINARY_PATH := ./bin
BINARY_NAME := cu-api
VERSION := $(shell git describe --tags 2>/dev/null || echo dev)
GIT_HASH := $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
BUILD_DATE := $(shell date)
BUILD_FLAGS := -ldflags="-w -s -X main.Version=$(VERSION) -X main.GitHash=$(GIT_HASH)"

# Pin tool versions (reproducible local + CI)
SWAG_VERSION := v1.16.6

# Phony targets
.PHONY: all build test clean deps tools swagger update build-all

# Targets
all: clean deps test build-all

build:
	CGO_ENABLED=0 go build $(BUILD_FLAGS) -o $(BINARY_PATH)/$(BINARY_NAME) -v

test:
	go test ./... --race
	mkdir -p tmp/test-coverage
	go test -coverprofile=tmp/test-coverage/coverage.out
	go tool cover -html=tmp/test-coverage/coverage.out -o ./tmp/test-coverage/coverage.html

clean:
	go clean
	find $(BINARY_PATH) -type f ! -name 'keep' -delete 2>/dev/null || true

# Reproducible: download locked modules only (used by Docker)
deps:
	go mod download

# Local tooling — pinned, not @latest
tools:
	go install github.com/swaggo/swag/cmd/swag@$(SWAG_VERSION)

# Regen OpenAPI + gin docs from annotations
swagger: tools
	swag init -g main.go -o docs

# Deliberate upgrades of *direct* deps only (never run naked go get -u ./... in Docker/CI)
update:
	go get github.com/gin-gonic/gin@latest
	go get github.com/prometheus/client_golang@latest
	go get github.com/stretchr/testify@latest
	go get github.com/swaggo/files@latest
	go get github.com/swaggo/gin-swagger@latest
	go get github.com/swaggo/swag@latest
	go get go.uber.org/zap@latest
	go get go.opentelemetry.io/otel@latest
	go get go.opentelemetry.io/otel/sdk@latest
	go get go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp@latest
	go get go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc@latest
	go get go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin@latest
	go get go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp@latest
	go get golang.org/x/net@latest
	go get golang.org/x/crypto@latest
	go get golang.org/x/sys@latest
	go get golang.org/x/text@latest
	go mod tidy

build-all:
	@echo "version is $(VERSION)"
	@echo "build_date is $(BUILD_DATE)"
	@echo "ld-flags is $(BUILD_FLAGS)"

	CGO_ENABLED=0 GOARCH=amd64 GOOS=windows  go build $(BUILD_FLAGS) -o $(BINARY_PATH)/$(BINARY_NAME).windows.amd64 -v
	CGO_ENABLED=0 GOARCH=amd64 GOOS=linux    go build $(BUILD_FLAGS) -o $(BINARY_PATH)/$(BINARY_NAME).linux.amd64 -v
	CGO_ENABLED=0 GOARCH=amd64 GOOS=darwin   go build $(BUILD_FLAGS) -o $(BINARY_PATH)/$(BINARY_NAME).darwin.amd64 -v
	CGO_ENABLED=0 GOARCH=arm64 GOOS=darwin   go build $(BUILD_FLAGS) -o $(BINARY_PATH)/$(BINARY_NAME).darwin.arm64 -v
	CGO_ENABLED=0 GOARCH=arm64 GOOS=linux    go build $(BUILD_FLAGS) -o $(BINARY_PATH)/$(BINARY_NAME).linux.arm64 -v
	CGO_ENABLED=0 GOARCH=arm64 GOOS=android  go build $(BUILD_FLAGS) -o $(BINARY_PATH)/$(BINARY_NAME).arm64 -v
