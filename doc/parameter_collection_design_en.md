# TiDB Parameter Collection Detailed Design Document

## 1. Overview

This document describes in detail the design and implementation of the parameter collection module in the TiDB upgrade precheck tool. This module is responsible for extracting default values of system variables and configuration parameters from different versions of TiDB source code, providing basic data for upgrade risk assessment.

## 2. Design Goals

1. **Multi-version Compatibility**: Support parameter collection from TiDB v6.x to the latest version
2. **Zero Intrusiveness**: Do not modify TiDB source code, collect parameters through external tools
3. **Accuracy**: Ensure that the collected parameter values are completely consistent with the actual version
4. **Extensibility**: Easy to add support for new versions
5. **Efficiency**: Avoid repeated collection of processed versions

## 3. Core Components

### 3.1 Parameter Collection Tool Files

To support differences in TiDB source code structure across different versions, we provide multiple version-specific collection tools:

- `export_defaults.go` - For the latest version (using pkg directory structure)
- `export_defaults_v6.go` - For v6.x versions
- `export_defaults_v71.go` - For v7.0 - v7.4 versions
- `export_defaults_v75plus.go` - For v7.5+ and v8.x versions
- `export_defaults_legacy.go` - For earlier versions (without pkg directory)

These tool files are located in the `tools/upgrade-precheck/` directory.

### 3.2 Version Routing Mechanism

Automatically select the appropriate collection tool based on the target version:

```go
func selectToolByVersion(tag string) string {
    version := strings.TrimPrefix(tag, "v")
    parts := strings.Split(version, ".")
    
    if len(parts) < 2 {
        return "export_defaults.go"
    }
    
    major, _ := strconv.Atoi(parts[0])
    minor, _ := strconv.Atoi(parts[1])
    
    switch {
    case major == 6:
        return "export_defaults_v6.go"
    case major == 7:
        if minor < 5 {
            return "export_defaults_v71.go"
        } else {
            return "export_defaults_v75plus.go"
        }
    case major >= 8:
        return "export_defaults_v75plus.go"
    default:
        return "export_defaults.go"
    }
}
```

### 3.3 Version Management Mechanism

To avoid collecting the same version repeatedly, the system implements a version management mechanism:

- Use the `knowledge/generated_versions.json` file to record collected versions
- Check if a version already exists before each collection
- Support forcing re-collection of all versions

## 4. Collection Process

### 4.1 Single Version Collection Process

1. Receive target version tag and TiDB source code path
2. Check if the version has already been collected
3. Create a temporary directory for cloning source code
4. Clone TiDB source code to temporary directory
5. Switch to target version tag
6. Select appropriate collection tool based on version
7. Copy collection tool to temporary directory
8. Run collection tool to export parameters
9. Save results to `knowledge/<version>/defaults.json`
10. Record version information to version management file

### 4.2 Full Collection Process

1. Get all LTS version tags
2. Execute single version collection process for each version
3. Aggregate parameter history across all versions
4. Generate global parameter history file `knowledge/tidb/parameters-history.json`

### 4.3 Incremental Collection Process

1. Determine version range based on start and end versions
2. Execute single version collection process for each version in range

## 5. Output Format

### 5.1 defaults.json

Default parameter values for each version are saved in `knowledge/<version>/defaults.json` file:

```json
{
  "sysvars": {
    "variable_name": "default_value",
    ...
  },
  "config": {
    "config_item": "default_value",
    ...
  },
  "bootstrap_version": 0
}
```

### 5.2 parameters-history.json

Aggregated parameter history across all versions is saved in `knowledge/tidb/parameters-history.json` file:

```json
{
  "component": "tidb",
  "parameters": [
    {
      "name": "parameter_name",
      "type": "parameter_type",
      "history": [
        {
          "version": 93,
          "default": "value",
          "scope": "unknown",
          "description": "unknown",
          "dynamic": false
        },
        ...
      ]
    },
    ...
  ]
}
```

## 6. Technical Implementation Details

### 6.1 Temporary Clone Mechanism

To avoid workspace state interference, all collection operations are performed in temporarily cloned repositories:

```go
tempDir, err := ioutil.TempDir("", "tidb_upgrade_precheck")
if err != nil {
    return fmt.Errorf("failed to create temp directory: %v", err)
}
defer os.RemoveAll(tempDir)
```

### 6.2 Dynamic Import Mechanism

Obtain parameter default values through Go's runtime import mechanism:

```go
// Collect config default values
cfg := config.GetGlobalConfig()
cfgMap := make(map[string]interface{})
data, _ := json.Marshal(cfg)
json.Unmarshal(data, &cfgMap)

// Collect all user-visible sysvars
sysvars := make(map[string]interface{})
for _, sv := range variable.GetSysVars() {
    if sv.Hidden || sv.Scope == variable.ScopeNone {
        continue
    }
    sysvars[sv.Name] = sv.Value
}
```

## 7. Usage Guide

### 7.1 Environment Preparation

Ensure the following dependencies are installed:

- Go 1.18+
- Git
- Access to TiDB source code repository

### 7.2 Full Collection

```bash
# Collect all uncollected LTS versions
make collect

# Or
go run cmd/kb-generator/main.go --all
```

### 7.3 Collect All Versions (Including Already Collected)

```bash
# Collect all LTS versions, including already collected versions
make collect-all

# Or
go run cmd/kb-generator/main.go --all --skip-generated=false
```

### 7.4 Incremental Collection

```bash
# Collect specified version range
go run cmd/kb-generator/main.go --from-tag=v7.5.0 --to-tag=v8.1.0
```

### 7.5 Parameter History Aggregation

```bash
# Aggregate parameter history across all versions
make aggregate

# Or
go run cmd/kb-generator/main.go --aggregate
```

### 7.6 Clean Collection Records

```bash
# Clean version collection records
make clean-generated

# Or manually delete
rm knowledge/generated_versions.json
```

## 8. Troubleshooting

### 8.1 Version Collection Failure

If a version collection fails, the system will log a warning and continue processing other versions without interrupting the entire collection process.

### 8.2 Duplicate Collection

Avoid duplicate collection through the version management mechanism to improve efficiency.

### 8.3 Path Issues

Ensure the TiDB source code path is correct, and the tool will validate path validity.

## 9. Extensibility Considerations

### 9.1 Adding New Version Support

1. Create a new version-specific collection tool file
2. Update version routing logic
3. Test new version collection functionality

### 9.2 Parallel Collection

Consider implementing parallel collection in the future to improve performance.

### 9.3 More Metadata Collection

Tools can be extended to collect more metadata about parameters, such as descriptions, scopes, etc.