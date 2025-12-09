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
        "allowed_values": [value1, value2]
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
  - Format: `"v6.5.0"`, `"v7.1.0"`, etc.
  - If empty, applies to all versions
  - The rule will only check this parameter when `sourceVersion >= fromVersion`

- **to_version** (string, optional): Up to which version this parameter is considered high-risk
  - Format: `"v7.5.0"`, `"v8.0.0"`, etc.
  - If empty, applies to all versions after `fromVersion`
  - The rule will only check this parameter when `sourceVersion <= toVersion` (if `to_version` is specified)

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

This configuration only checks the `tidb_enable_1pc` parameter when the source version is between v6.5.0 and v7.5.0.

### Example 5: Check from Specific Version

```json
{
  "tikv": {
    "config": {
      "raftstore.raft-entry-max-size": {
        "severity": "warning",
        "description": "Raft entry max size affects replication performance",
        "check_modified": true,
        "from_version": "v7.1.0"
      }
    }
  }
}
```

This configuration only checks the `raftstore.raft-entry-max-size` parameter when the source version >= v7.1.0.

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

## Configuration File Location Recommendations

It is recommended to place the configuration file in one of the following locations:

1. Project root directory: `high_risk_params.json`
2. Configuration directory: `config/high_risk_params.json`
3. Environment variable: Specify path via environment variable `HIGH_RISK_PARAMS_CONFIG`

## Best Practices

1. **Maintain by module**: It is recommended that different module R&D teams maintain their own component's high-risk parameter list
2. **Regular updates**: Update the high-risk parameter list in a timely manner as versions evolve
3. **Detailed descriptions**: Provide clear descriptions for each parameter explaining why it is high-risk
4. **Set appropriate severity levels**: Set appropriate severity levels based on the actual impact of parameters
5. **Use allowed_values**: For parameters with clear safe ranges, use allowed_values to restrict them

## Complete Example

Refer to the `high_risk_params_config.example.json` file for a complete example.
