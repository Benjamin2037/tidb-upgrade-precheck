# TiDB Upgrade Precheck Integration Test Plan

This document provides integration test cases for the TiDB Upgrade Precheck system, focusing on end-to-end workflows and component interactions.

## Test Objectives

Ensure all functional modules of `tidb-upgrade-precheck` work correctly and integrate properly with TiUP and real TiDB clusters.

## Test Phases

### Phase 1: Knowledge Base Generation Verification

**Objective**: Verify that the knowledge base generation script can correctly generate the complete knowledge base.

#### 1.1 Generate Full Knowledge Base

```bash
# In tidb-upgrade-precheck root directory
cd /path/to/tidb-upgrade-precheck

# Generate full knowledge base (serial mode)
bash scripts/generate_knowledge.sh --serial

# Or generate specific version range
bash scripts/generate_knowledge.sh --serial --start-from=v7.5.0 --stop-at=v8.1.0
```

**Verification Points**:
- [ ] Knowledge base directory `knowledge/` has been created
- [ ] Knowledge bases for all LTS versions have been generated (v6.5.x, v7.1.x, v7.5.x, v8.1.x, v8.5.x)
- [ ] Each version contains `defaults.json` for all components (tidb, pd, tikv, tiflash)
- [ ] TiDB's `upgrade_logic.json` has been generated in `knowledge/tidb/`
- [ ] Knowledge base file format is correct (JSON format validation)
- [ ] Bootstrap version is correctly extracted and stored in `tidb/defaults.json`

#### 1.2 Verify Knowledge Base Content

```bash
# Check knowledge base structure
tree knowledge/ -L 3

# Validate JSON format
find knowledge/ -name "*.json" -exec jq . {} \; > /dev/null

# Check key versions
ls -la knowledge/v7.5/v7.5.0/
ls -la knowledge/v8.1/v8.1.0/
```

**Verification Points**:
- [ ] JSON file format is correct
- [ ] Each component's `defaults.json` contains `config` and `system_variables` (for TiDB)
- [ ] `upgrade_logic.json` contains forced system variable changes with severity levels
- [ ] Bootstrap version values are correct and non-zero for applicable versions

#### 1.3 Knowledge Base Integrity Test

```bash
# Run knowledge base validation script
bash scripts/validate_knowledge_base.sh
```

**Verification Points**:
- [ ] Validation script passes
- [ ] All required files exist
- [ ] File content is non-empty
- [ ] Bootstrap versions are correctly extracted

---

### Phase 2: Unit Tests

**Objective**: Verify independent functionality of each module.

#### 2.1 KB Generator Module Tests

```bash
# Run KB Generator related tests
go test ./pkg/kbgenerator/... -v
```

**Test Coverage**:
- [ ] `pkg/kbgenerator/loader_test.go` - Knowledge base loading
- [ ] Component-specific collection logic (implicit through integration tests)

**Verification Points**:
- [ ] All tests pass
- [ ] Test coverage > 70%

#### 2.2 Collector Module Tests

```bash
# Run Collector related tests
go test ./pkg/collector/... -v
```

**Test Coverage**:
- [ ] `pkg/collector/runtime/collector_test.go` - Runtime collection
- [ ] `pkg/collector/runtime/topology_test.go` - Topology parsing

**Verification Points**:
- [ ] Topology parsing tests pass
- [ ] Configuration collection logic is tested
- [ ] Error handling is tested

#### 2.3 Analyzer Module Tests

```bash
# Run Analyzer related tests
go test ./pkg/analyzer/... -v
```

**Test Coverage**:
- [ ] `pkg/analyzer/analyzer_test.go` - Analyzer orchestration
- [ ] `pkg/analyzer/rules/context_test.go` - Rule context
- [ ] `pkg/analyzer/rules/user_modified_params_rule_test.go` - User modified params rule

**Verification Points**:
- [ ] Rule evaluation tests pass
- [ ] Analysis result generation tests pass
- [ ] Version comparison logic tests pass
- [ ] Data requirements merging tests pass

#### 2.4 Reporter Module Tests

```bash
# Run Reporter related tests
go test ./pkg/reporter/... -v
```

**Test Coverage**:
- [ ] `pkg/reporter/reporter_test.go` - Report generation

**Verification Points**:
- [ ] All format report generation tests pass (text, markdown, HTML, JSON)
- [ ] Report content format is correct

#### 2.5 Types Module Tests

```bash
# Run Types related tests
go test ./pkg/types/... -v
```

**Test Coverage**:
- [ ] `pkg/types/defaults_types_test.go` - Type serialization

**Verification Points**:
- [ ] Type conversion tests pass
- [ ] JSON marshalling/unmarshalling tests pass
- [ ] Bootstrap version serialization tests pass

#### 2.6 Run All Unit Tests

```bash
# Run all tests
make test
# or
go test ./pkg/... ./cmd/... -v -coverprofile=coverage.out
go tool cover -html=coverage.out -o coverage.html
```

---

### Phase 3: Runtime Collection Integration Tests

**Objective**: Verify runtime collection from real TiDB clusters.

