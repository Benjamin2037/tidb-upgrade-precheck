# High Risk Parameters Rule

## Overview

The `HighRiskParamsRule` checks for manually specified high-risk parameters across all components. This rule allows developers and operators to define custom high-risk parameters that require special attention during upgrade.

**Rule ID**: `HIGH_RISK_PARAMS`  
**Category**: `high_risk`  
**Risk Levels**: Configurable (Error, Warning, Info)

## Purpose

This rule provides a flexible mechanism for identifying and monitoring parameters that are considered high-risk based on:

- Manual configuration by developers/operators
- Version-specific risk assessment
- Custom severity levels
- Allowed value validation

## Configuration

The rule is configured via a JSON file that defines high-risk parameters for each component. The configuration file can be specified via:

- Command-line flag: `--high-risk-params-config <path>`
- Environment variable: `TIDB_UPGRADE_PRECHECK_HIGH_RISK_PARAMS_CONFIG`
- Default locations:
  - `~/.tiup/high_risk_params.json`
  - `~/.tidb-upgrade-precheck/high_risk_params.json`
  - `./high_risk_params.json`

### Configuration Structure

```json
{
  "tidb": {
    "config": {
      "max-connections": {
        "severity": "error",
        "description": "High connection count may cause resource exhaustion",
        "check_modified": true,
        "from_version": "v7.0.0",
        "to_version": "v8.0.0"
      }
    },
    "system_variables": {
      "tidb_mem_quota_query": {
        "severity": "warning",
        "description": "Memory quota per query",
        "allowed_values": [1073741824, 2147483648],
        "check_modified": false
      }
    }
  },
  "pd": {
    "config": {
      "schedule.max-merge-region-size": {
        "severity": "warning",
        "description": "Large merge region size may impact performance"
      }
    }
  },
  "tikv": {
    "config": {
      "storage.reserve-space": {
        "severity": "error",
        "description": "Reserve space configuration is critical"
      }
    }
  },
  "tiflash": {
    "config": {
      "storage.main.dir": {
        "severity": "warning",
        "description": "Storage directory configuration"
      }
    }
  }
}
```

### Configuration Fields

#### HighRiskParamConfig

- **`severity`** (string, required): Severity level when this parameter is found/modified
  - Values: `"error"`, `"warning"`, `"info"`
  - Default: `"warning"` if not specified

- **`description`** (string, optional): Human-readable description of why this parameter is high-risk

- **`allowed_values`** (array, optional): List of allowed values
  - If empty, any modification from default is considered risky
  - If specified, only values in this list are allowed
  - Values not in this list will be reported

- **`check_modified`** (boolean, optional): Whether to check if the parameter has been modified from default
  - If `true`: Only report if the parameter value differs from source default
  - If `false`: Always report if the parameter exists (regardless of value)
  - Default: `false`

- **`from_version`** (string, optional): Minimum version from which this parameter is considered high-risk
  - Format: `"v6.5.0"`, `"v7.1.0"`, etc.
  - If empty, applies to all versions
  - The rule will only check this parameter if `sourceVersion >= from_version`

- **`to_version`** (string, optional): Maximum version until which this parameter is considered high-risk
  - Format: `"v7.5.0"`, `"v8.0.0"`, etc.
  - If empty, applies to all versions after `from_version`
  - The rule will only check this parameter if `sourceVersion <= to_version` (if specified)

## Logic

### Evaluation Process

1. **Load Configuration**: Load high-risk parameters configuration from JSON file (if provided)

2. **Version Filtering**: For each configured parameter:
   - Check if `from_version` and `to_version` apply to current source version
   - Skip parameters that are not applicable to the current version range

3. **Component Checking**: For each component (TiDB, PD, TiKV, TiFlash):
   - Check config parameters (if configured)
   - Check system variables (TiDB only, if configured)

4. **Parameter Validation**: For each high-risk parameter:
   - **If `check_modified` is true**: Only report if value differs from source default
   - **If `allowed_values` is specified**: Only report if value is not in allowed list
   - **Otherwise**: Report if parameter exists

5. **Severity Assignment**: Use configured severity or default to `warning`

### Version Range Filtering

The rule uses `isVersionApplicable()` to filter parameters by version:

- If `from_version` is specified: `sourceVersion >= from_version`
- If `to_version` is specified: `sourceVersion <= to_version`
- If both are specified: `from_version <= sourceVersion <= to_version`
- If neither is specified: Always applicable

### Check Modified Logic

If `check_modified` is `true`:

1. Get source default value from knowledge base
2. Compare current value with source default
3. If `currentValue == sourceDefault`: Skip (not modified)
4. If `currentValue != sourceDefault`: Report (modified from default)

### Allowed Values Logic

If `allowed_values` is specified:

