# High-Risk Parameters Manual Edit Guide

## Overview

High-risk parameters can be managed by directly editing the JSON configuration file in the knowledge base directory. This approach is simpler and more straightforward than using command-line tools.

## Configuration Files

The high-risk parameters configuration consists of a single file:

1. **`knowledge/high_risk_params/high_risk_params.json`** (Editable)
   - Configuration file for high-risk parameters
   - Generated from `pkg/analyzer/rules/high_risk_params/default.json` during knowledge base generation
   - Contains default high-risk parameters for common upgrade scenarios
   - Technical support staff should edit this file directly to add custom high-risk parameters

## How to Add a High-Risk Parameter

### Step 1: Locate the Configuration File

The configuration file is located at:
```
knowledge/high_risk_params/high_risk_params.json
```

If the file doesn't exist, create it with the following structure:

```json
{
  "tidb": {
    "config": {},
    "system_variables": {}
  },
  "pd": {
    "config": {}
  },
  "tikv": {
    "config": {}
  },
  "tiflash": {
    "config": {}
  }
}
```

### Step 2: Add the Parameter

Edit the JSON file and add your parameter to the appropriate component section.

**Example: Adding a TiDB config parameter**

```json
{
  "tidb": {
    "config": {
      "max-connections": {
        "severity": "error",
        "description": "Max connections is critical for performance",
        "check_modified": true,
        "from_version": "v7.5.0",
        "to_version": ""
      }
    },
    "system_variables": {}
  },
  "pd": {
    "config": {}
  },
  "tikv": {
    "config": {}
  },
  "tiflash": {
    "config": {}
  }
}
```

**Example: Adding a TiDB system variable**

```json
{
  "tidb": {
    "config": {},
    "system_variables": {
      "tidb_enable_async_commit": {
        "severity": "warning",
        "description": "Async commit may cause compatibility issues",
        "check_modified": true,
        "from_version": "v8.0.0",
        "to_version": "v8.5.0"
      }
    }
  },
  "pd": {
    "config": {}
  },
  "tikv": {
    "config": {}
  },
  "tiflash": {
    "config": {}
  }
}
```

**Example: Adding a TiKV config parameter**

```json
{
  "tidb": {
    "config": {},
    "system_variables": {}
  },
  "pd": {
    "config": {}
  },
  "tikv": {
    "config": {
      "raftstore.raft-log-gc-size-limit": {
        "severity": "warning",
        "description": "Raft log GC size limit may need adjustment after upgrade",
        "check_modified": true,
        "from_version": "v7.5.0",
        "to_version": ""
      }
    }
  },
  "tiflash": {
    "config": {}
  }
}
```

## Configuration Field Descriptions

Each parameter configuration supports the following fields:

- **`severity`** (string, required): Severity level
  - `"error"`: Error level, must be addressed
  - `"warning"`: Warning level, recommended to address
  - `"info"`: Info level, for reference only

- **`description`** (string, optional): Human-readable description explaining why this parameter is high-risk

- **`check_modified`** (bool, optional): Whether to only check modified parameters
  - `true`: Only report if the parameter value differs from the source version's default value
  - `false`: Report as long as the parameter exists (regardless of whether it's modified)

- **`from_version`** (string, optional): From which version this parameter is considered high-risk
  - Format: `"v7.5.0"`, `"v8.0.0"`, etc.
  - If empty, applies to all versions

- **`to_version`** (string, optional): Until which version this parameter is considered high-risk
  - Format: `"v8.5.0"`, `"v9.0.0"`, etc.
  - If empty, applies to all versions after `from_version`

- **`allowed_values`** (array, optional): List of allowed values
  - If empty, any modification from default is considered risky
  - If specified, only values not in this list will be reported

## Reference Template

The `knowledge/high_risk_params/high_risk_params.json` file contains examples of high-risk parameters for common upgrade scenarios. You can edit this file directly to add or modify parameters.

## Validation

After editing the JSON file, you can validate it by:

1. **JSON Syntax Check**: Use any JSON validator or editor
2. **Runtime Validation**: The tool will validate the JSON when loading the configuration
3. **Test Run**: Run the precheck tool to verify the configuration is loaded correctly

## Notes

- The configuration file uses standard JSON format
- Changes take effect immediately after saving the file (no restart needed)
- The file is loaded directly from the knowledge base during analysis

