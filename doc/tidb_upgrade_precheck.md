# TiDB Upgrade Precheck System Design Document

## 1. Introduction

This document provides an overview of the TiDB Upgrade Precheck system, which is designed to identify potential risks and issues before upgrading TiDB clusters. The system automatically collects configuration parameters, system variables, and upgrade logic across different versions to support pre-upgrade validation and risk assessment.

The current design and implementation primarily focuses on collecting configuration and system variables from different modules, with future extensions planned to include upgrade checks for other components such as TiKV, PD, and TiFlash.

## 2. System Overview

The TiDB Upgrade Precheck system consists of several modules that work together to collect and analyze data needed for upgrade validation:

1. **Parameter Collection Module** - Collects default values of TiDB system variables and configuration parameters across different LTS versions
2. **Upgrade Logic Collection Module** - Extracts mandatory system variable changes that occur during TiDB version upgrades
3. **(Future) Component Upgrade Check Modules** - Will include upgrade checks for TiKV, PD, TiFlash and other components
4. **(Future) Additional Upgrade Check Modules** - Will include other types of upgrade checks beyond configuration and system variables

## 3. Core Components

### 3.1 Parameter Collection
Focuses on collecting default values of TiDB system variables and configuration parameters across different versions. For details, see [Parameter Collection Design](./parameter_collection_design.md).

Key aspects:
- Version-specific collection tools to handle code structure differences
- Temporary environment setup for accurate data collection
- Result aggregation and historical tracking

### 3.2 Upgrade Logic Collection
Focuses on identifying and extracting mandatory system variable changes that occur during TiDB upgrades. For details, see [Upgrade Logic Collection Design](./upgrade_logic_collection_design.md).

Key aspects:
- AST parsing of upgrade.go to extract upgradeToVerXX functions
- Pattern matching for SQL statements that modify system variables
- Version tracking for upgrade changes

## 4. Data Collection Architecture

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

## 5. Collection Process

### 5.1 Version Management
- Identify LTS versions using Git tags
- Track processed versions to avoid redundant work
- Support full, incremental, and single version collection modes

### 5.2 Environment Isolation
- Create temporary clones of TiDB repository for each collection
- Checkout specific tags for version-specific collection
- Clean up temporary environments after collection

### 5.3 Tool Routing
- Select appropriate collection tools based on TiDB version
- Handle differences in code structure across versions
- Maintain version-specific tool files for accuracy

## 6. Output Formats

### 6.1 Version-Specific Parameters ([defaults.json](file:///Users/benjamin2037/Desktop/workspace/sourcecode/tidb-upgrade-precheck/pkg/scan/defaults.go#L79-L79))
Stored per version in:
```
knowledge/<version>/defaults.json
```

### 6.2 Parameter History Aggregation
Stored in:
```
knowledge/tidb/parameters-history.json
```

### 6.3 Upgrade Logic
Stored in:
```
knowledge/tidb/upgrade_logic.json
```

## 7. Integration Points

### 7.1 With TiDB Source Code
- Direct parsing of sysvar and config packages
- AST analysis of upgrade.go functions
- No modifications to TiDB source required

### 7.2 With kb-generator Tool
- Command-line interface for different collection modes
- Automated version detection and processing
- Result aggregation and output management

### 7.3 With Precheck System
- Provide data for upgrade risk assessment
- Enable validation of parameter compatibility
- Support version range-based analysis

## 8. Future Extensions

### 8.1 Additional Components
- Extend collection to TiKV, PD, TiFlash
- Add component-specific upgrade logic analysis
- Implement cross-component dependency checks

### 8.2 Enhanced Analysis
- Improve parameter type detection
- Add semantic analysis of code comments
- Enhance cross-reference capabilities

### 8.3 Additional Upgrade Checks
- Schema compatibility checks
- SQL behavior changes
- Feature deprecation tracking
- Performance impact analysis