# TiDB Upgrade Precheck Test Plan

This document provides a comprehensive test plan for the TiDB Upgrade Precheck system based on the current implementation.

## 1. Introduction

### 1.1 Purpose
This document provides a comprehensive test plan and detailed test cases for the TiDB Upgrade Precheck system to ensure system functionality, stability, and reliability.

### 1.2 Scope
The test scope covers all core functional modules of the TiDB Upgrade Precheck system, including:
- Knowledge Base Generation (via TiUP playground)
- Runtime Collection (from running clusters)
- Rule-Based Analysis
- Report Generation
- Command Line Interface
- High-Risk Parameters Management

## 2. Test Strategy

### 2.1 Test Types
- **Unit Testing**: Independent testing of each functional module
- **Integration Testing**: Testing module interactions and overall workflow
- **System Testing**: End-to-end functional testing with real clusters
- **Regression Testing**: Ensuring new features don't affect existing functionality

### 2.2 Test Environment
- **Operating Systems**: Linux/macOS/Windows
- **Go Version**: 1.21+
- **TiUP**: Installed and configured
- **TiDB Source Repository**: Contains all LTS version tags
- **Network Environment**: Access to GitHub and TiUP mirrors

## 3. Knowledge Base Generation Testing

### 3.1 Test Objective
Verify that the knowledge base generation script can correctly generate knowledge bases for all components and versions.

### 3.2 Test Cases

| Case ID | Test Case Name | Test Steps | Expected Results | Priority |
|---------|---------------|------------|------------------|----------|
| TC-KB-001 | Generate Full Knowledge Base | 1. Run `bash scripts/generate_knowledge.sh --serial`<br>2. Check output files | 1. Successfully generate knowledge/ directory<br>2. All LTS versions have knowledge bases<br>3. Each version contains defaults.json for all components | High |
| TC-KB-002 | Generate Single Version | 1. Run `bash scripts/generate_knowledge.sh --serial --start-from=v7.5.0 --stop-at=v7.5.0`<br>2. Check output files | 1. Successfully generate knowledge/v7.5/v7.5.0/<br>2. Contains tidb, pd, tikv, tiflash defaults.json | High |
| TC-KB-003 | Generate Version Range | 1. Run `bash scripts/generate_knowledge.sh --serial --start-from=v7.5.0 --stop-at=v8.1.0`<br>2. Check output files | 1. Successfully generate knowledge for all versions in range<br>2. All components have knowledge bases | High |
| TC-KB-004 | Skip Existing Versions | 1. Generate v7.5.0<br>2. Run again with `--skip-existing`<br>3. Check output | 1. First run generates files<br>2. Second run skips existing version<br>3. Displays skip message | High |
| TC-KB-005 | Force Regeneration | 1. Generate knowledge base<br>2. Run with `--force`<br>3. Check output | 1. Deletes and recreates knowledge/ directory<br>2. Cleans logs/ directory<br>3. Regenerates all knowledge bases | Medium |
| TC-KB-006 | Generate Upgrade Logic | 1. Run knowledge generation<br>2. Check upgrade_logic.json | 1. Generates knowledge/tidb/upgrade_logic.json<br>2. Contains forced parameter changes<br>3. Includes severity levels | High |
| TC-KB-007 | Bootstrap Version Extraction | 1. Generate knowledge for v6.5.0<br>2. Check tidb/defaults.json | 1. Contains bootstrap_version field<br>2. Value matches source code<br>3. Not omitted when value is 0 | High |
| TC-KB-008 | Component-Specific Collection | 1. Run with `--components=tidb,pd`<br>2. Check output | 1. Only generates tidb and pd knowledge bases<br>2. Skips tikv and tiflash | Medium |

### 3.3 Component-Specific Testing

#### 3.3.1 TiDB Collection
- Verify `SHOW CONFIG WHERE type='tidb'` collection
- Verify `SHOW GLOBAL VARIABLES` collection
- Verify bootstrap version extraction from source code
- Verify upgrade logic extraction from upgrade.go

#### 3.3.2 PD Collection
- Verify HTTP API `/pd/api/v1/config/default` collection
- Verify default configuration completeness

#### 3.3.3 TiKV Collection
- Verify `last_tikv.toml` file collection
- Verify `SHOW CONFIG WHERE type='tikv'` collection
- Verify merging with priority (runtime > user-set)

#### 3.3.4 TiFlash Collection
- Verify `tiflash.toml` file collection
- Verify `SHOW CONFIG WHERE type='tiflash'` collection
- Verify merging with priority (runtime > default)

## 4. Runtime Collection Testing

### 4.1 Test Objective
Verify that the runtime collector can correctly collect configuration from running TiDB clusters.

### 4.2 Test Cases

