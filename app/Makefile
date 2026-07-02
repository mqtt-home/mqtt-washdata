# Makefile for the mqtt-washdata dryer power-profiling microservice

GO = go
GOFLAGS = -v

# Version information (injected into the version package at build time)
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
GIT_COMMIT ?= $(shell git rev-parse HEAD 2>/dev/null || echo "unknown")
BUILD_TIME ?= $(shell date -u '+%Y-%m-%d %H:%M:%S UTC')
LDFLAGS = -ldflags "-s -w -X 'github.com/mqtt-home/mqtt-washdata/version.Version=$(VERSION)' -X 'github.com/mqtt-home/mqtt-washdata/version.GitCommit=$(GIT_COMMIT)' -X 'github.com/mqtt-home/mqtt-washdata/version.BuildTime=$(BUILD_TIME)'"

# Paths
BUILD_DIR = build
CONFIG_DIR = $(PWD)/../production/config
WEB_DIR = web

# Names
BINARY_NAME = mqtt-washdata
DOCKER_IMAGE_NAME = pharndt/mqtt-washdata:latest

.DEFAULT_GOAL := help

.PHONY: help
help: ## Show this help
	@echo "mqtt-washdata - dryer power-profiling microservice"
	@echo ""
	@echo "Usage: make <target>"
	@echo ""
	@grep -E '^[a-zA-Z0-9_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-16s\033[0m %s\n", $$1, $$2}'

.PHONY: build
build: build-frontend build-backend ## Build frontend and backend

.PHONY: build-frontend
build-frontend: ## Build the React frontend (pnpm)
	@echo "Building the frontend..."
	@cd $(WEB_DIR) && pnpm install && pnpm run build

.PHONY: build-backend
build-backend: ## Build the Go backend binary
	@echo "Building the backend..."
	@$(GO) build $(GOFLAGS) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) .

.PHONY: dev
dev: build ## Build everything and run against the local config
	@$(BUILD_DIR)/$(BINARY_NAME) $(CONFIG_DIR)/config.json

.PHONY: dev-frontend
dev-frontend: ## Start the Vite frontend dev server
	@echo "Starting frontend dev server..."
	@cd $(WEB_DIR) && pnpm run dev

.PHONY: dev-backend
dev-backend: build-backend ## Build and run only the backend against the local config
	@echo "Starting backend..."
	@$(BUILD_DIR)/$(BINARY_NAME) $(CONFIG_DIR)/config.json

.PHONY: test
test: ## Run the Go tests
	@$(GO) test ./...

.PHONY: simulate
simulate: ## Publish a simulated dryer run over MQTT (PROGRAM=cottons|synthetics|eco)
	@./tools/simulate.sh $(PROGRAM)

.PHONY: version
version: ## Print the version information that would be embedded
	@echo "Version:    $(VERSION)"
	@echo "Git Commit: $(GIT_COMMIT)"
	@echo "Build Time: $(BUILD_TIME)"

.PHONY: docker
docker: build ## Build the Docker image
	@echo "Building Docker image..."
	@docker build -t $(DOCKER_IMAGE_NAME) .

.PHONY: deps
deps: ## Download and tidy Go dependencies
	@$(GO) mod download
	@$(GO) mod tidy

.PHONY: clean
clean: ## Remove build artifacts and node_modules
	@echo "Cleaning up..."
	@rm -rf $(BUILD_DIR)
	@cd $(WEB_DIR) && rm -rf dist node_modules
