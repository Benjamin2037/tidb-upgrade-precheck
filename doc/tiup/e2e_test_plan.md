# TiUP Cluster Integration End-to-End Test Plan

This document provides a step-by-step test plan for validating the integration of `tidb-upgrade-precheck` with TiUP `cluster upgrade` command in a local environment.

## Test Objectives

1. Verify `tidb-upgrade-precheck` binary is correctly located and executed by TiUP
2. Verify knowledge base is correctly located and loaded
3. Verify topology file is correctly generated and passed to precheck
4. Verify all command-line parameters are correctly passed
5. Verify report generation and display
6. Verify complete upgrade workflow with precheck integration

## Prerequisites

### 1. Environment Setup

```bash
# Ensure TiUP is installed
tiup --version

# Ensure tidb-upgrade-precheck is built
cd /path/to/tidb-upgrade-precheck
make build

# Verify binaries exist
ls -la bin/precheck
ls -la bin/kb_generator
```

### 2. Knowledge Base Preparation

```bash
# Generate knowledge base for test versions
cd /path/to/tidb-upgrade-precheck
bash scripts/generate_knowledge.sh --serial --start-from=v7.5.6 --stop-at=v7.5.6
bash scripts/generate_knowledge.sh --serial --start-from=v8.5.4 --stop-at=v8.5.4

# Verify knowledge base exists
ls -la knowledge/v7.5/v7.5.6/
ls -la knowledge/v8.5/v8.5.4/
```

### 3. TiUP Cluster Setup

```bash
# Build tiup-cluster with precheck integration
cd /path/to/tiup
make

# Verify tiup-cluster binary
ls -la bin/tiup-cluster
```

## Test Phases

### Phase 1: Environment Preparation

**Objective**: Set up test environment and verify prerequisites.

#### Step 1.1: Prepare Test Cluster

```bash
# Deploy a test cluster with v7.5.6
cd /path/to/tiup
tiup cluster deploy test-cluster v7.5.6 topology.yaml

# Start the cluster
tiup cluster start test-cluster

# Verify cluster is running
tiup cluster display test-cluster
```

**Verification Points**:
- [ ] Cluster deploys successfully
- [ ] Cluster starts successfully
- [ ] All components are running
- [ ] Cluster version is v7.5.6

#### Step 1.2: Prepare tidb-upgrade-precheck Binary

```bash
# Option 1: Copy binary to tiup-cluster directory
cp /path/to/tidb-upgrade-precheck/bin/precheck \
   /path/to/tiup/bin/tidb-upgrade-precheck

# Option 2: Use environment variable (for testing)
export TIDB_UPGRADE_PRECHECK_BIN=/path/to/tidb-upgrade-precheck/bin/precheck

# Verify binary is accessible
tiup cluster upgrade test-cluster v8.5.4 --precheck --help 2>&1 | head -5
```

**Verification Points**:
- [ ] Binary is accessible
- [ ] Binary executes correctly
- [ ] Help output is displayed

#### Step 1.3: Prepare Knowledge Base

```bash
# Option 1: Copy knowledge base to TiUP profile directory
cp -r /path/to/tidb-upgrade-precheck/knowledge \
   ~/.tiup/storage/cluster/knowledge

# Option 2: Use environment variable (for testing)
export TIDB_UPGRADE_PRECHECK_KB=/path/to/tidb-upgrade-precheck/knowledge

# Verify knowledge base structure
ls -la ~/.tiup/storage/cluster/knowledge/v7.5/v7.5.6/
ls -la ~/.tiup/storage/cluster/knowledge/v8.5/v8.5.4/
```

**Verification Points**:
- [ ] Knowledge base directory exists
- [ ] Source version knowledge base exists (v7.5.6)
- [ ] Target version knowledge base exists (v8.5.4)
- [ ] All component defaults.json files exist

---

### Phase 2: Basic Precheck Execution

**Objective**: Verify basic precheck execution through TiUP.

#### Step 2.1: Run Precheck Only (Default Format)

```bash
cd /path/to/tiup
tiup cluster upgrade test-cluster v8.5.4 --precheck
```

**Expected Behavior**:
- Precheck runs successfully
- Report is displayed to stdout
- Command exits without performing upgrade

**Verification Points**:
- [ ] Precheck executes without errors
- [ ] Report is displayed
- [ ] No upgrade is performed
- [ ] Exit code is 0

