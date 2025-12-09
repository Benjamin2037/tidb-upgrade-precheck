# PD Parameter Upgrade Comparison Design Document

## 1. Introduction

This document describes the design and implementation of the PD (Placement Driver) parameter upgrade comparison system, which is part of the tidb-upgrade-precheck project. The system automatically collects and compares configuration parameters across different PD versions to identify potential compatibility issues and risks during upgrades.

## 2. Purpose

The PD parameter upgrade comparison component is designed to:
1. Automatically collect configuration parameters from different PD versions
2. Compare parameter changes between versions
3. Identify deprecated, modified, or newly added parameters
4. Assess potential risks associated with parameter changes during upgrades
5. Provide actionable recommendations for smooth upgrades

## 3. Data Collection Scope

The collector focuses on extracting PD configuration parameters from different versions, specifically:
- Configuration file parameters (typically in TOML format)
- Command-line flags
- Dynamic configuration options that can be changed at runtime
- Default values for all parameters
- Parameter descriptions and constraints

## 4. Technical Implementation

### 4.1 Collection Process

1. **Source Code Analysis**: Analyze PD source code to extract configuration structures and default values
2. **Configuration File Parsing**: Parse sample configuration files from different versions
3. **Documentation Extraction**: Extract parameter descriptions from documentation and code comments
4. **API Endpoint Discovery**: Identify dynamic configuration endpoints and their capabilities
5. **Version Mapping**: Map parameters across different versions to track changes

### 4.2 Data Structure

The output follows this JSON structure:

```json
{
  "component": "pd",
  "version_from": "v6.5.0",
  "version_to": "v7.1.0",
  "parameters": [
    {
      "name": "schedule.max-store-down-time",
      "type": "duration",
      "from_value": "30m",
      "to_value": "1h",
      "change_type": "modified",
      "risk_level": "medium",
      "description": "Maximum downtime for a store before it is considered unavailable"
    },
    {
      "name": "replication.location-labels",
      "type": "string array",
      "from_value": null,
      "to_value": "",
      "change_type": "added",
      "risk_level": "info",
      "description": "Location labels used to describe the isolation level of nodes"
    }
  ],
  "summary": {
    "total_changes": 15,
    "added": 3,
    "removed": 2,
    "modified": 10,
    "high_risk": 1,
    "medium_risk": 4,
    "low_risk": 10
  }
}
```

Each parameter entry contains:
- `name`: Full parameter name using dot notation
- `type`: Parameter type (string, integer, boolean, duration, etc.)
- `from_value`: Default value in the source version
- `to_value`: Default value in the target version
- `change_type`: Type of change (added, removed, modified)
- `risk_level`: Risk level of the change (info, low, medium, high)
- `description`: Human-readable description of the parameter

### 4.3 Change Detection Logic

1. **Added Parameters**: Parameters present in target version but not in source version
2. **Removed Parameters**: Parameters present in source version but not in target version
3. **Modified Parameters**: Parameters present in both versions but with different default values
4. **Unchanged Parameters**: Parameters with identical values across versions (not included in output by default)

### 4.4 Risk Assessment Criteria

Risk levels are assigned based on the following criteria:
- **High**: Parameters that may cause cluster instability, data loss, or performance degradation
- **Medium**: Parameters that may affect cluster behavior or require manual intervention
- **Low**: Parameters with minimal impact on cluster operations
- **Info**: Parameters with no operational impact

### 4.5 Parameter History Management

To support flexible parameter comparison across arbitrary version pairs, we implement a parameter history management system:

1. **Parameter History Storage**: Store parameter values across all supported versions in a single file with the following structure:

```json
{
  "component": "pd",
  "parameters": [
    {
      "name": "schedule.enable-diagnostic",
      "type": "bool",
      "history": [
        {
          "version": "v6.5.0",
          "default": false,
          "description": "Enable diagnostic mode for scheduling"
        },
        {
          "version": "v7.1.0",
          "default": true,
          "description": "Enable diagnostic mode for scheduling"
        }
      ]
    }
  ]
}
```

