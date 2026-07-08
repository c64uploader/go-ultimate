.DEFAULT_GOAL := help

GO ?= go

.PHONY: help test e2e lint

help: ## Show available targets
	@awk 'BEGIN {FS = ":.*## "; printf "Usage: make <target>\n\nTargets:\n"} /^[a-zA-Z0-9_-]+:.*## / {printf "  %-8s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

test: ## Run unit tests (platform-neutral, suitable for CI)
	$(GO) test ./...

e2e: ## Run end-to-end tests (requires C64 Ultimate; C64U_ADDRESS, C64U_PASSWORD)
	$(GO) test -C tests/e2e ./... -count 1

lint: ## Run golangci-lint
	$(GO) tool -modfile=tools/go.mod golangci-lint run ./...
	cd tests/e2e && $(GO) tool -modfile=../../tools/go.mod golangci-lint run ./...
