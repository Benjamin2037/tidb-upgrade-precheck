#!/bin/bash
# Generate full knowledge base for all LTS versions (v6.5.0 and later) using tiup playground
# This script collects runtime configuration from actual running clusters
# and merges with code definitions.
#
# Usage:
#   ./scripts/generate_knowledge.sh [options]
#
# Options:
#   --skip-existing    Skip versions that already have knowledge base files
#   --force            Force regeneration: delete and recreate knowledge directory, clean logs directory
#   --components=LIST   Comma-separated list of components (tidb,pd,tikv,tiflash)
#   --versions=FILE    Path to versions list file (optional, defaults to auto-detect from git tags)
#   --repo=REPO        Repository to get tags from (default: TIDB_REPO or ../tidb)
#   --start-from=VER   Start from a specific version (e.g., v7.5.0)
#   --stop-at=VER      Stop at a specific version (e.g., v8.1.0)
#   --max-concurrent=N Maximum number of versions to process concurrently (default: 1, serial mode)
#   --serial           Alias for --max-concurrent=1 (serial execution, one version at a time)
# 
# Environment variables:
#   TIDB_REPO: Path to TiDB repository (default: ../tidb)
#              Required for: code definitions extraction and upgrade_logic.json generation
#              Optional: If not provided, only runtime config will be collected (no code definitions)
#   PD_REPO: Path to PD repository (default: ../pd)
#            Required for: code definitions extraction
#            Optional: If not provided, only runtime config will be collected (no code definitions)
#   TIKV_REPO: Path to TiKV repository (default: ../tikv)
#              Required for: code definitions extraction
#              Optional: If not provided, only runtime config will be collected (no code definitions)
#   TIFLASH_REPO: Path to TiFlash repository (default: ../tiflash)
#                 Required for: code definitions extraction
#                 Optional: If not provided, only runtime config will be collected (no code definitions)
#
# Note: Even when using tiup playground to start clusters, repo paths are still needed for:
#   - Extracting code definitions (parameter defaults from source code)
#   - Generating upgrade_logic.json (TiDB only, from bootstrap.go)
#   If repo paths are not provided, the script will still work but only collect runtime configuration
#   from the running cluster (missing code definitions that may not appear in runtime config)
#
# Knowledge Base Directory Structure:
#   knowledge/
#     ├── v6.5/
#     │   ├── v6.5.0/
#     │   │   ├── tidb/defaults.json
#     │   │   ├── pd/defaults.json
#     │   │   ├── tikv/defaults.json
#     │   │   └── tiflash/defaults.json
#     │   └── ... (all LTS versions in v6.5 group)
#     ├── v7.1/
#     │   └── ... (same structure)
#     ├── tidb/
#     │   └── upgrade_logic.json
#     └── ... (other components' upgrade_logic.json)
#
# Run Directory Structure:
#   run/
#     ├── logs/          # Generation logs (knowledge_generation_*.log)
#     └── tmp/           # Temporary files (.version_results)

set -euo pipefail

# Default values
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
RUN_DIR="${PROJECT_ROOT}/run"
LOGS_DIR="${RUN_DIR}/logs"
TMP_DIR="${RUN_DIR}/tmp"
VERSIONS_FILE=""
SKIP_EXISTING=false
FORCE_REGENERATE=false
COMPONENTS="tidb,pd,tikv,tiflash"
START_FROM=""
STOP_AT=""
TAG_REPO=""
MAX_CONCURRENT=1

# Create run directories if they don't exist
mkdir -p "$LOGS_DIR" "$TMP_DIR"

