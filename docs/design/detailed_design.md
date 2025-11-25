# TiDB Upgrade Precheck - Detailed Design Document

## 1. Overview

This document provides a detailed design for the third phase of the tidb-upgrade-precheck system, focusing on:
1. Collection of current configuration and system variables from target clusters
2. Comparison of current values with current default values to determine user-modified parameters
3. Final comparison with target version parameters to identify upgrade risks

## 2. Collector Design

### 2.1. Data Collection Components

The collector module is responsible for gathering real-time configuration data from a running TiDB cluster. It consists of the following components:

#### 2.1.1. TiDB Configuration Collector
- Collects TiDB configuration via HTTP API endpoint `/config`
- Retrieves global system variables via SQL query `SHOW GLOBAL VARIABLES`
- Stores data in structured format for further processing

#### 2.1.2. TiKV Configuration Collector
- Collects TiKV configuration via HTTP API endpoint `/config`
- Retrieves relevant metrics and settings
- Maps configuration to standardized format

#### 2.1.3. PD Configuration Collector
- Collects PD configuration via HTTP API endpoint `/pd/api/v1/config`
- Retrieves schedule configuration and replication settings
- Standardizes collected data

### 2.2. Collector Data Structures

```go
// ClusterSnapshot represents the complete configuration state of a cluster
type ClusterSnapshot struct {
    Timestamp     time.Time              `json:"timestamp"`
    SourceVersion string                 `json:"source_version"`
    TargetVersion string                 `json:"target_version"`
    Components    map[string]ComponentState `json:"components"`
}

// ComponentState represents the configuration state of a single component
type ComponentState struct {
    Type       string                 `json:"type"`        // tidb, tikv, pd, tiflash
    Version    string                 `json:"version"`
    Config     map[string]interface{} `json:"config"`      // Configuration parameters
    Variables  map[string]string      `json:"variables"`   // System variables (for TiDB)
    Status     map[string]interface{} `json:"status"`      // Runtime status information
}

// CollectedData represents the raw data collected from a cluster
type CollectedData struct {
    Component string      `json:"component"`
    Type      string      `json:"type"`
    Data      interface{} `json:"data"`
}
```

### 2.3. Collection Process

1. **Connection Establishment**
   - Establish connection to TiDB (MySQL protocol)
   - Establish HTTP connections to TiKV and PD endpoints

2. **Data Retrieval**
   - Query TiDB system variables: `SHOW GLOBAL VARIABLES`
   - Retrieve TiDB configuration: HTTP GET to `/config`
   - Retrieve TiKV configuration: HTTP GET to `/config`
   - Retrieve PD configuration: HTTP GET to `/pd/api/v1/config`

3. **Data Processing**
   - Normalize parameter names across components
   - Convert values to appropriate types
   - Store in structured format

## 3. Analyzer Design

### 3.1. Analysis Components

The analyzer compares the current cluster state with knowledge base data to identify potential upgrade risks.

#### 3.1.1. Parameter State Determination
Determines whether each parameter is using its default value or has been modified by the user:

```go
type ParameterState string

const (
    UseDefault ParameterState = "use_default"  // Using default value
    UserSet    ParameterState = "user_set"     // Modified by user
)

type ParameterAnalysis struct {
    Name          string          `json:"name"`
    Component     string          `json:"component"`
    CurrentValue  interface{}     `json:"current_value"`
    SourceDefault interface{}     `json:"source_default"`
    TargetDefault interface{}     `json:"target_default"`
    State         ParameterState  `json:"state"`
}
```

#### 3.1.2. Risk Identification
Identifies risks based on the comparison matrix:

| Source State | Target State           | Risk Level | Action           |
|--------------|------------------------|------------|------------------|
| UseDefault   | Default Changed        | MEDIUM     | Recommendation   |
| UseDefault   | Forced Upgrade         | HIGH       | Must Handle      |
| UserSet      | Default Changed        | INFO       | Configuration Audit |
| UserSet      | Forced Upgrade         | HIGH       | Must Handle      |

### 3.2. Analysis Process

1. **Load Knowledge Base**
   - Load `defaults.json` for source and target versions
   - Load `upgrade_logic.json` for forced changes

2. **Parameter State Analysis**
   - For each parameter in the cluster:
     - Compare current value with source version default
     - Determine if it's `UseDefault` or `UserSet`

3. **Risk Assessment**
   - For `UserSet` parameters:
     - Check if target default differs from source default
     - Check if parameter is forcibly changed in upgrade
   - For `UseDefault` parameters:
     - Check if default value changes in target version
     - Check if parameter is forcibly changed in upgrade

4. **Result Compilation**
   - Group findings by risk level
   - Prepare detailed information for each finding

## 4. Knowledge Base Structure

### 4.1. Defaults Knowledge Base

