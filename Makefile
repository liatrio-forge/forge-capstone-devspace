SHELL := /usr/bin/env bash

BINARY_NAME ?= devspace
CMD_PATH ?= ./cmd/devspace
BIN_DIR ?= bin
DIST_DIR ?= dist
GOLANGCI_LINT ?= go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.12.2
GOVULNCHECK ?= go run golang.org/x/vuln/cmd/govulncheck@v1.1.4

.PHONY: all fmt format fmt-check test vet lint vulncheck build verify precommit install-hooks clean tui-install tui-verify tui-build tui-build-all

all: verify

fmt format:
	gofmt -w cmd internal

fmt-check:
	test -z "$$(gofmt -l cmd internal)" || (gofmt -l cmd internal && exit 1)

test:
	go test ./...

vet:
	go vet ./...

lint: fmt-check
	$(GOLANGCI_LINT) run ./...

vulncheck:
	$(GOVULNCHECK) ./...

build:
	mkdir -p $(BIN_DIR)
	go build -trimpath -o $(BIN_DIR)/$(BINARY_NAME) $(CMD_PATH)

verify: test vet lint vulncheck build

# devspace-tui (Bun) — not part of `verify` so Go-only work never needs Bun.
tui-install:
	cd tui && bun install --frozen-lockfile

tui-verify: tui-install
	cd tui && bun run typecheck && bun test

tui-build: tui-install
	cd tui && bun run build

tui-build-all: tui-install
	cd tui && ./build-all.sh

precommit: fmt lint test build

install-hooks:
	git config core.hooksPath .githooks
	chmod +x .githooks/pre-commit

clean:
	rm -rf $(BIN_DIR) $(DIST_DIR)

help:
	@echo "Usage: make <target>"
	@echo "Targets:"
	@echo "  all - Run all checks and build the binary"
	@echo "  fmt - Format the code"
	@echo "  fmt-check - Check if the code is formatted"
	@echo "  test - Run the tests"
	@echo "  vet - Run the vet checks"
	@echo "  lint - Run the lint checks"
	@echo "  vulncheck - Run the vulnerability checks"
	@echo "  build - Build the binary"
	@echo "  verify - Run all checks and build the binary"