2. **Dynamic Change Detection**: Given any source and target version pair, the system can dynamically extract and compare parameter values to detect changes.

3. **Efficient Querying**: The centralized history storage enables efficient querying of parameter changes between any two versions without requiring separate comparison operations for each version pair.

### 4.6 Backward Compatibility Design

The PD parameter system incorporates backward compatibility design principles:

1. **Graceful Parameter Deprecation**: When removing parameters, the system provides clear messaging about alternatives or migration paths.

2. **Default Value Evolution**: Parameter default values may change between versions, but such changes are carefully evaluated for their impact on existing deployments.

3. **Behavioral Consistency**: Even when parameter names or structures change, equivalent functionality is preserved to ensure behavioral consistency.

4. **Migration Guidance**: For significant parameter changes, the system provides specific guidance on how to migrate configurations from older versions.

## 5. Integration with Existing System

### 5.1 Data Flow

```
┌─────────────────┐    ┌──────────────────┐    ┌────────────────────┐
│   PD Source     │    │  Collection &    │    │  Analysis &        │
│     Code        │    │  Comparison      │    │  Reporting         │
│                 │    │                  │    │                    │
│ ┌─────────────┐ │    │ ┌──────────────┐ │    │ ┌────────────────┐ │
│ │ Config      │ │    │ │ Parameter    │ │    │ │ Risk           │ │
│ │ Structures  │ │    │ │ Comparison   │ │    │ │ Assessment     │ │
│ └─────────────┘ │    │ │ Engine       │ │    │ │ Engine         │ │
│ ┌─────────────┐ │    │ └──────────────┘ │    │ └────────────────┘ │
│ │ Sample      │ │    │ ┌──────────────┐ │    │ ┌────────────────┐ │
│ │ Configs     │ │    │ │ Change       │ │    │ │ Report         │ │
│ │ (TOML)      │ │    │ │ Detector     │ │    │ │ Generator      │ │
│ └─────────────┘ │    │ └──────────────┘ │    │ └────────────────┘ │
│ ┌─────────────┐ │    │ ┌──────────────┐ │    │ ┌────────────────┐ │
│ │ Docs &      │ │    │ │ Data         │ │    │ │ Integration    │ │
│ │ Comments    │ │    │ │ Aggregator   │ │    │ │ Layer          │ │
│ └─────────────┘ │    │ └──────────────┘ │    │ └────────────────┘ │
└─────────────────┘    └──────────────────┘    └────────────────────┘
         │                       │                         │
         ▼                       ▼                         ▼
┌─────────────────┐    ┌──────────────────┐    ┌────────────────────┐
│  Raw Parameter  │    │  Structured      │    │  Final Risk        │
│     Data        │    │  Comparison      │    │  Report            │
│                 │    │     Data         │    │                    │
└─────────────────┘    └──────────────────┘    └────────────────────┘
```

### 5.2 API Interface

The PD parameter comparison module exposes the following interface:

```go
type PDParameterComparator interface {
    // CompareParameters compares PD parameters between two versions
    CompareParameters(fromVersion, toVersion string) (*ParameterComparisonReport, error)
    
    // GetParameterDetails retrieves detailed information about a specific parameter
    GetParameterDetails(version, paramName string) (*ParameterDetail, error)
    
    // ListSupportedVersions returns all PD versions supported by the comparator
    ListSupportedVersions() ([]string, error)
}
```

## 6. Future Enhancements

1. **Dynamic Configuration Support**: Track changes to runtime-configurable parameters
2. **Validation Rules**: Implement parameter value validation based on constraints
3. **Cross-component Dependencies**: Identify parameters that interact with TiDB or TiKV settings
4. **Performance Impact Analysis**: Estimate performance implications of parameter changes
5. **Migration Scripts**: Generate automatic migration scripts for compatible parameter changes