# Repository paths (default to relative paths from project root)
# These are used for extracting code definitions and upgrade_logic.json
# Even when using tiup playground, repo paths are needed to:
#   1. Extract code definitions (parameter defaults from source code)
#   2. Generate upgrade_logic.json (TiDB only, from bootstrap.go)
# If not provided, only runtime configuration will be collected (no code definitions)
TIDB_REPO=${TIDB_REPO:-${PROJECT_ROOT}/../tidb}
PD_REPO=${PD_REPO:-${PROJECT_ROOT}/../pd}
TIKV_REPO=${TIKV_REPO:-${PROJECT_ROOT}/../tikv}
TIFLASH_REPO=${TIFLASH_REPO:-${PROJECT_ROOT}/../tiflash}

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --skip-existing)
            SKIP_EXISTING=true
            shift
            ;;
        --force)
            FORCE_REGENERATE=true
            shift
            ;;
        --components=*)
            COMPONENTS="${1#*=}"
            shift
            ;;
        --versions=*)
            VERSIONS_FILE="${1#*=}"
            shift
            ;;
        --repo=*)
            TAG_REPO="${1#*=}"
            shift
            ;;
        --start-from=*)
            START_FROM="${1#*=}"
            shift
            ;;
        --stop-at=*)
            STOP_AT="${1#*=}"
            shift
            ;;
        --max-concurrent=*)
            MAX_CONCURRENT="${1#*=}"
            # Validate it's a number
            if ! [[ "$MAX_CONCURRENT" =~ ^[0-9]+$ ]] || [ "$MAX_CONCURRENT" -lt 1 ]; then
                echo "Error: --max-concurrent must be a positive integer"
                exit 1
            fi
            shift
            ;;
        --serial)
            # Alias for --max-concurrent=1
            MAX_CONCURRENT=1
            shift
            ;;
        *)
            echo "Unknown option: $1"
            echo "Usage: $0 [--skip-existing] [--force] [--components=tidb,pd] [--versions=FILE] [--repo=REPO] [--start-from=VER] [--stop-at=VER] [--max-concurrent=N] [--serial]"
            exit 1
          ;;
      esac
done

# Change to project root
cd "$PROJECT_ROOT"

# Function to get version group from version tag (e.g., v6.5.0 -> v6.5)
get_version_group() {
    local version="$1"
    # Remove 'v' prefix and extract major.minor
    echo "$version" | sed 's/v\([0-9]*\)\.\([0-9]*\)\..*/v\1.\2/'
}

# Function to check if version is LTS and standard format (vX.Y.Z only)
# LTS versions are: v6.5.x, v7.1.x, v7.5.x, v8.1.x, v8.5.x
is_lts_version() {
    local version="$1"
    
    # Only accept standard format: vX.Y.Z (no suffixes like -20230109)
    if [[ ! "$version" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
        return 1  # Not standard format
    fi
    
    # Remove 'v' prefix
    local num_version=$(echo "$version" | sed 's/^v//')
    
    # Extract major.minor
    local major=$(echo "$num_version" | cut -d. -f1)
    local minor=$(echo "$num_version" | cut -d. -f2)
    
    # Check if version is one of the LTS series
    case "$major.$minor" in
        6.5) return 0 ;;  # v6.5.x
        7.1) return 0 ;;  # v7.1.x
        7.5) return 0 ;;  # v7.5.x
        8.1) return 0 ;;  # v8.1.x
        8.5) return 0 ;;  # v8.5.x
        *)   return 1 ;;  # Not an LTS version
    esac
}

# Function to get versions from git tags (LTS versions only, starting from v6.5.0)
get_versions_from_tags() {
    local repo_path="$1"
    local temp_file=$(mktemp)
    
    if [ ! -d "$repo_path" ]; then
        echo "Error: Repository not found: $repo_path" >&2
        rm -f "$temp_file"
        return 1
      fi
    
    # Get all tags matching standard version pattern (vX.Y.Z only, no suffixes)
    # Use strict pattern to exclude versions like v6.5.0-20230109 or v6.6.0-alpha
    (cd "$repo_path" && git tag -l | grep -E "^v[0-9]+\.[0-9]+\.[0-9]+$" | sort -V) > "$temp_file"
    
    # Filter LTS versions (v6.5.0 and later) and format as version_group/version
    while IFS= read -r version; do
        if is_lts_version "$version"; then
            version_group=$(get_version_group "$version")
            echo "${version_group}/${version}"
        fi
    done < "$temp_file"
    
    rm -f "$temp_file"
}

