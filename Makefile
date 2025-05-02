# Makefile for chainlist-parser

# Variables
BINARY_NAME=chainlist-parser
BUILD_DIR=build
CMD_PATH=./cmd/api/main.go
PKG_LIST=$(shell go list ./... | grep -v /vendor/)

# Default target
.PHONY: help
help:
	@echo "Usage: make <target>"
	@echo ""
	@echo "Targets:"
	@echo "  help          Show this help message"
	@echo "  lint          Run golangci-lint"
	@echo "  generate      Generate mocks (go generate)"
	@echo "  test          Run tests"
	@echo "  build         Build the binary into $(BUILD_DIR)/"
	@echo "  run           Run the service locally using go run"
	@echo "  clean         Clean the build directory"
	@echo "  docker-build  Build the docker image"
	@echo "  docker-up     Start the service using docker-compose"
	@echo "  docker-down   Stop the service using docker-compose"

# Linting
.PHONY: lint
lint:
	@echo "Running linter..."
	@golangci-lint run ./...

# Generate mocks
.PHONY: generate
generate:
	@echo "Generating mocks..."
	@go generate ./...

# Testing
.PHONY: test
test: generate ## Ensure mocks are generated before testing
	@echo "Running tests..."
	@go test -v -race -coverprofile=coverage.out $(PKG_LIST)
	@echo "Test coverage report generated: coverage.out"

# Build
.PHONY: build
build: clean
	@echo "Building binary..."
	@mkdir -p $(BUILD_DIR)
	@go build -ldflags="-s -w" -o $(BUILD_DIR)/$(BINARY_NAME) $(CMD_PATH)
	@echo "Binary built at $(BUILD_DIR)/$(BINARY_NAME)"

# Run locally
.PHONY: run
run: generate
	@echo "Running service locally (go run)..."
	@go run $(CMD_PATH)

# Clean build directory
.PHONY: clean
clean:
	@echo "Cleaning build directory..."
	@rm -rf $(BUILD_DIR)

# Docker operations
.PHONY: docker-build
docker-build:
	@echo "Building docker image..."
	@docker-compose build

.PHONY: docker-up
docker-up: docker-build
	@echo "Starting service with docker-compose..."
	@docker-compose up -d # Run in detached mode

.PHONY: docker-down
docker-down:
	@echo "Stopping service with docker-compose..."
	@docker-compose down 