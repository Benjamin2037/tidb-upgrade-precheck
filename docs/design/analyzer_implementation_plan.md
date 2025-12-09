# Analyzer Implementation Plan

## 1. Overview

This document outlines the implementation plan for the analyzer module that will compare current cluster configuration with knowledge base data to identify potential upgrade risks.

## 2. Module Structure

```
pkg/
└── analyzer/
    ├── analyzer.go          # Main analyzer interface
    ├── comparator.go        # Comparison logic
    ├── risk_evaluator.go    # Risk evaluation logic
    ├── types.go             # Data structures
    └── utils.go             # Utility functions
```

## 3. Implementation Steps

### 3.1. Define Data Structures (types.go)

```go
package analyzer

import (
    "time"
    
    "github.com/pingcap/tidb-upgrade-precheck/pkg/collector"
)

// ParameterState represents whether a parameter is using default or user-set value
type ParameterState string

const (
    UseDefault ParameterState = "use_default"  // Using default value
    UserSet    ParameterState = "user_set"     // Modified by user
)

// ParameterType represents the type of a parameter
type ParameterType string

const (
    ConfigParam   ParameterType = "config"    // Configuration parameter
    VariableParam ParameterType = "variable"  // System variable
)

// ParameterInfo contains information about a parameter
type ParameterInfo struct {
    Name      string        `json:"name"`
    Component string        `json:"component"`
    Type      ParameterType `json:"type"`
    Value     interface{}   `json:"value"`
}

// ParameterAnalysis contains analysis results for a parameter
type ParameterAnalysis struct {
    Name          string          `json:"name"`
    Component     string          `json:"component"`
    Type          ParameterType   `json:"type"`
    CurrentValue  interface{}     `json:"current_value"`
    SourceDefault interface{}     `json:"source_default"`
    TargetDefault interface{}     `json:"target_default"`
    State         ParameterState  `json:"state"`
    IsForced      bool            `json:"is_forced"`
    ForcedValue   interface{}     `json:"forced_value,omitempty"`
}

// RiskLevel represents the severity of a risk
type RiskLevel string

const (
    RiskHigh   RiskLevel = "high"
    RiskMedium RiskLevel = "medium"
    RiskLow    RiskLevel = "low"
)

// RiskItem represents a single identified risk
type RiskItem struct {
    ParameterName string      `json:"parameter_name"`
    Component     string      `json:"component"`
    RiskLevel     RiskLevel   `json:"risk_level"`
    CurrentValue  interface{} `json:"current_value"`
    DefaultValue  interface{} `json:"default_value,omitempty"`
    ForcedValue   interface{} `json:"forced_value,omitempty"`
    Description   string      `json:"description"`
    Suggestions   []string    `json:"suggestions,omitempty"`
}

// AuditItem represents a configuration audit item
type AuditItem struct {
    ParameterName  string      `json:"parameter_name"`
    Component      string      `json:"component"`
    CurrentValue   interface{} `json:"current_value"`
    TargetDefault  interface{} `json:"target_default"`
    Description    string      `json:"description"`
}

// AnalysisResult contains the complete analysis results
type AnalysisResult struct {
    Timestamp      time.Time    `json:"timestamp"`
    SourceVersion  string       `json:"source_version"`
    TargetVersion  string       `json:"target_version"`
    Risks          []RiskItem   `json:"risks"`
    Audits         []AuditItem  `json:"audits"`
    Summary        RiskSummary  `json:"summary"`
}

// RiskSummary provides a summary of identified risks
type RiskSummary struct {
    Total  int `json:"total"`
    High   int `json:"high"`
    Medium int `json:"medium"`
    Low    int `json:"low"`
}

// Analyzer defines the interface for analyzing cluster configurations
type Analyzer interface {
    Analyze(snapshot *collector.ClusterSnapshot, sourceKB, targetKB map[string]interface{}) (*AnalysisResult, error)
}
```

### 3.2. Main Analyzer Interface (analyzer.go)

