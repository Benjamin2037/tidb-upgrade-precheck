# TiUP Cluster Integration End-to-End Test Plan (Local Development)

This document provides a **step-by-step local test plan** for validating the integration of `tidb-upgrade-precheck` with TiUP `cluster upgrade` command using **local source code**.

## Overview

This test plan is designed for **local development environments** where:
- TiUP source code is in `/Users/benjamin2037/Desktop/workspace/sourcecode/tiup`
- tidb-upgrade-precheck source code is in `/Users/benjamin2037/Desktop/workspace/sourcecode/tidb-upgrade-precheck`
- All code is built locally from source

## Prerequisites

### 1. Directory Structure

```bash
# Verify directory structure
cd /Users/benjamin2037/Desktop/workspace/sourcecode
ls -d tiup tidb-upgrade-precheck
```

Expected output:
```
tiup
tidb-upgrade-precheck
```

### 2. Required Tools

```bash
# Check Go version (should be 1.21+)
go version

# Check TiUP (if installed)
tiup --version

# Check make
make --version
```

---

## Phase 1: Build Local Binaries

**Objective**: Build both TiUP and tidb-upgrade-precheck from local source code.

### Step 1.1: Build tidb-upgrade-precheck

```bash
cd /Users/benjamin2037/Desktop/workspace/sourcecode/tidb-upgrade-precheck

# Build binary
make build

# Verify binary exists
ls -lh bin/upgrade-precheck

# Test binary works
bin/upgrade-precheck precheck --help | head -10
```

**Verification Points**:
- [ ] Binary is built successfully
- [ ] Binary size is reasonable (~10-20MB)
- [ ] Help command works

### Step 1.2: Build TiUP (tiup-cluster component)

```bash
cd /Users/benjamin2037/Desktop/workspace/sourcecode/tiup

# Build TiUP
make

# Or build only cluster component
cd components/cluster
make

# Verify tiup-cluster binary
ls -lh bin/tiup-cluster
```

**Verification Points**:
- [ ] TiUP builds successfully
- [ ] tiup-cluster binary exists
- [ ] Binary is executable

### Step 1.3: Set Up Binary Paths

**Option A: Use Environment Variables (Recommended for Testing)**

```bash
# Set environment variables for this session
export TIDB_UPGRADE_PRECHECK_BIN=/Users/benjamin2037/Desktop/workspace/sourcecode/tidb-upgrade-precheck/bin/upgrade-precheck
export TIDB_UPGRADE_PRECHECK_KB=/Users/benjamin2037/Desktop/workspace/sourcecode/tidb-upgrade-precheck/knowledge

# Verify paths
echo "Binary: $TIDB_UPGRADE_PRECHECK_BIN"
echo "KB: $TIDB_UPGRADE_PRECHECK_KB"
ls -lh "$TIDB_UPGRADE_PRECHECK_BIN"
ls -d "$TIDB_UPGRADE_PRECHECK_KB"
```

**Option B: Copy Binaries to TiUP Directory**

```bash
# Copy binary to tiup-cluster directory
cp /Users/benjamin2037/Desktop/workspace/sourcecode/tidb-upgrade-precheck/bin/upgrade-precheck \
   /Users/benjamin2037/Desktop/workspace/sourcecode/tiup/components/cluster/bin/tidb-upgrade-precheck

# Copy knowledge base to TiUP profile
mkdir -p ~/.tiup/storage/cluster/knowledge
cp -r /Users/benjamin2037/Desktop/workspace/sourcecode/tidb-upgrade-precheck/knowledge/* \
      ~/.tiup/storage/cluster/knowledge/
```

**Verification Points**:
- [ ] Environment variables are set (Option A)
- [ ] OR binaries are copied to correct locations (Option B)
- [ ] Knowledge base is accessible

---

## Phase 2: Prepare Knowledge Base

**Objective**: Ensure knowledge base exists for test versions.

### Step 2.1: Check Existing Knowledge Base

```bash
cd /Users/benjamin2037/Desktop/workspace/sourcecode/tidb-upgrade-precheck

# Check for test versions
ls -d knowledge/v7.5/v7.5.6 knowledge/v8.5/v8.5.4 2>/dev/null || echo "Knowledge base missing for test versions"
```

### Step 2.2: Generate Knowledge Base (If Needed)

