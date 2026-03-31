.PHONY: help build test test-unit test-e2e test-integration test-manual clean fmt lint vet install-tools k8s-create k8s-delete k8s-status k8s-kubeconfig

# Variables
BINARY_NAME := blcli
BINARY_PATH := ./cmd/blcli
OUTPUT_DIR := ../workspace/bin
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME := $(shell date +%Y%m%d%H%M%S)
LDFLAGS := -X 'blcli/pkg/cli.buildTime=$(BUILD_TIME)'

# Test variables
TEST_TIMEOUT := 30m
COVERAGE_OUT := coverage.out
COVERAGE_HTML := coverage.html

# Kubernetes cluster variables
KIND_CLUSTER_NAME := blcli-test
KIND_KUBECONFIG := $(HOME)/.kube/blcli-test-config

# Colors for output
GREEN := \033[0;32m
YELLOW := \033[0;33m
RED := \033[0;31m
NC := \033[0m # No Color

help: ## Show this help message
	@echo "$(GREEN)Available targets:$(NC)"
	@echo ""
	@echo "$(YELLOW)Build:$(NC)"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | grep -E '(build|install)' | awk 'BEGIN {FS = ":.*?## "}; {printf "  $(GREEN)%-20s$(NC) %s\n", $$1, $$2}'
	@echo ""
	@echo "$(YELLOW)Testing:$(NC)"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | grep -E 'test' | awk 'BEGIN {FS = ":.*?## "}; {printf "  $(GREEN)%-20s$(NC) %s\n", $$1, $$2}'
	@echo ""
	@echo "$(YELLOW)Kubernetes:$(NC)"
	@grep -E '^k8s[^:]*:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  $(GREEN)%-20s$(NC) %s\n", $$1, $$2}'
	@echo ""
	@echo "$(YELLOW)Utilities:$(NC)"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | grep -E '(clean|fmt|lint|vet|install-tools)' | awk 'BEGIN {FS = ":.*?## "}; {printf "  $(GREEN)%-20s$(NC) %s\n", $$1, $$2}'

# Build targets
build: ## Build the blcli binary
	@echo "$(GREEN)Building $(BINARY_NAME)...$(NC)"
	@mkdir -p $(OUTPUT_DIR)
	@go build -ldflags "$(LDFLAGS)" -o $(OUTPUT_DIR)/$(BINARY_NAME) $(BINARY_PATH)
	@echo "$(GREEN)✓ Binary built at $(OUTPUT_DIR)/$(BINARY_NAME)$(NC)"

build-dev: ## Build with race detector and debug symbols
	@echo "$(GREEN)Building $(BINARY_NAME) (dev mode)...$(NC)"
	@mkdir -p $(OUTPUT_DIR)
	@go build -race -ldflags "$(LDFLAGS)" -o $(OUTPUT_DIR)/$(BINARY_NAME) $(BINARY_PATH)
	@echo "$(GREEN)✓ Binary built at $(OUTPUT_DIR)/$(BINARY_NAME)$(NC)"

install: build ## Build and install to $GOPATH/bin
	@echo "$(GREEN)Installing $(BINARY_NAME)...$(NC)"
	@go install -ldflags "$(LDFLAGS)" $(BINARY_PATH)
	@echo "$(GREEN)✓ Installed to $$(go env GOPATH)/bin/$(BINARY_NAME)$(NC)"

# Test targets
test: test-unit ## Run all tests (unit tests only by default)
	@echo "$(GREEN)✓ All tests completed$(NC)"

test-unit: ## Run unit tests
	@echo "$(GREEN)Running unit tests...$(NC)"
	@go test -v -timeout $(TEST_TIMEOUT) -race -coverprofile=$(COVERAGE_OUT) ./pkg/...
	@echo "$(GREEN)✓ Unit tests completed$(NC)"

test-unit-verbose: ## Run unit tests with verbose output
	@echo "$(GREEN)Running unit tests (verbose)...$(NC)"
	@go test -v -timeout $(TEST_TIMEOUT) -race ./pkg/...

test-unit-coverage: ## Run unit tests with coverage report
	@echo "$(GREEN)Running unit tests with coverage...$(NC)"
	@go test -v -timeout $(TEST_TIMEOUT) -race -coverprofile=$(COVERAGE_OUT) -covermode=atomic ./pkg/...
	@go tool cover -html=$(COVERAGE_OUT) -o $(COVERAGE_HTML)
	@echo "$(GREEN)✓ Coverage report generated: $(COVERAGE_HTML)$(NC)"

test-e2e: ## Run e2e tests (requires k8s cluster)
	@echo "$(YELLOW)Running e2e tests...$(NC)"
	@echo "$(YELLOW)Note: This requires a Kubernetes cluster. Use 'make k8s-create' to create one.$(NC)"
	@go test -v -timeout $(TEST_TIMEOUT) ./integration/e2e/...
	@echo "$(GREEN)✓ E2E tests completed$(NC)"

test-e2e-verbose: ## Run e2e tests with verbose output
	@echo "$(YELLOW)Running e2e tests (verbose)...$(NC)"
	@go test -v -timeout $(TEST_TIMEOUT) -ginkgo.v ./integration/e2e/...

test-integration: ## Run integration tests
	@echo "$(GREEN)Running integration tests...$(NC)"
	@go test -v -timeout $(TEST_TIMEOUT) ./integration/...
	@echo "$(GREEN)✓ Integration tests completed$(NC)"

test-all: test-unit test-integration ## Run all tests (unit + integration, excluding e2e)
	@echo "$(GREEN)✓ All tests (excluding e2e) completed$(NC)"

test-manual: ## Run manual test scenarios (interactive)
	@echo "$(YELLOW)Manual testing mode$(NC)"
	@echo "$(YELLOW)Available manual test scenarios:$(NC)"
	@echo "  1. Test init kubernetes: make test-manual-init-k8s"
	@echo "  2. Test apply kubernetes: make test-manual-apply-k8s"
	@echo "  3. Test init terraform: make test-manual-init-tf"
	@echo "  4. Test init-args: make test-manual-init-args"

test-manual-init-k8s: build ## Manual test: init kubernetes
	@echo "$(GREEN)Manual test: init kubernetes$(NC)"
	@echo "$(YELLOW)Running: $(OUTPUT_DIR)/$(BINARY_NAME) init kubernetes --args ../workspace/config/args.yaml --template-repo ../bl-template --output ../workspace/output --overwrite$(NC)"
	@$(OUTPUT_DIR)/$(BINARY_NAME) init kubernetes --args ../workspace/config/args.yaml --template-repo ../bl-template --output ../workspace/output --overwrite

test-manual-apply-k8s: build ## Manual test: apply kubernetes (requires k8s cluster)
	@echo "$(GREEN)Manual test: apply kubernetes$(NC)"
	@echo "$(YELLOW)Running: $(OUTPUT_DIR)/$(BINARY_NAME) apply kubernetes -d ../workspace/output/kubernetes --template-repo ../bl-template --dry-run$(NC)"
	@$(OUTPUT_DIR)/$(BINARY_NAME) apply kubernetes -d ../workspace/output/kubernetes --template-repo ../bl-template --dry-run

test-manual-init-tf: build ## Manual test: init terraform
	@echo "$(GREEN)Manual test: init terraform$(NC)"
	@echo "$(YELLOW)Running: $(OUTPUT_DIR)/$(BINARY_NAME) init terraform --args ../workspace/config/args.yaml --template-repo ../bl-template --output ../workspace/output --overwrite$(NC)"
	@$(OUTPUT_DIR)/$(BINARY_NAME) init terraform --args ../workspace/config/args.yaml --template-repo ../bl-template --output ../workspace/output --overwrite

test-manual-init-args: build ## Manual test: init-args
	@echo "$(GREEN)Manual test: init-args$(NC)"
	@echo "$(YELLOW)Running: $(OUTPUT_DIR)/$(BINARY_NAME) init-args --template-repo ../bl-template -o ../workspace/config/test-args.yaml$(NC)"
	@$(OUTPUT_DIR)/$(BINARY_NAME) init-args --template-repo ../bl-template -o ../workspace/config/test-args.yaml

# Kubernetes cluster management (using kind)
k8s-create: ## Create a local Kubernetes cluster using kind (for Mac with Docker)
	@echo "$(GREEN)Creating local Kubernetes cluster '$(KIND_CLUSTER_NAME)'...$(NC)"
	@if ! command -v kind >/dev/null 2>&1; then \
		echo "$(RED)Error: kind is not installed. Install it with: brew install kind$(NC)"; \
		exit 1; \
	fi
	@if kind get clusters | grep -q "^$(KIND_CLUSTER_NAME)$$"; then \
		echo "$(YELLOW)Cluster '$(KIND_CLUSTER_NAME)' already exists. Use 'make k8s-delete' to remove it first.$(NC)"; \
	else \
		echo "apiVersion: kind.x-k8s.io/v1alpha4" > /tmp/kind-config.yaml; \
		echo "kind: Cluster" >> /tmp/kind-config.yaml; \
		echo "nodes:" >> /tmp/kind-config.yaml; \
		echo "- role: control-plane" >> /tmp/kind-config.yaml; \
		echo "  kubeadmConfigPatches:" >> /tmp/kind-config.yaml; \
		echo "  - |" >> /tmp/kind-config.yaml; \
		echo "    kind: InitConfiguration" >> /tmp/kind-config.yaml; \
		echo "    nodeRegistration:" >> /tmp/kind-config.yaml; \
		echo "      kubeletExtraArgs:" >> /tmp/kind-config.yaml; \
		echo "        node-labels: \"ingress-ready=true\"" >> /tmp/kind-config.yaml; \
		echo "  extraPortMappings:" >> /tmp/kind-config.yaml; \
		echo "  - containerPort: 80" >> /tmp/kind-config.yaml; \
		echo "    hostPort: 8080" >> /tmp/kind-config.yaml; \
		echo "    protocol: TCP" >> /tmp/kind-config.yaml; \
		echo "  - containerPort: 443" >> /tmp/kind-config.yaml; \
		echo "    hostPort: 8443" >> /tmp/kind-config.yaml; \
		echo "    protocol: TCP" >> /tmp/kind-config.yaml; \
		kind create cluster --name $(KIND_CLUSTER_NAME) --config /tmp/kind-config.yaml; \
		rm -f /tmp/kind-config.yaml; \
		mkdir -p $$(dirname $(KIND_KUBECONFIG)); \
		kind get kubeconfig --name $(KIND_CLUSTER_NAME) > $(KIND_KUBECONFIG); \
		echo "$(GREEN)✓ Cluster '$(KIND_CLUSTER_NAME)' created$(NC)"; \
		echo "$(YELLOW)To use this cluster, set KUBECONFIG:$(NC)"; \
		echo "  export KUBECONFIG=$(KIND_KUBECONFIG)"; \
		echo "$(YELLOW)Or use: make k8s-kubeconfig$(NC)"; \
	fi

k8s-delete: ## Delete the local Kubernetes cluster
	@echo "$(YELLOW)Deleting local Kubernetes cluster '$(KIND_CLUSTER_NAME)'...$(NC)"
	@if kind get clusters | grep -q "^$(KIND_CLUSTER_NAME)$$"; then \
		kind delete cluster --name $(KIND_CLUSTER_NAME); \
		echo "$(GREEN)✓ Cluster '$(KIND_CLUSTER_NAME)' deleted$(NC)"; \
	else \
		echo "$(YELLOW)Cluster '$(KIND_CLUSTER_NAME)' does not exist$(NC)"; \
	fi

k8s-status: ## Show status of the local Kubernetes cluster
	@echo "$(GREEN)Kubernetes cluster status:$(NC)"
	@if command -v kind >/dev/null 2>&1; then \
		if kind get clusters | grep -q "^$(KIND_CLUSTER_NAME)$$"; then \
			echo "$(GREEN)Cluster '$(KIND_CLUSTER_NAME)' exists$(NC)"; \
			echo ""; \
			echo "$(YELLOW)Cluster nodes:$(NC)"; \
			kubectl --context kind-$(KIND_CLUSTER_NAME) get nodes; \
			echo ""; \
			mkdir -p $$(dirname $(KIND_KUBECONFIG)); \
			kind get kubeconfig --name $(KIND_CLUSTER_NAME) > $(KIND_KUBECONFIG) 2>/dev/null || true; \
			echo "$(YELLOW)To use this cluster:$(NC)"; \
			echo "  export KUBECONFIG=$(KIND_KUBECONFIG)"; \
		else \
			echo "$(YELLOW)Cluster '$(KIND_CLUSTER_NAME)' does not exist$(NC)"; \
			echo "$(YELLOW)Create it with: make k8s-create$(NC)"; \
		fi; \
	else \
		echo "$(RED)Error: kind is not installed$(NC)"; \
	fi

k8s-kubeconfig: ## Print the kubeconfig export command for the local cluster
	@if kind get clusters | grep -q "^$(KIND_CLUSTER_NAME)$$"; then \
		mkdir -p $$(dirname $(KIND_KUBECONFIG)); \
		kind get kubeconfig --name $(KIND_CLUSTER_NAME) > $(KIND_KUBECONFIG); \
		echo "$(GREEN)Kubeconfig saved to $(KIND_KUBECONFIG)$(NC)"; \
		echo "$(YELLOW)Export KUBECONFIG to use cluster '$(KIND_CLUSTER_NAME)':$(NC)"; \
		echo "  export KUBECONFIG=$(KIND_KUBECONFIG)"; \
	else \
		echo "$(RED)Error: Cluster '$(KIND_CLUSTER_NAME)' does not exist$(NC)"; \
		echo "$(YELLOW)Create it with: make k8s-create$(NC)"; \
	fi

k8s-reset: k8s-delete k8s-create ## Delete and recreate the local Kubernetes cluster
	@echo "$(GREEN)✓ Cluster reset complete$(NC)"

# Utility targets
clean: ## Clean build artifacts and test files
	@echo "$(YELLOW)Cleaning...$(NC)"
	@rm -rf $(OUTPUT_DIR)/$(BINARY_NAME)
	@rm -f $(COVERAGE_OUT) $(COVERAGE_HTML)
	@go clean -testcache
	@echo "$(GREEN)✓ Clean complete$(NC)"

fmt: ## Format Go code
	@echo "$(GREEN)Formatting Go code...$(NC)"
	@go fmt ./...
	@echo "$(GREEN)✓ Formatting complete$(NC)"

lint: ## Run golangci-lint (if installed)
	@if command -v golangci-lint >/dev/null 2>&1; then \
		echo "$(GREEN)Running golangci-lint...$(NC)"; \
		golangci-lint run ./...; \
		echo "$(GREEN)✓ Linting complete$(NC)"; \
	else \
		echo "$(YELLOW)golangci-lint is not installed. Install it with:$(NC)"; \
		echo "  brew install golangci-lint"; \
		echo "  or: curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b \$$(go env GOPATH)/bin v1.54.2"; \
	fi

vet: ## Run go vet
	@echo "$(GREEN)Running go vet...$(NC)"
	@go vet ./...
	@echo "$(GREEN)✓ Vet complete$(NC)"

install-tools: ## Install development tools (kind, golangci-lint, etc.)
	@echo "$(GREEN)Installing development tools...$(NC)"
	@if command -v brew >/dev/null 2>&1; then \
		echo "$(YELLOW)Installing kind...$(NC)"; \
		brew install kind || echo "$(YELLOW)kind already installed or brew not available$(NC)"; \
		echo "$(YELLOW)Installing golangci-lint...$(NC)"; \
		brew install golangci-lint || echo "$(YELLOW)golangci-lint already installed or brew not available$(NC)"; \
	else \
		echo "$(YELLOW)Homebrew not found. Please install tools manually:$(NC)"; \
		echo "  - kind: https://kind.sigs.k8s.io/docs/user/quick-start/#installation"; \
		echo "  - golangci-lint: https://golangci-lint.run/usage/install/"; \
	fi
	@echo "$(GREEN)✓ Tools installation complete$(NC)"

# Development workflow
dev-setup: install-tools ## Set up development environment
	@echo "$(GREEN)Development environment setup complete$(NC)"
	@echo "$(YELLOW)Next steps:$(NC)"
	@echo "  1. Create k8s cluster: make k8s-create"
	@echo "  2. Run tests: make test"
	@echo "  3. Build: make build"

# CI/CD helpers
ci-test: fmt vet test-unit ## Run CI test suite (format, vet, unit tests)
	@echo "$(GREEN)✓ CI tests passed$(NC)"

ci-build: build ## Build for CI
	@echo "$(GREEN)✓ CI build complete$(NC)"
