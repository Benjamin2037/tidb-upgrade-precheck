# LTS Version Default Collection Design Document

## 1. Introduction

This document describes the detailed design and implementation of collecting default parameter values from TiDB LTS (Long Term Support) versions. As part of the [TiDB Parameter Collection](./parameter_collection_design.md) system, this component focuses specifically on accurately extracting system variable and configuration defaults from various LTS releases.

For an overview of the entire parameter collection system, please refer to the [Parameter Collection Design](./parameter_collection_design.md) document.

## 2. LTS Version Identification

TiDB follows a release model where specific versions are designated as LTS versions. These versions receive extended support and are commonly used in production environments. The collection system identifies LTS versions by their Git tags in the TiDB repository.

### 2.1 LTS Version Patterns

The system recognizes the following LTS version patterns:
- v6.5.x series
- v7.1.x series (LTS version)
- v7.5.x series
- v8.1.x series (LTS version)
- v8.5.x series

### 2.2 Version Filtering

To ensure only official LTS releases are processed, the system applies the following filters:
- Excludes release candidates (e.g., v7.1.0-rc)
- Excludes development versions (e.g., v7.1.0-beta)
- Focuses on patch releases that represent stable versions

## 3. Version-Specific Collection Approach

Different TiDB versions have different code structures, which requires version-specific collection approaches.

### 3.1 Code Structure Evolution

1. **Legacy versions** (v6.5.x and earlier):
   - sysvar and config packages located in the root directory
   - Different function signatures for accessing default values

2. **Modern versions** (v7.0.0 and later):
   - sysvar and config packages moved to the pkg directory
   - Updated APIs for accessing system variables and configurations

### 3.2 Tool Selection Logic

The system automatically selects the appropriate collection tool based on the version being processed:
- For v6.5.x versions: [export_defaults_v6.go](file:///Users/benjamin2037/Desktop/workspace/sourcecode/tidb/tools/export_defaults_v6.go)
- For v7.1.x versions: [export_defaults_v71.go](file:///Users/benjamin2037/Desktop/workspace/sourcecode/tidb-upgrade-precheck/tools/upgrade-precheck/export_defaults_v71.go)
- For v7.5+ and v8.x versions: [export_defaults_v75plus.go](file:///Users/benjamin2037/Desktop/workspace/sourcecode/tidb-upgrade-precheck/tools/upgrade-precheck/export_defaults_v75plus.go)
- For latest versions: [export_defaults.go](file:///Users/benjamin2037/Desktop/workspace/sourcecode/tidb/tools/export_defaults.go)
- For legacy versions: [export_defaults_legacy.go](file:///Users/benjamin2037/Desktop/workspace/sourcecode/tidb/tools/export_defaults_legacy.go)

## 4. Collection Process Details

### 4.1 Environment Setup

For each LTS version:
1. Create a temporary clone of the TiDB repository
2. Checkout the specific LTS tag
3. Copy the appropriate collection tool to the cloned repository
4. Execute the tool in the context of that version

### 4.2 Data Extraction

The collection tools extract the following types of parameters:
1. **System Variables** - Variables accessible through `SHOW VARIABLES` or `SET` statements
2. **Configuration Parameters** - Parameters defined in configuration files (config.toml)

### 4.3 Data Validation

The collection process includes validation steps:
1. Ensure all expected parameters are collected
2. Verify data types of parameter values
3. Check for consistency with known parameter behaviors

## 5. Handling Version-Specific Challenges

### 5.1 API Changes

Different versions may have different APIs for accessing system variables:
- Some versions use `variable.GetSysVar`
- Others use direct package access
- Newer versions may have refactored packages

### 5.2 Package Location Changes

The location of sysvar and config packages has changed across versions:
- Legacy: direct access to packages
- Modern: access through pkg directory

### 5.3 Default Value Evolution

Some parameters may have changed their default values or even been removed/added across versions, which the system tracks in the historical aggregation.

## 6. Output Format

Each LTS version's defaults are stored in:
```
knowledge/<version>/defaults.json
```

With the structure:
```json
{
  "sysvars": {
    "variable_name": "default_value",
    ...
  },
  "config": {
    "config_name": "default_value",
    ...
  },
  "bootstrap_version": 99
}
```

## 7. Error Handling and Recovery

### 7.1 Version-Specific Failures

When a specific version collection fails:
1. Log detailed error information
2. Continue with other versions
3. Report failed versions at the end

### 7.2 Tool Compatibility Issues

If a collection tool is incompatible with a version:
1. Attempt to use alternative tools
2. Log compatibility issues
3. Mark version as requiring manual attention

## 8. Performance Considerations

### 8.1 Parallel Processing

The system can process multiple versions in parallel to improve collection speed, though this is currently implemented as sequential processing to ensure resource constraints are respected.

### 8.2 Caching Mechanisms

Processed versions are tracked to avoid redundant collection:
- VersionManager records processed versions
- Git commit hashes are stored for verification
- Allows incremental updates

## 9. Future Improvements

1. **Enhanced Version Detection**: More sophisticated logic for identifying LTS versions
2. **Cross-Version Analysis**: Better tools for comparing parameter changes across versions
3. **Automated Tool Generation**: Generate version-specific tools based on code analysis
4. **Performance Optimization**: Parallel processing of multiple versions

## 10. Related Documentation

- [Parameter Collection Design](./parameter_collection_design.md) - Overall parameter collection system
- [TiDB Upgrade Precheck Design](./tidb_upgrade_precheck.md) - Complete upgrade precheck system