```bash
# Generate for specific versions
cd /Users/benjamin2037/Desktop/workspace/sourcecode/tidb-upgrade-precheck

# Generate v7.5.6
bash scripts/generate_knowledge.sh --serial --start-from=v7.5.6 --stop-at=v7.5.6

# Generate v8.5.4
bash scripts/generate_knowledge.sh --serial --start-from=v8.5.4 --stop-at=v8.5.4
```

**Verification Points**:
- [ ] Knowledge base exists for v7.5.6
- [ ] Knowledge base exists for v8.5.4
- [ ] All component defaults.json files exist

### Step 2.3: Verify Knowledge Base Structure

```bash
cd /Users/benjamin2037/Desktop/workspace/sourcecode/tidb-upgrade-precheck

# Check structure
for version in "v7.5/v7.5.6" "v8.5/v8.5.4"; do
  echo "Checking $version..."
  for comp in tidb pd tikv tiflash; do
    if [ -f "knowledge/$version/$comp/defaults.json" ]; then
      echo "  ✓ $comp/defaults.json"
    else
      echo "  ✗ $comp/defaults.json MISSING"
    fi
  done
done
```

**Verification Points**:
- [ ] All component files exist
- [ ] JSON files are valid

---

## Phase 3: Prepare Test Cluster

**Objective**: Set up a test cluster for upgrade testing.

### Step 3.1: Deploy Test Cluster with TiUP Cluster (Recommended)

**Note**: TiUP Playground does not support upgrade operations. For E2E upgrade testing, we must use `tiup cluster` to deploy a real cluster.

```bash
# Create topology file
cat > /tmp/test-topology.yaml << 'EOF'
global:
  user: "tidb"
  ssh_port: 22
  deploy_dir: "/tmp/tidb-deploy"
  data_dir: "/tmp/tidb-data"

pd_servers:
  - host: 127.0.0.1
    client_port: 2379
    peer_port: 2380

tidb_servers:
  - host: 127.0.0.1
    port: 4000
    status_port: 10080

tikv_servers:
  - host: 127.0.0.1
    port: 20160
    status_port: 20180
EOF

# Deploy cluster
cd /Users/benjamin2037/Desktop/workspace/sourcecode/tiup
tiup cluster deploy test-cluster v7.5.1 /tmp/test-topology.yaml

# Start cluster
tiup cluster start test-cluster

# Verify cluster
tiup cluster display test-cluster
```

**Verification Points**:
- [ ] Cluster deploys successfully
- [ ] Cluster starts successfully
- [ ] All components are running
- [ ] Cluster version is v7.5.1

### Step 3.2: Use Automated Test Script (Recommended)

For convenience, use the automated test script:

```bash
cd /Users/benjamin2037/Desktop/workspace/sourcecode/tidb-upgrade-precheck
./scripts/run_e2e_cluster_test.sh
```

This script will:
1. Check prerequisites
2. Deploy test cluster
3. Start cluster
4. Run precheck tests (all formats)
5. Test TiUP integration
6. Generate test report

**Verification Points**:
- [ ] Script executes successfully
- [ ] Cluster deploys and starts
- [ ] All precheck tests pass
- [ ] Test report is generated

---

## Phase 4: Test Precheck Integration

**Objective**: Test the integration between TiUP and tidb-upgrade-precheck.

### Step 4.1: Test Binary Location

```bash
# Set environment variable
export TIDB_UPGRADE_PRECHECK_BIN=/Users/benjamin2037/Desktop/workspace/sourcecode/tidb-upgrade-precheck/bin/upgrade-precheck

# Test if TiUP can find the binary
cd /Users/benjamin2037/Desktop/workspace/sourcecode/tiup

# Use local tiup-cluster binary
./components/cluster/bin/tiup-cluster upgrade test-cluster v8.5.4 --precheck 2>&1 | head -20
```

**Verification Points**:
- [ ] TiUP can locate the binary
- [ ] Binary executes successfully
- [ ] No "binary not found" errors

### Step 4.2: Test Knowledge Base Location

```bash
# Set knowledge base path
export TIDB_UPGRADE_PRECHECK_KB=/Users/benjamin2037/Desktop/workspace/sourcecode/tidb-upgrade-precheck/knowledge

# Test precheck with playground (if using playground)
cd /Users/benjamin2037/Desktop/workspace/sourcecode/tidb-upgrade-precheck

bin/upgrade-precheck precheck \
  --tidb-addr 127.0.0.1:4000 \
  --tidb-user root \
  --pd-addrs 127.0.0.1:2379 \
  --tikv-addrs 127.0.0.1:20160 \
  --source-version v7.5.1 \
  --target-version v8.5.4 \
  --format text \
  --output-dir /tmp/precheck_test
```

