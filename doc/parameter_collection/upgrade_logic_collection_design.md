# TiDB Upgrade Logic Collection Design Document

## 1. Introduction

This document describes the design and implementation of the TiDB upgrade logic collection system, which is a submodule of the [TiDB Parameter Collection](../parameter_collection_design.md) system. The system automatically extracts and analyzes mandatory system variable changes that occur during TiDB version upgrades to support pre-upgrade validation and risk assessment.

For an overview of the entire parameter collection system, please refer to the [Parameter Collection Design](../parameter_collection_design.md) document.

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

## 4. mysql.global_variables Operations Analysis

The following are all identified operations related to the `mysql.global_variables` table in TiDB upgrade functions:

### 4.1 upgradeToVer44 (Version 44)
Operation: DELETE
```sql
DELETE FROM mysql.global_variables where variable_name = "tidb_isolation_read_engines"
```
Impact: Removes the `tidb_isolation_read_engines` system variable. After upgrade, this variable will no longer exist in the system, and any application logic depending on it will no longer work.

### 4.2 upgradeToVer68 (Version 68)
Operation: DELETE
```sql
DELETE FROM mysql.global_variables where VARIABLE_NAME = 'tidb_enable_clustered_index' and VARIABLE_VALUE = 'OFF'
```
Impact: Removes the `tidb_enable_clustered_index` variable only when its value is 'OFF'. This affects databases that explicitly disabled clustered indexes. After upgrade, these databases will use the default clustered index behavior.

### 4.3 upgradeToVer71 (Version 71)
Operation: UPDATE
```sql
UPDATE mysql.global_variables SET VARIABLE_VALUE='OFF' WHERE VARIABLE_NAME = 'tidb_multi_statement_mode' AND VARIABLE_VALUE = 'WARN'
```
Impact: Changes the value of `tidb_multi_statement_mode` from 'WARN' to 'OFF'. This affects how multi-statement queries are handled, disabling the warning mode and potentially changing application behavior for multi-statement executions.

### 4.4 upgradeToVer74 (Version 74)
Operation: UPDATE
```sql
UPDATE mysql.global_variables SET VARIABLE_VALUE='%[1]v' WHERE VARIABLE_NAME = 'tidb_stmt_summary_max_stmt_count' AND CAST(VARIABLE_VALUE AS SIGNED) = 200
```
Impact: Updates the `tidb_stmt_summary_max_stmt_count` variable value from the old default (200) to a new default value. This affects statement summary statistics collection, potentially changing the amount of statement data collected and stored.

### 4.5 upgradeToVer179 (Version 179)
Operation: ALTER
```sql
ALTER TABLE mysql.global_variables MODIFY COLUMN `VARIABLE_VALUE` varchar(16383)
```
Impact: Increases the maximum length of variable values from 1024 to 16383 characters. This allows storing much larger values for system variables, accommodating configurations that require longer values.

### 4.6 upgradeToVer216 (Version 216)
Operation: UPDATE
```sql
UPDATE mysql.global_variables SET VARIABLE_VALUE='' WHERE VARIABLE_NAME = 'tidb_scatter_region' AND VARIABLE_VALUE = 'OFF'
UPDATE mysql.global_variables SET VARIABLE_VALUE='table' WHERE VARIABLE_NAME = 'tidb_scatter_region' AND VARIABLE_VALUE = 'ON'
```
Impact: Standardizes the values of `tidb_scatter_region` variable. Changes 'OFF' to empty string and 'ON' to 'table'. This ensures consistent behavior for region scattering functionality after upgrade.

### 4.7 upgradeToVer217 (Version 217)
Operation: INSERT
```sql
INSERT IGNORE INTO mysql.global_variables VALUES ('tidb_schema_cache_size', 0)
```
Impact: Adds a new `tidb_schema_cache_size` system variable with a default value of 0. This enables schema caching functionality with an initial size of 0, which can be adjusted by users for performance optimization.

## 5. Variable Changes After Upgrade - Conclusions

Based on the identified operations, here are the key conclusions about system variable changes after TiDB upgrades:

1. **Variable Removal**: Some variables become obsolete and are removed entirely (e.g., `tidb_isolation_read_engines`), requiring applications to adapt to alternative approaches.

2. **Value Standardization**: Certain variables undergo value transformations to maintain consistency (e.g., `tidb_scatter_region`), which may affect existing configurations.

3. **Default Value Updates**: Variables may get updated default values to reflect improved behaviors or new capabilities (e.g., `tidb_stmt_summary_max_stmt_count`).

4. **Enhanced Capabilities**: Schema changes like increasing `VARIABLE_VALUE` column size enable more flexible configurations.

5. **New Functionality**: New variables are introduced to support additional features (e.g., `tidb_schema_cache_size`).

## 6. Incremental Upgrade Logic

For incremental upgrades from a source version to a target version, the system should:
1. Identify all upgrade functions between the source and target versions
2. Extract and analyze operations in these functions
3. Provide a consolidated view of all changes that will occur during the upgrade process
4. Highlight potential impacts and necessary user actions

## 7. Implementation Plan

1. Enhance the AST parser to identify all relevant SQL operations
2. Implement version range filtering for incremental upgrades
3. Create detailed reports showing variable changes
4. Add functionality to analyze impact of changes on user configurations

## 8. Technical Implementation

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

## 9. Output Location

The collected upgrade logic is stored in:
```
knowledge/tidb/upgrade_logic.json
```

This location ensures it's separate from version-specific parameter data and clearly indicates it contains global upgrade information.

## 10. Usage

### 6.1 Direct Execution
```bash
go run tools/upgrade_logic_collector.go ../tidb > knowledge/tidb/upgrade_logic.json
```

### 6.2 Integration with kb-generator
The collection is automatically triggered when using the kb-generator tool:
```bash
go run cmd/kb-generator/main.go --all
```

## 11. Versioning Strategy

The tool only needs to be run once using the latest TiDB source code, as all historical upgrade functions are preserved in the codebase. This approach ensures:
1. All historical upgrade changes are captured
2. No need to check out specific versions for upgrade logic collection
3. Consistent data structure across runs
4. Simplified maintenance

## 12. Future Enhancements

1. **Enhanced Change Classification**: Improve categorization of changes beyond just SQL statements
2. **Parameter Name Extraction**: Automatically extract parameter names from SQL statements
3. **Comment Analysis**: Include code comments to provide context for changes
4. **Cross-reference with Parameter History**: Link upgrade changes with parameter history data
5. **Performance Optimization**: Improve parsing performance for large codebases

## 13. Integration with Precheck System

The collected upgrade logic will be used by the precheck system to:
1. Identify mandatory changes that will occur during an upgrade
2. Validate that these changes won't conflict with current settings
3. Provide detailed reports on what will change during the upgrade
4. Enable version range-based filtering for targeted checks

## 14. Maintenance Considerations

1. **Code Structure Dependencies**: The tool depends on the structure of [pkg/session/upgrade.go](file:///Users/benjamin2037/Desktop/workspace/sourcecode/tidb/pkg/session/upgrade.go)
2. **Function Naming Patterns**: Assumes consistent naming of `upgradeToVerXX` functions
3. **SQL Pattern Matching**: Regex patterns may need updates as SQL statements evolve
4. **Error Handling**: Robust error handling for file access and parsing issues

## 15. Related Documentation

- [Parameter Collection Design](../parameter_collection_design.md) - Overall parameter collection system
- [TiDB Upgrade Precheck Design](../tidb_upgrade_precheck.md) - Complete upgrade precheck system