```go
package analyzer

import (
    "context"
    "fmt"
    
    "github.com/pingcap/tidb-upgrade-precheck/pkg/collector"
)

type analyzer struct {
    comparator      Comparator
    riskEvaluator   RiskEvaluator
}

// NewAnalyzer creates a new analyzer instance
func NewAnalyzer() Analyzer {
    return &analyzer{
        comparator:    NewComparator(),
        riskEvaluator: NewRiskEvaluator(),
    }
}

// Analyze performs analysis of a cluster snapshot against knowledge base data
func (a *analyzer) Analyze(snapshot *collector.ClusterSnapshot, sourceKB, targetKB map[string]interface{}) (*AnalysisResult, error) {
    result := &AnalysisResult{
        Timestamp:     snapshot.Timestamp,
        SourceVersion: snapshot.SourceVersion,
        TargetVersion: snapshot.TargetVersion,
        Risks:         make([]RiskItem, 0),
        Audits:        make([]AuditItem, 0),
    }

    // Perform parameter analysis
    analyses, err := a.comparator.Compare(snapshot, sourceKB, targetKB)
    if err != nil {
        return nil, fmt.Errorf("failed to compare configurations: %w", err)
    }

    // Evaluate risks
    risks, audits := a.riskEvaluator.Evaluate(analyses, sourceKB, targetKB)
    result.Risks = risks
    result.Audits = audits

    // Generate summary
    result.Summary = generateSummary(risks)

    return result, nil
}

func generateSummary(risks []RiskItem) RiskSummary {
    summary := RiskSummary{}
    summary.Total = len(risks)
    
    for _, risk := range risks {
        switch risk.RiskLevel {
        case RiskHigh:
            summary.High++
        case RiskMedium:
            summary.Medium++
        case RiskLow:
            summary.Low++
        }
    }
    
    return summary
}
```

### 3.3. Comparator Logic (comparator.go)

```go
package analyzer

import (
    "fmt"
    
    "github.com/pingcap/tidb-upgrade-precheck/pkg/collector"
)

// Comparator handles comparison of current configuration with knowledge base data
type Comparator interface {
    Compare(snapshot *collector.ClusterSnapshot, sourceKB, targetKB map[string]interface{}) ([]*ParameterAnalysis, error)
}

type comparator struct{}

// NewComparator creates a new comparator instance
func NewComparator() Comparator {
    return &comparator{}
}

// Compare performs parameter-by-parameter comparison
func (c *comparator) Compare(snapshot *collector.ClusterSnapshot, sourceKB, targetKB map[string]interface{}) ([]*ParameterAnalysis, error) {
    var analyses []*ParameterAnalysis

    // Analyze TiDB parameters
    if tidbComponent, exists := snapshot.Components["tidb"]; exists {
        // Analyze system variables
        variableAnalyses, err := c.analyzeVariables(tidbComponent, sourceKB, targetKB)
        if err != nil {
            return nil, fmt.Errorf("failed to analyze variables: %w", err)
        }
        analyses = append(analyses, variableAnalyses...)

        // Analyze configuration parameters
        configAnalyses, err := c.analyzeConfig(tidbComponent, sourceKB, targetKB)
        if err != nil {
            return nil, fmt.Errorf("failed to analyze config: %w", err)
        }
        analyses = append(analyses, configAnalyses...)
    }

    // TODO: Analyze TiKV and PD parameters

    return analyses, nil
}

func (c *comparator) analyzeVariables(component collector.ComponentState, sourceKB, targetKB map[string]interface{}) ([]*ParameterAnalysis, error) {
    var analyses []*ParameterAnalysis

    // Get source and target system variable defaults
    sourceSysVars := make(map[string]interface{})
    targetSysVars := make(map[string]interface{})

    if source, ok := sourceKB["system_variables"].(map[string]interface{}); ok {
        sourceSysVars = source
    }

    if target, ok := targetKB["system_variables"].(map[string]interface{}); ok {
        targetSysVars = target
    }

    // Analyze each variable
    for name, currentValue := range component.Variables {
        analysis := &ParameterAnalysis{
            Name:         name,
            Component:    component.Type,
            Type:         VariableParam,
            CurrentValue: currentValue,
            State:        UseDefault, // Default assumption
        }

        // Get source and target defaults
        if sourceDefault, exists := sourceSysVars[name]; exists {
            analysis.SourceDefault = sourceDefault
        }

        if targetDefault, exists := targetSysVars[name]; exists {
            analysis.TargetDefault = targetDefault
        }

        // Determine if parameter is user-set
        if analysis.SourceDefault != nil && fmt.Sprintf("%v", analysis.SourceDefault) != fmt.Sprintf("%v", currentValue) {
            analysis.State = UserSet
        }

        analyses = append(analyses, analysis)
    }

    return analyses, nil
}

func (c *comparator) analyzeConfig(component collector.ComponentState, sourceKB, targetKB map[string]interface{}) ([]*ParameterAnalysis, error) {
    var analyses []*ParameterAnalysis

    // Get source and target config defaults
    sourceConfigs := make(map[string]interface{})
    targetConfigs := make(map[string]interface{})

    if source, ok := sourceKB["config_defaults"].(map[string]interface{}); ok {
        sourceConfigs = source
    }

    if target, ok := targetKB["config_defaults"].(map[string]interface{}); ok {
        targetConfigs = target
    }

    // Analyze each config parameter
    for name, currentValue := range component.Config {
        analysis := &ParameterAnalysis{
            Name:         name,
            Component:    component.Type,
            Type:         ConfigParam,
            CurrentValue: currentValue,
            State:        UseDefault, // Default assumption
        }

        // Get source and target defaults
        if sourceDefault, exists := sourceConfigs[name]; exists {
            analysis.SourceDefault = sourceDefault
        }

        if targetDefault, exists := targetConfigs[name]; exists {
            analysis.TargetDefault = targetDefault
        }

        // Determine if parameter is user-set
        if analysis.SourceDefault != nil && fmt.Sprintf("%v", analysis.SourceDefault) != fmt.Sprintf("%v", currentValue) {
            analysis.State = UserSet
        }

        analyses = append(analyses, analysis)
    }

    return analyses, nil
}
```