#### 3.1 TiUP Playground Collection Test

```bash
# Start TiUP playground cluster
tiup playground v7.5.0 --tag test-collection

# Wait for cluster to be ready
sleep 30

# Run collection test (using kb_generator or direct collector)
./bin/kb_generator --version=v7.5.0 --components=tidb

# Stop playground
tiup clean test-collection
```

**Verification Points**:
- [ ] Successfully connects to TiDB cluster
- [ ] Collects TiDB configuration via `SHOW CONFIG`
- [ ] Collects system variables via `SHOW GLOBAL VARIABLES`
- [ ] Extracts bootstrap version from source code
- [ ] Generates correct knowledge base snapshot

#### 3.2 Multi-Component Collection Test

```bash
# Start full cluster
tiup playground v7.5.0 --tag test-full

# Collect all components
./bin/kb_generator --version=v7.5.0

# Verify all components
ls -la knowledge/v7.5/v7.5.0/
```

**Verification Points**:
- [ ] TiDB collection succeeds
- [ ] PD collection succeeds (via HTTP API)
- [ ] TiKV collection succeeds (from last_tikv.toml and SHOW CONFIG)
- [ ] TiFlash collection succeeds (from tiflash.toml and SHOW CONFIG)

#### 3.3 Collection Optimization Test

```bash
# Test optimized collection with data requirements
# (This would be tested through precheck command)
./bin/precheck --target-version=v8.1.0 --tidb-addr=127.0.0.1:4000 \
  --components=tidb
```

**Verification Points**:
- [ ] Only collects requested components
- [ ] Skips unnecessary data collection
- [ ] Collection time is reduced

---

### Phase 4: Precheck Command Integration Tests

**Objective**: Verify the complete precheck workflow.

#### 4.1 Basic Precheck Test

```bash
# Start source version cluster
tiup playground v7.5.0 --tag test-precheck

# Run precheck for target version
./bin/precheck \
  --target-version=v8.1.0 \
  --tidb-addr=127.0.0.1:4000 \
  --tidb-user=root \
  --tidb-password="" \
  --format=html \
  --output-dir=./reports
```

**Verification Points**:
- [ ] Successfully connects to cluster
- [ ] Collects runtime configuration
- [ ] Loads knowledge base for source and target versions
- [ ] Executes all rules
- [ ] Generates HTML report
- [ ] Report contains findings

#### 4.2 Topology File Precheck Test

```bash
# Create topology file
cat > /tmp/topology.yaml <<EOF
tidb_servers:
  - host: 127.0.0.1
    port: 4000
pd_servers:
  - host: 127.0.0.1
    port: 2379
tikv_servers:
  - host: 127.0.0.1
    port: 20160
EOF

# Run precheck with topology file
./bin/precheck \
  --target-version=v8.1.0 \
  --topology-file=/tmp/topology.yaml \
  --format=json
```

**Verification Points**:
- [ ] Parses topology file correctly
- [ ] Extracts connection information
- [ ] Performs precheck successfully
- [ ] Generates JSON report

#### 4.3 Rule Execution Test

```bash
# Run precheck and verify all rules are executed
./bin/precheck --target-version=v8.1.0 --tidb-addr=127.0.0.1:4000 --format=text
```

**Verification Points**:
- [ ] User Modified Params Rule executes
- [ ] Upgrade Differences Rule executes
- [ ] TiKV Consistency Rule executes (if TiKV nodes present)
- [ ] High Risk Params Rule executes (if config provided)
- [ ] All rule results are included in report

#### 4.4 Report Format Tests

```bash
# Test all report formats
for format in text markdown html json; do
  ./bin/precheck --target-version=v8.1.0 --tidb-addr=127.0.0.1:4000 \
    --format=$format --output-dir=./reports/$format
done
```

**Verification Points**:
- [ ] Text format generates .txt file
- [ ] Markdown format generates .md file
- [ ] HTML format generates .html file
- [ ] JSON format generates .json file
- [ ] All formats contain complete information

---

### Phase 5: High-Risk Parameters Management Tests

**Objective**: Verify high-risk parameters management functionality.

#### 5.1 Add High-Risk Parameter

```bash
# Interactive mode
./bin/precheck high-risk-params add

# Command-line mode
./bin/precheck high-risk-params add \
  --component=tidb \
  --type=system_variable \
  --name=tidb_enable_async_commit \
  --severity=high \
  --from-version=v7.0.0
```

**Verification Points**:
- [ ] Parameter is added to config file
- [ ] Config file is saved to default location
- [ ] Parameter appears in list command

#### 5.2 List High-Risk Parameters

```bash
# List all parameters
./bin/precheck high-risk-params list

# Filter by component
./bin/precheck high-risk-params list --component=tidb

# JSON output
./bin/precheck high-risk-params list --format=json
```

**Verification Points**:
- [ ] Displays all parameters
- [ ] Filtering works correctly
- [ ] JSON format is valid

#### 5.3 Use High-Risk Parameters in Precheck

