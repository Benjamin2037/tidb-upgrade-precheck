# TiKV Parameter Upgrade Comparison Design Document

## 1. Introduction

This document describes the design and implementation of the TiKV parameter upgrade comparison system, which is part of the tidb-upgrade-precheck project. The system automatically collects and compares configuration parameters across different TiKV versions to identify potential compatibility issues and risks during upgrades.

## 2. Purpose

The TiKV parameter upgrade comparison component is designed to:
1. Automatically collect configuration parameters from different TiKV versions
2. Compare parameter changes between versions
3. Identify deprecated, modified, or newly added parameters
4. Assess potential risks associated with parameter changes during upgrades
5. Provide actionable recommendations for smooth upgrades
6. Support scale-in/scale-out scenario analysis for configuration consistency

## 3. Data Collection Scope

The collector focuses on extracting TiKV configuration parameters from different versions, specifically:
- Configuration file parameters (typically in TOML format)
- Command-line flags
- Dynamic configuration options that can be changed at runtime
- Default values for all parameters
- Parameter descriptions and constraints
- Feature gates and experimental features

## 4. Technical Implementation

### 4.1 Collection Process

1. **Source Code Analysis**: Analyze TiKV source code to extract configuration structures and default values
2. **Configuration File Parsing**: Parse sample configuration files from different versions
3. **Documentation Extraction**: Extract parameter descriptions from documentation and code comments
4. **API Endpoint Discovery**: Identify dynamic configuration endpoints and their capabilities
5. **Feature Gate Tracking**: Monitor feature gates and experimental features across versions
6. **Version Mapping**: Map parameters across different versions to track changes

### 4.2 Data Structure

The output follows this JSON structure:

```json
{
  "component": "tikv",
  "version_from": "v6.5.0",
  "version_to": "v7.1.0",
  "parameters": [
    {
      "name": "raftstore.raft-entry-max-size",
      "type": "size",
      "from_value": "8MB",
      "to_value": "16MB",
      "change_type": "modified",
      "risk_level": "medium",
      "description": "Max size of request payload for raft election"
    },
    {
      "name": "storage.reserve-space",
      "type": "size",
      "from_value": "2GB",
      "to_value": "0",
      "change_type": "modified",
      "risk_level": "high",
      "description": "Disk space to reserve for non-TiKV purposes"
    }
  ],
  "feature_gates": [
    {
      "name": "unified_read_pool",
      "status_from": "stable",
      "status_to": "deprecated",
      "risk_level": "medium",
      "description": "Unified read pool feature"
    }
  ],
  "summary": {
    "total_changes": 25,
    "added": 5,
    "removed": 3,
    "modified": 17,
    "high_risk": 2,
    "medium_risk": 8,
    "low_risk": 15,
    "feature_gate_changes": 3
  }
}
```

Each parameter entry contains:
- `name`: Full parameter name using dot notation
- `type`: Parameter type (string, integer, boolean, size, duration, etc.)
- `from_value`: Default value in the source version
- `to_value`: Default value in the target version
- `change_type`: Type of change (added, removed, modified)
- `risk_level`: Risk level of the change (info, low, medium, high)
- `description`: Human-readable description of the parameter

Feature gate entries contain:
- `name`: Name of the feature gate
- `status_from`: Status in the source version (experimental, stable, deprecated, removed)
- `status_to`: Status in the target version
- `risk_level`: Risk level associated with the status change
- `description`: Description of the feature

### 4.3 Change Detection Logic

1. **Added Parameters**: Parameters present in target version but not in source version
2. **Removed Parameters**: Parameters present in source version but not in target version
3. **Modified Parameters**: Parameters present in both versions but with different default values
4. **Feature Gate Changes**: Changes in feature gate statuses (experimental → stable, stable → deprecated, etc.)
5. **Unchanged Parameters**: Parameters with identical values across versions (not included in output by default)

### 4.4 Risk Assessment Criteria

Risk levels are assigned based on the following criteria:
- **High**: Parameters that may cause cluster instability, data loss, performance degradation, or require significant operational changes
- **Medium**: Parameters that may affect cluster behavior, require configuration adjustments, or impact performance
- **Low**: Parameters with minimal impact on cluster operations or user workloads
- **Info**: Parameters with no operational impact or purely informational changes

Special considerations for TiKV:
- Storage-related parameters often carry higher risk levels
- Raft-related parameters can significantly impact cluster stability
- Memory and CPU-related parameters may affect performance
- Network-related parameters may affect connectivity and latency

## 5. Scale-In/Scale-Out Scenario Support

### 5.1 Configuration Consistency Checking

Since each TiKV node manages its own parameters and configurations may vary across nodes (especially in scale-in/scale-out scenarios), the system provides specialized functionality to:

1. **Detect Inconsistent Configurations**: Identify parameters that have different values across TiKV instances in a cluster
2. **Assess Risk of Inconsistencies**: Evaluate the risk level of configuration inconsistencies
3. **Provide Resolution Recommendations**: Offer guidance on how to resolve configuration inconsistencies

### 5.2 Data Structure for Cluster Configuration Analysis

For scale-in/scale-out scenarios, the system uses the following data structure:

```json
{
  "instances": [
    {
      "address": "192.168.1.10:20160",
      "state": {
        "type": "tikv",
        "version": "v6.5.0",
        "config": {
          "storage.reserve-space": "2GB",
          "raftstore.raft-entry-max-size": "8MB"
        }
      }
    },
    {
      "address": "192.168.1.11:20160",
      "state": {
        "type": "tikv",
        "version": "v6.5.0",
        "config": {
          "storage.reserve-space": "0",
          "raftstore.raft-entry-max-size": "8MB"
        }
      }
    }
  ],
  "inconsistent_configs": [
    {
      "parameter_name": "storage.reserve-space",
      "values": [
        {
          "instance_address": "192.168.1.10:20160",
          "value": "2GB"
        },
        {
          "instance_address": "192.168.1.11:20160",
          "value": "0"
        }
      ],
      "risk_level": "high",
      "description": "Parameter storage.reserve-space has different values across instances"
    }
  ]
}
```

### 5.3 Inconsistency Risk Assessment

The system evaluates the risk of configuration inconsistencies based on:

1. **Parameter Criticality**: How critical the parameter is to cluster operation
2. **Value Differences**: The magnitude of differences between values
3. **Operational Impact**: Potential impact on cluster behavior and performance

Risk levels for inconsistencies:
- **High**: Critical parameters with significantly different values that may cause data inconsistency or cluster instability
- **Medium**: Important parameters with different values that may affect performance or behavior
- **Low**: Non-critical parameters with minor differences

### 5.4 Scale-Out Scenario Analysis

When adding new TiKV nodes to a cluster, the system can:

1. **Compare New Node Configuration**: Analyze the configuration of the new node against existing nodes
2. **Identify Potential Issues**: Detect configurations that may cause problems when integrated with the existing cluster
3. **Provide Setup Recommendations**: Suggest configuration changes for the new node to ensure consistency and compatibility

### 5.5 Scale-In Scenario Analysis

When removing TiKV nodes from a cluster, the system can:

1. **Analyze Impact of Removal**: Determine how the removal affects configuration balance in the cluster
2. **Check for Unique Configurations**: Identify if the node to be removed has unique configurations that need to be preserved elsewhere
3. **Recommend Actions**: Suggest any necessary configuration changes to other nodes before removal

## 6. Integration with Existing System

### 6.1 Data Flow

```
┌─────────────────┐    ┌──────────────────┐    ┌────────────────────┐
│   TiKV Source   │    │  Collection &    │    │  Analysis &        │
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
│ ┌─────────────┐ │    │ ┌──────────────┐ │    │                    │
│ │ Feature     │ │    │ │ Feature Gate │ │    │                    │
│ │ Gates       │ │    │ │ Tracker      │ │    │                    │
│ └─────────────┘ │    │ └──────────────┘ │    │                    │
└─────────────────┘    └──────────────────┘    └────────────────────┘
         │                       │                         │
         ▼                       ▼                         ▼
┌─────────────────┐    ┌──────────────────┐    ┌────────────────────┐
│  Raw Parameter  │    │  Structured      │    │  Final Risk        │
│     Data        │    │  Comparison      │    │  Report            │
│                 │    │     Data         │    │                    │
└─────────────────┘    └──────────────────┘    └────────────────────┘
```

### 6.2 API Interface

The TiKV parameter comparison module exposes the following interface:

```go
type TiKVParameterComparator interface {
    // CompareParameters compares TiKV parameters between two versions
    CompareParameters(fromVersion, toVersion string) (*ParameterComparisonReport, error)
    
    // GetParameterDetails retrieves detailed information about a specific parameter
    GetParameterDetails(version, paramName string) (*ParameterDetail, error)
    
    // ListSupportedVersions returns all TiKV versions supported by the comparator
    ListSupportedVersions() ([]string, error)
    
    // GetFeatureGateStatus retrieves the status of a feature gate in a specific version
    GetFeatureGateStatus(version, featureName string) (*FeatureGateStatus, error)
    
    // AnalyzeClusterConsistency analyzes configuration consistency across TiKV instances
    AnalyzeClusterConsistency(instances []InstanceState) (*ClusterConsistencyReport, error)
}

type TiKVClusterAnalyzer interface {
    // AnalyzeScaleOutScenario analyzes a scale-out scenario
    AnalyzeScaleOutScenario(existingInstances []InstanceState, newNode InstanceState) (*ScaleOutAnalysisReport, error)
    
    // AnalyzeScaleInScenario analyzes a scale-in scenario
    AnalyzeScaleInScenario(existingInstances []InstanceState, nodeToRemove InstanceState) (*ScaleInAnalysisReport, error)
}
```

## 7. Special Considerations for TiKV

### 7.1 Configuration Complexity

TiKV configurations are more complex than TiDB due to:
- Multiple subsystems (storage, raftstore, coprocessor, etc.)
- Hardware-specific tuning parameters
- Complex interactions between parameters
- More feature gates and experimental features

### 7.2 Performance Sensitivity

Many TiKV parameters directly impact performance:
- Block cache sizes
- Thread pool configurations
- I/O scheduling parameters
- Compression settings

### 7.3 Stability Impact

TiKV parameter changes can have significant stability impacts:
- Raft consensus parameters
- Storage engine configurations
- Memory allocation limits
- Crash recovery settings

## 8. Future Enhancements

1. **Hardware-aware Recommendations**: Provide parameter suggestions based on hardware profiles
2. **Workload-aware Tuning**: Recommend parameters based on workload characteristics
3. **Cross-component Validation**: Check parameter consistency across TiDB, PD, and TiKV
4. **Performance Modeling**: Predict performance impacts of parameter changes
5. **Automated Migration**: Generate safe migration paths for complex parameter changes
6. **Rollback Planning**: Identify parameters that require special handling during rollbacks
7. **Advanced Scale Scenario Analysis**: More sophisticated analysis of scale-in/scale-out scenarios
8. **Configuration Drift Detection**: Monitor and alert on configuration drift over time