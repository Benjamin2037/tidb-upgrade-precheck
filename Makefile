# Copyright 2024 PingCAP, Inc.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# See the License for the specific language governing permissions and
# limitations under the License.

.PHONY: all build clean test test-kbgenerator test-precheck help

# Variables
GOBIN ?= $(CURDIR)/bin
GO ?= go

# Default target
all: build

# Build all binaries
build: kb-generator upgrade-precheck

# Build kb-generator
kb-generator:
	@echo "Building kb-generator..."
	@mkdir -p $(GOBIN)
	@$(GO) build -o $(GOBIN)/kb-generator ./cmd/kb-generator

# Build upgrade-precheck
upgrade-precheck:
	@echo "Building upgrade-precheck..."
	@mkdir -p $(GOBIN)
	@$(GO) build -o $(GOBIN)/upgrade-precheck ./cmd/upgrade-precheck

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	@rm -rf $(GOBIN)/kb-generator $(GOBIN)/upgrade-precheck

# Run all tests
test:
	@echo "Running all tests..."
	@$(GO) test ./pkg/... ./cmd/...

# Run kb-generator tests
test-kbgenerator:
	@echo "Running kb-generator tests..."
	@$(GO) test ./pkg/kbgenerator/... ./cmd/kb-generator

# Run upgrade-precheck tests
test-precheck:
	@echo "Running upgrade-precheck tests..."
	@$(GO) test ./pkg/analyzer/... ./pkg/runtime/... ./pkg/reporter/... ./cmd/upgrade-precheck

# Help
help:
	@echo "Available targets:"
	@echo "  all              - Build all binaries (default)"
	@echo "  build            - Build all binaries"
	@echo "  kb-generator     - Build kb-generator"
	@echo "  upgrade-precheck - Build upgrade-precheck"
	@echo "  clean            - Clean build artifacts"
	@echo "  test             - Run all tests"
	@echo "  test-kbgenerator - Run kb-generator tests"
	@echo "  test-precheck    - Run upgrade-precheck tests"
	@echo "  help             - Show this help message"