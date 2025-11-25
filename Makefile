# Makefile for TiDB Upgrade Precheck Tool

.PHONY: build build-precheck clean run collect aggregate parse-upgrade clean-generated help

# Build the kb-generator tool
build:
	cd cmd/kb-generator && go build -o ../../bin/kb-generator .

# Build the precheck tool
build-precheck:
	cd cmd/precheck && go build -o ../../bin/precheck .

# Clean build artifacts
clean:
	rm -rf bin/

# Run the tool with default options
run:
	go run cmd/kb-generator/main.go

# Collect current TiDB parameters
collect:
	go run cmd/kb-generator/main.go --all

# Collect all TiDB parameters including already generated ones
collect-all:
	go run cmd/kb-generator/main.go --all --skip-generated=false

# Aggregate parameter histories
aggregate:
	go run cmd/kb-generator/main.go --aggregate

# Parse upgrade logic
parse-upgrade:
	go run cmd/kb-generator/main.go --all

# Run precheck
precheck:
	go run cmd/precheck/main.go

# Run precheck with markdown report
precheck-md:
	go run cmd/precheck/main.go --format=markdown --report-dir=./out

# Run precheck with HTML report
precheck-html:
	go run cmd/precheck/main.go --format=html --report-dir=./out

# Clean generated version records
clean-generated:
	rm -f knowledge/generated_versions.json

# Help
help:
	@echo "Available targets:"
	@echo "  build            - Build the kb-generator tool"
	@echo "  build-precheck   - Build the precheck tool"
	@echo "  clean            - Clean build artifacts"
	@echo "  run              - Run the tool with default options"
	@echo "  collect          - Collect all TiDB parameters (skipping already generated)"
	@echo "  collect-all      - Collect all TiDB parameters (including already generated)"
	@echo "  aggregate        - Aggregate parameter histories"
	@echo "  parse-upgrade    - Parse upgrade logic"
	@echo "  precheck         - Run upgrade precheck"
	@echo "  precheck-md      - Run upgrade precheck and generate markdown report"
	@echo "  precheck-html    - Run upgrade precheck and generate HTML report"
	@echo "  clean-generated  - Clean generated version records"
	@echo "  help             - Show this help message"