| Case ID | Test Case Name | Test Steps | Expected Results | Priority |
|---------|---------------|------------|------------------|----------|
| TC-RC-001 | Collect from TiDB | 1. Start TiDB cluster<br>2. Run collector<br>3. Check output | 1. Successfully collects config and system variables<br>2. Returns ComponentState | High |
| TC-RC-002 | Collect from PD | 1. Start PD cluster<br>2. Run collector<br>3. Check output | 1. Successfully collects config via HTTP API<br>2. Returns ComponentState | High |
| TC-RC-003 | Collect from TiKV | 1. Start TiKV cluster<br>2. Run collector<br>3. Check output | 1. Successfully collects config<br>2. Handles multiple TiKV nodes | High |
| TC-RC-004 | Collect from TiFlash | 1. Start TiFlash cluster<br>2. Run collector<br>3. Check output | 1. Successfully collects config<br>2. Handles multiple TiFlash nodes | Medium |
| TC-RC-005 | Optimized Collection | 1. Request only specific components<br>2. Run collector with requirements<br>3. Check output | 1. Only collects requested components<br>2. Skips unnecessary collection | Medium |
| TC-RC-006 | Connection Error Handling | 1. Try to collect from non-existent cluster<br>2. Check error handling | 1. Returns appropriate error<br>2. Error message is clear | High |

## 5. Analyzer Testing

### 5.1 Test Objective
Verify that the rule-based analyzer can correctly identify upgrade risks.

### 5.2 Test Cases

| Case ID | Test Case Name | Test Steps | Expected Results | Priority |
|---------|---------------|------------|------------------|----------|
| TC-AN-001 | User Modified Params Rule | 1. Create cluster with modified params<br>2. Run analyzer<br>3. Check results | 1. Detects modified parameters<br>2. Compares with source defaults | High |
| TC-AN-002 | Upgrade Differences Rule | 1. Set source and target versions<br>2. Run analyzer<br>3. Check results | 1. Detects forced changes in upgrade range<br>2. Filters by bootstrap version<br>3. Includes severity levels | High |
| TC-AN-003 | TiKV Consistency Rule | 1. Create cluster with inconsistent TiKV nodes<br>2. Run analyzer<br>3. Check results | 1. Detects parameter inconsistencies<br>2. Uses last_tikv.toml and SHOW CONFIG | High |
| TC-AN-004 | High Risk Params Rule | 1. Configure high-risk params<br>2. Run analyzer<br>3. Check results | 1. Validates high-risk parameters<br>2. Respects version range filtering | Medium |
| TC-AN-005 | Data Requirements Merging | 1. Create analyzer with multiple rules<br>2. Check data requirements | 1. Merges requirements from all rules<br>2. Optimizes data loading | Medium |
| TC-AN-006 | Version Range Filtering | 1. Test with different version ranges<br>2. Check upgrade logic filtering | 1. Correctly filters by bootstrap version range<br>2. Handles edge cases | High |

## 6. Report Generator Testing

### 6.1 Test Objective
Verify that reports can be generated in all supported formats.

### 6.2 Test Cases

| Case ID | Test Case Name | Test Steps | Expected Results | Priority |
|---------|---------------|------------|------------------|----------|
| TC-RP-001 | Text Format Report | 1. Run precheck<br>2. Generate text report<br>3. Check output | 1. Generates .txt file<br>2. Content is readable<br>3. Contains all sections | High |
| TC-RP-002 | Markdown Format Report | 1. Run precheck<br>2. Generate markdown report<br>3. Check output | 1. Generates .md file<br>2. Markdown syntax is correct<br>3. Can be rendered | High |
| TC-RP-003 | HTML Format Report | 1. Run precheck<br>2. Generate HTML report<br>3. Check output | 1. Generates .html file<br>2. Can be opened in browser<br>3. Styling is correct | High |
| TC-RP-004 | JSON Format Report | 1. Run precheck<br>2. Generate JSON report<br>3. Check output | 1. Generates .json file<br>2. JSON is valid<br>3. Contains complete data | High |
| TC-RP-005 | Report Sections | 1. Generate report with findings<br>2. Check sections | 1. Contains header<br>2. Contains summary<br>3. Contains detailed findings<br>4. Contains footer | High |

## 7. Command Line Interface Testing

### 7.1 Precheck Command Testing

