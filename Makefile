# Variables
BINARY_PATH := ./bin
BINARY_NAME := cu-api
VERSION := $(shell git describe --tags 2>/dev/null || echo dev)
GIT_HASH := $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
BUILD_DATE := $(shell date)
BUILD_FLAGS := -ldflags="-w -s -X main.Version=$(VERSION) -X main.GitHash=$(GIT_HASH)"

# Phony targets
.PHONY: all build test clean deps tools update build-all

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
	find $(BINARY_PATH) -type f ! -name 'keep' -delete

# Reproducible: download locked modules only (used by Docker)
deps:
	go mod download

# Local tooling (swagger regen, etc.)
tools:
	go install github.com/swaggo/swag/cmd/swag@latest

# Explicit dependency upgrades (local / deliberate only)
update:
	go get -u ./...
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