**Check Output For**:
- "Running parameter precheck..."
- "Loading topology from file..."
- "Collecting cluster configuration..."
- "Running compatibility checks..."
- "Report generated successfully: ..."

#### Step 2.2: Run Precheck with HTML Format

```bash
tiup cluster upgrade test-cluster v8.5.4 \
  --precheck \
  --precheck-output html
```

**Verification Points**:
- [ ] HTML report is generated
- [ ] Report file is in `~/.tiup/storage/cluster/upgrade_precheck/reports/`
- [ ] Report filename matches pattern `upgrade_precheck_report_*.html`
- [ ] Report content is valid HTML

**Check Report File**:
```bash
ls -la ~/.tiup/storage/cluster/upgrade_precheck/reports/*.html
cat ~/.tiup/storage/cluster/upgrade_precheck/reports/*.html | head -20
```

#### Step 2.3: Run Precheck with Custom Output File

```bash
tiup cluster upgrade test-cluster v8.5.4 \
  --precheck \
  --precheck-output markdown \
  --precheck-output-file ./test-report.md
```

**Verification Points**:
- [ ] Markdown report is generated
- [ ] Report is saved to specified file
- [ ] Report content is valid markdown

---

### Phase 3: Credentials and Authentication

**Objective**: Verify TiDB credentials handling.

#### Step 3.1: Use Default Credentials

```bash
tiup cluster upgrade test-cluster v8.5.4 --precheck
```

**Verification Points**:
- [ ] Uses default user "root"
- [ ] Connects to TiDB successfully
- [ ] Collects configuration successfully

#### Step 3.2: Use Custom User

```bash
tiup cluster upgrade test-cluster v8.5.4 \
  --precheck \
  --precheck-tidb-user admin
```

**Verification Points**:
- [ ] Uses specified user
- [ ] Connects successfully (if user exists)
- [ ] Or shows appropriate error message

#### Step 3.3: Use Password Prompt

```bash
tiup cluster upgrade test-cluster v8.5.4 \
  --precheck \
  --precheck-tidb-password-prompt
```

**Verification Points**:
- [ ] Prompts for password
- [ ] Accepts password input
- [ ] Uses password for connection

#### Step 3.4: Use Password File

```bash
echo "your-password" > /tmp/tidb-password.txt
tiup cluster upgrade test-cluster v8.5.4 \
  --precheck \
  --precheck-tidb-password-file /tmp/tidb-password.txt
```

**Verification Points**:
- [ ] Reads password from file
- [ ] Uses password for connection
- [ ] Cleans up sensitive data appropriately

---

### Phase 4: High-Risk Parameters Configuration

**Objective**: Verify high-risk parameters configuration handling.

#### Step 4.1: Create High-Risk Parameters Config

```bash
cat > ~/.tiup/high_risk_params.json <<EOF
{
  "tidb": [
    {
      "type": "system_variable",
      "name": "tidb_enable_async_commit",
      "severity": "high",
      "description": "Async commit may cause data inconsistency",
      "from_version": "v7.0.0",
      "to_version": "",
      "check_modified": true,
      "allowed_values": []
    }
  ]
}
EOF
```

#### Step 4.2: Run Precheck with High-Risk Config

```bash
tiup cluster upgrade test-cluster v8.5.4 \
  --precheck \
  --precheck-high-risk-params-config ~/.tiup/high_risk_params.json
```

**Verification Points**:
- [ ] High-risk params config is loaded
- [ ] High-risk params rule is executed
- [ ] Findings are included in report

#### Step 4.3: Use Default Config Location

```bash
# Config already in ~/.tiup/high_risk_params.json
tiup cluster upgrade test-cluster v8.5.4 --precheck
```

**Verification Points**:
- [ ] Automatically loads from default location
- [ ] High-risk params rule is executed

---

### Phase 5: Complete Upgrade Workflow

**Objective**: Verify complete upgrade workflow with precheck integration.

#### Step 5.1: Default Upgrade Workflow (With Precheck)

```bash
tiup cluster upgrade test-cluster v8.5.4
```

**Expected Behavior**:
1. Automatically runs precheck
2. Displays precheck report
3. Asks for user confirmation
4. Proceeds with upgrade if confirmed

