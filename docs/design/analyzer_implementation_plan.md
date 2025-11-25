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
    HighRiskItems  []RiskItem   `json:"high_risk_items"`
    MediumRiskItems []RiskItem  `json:"medium_risk_items"`
    LowRiskItems   []RiskItem   `json:"low_risk_items"`
    AuditItems     []AuditItem  `json:"audit_items"`
    ParameterAnalyses []ParameterAnalysis `json:"parameter_analyses"`
}

// KnowledgeBase represents the knowledge base data
type KnowledgeBase struct {
    Version          string                           `json:"version"`
    BootstrapVersion int                              `json:"bootstrap_version"`
    Components       map[string]ComponentKnowledgeBase `json:"components"`
}

// ComponentKnowledgeBase represents knowledge base for a single component
type ComponentKnowledgeBase struct {
    Config    map[string]interface{} `json:"config"`
    Variables map[string]interface{} `json:"variables"`
}

// UpgradeLogic represents forced changes in upgrade process
type UpgradeLogic struct {
    Version  int    `json:"version"`
    Function string `json:"function"`
    Changes  []struct {
        Type        string `json:"type"`
        Variable    string `json:"variable,omitempty"`
        ForcedValue string `json:"forced_value,omitempty"`
        Description string `json:"description,omitempty"`
    } `json:"changes"`
}

// Analyzer defines the interface for analyzing cluster configurations
type Analyzer interface {
    Analyze(snapshot *collector.ClusterSnapshot, sourceKB, targetKB *KnowledgeBase, upgradeLogic []UpgradeLogic) (*AnalysisResult, error)
}
```

### 3.2. Main Analyzer Interface (analyzer.go)

```go
package analyzer