### 3.4. Risk Evaluation Logic (risk_evaluator.go)

```go
package analyzer

import (
    "fmt"
)

// RiskEvaluator handles risk evaluation based on parameter analysis
type RiskEvaluator interface {
    Evaluate(analyses []*ParameterAnalysis, sourceKB, targetKB map[string]interface{}) ([]RiskItem, []AuditItem)
}

type riskEvaluator struct{}

// NewRiskEvaluator creates a new risk evaluator instance
func NewRiskEvaluator() RiskEvaluator {
    return &riskEvaluator{}
}

// Evaluate performs risk evaluation based on the risk matrix
func (r *riskEvaluator) Evaluate(analyses []*ParameterAnalysis, sourceKB, targetKB map[string]interface{}) ([]RiskItem, []AuditItem) {
    var risks []RiskItem
    var audits []AuditItem

    // Get forced changes from target knowledge base
    forcedChanges := r.extractForcedChanges(targetKB)

    for _, analysis := range analyses {
        // Check for forced changes (HIGH risk)
        if forcedValue, isForced := forcedChanges[analysis.Name]; isForced {
            risk := RiskItem{
                ParameterName: analysis.Name,
                Component:     analysis.Component,
                RiskLevel:     RiskHigh,
                CurrentValue:  analysis.CurrentValue,
                ForcedValue:   forcedValue,
                Description:   fmt.Sprintf("Parameter %s will be forcibly changed during upgrade from '%v' to '%v'", analysis.Name, analysis.CurrentValue, forcedValue),
            }
            risks = append(risks, risk)
            continue // Skip other evaluations for forced changes
        }

        // Check for default value changes
        if analysis.SourceDefault != nil && analysis.TargetDefault != nil &&
           fmt.Sprintf("%v", analysis.SourceDefault) != fmt.Sprintf("%v", analysis.TargetDefault) {
            
            if analysis.State == UserSet {
                // UserSet + Default Changed = MEDIUM risk
                risk := RiskItem{
                    ParameterName: analysis.Name,
                    Component:     analysis.Component,
                    RiskLevel:     RiskMedium,
                    CurrentValue:  analysis.CurrentValue,
                    DefaultValue:  analysis.SourceDefault,
                    Description:   fmt.Sprintf("Parameter %s has custom value and default is changing in target version", analysis.Name),
                }
                risks = append(risks, risk)
            } else {
                // UseDefault + Default Changed = Audit item
                audit := AuditItem{
                    ParameterName: analysis.Name,
                    Component:     analysis.Component,
                    CurrentValue:  analysis.CurrentValue,
                    TargetDefault: analysis.TargetDefault,
                    Description:   fmt.Sprintf("Default value for parameter %s is changing in target version", analysis.Name),
                }
                audits = append(audits, audit)
            }
        } else if analysis.State == UserSet {
            // UserSet + Default Not Changed = Audit item
            audit := AuditItem{
                ParameterName: analysis.Name,
                Component:     analysis.Component,
                CurrentValue:  analysis.CurrentValue,
                TargetDefault: analysis.TargetDefault,
                Description:   fmt.Sprintf("Parameter %s has custom value", analysis.Name),
            }
            audits = append(audits, audit)
        }
    }

    return risks, audits
}

func (r *riskEvaluator) extractForcedChanges(targetKB map[string]interface{}) map[string]interface{} {
    forcedChanges := make(map[string]interface{})

    // Extract forced changes from upgrade logic if available
    if upgradeLogic, ok := targetKB["upgrade_logic"].([]interface{}); ok {
        for _, change := range upgradeLogic {
            if changeMap, ok := change.(map[string]interface{}); ok {
                if variable, ok := changeMap["variable"].(string); ok {
                    if forcedValue, ok := changeMap["forced_value"]; ok {
                        forcedChanges[variable] = forcedValue
                    }
                }
            }
        }
    }

    return forcedChanges
}
```

