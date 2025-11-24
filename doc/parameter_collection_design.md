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
# TiDB Parameter Collection Detailed Design Document

## 1. Introduction

This document describes the design and implementation of the TiDB parameter collection system, which is a submodule of the tidb-upgrade-precheck project. This system automatically collects TiDB system variable defaults across different versions to support pre-upgrade validation and risk assessment.

For an overview of the entire configuration and system variable collection system, please refer to the [TiDB Configuration and System Variable Collection Design](./tidb_config_var_collection_design.md) document.

## 2. Design Goals

1. **Multi-version Compatibility**: Support parameter collection from various TiDB versions
2. **Non-intrusive**: Collect data without modifying the TiDB source code
3. **Accuracy**: Ensure collected data precisely reflects actual parameter defaults
4. **Extensibility**: Allow easy addition of new versions and collection methods
5. **Efficiency**: Minimize resource consumption during collection

## 3. Core Components

### 3.1 Version-Specific Collection Tools

To handle differences in TiDB code structure across versions, we maintain version-specific collection tools:

- **[export_defaults.go](file:///Users/benjamin2037/Desktop/workspace/sourcecode/tidb/tools/export_defaults.go)** - For latest versions (with pkg directory)
- **[export_defaults_legacy.go](file:///Users/benjamin2037/Desktop/workspace/sourcecode/tidb/tools/export_defaults_legacy.go)** - For older versions (without pkg directory)
- **[export_defaults_v6.go](file:///Users/benjamin2037/Desktop/workspace/sourcecode/tidb/tools/export_defaults_v6.go)** - For v6.x versions
- **[export_defaults_v71.go](file:///Users/benjamin2037/Desktop/workspace/sourcecode/tidb-upgrade-precheck/tools/upgrade-precheck/export_defaults_v71.go)** - For v7.1 LTS versions
- **[export_defaults_v75plus.go](file:///Users/benjamin2037/Desktop/workspace/sourcecode/tidb-upgrade-precheck/tools/upgrade-precheck/export_defaults_v75plus.go)** - For v7.5+ and v8.x versions

### 3.2 Collection Orchestration ([pkg/scan/scan.go](file:///Users/benjamin2037/Desktop/workspace/sourcecode/tidb-upgrade-precheck/pkg/scan/scan.go))

This component manages the overall collection process:
- Version detection and tool selection
- Temporary environment setup
- Execution of collection tools
- Result aggregation and output

### 3.3 Version Management ([pkg/scan/version_manager.go](file:///Users/benjamin2037/Desktop/workspace/sourcecode/tidb-upgrade-precheck/pkg/scan/version_manager.go))

Tracks which versions have been processed to avoid redundant work:
- Records processed versions and their commit hashes
- Provides skip/check functionality
- Manages version metadata

## 4. Collection Process

### 4.1 Single Version Collection

For collecting parameters from a specific version:
1. User specifies a Git tag
2. System creates a temporary clone of the TiDB repository at that tag
3. Appropriate export_defaults tool is copied to the cloned repository
4. Tool is executed in the context of the cloned repository
5. Results are saved to `knowledge/<version>/defaults.json`

### 4.2 Full Collection

For collecting parameters from all LTS versions:
1. System identifies all LTS tags in the TiDB repository
2. For each tag:
   - Check if already processed (using VersionManager)
   - If not, perform single version collection
3. Aggregate all collected parameters into `knowledge/tidb/parameters-history.json`

### 4.3 Incremental Collection

For collecting parameters from a range of versions:
1. User specifies from-tag and to-tag
2. System identifies tags in that range
3. Process each tag following the single version collection process

## 5. Output Formats

### 5.1 Version-Specific Parameters ([defaults.json](file:///Users/benjamin2037/Desktop/workspace/sourcecode/tidb-upgrade-precheck/pkg/scan/defaults.go#L79-L79))

Each version's parameters are stored in:
```
knowledge/<version>/defaults.json
```

Structure:
```json
{
  "sysvars": {
    "variable_name": "default_value",
    ...
  },
  "config": {
    "config_name": "default_value",
    ...
  },
  "bootstrap_version": 99
}
```

### 5.2 Parameter History Aggregation

All parameters across versions are aggregated into:
```
knowledge/tidb/parameters-history.json
```

Structure:
```json
{
  "component": "tidb",
  "parameters": [
    {
      "name": "variable_name",
      "type": "string|int|bool|float",
      "history": [
        {
          "version": 95,
          "default": "value",
          "scope": "unknown",
          "description": "unknown",
          "dynamic": false
        },
        ...
      ]
    },
    ...
  ]
}
```

## 6. Technical Implementation Details

### 6.1 Temporary Clone Mechanism

To ensure accurate parameter collection without affecting the original repository:
1. Create a temporary directory
2. Clone the TiDB repository to the temporary directory
3. Checkout the specific tag
4. Copy the appropriate collection tool
5. Execute the tool in the cloned environment
6. Clean up the temporary directory

This approach guarantees that:
- Collection tools match the code structure of the target version
- No interference with the original repository
- Isolated execution environment

### 6.2 Dynamic Import Mechanism

Different TiDB versions have different code structures:
- Older versions: sysvar and config packages in root directory
- Newer versions: sysvar and config packages in pkg directory
- Version-specific import paths and function names

To handle this, we maintain version-specific tools that:
- Import the correct packages for each version
- Call the appropriate functions
- Export data in a consistent format

## 7. Usage Instructions

### 7.1 Environment Setup

1. Ensure both tidb-upgrade-precheck and tidb repositories are cloned
2. Place them in sibling directories
3. Ensure Go 1.18+ is installed
4. Verify Git access to the repositories

### 7.2 Collection Commands

1. **Full Collection**:
   ```bash
   go run cmd/kb-generator/main.go --all
   ```

2. **Single Version**:
   ```bash
   go run cmd/kb-generator/main.go --tag v7.1.0
   ```

3. **Version Range**:
   ```bash
   go run cmd/kb-generator/main.go --from-tag v7.1.0 --to-tag v7.5.0
   ```

4. **Aggregation Only**:
   ```bash
   go run cmd/kb-generator/main.go --aggregate
   ```

## 8. Extensibility

The system is designed to be extensible:
- Adding new version-specific tools for future TiDB versions
- Extending collection to other components (TiKV, PD, TiFlash)
- Adding new output formats
- Integrating with CI/CD systems for automatic updates

## 9. Related Documentation

- [TiDB Configuration and System Variable Collection Design](./tidb_config_var_collection_design.md) - Overview of the entire system
- [Upgrade Logic Collection Design](./upgrade_logic_collection_design.md) - Collection of mandatory changes during upgrades