| Case ID | Test Case Name | Test Steps | Expected Results | Priority |
|---------|---------------|------------|------------------|----------|
| TC-CLI-001 | Basic Precheck | 1. Run `./bin/precheck --target-version=v8.1.0 --tidb-addr=127.0.0.1:4000`<br>2. Check output | 1. Successfully connects to cluster<br>2. Performs analysis<br>3. Generates report | High |
| TC-CLI-002 | Topology File Input | 1. Run with `--topology-file`<br>2. Check output | 1. Parses topology file<br>2. Extracts connection info<br>3. Performs precheck | High |
| TC-CLI-003 | Format Selection | 1. Run with `--format=html`<br>2. Check output | 1. Generates HTML report<br>2. Saves to output directory | High |
| TC-CLI-004 | High Risk Params Config | 1. Run with `--high-risk-params-config`<br>2. Check output | 1. Loads custom config<br>2. Applies high-risk rule | Medium |
| TC-CLI-005 | Help Information | 1. Run `./bin/precheck --help` | 1. Displays help information<br>2. Lists all options | High |

### 7.2 High-Risk Parameters Management Testing

| Case ID | Test Case Name | Test Steps | Expected Results | Priority |
|---------|---------------|------------|------------------|----------|
| TC-HR-001 | Add High-Risk Param | 1. Run `./bin/precheck high-risk-params add`<br>2. Follow prompts | 1. Adds parameter to config<br>2. Saves to default location | High |
| TC-HR-002 | List High-Risk Params | 1. Run `./bin/precheck high-risk-params list` | 1. Displays all parameters<br>2. Shows component, type, name | High |
| TC-HR-003 | Remove High-Risk Param | 1. Run `./bin/precheck high-risk-params remove` | 1. Removes parameter<br>2. Updates config file | High |
| TC-HR-004 | View Config | 1. Run `./bin/precheck high-risk-params view` | 1. Displays JSON config<br>2. Format is correct | Medium |
| TC-HR-005 | Edit Config | 1. Run `./bin/precheck high-risk-params edit` | 1. Opens config in editor<br>2. Changes are saved | Medium |

### 7.3 Knowledge Base Generator Testing

| Case ID | Test Case Name | Test Steps | Expected Results | Priority |
|---------|---------------|------------|------------------|----------|
| TC-KBG-001 | Single Version Generation | 1. Run `./bin/kb_generator --version=v7.5.0`<br>2. Check output | 1. Generates knowledge base<br>2. All components included | High |
| TC-KBG-002 | Version Range Generation | 1. Run `./bin/kb_generator --from-tag=v7.5.0 --to-tag=v8.1.0`<br>2. Check output | 1. Generates for both versions<br>2. All components included | High |
| TC-KBG-003 | Component Selection | 1. Run with `--components=tidb,pd`<br>2. Check output | 1. Only generates tidb and pd<br>2. Skips other components | Medium |
| TC-KBG-004 | Repository Paths | 1. Run with custom repo paths<br>2. Check output | 1. Uses specified repositories<br>2. Extracts code definitions | Medium |

## 8. Unit Testing

### 8.1 Test Coverage

Unit tests cover the following packages:

#### 8.1.1 pkg/kbgenerator
- **loader_test.go**: Knowledge base loading and validation
- **Component collectors**: TiDB, PD, TiKV, TiFlash collection logic

#### 8.1.2 pkg/collector/runtime
- **collector_test.go**: Runtime collection functionality
- **topology_test.go**: Topology parsing and validation

#### 8.1.3 pkg/analyzer
- **analyzer_test.go**: Analyzer orchestration and data loading
- **rules/context_test.go**: Rule context and data access
- **rules/user_modified_params_rule_test.go**: User modified params rule

#### 8.1.4 pkg/reporter
- **reporter_test.go**: Report generation in all formats

#### 8.1.5 pkg/types
- **defaults_types_test.go**: Type definitions and serialization

### 8.2 Test Execution

```bash
# Run all unit tests
make test
# or
go test ./pkg/... ./cmd/... -v

# Run with coverage
go test ./pkg/... ./cmd/... -v -coverprofile=coverage.out
go tool cover -html=coverage.out -o coverage.html

# Run specific package tests
go test ./pkg/analyzer/... -v
go test ./pkg/collector/... -v
```

## 9. Integration Testing

### 9.1 Knowledge Base Generation Integration

**Test Flow**:
1. Run `bash scripts/generate_knowledge.sh --serial --start-from=v7.5.0 --stop-at=v7.5.0`
2. Verify knowledge base structure
3. Validate JSON format
4. Check bootstrap version extraction
5. Verify upgrade logic generation

**Verification Points**:
- Knowledge base directory structure is correct
- All component defaults.json files exist
- Bootstrap version is correctly extracted
- Upgrade logic contains forced changes

### 9.2 Precheck Integration

**Test Flow**:
1. Start TiUP playground cluster
2. Run precheck command
3. Verify collection
4. Verify analysis
5. Verify report generation

**Verification Points**:
- Successfully connects to cluster
- Collects all component configurations
- Analyzes risks correctly
- Generates report in requested format

### 9.3 End-to-End Testing

