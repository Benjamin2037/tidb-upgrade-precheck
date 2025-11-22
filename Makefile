# Makefile for TiDB Upgrade Precheck Tool

.PHONY: build clean run collect aggregate parse-upgrade clean-generated help

# Build the kb-generator tool
build:
	cd cmd/kb-generator && go build -o ../../bin/kb-generator .

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

# Clean generated version records
clean-generated:
	rm -f knowledge/generated_versions.json

# Help
help:
	@echo "Available targets:"
	@echo "  build            - Build the kb-generator tool"
	@echo "  clean            - Clean build artifacts"
	@echo "  run              - Run the tool with default options"
	@echo "  collect          - Collect all TiDB parameters (skipping already generated)"
	@echo "  collect-all      - Collect all TiDB parameters (including already generated)"
	@echo "  aggregate        - Aggregate parameter histories"
	@echo "  parse-upgrade    - Parse upgrade logic"
	@echo "  clean-generated  - Clean generated version records"
	@echo "  help             - Show this help message"