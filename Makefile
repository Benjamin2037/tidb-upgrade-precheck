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

.PHONY: all build clean test test-kbgenerator test-precheck test-integration help

# Variables
GOBIN ?= $(CURDIR)/bin
GO ?= go

# Default target
all: build

build: kb_generator upgrade_precheck baseline_validator

# Build kb-generator
kb_generator:
	@echo "Building kb-generator..."
	@mkdir -p $(GOBIN)
	@$(GO) build -o $(GOBIN)/kb-generator ./cmd/kb_generator

# Build upgrade-precheck
upgrade_precheck:
	@echo "Building upgrade-precheck..."
	@mkdir -p $(GOBIN)
	@$(GO) build -o $(GOBIN)/upgrade-precheck ./cmd/precheck

# Build baseline-validator
baseline_validator:
	@echo "Building baseline-validator..."
	@mkdir -p $(GOBIN)
	@$(GO) build -o $(GOBIN)/baseline-validator ./cmd/baseline_validator

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	@rm -rf $(GOBIN)/kb-generator $(GOBIN)/upgrade-precheck $(GOBIN)/baseline-validator

# Run all tests
test:
	@echo "Running all tests..."
	@$(GO) test ./pkg/... ./cmd/... ./test/... -v

# Run all tests with coverage
test-coverage:
	@echo "Running all tests with coverage..."
	@$(GO) test ./pkg/... ./cmd/... ./test/... -v -coverprofile=coverage.out
	@$(GO) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Run kb-generator tests
test-kbgenerator:
	@echo "Running kb-generator tests..."
	@$(GO) test ./pkg/kbgenerator/... ./cmd/kb_generator

# Run upgrade-precheck tests
test-precheck:
	@echo "Running upgrade-precheck tests..."
	@$(GO) test ./pkg/analyzer/... ./pkg/collector/... ./pkg/reporter/... ./cmd/precheck -v

# Run integration tests
test-integration:
	@echo "Running integration tests..."
	@$(GO) test ./test/... -v

# Package for TiUP (requires knowledge base to be generated first)
package: build
	@echo "Packaging tidb-upgrade-precheck for TiUP..."
	@mkdir -p package/tidb-upgrade-precheck
	@cp $(GOBIN)/tidb-upgrade-precheck package/tidb-upgrade-precheck/
	@if [ -d knowledge ]; then \
		cp -r knowledge package/tidb-upgrade-precheck/; \
		echo "Knowledge base included in package"; \
	else \
		echo "WARNING: knowledge directory not found"; \
		echo "Please run 'bash scripts/generate_knowledge.sh' first to generate knowledge base"; \
		exit 1; \
	fi
	@echo "Package directory prepared: package/tidb-upgrade-precheck/"
	@echo "Use 'tiup package package/tidb-upgrade-precheck --name tidb-upgrade-precheck --release <version> --entry tidb-upgrade-precheck' to create TiUP package"

# Generate knowledge base (required before packaging)
generate-kb:
	@echo "Generating knowledge base..."
	@bash scripts/generate_knowledge.sh

# Validate knowledge base against baseline from tiup playground
validate-baseline:
	@echo "Validating knowledge base against baseline..."
	@bash scripts/validate_baseline.sh

# Batch validate knowledge base for all versions
validate-baseline-batch:
	@echo "Batch validating knowledge base for all versions..."
	@bash scripts/validate_baseline_batch.sh

# Validate knowledge base structure and content
validate-kb:
	@echo "Validating knowledge base..."
	@bash scripts/validate_knowledge_base.sh

# Generate versions list from knowledge base
generate-versions-list:
	@echo "Generating versions list from knowledge base..."
	@bash scripts/generate_versions_list.sh

# Full package workflow: generate KB and package
package-full: generate-kb package

# Clean package directory
clean-package:
	@echo "Cleaning package directory..."
	@rm -rf package/

# Help
help:
	@echo "Available targets:"
	@echo "  all              - Build all binaries (default)"
	@echo "  build            - Build all binaries"
	@echo "  kb_generator     - Build kb-generator"
	@echo "  upgrade_precheck - Build upgrade-precheck"
	@echo "  clean            - Clean build artifacts"
	@echo "  test             - Run all tests"
	@echo "  test-kbgenerator - Run kb-generator tests"
	@echo "  test-precheck    - Run upgrade-precheck tests"
	@echo "  test-integration - Run integration tests"
	@echo "  generate-kb      - Generate knowledge base"
	@echo "  package          - Package for TiUP (requires knowledge base)"
	@echo "  package-full     - Generate KB and package (full workflow)"
	@echo "  clean-package    - Clean package directory"
	@echo "  help             - Show this help message"