**Verification Points**:
- [ ] Knowledge base is found
- [ ] Precheck runs successfully
- [ ] Report is generated

### Step 4.3: Test TiUP Integration (Full Workflow)

```bash
# Set environment variables
export TIDB_UPGRADE_PRECHECK_BIN=/Users/benjamin2037/Desktop/workspace/sourcecode/tidb-upgrade-precheck/bin/upgrade-precheck
export TIDB_UPGRADE_PRECHECK_KB=/Users/benjamin2037/Desktop/workspace/sourcecode/tidb-upgrade-precheck/knowledge

# Test with TiUP (using playground cluster)
cd /Users/benjamin2037/Desktop/workspace/sourcecode/tiup

# If using deployed cluster
./components/cluster/bin/tiup-cluster upgrade test-cluster v8.5.4 --precheck

# Or test with playground (need to create a cluster metadata first)
# This requires additional setup
```

**Verification Points**:
- [ ] TiUP calls precheck successfully
- [ ] Topology file is generated correctly
- [ ] All parameters are passed correctly
- [ ] Report is generated and displayed

---

## Phase 5: Test All Report Formats

**Objective**: Verify all report formats work correctly.

### Step 5.1: Test Text Format

```bash
export TIDB_UPGRADE_PRECHECK_BIN=/Users/benjamin2037/Desktop/workspace/sourcecode/tidb-upgrade-precheck/bin/upgrade-precheck
export TIDB_UPGRADE_PRECHECK_KB=/Users/benjamin2037/Desktop/workspace/sourcecode/tidb-upgrade-precheck/knowledge

cd /Users/benjamin2037/Desktop/workspace/sourcecode/tiup

./components/cluster/bin/tiup-cluster upgrade test-cluster v8.5.4 \
  --precheck \
  --precheck-output text \
  --precheck-output-file /tmp/precheck_text.txt
```

**Verification Points**:
- [ ] Text report is generated
- [ ] Report file exists
- [ ] Report content is readable

### Step 5.2: Test Markdown Format

```bash
./components/cluster/bin/tiup-cluster upgrade test-cluster v8.5.4 \
  --precheck \
  --precheck-output markdown \
  --precheck-output-file /tmp/precheck_markdown.md
```

**Verification Points**:
- [ ] Markdown report is generated
- [ ] Report is valid markdown

### Step 5.3: Test HTML Format

```bash
./components/cluster/bin/tiup-cluster upgrade test-cluster v8.5.4 \
  --precheck \
  --precheck-output html \
  --precheck-output-file /tmp/precheck_html.html
```

**Verification Points**:
- [ ] HTML report is generated
- [ ] Report can be opened in browser
- [ ] Report is properly formatted

### Step 5.4: Test JSON Format

```bash
./components/cluster/bin/tiup-cluster upgrade test-cluster v8.5.4 \
  --precheck \
  --precheck-output json \
  --precheck-output-file /tmp/precheck_json.json
```

**Verification Points**:
- [ ] JSON report is generated
- [ ] JSON is valid (can be parsed)
- [ ] JSON contains expected fields

---

## Phase 6: Test High-Risk Parameters

**Objective**: Verify high-risk parameters configuration works.

### Step 6.1: Create High-Risk Parameters Config

```bash
cat > /tmp/high_risk_params.json << 'EOF'
{
  "tidb": {
    "config": {},
    "system_variables": {
      "tidb_enable_async_commit": {
        "severity": "error",
        "description": "Async commit may cause data inconsistency",
        "from_version": "v7.0.0",
        "to_version": "",
        "check_modified": true,
        "allowed_values": []
      }
    }
  },
  "pd": {
    "config": {}
  },
  "tikv": {
    "config": {}
  },
  "tiflash": {
    "config": {}
  }
}
EOF
```

### Step 6.2: Test with High-Risk Config

```bash
export TIDB_UPGRADE_PRECHECK_BIN=/Users/benjamin2037/Desktop/workspace/sourcecode/tidb-upgrade-precheck/bin/upgrade-precheck
export TIDB_UPGRADE_PRECHECK_KB=/Users/benjamin2037/Desktop/workspace/sourcecode/tidb-upgrade-precheck/knowledge

cd /Users/benjamin2037/Desktop/workspace/sourcecode/tiup

./components/cluster/bin/tiup-cluster upgrade test-cluster v8.5.4 \
  --precheck \
  --precheck-high-risk-params-config /tmp/high_risk_params.json
```

