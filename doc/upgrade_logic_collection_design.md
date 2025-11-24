# TiDB Upgrade Logic Collection Design Document

## 1. Introduction

This document describes the design and implementation of the TiDB upgrade logic collection system, which is a submodule of the tidb-upgrade-precheck project. The system automatically extracts and analyzes mandatory system variable changes that occur during TiDB version upgrades to support pre-upgrade validation and risk assessment.

For an overview of the entire configuration and system variable collection system, please refer to the [TiDB Configuration and System Variable Collection Design](./tidb_config_var_collection_design.md) document.

## 2. Purpose

The main purpose of this component is to:
1. Identify all system variable modifications that happen during TiDB upgrades
2. Track which version introduces each change
3. Provide data for precheck tools to validate upgrade compatibility
4. Enable filtering of changes based on version ranges

## 3. Data Collection Scope

The collector focuses on extracting SQL statements that modify system variables during upgrades, specifically:
- `SET GLOBAL variable_name = value` statements
- `INSERT INTO mysql.global_variables` statements
- `UPDATE mysql.global_variables` statements
- `DELETE FROM mysql.global_variables` statements

These statements are typically found in functions named `upgradeToVerXX` where XX represents the bootstrap version number in the [pkg/session/upgrade.go](file:///Users/benjamin2037/Desktop/workspace/sourcecode/tidb/pkg/session/upgrade.go) file.

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

## 11. Related Documentation

- [TiDB Configuration and System Variable Collection Design](./tidb_config_var_collection_design.md) - Overview of the entire system
- [Parameter Collection Design](./parameter_collection_design.md) - Collection of parameter defaults across versions