# Get versions list
if [ -z "$VERSIONS_FILE" ]; then
    # Auto-detect from git tags
    if [ -z "$TAG_REPO" ]; then
        # Default to TIDB_REPO
        TAG_REPO=${TIDB_REPO:-${PROJECT_ROOT}/../tidb}
    fi
    
    echo "Auto-detecting LTS versions (v6.5.0 and later) from git tags in: $TAG_REPO"
    VERSIONS_TEMP=$(mktemp)
    if ! get_versions_from_tags "$TAG_REPO" > "$VERSIONS_TEMP"; then
        echo "Error: Failed to get versions from git tags"
        echo "Please specify --versions=FILE or ensure repository exists at $TAG_REPO"
        exit 1
    fi
    
    VERSION_COUNT=$(wc -l < "$VERSIONS_TEMP" | tr -d ' ')
    echo "Found $VERSION_COUNT LTS versions (v6.5.0 and later) from git tags"
    VERSIONS_FILE="$VERSIONS_TEMP"
    USE_TEMP_FILE=true
else
    # Use provided file
    if [ ! -f "$VERSIONS_FILE" ]; then
        echo "Error: Versions file not found: $VERSIONS_FILE"
        exit 1
    fi
    USE_TEMP_FILE=false
fi

# Store components in a simple array for checking
COMPONENT_LIST="${COMPONENTS//,/ }"

# Function to check if knowledge base exists for a version and component
knowledge_exists() {
    local version_group="$1"
    local version="$2"
    local component="$3"
    local kb_file="${PROJECT_ROOT}/knowledge/${version_group}/${version}/${component}/defaults.json"
    [ -f "$kb_file" ]
}