**Verification Points**:
- [ ] Config file is loaded
- [ ] High-risk params are checked
- [ ] Findings are included in report

---

## Phase 7: Test Complete Upgrade Workflow

**Objective**: Test the complete upgrade workflow with precheck.

### Step 7.1: Test Default Workflow (With Precheck)

```bash
export TIDB_UPGRADE_PRECHECK_BIN=/Users/benjamin2037/Desktop/workspace/sourcecode/tidb-upgrade-precheck/bin/upgrade-precheck
export TIDB_UPGRADE_PRECHECK_KB=/Users/benjamin2037/Desktop/workspace/sourcecode/tidb-upgrade-precheck/knowledge

cd /Users/benjamin2037/Desktop/workspace/sourcecode/tiup

# This will run precheck, show report, and ask for confirmation
./components/cluster/bin/tiup-cluster upgrade test-cluster v8.5.4
```

**Verification Points**:
- [ ] Precheck runs automatically
- [ ] Report is displayed
- [ ] User confirmation prompt appears
- [ ] Upgrade proceeds after confirmation (if confirmed)

### Step 7.2: Test Precheck-Only Mode

```bash
./components/cluster/bin/tiup-cluster upgrade test-cluster v8.5.4 --precheck
```

**Verification Points**:
- [ ] Precheck runs
- [ ] Report is displayed
- [ ] No upgrade is performed
- [ ] Command exits after precheck

### Step 7.3: Test Skip Precheck Mode

```bash
./components/cluster/bin/tiup-cluster upgrade test-cluster v8.5.4 --without-precheck
```

**Verification Points**:
- [ ] Precheck is skipped
- [ ] Upgrade proceeds directly
- [ ] No precheck report is generated

---

## Phase 8: Test Error Handling

**Objective**: Verify error handling in various scenarios.

### Step 8.1: Test Missing Binary

```bash
# Temporarily unset environment variable
unset TIDB_UPGRADE_PRECHECK_BIN

# Try to run precheck
cd /Users/benjamin2037/Desktop/workspace/sourcecode/tiup
./components/cluster/bin/tiup-cluster upgrade test-cluster v8.5.4 --precheck 2>&1
```

**Verification Points**:
- [ ] Error message is clear
- [ ] Suggests solution
- [ ] Does not crash

### Step 8.2: Test Missing Knowledge Base

```bash
# Temporarily move knowledge base
mv /Users/benjamin2037/Desktop/workspace/sourcecode/tidb-upgrade-precheck/knowledge \
   /Users/benjamin2037/Desktop/workspace/sourcecode/tidb-upgrade-precheck/knowledge.bak

# Try to run precheck
export TIDB_UPGRADE_PRECHECK_BIN=/Users/benjamin2037/Desktop/workspace/sourcecode/tidb-upgrade-precheck/bin/upgrade-precheck
./components/cluster/bin/tiup-cluster upgrade test-cluster v8.5.4 --precheck 2>&1

# Restore knowledge base
mv /Users/benjamin2037/Desktop/workspace/sourcecode/tidb-upgrade-precheck/knowledge.bak \
   /Users/benjamin2037/Desktop/workspace/sourcecode/tidb-upgrade-precheck/knowledge
```

**Verification Points**:
- [ ] Warning/error message is clear
- [ ] Suggests solution
- [ ] Handles gracefully

### Step 8.3: Test Invalid Cluster Name

```bash
./components/cluster/bin/tiup-cluster upgrade non-existent-cluster v8.5.4 --precheck 2>&1
```

**Verification Points**:
- [ ] Error message is clear
- [ ] Does not crash
- [ ] Exit code is non-zero

---

## Phase 9: Test Credentials Handling

**Objective**: Verify TiDB credentials handling.

### Step 9.1: Test Default Credentials

```bash
export TIDB_UPGRADE_PRECHECK_BIN=/Users/benjamin2037/Desktop/workspace/sourcecode/tidb-upgrade-precheck/bin/upgrade-precheck
export TIDB_UPGRADE_PRECHECK_KB=/Users/benjamin2037/Desktop/workspace/sourcecode/tidb-upgrade-precheck/knowledge

cd /Users/benjamin2037/Desktop/workspace/sourcecode/tiup

./components/cluster/bin/tiup-cluster upgrade test-cluster v8.5.4 \
  --precheck \
  --precheck-tidb-user root
```