```bash
# Run precheck with high-risk params config
./bin/precheck \
  --target-version=v8.1.0 \
  --tidb-addr=127.0.0.1:4000 \
  --high-risk-params-config=~/.tiup/storage/upgrade-precheck/config/high_risk_params.json
```

**Verification Points**:
- [ ] High-risk params rule is loaded
- [ ] Rule validates configured parameters
- [ ] Findings are included in report

---

### Phase 6: End-to-End Workflow Tests

**Objective**: Verify complete workflows from knowledge base generation to precheck execution.

#### 6.1 Complete Workflow Test

```bash
# Step 1: Generate knowledge base for source and target versions
bash scripts/generate_knowledge.sh --serial --start-from=v7.5.0 --stop-at=v7.5.0
bash scripts/generate_knowledge.sh --serial --start-from=v8.1.0 --stop-at=v8.1.0

# Step 2: Start source version cluster
tiup playground v7.5.0 --tag test-e2e

# Step 3: Run precheck
./bin/precheck \
  --target-version=v8.1.0 \
  --tidb-addr=127.0.0.1:4000 \
  --format=html \
  --output-dir=./reports

# Step 4: Verify report
ls -la ./reports/
cat ./reports/*.html | grep -i "risk\|issue\|warning"
```

**Verification Points**:
- [ ] Knowledge base generation succeeds
- [ ] Cluster starts successfully
- [ ] Precheck executes without errors
- [ ] Report is generated
- [ ] Report contains relevant findings

#### 6.2 Upgrade Logic Filtering Test

```bash
# Generate knowledge base with upgrade logic
bash scripts/generate_knowledge.sh --serial --start-from=v7.5.0 --stop-at=v8.1.0

# Verify upgrade logic contains changes in bootstrap version range
jq '.changes[] | select(.bootstrap_version > 109 and .bootstrap_version <= 218)' \
  knowledge/tidb/upgrade_logic.json
```

**Verification Points**:
- [ ] Upgrade logic contains changes for version range
- [ ] Bootstrap version filtering works correctly
- [ ] Severity levels are assigned

---

### Phase 7: Error Handling and Edge Cases

**Objective**: Verify error handling and edge case scenarios.

#### 7.1 Missing Knowledge Base Test

```bash
# Remove knowledge base
rm -rf knowledge/

# Try to run precheck
./bin/precheck --target-version=v8.1.0 --tidb-addr=127.0.0.1:4000
```

**Verification Points**:
- [ ] Error message is clear
- [ ] Suggests generating knowledge base
- [ ] Exit code is non-zero

#### 7.2 Invalid Version Test

```bash
# Try to generate knowledge base for non-existent version
./bin/kb_generator --version=v99.99.99
```

**Verification Points**:
- [ ] Error message is clear
- [ ] Exit code is non-zero

#### 7.3 Connection Failure Test

```bash
# Try to connect to non-existent cluster
./bin/precheck --target-version=v8.1.0 --tidb-addr=127.0.0.1:9999
```

**Verification Points**:
- [ ] Connection error is handled gracefully
- [ ] Error message is informative
- [ ] Exit code is non-zero

---

### Phase 8: Performance and Scalability Tests

**Objective**: Verify system performance with realistic workloads.

#### 8.1 Large Cluster Test

```bash
# Start cluster with multiple TiKV nodes
tiup playground v7.5.0 --tag test-large --tikv 3

# Run precheck
./bin/precheck --target-version=v8.1.0 --tidb-addr=127.0.0.1:4000
```

**Verification Points**:
- [ ] Handles multiple TiKV nodes
- [ ] Collection completes in reasonable time
- [ ] Memory usage is acceptable

#### 8.2 Full Knowledge Base Generation Test

```bash
# Generate knowledge base for all LTS versions
time bash scripts/generate_knowledge.sh --serial
```

**Verification Points**:
- [ ] Completes within reasonable time
- [ ] No memory leaks
- [ ] All versions are processed

---

## Test Execution Checklist

### Pre-Test Setup
- [ ] TiUP is installed and configured
- [ ] TiDB, PD, TiKV, TiFlash repositories are cloned
- [ ] Go environment is set up (Go 1.21+)
- [ ] Network access to GitHub and TiUP mirrors

### Test Execution
- [ ] Phase 1: Knowledge Base Generation - All tests pass
- [ ] Phase 2: Unit Tests - All tests pass, coverage > 70%
- [ ] Phase 3: Runtime Collection - All tests pass
- [ ] Phase 4: Precheck Command - All tests pass
- [ ] Phase 5: High-Risk Parameters - All tests pass
- [ ] Phase 6: End-to-End Workflow - All tests pass
- [ ] Phase 7: Error Handling - All tests pass
- [ ] Phase 8: Performance - All tests pass

### Post-Test Cleanup
- [ ] Clean up TiUP playground clusters
- [ ] Clean up test knowledge bases
- [ ] Clean up test reports
- [ ] Document any issues found

## Related Documents

- [Test Plan](./test_plan.md) - Detailed test plan and test cases
- [Knowledge Base Generation Guide](../knowledge_generation_guide.md) - Knowledge base generation guide
- [System Design](../design.md) - System architecture

---

**Last Updated**: 2025