import (
    "fmt"
    "time"
    
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

// Analyze performs analysis of cluster configuration against knowledge base
func (a *analyzer) Analyze(
    snapshot *collector.ClusterSnapshot, 
    sourceKB, targetKB *KnowledgeBase, 
    upgradeLogic []UpgradeLogic,
) (*AnalysisResult, error) {
    result := &AnalysisResult{
        Timestamp:     time.Now(),
        SourceVersion: sourceKB.Version,
        TargetVersion: targetKB.Version,
    }

    // Perform parameter analysis
    analyses, err := a.comparator.Compare(snapshot, sourceKB, targetKB, upgradeLogic)
    if err != nil {
        return nil, fmt.Errorf("failed to compare configurations: %w", err)
    }
    result.ParameterAnalyses = analyses

    // Evaluate risks
    risks, audits := a.riskEvaluator.Evaluate(analyses)
    result.HighRiskItems = risks[RiskHigh]
    result.MediumRiskItems = risks[RiskMedium]
    result.LowRiskItems = risks[RiskLow]
    result.AuditItems = audits

    return result, nil
}
```

### 3.3. Comparator Logic (comparator.go)

```go
package analyzer

import (
    "fmt"
    "reflect"
    
    "github.com/pingcap/tidb-upgrade-precheck/pkg/collector"
)

// Comparator handles comparison of current configuration with knowledge base
type Comparator interface {
    Compare(
        snapshot *collector.ClusterSnapshot,
        sourceKB, targetKB *KnowledgeBase,
        upgradeLogic []UpgradeLogic,
    ) ([]ParameterAnalysis, error)
}

type comparator struct{}

// NewComparator creates a new comparator instance
func NewComparator() Comparator {
    return &comparator{}
}

// Compare performs comparison of current configuration with knowledge base data
func (c *comparator) Compare(
    snapshot *collector.ClusterSnapshot,
    sourceKB, targetKB *KnowledgeBase,
    upgradeLogic []UpgradeLogic,
) ([]ParameterAnalysis, error) {
    var analyses []ParameterAnalysis

    // Process each component in the snapshot
    for componentName, componentState := range snapshot.Components {
        componentType := componentState.Type
        
        // Process configuration parameters
        configAnalyses, err := c.compareConfigParameters(
            componentName, componentType, componentState.Config,
            sourceKB, targetKB, upgradeLogic)
        if err != nil {
            return nil, fmt.Errorf("failed to compare config parameters for %s: %w", componentName, err)
        }
        analyses = append(analyses, configAnalyses...)

        // Process system variables (for TiDB)
        if componentType == "tidb" {
            variableAnalyses, err := c.compareSystemVariables(
                componentName, componentState.Variables,
                sourceKB, targetKB, upgradeLogic)
            if err != nil {
                return nil, fmt.Errorf("failed to compare system variables for %s: %w", componentName, err)
            }
            analyses = append(analyses, variableAnalyses...)
        }
    }

    return analyses, nil
}

func (c *comparator) compareConfigParameters(
    componentName, componentType string,
    currentConfig map[string]interface{},
    sourceKB, targetKB *KnowledgeBase,
    upgradeLogic []UpgradeLogic,
) ([]ParameterAnalysis, error) {
    var analyses []ParameterAnalysis

    sourceComponent, sourceExists := sourceKB.Components[componentType]
    targetComponent, targetExists := targetKB.Components[componentType]

    // Compare each configuration parameter
    for paramName, currentValue := range currentConfig {
        analysis := ParameterAnalysis{
            Name:         paramName,
            Component:    componentName,
            Type:         ConfigParam,
            CurrentValue: currentValue,
        }

        // Get source and target defaults
        if sourceExists {
            if sourceVal, ok := sourceComponent.Config[paramName]; ok {
                analysis.SourceDefault = sourceVal
            }
        }

        if targetExists {
            if targetVal, ok := targetComponent.Config[paramName]; ok {
                analysis.TargetDefault = targetVal
            }
        }

        // Determine parameter state
        analysis.State = c.determineParameterState(currentValue, analysis.SourceDefault)

        // Check if parameter is forcibly changed during upgrade
        forcedValue := c.checkForcedChange(componentType, paramName, upgradeLogic)
        if forcedValue != nil {
            analysis.IsForced = true
            analysis.ForcedValue = forcedValue
        }

        analyses = append(analyses, analysis)
    }

    return analyses, nil
}

func (c *comparator) compareSystemVariables(
    componentName string,
    currentVariables map[string]string,
    sourceKB, targetKB *KnowledgeBase,
    upgradeLogic []UpgradeLogic,
) ([]ParameterAnalysis, error) {
    var analyses []ParameterAnalysis

    sourceComponent, sourceExists := sourceKB.Components["tidb"]
    targetComponent, targetExists := targetKB.Components["tidb"]

    // Compare each system variable
    for varName, currentValue := range currentVariables {
        analysis := ParameterAnalysis{
            Name:         varName,
            Component:    componentName,
            Type:         VariableParam,
            CurrentValue: currentValue,
        }

        // Get source and target defaults
        if sourceExists {
            if sourceVal, ok := sourceComponent.Variables[varName]; ok {
                analysis.SourceDefault = sourceVal
            }
        }

        if targetExists {
            if targetVal, ok := targetComponent.Variables[varName]; ok {
                analysis.TargetDefault = targetVal
            }
        }

        // Determine parameter state
        analysis.State = c.determineParameterState(currentValue, analysis.SourceDefault)

        // Check if variable is forcibly changed during upgrade
        forcedValue := c.checkForcedChange("tidb", varName, upgradeLogic)
        if forcedValue != nil {
            analysis.IsForced = true
            analysis.ForcedValue = forcedValue
        }

        analyses = append(analyses, analysis)
    }

    return analyses, nil
}

func (c *comparator) determineParameterState(currentValue, defaultValue interface{}) ParameterState {
    if isDefaultValue(currentValue, defaultValue) {
        return UseDefault
    }
    return UserSet
}

func (c *comparator) checkForcedChange(componentType, paramName string, upgradeLogic []UpgradeLogic) interface{} {
    // Check upgrade logic for forced changes to this parameter
    for _, logic := range upgradeLogic {
        for _, change := range logic.Changes {
            if change.Type == "variable_change" && change.Variable == paramName {
                return change.ForcedValue
            }
        }
    }
    return nil
}

// isDefaultValue checks if current value equals default value
func isDefaultValue(currentValue, defaultValue interface{}) bool {
    if currentValue == nil && defaultValue == nil {
        return true
    }
    
    if currentValue == nil || defaultValue == nil {
        return false
    }
    
    // Handle string comparisons
    currentStr, isCurrentStr := currentValue.(string)
    defaultStr, isDefaultStr := defaultValue.(string)
    if isCurrentStr && isDefaultStr {
        return currentStr == defaultStr
    }
    
    // Use reflection for other types
    return reflect.DeepEqual(currentValue, defaultValue)
}
```

### 3.4. Risk Evaluator (risk_evaluator.go)

```go
package analyzer

// RiskEvaluator handles risk evaluation based on parameter analysis
type RiskEvaluator interface {
    Evaluate(analyses []ParameterAnalysis) (map[RiskLevel][]RiskItem, []AuditItem)
}

type riskEvaluator struct{}

// NewRiskEvaluator creates a new risk evaluator instance
func NewRiskEvaluator() RiskEvaluator {
    return &riskEvaluator{}
}

// Evaluate performs risk evaluation based on parameter analyses
func (r *riskEvaluator) Evaluate(analyses []ParameterAnalysis) (map[RiskLevel][]RiskItem, []AuditItem) {
    risks := make(map[RiskLevel][]RiskItem)
    var audits []AuditItem

    for _, analysis := range analyses {
        // Check for high risk items (forced changes)
        if analysis.IsForced {
            risk := r.createForcedChangeRisk(analysis)
            risks[RiskHigh] = append(risks[RiskHigh], risk)
            continue
        }

        // Check for medium risk items (default value changes)
        if analysis.State == UseDefault && !isValueEqual(analysis.SourceDefault, analysis.TargetDefault) {
            risk := r.createDefaultValueChangeRisk(analysis)
            risks[RiskMedium] = append(risks[RiskMedium], risk)
            continue
        }

        // Check for audit items (user-set values that differ from target defaults)
        if analysis.State == UserSet && !isValueEqual(analysis.CurrentValue, analysis.TargetDefault) {
            audit := r.createAuditItem(analysis)
            audits = append(audits, audit)
        }
    }

    return risks, audits
}

func (r *riskEvaluator) createForcedChangeRisk(analysis ParameterAnalysis) RiskItem {
    return RiskItem{
        ParameterName: analysis.Name,
        Component:     analysis.Component,
        RiskLevel:     RiskHigh,
        CurrentValue:  analysis.CurrentValue,
        ForcedValue:   analysis.ForcedValue,
        Description:   "This parameter will be forcibly changed during upgrade regardless of current value",
        Suggestions: []string{
            "Review the impact of the forced change",
            "Plan for potential service disruption during upgrade",
        },
    }
}

func (r *riskEvaluator) createDefaultValueChangeRisk(analysis ParameterAnalysis) RiskItem {
    return RiskItem{
        ParameterName: analysis.Name,
        Component:     analysis.Component,
        RiskLevel:     RiskMedium,
        CurrentValue:  analysis.CurrentValue,
        DefaultValue:  analysis.TargetDefault,
        Description:   "The default value for this parameter will change in the target version",
        Suggestions: []string{
            "Consider whether to explicitly set this parameter to the new default value",
            "Review the impact of the default value change",
        },
    }
}

func (r *riskEvaluator) createAuditItem(analysis ParameterAnalysis) AuditItem {
    return AuditItem{
        ParameterName: analysis.Name,
        Component:     analysis.Component,
        CurrentValue:  analysis.CurrentValue,
        TargetDefault: analysis.TargetDefault,
        Description:   "User-modified parameter that differs from target version default",
    }
}

// isValueEqual checks if two values are equal
func isValueEqual(val1, val2 interface{}) bool {
    if val1 == nil && val2 == nil {
        return true
    }
    
    if val1 == nil || val2 == nil {
        return false
    }
    
    // Handle string comparisons
    str1, isStr1 := val1.(string)
    str2, isStr2 := val2.(string)
    if isStr1 && isStr2 {
        return str1 == str2
    }
    
    // For other types, convert to string for comparison
    return fmt.Sprintf("%v", val1) == fmt.Sprintf("%v", val2)
}
```

## 4. Usage Example

```go
package main

import (
    "encoding/json"
    "fmt"
    "io/ioutil"
    "os"
    
    "github.com/pingcap/tidb-upgrade-precheck/pkg/analyzer"
    "github.com/pingcap/tidb-upgrade-precheck/pkg/collector"
)

func main() {
    // Load cluster snapshot
    snapshotData, err := ioutil.ReadFile("cluster_snapshot.json")
    if err != nil {
        fmt.Printf("Error reading snapshot file: %v\n", err)
        os.Exit(1)
    }
    
    var snapshot collector.ClusterSnapshot
    if err := json.Unmarshal(snapshotData, &snapshot); err != nil {
        fmt.Printf("Error unmarshaling snapshot: %v\n", err)
        os.Exit(1)
    }

    // Load knowledge base files
    sourceKB, err := loadKnowledgeBase("knowledge/source_defaults.json")
    if err != nil {
        fmt.Printf("Error loading source knowledge base: %v\n", err)
        os.Exit(1)
    }
    
    targetKB, err := loadKnowledgeBase("knowledge/target_defaults.json")
    if err != nil {
        fmt.Printf("Error loading target knowledge base: %v\n", err)
        os.Exit(1)
    }
    
    upgradeLogic, err := loadUpgradeLogic("knowledge/upgrade_logic.json")
    if err != nil {
        fmt.Printf("Error loading upgrade logic: %v\n", err)
        os.Exit(1)
    }

    // Create analyzer
    a := analyzer.NewAnalyzer()

    // Perform analysis
    result, err := a.Analyze(&snapshot, sourceKB, targetKB, upgradeLogic)
    if err != nil {
        fmt.Printf("Error analyzing cluster: %v\n", err)
        os.Exit(1)
    }

    // Output results
    data, err := json.MarshalIndent(result, "", "  ")
    if err != nil {
        fmt.Printf("Error marshaling result: %v\n", err)
        os.Exit(1)
    }

    fmt.Println(string(data))
}

func loadKnowledgeBase(filename string) (*analyzer.KnowledgeBase, error) {
    data, err := ioutil.ReadFile(filename)
    if err != nil {
        return nil, err
    }
    
    var kb analyzer.KnowledgeBase
    if err := json.Unmarshal(data, &kb); err != nil {
        return nil, err
    }
    
    return &kb, nil
}

func loadUpgradeLogic(filename string) ([]analyzer.UpgradeLogic, error) {
    data, err := ioutil.ReadFile(filename)
    if err != nil {
        return nil, err
    }
    
    var logic []analyzer.UpgradeLogic
    if err := json.Unmarshal(data, &logic); err != nil {
        return nil, err
    }
    
    return logic, nil
}
```

## 5. Implementation Considerations

### 5.1. Data Type Handling
- Proper comparison of different data types (string, int, bool, etc.)
- Handling of nil values
- Type conversion when necessary

### 5.2. Performance Optimization
- Efficient data structures for comparisons
- Caching of knowledge base data
- Parallel processing where applicable

### 5.3. Error Handling
- Graceful handling of missing knowledge base data
- Clear error messages for troubleshooting
- Validation of input data

### 5.4. Extensibility
- Modular design for easy addition of new risk types
- Configurable risk evaluation rules
- Plugin architecture for custom risk evaluators

### 5.5. Testing
- Unit tests for each component
- Integration tests for end-to-end functionality
- Test cases for edge cases and error conditions