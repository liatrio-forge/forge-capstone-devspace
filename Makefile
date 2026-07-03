SHELL := /usr/bin/env bash

BINARY_NAME ?= devspace
CMD_PATH ?= ./cmd/devspace
BIN_DIR ?= bin
DIST_DIR ?= dist
GOLANGCI_LINT ?= go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.12.2
GOVULNCHECK ?= go run golang.org/x/vuln/cmd/govulncheck@v1.1.4

.PHONY: all test vet lint vulncheck build verify clean

all: verify

test:
	go test ./...

vet:
	go vet ./...

lint:
	$(GOLANGCI_LINT) run ./...
	test -z "$$(gofmt -l cmd internal)" || (gofmt -l cmd internal && exit 1)

vulncheck:
	$(GOVULNCHECK) ./...

build:
	mkdir -p $(BIN_DIR)
	go build -trimpath -o $(BIN_DIR)/$(BINARY_NAME) $(CMD_PATH)

verify: test vet lint vulncheck build

clean:
	rm -rf $(BIN_DIR) $(DIST_DIR)