```json
{
  "version": "v7.5.0",
  "bootstrap_version": 180,
  "components": {
    "tidb": {
      "config": {
        "performance.max-procs": 0,
        "log.level": "info"
      },
      "variables": {
        "tidb_enable_clustered_index": "INT_ONLY",
        "max_connections": "151"
      }
    },
    "tikv": {
      "config": {
        "raftstore.apply-pool-size": 2,
        "server.grpc-concurrency": 5
      }
    },
    "pd": {
      "config": {
        "schedule.replica-schedule-limit": 64,
        "schedule.region-schedule-limit": 2048
      }
    }
  }
}
```

### 4.2. Upgrade Logic Knowledge Base

```json
[
  {
    "version": 180,
    "function": "upgradeToVer180",
    "changes": [
      {
        "type": "variable_change",
        "variable": "tidb_enable_clustered_index",
        "forced_value": "ON",
        "scope": "global",
        "description": "Enable clustered index by default"
      }
    ]
  }
]
```

## 5. Comparator Design

### 5.1. Comparison Logic

The comparator performs three-level comparisons:

1. **Current vs Source Default**
   - Determines if parameter is user-modified
   
2. **Source Default vs Target Default**
   - Identifies default value changes
   
3. **Forced Changes Check**
   - Checks if parameter is forcibly modified during upgrade

### 5.2. Comparison Algorithm

```go
func analyzeParameter(
    paramName string,
    currentValue interface{},
    sourceDefault interface{},
    targetDefault interface{},
    forcedChanges map[string]interface{}
) *ParameterAnalysis {
    analysis := &ParameterAnalysis{
        Name:          paramName,
        CurrentValue:  currentValue,
        SourceDefault: sourceDefault,
        TargetDefault: targetDefault,
    }
    
    // Step 1: Determine parameter state
    if isDefaultValue(currentValue, sourceDefault) {
        analysis.State = UseDefault
    } else {
        analysis.State = UserSet
    }
    
    // Step 2: Check for risks
    // ... risk assessment logic ...
    
    return analysis
}
```

## 6. Risk Classification

### 6.1. HIGH Risk (P0)
- Parameters that will be forcibly changed during upgrade regardless of current value
- Potential for service disruption or data inconsistency

### 6.2. MEDIUM Risk (P1)
- Default value changes that may impact performance or behavior
- User-set values that differ from new defaults but won't be forcibly changed

### 6.3. LOW Risk (P2)
- Configuration audit items
- User-set values that differ from defaults but pose minimal risk

## 7. Implementation Plan

### 7.1. Phase 1: Collector Implementation
- [ ] Implement TiDB configuration collector
- [ ] Implement TiKV configuration collector
- [ ] Implement PD configuration collector
- [ ] Create unified collection interface
- [ ] Add error handling and retry logic

### 7.2. Phase 2: Analyzer Implementation
- [ ] Implement parameter state determination
- [ ] Implement risk assessment logic
- [ ] Create knowledge base loading utilities
- [ ] Add comparison algorithms

### 7.3. Phase 3: Integration and Testing
- [ ] Integrate collector and analyzer
- [ ] Create end-to-end tests
- [ ] Performance optimization
- [ ] Documentation and examples

## 8. API Design

### 8.1. Collector Interface

```go
type Collector interface {
    Collect(ctx context.Context, endpoints ClusterEndpoints) (*ClusterSnapshot, error)
}

type ClusterEndpoints struct {
    TiDBAddr string   // MySQL protocol endpoint
    TiKVAddrs []string // HTTP API endpoints
    PDAddrs   []string // HTTP API endpoints
}
```

### 8.2. Analyzer Interface

```go
type Analyzer interface {
    Analyze(ctx context.Context, snapshot *ClusterSnapshot, sourceKB, targetKB *KnowledgeBase) (*AnalysisResult, error)
}

type AnalysisResult struct {
    HighRiskItems   []RiskItem `json:"high_risk_items"`
    MediumRiskItems []RiskItem `json:"medium_risk_items"`
    LowRiskItems    []RiskItem `json:"low_risk_items"`
    AuditItems      []AuditItem `json:"audit_items"`
}
```

## 9. Error Handling

### 9.1. Collection Errors
- Network connectivity issues
- Authentication failures
- API endpoint unavailability
- Data parsing errors

### 9.2. Analysis Errors
- Missing knowledge base data
- Version mismatch errors
- Data inconsistency issues

### 9.3. Recovery Strategies
- Retry mechanisms for transient errors
- Graceful degradation for partial data
- Clear error reporting for user action

## 10. Performance Considerations

### 10.1. Collection Optimization
- Parallel collection from multiple components
- Connection pooling for HTTP clients
- Efficient data serialization

### 10.2. Analysis Optimization
- Caching of knowledge base data
- Early termination of risk checks
- Memory-efficient data structures

## 11. Security Considerations

### 11.1. Data Protection
- Secure storage of credentials
- Encryption of sensitive data in transit
- Minimal privilege principle for database connections

### 11.2. Access Control
- Role-based access to cluster information
- Audit logging of collection activities
- Compliance with data protection regulations