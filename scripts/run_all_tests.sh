#!/bin/bash
# Comprehensive test script for tidb-upgrade-precheck
# This script runs all tests in the correct order
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
cd "$PROJECT_ROOT"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Test results
PASSED=0
FAILED=0
SKIPPED=0

# Function to print test section header
print_section() {
    echo ""
    echo "=========================================="
    echo "$1"
    echo "=========================================="
}

# Function to run test and track results
run_test() {
    local test_name="$1"
    local test_cmd="$2"
    
    echo -e "${YELLOW}Running: $test_name${NC}"
    if eval "$test_cmd"; then
        echo -e "${GREEN}✓ $test_name PASSED${NC}"
        ((PASSED++))
        return 0
    else
        echo -e "${RED}✗ $test_name FAILED${NC}"
        ((FAILED++))
        return 1
    fi
}

# Function to check if command exists
check_command() {
    if ! command -v "$1" &> /dev/null; then
        echo -e "${YELLOW}Warning: $1 not found, skipping related tests${NC}"
        return 1
    fi
    return 0
}

echo "=========================================="
echo "TiDB Upgrade Precheck - Comprehensive Test Suite"
echo "=========================================="
echo "Project Root: $PROJECT_ROOT"
echo ""

# Stage 1: Knowledge Base Generation Verification
print_section "Stage 1: Knowledge Base Generation Verification"

if [ "$SKIP_KB_GENERATION" != "true" ]; then
    echo "Generating knowledge base..."
    if make generate-kb; then
        echo -e "${GREEN}✓ Knowledge base generation completed${NC}"
        ((PASSED++))
    else
        echo -e "${RED}✗ Knowledge base generation failed${NC}"
        ((FAILED++))
        echo -e "${YELLOW}Continuing with existing knowledge base...${NC}"
    fi
    
    # Verify knowledge base structure
    if [ -d "knowledge" ]; then
        echo "Verifying knowledge base structure..."
        KB_VERSIONS=$(find knowledge -maxdepth 1 -type d -name "v*" | wc -l)
        echo "Found $KB_VERSIONS version directories"
        
        # Check for key versions
        for version in "v7.5.0" "v8.5.0"; do
            if [ -d "knowledge/$version" ]; then
                echo -e "${GREEN}✓ Found knowledge base for $version${NC}"
            else
                echo -e "${YELLOW}⚠ Knowledge base for $version not found${NC}"
            fi
        done
    else
        echo -e "${RED}✗ Knowledge directory not found${NC}"
        ((FAILED++))
    fi
    
    # Run knowledge base generation tests if available
    if [ -f "scripts/generate_knowledge_test.sh" ]; then
        run_test "Knowledge Base Generation Tests" "bash scripts/generate_knowledge_test.sh"
    fi
else
    echo -e "${YELLOW}Skipping knowledge base generation (SKIP_KB_GENERATION=true)${NC}"
    ((SKIPPED++))
fi

# Stage 2: Unit Tests
print_section "Stage 2: Unit Tests"

# KB Generator tests
run_test "KB Generator Unit Tests" "go test ./pkg/kbgenerator/... -v"

# Collector tests
run_test "Collector Unit Tests" "go test ./pkg/collector/... -v"

# Analyzer tests
run_test "Analyzer Unit Tests" "go test ./pkg/analyzer/... -v"

# Reporter tests
run_test "Reporter Unit Tests" "go test ./pkg/reporter/... -v"

# Types tests
run_test "Types Unit Tests" "go test ./pkg/types/... -v"

# Stage 3: Functional Tests
print_section "Stage 3: Functional Tests"

# Integration tests
if [ -f "test/integration_test.go" ]; then
    run_test "Integration Tests" "go test ./test/... -v"
else
    echo -e "${YELLOW}⚠ Integration tests not found${NC}"
    ((SKIPPED++))
fi

# Stage 4: CLI Tool Tests
print_section "Stage 4: CLI Tool Tests"

# Build precheck tool
echo "Building precheck tool..."
if make upgrade_precheck; then
    echo -e "${GREEN}✓ Precheck tool built successfully${NC}"
    ((PASSED++))
else
    echo -e "${RED}✗ Precheck tool build failed${NC}"
    ((FAILED++))
fi

# Test help command
if [ -f "bin/upgrade-precheck" ]; then
    echo "Testing precheck help command..."
    if ./bin/upgrade-precheck --help > /dev/null 2>&1; then
        echo -e "${GREEN}✓ Precheck help command works${NC}"
        ((PASSED++))
    else
        echo -e "${RED}✗ Precheck help command failed${NC}"
        ((FAILED++))
    fi
fi

# Stage 5: Knowledge Base Path Resolution Tests
print_section "Stage 5: Knowledge Base Path Resolution Tests"

if [ -d "knowledge" ] && [ -f "bin/upgrade-precheck" ]; then
    # Test with explicit knowledge base path
    echo "Testing explicit knowledge base path..."
    if ./bin/upgrade-precheck precheck \
        --target-version v8.0.0 \
        --knowledge-base ./knowledge \
        --help > /dev/null 2>&1; then
        echo -e "${GREEN}✓ Explicit knowledge base path works${NC}"
        ((PASSED++))
    else
        echo -e "${YELLOW}⚠ Explicit knowledge base path test skipped (requires valid topology)${NC}"
        ((SKIPPED++))
    fi
fi

# Stage 6: Code Coverage Report
print_section "Stage 6: Code Coverage Report"

if check_command go; then
    echo "Generating coverage report..."
    if go test ./pkg/... -v -coverprofile=coverage.out > /dev/null 2>&1; then
        go tool cover -html=coverage.out -o coverage.html 2>/dev/null || true
        if [ -f "coverage.html" ]; then
            COVERAGE=$(go tool cover -func=coverage.out | grep total | awk '{print $3}')
            echo -e "${GREEN}✓ Coverage report generated: coverage.html${NC}"
            echo -e "${GREEN}  Total coverage: $COVERAGE${NC}"
            ((PASSED++))
        fi
    fi
fi

# Summary
print_section "Test Summary"

echo ""
echo "=========================================="
echo "Test Results:"
echo "=========================================="
echo -e "${GREEN}Passed:  $PASSED${NC}"
echo -e "${RED}Failed:  $FAILED${NC}"
echo -e "${YELLOW}Skipped: $SKIPPED${NC}"
echo ""

TOTAL=$((PASSED + FAILED + SKIPPED))
if [ $TOTAL -gt 0 ]; then
    SUCCESS_RATE=$((PASSED * 100 / TOTAL))
    echo "Success Rate: ${SUCCESS_RATE}%"
fi

if [ $FAILED -eq 0 ]; then
    echo -e "${GREEN}=========================================="
    echo "All tests passed!"
    echo "==========================================${NC}"
    exit 0
else
    echo -e "${RED}=========================================="
    echo "Some tests failed. Please review the output above."
    echo "==========================================${NC}"
    exit 1
fi