**Verification Points**:
- [ ] Precheck runs automatically
- [ ] Report is displayed
- [ ] User confirmation prompt appears
- [ ] Upgrade proceeds after confirmation
- [ ] Cluster is upgraded to v8.5.4

**Check Cluster Version**:
```bash
tiup cluster display test-cluster | grep "Version"
```

#### Step 5.2: Upgrade Workflow with Precheck Failure

```bash
# Simulate precheck failure by removing knowledge base temporarily
mv ~/.tiup/storage/cluster/knowledge ~/.tiup/storage/cluster/knowledge.bak

# Try to upgrade
tiup cluster upgrade test-cluster v8.5.4
```

**Expected Behavior**:
- Precheck fails with clear error message
- User is still asked for confirmation
- Upgrade can proceed if user confirms

**Verification Points**:
- [ ] Precheck failure is handled gracefully
- [ ] Error message is clear
- [ ] User can still proceed with upgrade
- [ ] Warning is displayed

**Restore Knowledge Base**:
```bash
mv ~/.tiup/storage/cluster/knowledge.bak ~/.tiup/storage/cluster/knowledge
```

#### Step 5.3: Skip Precheck Workflow

```bash
# Reset cluster to v7.5.6 first
tiup cluster upgrade test-cluster v7.5.6 --without-precheck

# Upgrade without precheck
tiup cluster upgrade test-cluster v8.5.4 --without-precheck
```

**Verification Points**:
- [ ] Precheck is skipped
- [ ] Upgrade proceeds directly
- [ ] No precheck report is generated

---

### Phase 6: Report Generation and Formats

**Objective**: Verify all report formats are generated correctly.

#### Step 6.1: Test All Report Formats

```bash
for format in text markdown html json; do
  echo "Testing $format format..."
  tiup cluster upgrade test-cluster v8.5.4 \
    --precheck \
    --precheck-output $format \
    --precheck-output-file ./test-report.$format
  ls -la ./test-report.$format
done
```

**Verification Points**:
- [ ] Text format generates .txt file
- [ ] Markdown format generates .md file
- [ ] HTML format generates .html file
- [ ] JSON format generates .json file
- [ ] All formats contain complete information
- [ ] All formats are valid

#### Step 6.2: Verify Report Content

```bash
# Check text report
cat ./test-report.txt | grep -i "risk\|issue\|warning\|error"

# Check JSON report structure
cat ./test-report.json | jq '.summary'
cat ./test-report.json | jq '.check_results | length'

# Check HTML report
cat ./test-report.html | grep -i "precheck\|risk\|issue"
```

**Verification Points**:
- [ ] Reports contain risk findings
- [ ] Reports contain parameter changes
- [ ] Reports contain system variable changes
- [ ] JSON structure is valid
- [ ] HTML is properly formatted

---

### Phase 7: Error Handling

**Objective**: Verify error handling in various scenarios.

#### Step 7.1: Missing Binary

```bash
# Temporarily rename binary
mv /path/to/tiup/bin/tidb-upgrade-precheck \
   /path/to/tiup/bin/tidb-upgrade-precheck.bak

# Try to run precheck
tiup cluster upgrade test-cluster v8.5.4 --precheck
```

**Expected Behavior**:
- Falls back to PATH lookup
- Shows warning message
- Or shows clear error if not found

**Verification Points**:
- [ ] Error message is clear
- [ ] Suggests solution
- [ ] Does not crash

**Restore Binary**:
```bash
mv /path/to/tiup/bin/tidb-upgrade-precheck.bak \
   /path/to/tiup/bin/tidb-upgrade-precheck
```

#### Step 7.2: Missing Knowledge Base

```bash
# Temporarily move knowledge base
mv ~/.tiup/storage/cluster/knowledge \
   ~/.tiup/storage/cluster/knowledge.bak

# Try to run precheck
tiup cluster upgrade test-cluster v8.5.4 --precheck
```

**Expected Behavior**:
- Shows warning about missing knowledge base
- Attempts to continue
- Or shows clear error

**Verification Points**:
- [ ] Warning/error message is clear
- [ ] Suggests solution
- [ ] Handles gracefully

**Restore Knowledge Base**:
```bash
mv ~/.tiup/storage/cluster/knowledge.bak \
   ~/.tiup/storage/cluster/knowledge
```

#### Step 7.3: Invalid Cluster Name

```bash
tiup cluster upgrade non-existent-cluster v8.5.4 --precheck
```

