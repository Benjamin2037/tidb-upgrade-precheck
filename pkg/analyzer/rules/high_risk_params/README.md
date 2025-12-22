# High Risk Parameters Rule

## Overview

`HighRiskParamsRule` allows R&D teams to manually specify high-risk parameters that will be automatically detected during pre-upgrade checks.

## Features

1. **Component-based definition**: Supports defining high-risk parameters separately for TiDB, PD, TiKV, and TiFlash
2. **Configuration parameters and system variables**: Supports checking both configuration parameters and system variables (TiDB)
3. **Flexible condition checking**:
   - `check_modified`: Whether to only check modified parameters
   - `allowed_values`: Specify a list of allowed values
   - `severity`: Custom severity level (error/warning/info)
4. **Detailed descriptions**: Each parameter can include description information explaining why it is high-risk
5. **Version range support**: Parameters can be configured to apply only to specific version ranges (from_version to to_version)
6. **Upgrade path checking**: The rule checks if the upgrade path (sourceVersion -> targetVersion) overlaps with the configured version range

## Managing High-Risk Parameters

High-risk parameters are managed by directly editing JSON configuration files in the knowledge base directory. This approach is simpler and more straightforward than using command-line tools.

### Configuration Files

1. **`knowledge/high_risk_params/high_risk_params.json`** (Editable)
   - Configuration file for high-risk parameters
   - Generated from `pkg/analyzer/rules/high_risk_params/default.json` during knowledge base generation
   - Contains default high-risk parameters for common upgrade scenarios
   - Technical support staff should edit this file directly to add custom high-risk parameters

### How to Add a Parameter

1. Open `knowledge/high_risk_params/high_risk_params.json` in a text editor
2. Add your parameter to the appropriate component section (tidb, pd, tikv, or tiflash)
3. Save the file

See [MANUAL_EDIT_GUIDE.md](./MANUAL_EDIT_GUIDE.md) for detailed instructions and examples.

## Configuration File Format

The configuration file is in JSON format with the following structure:

```json
{
  "tidb": {
    "config": {
      "parameter_name": {
        "severity": "error",
        "description": "Why this parameter is high-risk",
        "check_modified": true,
        "allowed_values": [value1, value2],
        "from_version": "v7.5.0",
        "to_version": "v8.5.0"
      }
    },
    "system_variables": {
      "variable_name": {
        "severity": "warning",
        "description": "Why this variable is high-risk",
        "check_modified": false
      }
    }
  },
  "pd": {
    "config": {
      "parameter_name": {
        "severity": "error",
        "description": "Description",
        "check_modified": true
      }
    }
  },
  "tikv": {
    "config": {
      "parameter_name": {
        "severity": "warning",
        "description": "Description",
        "check_modified": true
      }
    }
  },
  "tiflash": {
    "config": {
      "parameter_name": {
        "severity": "error",
        "description": "Description",
        "check_modified": true
      }
    }
  }
}
```

## Configuration Field Descriptions

### HighRiskParamConfig

- **severity** (string, required): Severity level
  - `"error"`: Error level, must be addressed
  - `"warning"`: Warning level, recommended to address
  - `"info"`: Info level, for reference only

- **description** (string, optional): Parameter description explaining why it is high-risk

- **allowed_values** (array, optional): List of allowed values
  - If empty, any value will be reported (if check_modified is false)
  - If specified, only values not in the list will be reported

