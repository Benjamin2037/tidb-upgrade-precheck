# Parameter Comparison Design

This document describes the detailed design and implementation of parameter comparison capabilities in the TiDB Upgrade Precheck system.

## Overview

The parameter comparison module is responsible for comparing configuration parameters and system variables between the current cluster state and the target version's knowledge base to identify potential risks during upgrades.

## Architecture

Parameter comparison is performed by the **Analyzer** module using **Rules**. The comparison logic is distributed across multiple rules, each focusing on specific aspects of parameter changes.

## Comparison Rules

### User Modified Params Rule

**Location**: `pkg/analyzer/rules/rule_user_modified_params.go`

**Purpose**: Detects parameters that differ from default values in the source version.

**Comparison Logic:**
- Compare runtime parameter values against source version defaults
- Identify user-customized configurations that may be affected by default value changes

**Use Case**: When a parameter's default value changes in the target version, user-modified values may need attention.

### Upgrade Differences Rule

**Location**: `pkg/analyzer/rules/rule_upgrade_differences.go`

**Purpose**: Detects forced parameter changes during upgrades.

**Comparison Logic:**
- Load upgrade logic from `upgrade_logic.json` (contains all historical forced changes)
- Filter changes by bootstrap version range `(source, target]`
- Compare current runtime values with forced change values
- Categorize by operation type (UPDATE, REPLACE, DELETE) with severity levels

**Severity Levels:**
- **Medium**: UPDATE, REPLACE operations (parameter default value or behavior may change)
- **Low-Medium**: DELETE operations (parameter is deprecated)

**Use Case**: Identify parameters that will be forcibly changed during the upgrade process, regardless of current values.

### TiKV Consistency Rule

**Location**: `pkg/analyzer/rules/rule_tikv_consistency.go`

**Purpose**: Checks parameter consistency across TiKV nodes.

**Comparison Logic:**
- Collect configuration from each TiKV node:
  - User-set values: `last_tikv.toml` from playground data directory
  - Runtime values: `SHOW CONFIG WHERE type='tikv' AND instance='IP:port'`
- Merge with priority: runtime > user-set
- Compare parameter values across all TiKV nodes
- Identify inconsistencies

**Use Case**: Ensure all TiKV nodes have consistent parameter values before upgrade.

### High Risk Params Rule

**Location**: `pkg/analyzer/rules/rule_high_risk_params.go`

**Purpose**: Validates manually specified high-risk parameters.

**Comparison Logic:**
- Load high-risk parameter configuration from JSON file
- Filter by version range (if specified)
- Compare current runtime values with:
  - Allowed values (if specified)
  - Source defaults (if `check_modified` is enabled)

**Use Case**: Allow R&D to manually specify parameters that require special attention during upgrades.

## Data Flow

```
Runtime Cluster
    ↓
Runtime Collector
    ↓
ClusterSnapshot (current state)
    ↓
Analyzer
    ↓
Load Knowledge Base (source & target defaults, upgrade logic)
    ↓
Rule Context (shared data for all rules)
    ↓
Rules (compare and identify risks)
    ↓
CheckResults (findings from each rule)
    ↓
AnalysisResult (organized by category and severity)
```

## Knowledge Base Structure

### Parameter Defaults

Stored in `knowledge/v<major>.<minor>/v<major>.<minor>.<patch>/<component>/defaults.json`:

```json
{
  "component": "tidb",
  "version": "v8.1.0",
  "bootstrap_version": 218,
  "config": {
    "param_name": {
      "value": "default_value",
      "type": "string"
    }
  },
  "system_variables": {
    "var_name": {
      "value": "default_value",
      "type": "string"
    }
  }
}
```

### Upgrade Logic

Stored in `knowledge/tidb/upgrade_logic.json` (generated once globally from master branch):

```json
{
  "changes": [
    {
      "bootstrap_version": 218,
      "operation": "UPDATE",
      "component": "tidb",
      "param_type": "system_variable",
      "param_name": "var_name",
      "new_value": "new_default",
      "severity": "medium"
    }
  ]
}
```

## Comparison Process

1. **Load Knowledge Base**: Load source and target version defaults, and upgrade logic
2. **Extract Bootstrap Versions**: Get bootstrap versions for source and target (TiDB only)
3. **Filter Upgrade Logic**: Filter forced changes by bootstrap version range `(source, target]`
4. **Compare Values**: For each rule, compare:
   - Runtime values vs source defaults (user modifications)
   - Runtime values vs forced changes (upgrade differences)
   - Values across nodes (consistency)
   - Runtime values vs allowed values (high-risk params)
5. **Generate Results**: Create `CheckResult` items for each finding

## Data Structures

See [Types Definition](../../../pkg/types/defaults_types.go) for detailed data structures.

**Key Types:**
- `ParameterValue`: Represents a parameter value with type information
- `ConfigDefaults`: Map of parameter names to `ParameterValue`
- `SystemVariables`: Map of system variable names to `ParameterValue`
- `UpgradeParamChange`: Represents a forced parameter change in upgrade logic
- `CheckResult`: Represents a finding from a rule

## Related Documents

- [Collector Design](../collector/) - Knowledge base generator and runtime collector implementation
- [Analyzer Design](../analyzer/) - Rule-based analyzer implementation
- [Knowledge Base Generation Guide](../../knowledge_generation_guide.md) - User guide for knowledge base generation
