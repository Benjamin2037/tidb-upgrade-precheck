# TiKV Consistency Rule

## Overview

The `TikvConsistencyRule` compares all TiKV node parameters with the source version knowledge base defaults. This rule identifies parameters that differ from source version defaults across all TiKV nodes in the cluster.

**Rule ID**: `TIKV_CONSISTENCY`  
**Category**: `consistency`  
**Risk Level**: Medium (Warning)

## Purpose

This rule helps identify TiKV node parameters that have been modified from source version defaults, providing visibility into:

- Configuration customizations across TiKV nodes
- Parameters that differ from source version defaults
- Potential inconsistencies that may need attention during upgrade

## Logic

### Comparison Method

1. **Collect TiKV Node Configurations**: For each TiKV node in the cluster:
   - Start with user-set values from `last_tikv.toml` (from `SourceClusterSnapshot`)
   - Get runtime values via `SHOW CONFIG WHERE type='tikv' AND instance='IP:port'` (if TiDB connection available)
   - Merge with priority: **runtime values > user-set values**

2. **Compare with Source Version Defaults**: For each TiKV node:
   - Iterate through all parameters in the merged configuration
   - Compare each parameter value with source version default
   - If `currentValue != sourceDefault` → Report difference

3. **Report Differences**: For each node-parameter combination with a difference:
   - **Severity**: `warning`
   - **Risk Level**: `medium`
   - **Message**: Parameter differs from source version default
   - **Details**: Include node name, instance address, current value, and source default

### Data Sources

- **Source Cluster Snapshot**: TiKV node configurations from `last_tikv.toml`
- **TiDB Connection**: Runtime values via `SHOW CONFIG WHERE type='tikv' AND instance='...'`
- **Source Version Knowledge Base**: Default values for TiKV in source version

### Configuration Merging

The rule merges configuration from two sources with priority:

1. **User-set values** (`last_tikv.toml`): Base configuration
2. **Runtime values** (`SHOW CONFIG`): Override user-set values

**Priority**: Runtime values > User-set values

### Components Checked

- TiKV only (config parameters, no system variables)

### Special Handling

- **TiDB Connection**: Optional - if TiDB connection is not available, only uses `last_tikv.toml` values
- **All Nodes**: Checks all TiKV nodes in the cluster (not just one instance)
- **Per-Node Reporting**: Each node-parameter combination is reported separately

## Output Format

Each difference is reported as a separate entry with:

- **Component**: `tikv`
- **Parameter Name**: Parameter name
- **Param Type**: `config`
- **Current Value**: Value from merged configuration (runtime or user-set)
- **Source Default**: Default value from source version
- **Severity**: `warning`
- **Risk Level**: `medium`
- **Message**: "Parameter X in TiKV node Y differs from source version default"
- **Details**: Includes node name, instance address, current value, and source default
- **Metadata**: 
  - `node_name`: TiKV node name
  - `node_instance`: Instance address (IP:port)
  - `config_sources`: ["last_tikv.toml", "SHOW CONFIG WHERE type='tikv' AND instance='...'"]

## Example

```
Parameter: storage.reserve-space
Component: tikv
Node: tikv-0
Instance: 127.0.0.1:20160
Current Value: 10GB
Source Default: 5GB
Severity: warning
Risk Level: medium
Message: Parameter storage.reserve-space in TiKV node tikv-0 differs from source version default
Details: Node: tikv-0 (instance: 127.0.0.1:20160), Current value: 10GB, Source version default: 5GB
```

## Use Cases

1. **Configuration Audit**: Identify all TiKV parameters that differ from defaults
2. **Upgrade Preparation**: Understand which parameters have been customized
3. **Node Comparison**: Compare parameters across TiKV nodes (though this rule focuses on KB comparison)
4. **Compatibility Check**: Ensure modified values are compatible with target version

## Implementation Details

### Code Location

`pkg/analyzer/rules/rule_tikv_consistency.go`

### Key Functions

- `Evaluate()`: Main rule evaluation logic
- `GetConfigByTypeAndInstance()`: Gets runtime config for specific TiKV instance
- `determineValueType()`: Determines the type of a value
- `extractValueFromDefault()`: Extracts actual value from ParameterValue structure

### Data Requirements

```go
SourceClusterRequirements:
  - Components: ["tikv"]
  - NeedConfig: true
  - NeedSystemVariables: false
  - NeedAllTikvNodes: true

SourceKBRequirements:
  - Components: ["tikv"]
  - NeedConfigDefaults: true
  - NeedSystemVariables: false
  - NeedUpgradeLogic: false
```

### Configuration Collection

1. **User-set Config**: From `component.Config` (populated from `last_tikv.toml`)
2. **Runtime Config**: Via `SHOW CONFIG WHERE type='tikv' AND instance='IP:port'`
3. **Merging**: Runtime values override user-set values

### TiDB Connection

- The rule attempts to connect to TiDB to get runtime configs
- If connection fails, continues with user-set config only
- Connection is optional (rule can work without it)

## Notes

- **All TiKV nodes are checked** (not just one instance)
- **Each node-parameter combination is a separate entry**
- **Runtime values take priority** over user-set values
- **Only reports differences** - parameters matching source defaults are not reported
- **TiDB connection is optional** - rule works with or without it

