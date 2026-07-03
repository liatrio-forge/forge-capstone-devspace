SHELL := /usr/bin/env bash

BINARY_NAME ?= devspace
CMD_PATH ?= ./cmd/devspace
BIN_DIR ?= bin
DIST_DIR ?= dist

.PHONY: all test vet build verify lint vulncheck clean

all: verify

test:
	go test ./...

vet:
	go vet ./...

build:
	mkdir -p $(BIN_DIR)
	go build -trimpath -o $(BIN_DIR)/$(BINARY_NAME) $(CMD_PATH)

lint:
	golangci-lint run --timeout 5m
	test -z "$$(gofmt -l cmd internal)" || (gofmt -l cmd internal && exit 1)

vulncheck:
	go run golang.org/x/vuln/cmd/govulncheck@latest ./...

verify: test vet lint build

clean:
	rm -rf $(BIN_DIR) $(DIST_DIR)
