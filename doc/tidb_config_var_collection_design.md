# TiDB Configuration and System Variable Collection Design Document

## 1. Introduction

This document describes the overall design and implementation of the TiDB configuration and system variable collection system, which is part of the tidb-upgrade-precheck project. The system automatically collects TiDB configuration parameters and system variables across different versions, as well as tracks mandatory changes that occur during version upgrades, to support pre-upgrade validation and risk assessment.

## 2. Design Goals

1. **Comprehensive Coverage**: Collect all user-visible parameters, system variables and their history across versions
2. **Multi-version Compatibility**: Support collection from various TiDB versions including LTS releases
3. **Non-intrusive**: Collect data without modifying the TiDB source code
4. **Accuracy**: Ensure collected data precisely reflects actual parameter values and changes
5. **Extensibility**: Allow easy addition of new versions, components and collection methods
6. **Efficiency**: Minimize resource consumption during collection

## 3. Scope

The collection system covers:
1. **TiDB System Variables**: Default values across versions
2. **TiDB Configuration Parameters**: Default configuration values
3. **Upgrade Logic**: Mandatory changes during TiDB version upgrades
4. **Cross-version Analysis**: Tracking parameter evolution across releases

## 4. Components Overview

### 4.1 Parameter Collection
Focuses on collecting default values of TiDB system variables and configuration parameters across different versions. For details, see [Parameter Collection Design](./parameter_collection_design.md).

Key aspects:
- Version-specific collection tools to handle code structure differences
- Temporary environment setup for accurate data collection
- Result aggregation and historical tracking

### 4.2 Upgrade Logic Collection
Focuses on identifying and extracting mandatory system variable changes that occur during TiDB upgrades. For details, see [Upgrade Logic Collection Design](./upgrade_logic_collection_design.md).

Key aspects:
- AST parsing of upgrade.go to extract upgradeToVerXX functions
- Pattern matching for SQL statements that modify system variables
- Version tracking for upgrade changes

## 5. Data Collection Architecture

```
┌─────────────────┐    ┌──────────────────┐    ┌────────────────────┐
│   TiDB Source   │    │  Collection Core │    │  Output Processors │
│      Code       │    │                  │    │                    │
│                 │    │                  │    │                    │
│ ┌─────────────┐ │    │ ┌──────────────┐ │    │ ┌────────────────┐ │
│ │  sysvar/    │ │    │ │ Version      │ │    │ │  Parameter     │ │
│ │ config pkgs │ │◄───┤ │  Detection   │ │    │ │  History       │ │
│ └─────────────┘ │    │ └──────────────┘ │    │ │  Aggregator    │ │
│ ┌─────────────┐ │    │ ┌──────────────┐ │    │ └────────────────┘ │
│ │ upgrade.go  │ │    │ │ Tool         │ │    │ ┌────────────────┐ │
│ └─────────────┘ │    │ │ Selection    │ │    │ │  Upgrade       │ │
│                 │    │ └──────────────┘ │    │ │  Logic         │ │
│                 │    │ ┌──────────────┐ │    │ │  Generator     │ │
│                 │    │ │ Temp Env     │ │    │ └────────────────┘ │
│                 │    │ │  Setup       │ │    │                    │
└─────────────────┘    │ └──────────────┘ │    └────────────────────┘
                       └──────────────────┘              │
                                                         ▼
                                             ┌───────────────────────┐
                                             │      Knowledge        │
                                             │ ┌───────────────────┐ │
                                             │ │  <version>/       │ │
                                             │ │  defaults.json    │ │
                                             │ └───────────────────┘ │
                                             │ ┌───────────────────┐ │
                                             │ │  tidb/            │ │
                                             │ │  parameters-      │ │
                                             │ │  history.json     │ │
                                             │ └───────────────────┘ │
                                             │ ┌───────────────────┐ │
                                             │ │  tidb/            │ │
                                             │ │  upgrade_logic.   │ │
                                             │ │  json             │ │
                                             │ └───────────────────┘ │
                                             └───────────────────────┘
```

## 6. Collection Process

### 6.1 Version Management
- Identify LTS versions using Git tags
- Track processed versions to avoid redundant work
- Support full, incremental, and single version collection modes

### 6.2 Environment Isolation
- Create temporary clones of TiDB repository for each collection
- Checkout specific tags for version-specific collection
- Clean up temporary environments after collection

### 6.3 Tool Routing
- Select appropriate collection tools based on TiDB version
- Handle differences in code structure across versions
- Maintain version-specific tool files for accuracy

## 7. Output Formats

### 7.1 Version-Specific Parameters ([defaults.json](file:///Users/benjamin2037/Desktop/workspace/sourcecode/tidb-upgrade-precheck/pkg/scan/defaults.go#L79-L79))
Stored per version in:
```
knowledge/<version>/defaults.json
```

### 7.2 Parameter History Aggregation
Stored in:
```
knowledge/tidb/parameters-history.json
```

### 7.3 Upgrade Logic
Stored in:
```
knowledge/tidb/upgrade_logic.json
```

## 8. Integration Points

### 8.1 With TiDB Source Code
- Direct parsing of sysvar and config packages
- AST analysis of upgrade.go functions
- No modifications to TiDB source required

### 8.2 With kb-generator Tool
- Command-line interface for different collection modes
- Automated version detection and processing
- Result aggregation and output management

### 8.3 With Precheck System
- Provide data for upgrade risk assessment
- Enable validation of parameter compatibility
- Support version range-based analysis

## 9. Extensibility

### 9.1 New TiDB Versions
- Add version-specific tool files as needed
- Update version detection logic for new LTS versions
- Maintain backward compatibility

### 9.2 Additional Components
- Extend collection to TiKV, PD, TiFlash
- Add new output processors for different formats
- Support additional analysis types

### 9.3 Enhanced Analysis
- Improve parameter type detection
- Add semantic analysis of code comments
- Enhance cross-reference capabilities