# Function to extract version from version_group/version format
extract_version() {
    local line="$1"
    # Format: v6.5/v6.5.0 -> v6.5.0
    if [[ "$line" =~ ^v[0-9]+\.[0-9]+/v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
        echo "${line#*/}"
    else
        echo "$line"
    fi
}

# Function to extract version group from version_group/version format
extract_version_group() {
    local line="$1"
    # Format: v6.5/v6.5.0 -> v6.5
    if [[ "$line" =~ ^(v[0-9]+\.[0-9]+)/ ]]; then
        echo "${BASH_REMATCH[1]}"
    else
        # Fallback: extract from version
        local version=$(extract_version "$line")
        local major_minor=$(echo "$version" | sed 's/v\([0-9]*\)\.\([0-9]*\)\..*/v\1.\2/')
        echo "$major_minor"
    fi
}

# Read versions from file
VERSIONS=()
while IFS= read -r line || [ -n "$line" ]; do
    line=$(echo "$line" | xargs)  # Trim whitespace
    [ -z "$line" ] && continue
    [ "${line:0:1}" = "#" ] && continue  # Skip comments
    
    VERSIONS+=("$line")
done < "$VERSIONS_FILE"

# Filter versions based on --start-from and --stop-at
FILTERED_VERSIONS=()
START_FOUND=false
if [ -z "$START_FROM" ]; then
    START_FOUND=true
fi

for version_line in "${VERSIONS[@]}"; do
    version=$(extract_version "$version_line")
    
    # Check if we should start from this version
    if [ -n "$START_FROM" ] && [ "$version" = "$START_FROM" ]; then
        START_FOUND=true
    fi
    
    # Skip until start version
    if [ "$START_FOUND" = false ]; then
        continue
    fi
    
    # Add to filtered list
    FILTERED_VERSIONS+=("$version_line")
    
    # Check if we should stop at this version
    if [ -n "$STOP_AT" ] && [ "$version" = "$STOP_AT" ]; then
        break
    fi
done

TOTAL_VERSIONS=${#FILTERED_VERSIONS[@]}
echo "=========================================="
echo "Knowledge Base Generation"
echo "=========================================="
echo "Total versions to process: $TOTAL_VERSIONS"
echo "Components: $COMPONENTS"
echo "Skip existing: $SKIP_EXISTING"
echo "Max concurrent: $MAX_CONCURRENT (serial mode: $([ "$MAX_CONCURRENT" -eq 1 ] && echo "yes" || echo "no"))"
[ -n "$START_FROM" ] && echo "Start from: $START_FROM"
[ -n "$STOP_AT" ] && echo "Stop at: $STOP_AT"
echo ""

# Force regeneration: clean knowledge and logs directories
KNOWLEDGE_DIR="${PROJECT_ROOT}/knowledge"
if [ "$FORCE_REGENERATE" = true ]; then
    echo "=========================================="
    echo "Force Regeneration Mode"
    echo "=========================================="
    echo "This will delete and recreate:"
    echo "  - knowledge/ directory"
    echo "  - run/logs/ directory"
    echo ""
    
    # Clean logs directory
    if [ -d "$LOGS_DIR" ]; then
        echo "Cleaning logs directory: $LOGS_DIR"
        rm -rf "$LOGS_DIR"/*
        echo "✓ Logs directory cleaned"
    fi
    
    # Clean and rebuild knowledge directory
    if [ -d "$KNOWLEDGE_DIR" ]; then
        echo "Removing knowledge directory: $KNOWLEDGE_DIR"
        rm -rf "$KNOWLEDGE_DIR"
    fi
    
    # Recreate knowledge directory
    echo "Creating fresh knowledge directory..."
    mkdir -p "$KNOWLEDGE_DIR"
    echo "✓ Knowledge directory created"
    echo ""
elif [ -d "$KNOWLEDGE_DIR" ]; then
    # Non-force mode: preserve existing knowledge base
    if [ "$SKIP_EXISTING" = true ]; then
        # When using --skip-existing, keep all existing knowledge base
        echo "=========================================="
        echo "Knowledge directory exists, preserving all existing knowledge base"
        echo "=========================================="
        echo "Using --skip-existing: will only generate missing versions"
        echo "Existing versions will be skipped"
        echo ""
    else
        # Without --skip-existing, clean and regenerate (old behavior)
        echo "=========================================="
        echo "Knowledge directory exists, preserving upgrade_logic.json files..."
        echo "=========================================="
        
        # Backup upgrade_logic.json files (they are not version-specific)
        UPGRADE_LOGIC_BACKUP=$(mktemp -d)
        echo "  Backing up upgrade_logic.json files..."
        for component in tidb pd tikv tiflash; do
            if [ -f "${KNOWLEDGE_DIR}/${component}/upgrade_logic.json" ]; then
                mkdir -p "${UPGRADE_LOGIC_BACKUP}/${component}"
                cp "${KNOWLEDGE_DIR}/${component}/upgrade_logic.json" "${UPGRADE_LOGIC_BACKUP}/${component}/" 2>/dev/null || true
            fi
        done
        
        # Remove the entire knowledge directory
        echo "  Removing knowledge directory: $KNOWLEDGE_DIR"
        rm -rf "$KNOWLEDGE_DIR"
        
        # Recreate knowledge directory structure
        echo "  Recreating knowledge directory structure..."
        mkdir -p "$KNOWLEDGE_DIR"
        
        # Restore upgrade_logic.json files if they existed
        if [ -d "$UPGRADE_LOGIC_BACKUP" ]; then
            echo "  Restoring upgrade_logic.json files..."
            for component in tidb pd tikv tiflash; do
                if [ -f "${UPGRADE_LOGIC_BACKUP}/${component}/upgrade_logic.json" ]; then
                    mkdir -p "${KNOWLEDGE_DIR}/${component}"
                    cp "${UPGRADE_LOGIC_BACKUP}/${component}/upgrade_logic.json" "${KNOWLEDGE_DIR}/${component}/" 2>/dev/null || true
                fi
            done
        fi
        
        # Cleanup backup directory
        rm -rf "$UPGRADE_LOGIC_BACKUP"
        
        echo "✓ Knowledge directory cleaned and ready for full regeneration"
        echo ""
    fi
else
    echo "=========================================="
    echo "Creating knowledge base directory..."
    echo "=========================================="
    mkdir -p "$KNOWLEDGE_DIR"
    echo "✓ Knowledge directory created"
    echo ""
fi

# Statistics
SUCCESS_COUNT=0
SKIP_COUNT=0
FAIL_COUNT=0
FAILED_VERSIONS=()

# Process each version with limited concurrency
# Each version will:
# 1. Create a new playground cluster with unique tag
# 2. Generate knowledge base for all components
# 3. Clean up the cluster immediately after generation
CURRENT=0
PIDS=()  # Array to store background process PIDs
PENDING_VERSIONS=()  # Array to store pending versions

# First, collect all versions to process
for version_line in "${FILTERED_VERSIONS[@]}"; do
    version=$(extract_version "$version_line")
    version_group=$(extract_version_group "$version_line")
    
    # Check if we should skip this version
    SKIP_VERSION=false
    if [ "$SKIP_EXISTING" = true ]; then
        ALL_EXIST=true
        for component in $COMPONENT_LIST; do
            if ! knowledge_exists "$version_group" "$version" "$component"; then
                ALL_EXIST=false
                break
  fi
done

        if [ "$ALL_EXIST" = true ]; then
            SKIP_COUNT=$((SKIP_COUNT + 1))
            SKIP_VERSION=true
        fi
    fi
    
    if [ "$SKIP_VERSION" = false ]; then
        PENDING_VERSIONS+=("$version_line")
    fi
done

TOTAL_PENDING=${#PENDING_VERSIONS[@]}
echo "=========================================="
echo "Knowledge Base Generation (with concurrency limit)"
echo "=========================================="
echo "Total versions to process: $TOTAL_PENDING"
echo "Max concurrent: $MAX_CONCURRENT"
echo "Components: $COMPONENTS"
echo ""

# Note: upgrade_logic.json is now automatically generated by kb_generator
# when TiDB component is included. No need to generate it separately here.

# Function to count running processes
count_running() {
    local count=0
    # Handle empty array case to avoid "unbound variable" error with set -u
    if [ ${#PIDS[@]} -eq 0 ]; then
        echo 0
        return
    fi
    for pid in "${PIDS[@]}"; do
        if kill -0 "$pid" 2>/dev/null; then
            count=$((count + 1))
        fi
    done
    echo $count
}

# Function to cleanup completed clusters
cleanup_completed_clusters() {
    # Find and cleanup clusters for completed processes
    local cleaned=0
    local new_pids=()
    
    # Handle empty array case to avoid "unbound variable" error with set -u
    if [ ${#PIDS[@]} -eq 0 ]; then
        # No PIDS to check, just cleanup old cluster directories
        # Keep new_pids empty
        :
    else
        for pid in "${PIDS[@]}"; do
            if kill -0 "$pid" 2>/dev/null; then
                # Process still running, keep it
                new_pids+=($pid)
            else
                # Process completed, cleanup its cluster
                # Find cluster tag from process info (if available)
                # Since we can't easily get tag from PID, we'll cleanup old clusters periodically
                cleaned=$((cleaned + 1))
            fi
        done
    fi
    
    # Update PIDS array (handle empty array case)
    if [ ${#new_pids[@]} -eq 0 ]; then
        PIDS=()
    else
        PIDS=("${new_pids[@]}")
    fi
    
    # Cleanup old cluster directories (older than 5 minutes)
    local now=$(date +%s)
    for cluster_dir in ~/.tiup/data/kb-gen-*; do
        if [ -d "$cluster_dir" ]; then
            local mtime=$(stat -f %m "$cluster_dir" 2>/dev/null || stat -c %Y "$cluster_dir" 2>/dev/null)
            if [ -n "$mtime" ]; then
                local age=$((now - mtime))
                # Cleanup clusters older than 5 minutes (likely completed)
                if [ $age -gt 300 ]; then
                    local tag=$(basename "$cluster_dir")
                    # Check if there's a running playground for this tag
                    if ! pgrep -f "tiup playground.*$tag" > /dev/null; then
                        rm -rf "$cluster_dir" 2>/dev/null && cleaned=$((cleaned + 1))
                    fi
                fi
            fi
        fi
    done
    
    if [ $cleaned -gt 0 ]; then
        echo "  Cleaned up $cleaned completed cluster(s)"
    fi
}

# Function to start a version generation
start_version() {
    local version_line="$1"
    local version=$(extract_version "$version_line")
    local version_group=$(extract_version_group "$version_line")
    
    CURRENT=$((CURRENT + 1))
    echo "=========================================="
    echo "[$CURRENT/$TOTAL_PENDING] Starting version: $version"
    echo "=========================================="

# Build command arguments
    CMD_ARGS=(
        "--version=$version"
    )
    
    # Add repository paths for components that need code definitions
    if [[ "$COMPONENTS" == *"tidb"* ]] && [ -n "$TIDB_REPO" ] && [ -d "$TIDB_REPO" ]; then
        CMD_ARGS+=("--tidb-repo=$TIDB_REPO")
    fi
    if [[ "$COMPONENTS" == *"pd"* ]] && [ -n "$PD_REPO" ] && [ -d "$PD_REPO" ]; then
        CMD_ARGS+=("--pd-repo=$PD_REPO")
    fi
    if [[ "$COMPONENTS" == *"tikv"* ]] && [ -n "$TIKV_REPO" ] && [ -d "$TIKV_REPO" ]; then
        CMD_ARGS+=("--tikv-repo=$TIKV_REPO")
    fi
    if [[ "$COMPONENTS" == *"tiflash"* ]] && [ -n "$TIFLASH_REPO" ] && [ -d "$TIFLASH_REPO" ]; then
        CMD_ARGS+=("--tiflash-repo=$TIFLASH_REPO")
    fi
    
    # Add components flag (remove keep-cluster to allow immediate cleanup after each version)
    CMD_ARGS+=("--components=$COMPONENTS")
    # Note: Removed --keep-cluster to allow immediate cleanup and prevent resource accumulation
    
    # Execute generation in background
    (
        VERSION_LOG="${LOGS_DIR}/knowledge_generation_${version}.log"
        echo "[$version] Starting generation at $(date)" >> "$VERSION_LOG"
        
        echo "  Running in background: go run cmd/kb_generator/main.go ${CMD_ARGS[*]}" | tee -a "$VERSION_LOG"
        # Use GOWORK=off to disable workspace mode and avoid replace directive issues
        if (cd "$PROJECT_ROOT" && GOWORK=off go run cmd/kb_generator/main.go "${CMD_ARGS[@]}" >> "$VERSION_LOG" 2>&1); then
            echo "[$version] ✓ Successfully generated at $(date)" >> "$VERSION_LOG"
            echo "$version:SUCCESS" >> "${TMP_DIR}/.version_results"
        else
            echo "[$version] ✗ Failed at $(date)" >> "$VERSION_LOG"
            echo "$version:FAILED" >> "${TMP_DIR}/.version_results"
        fi
    ) &
    
    PID=$!
    PIDS+=($PID)
    echo "  Started background process for $version (PID: $PID)"
    echo ""
}

# Process versions with concurrency limit
PENDING_INDEX=0
CLEANUP_COUNTER=0
while [ $PENDING_INDEX -lt ${#PENDING_VERSIONS[@]} ] || [ "$(count_running)" -gt 0 ]; do
    # Cleanup completed clusters periodically (every 10 iterations)
    CLEANUP_COUNTER=$((CLEANUP_COUNTER + 1))
    if [ $CLEANUP_COUNTER -ge 10 ]; then
        cleanup_completed_clusters
        CLEANUP_COUNTER=0
    fi
    
    # Start new versions if we have capacity
    RUNNING_COUNT=$(count_running)
    while [ "$RUNNING_COUNT" -lt $MAX_CONCURRENT ] && [ $PENDING_INDEX -lt ${#PENDING_VERSIONS[@]} ]; do
        # Cleanup before starting new version to free up resources
        cleanup_completed_clusters
        RUNNING_COUNT=$(count_running)
        
        # Check capacity again after cleanup
        if [ "$RUNNING_COUNT" -lt $MAX_CONCURRENT ] && [ $PENDING_INDEX -lt ${#PENDING_VERSIONS[@]} ]; then
            start_version "${PENDING_VERSIONS[$PENDING_INDEX]}"
            PENDING_INDEX=$((PENDING_INDEX + 1))
            # Add delay between starts to avoid resource conflicts
            sleep 5
            # Update running count after starting
            RUNNING_COUNT=$(count_running)
        fi
    done
    
    # Wait a bit before checking again
    sleep 2
done

# Final cleanup of completed clusters
cleanup_completed_clusters

# Wait for all background processes to complete
echo "=========================================="
echo "Waiting for all versions to complete..."
echo "Total background processes: ${#PIDS[@]}"
echo "=========================================="

# Monitor progress
while true; do
    RUNNING=$(count_running)
    
    if [ $RUNNING -eq 0 ]; then
        break
    fi
    
    echo "  Still running: $RUNNING / ${#PIDS[@]} versions"
    sleep 10
done

echo "All versions completed. Collecting results..."

# Collect results from result file
if [ -f "${TMP_DIR}/.version_results" ]; then
    while IFS=: read -r version result; do
        if [ "$result" = "SUCCESS" ]; then
            SUCCESS_COUNT=$((SUCCESS_COUNT + 1))
        else
            FAIL_COUNT=$((FAIL_COUNT + 1))
            FAILED_VERSIONS+=("$version")
        fi
    done < "${TMP_DIR}/.version_results"
    rm -f "${TMP_DIR}/.version_results"
fi

# Print summary
echo "=========================================="
echo "Generation Summary"
echo "=========================================="
echo "Total versions: $TOTAL_VERSIONS"
echo "Processed: $TOTAL_PENDING"
echo "Successful: $SUCCESS_COUNT"
echo "Skipped: $SKIP_COUNT"
echo "Failed: $FAIL_COUNT"
echo ""

if [ ${#FAILED_VERSIONS[@]} -gt 0 ]; then
    echo "Failed versions:"
    for ver in "${FAILED_VERSIONS[@]}"; do
        echo "  - $ver"
    done
    echo ""
fi

# Final cleanup of any remaining tiup playground clusters
# Note: Since --keep-cluster is removed, most clusters should be cleaned up automatically
# This is just a safety net for any remaining clusters
echo "=========================================="
echo "Final cleanup of any remaining clusters..."
echo "=========================================="

# Kill any remaining tiup playground processes
REMAINING=$(pgrep -f "tiup playground.*kb-gen-" | wc -l | tr -d ' ')
if [ "$REMAINING" -gt 0 ]; then
    echo "Found $REMAINING remaining playground processes, cleaning up..."
    pkill -TERM -f "tiup playground.*kb-gen-" 2>/dev/null || true
    sleep 2
    pkill -9 -f "tiup playground.*kb-gen-" 2>/dev/null || true
    
    # Clean up data directories
    find ~/.tiup/data -maxdepth 1 -type d -name "kb-gen-*" -exec rm -rf {} \; 2>/dev/null
    echo "✓ Cleanup completed"
else
    echo "✓ No remaining clusters to clean up"
fi

# Clean up temp file if used
if [ "$USE_TEMP_FILE" = true ] && [ -n "$VERSIONS_TEMP" ] && [ -f "$VERSIONS_TEMP" ]; then
    rm -f "$VERSIONS_TEMP"
fi

if [ ${#FAILED_VERSIONS[@]} -gt 0 ]; then
    echo ""
    echo "Some versions failed. Check individual log files: ${LOGS_DIR}/knowledge_generation_<version>.log"
    exit 1
fi

# Generate high-risk parameters default config
# Copy from pkg/analyzer/rules/high_risk_params/default.json to knowledge/high_risk_params/high_risk_params.json
echo ""
echo "Generating high-risk parameters default config..."
SOURCE_FILE="${PROJECT_ROOT}/pkg/analyzer/rules/high_risk_params/default.json"
OUTPUT_FILE="${PROJECT_ROOT}/knowledge/high_risk_params/high_risk_params.json"
OUTPUT_DIR=$(dirname "$OUTPUT_FILE")

if [ ! -f "$SOURCE_FILE" ]; then
    echo "⚠ Warning: Source file not found: $SOURCE_FILE"
    echo "  Skipping high-risk parameters default config generation"
else
    # Create output directory if it doesn't exist
    mkdir -p "$OUTPUT_DIR"
    
    # Copy file
    if cp "$SOURCE_FILE" "$OUTPUT_FILE"; then
        echo "✓ High-risk parameters default config generated successfully"
        echo "  Source: $SOURCE_FILE"
        echo "  Output: $OUTPUT_FILE"
        echo "  Note: Technical support can manually edit knowledge/high_risk_params/high_risk_params.json to add custom parameters"
    else
        echo "⚠ Warning: Failed to copy high-risk parameters default config"
    fi
fi

echo ""
echo "All knowledge bases generated successfully!"

exit 0
