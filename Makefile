# Go parameters
GOBASE := $(shell pwd)
GOBUILD := go build
GOTEST := go test
GOCLEAN := go clean
GOMOD := go mod
LINT := golangci-lint run

# Binary name
BINARY_NAME := api
CMD_PATH := ./cmd/api

.PHONY: all build clean test lint run docker-build infra-run infra-down help

all: build

build: ## Build the application binary
	@echo "Building $(BINARY_NAME)..."
	$(GOBUILD) -o $(BINARY_NAME) $(CMD_PATH)/main.go

clean: ## Remove previous build
	@echo "Cleaning..."
	$(GOCLEAN)
	rm -f $(BINARY_NAME)

test: ## Run tests
	@echo "Running tests..."
	$(GOTEST) ./...

lint: ## Run linters
	@echo "Running linters..."
	$(LINT) ./...

run: build ## Build and run the application (foreground)
	@echo "Running $(BINARY_NAME)..."
	./$(BINARY_NAME)

# Docker commands
docker-build: ## Build the Docker image
	@echo "Building Docker image..."
	docker build -t $(BINARY_NAME):latest .

docker-run: ## Run the application in a Docker container
	@echo "Running Docker container..."
	docker run --rm -p 8080:8080 --name $(BINARY_NAME)-instance $(BINARY_NAME):latest

infra-up: ## Start the application using docker-compose
	@echo "Starting docker-compose service..."
	docker-compose up -d --build

infra-down: ## Stop the application using docker-compose
	@echo "Stopping docker-compose service..."
	docker-compose down

help: ## Display this help screen
	@echo 'Usage: make <TARGETS>'
	@echo '\nAvailable targets:'
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)

# Default target
.DEFAULT_GOAL := help 