## 4. TiUP Integration Considerations

### 4.1. Integration Approach

The analyzer module is designed to be used as a library by TiUP. TiUP should:

1. Collect cluster configuration data using the collector module
2. Load appropriate knowledge base data for source and target versions
3. Call the analyzer APIs to perform risk analysis
4. Process the analysis results and present them to users

### 4.2. Knowledge Base Handling

TiUP will need to provide the following knowledge base data:

- Source version knowledge base (defaults and current values)
- Target version knowledge base (defaults and upgrade logic)

The analyzer expects knowledge base data in the following format:

```json
{
  "system_variables": {
    "tidb_enable_clustered_index": "INT_ONLY",
    "max_connections": "151"
  },
  "config_defaults": {
    "performance.max-procs": 0,
    "log.level": "info"
  },
  "upgrade_logic": [
    {
      "variable": "tidb_enable_clustered_index",
      "forced_value": "ON",
      "type": "set_global"
    }
  ]
}
```

### 4.3. Error Handling

The analyzer is designed to handle various error conditions:

- Missing knowledge base data (will use sensible defaults)
- Malformed knowledge base data (will return specific error messages)
- Incomplete cluster snapshots (will analyze available data)
- Unexpected parameter types (will attempt best-effort analysis)

### 4.4. Data Format

The analyzer consumes data from the collector and produces results in a standardized format that can be:

- Serialized to JSON for file storage or transmission
- Used directly in-memory by the reporter
- Extended in the future without breaking existing integrations

## 5. Risk Assessment Matrix Implementation

### 5.1. Matrix Definition

The analyzer implements the following risk assessment matrix:

| Source State | Target State      | Risk Level | Action        |
|--------------|-------------------|------------|---------------|
| UseDefault   | Default Changed   | LOW        | Configuration Audit |
| UseDefault   | Forced Upgrade    | HIGH       | Must Handle   |
| UserSet      | Default Changed   | MEDIUM     | Recommendation |
| UserSet      | Forced Upgrade    | HIGH       | Must Handle   |

### 5.2. Implementation Details

1. **Parameter State Determination**
   - Compare current value with source version default
   - Classify as UseDefault or UserSet

2. **Target State Determination**
   - Compare source and target defaults
   - Check for forced upgrade conditions in upgrade logic

3. **Risk Classification**
   - Apply risk matrix to determine risk level
   - Generate appropriate risk items or audit items

## 6. Testing Plan

### 6.1. Unit Tests

- Test parameter state determination logic
- Test risk evaluation with various scenarios
- Test knowledge base data handling
- Test error conditions and edge cases

### 6.2. Integration Tests

- Test end-to-end analysis with real knowledge base data
- Test various combinations of parameter states
- Test forced upgrade detection

### 6.3. Test Data

Prepare test data covering:

- Parameters with default values that change in target version
- Parameters with user-set values that change in target version
- Parameters with user-set values that are forcibly changed during upgrade
- Parameters with user-set values that remain unchanged
- Mixed scenarios with multiple parameter types