**Expected Behavior**:
- Shows clear error about cluster not found
- Does not proceed

**Verification Points**:
- [ ] Error message is clear
- [ ] Does not crash
- [ ] Exit code is non-zero

#### Step 7.4: Invalid Target Version

```bash
tiup cluster upgrade test-cluster invalid-version --precheck
```

**Expected Behavior**:
- Shows error about invalid version
- Or shows error from precheck about missing knowledge base

**Verification Points**:
- [ ] Error message is clear
- [ ] Handles gracefully

---

### Phase 8: Integration Verification

**Objective**: Verify complete integration between TiUP and tidb-upgrade-precheck.

#### Step 8.1: Verify Topology File Generation

```bash
# Run precheck with debug output
tiup cluster upgrade test-cluster v8.5.4 --precheck 2>&1 | tee precheck.log

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

#### Step 8.2: Verify Command Arguments

```bash
# Enable debug logging (if available)
export TIUP_LOG_LEVEL=debug

# Run precheck and capture command
tiup cluster upgrade test-cluster v8.5.4 --precheck 2>&1 | grep -i "executing\|command"
```

**Verification Points**:
- [ ] Command includes --topology-file
- [ ] Command includes --target-version
- [ ] Command includes --format
- [ ] Command includes --output-dir
- [ ] Command includes --source-version (if available)
- [ ] Command includes credentials (if provided)

#### Step 8.3: Verify Working Directory

```bash
# Check if working directory is set correctly
tiup cluster upgrade test-cluster v8.5.4 --precheck 2>&1 | grep -i "knowledge\|working"

# Verify knowledge base is accessible from working directory
ls -la ~/.tiup/storage/cluster/knowledge/
```

**Verification Points**:
- [ ] Working directory is set correctly
- [ ] Knowledge base is accessible
- [ ] Precheck can find knowledge base

#### Step 8.4: Verify Report Path Extraction

```bash
# Run precheck and check output
tiup cluster upgrade test-cluster v8.5.4 --precheck 2>&1 | grep -i "report generated"

