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

# ─── Default ───────────────────────────────────────────────────────

.DEFAULT_GOAL := help

# ─── Build ─────────────────────────────────────────────────────────

.PHONY: all build run install clean

all: fmt vet lint build

build:
	@mkdir -p $(BUILD_DIR)
	@printf '\033[0;34m[build]\033[0m Compiling $(BINARY_NAME) v$(VERSION)...\n'
	$(GO) build -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/kboot/

run: build
	@printf '\033[0;34m[run]\033[0m Launching $(BINARY_NAME)...\n'
	$(BUILD_DIR)/$(BINARY_NAME) $(filter-out $@,$(MAKECMDGOALS))

install: build
	@printf '\033[0;34m[install]\033[0m Installing $(BINARY_NAME) to ~/.local/bin...\n'
	@mkdir -p $$HOME/.local/bin
	cp $(BUILD_DIR)/$(BINARY_NAME) $$HOME/.local/bin/$(BINARY_NAME)

clean:
	@printf '\033[0;34m[clean]\033[0m Removing build artifacts...\n'
	rm -rf $(BUILD_DIR)
	rm -f main.exe

# ─── Code Quality ──────────────────────────────────────────────────

.PHONY: fmt lint vet check tidy

fmt:
	@printf '\033[0;34m[fmt]\033[0m Formatting source...\n'
	$(GOFMT) -s -w .

lint:
	@printf '\033[0;34m[lint]\033[0m Running golangci-lint...\n'
	$(GOLANGCI_LINT) run ./... --timeout=5m

vet:
	@printf '\033[0;34m[vet]\033[0m Running go vet...\n'
	$(GO) vet ./...

check: fmt vet lint
	@printf '\033[0;32m✓ All checks passed\033[0m\n'

tidy:
	@printf '\033[0;34m[tidy]\033[0m Cleaning up dependencies...\n'
	$(GO) mod tidy
	$(GO) mod verify

# ─── Test ──────────────────────────────────────────────────────────

.PHONY: test test-verbose test-coverage test-e2e test-integration

test:
	@printf '\033[0;34m[test]\033[0m Running unit tests...\n'
	$(GO) test -race -count=1 ./...

test-verbose:
	@printf '\033[0;34m[test]\033[0m Running unit tests (verbose)...\n'
	$(GO) test -race -v -count=1 ./...

test-coverage:
	@printf '\033[0;34m[test]\033[0m Running unit tests with coverage...\n'
	$(GO) test -race -coverprofile=coverage.out -covermode=atomic ./...
	$(GO) tool cover -func=coverage.out
	@rm -f coverage.out

test-e2e: build
	@printf '\033[0;34m[e2e]\033[0m Running CLI unit tests...\n'
	@bash ./scripts/test-e2e.sh

test-integration: build
	@printf '\033[0;34m[integration]\033[0m Running full integration test suite...\n'
	@bash ./scripts/test-integration.sh

# ─── YOLO Install ──────────────────────────────────────────────────

.PHONY: install-yolo

install-yolo:
	@printf '\033[0;34m[yolo]\033[0m Running automated installation...\n'
	@bash ./scripts/install.sh

# ─── Infrastructure (LocalStack + kind) ────────────────────────────

.PHONY: infra infra-setup infra-cleanup infra-status infra-destroy

infra: infra-setup

infra-setup:
	@printf '\033[0;34m[infra]\033[0m Setting up LocalStack + kind test environment...\n'
	@bash $(INFRA_DIR)/bootstrap.sh setup

infra-cleanup:
	@printf '\033[1;33m[infra]\033[0m Tearing down test environment...\n'
	@bash $(INFRA_DIR)/bootstrap.sh cleanup

infra-status:
	@bash $(INFRA_DIR)/bootstrap.sh status

infra-destroy: infra-cleanup
	@printf '\033[1;33m[infra]\033[0m Destroying Terraform resources...\n'
	@$(TERRAFORM) -chdir=$(INFRA_DIR) destroy -auto-approve -lock=false 2>/dev/null || true

# ─── Docker ────────────────────────────────────────────────────────

.PHONY: docker-up docker-down

docker-up:
	@printf '\033[0;34m[docker]\033[0m Starting LocalStack via docker-compose...\n'
	docker compose -f $(INFRA_DIR)/docker-compose.yml up -d

docker-down:
	@printf '\033[1;33m[docker]\033[0m Stopping LocalStack...\n'
	docker compose -f $(INFRA_DIR)/docker-compose.yml down

# ─── Help ──────────────────────────────────────────────────────────

.PHONY: help

help:
	@printf '\033[0;34m╔══════════════════════════════════════════════╗\033[0m\n'
	@printf '\033[0;34m║              kboot Makefile                  ║\033[0m\n'
	@printf '\033[0;34m╚══════════════════════════════════════════════╝\033[0m\n'
	@printf '\n'
	@printf '\033[0;32mBuild:\033[0m\n'
	@printf '  make build           — Compile binary to ./bin/$(BINARY_NAME)\n'
	@printf '  make run             — Build and run\n'
	@printf '  make install         — Install to $$GOPATH/bin\n'
	@printf '  make clean           — Remove build artifacts\n'
	@printf '\n'
	@printf '\033[0;32mCode Quality:\033[0m\n'
	@printf '  make fmt             — Format source with gofmt\n'
	@printf '  make vet             — Run go vet\n'
	@printf '  make lint            — Run golangci-lint\n'
	@printf '  make check           — fmt + vet + lint\n'
	@printf '  make tidy            — Clean up go.mod dependencies\n'
	@printf '\n'
	@printf '\033[0;32mTest:\033[0m\n'
	@printf '  make test            — Run unit tests with race detection\n'
	@printf '  make test-verbose    — Run unit tests (verbose output)\n'
	@printf '  make test-coverage   — Run unit tests with coverage report\n'
	@printf '  make test-e2e        — Run CLI unit tests (config CRUD, flags)\n'
	@printf '  make test-integration — Run full integration suite (11 phases)\n'
	@printf '\n'
	@printf '\033[0;32mInstall:\033[0m\n'
	@printf '  make install-yolo    — Automated install (deps + infra + config)\n'
	@printf '\n'
	@printf '\033[0;32mInfrastructure:\033[0m\n'
	@printf '  make infra           — Setup LocalStack + kind test env\n'
	@printf '  make infra-cleanup   — Tear down test environment\n'
	@printf '  make infra-status    — Show current infra status\n'
	@printf '  make infra-destroy   — Full destroy (Terraform + kind)\n'
	@printf '\n'
	@printf '\033[0;32mDocker:\033[0m\n'
	@printf '  make docker-up       — Start LocalStack container\n'
	@printf '  make docker-down     — Stop LocalStack container\n'
