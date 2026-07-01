SHELL := /usr/bin/env bash

BINARY_NAME ?= devspace
CMD_PATH ?= ./cmd/devdrop
BIN_DIR ?= bin
DIST_DIR ?= dist
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
GOOS ?= $(shell go env GOOS)
GOARCH ?= $(shell go env GOARCH)
ARTIFACT_BASE := $(BINARY_NAME)_$(VERSION)_$(GOOS)_$(GOARCH)
ARTIFACT_DIR := $(DIST_DIR)/$(ARTIFACT_BASE)

.PHONY: all test vet build verify release checksums clean

all: verify

test:
	go test ./...

vet:
	go vet ./...

build:
	mkdir -p $(BIN_DIR)
	go build -trimpath -o $(BIN_DIR)/$(BINARY_NAME) $(CMD_PATH)

verify: test vet build

release: verify
	rm -rf $(ARTIFACT_DIR)
	mkdir -p $(ARTIFACT_DIR)
	cp $(BIN_DIR)/$(BINARY_NAME) $(ARTIFACT_DIR)/$(BINARY_NAME)
	cp README.md $(ARTIFACT_DIR)/README.md
	cp docs/release.md $(ARTIFACT_DIR)/RELEASE.md
	( cd $(DIST_DIR) && tar -czf $(ARTIFACT_BASE).tar.gz $(ARTIFACT_BASE) )
	$(MAKE) checksums

checksums:
	mkdir -p $(DIST_DIR)
	cd $(DIST_DIR) && if command -v sha256sum >/dev/null 2>&1; then sha256sum $(ARTIFACT_BASE).tar.gz > SHA256SUMS; else shasum -a 256 $(ARTIFACT_BASE).tar.gz > SHA256SUMS; fi

clean:
	rm -rf $(BIN_DIR) $(DIST_DIR)
