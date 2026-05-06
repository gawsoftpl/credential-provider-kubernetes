# =============================================================================
# VARIABLES
# =============================================================================

# Binary and build info
BINARY_NAME ?= credential-provider-kubernetes
BUILD_DIR ?= ./build
MODULE_NAME ?= credential-provider-kubernetes

# Build info - with fallbacks for local development
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
GIT_COMMIT ?= $(shell git rev-parse HEAD 2>/dev/null || echo "unknown")
BUILD_TIME ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
BUILD_BY ?= $(shell whoami 2>/dev/null || echo "unknown")

# Use CI variables if available, otherwise use git/local values
ifeq ($(CI_COMMIT_TAG),)
    CI_VERSION = $(VERSION)
else
    CI_VERSION = $(CI_COMMIT_TAG)
endif

# Platform defaults
GOOS ?= linux
GOARCH ?= amd64

# Build flags
LDFLAGS = -X $(MODULE_NAME)/pkg/version.Version=$(CI_VERSION) \
          -X $(MODULE_NAME)/pkg/version.GitCommit=$(GIT_COMMIT) \
          -X $(MODULE_NAME)/pkg/version.BuildTime=$(BUILD_TIME) \
          -X $(MODULE_NAME)/pkg/version.BuildBy=$(BUILD_BY)



# =============================================================================
# PHONY TARGETS
# =============================================================================

.PHONY: build docker dev test


build: ## Build for specified platform (default: linux/arm64)

	@echo "🔨 Building credential-provider-k8s for $(GOOS)/$(GOARCH)..."
	@mkdir -p $(BUILD_DIR)
	@GOOS=$(GOOS) GOARCH=$(GOARCH) go build \
		-ldflags "$(LDFLAGS)" \
		-o $(BUILD_DIR)/credential-provider-k8s-$(GOOS)-$(GOARCH) \
		./main.go
	@echo "✅ Built: $(BUILD_DIR)/credential-provider-k8s-$(GOOS)-$(GOARCH)"


build-local: ## Build for current platform
	@echo "🔨 Building $(BINARY_NAME) for local platform..."
	@mkdir -p $(BUILD_DIR)
	@go build \
		-ldflags "$(LDFLAGS)" \
		-o $(BUILD_DIR)/$(BINARY_NAME) \
		./main.go
	@echo "✅ Built: $(BUILD_DIR)/$(BINARY_NAME)"

build-all: ## Build for multiple platforms
	@echo "🔨 Building for multiple platforms..."
	@$(MAKE) build GOOS=linux GOARCH=amd64
	@$(MAKE) build GOOS=linux GOARCH=arm64
	#@$(MAKE) build GOOS=darwin GOARCH=amd64
	#@$(MAKE) build GOOS=darwin GOARCH=arm64
	#@$(MAKE) build GOOS=windows GOARCH=amd64
	@echo "✅ Built all platforms"

# =============================================================================
# DEPENDENCIES
# =============================================================================

deps: ## Download dependencies
	@echo "📦 Downloading dependencies..."
	@go mod download
	@go mod tidy

deps-update: ## Update dependencies
	@echo "📦 Updating dependencies..."
	@go get -u ./...
	@go mod tidy

dev:
	air

# =============================================================================
# TESTING AND LINTING
# =============================================================================

test: ## Run tests
	@echo "🧪 Running tests..."
	@go test -v ./...

test-coverage: ## Run tests with coverage
	@echo "🧪 Running tests with coverage..."
	@go test -v -coverprofile=coverage.out ./...
	@go tool cover -html=coverage.out -o coverage.html
	@echo "✅ Coverage report: coverage.html"

lint: ## Run linter
	@echo "🔍 Running linter..."
	@golangci-lint run ./...

# =============================================================================
# UTILITY TARGETS
# =============================================================================

clean: ## Clean build artifacts
	@echo "🧹 Cleaning build artifacts..."
	@rm -rf $(BUILD_DIR)
	@rm -f coverage.out coverage.html
	@echo "✅ Cleaned"

version: ## Show version information
	@echo "📋 Build Information:"
	@echo "  Version:    $(CI_VERSION)"
	@echo "  Git Commit: $(GIT_COMMIT)"
	@echo "  Build Time: $(BUILD_TIME)"
	@echo "  Build By:   $(BUILD_BY)"
	@echo "  Platform:   $(GOOS)/$(GOARCH)"

# =============================================================================
# DEBUG TARGETS
# =============================================================================

debug: ## Show debug information
	@echo "🐛 Debug Information:"
	@echo "  BINARY_NAME: $(BINARY_NAME)"
	@echo "  BUILD_DIR:   $(BUILD_DIR)"
	@echo "  MODULE_NAME: $(MODULE_NAME)"
	@echo "  VERSION:     $(VERSION)"
	@echo "  CI_VERSION:  $(CI_VERSION)"
	@echo "  GIT_COMMIT:  $(GIT_COMMIT)"
	@echo "  BUILD_TIME:  $(BUILD_TIME)"
	@echo "  BUILD_BY:    $(BUILD_BY)"
	@echo "  GOOS:        $(GOOS)"
	@echo "  GOARCH:      $(GOARCH)"
	@echo "  LDFLAGS:     $(LDFLAGS)"