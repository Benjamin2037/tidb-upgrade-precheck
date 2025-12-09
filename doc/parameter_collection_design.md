# TiDB Upgrade Logic Collection Detailed Design Document

## 1. Introduction

This document describes the design and implementation of the TiDB upgrade logic collection system, which is part of the tidb-upgrade-precheck project. The system automatically extracts and analyzes mandatory system variable changes that occur during TiDB version upgrades to support pre-upgrade validation and risk assessment.

## 2. Purpose

The upgrade logic collection component is designed to automatically extract and analyze mandatory system variable changes that occur during TiDB version upgrades. These changes are typically found in the `upgradeToVerXX` functions within the [pkg/session/upgrade.go](file:///Users/benjamin2037/Desktop/workspace/sourcecode/tidb/pkg/session/upgrade.go) file of the TiDB source code.

## 3. Data Collection Scope

The collector focuses on extracting SQL statements that modify system variables during upgrades, specifically:
- `SET GLOBAL variable_name = value` statements
- `INSERT INTO mysql.global_variables` statements
- `UPDATE mysql.global_variables` statements
- `DELETE FROM mysql.global_variables` statements

These statements are typically found in functions named `upgradeToVerXX` where XX represents the bootstrap version number.

## 4. Technical Implementation

### 4.1 Collection Process

1. **Source Code Parsing**: The tool uses Go's AST (Abstract Syntax Tree) parser to analyze the [pkg/session/upgrade.go](file:///Users/benjamin2037/Desktop/workspace/sourcecode/tidb/pkg/session/upgrade.go) file
2. **Function Identification**: It identifies all functions matching the pattern `upgradeToVerXX` using regex
3. **Statement Extraction**: For each identified function, it traverses the AST to find SQL string literals
4. **Pattern Matching**: It applies regex patterns to identify statements that modify system variables
5. **Data Structuring**: Collected data is structured into JSON format for further processing

### 4.2 Data Structure

The output follows this JSON structure:

```json
[
  {
    "version": 71,
    "function": "upgradeToVer71",
    "changes": [
      {
        "type": "SQL",
        "sql": "\"UPDATE mysql.global_variables SET VARIABLE_VALUE='OFF' WHERE VARIABLE_NAME = 'tidb_multi_statement_mode' AND VARIABLE_VALUE = 'WARN'\"",
        "location": "../tidb/pkg/session/upgrade.go:1302:17"
      }
    ]
  }
]
```

Each entry contains:
- [version](file:///Users/benjamin2037/Desktop/workspace/sourcecode/tidb/pkg/session/upgrade.go#L481-L481): The bootstrap version number (XX from upgradeToVerXX)
- [function](file:///Users/benjamin2037/Desktop/workspace/sourcecode/tidb/pkg/expression/generator/time_vec.go#L905-L908): The full function name
- `changes`: Array of all system variable changes in that function

For each change:
- `type`: Type of change (currently only "SQL")
- [sql](file:///Users/benjamin2037/Desktop/workspace/sourcecode/tidb/pkg/session/nontransactional.go#L61-L61): The actual SQL statement
- [location](file:///Users/benjamin2037/Desktop/workspace/sourcecode/tidb/br/pkg/kms/gcp.go#L28-L28): File path and line number where the statement is found

## 5. Output Location

The collected upgrade logic is stored in:
```
knowledge/tidb/upgrade_logic.json
```

This location ensures it's separate from version-specific parameter data and clearly indicates it contains global upgrade information.

## 6. Usage

### 6.1 Direct Execution
```bash
go run tools/upgrade_logic_collector.go ../tidb > knowledge/tidb/upgrade_logic.json
```

### 6.2 Integration with kb-generator
The collection is automatically triggered when using the kb-generator tool:
```bash
go run cmd/kb-generator/main.go --all
```

## 7. Versioning Strategy

The tool only needs to be run once using the latest TiDB source code, as all historical upgrade functions are preserved in the codebase. This approach ensures:
1. All historical upgrade changes are captured
2. No need to check out specific versions for upgrade logic collection
3. Consistent data structure across runs
4. Simplified maintenance

## 8. Future Enhancements

1. **Enhanced Change Classification**: Improve categorization of changes beyond just SQL statements
2. **Parameter Name Extraction**: Automatically extract parameter names from SQL statements
3. **Comment Analysis**: Include code comments to provide context for changes
4. **Cross-reference with Parameter History**: Link upgrade changes with parameter history data
5. **Performance Optimization**: Improve parsing performance for large codebases

## 9. Integration with Precheck System

The collected upgrade logic will be used by the precheck system to:
1. Identify mandatory changes that will occur during an upgrade
2. Validate that these changes won't conflict with current settings
3. Provide detailed reports on what will change during the upgrade
4. Enable version range-based filtering for targeted checks

## 10. Maintenance Considerations

1. **Code Structure Dependencies**: The tool depends on the structure of [pkg/session/upgrade.go](file:///Users/benjamin2037/Desktop/workspace/sourcecode/tidb/pkg/session/upgrade.go)
2. **Function Naming Patterns**: Assumes consistent naming of `upgradeToVerXX` functions
3. **SQL Pattern Matching**: Regex patterns may need updates as SQL statements evolve
4. **Error Handling**: Robust error handling for file access and parsing issues

# TiDB Parameter Collection Design

## Overview

The TiDB upgrade precheck system consists of two main components for parameter collection:

1. **Knowledge Base Generation**: Collects parameter defaults and upgrade logic from TiDB source code to build a knowledge base
2. **Runtime Collection**: Collects current configuration from running clusters for risk analysis

These two components serve different purposes and operate in different contexts.

## Knowledge Base Generation

Knowledge base generation is an offline process that analyzes TiDB source code to extract parameter defaults and upgrade logic across different versions. This information is used to build a knowledge base that can be used for upgrade compatibility checking.

### Purpose

- Build a comprehensive database of parameter defaults across TiDB versions
- Track changes in system behavior across versions
- Identify forced parameter changes during upgrades
- Provide baseline information for risk assessment

### Data Sources

- TiDB source code repository
- Git tags representing released versions
- Configuration and system variable definitions in source code
- Bootstrap/upgrade logic in `session/bootstrap.go`

### Output

- `knowledge/<version>/defaults.json`: Parameter defaults for each version
- `knowledge/upgrade_logic.json`: Forced parameter changes during upgrades (part of knowledge base)
- `knowledge/parameters-history.json`: Aggregated parameter history across versions

## Runtime Collection

Runtime collection is an online process that gathers current configuration from running TiDB clusters. This information is compared against the knowledge base to identify potential upgrade risks.

### Purpose

- Capture current cluster configuration
- Identify user-modified parameters
- Compare current state against version-specific defaults
- Enable real-time risk assessment

### Data Sources

- Running TiDB cluster
- HTTP APIs of TiDB, TiKV, and PD components
- MySQL protocol for system variable access

### Output

- Cluster configuration snapshot
- Current parameter values
- Component version information

## Comparison of Approaches

| Aspect | Knowledge Base Generation | Runtime Collection |
|--------|---------------------------|-------------------|
| **Timing** | Offline, periodic | Online, on-demand |
| **Data Source** | TiDB source code | Running cluster |
| **Purpose** | Build reference database | Risk assessment |
| **Frequency** | When new versions are released | Before each upgrade |
| **Scope** | All versions' defaults | Current cluster state |

## Implementation Details

### Knowledge Base Collection

Knowledge base collection is implemented in the `pkg/kbgenerator` package and uses two methods:

1. **Source Code Parsing**: Directly parses Go source files to extract parameter defaults
2. **Binary Execution**: Runs tools against TiDB source to extract runtime defaults

#### Parameter Defaults Collection

The parameter defaults collection extracts default values for:
- Configuration parameters defined in the `config.Config` struct
- System variables defined in the `sessionctx/variable` package

#### Upgrade Logic Collection

The upgrade logic collection is a critical part of the knowledge base. It parses `session/bootstrap.go` to identify:
- Forced parameter changes that occur during the upgrade process
- Version-specific upgrade functions (`upgradeToVerXX`)
- System variable modifications using `setGlobalSysVar`, `SetGlobalSysVar`, etc.
- SQL statements that modify global variables

This information is stored in `knowledge/upgrade_logic.json` and is used to identify P0 risks during upgrade precheck - parameters that will be forcibly changed regardless of user settings.

### Runtime Collection

Runtime collection is implemented in the `pkg/runtime` package and uses HTTP APIs and MySQL protocol to collect current configuration:

1. **TiDB Collector**: Collects configuration via HTTP API and system variables via MySQL protocol
2. **TiKV Collector**: Collects configuration via HTTP API
3. **PD Collector**: Collects configuration via HTTP API

## Integration

The knowledge base and runtime collection work together to provide comprehensive upgrade precheck capabilities:

1. Knowledge base provides reference data for all TiDB versions
2. Runtime collection provides current cluster state
3. Comparison engine identifies potential risks based on differences
4. Report generator produces actionable upgrade recommendations

## Knowledge Base Components

The knowledge base consists of three main components:

1. **Parameter Defaults** (`defaults.json`): Default values for each version
2. **Upgrade Logic** (`upgrade_logic.json`): Forced parameter changes during upgrades
3. **Parameter History** (`parameters-history.json`): Aggregated parameter history across versions

These components are generated by the kbgenerator and consumed by the precheck tools to identify potential upgrade risks.

# PD Parameter History Management

## Overview

PD (Placement Driver) parameter management takes a different approach compared to TiDB. Instead of tracking upgrade logic in bootstrap functions, PD evolves its configuration parameters over time through source code changes. To effectively track these changes, we implement a parameter history management system.

## Purpose

- Track evolution of PD configuration parameters across versions
- Enable flexible comparison between any two versions
- Identify added, removed, and modified parameters
- Support automated upgrade compatibility checking

## Data Sources

- PD source code repository
- Git tags representing released versions
- Configuration definitions in `server/config/config.go`
- Default value assignments in source code

## Implementation

### Parameter History Collection

The parameter history collection works by:

1. Checking out each supported PD version
2. Parsing `server/config/config.go` to extract parameter names and default values
3. Storing parameter values in a versioned history structure
4. Aggregating all version data into a single `parameters-history.json` file

### Data Structure

The parameter history follows this JSON structure:

```json
{
  "component": "pd",
  "parameters": [
    {
      "name": "schedule.enable-diagnostic",
      "type": "bool",
      "history": [
        {
          "version": "v6.5.0",
          "default": false,
          "description": "Enable diagnostic mode for scheduling"
        },
        {
          "version": "v7.1.0",
          "default": true,
          "description": "Enable diagnostic mode for scheduling"
        }
      ]
    }
  ]
}
```

### Dynamic Change Detection

With the parameter history in place, we can dynamically detect changes between any two versions by:

1. Extracting parameter values for the source version
2. Extracting parameter values for the target version
3. Comparing the two sets to identify:
   - Added parameters (present in target but not source)
   - Removed parameters (present in source but not target)
   - Modified parameters (present in both but with different values)

This approach provides flexibility to check compatibility between any two versions without requiring pre-computed comparison results for each version pair.

## Benefits

1. **Flexibility**: Can compare any two versions without pre-computing all combinations
2. **Completeness**: Captures the complete evolution of all parameters
3. **Efficiency**: Single file lookup instead of multiple version comparisons
4. **Maintainability**: Centralized parameter history management