# Verify report file exists
ls -la ~/.tiup/storage/cluster/upgrade_precheck/reports/
```

**Verification Points**:
- [ ] Report path is extracted from output
- [ ] Report file exists at extracted path
- [ ] Report file is readable

---

### Phase 9: Performance and Resource Usage

**Objective**: Verify precheck performance and resource usage.

#### Step 9.1: Measure Precheck Execution Time

```bash
time tiup cluster upgrade test-cluster v8.5.4 --precheck
```

**Verification Points**:
- [ ] Precheck completes in reasonable time (< 2 minutes for small cluster)
- [ ] No excessive resource usage
- [ ] Memory usage is acceptable

#### Step 9.2: Verify Concurrent Execution

```bash
# Run multiple prechecks concurrently (if supported)
tiup cluster upgrade test-cluster v8.5.4 --precheck &
tiup cluster upgrade test-cluster v8.5.4 --precheck &
wait
```

**Verification Points**:
- [ ] No conflicts between concurrent executions
- [ ] Each execution uses separate temporary files
- [ ] Reports are generated correctly

---

### Phase 10: Cleanup and Restoration

**Objective**: Clean up test environment and restore cluster.

#### Step 10.1: Clean Up Test Files

```bash
# Remove test reports
rm -f ./test-report.*
rm -rf ~/.tiup/storage/cluster/upgrade_precheck/reports/*

# Remove temporary topology files (optional, auto-cleaned)
# rm -rf ~/.tiup/storage/cluster/upgrade_precheck/tmp/*
```

#### Step 10.2: Restore Cluster (If Needed)

```bash
# If cluster was upgraded, restore to original version
tiup cluster upgrade test-cluster v7.5.6 --without-precheck

# Or destroy and redeploy
tiup cluster destroy test-cluster
tiup cluster deploy test-cluster v7.5.6 topology.yaml
tiup cluster start test-cluster
```

---

## Test Checklist

### Prerequisites
- [ ] TiUP is installed and working
- [ ] tidb-upgrade-precheck is built
- [ ] Knowledge base is generated for test versions
- [ ] Test cluster is deployed and running

### Basic Functionality
- [ ] Precheck executes through TiUP
- [ ] Report is generated and displayed
- [ ] All report formats work correctly
- [ ] Credentials handling works correctly

### Integration
- [ ] Topology file is generated correctly
- [ ] Command arguments are passed correctly
- [ ] Knowledge base is located correctly
- [ ] Working directory is set correctly

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

### High-Risk Parameters
- [ ] High-risk params config is loaded
- [ ] High-risk params rule is executed
- [ ] Findings are included in report

---

## Troubleshooting

### Issue: Precheck binary not found

**Solution**:
```bash
# Check binary location
ls -la /path/to/tiup/bin/tidb-upgrade-precheck

# Or set environment variable
export TIDB_UPGRADE_PRECHECK_BIN=/path/to/tidb-upgrade-precheck/bin/precheck
```

### Issue: Knowledge base not found

**Solution**:
```bash
# Check knowledge base location
ls -la ~/.tiup/storage/cluster/knowledge/

# Or set environment variable
export TIDB_UPGRADE_PRECHECK_KB=/path/to/tidb-upgrade-precheck/knowledge
```

### Issue: Precheck fails to connect to TiDB

**Solution**:
```bash
# Check cluster status
tiup cluster display test-cluster

# Verify TiDB is running
tiup cluster exec test-cluster -N 127.0.0.1 -R tidb -- "ps aux | grep tidb-server"

# Check credentials
tiup cluster upgrade test-cluster v8.5.4 --precheck \
  --precheck-tidb-user root \
  --precheck-tidb-password-prompt
```

### Issue: Report not generated

**Solution**:
```bash
# Check output directory
ls -la ~/.tiup/storage/cluster/upgrade_precheck/reports/

# Check command output for errors
tiup cluster upgrade test-cluster v8.5.4 --precheck 2>&1 | tee precheck.log

# Verify precheck binary works standalone
/path/to/tidb-upgrade-precheck/bin/precheck --help
```

---

## Next Steps

After completing all test phases:

1. **Review Test Results**: Document any issues found
2. **Fix Issues**: Address any bugs or problems discovered
3. **Re-test**: Re-run failed tests after fixes
4. **Documentation**: Update documentation based on findings
5. **Performance Optimization**: Optimize if performance issues are found

---

## Test Execution Log Template

```markdown
## Test Execution Log

**Date**: YYYY-MM-DD
**Tester**: [Name]
**Environment**: [OS, TiUP version, tidb-upgrade-precheck version]

### Phase 1: Environment Preparation
- [ ] Step 1.1: Test cluster deployed
- [ ] Step 1.2: Binary prepared
- [ ] Step 1.3: Knowledge base prepared

### Phase 2: Basic Precheck Execution
- [ ] Step 2.1: Precheck only (default format)
- [ ] Step 2.2: Precheck with HTML format
- [ ] Step 2.3: Precheck with custom output file

### Phase 3: Credentials and Authentication
- [ ] Step 3.1: Default credentials
- [ ] Step 3.2: Custom user
- [ ] Step 3.3: Password prompt
- [ ] Step 3.4: Password file

### Phase 4: High-Risk Parameters Configuration
- [ ] Step 4.1: Config created
- [ ] Step 4.2: Precheck with config
- [ ] Step 4.3: Default config location

### Phase 5: Complete Upgrade Workflow
- [ ] Step 5.1: Default upgrade workflow
- [ ] Step 5.2: Upgrade with precheck failure
- [ ] Step 5.3: Skip precheck workflow

### Phase 6: Report Generation and Formats
- [ ] Step 6.1: All formats tested
- [ ] Step 6.2: Report content verified

### Phase 7: Error Handling
- [ ] Step 7.1: Missing binary
- [ ] Step 7.2: Missing knowledge base
- [ ] Step 7.3: Invalid cluster name
- [ ] Step 7.4: Invalid target version

### Phase 8: Integration Verification
- [ ] Step 8.1: Topology file generation
- [ ] Step 8.2: Command arguments
- [ ] Step 8.3: Working directory
- [ ] Step 8.4: Report path extraction

### Phase 9: Performance and Resource Usage
- [ ] Step 9.1: Execution time measured
- [ ] Step 9.2: Concurrent execution tested

### Phase 10: Cleanup and Restoration
- [ ] Step 10.1: Test files cleaned up
- [ ] Step 10.2: Cluster restored

### Issues Found
[List any issues found during testing]

### Notes
[Any additional notes or observations]
```

---

**Last Updated**: 2025

