# ─── kboot Makefile ────────────────────────────────────────────────

BINARY_NAME   := kboot
MODULE        := kboot
GO            := go
GOFMT         := gofmt
INFRA_DIR     := ./infra

# Build flags
VERSION       := $(shell cat version.txt 2>/dev/null || echo "dev")
LDFLAGS       := -s -w
BUILD_DIR     := ./bin

# Go tools
GOLANGCI_LINT := $(shell command -v golangci-lint 2>/dev/null || echo "$(GO) run github.com/golangci/golangci-lint/cmd/golangci-lint@latest")

# Infra
KIND          := $(shell command -v kind 2>/dev/null)
LOCALSTACK    := $(shell command -v localstack 2>/dev/null)
TERRAFORM     := $(shell command -v terraform 2>/dev/null)

# Colors
GREEN  := \033[0;32m
YELLOW := \033[1;33m
BLUE   := \033[0;34m
NC     := \033[0m

# ─── Build ─────────────────────────────────────────────────────────

.PHONY: all build run install clean

all: fmt vet lint build

build:
	@mkdir -p $(BUILD_DIR)
	@echo "$(BLUE)[build]$(NC) Compiling $(BINARY_NAME) v$(VERSION)..."
	$(GO) build -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/kboot/

run: build
	@echo "$(BLUE)[run]$(NC) Launching $(BINARY_NAME)..."
	$(BUILD_DIR)/$(BINARY_NAME) $(filter-out $@,$(MAKECMDGOALS))

install: build
	@echo "$(BLUE)[install]$(NC) Installing $(BINARY_NAME) to $$GOPATH/bin..."
	cp $(BUILD_DIR)/$(BINARY_NAME) $$($(GO) env GOPATH)/bin/$(BINARY_NAME)

clean:
	@echo "$(BLUE)[clean]$(NC) Removing build artifacts..."
	rm -rf $(BUILD_DIR)
	rm -f main.exe

# ─── Code Quality ──────────────────────────────────────────────────

.PHONY: fmt lint vet check tidy

fmt:
	@echo "$(BLUE)[fmt]$(NC) Formatting source..."
	$(GOFMT) -s -w .

lint:
	@echo "$(BLUE)[lint]$(NC) Running golangci-lint..."
	$(GOLANGCI_LINT) run ./... --timeout=5m

vet:
	@echo "$(BLUE)[vet]$(NC) Running go vet..."
	$(GO) vet ./...

check: fmt vet lint
	@echo "$(GREEN)✓$(NC) All checks passed"

tidy:
	@echo "$(BLUE)[tidy]$(NC) Cleaning up dependencies..."
	$(GO) mod tidy
	$(GO) mod verify

# ─── Test ──────────────────────────────────────────────────────────

.PHONY: test test-verbose test-coverage test-e2e test-integration

test:
	@echo "$(BLUE)[test]$(NC) Running unit tests..."
	$(GO) test -race -count=1 ./...

test-verbose:
	@echo "$(BLUE)[test]$(NC) Running unit tests (verbose)..."
	$(GO) test -race -v -count=1 ./...

test-coverage:
	@echo "$(BLUE)[test]$(NC) Running unit tests with coverage..."
	$(GO) test -race -coverprofile=coverage.out -covermode=atomic ./...
	$(GO) tool cover -func=coverage.out
	@rm -f coverage.out

test-e2e: build
	@echo "$(BLUE)[e2e]$(NC) Running CLI unit tests..."
	@bash ./scripts/test-e2e.sh

test-integration: build
	@echo "$(BLUE)[integration]$(NC) Running full integration test suite..."
	@bash ./scripts/test-integration.sh

# ─── YOLO Install ──────────────────────────────────────────────────

.PHONY: install-yolo
	@echo "$(YELLOW)[infra]$(NC) Tearing down test environment..."
	@bash $(INFRA_DIR)/bootstrap.sh cleanup

infra-status:
	@bash $(INFRA_DIR)/bootstrap.sh status

infra-destroy: infra-cleanup
	@echo "$(YELLOW)[infra]$(NC) Destroying Terraform resources..."
	@$(TERRAFORM) -chdir=$(INFRA_DIR) destroy -auto-approve -lock=false 2>/dev/null || true

# ─── Docker ────────────────────────────────────────────────────────

.PHONY: docker-up docker-down

docker-up:
	@echo "$(BLUE)[docker]$(NC) Starting LocalStack via docker-compose..."
	docker compose -f $(INFRA_DIR)/docker-compose.yml up -d

docker-down:
	@echo "$(YELLOW)[docker]$(NC) Stopping LocalStack..."
	docker compose -f $(INFRA_DIR)/docker-compose.yml down

# ─── Help ──────────────────────────────────────────────────────────

.PHONY: help

help:
	@echo "$(BLUE)╔══════════════════════════════════════════════╗$(NC)"
	@echo "$(BLUE)║              kboot Makefile                  ║$(NC)"
	@echo "$(BLUE)╚══════════════════════════════════════════════╝$(NC)"
	@echo ""
	@echo "$(GREEN)Build:$(NC)"
	@echo "  make build        — Compile binary to ./bin/$(BINARY_NAME)"
	@echo "  make run          — Build and run"
	@echo "  make install      — Install to $$GOPATH/bin"
	@echo "  make clean        — Remove build artifacts"
	@echo ""
	@echo "$(GREEN)Code Quality:$(NC)"
	@echo "  make fmt          — Format source with gofmt"
	@echo "  make vet          — Run go vet"
	@echo "  make lint         — Run golangci-lint"
	@echo "  make check        — fmt + vet + lint"
	@echo "  make tidy         — Clean up go.mod dependencies"
	@echo ""
	@echo "$(GREEN)Test:$(NC)"
	@echo "  make test         — Run tests with race detection"
	@echo "  make test-verbose — Run tests (verbose output)"
	@echo "  make test-coverage— Run tests with coverage report"
	@echo "  make test-e2e        — Run end-to-end tests (build + config + CLI)"
	@echo "  make test-integration — Run full integration suite (infra + kubectl)"
	@echo ""
	@echo "$(GREEN)Install:$(NC)"
	@echo "  make install-yolo    — Automated install (Go, Docker, kubectl, kind, TF, k9s + infra)"
	@echo ""
	@echo "$(GREEN)Infrastructure:$(NC)"
	@echo "  make infra        — Setup LocalStack + kind test env"
	@echo "  make infra-cleanup— Tear down test environment"
	@echo "  make infra-status — Show current infra status"
	@echo "  make infra-destroy— Full destroy (Terraform + kind)"
	@echo ""
	@echo "$(GREEN)Docker:$(NC)"
	@echo "  make docker-up    — Start LocalStack container"
	@echo "  make docker-down  — Stop LocalStack container"
