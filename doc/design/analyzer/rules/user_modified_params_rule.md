# User Modified Parameters Rule

## Overview

The `UserModifiedParamsRule` detects parameters that have been modified by the user from the source version defaults. This rule compares the current cluster runtime values with the source version knowledge base defaults to identify user customizations.

**Rule ID**: `USER_MODIFIED_PARAMS`  
**Category**: `user_modified`  
**Risk Level**: Low (Info)

## Purpose

This rule helps identify which parameters have been customized by users, providing visibility into configuration changes that may need attention during upgrade. Understanding user modifications is crucial for:

- Identifying intentional customizations that should be preserved
- Detecting potential compatibility issues with target version
- Planning configuration migration strategies

## Logic

### Comparison Method

1. **Iterate through source version defaults**: For each component (TiDB, PD, TiKV, TiFlash), iterate through all parameters defined in the source version knowledge base.

2. **Extract current cluster values**: For each parameter:
   - **Config parameters**: Read from `component.Config`
   - **System variables**: Read from `component.Variables` (for TiDB only)

3. **Compare values**: Compare current cluster value with source version default:
   - If `currentValue != sourceDefault` → User has modified this parameter
   - If `currentValue == sourceDefault` → Skip (no modification)

4. **Report differences**: For each modified parameter, create a `CheckResult` with:
   - **Severity**: `info`
   - **Risk Level**: `low`
   - **Message**: Indicates the parameter has been modified
   - **Details**: Shows current value vs source default

### Data Sources

- **Source Cluster Snapshot**: Current runtime configuration and system variables
- **Source Version Knowledge Base**: Default values for source version

### Components Checked

- TiDB (config + system variables)
- PD (config)
- TiKV (config)
- TiFlash (config)

### Special Handling

- **TiKV/TiFlash**: Only checks the first instance to avoid duplicate results
- **System Variables**: Handles the `sysvar:` prefix for knowledge base lookups
- **Missing Parameters**: If a parameter exists in source KB but not in runtime, reports as error (validation issue)

## Output Format

Each modified parameter is reported as a separate entry with:

- **Component**: Component name (tidb, pd, tikv, tiflash)
- **Parameter Name**: Parameter or system variable name
- **Param Type**: `config` or `system_variable`
- **Current Value**: Value currently set in the cluster
- **Source Default**: Default value from source version
- **Severity**: `info`
- **Risk Level**: `low`
- **Message**: "Parameter X in Y has been modified by user (differs from source version default)"
- **Suggestions**: 
  - "This parameter has been modified from the source version default"
  - "Review if this modification is intentional and appropriate"
  - "Ensure the modified value is compatible with target version"

## Example

```
Parameter: max-connections
Component: tidb
Current Value: 2000
Source Default: 1000
Severity: info
Risk Level: low
Message: Parameter max-connections in tidb has been modified by user (differs from source version default)
```

## Use Cases

1. **Pre-upgrade Review**: Identify all user customizations before upgrade
2. **Configuration Audit**: Track which parameters have been modified from defaults
3. **Compatibility Check**: Ensure modified values are compatible with target version
4. **Documentation**: Maintain a record of custom configurations

## Implementation Details

### Code Location

`pkg/analyzer/rules/rule_user_modified_params.go`

### Key Functions

- `Evaluate()`: Main rule evaluation logic
- `extractValueFromDefault()`: Extracts actual value from ParameterValue structure

### Data Requirements

```go
SourceClusterRequirements:
  - Components: ["tidb", "pd", "tikv", "tiflash"]
  - NeedConfig: true
  - NeedSystemVariables: true
  - NeedAllTikvNodes: false

SourceKBRequirements:
  - Components: ["tidb", "pd", "tikv", "tiflash"]
  - NeedConfigDefaults: true
  - NeedSystemVariables: true
  - NeedUpgradeLogic: false
```

## Notes

- This rule only reports **differences**, not identical values
- Parameters that match source defaults are **not reported** (to reduce noise)
- The rule assumes one-to-one correspondence between source KB and runtime cluster (validated by `validateComponentMapping`)