- **check_modified** (bool, optional): Whether to only check modified parameters
  - `true`: Only report parameters that differ from the source version's default value
  - `false`: Report as long as the parameter exists (regardless of whether it's modified)

- **from_version** (string, optional): From which version this parameter is considered high-risk
  - Format: `"v6.5.0"`, `"v7.5.0"`, etc.
  - If empty, applies to all versions
  - **Semantic meaning**: If only `from_version` is specified (without `to_version`), it indicates that the parameter change/risk starts from this version and continues indefinitely
  - The rule will check this parameter if the upgrade path overlaps with the version range

- **to_version** (string, optional): Up to which version this parameter is considered high-risk
  - Format: `"v7.5.0"`, `"v8.5.0"`, etc.
  - If empty, applies to all versions after `fromVersion`
  - **Semantic meaning**: If only `to_version` is specified (without `from_version`), it indicates that:
    - The parameter may be deprecated after this version, OR
    - There may be further changes after this version that require different handling
  - The rule will check this parameter if the upgrade path overlaps with the version range

### Version Range Checking

The rule checks if the upgrade path (sourceVersion -> targetVersion) overlaps with the configured version range (fromVersion -> toVersion).

**Version Range Semantics**:
- **Both `from_version` and `to_version` specified**: The parameter is high-risk within this specific version range
- **Only `from_version` specified**: The parameter change/risk starts from this version and continues indefinitely (ongoing risk)
- **Only `to_version` specified**: The parameter may be deprecated after this version, or there may be further changes requiring different handling
- **Neither specified**: The parameter is high-risk for all versions

**Examples**:
- Config: `fromVersion=v7.5.0, toVersion=v8.5.0`
  - Upgrade: `v7.5.0 -> v8.5.0` → **Will check** (overlap: v7.5.0 to v8.5.0)
  - Upgrade: `v6.5.0 -> v8.5.0` → **Will check** (overlap: v7.5.0 to v8.5.0)
  - Upgrade: `v7.5.0 -> v8.5.0` → **Will check** (full overlap)

- Config: `fromVersion=v7.5.0` (only from_version)
  - Upgrade: `v7.5.0 -> v8.5.0` → **Will check** (overlap: v7.5.0 to v8.5.0)
  - Upgrade: `v6.5.0 -> v7.5.0` → **Will not check** (source version < from_version)

- Config: `toVersion=v7.5.0` (only to_version)
  - Upgrade: `v6.5.0 -> v7.5.0` → **Will check** (overlap: v6.5.0 to v7.5.0)
  - Upgrade: `v7.5.0 -> v8.5.0` → **Will not check** (source version > to_version)

## Usage Examples

### Example 1: Check Modified Critical Parameters

```json
{
  "tidb": {
    "config": {
      "performance.max-procs": {
        "severity": "error",
        "description": "Max procs setting is critical for performance",
        "check_modified": true
      }
    }
  }
}
```

This configuration checks whether the `performance.max-procs` parameter has been modified, and reports it as error level if modified.

### Example 2: Check Specific Value Range

```json
{
  "tidb": {
    "system_variables": {
      "tidb_mem_quota_query": {
        "severity": "error",
        "description": "Query memory quota is critical",
        "allowed_values": [1073741824, 2147483648, 4294967296],
        "check_modified": false
      }
    }
  }
}
```

This configuration checks whether the value of `tidb_mem_quota_query` is in the allowed list, and reports if it's not.

### Example 3: Always Check Parameters

```json
{
  "tikv": {
    "config": {
      "storage.reserve-space": {
        "severity": "error",
        "description": "Reserve space is critical for preventing disk full",
        "check_modified": false
      }
    }
  }
}
```

This configuration checks whether the `storage.reserve-space` parameter exists, and reports regardless of whether it's modified.

### Example 4: Specify Version Range

```json
{
  "tidb": {
    "system_variables": {
      "tidb_enable_1pc": {
        "severity": "warning",
        "description": "One-phase commit setting affects transaction performance",
        "check_modified": true,
        "from_version": "v6.5.0",
        "to_version": "v7.5.0"
      }
    }
  }
}
```

This configuration only checks the `tidb_enable_1pc` parameter when the upgrade path overlaps with the version range v6.5.0 to v7.5.0.

### Example 5: Check from Specific Version (Ongoing Risk)

```json
{
  "tikv": {
    "config": {
      "raftstore.raft-entry-max-size": {
        "severity": "warning",
        "description": "Raft entry max size affects replication performance",
        "check_modified": true,
        "from_version": "v7.5.0"
      }
    }
  }
}
```

This configuration checks the `raftstore.raft-entry-max-size` parameter when the upgrade path overlaps with versions >= v7.5.0. Since only `from_version` is specified, it indicates that the risk starts from v7.5.0 and continues indefinitely.

### Example 6: Check Until Specific Version (Deprecated or Further Changes)

```json
{
  "tidb": {
    "config": {
      "old-parameter-name": {
        "severity": "warning",
        "description": "This parameter is deprecated after v7.5.0",
        "check_modified": true,
        "to_version": "v7.5.0"
      }
    }
  }
}
```

This configuration checks the `old-parameter-name` parameter when the upgrade path overlaps with versions <= v7.5.0. Since only `to_version` is specified, it indicates that the parameter may be deprecated after v7.5.0, or there may be further changes requiring different handling.

## Usage in Code

```go
// Create rule and load configuration file
rule, err := rules.NewHighRiskParamsRule("/path/to/high_risk_params.json")
if err != nil {
    log.Fatal(err)
}

// Add to analyzer
options := &analyzer.AnalysisOptions{
    Rules: []rules.Rule{
        rules.NewUserModifiedParamsRule(),
        rules.NewUpgradeDifferencesRule(),
        rules.NewTikvConsistencyRule(),
        rule, // Add high-risk parameters rule
    },
}

analyzer := analyzer.NewAnalyzer(options)
```

## Configuration File Location

The configuration file is located at:
- `knowledge/high_risk_params/high_risk_params.json`

This file is automatically loaded by the tool from the knowledge base.

## Best Practices

1. **Maintain by module**: It is recommended that different module R&D teams maintain their own component's high-risk parameter list
2. **Regular updates**: Update the high-risk parameter list in a timely manner as versions evolve
3. **Detailed descriptions**: Provide clear descriptions for each parameter explaining why it is high-risk
4. **Set appropriate severity levels**: Set appropriate severity levels based on the actual impact of parameters
5. **Use allowed_values**: For parameters with clear safe ranges, use allowed_values to restrict them
6. **Use version ranges**: Specify `from_version` and `to_version` to limit when parameters are checked

## Complete Example

Refer to the `example.json` file in this directory for a complete example.

