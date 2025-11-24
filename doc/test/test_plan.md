# TiDB Upgrade Precheck Test Plan and Detailed Test Cases

## 1. Introduction

### 1.1 Purpose
This document provides a comprehensive test plan and detailed test cases for the TiDB Upgrade Precheck system to ensure system functionality, stability, and reliability.

### 1.2 Scope
The test scope covers all core functional modules of the TiDB Upgrade Precheck system, including:
- Parameter Collection Module
- Upgrade Logic Collection Module
- Version Management Module
- Knowledge Base Generation Module
- Command Line Interface Module

## 2. Test Strategy

### 2.1 Test Types
- Unit Testing: Independent testing of each functional module
- Integration Testing: Testing module interactions and overall workflow
- System Testing: End-to-end functional testing
- Regression Testing: Ensuring new features don't affect existing functionality

### 2.2 Test Environment
- Operating Systems: Linux/macOS/Windows
- Go Version: 1.18+
- TiDB Source Repository: Contains all LTS version tags
- Network Environment: Access to GitHub

## 3. Test Plan

### 3.1 Parameter Collection Module Testing

#### 3.1.1 Test Objective
Verify that the system can correctly collect parameter default values from different TiDB versions.

#### 3.1.2 Test Cases

| Case ID | Test Case Name | Test Steps | Expected Results | Priority |
|---------|---------------|------------|------------------|----------|
| TC-PC-001 | Collect Latest Version Parameters | 1. Run `go run cmd/kb-generator/main.go --tag v8.5.0`<br>2. Check output files | 1. Successfully generate knowledge/v8.5.0/defaults.json<br>2. File contains sysvars and config information | High |
| TC-PC-002 | Collect v7.5+ Version Parameters | 1. Run `go run cmd/kb-generator/main.go --tag v7.5.0`<br>2. Check output files | 1. Successfully generate knowledge/v7.5.0/defaults.json<br>2. Use correct tool file export_defaults_v75plus.go | High |
| TC-PC-003 | Collect v7.1 LTS Version Parameters | 1. Run `go run cmd/kb-generator/main.go --tag v7.1.0`<br>2. Check output files | 1. Successfully generate knowledge/v7.1.0/defaults.json<br>2. Use correct tool file export_defaults_v71.go | High |
| TC-PC-004 | Collect v6.x Version Parameters | 1. Run `go run cmd/kb-generator/main.go --tag v6.5.0`<br>2. Check output files | 1. Successfully generate knowledge/v6.5.0/defaults.json<br>2. Use correct tool file export_defaults_v6.go | High |
| TC-PC-005 | Collect Legacy Version Parameters | 1. Run `go run cmd/kb-generator/main.go --tag v5.4.0`<br>2. Check output files | 1. Successfully generate knowledge/v5.4.0/defaults.json<br>2. Use correct tool file export_defaults_legacy.go | Medium |
| TC-PC-006 | Version Not Exist Error Handling | 1. Run `go run cmd/kb-generator/main.go --tag v99.99.99` | 1. Display error message<br>2. Program exit code non-zero | High |

### 3.2 Upgrade Logic Collection Module Testing

#### 3.2.1 Test Objective
Verify that the system can correctly collect mandatory parameter changes during TiDB upgrades.

#### 3.2.2 Test Cases

| Case ID | Test Case Name | Test Steps | Expected Results | Priority |
|---------|---------------|------------|------------------|----------|
| TC-UL-001 | Collect Upgrade Logic | 1. Run `go run cmd/kb-generator/main.go --all`<br>2. Check output files | 1. Successfully generate knowledge/tidb/upgrade_logic.json<br>2. File contains upgradeToVerXX function information | High |
| TC-UL-002 | SQL Statement Recognition | 1. Check upgrade_logic.json content<br>2. Verify SET GLOBAL statement recognition | 1. Correctly identify SET GLOBAL statements<br>2. Extract parameter names and values | High |
| TC-UL-003 | SQL Statement Recognition | 1. Check upgrade_logic.json content<br>2. Verify INSERT statement recognition | 1. Correctly identify INSERT INTO mysql.global_variables statements | High |
| TC-UL-004 | SQL Statement Recognition | 1. Check upgrade_logic.json content<br>2. Verify UPDATE statement recognition | 1. Correctly identify UPDATE mysql.global_variables statements | High |

### 3.3 Version Management Module Testing

#### 3.3.1 Test Objective
Verify that the system can correctly manage processed versions to avoid duplicate work.

#### 3.3.2 Test Cases

| Case ID | Test Case Name | Test Steps | Expected Results | Priority |
|---------|---------------|------------|------------------|----------|
| TC-VM-001 | Version Recording | 1. Run `go run cmd/kb-generator/main.go --all`<br>2. Check generated_versions.json | 1. Generate knowledge/generated_versions.json<br>2. Contains all processed version information | High |
| TC-VM-002 | Version Skipping | 1. Run `go run cmd/kb-generator/main.go --all` again<br>2. Check output | 1. Skip already processed versions<br>2. Display skip information | High |

### 3.4 Knowledge Base Generation Module Testing

#### 3.4.1 Test Objective
Verify that the system can correctly generate aggregated parameter history.

#### 3.4.2 Test Cases