1. Compare current value with each allowed value
2. If `currentValue` matches any allowed value: Skip (value is allowed)
3. If `currentValue` doesn't match any allowed value: Report (value not allowed)

## Data Sources

- **Source Cluster Snapshot**: Current runtime configuration and system variables
- **Source Version Knowledge Base**: Default values (for `check_modified` comparison)
- **High-Risk Parameters Config**: Manual configuration file

## Components Checked

- TiDB (config + system variables)
- PD (config)
- TiKV (config)
- TiFlash (config)

## Output Format

Each high-risk parameter is reported as a separate entry with:

- **Component**: Component name
- **Parameter Name**: Parameter or system variable name
- **Param Type**: `config` or `system_variable`
- **Current Value**: Value currently set in the cluster
- **Source Default**: Default value from source version (if `check_modified` is true)
- **Severity**: Configurable (`error`, `warning`, or `info`)
- **Risk Level**: Determined from severity
- **Message**: "High-risk parameter X found in Y"
- **Details**: Includes current value, description, source default (if applicable), and allowed values (if applicable)
- **Suggestions**: Actionable recommendations

## Examples

### Example 1: Modified Parameter (Error)

```json
{
  "tidb": {
    "config": {
      "max-connections": {
        "severity": "error",
        "description": "High connection count may cause resource exhaustion",
        "check_modified": true
      }
    }
  }
}
```

**Output**:
```
Parameter: max-connections
Component: tidb
Current Value: 5000
Source Default: 1000
Severity: error
Risk Level: high
Message: High-risk parameter max-connections found in tidb
Details: Current value: 5000
Reason: High connection count may cause resource exhaustion
Source default: 1000
```

### Example 2: Value Not in Allowed List (Warning)

```json
{
  "tidb": {
    "system_variables": {
      "tidb_mem_quota_query": {
        "severity": "warning",
        "description": "Memory quota per query",
        "allowed_values": [1073741824, 2147483648],
        "check_modified": false
      }
    }
  }
}
```

**Output** (if current value is 536870912):
```
Parameter: tidb_mem_quota_query
Component: tidb
Current Value: 536870912
Severity: warning
Risk Level: medium
Message: High-risk parameter tidb_mem_quota_query found in tidb
Details: Current value: 536870912
Reason: Memory quota per query
Allowed values: [1073741824 2147483648]
```

### Example 3: Version-Specific Parameter

```json
{
  "pd": {
    "config": {
      "schedule.max-merge-region-size": {
        "severity": "warning",
        "description": "Large merge region size may impact performance",
        "from_version": "v7.0.0",
        "to_version": "v7.5.0"
      }
    }
  }
}
```

**Output** (only if source version is between v7.0.0 and v7.5.0):
```
Parameter: schedule.max-merge-region-size
Component: pd
Current Value: 200
Severity: warning
Risk Level: medium
Message: High-risk parameter schedule.max-merge-region-size found in pd
Details: Current value: 200
Reason: Large merge region size may impact performance
```

## Use Cases

1. **Custom Risk Management**: Define parameters that are critical for specific environments
2. **Version-Specific Monitoring**: Monitor parameters that are risky in specific version ranges
3. **Value Validation**: Ensure parameters are set to allowed values
4. **Compliance**: Track parameters that must be reviewed before upgrade

## Implementation Details

### Code Location

`pkg/analyzer/rules/rule_high_risk_params.go`

### Key Functions

- `NewHighRiskParamsRule()`: Creates a new rule instance and loads configuration
- `loadConfig()`: Loads configuration from JSON file
- `Evaluate()`: Main rule evaluation logic
- `checkComponent()`: Checks high-risk parameters for a specific component
- `checkParameter()`: Checks a single parameter against high-risk configuration
- `isVersionApplicable()`: Checks if a parameter is applicable to the current version

### Data Requirements

```go
SourceClusterRequirements:
  - Components: Dynamic (based on config)
  - NeedConfig: true
  - NeedSystemVariables: true (if TiDB system variables are configured)
  - NeedAllTikvNodes: false

SourceKBRequirements:
  - Components: Dynamic (based on config)
  - NeedConfigDefaults: true (for check_modified)
  - NeedSystemVariables: true (if TiDB system variables are configured)
  - NeedUpgradeLogic: false
```

### Configuration Management

The rule can be managed via the `precheck high-risk-params` subcommand:

- `add`: Add a new high-risk parameter
- `list`: List all configured high-risk parameters
- `remove`: Remove a high-risk parameter
- `view`: View the entire configuration
- `edit`: Open configuration file in editor

## Notes

- **Configuration is optional**: If no config file is provided, the rule returns no results
- **Version filtering**: Parameters are automatically filtered by version range
- **Flexible severity**: Each parameter can have its own severity level
- **Value validation**: Supports both modification checking and allowed value validation
- **Component-specific**: Different parameters can be configured for different components

