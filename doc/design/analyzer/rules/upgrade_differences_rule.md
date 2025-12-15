# Upgrade Differences Rule

## Overview

The `UpgradeDifferencesRule` detects parameters that will differ after upgrade, including forced changes from upgrade logic. This rule compares target version defaults with current cluster values and identifies various types of changes that will occur during upgrade.

**Rule ID**: `UPGRADE_DIFFERENCES`  
**Category**: `upgrade_difference`  
**Risk Levels**: High (Error), Medium (Warning), Low (Info)

## Purpose

This rule provides comprehensive visibility into parameter changes that will occur during upgrade, helping users:

- Identify forced changes that cannot be prevented
- Understand default value changes between versions
- Detect deprecated and new parameters
- Plan for configuration adjustments

## Logic

The rule processes parameters in the following order:

### 1. Target Version Parameters vs Current Cluster

For each parameter that exists in the target version knowledge base:

#### 1.1 Forced Changes (in upgrade_logic.json)

If the parameter is listed in `upgrade_logic.json`:

- **If `forcedValue != currentValue`**:
  - **Severity**: `warning`
  - **Risk Level**: `medium`
  - **Message**: Parameter will be forcibly changed (forced value differs from current)
  - **Action**: User must be aware that this change cannot be prevented

- **If `forcedValue == currentValue`**:
  - **Severity**: `info`
  - **Risk Level**: `low`
  - **Message**: Default value changed (forced change matches current value)
  - **Action**: Informational only, no actual change will occur

#### 1.2 Non-Forced Changes

If the parameter is **not** in `upgrade_logic.json`:

- **If `targetDefault != currentValue`**:
  - **Severity**: `info`
  - **Risk Level**: `low`
  - **Message**: Default value changed (target default differs from current)
  - **Action**: Informational, user should review

- **If `targetDefault == currentValue` but `sourceDefault != targetDefault`**:
  - **Severity**: `info`
  - **Risk Level**: `low`
  - **Message**: Default value changed between source and target versions
  - **Action**: Informational, current value matches new default

- **If all values are consistent**: Skip (don't report)

### 2. Deprecated Parameters

For parameters that exist in source version but **not** in target version:

- **Severity**: `info`
- **Risk Level**: `low`
- **Message**: Parameter is deprecated (exists in source version but removed in target version)
- **Action**: User should review if this parameter is still needed and plan for migration

### 3. New Parameters

For parameters that exist in target version but **not** in source version:

- **Severity**: `info`
- **Risk Level**: `low`
- **Message**: Parameter is new (added in target version)
- **Action**: User should review the new parameter and its default value

### 4. Consistent Parameters

Parameters where:
- `currentValue == targetDefault == sourceDefault`

These are **not reported** to reduce noise.

## Data Sources

- **Source Cluster Snapshot**: Current runtime configuration and system variables
- **Source Version Knowledge Base**: Default values for source version
- **Target Version Knowledge Base**: Default values for target version
- **Upgrade Logic**: Forced changes from `upgrade_logic.json` (filtered by bootstrap version range)

## Components Checked

- TiDB (config + system variables)
- PD (config)
- TiKV (config)
- TiFlash (config)

## Special Handling

### Forced Changes

- Forced changes are extracted from `upgrade_logic.json`
- Filtered by bootstrap version range: `(sourceBootstrapVersion, targetBootstrapVersion]`
- For TiDB: Forced system variable changes are critical (error severity)
- For other components: Forced config changes are warnings (medium risk)

### System Variables (TiDB)

- TiDB system variables have special behavior: they keep old values after upgrade unless forced
- If not forced, system variables maintain their current value
- Only reports if current value differs from target default (informational)

## Output Format

Each difference is reported as a separate entry with:

- **Component**: Component name
- **Parameter Name**: Parameter or system variable name
- **Param Type**: `config` or `system_variable`
- **Current Value**: Value currently set in the cluster
- **Source Default**: Default value from source version
- **Target Default**: Default value from target version
- **Forced Value**: Value that will be forced (if applicable)
- **Severity**: `error`, `warning`, or `info`
- **Risk Level**: `high`, `medium`, or `low`
- **Message**: Describes the type of change
- **Suggestions**: Actionable recommendations

## Examples

### Example 1: Forced Change (Medium Risk)

```
Parameter: tidb_mem_quota_query
Component: tidb
Current Value: 1073741824
Forced To: 2147483648
Severity: warning
Risk Level: medium
Message: Parameter tidb_mem_quota_query in tidb will be forcibly changed during upgrade (forced value differs from current)
```

### Example 2: Default Value Changed (Low Risk)

```
Parameter: max-connections
Component: tidb
Current Value: 2000
Target Default: 3000
Source Default: 1000
Severity: info
Risk Level: low
Message: Parameter max-connections in tidb: default value changed (target default differs from current)
```

### Example 3: Deprecated Parameter (Low Risk)

```
Parameter: old-param
Component: pd
Current Value: true
Source Default: true
Severity: info
Risk Level: low
Message: Parameter old-param in pd is deprecated (exists in source version but removed in target version)
```

### Example 4: New Parameter (Low Risk)

```
Parameter: new-param
Component: tikv
Target Default: 100
Severity: info
Risk Level: low
Message: Parameter new-param in tikv is new (added in target version)
```

## Use Cases

1. **Upgrade Planning**: Understand what will change during upgrade
2. **Risk Assessment**: Identify forced changes that require attention
3. **Configuration Review**: Review default value changes
4. **Migration Planning**: Plan for deprecated and new parameters

## Implementation Details

### Code Location

`pkg/analyzer/rules/rule_upgrade_differences.go`

### Key Functions

- `Evaluate()`: Main rule evaluation logic
- `GetForcedChanges()`: Extracts forced changes from upgrade logic (in RuleContext)
- `extractValueFromDefault()`: Extracts actual value from ParameterValue structure

### Data Requirements

```go
SourceClusterRequirements:
  - Components: ["tidb", "pd", "tikv", "tiflash"]
  - NeedConfig: true
  - NeedSystemVariables: true
  - NeedAllTikvNodes: false

TargetKBRequirements:
  - Components: ["tidb", "pd", "tikv", "tiflash"]
  - NeedConfigDefaults: true
  - NeedSystemVariables: true
  - NeedUpgradeLogic: true

SourceKBRequirements:
  - Components: ["tidb", "pd", "tikv", "tiflash"]
  - NeedConfigDefaults: true
  - NeedSystemVariables: true
  - NeedUpgradeLogic: true
```

### Processing Order

1. Process all target version parameters (compare with current cluster)
2. Process deprecated parameters (source has, target doesn't)
3. Process new parameters (target has, source doesn't)
4. Skip consistent parameters (all values match)

## Notes

- **Consistent parameters are not reported** to reduce noise
- **Forced changes take priority** over default value changes
- **Bootstrap version filtering** ensures only relevant forced changes are considered
- **System variables** have special handling due to TiDB's upgrade behavior