| Case ID | Test Case Name | Test Steps | Expected Results | Priority |
|---------|---------------|------------|------------------|----------|
| TC-KG-001 | Parameter History Aggregation | 1. Run `go run cmd/kb-generator/main.go --all`<br>2. Check parameters-history.json | 1. Generate knowledge/tidb/parameters-history.json<br>2. Contains parameter history for all versions | High |
| TC-KG-002 | Parameter History Format | 1. Check parameters-history.json format | 1. Contains component field<br>2. Contains parameters array<br>3. Each parameter contains history array | High |

### 3.5 Command Line Interface Testing

#### 3.5.1 Test Objective
Verify the correctness and robustness of the command line interface.

#### 3.5.2 Test Cases

| Case ID | Test Case Name | Test Steps | Expected Results | Priority |
|---------|---------------|------------|------------------|----------|
| TC-CLI-001 | Full Collection | 1. Run `go run cmd/kb-generator/main.go --all` | 1. Successfully process all LTS versions<br>2. Generate all output files | High |
| TC-CLI-002 | Single Version Collection | 1. Run `go run cmd/kb-generator/main.go --tag v7.1.0` | 1. Successfully process specified version<br>2. Generate corresponding output files | High |
| TC-CLI-003 | Range Collection | 1. Run `go run cmd/kb-generator/main.go --from-tag v7.1.0 --to-tag v8.1.0` | 1. Successfully process specified version range<br>2. Generate corresponding output files | Medium |
| TC-CLI-004 | Help Information | 1. Run `go run cmd/kb-generator/main.go --help` | 1. Display help information<br>2. List all parameter options | High |
| TC-CLI-005 | Invalid Parameters | 1. Run `go run cmd/kb-generator/main.go --invalid-param` | 1. Display error message<br>2. Program exit code non-zero | High |

## 4. Performance Testing

### 4.1 Test Objective
Verify system performance when processing large numbers of versions.

### 4.2 Test Cases

| Case ID | Test Case Name | Test Steps | Expected Results | Priority |
|---------|---------------|------------|------------------|----------|
| TC-PERF-001 | Full Collection Time | 1. Run `go run cmd/kb-generator/main.go --all`<br>2. Record execution time | 1. Complete within reasonable time (depends on network and machine performance)<br>2. No memory overflow | Medium |

## 5. Compatibility Testing

### 5.1 Test Objective
Verify system compatibility across different environments.

### 5.2 Test Cases

| Case ID | Test Case Name | Test Steps | Expected Results | Priority |
|---------|---------------|------------|------------------|----------|
| TC-COMP-001 | Different Go Versions | 1. Run tool in Go 1.18, 1.19, 1.20 environments | 1. Work normally<br>2. No compilation errors | Medium |
| TC-COMP-002 | Different Operating Systems | 1. Run tool in Linux, macOS, Windows environments | 1. Work normally<br>2. Path handling correct | Medium |

## 6. Regression Testing

### 6.1 Test Objective
Ensure new features don't affect existing functionality.

### 6.2 Test Cases

| Case ID | Test Case Name | Test Steps | Expected Results | Priority |
|---------|---------------|------------|------------------|----------|
| TC-REG-001 | Core Functionality Regression | 1. Execute all high-priority test cases | 1. All tests pass<br>2. No functionality degradation | High |

## 7. Test Execution Plan

### 7.1 Test Phases
1. **Unit Testing Phase**: Continuously executed during development
2. **Integration Testing Phase**: Executed after feature development completion
3. **System Testing Phase**: Executed before release
4. **Regression Testing Phase**: Executed after each code change

### 7.2 Test Tools
- Go built-in testing framework
- GitHub Actions CI/CD
- Manual testing verification

### 7.3 Test Data
- Real TiDB source repository
- Contains all LTS version tags
- Simulated error inputs and boundary conditions

## 8. Test Pass Criteria

### 8.1 Functional Test Pass Criteria
- All high-priority test cases must pass
- Medium-priority test cases pass rate no less than 95%
- Low-priority test cases pass rate no less than 90%

### 8.2 Performance Test Pass Criteria
- Full collection completes within reasonable time (specific time determined by hardware environment)
- Memory usage within reasonable range
- No memory leaks

### 8.3 Compatibility Test Pass Criteria
- Work normally on supported Go versions and operating systems
- Path handling correct, no platform-related issues

## 9. Risks and Mitigation Measures

### 9.1 Major Risks
1. **Network Dependency Risk**: Tool needs to access GitHub to get TiDB source code
   - Mitigation: Ensure stable network in test environment, consider using local mirror
   
2. **Version Compatibility Risk**: Different TiDB versions have significantly different code structures
   - Mitigation: Maintain multiple version-specific tool files, ensure correct routing

3. **Performance Risk**: Performance issues may occur when processing large numbers of versions
   - Mitigation: Optimize code, add concurrent processing capabilities

### 9.2 Contingency Plan
1. If tests fail, immediately rollback related code changes
2. For environment issues, prepare backup test environment
3. For performance issues, conduct performance analysis and optimization

## 10. Appendix

### 10.1 Glossary
- **LTS**: Long Term Support, long-term supported versions
- **Parameters**: TiDB system variables and configuration items
- **Upgrade Logic**: System variables forcibly modified during TiDB version upgrades

### 10.2 Reference Documents
- [TiDB Upgrade Precheck Design](../tidb_upgrade_precheck.md)
- [Parameter Collection Design](../parameter_collection_design.md)
- [LTS Version Default Collection Design](../parameter_collection/LTS_version_default_design.md)
- [Upgrade Logic Collection Design](../parameter_collection/upgrade_logic_collection_design.md)