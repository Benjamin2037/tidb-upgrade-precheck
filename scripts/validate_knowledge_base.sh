#!/bin/bash
# Knowledge base validation script
# Validates that the generated knowledge base is correct

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$PROJECT_ROOT"

# Color output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Statistics
TOTAL_CHECKS=0
PASSED_CHECKS=0
FAILED_CHECKS=0
WARNINGS=0

# Print result
print_result() {
    local status=$1
    local message=$2
    TOTAL_CHECKS=$((TOTAL_CHECKS + 1))
    
    if [ "$status" = "PASS" ]; then
        echo -e "${GREEN}✓${NC} $message"
        PASSED_CHECKS=$((PASSED_CHECKS + 1))
    elif [ "$status" = "FAIL" ]; then
        echo -e "${RED}✗${NC} $message"
        FAILED_CHECKS=$((FAILED_CHECKS + 1))
    elif [ "$status" = "WARN" ]; then
        echo -e "${YELLOW}⚠${NC} $message"
        WARNINGS=$((WARNINGS + 1))
    fi
}

# Check JSON format
check_json_format() {
    local file=$1
    if jq . "$file" > /dev/null 2>&1; then
        print_result "PASS" "JSON format valid: $file"
    else
        print_result "FAIL" "JSON format invalid: $file"
        return 1
    fi
}

# Check knowledge base structure
check_kb_structure() {
    local version=$1
    local version_group=$(echo "$version" | sed -E 's/(v[0-9]+\.[0-9]+)\..*/\1/')
    local kb_dir="knowledge/$version_group/$version"
    
    # Debug: print path being checked
    # echo "Checking: $kb_dir"
    
    if [ ! -d "$kb_dir" ]; then
        print_result "FAIL" "Knowledge base directory not found: $kb_dir"
        return 1
    fi
    
    local components=("tidb" "pd" "tikv" "tiflash")
    for component in "${components[@]}"; do
        local defaults_file="$kb_dir/$component/defaults.json"
        if [ -f "$defaults_file" ]; then
            print_result "PASS" "Found defaults.json for $component in $version"
            check_json_format "$defaults_file"
        else
            print_result "WARN" "Missing defaults.json for $component in $version"
        fi
    done
}

# Check configuration parameter count
check_config_count() {
    local file=$1
    local component=$2
    local version=$3
    
    local config_count=$(jq '.config_defaults | length' "$file" 2>/dev/null || echo "0")
    local vars_count=$(jq '.system_variables | length' "$file" 2>/dev/null || echo "0")
    
    # Set reasonable thresholds based on component
    local min_config=0
    local min_vars=0
    case "$component" in
        tidb)
            min_config=50  # TiDB should have a large number of configuration parameters
            min_vars=500  # TiDB should have a large number of system variables
            ;;
        pd)
            min_config=20
            min_vars=0
            ;;
        tikv)
            min_config=30
            min_vars=0
            ;;
        tiflash)
            min_config=20
            min_vars=0
            ;;
    esac
    
    if [ "$config_count" -lt "$min_config" ]; then
        print_result "WARN" "$component/$version: config_defaults count ($config_count) is below threshold ($min_config)"
    else
        print_result "PASS" "$component/$version: config_defaults count: $config_count"
    fi
    
    if [ "$vars_count" -lt "$min_vars" ]; then
        print_result "WARN" "$component/$version: system_variables count ($vars_count) is below threshold ($min_vars)"
    else
        print_result "PASS" "$component/$version: system_variables count: $vars_count"
    fi
}

# Check key configuration items
check_key_configs() {
    local file=$1
    local component=$2
    
    local key_configs=()
    case "$component" in
        tidb)
            key_configs=("port" "host" "path" "store" "lease")
            ;;
        pd)
            key_configs=("name" "data-dir" "client-urls" "peer-urls")
            ;;
        tikv)
            key_configs=("addr" "data-dir" "pd-endpoints")
            ;;
        tiflash)
            key_configs=("flash.proxy.addr" "flash.service_addr")
            ;;
    esac
    
    for key in "${key_configs[@]}"; do
        if jq -e ".config_defaults.\"$key\"" "$file" > /dev/null 2>&1; then
            print_result "PASS" "$component: key config '$key' exists"
        else
            print_result "WARN" "$component: key config '$key' missing"
        fi
    done
}

