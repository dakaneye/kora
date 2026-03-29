BINARY_NAME := kora
BUILD_DIR := bin
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
DATE := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS := -ldflags "-X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)"

.PHONY: all build test test-integration test-e2e lint clean help

all: lint test build

build:
	@mkdir -p $(BUILD_DIR)
	go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/kora

test:
	go test -race -v ./internal/...

test-integration:
	go test -race -v -tags=integration ./tests/integration/...

test-e2e: build
	go test -race -v -tags=e2e ./tests/e2e/...

lint:
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run ./...; \
	else \
		echo "golangci-lint not installed"; \
		exit 1; \
	fi

install: build
	@mkdir -p ~/.local/bin
	@cp $(BUILD_DIR)/$(BINARY_NAME) ~/.local/bin/
	@echo "Installed to ~/.local/bin/$(BINARY_NAME)"

clean:
	@rm -rf $(BUILD_DIR)

help:
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@echo "  build            Build the kora binary"
	@echo "  test             Run unit tests"
	@echo "  test-integration Run integration tests (requires CLI tools + auth)"
	@echo "  test-e2e         Run end-to-end tests (builds binary first)"
	@echo "  lint             Run golangci-lint"
	@echo "  install          Install to ~/.local/bin"
	@echo "  clean            Remove build artifacts"