**Verification Points**:
- [ ] Uses default user "root"
- [ ] Connects to TiDB successfully
- [ ] Collects configuration successfully

### Step 9.2: Test Password Prompt

```bash
./components/cluster/bin/tiup-cluster upgrade test-cluster v8.5.4 \
  --precheck \
  --precheck-tidb-password-prompt
```

**Verification Points**:
- [ ] Prompts for password
- [ ] Accepts password input
- [ ] Uses password for connection

### Step 9.3: Test Password File

```bash
echo "your-password" > /tmp/tidb-password.txt
chmod 600 /tmp/tidb-password.txt

./components/cluster/bin/tiup-cluster upgrade test-cluster v8.5.4 \
  --precheck \
  --precheck-tidb-password-file /tmp/tidb-password.txt

# Clean up
rm /tmp/tidb-password.txt
```

**Verification Points**:
- [ ] Reads password from file
- [ ] Uses password for connection
- [ ] Cleans up appropriately

---

## Phase 10: Integration Verification

**Objective**: Verify complete integration between TiUP and tidb-upgrade-precheck.

### Step 10.1: Verify Topology File Generation

```bash
# Run precheck with debug output
export TIDB_UPGRADE_PRECHECK_BIN=/Users/benjamin2037/Desktop/workspace/sourcecode/tidb-upgrade-precheck/bin/upgrade-precheck
export TIDB_UPGRADE_PRECHECK_KB=/Users/benjamin2037/Desktop/workspace/sourcecode/tidb-upgrade-precheck/knowledge

cd /Users/benjamin2037/Desktop/workspace/sourcecode/tiup

./components/cluster/bin/tiup-cluster upgrade test-cluster v8.5.4 --precheck 2>&1 | tee /tmp/precheck.log

# Check if topology file is created
ls -la ~/.tiup/storage/cluster/upgrade_precheck/tmp/topology-*.yaml

# Verify topology file content
cat ~/.tiup/storage/cluster/upgrade_precheck/tmp/topology-*.yaml | head -30
```

**Verification Points**:
- [ ] Topology file is created
- [ ] Topology file contains cluster information
- [ ] Topology file contains source version
- [ ] Topology file is valid YAML

### Step 10.2: Verify Command Arguments

```bash
# Check command output for arguments
./components/cluster/bin/tiup-cluster upgrade test-cluster v8.5.4 --precheck 2>&1 | grep -i "executing\|command"
```

**Verification Points**:
- [ ] Command includes --topology-file
- [ ] Command includes --target-version
- [ ] Command includes --format
- [ ] Command includes --output-dir
- [ ] Command includes --source-version (if available)
- [ ] Command includes credentials (if provided)

### Step 10.3: Verify Report Path Extraction

```bash
# Run precheck and check output
./components/cluster/bin/tiup-cluster upgrade test-cluster v8.5.4 --precheck 2>&1 | grep -i "report generated"

# Verify report file exists
ls -la ~/.tiup/storage/cluster/upgrade_precheck/reports/
```

**Verification Points**:
- [ ] Report path is extracted from output
- [ ] Report file exists at extracted path
- [ ] Report file is readable

---

## Quick Test Script

For convenience, here's a quick test script that sets up environment and runs basic tests:

```bash
#!/bin/bash
# Quick E2E test script for local development

set -e

TIUP_DIR="/Users/benjamin2037/Desktop/workspace/sourcecode/tiup"
PRECHECK_DIR="/Users/benjamin2037/Desktop/workspace/sourcecode/tidb-upgrade-precheck"

# Set environment variables
export TIDB_UPGRADE_PRECHECK_BIN="$PRECHECK_DIR/bin/upgrade-precheck"
export TIDB_UPGRADE_PRECHECK_KB="$PRECHECK_DIR/knowledge"

# Build binaries
echo "=== Building binaries ==="
cd "$PRECHECK_DIR" && make build
cd "$TIUP_DIR/components/cluster" && make

# Verify binaries
echo "=== Verifying binaries ==="
ls -lh "$TIDB_UPGRADE_PRECHECK_BIN"
ls -lh "$TIUP_DIR/components/cluster/bin/tiup-cluster"

# Test basic precheck
echo "=== Testing basic precheck ==="
"$TIDB_UPGRADE_PRECHECK_BIN" precheck --help | head -5

echo "=== Setup complete ==="
echo "Environment variables set:"
echo "  TIDB_UPGRADE_PRECHECK_BIN=$TIDB_UPGRADE_PRECHECK_BIN"
echo "  TIDB_UPGRADE_PRECHECK_KB=$TIDB_UPGRADE_PRECHECK_KB"
echo ""
echo "To test with TiUP, run:"
echo "  cd $TIUP_DIR"
echo "  ./components/cluster/bin/tiup-cluster upgrade <cluster> <version> --precheck"
```