# Check value types
check_value_types() {
    local file=$1
    local component=$2
    
    # Check type field for all configuration items
    local invalid_types=$(jq -r '.config_defaults | to_entries[] | select(.value.type == null or .value.type == "") | .key' "$file" 2>/dev/null | head -5)
    
    if [ -n "$invalid_types" ]; then
        print_result "WARN" "$component: found config items with missing type field"
    else
        print_result "PASS" "$component: all config items have type field"
    fi
}

# Compare with baseline (if exists)
compare_with_baseline() {
    local version=$1
    local component=$2
    
    local baseline_file="baseline/${version}-baseline.json"
    if [ ! -f "$baseline_file" ]; then
        return 0  # Baseline does not exist, skip
    fi
    
    # Use baseline_validator for comparison
    if [ -f "bin/baseline-validator" ]; then
        # Can call baseline_validator here for detailed comparison
        print_result "PASS" "$component/$version: baseline comparison available"
    else
        print_result "WARN" "$component/$version: baseline exists but validator not found"
    fi
}

# Main validation function
validate_version() {
    local version=$1
    echo ""
    echo "=========================================="
    echo "Validating knowledge base for $version"
    echo "=========================================="
    
    check_kb_structure "$version"
    
    local version_group=$(echo "$version" | sed 's/\(v[0-9]\+\.[0-9]\+\)\..*/\1/')
    local kb_dir="knowledge/$version_group/$version"
    local components=("tidb" "pd" "tikv" "tiflash")
    
    for component in "${components[@]}"; do
        local defaults_file="$kb_dir/$component/defaults.json"
        if [ -f "$defaults_file" ]; then
            echo ""
            echo "--- Validating $component ---"
            check_config_count "$defaults_file" "$component" "$version"
            check_key_configs "$defaults_file" "$component"
            check_value_types "$defaults_file" "$component"
            compare_with_baseline "$version" "$component"
        fi
    done
}

# Main function
main() {
    echo "=========================================="
    echo "Knowledge Base Validation"
    echo "=========================================="
    echo ""
    
    # If version arguments are provided, only validate those versions
    if [ $# -gt 0 ]; then
        for version in "$@"; do
            validate_version "$version"
        done
    else
        # Validate all LTS versions
        local lts_versions=(
            "v6.5.0" "v6.5.1" "v6.5.2" "v6.5.3" "v6.5.4" "v6.5.5" "v6.5.6" "v6.5.7" "v6.5.8" "v6.5.9" "v6.5.10" "v6.5.11" "v6.5.12"
            "v7.1.0" "v7.1.1" "v7.1.2" "v7.1.3" "v7.1.4" "v7.1.5" "v7.1.6"
            "v7.5.0" "v7.5.1" "v7.5.2" "v7.5.3" "v7.5.4" "v7.5.5" "v7.5.6" "v7.5.7"
            "v8.1.0" "v8.1.1" "v8.1.2"
            "v8.5.0" "v8.5.1" "v8.5.2" "v8.5.3" "v8.5.4"
        )
        
        for version in "${lts_versions[@]}"; do
            local version_group=$(echo "$version" | sed 's/\(v[0-9]\+\.[0-9]\+\)\..*/\1/')
            if [ -d "knowledge/$version_group/$version" ]; then
                validate_version "$version"
            fi
        done
    fi
    
    # Print summary
    echo ""
    echo "=========================================="
    echo "Validation Summary"
    echo "=========================================="
    echo "Total checks: $TOTAL_CHECKS"
    echo -e "${GREEN}Passed: $PASSED_CHECKS${NC}"
    echo -e "${RED}Failed: $FAILED_CHECKS${NC}"
    echo -e "${YELLOW}Warnings: $WARNINGS${NC}"
    echo ""
    
    if [ $FAILED_CHECKS -gt 0 ]; then
        echo -e "${RED}Validation failed!${NC}"
        exit 1
    elif [ $WARNINGS -gt 0 ]; then
        echo -e "${YELLOW}Validation passed with warnings${NC}"
        exit 0
    else
        echo -e "${GREEN}All validations passed!${NC}"
        exit 0
    fi
}

main "$@"