**Test Flow**:
1. Generate knowledge base for source and target versions
2. Start cluster with source version
3. Run precheck for target version
4. Verify findings match expected risks
5. Generate reports in all formats

**Verification Points**:
- Complete workflow executes successfully
- All rules are evaluated
- Reports contain accurate information
- No errors or crashes

## 10. Performance Testing

### 10.1 Test Cases

| Case ID | Test Case Name | Test Steps | Expected Results | Priority |
|---------|---------------|------------|------------------|----------|
| TC-PERF-001 | Full Knowledge Base Generation | 1. Run full generation<br>2. Measure time | 1. Completes within reasonable time<br>2. No memory leaks | Medium |
| TC-PERF-002 | Large Cluster Collection | 1. Test with 10+ TiKV nodes<br>2. Measure collection time | 1. Completes efficiently<br>2. Handles concurrency | Medium |
| TC-PERF-003 | Report Generation Performance | 1. Generate reports for large analysis results<br>2. Measure time | 1. Generates quickly<br>2. Memory usage is reasonable | Low |

## 11. Compatibility Testing

### 11.1 Test Cases

| Case ID | Test Case Name | Test Steps | Expected Results | Priority |
|---------|---------------|------------|------------------|----------|
| TC-COMP-001 | Go Version Compatibility | 1. Test with Go 1.21, 1.22, 1.23 | 1. Compiles successfully<br>2. Tests pass | High |
| TC-COMP-002 | OS Compatibility | 1. Test on Linux, macOS, Windows | 1. Works correctly<br>2. Path handling is correct | High |
| TC-COMP-003 | TiUP Version Compatibility | 1. Test with different TiUP versions | 1. Component installation works<br>2. Playground starts correctly | Medium |

## 12. Regression Testing

### 12.1 Test Cases

| Case ID | Test Case Name | Test Steps | Expected Results | Priority |
|---------|---------------|------------|------------------|----------|
| TC-REG-001 | Core Functionality Regression | 1. Execute all high-priority test cases | 1. All tests pass<br>2. No functionality degradation | High |
| TC-REG-002 | Knowledge Base Format Compatibility | 1. Test with existing knowledge bases | 1. Loads correctly<br>2. No format errors | High |

## 13. Test Execution Plan

### 13.1 Test Phases
1. **Unit Testing Phase**: Continuously executed during development
2. **Integration Testing Phase**: Executed after feature development completion
3. **System Testing Phase**: Executed before release
4. **Regression Testing Phase**: Executed after each code change

### 13.2 Test Tools
- Go built-in testing framework
- GitHub Actions CI/CD
- Manual testing verification
- TiUP playground for cluster testing

### 13.3 Test Data
- Real TiDB source repository with LTS version tags
- TiUP playground clusters for runtime testing
- Simulated error inputs and boundary conditions

## 14. Test Pass Criteria

### 14.1 Functional Test Pass Criteria
- All high-priority test cases must pass
- Medium-priority test cases pass rate no less than 95%
- Low-priority test cases pass rate no less than 90%

### 14.2 Performance Test Pass Criteria
- Full knowledge base generation completes within reasonable time
- Memory usage within reasonable range
- No memory leaks

### 14.3 Compatibility Test Pass Criteria
- Works normally on supported Go versions and operating systems
- Path handling correct, no platform-related issues

## 15. Risks and Mitigation Measures

### 15.1 Major Risks
1. **TiUP Playground Dependency**: Knowledge base generation requires TiUP playground
   - Mitigation: Ensure TiUP is installed and components are available

2. **Version Compatibility Risk**: Different TiDB versions have different code structures
   - Mitigation: Test with multiple LTS versions, handle version-specific logic

3. **Performance Risk**: Performance issues when processing large numbers of versions
   - Mitigation: Use serial mode for stability, optimize collection logic

### 15.2 Contingency Plan
1. If tests fail, immediately investigate and fix issues
2. For environment issues, prepare backup test environment
3. For performance issues, conduct performance analysis and optimization

## 16. Appendix

### 16.1 Glossary
- **LTS**: Long Term Support, long-term supported versions
- **Knowledge Base**: Parameter defaults and upgrade logic for different versions
- **Bootstrap Version**: Internal TiDB version number for upgrade logic filtering
- **Upgrade Logic**: Forced parameter changes during TiDB version upgrades

### 16.2 Reference Documents
- [System Design](../design.md) - Main design document
- [Knowledge Base Generation Guide](../knowledge_generation_guide.md) - User guide
- [Collector Design](../design/collector/README.md) - Collector implementation
- [Analyzer Design](../design/analyzer/README.md) - Analyzer implementation
- [Report Generator Design](../design/reporter/README.md) - Reporter implementation

---

**Last Updated**: 2025