---

## Test Checklist

### Prerequisites
- [ ] TiUP source code is available
- [ ] tidb-upgrade-precheck source code is available
- [ ] Go is installed (1.21+)
- [ ] Make is installed

### Build
- [ ] tidb-upgrade-precheck builds successfully
- [ ] TiUP (tiup-cluster) builds successfully
- [ ] Binaries are executable

### Knowledge Base
- [ ] Knowledge base exists for test versions
- [ ] Knowledge base structure is correct
- [ ] Knowledge base is accessible

### Integration
- [ ] TiUP can locate precheck binary
- [ ] TiUP can locate knowledge base
- [ ] Topology file is generated correctly
- [ ] Command arguments are passed correctly

### Functionality
- [ ] Precheck executes through TiUP
- [ ] Report is generated and displayed
- [ ] All report formats work correctly
- [ ] Credentials handling works correctly
- [ ] High-risk params config works correctly

### Workflow
- [ ] Default upgrade workflow works
- [ ] Precheck-only mode works
- [ ] Skip precheck mode works
- [ ] User confirmation works

### Error Handling
- [ ] Missing binary is handled gracefully
- [ ] Missing knowledge base is handled gracefully
- [ ] Invalid inputs show clear errors
- [ ] Precheck failures don't block upgrade (with warning)

---

## Troubleshooting

### Issue: Binary not found

**Solution**:
```bash
# Set environment variable
export TIDB_UPGRADE_PRECHECK_BIN=/Users/benjamin2037/Desktop/workspace/sourcecode/tidb-upgrade-precheck/bin/upgrade-precheck

# Or copy to TiUP directory
cp "$TIDB_UPGRADE_PRECHECK_BIN" /Users/benjamin2037/Desktop/workspace/sourcecode/tiup/components/cluster/bin/tidb-upgrade-precheck
```

### Issue: Knowledge base not found

**Solution**:
```bash
# Set environment variable
export TIDB_UPGRADE_PRECHECK_KB=/Users/benjamin2037/Desktop/workspace/sourcecode/tidb-upgrade-precheck/knowledge

# Or copy to TiUP profile
mkdir -p ~/.tiup/storage/cluster/knowledge
cp -r /Users/benjamin2037/Desktop/workspace/sourcecode/tidb-upgrade-precheck/knowledge/* \
      ~/.tiup/storage/cluster/knowledge/
```

### Issue: Precheck fails to connect to TiDB

**Solution**:
```bash
# Check cluster status
tiup cluster display test-cluster

# Verify TiDB is running
# For playground: check if port 4000 is listening
netstat -an | grep 4000

# Test connection manually
mysql -h 127.0.0.1 -P 4000 -u root
```

### Issue: Build errors

**Solution**:
```bash
# Clean and rebuild
cd /Users/benjamin2037/Desktop/workspace/sourcecode/tidb-upgrade-precheck
make clean && make build

cd /Users/benjamin2037/Desktop/workspace/sourcecode/tiup
make clean && make
```

---

## Notes

1. **Environment Variables**: Using environment variables (`TIDB_UPGRADE_PRECHECK_BIN`, `TIDB_UPGRADE_PRECHECK_KB`) is the easiest way for local testing.

2. **Binary Location**: TiUP looks for the binary in this order:
   - Environment variable `TIDB_UPGRADE_PRECHECK_BIN`
   - Same directory as `tiup-cluster` binary
   - PATH

3. **Knowledge Base Location**: TiUP looks for knowledge base in this order:
   - Environment variable `TIDB_UPGRADE_PRECHECK_KB`
   - TiUP profile: `~/.tiup/storage/cluster/knowledge/`
   - Same directory as binary: `{binary-dir}/knowledge/`

4. **Testing with Playground**: Using `tiup playground` is recommended for local testing as it's faster and doesn't require full cluster deployment.

5. **Report Files**: Reports are stored in `~/.tiup/storage/cluster/upgrade_precheck/reports/` and are not automatically cleaned up.

---

**Last Updated**: 2025-12-12

