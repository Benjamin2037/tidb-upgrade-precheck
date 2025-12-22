# Custom Rules Development Guide

This guide explains how to create custom rules for the TiDB upgrade precheck system.

## Overview

The rule system is designed around a simple concept: **each rule compares source cluster sampled values vs target cluster target values**. 

**Important**: Parameters are preprocessed before rule evaluation. Deployment-specific parameters (paths, hostnames, etc.) and resource-dependent parameters are filtered out in the preprocessing stage. Rules receive cleaned defaults maps and only need to focus on core comparison logic.

## Architecture

### Preprocessing Stage

Before rules are evaluated, the analyzer runs a preprocessing stage that:
1. **Filters deployment-specific parameters**: Path parameters, host/network parameters, etc.
2. **Filters resource-dependent parameters**: Parameters auto-tuned by system (if source default == target default)
3. **Filters identical parameters**: Parameters where current == source == target (no difference)
4. **Removes filtered parameters** from `sourceDefaults` and `targetDefaults` maps
5. **Generates CheckResults** for filtered parameters (for reporting)

**Result**: Rules receive cleaned defaults maps containing only parameters that need actual comparison.

### Rule Evaluation

Rules receive:
- **Cleaned defaults**: Only parameters that passed preprocessing
- **Current values**: From cluster snapshot
- **Upgrade logic**: Forced changes and special handling metadata
- **Parameter notes**: Special notes for parameters

Rules focus on:
- **Core comparison logic**: Compare current vs target defaults
- **Forced changes**: Check if parameters will be forcibly changed
- **Special handling**: Use metadata from upgrade logic and parameter notes

## Rule Interface

All rules must implement the `Rule` interface:

```go
type Rule interface {
    Name() string
    Description() string
    Category() string
    DataRequirements() DataSourceRequirement
    Evaluate(ctx context.Context, ruleCtx *RuleContext) ([]CheckResult, error)
}
```

### DataRequirements

Each rule must declare what data it needs:

```go
type DataSourceRequirement struct {
    SourceClusterRequirements struct {
        Components          []string
        NeedConfig          bool
        NeedSystemVariables bool
        NeedAllTikvNodes    bool  // For consistency checks
    }
    SourceKBRequirements struct {
        Components          []string
        NeedConfigDefaults  bool
        NeedSystemVariables bool
        NeedUpgradeLogic    bool
    }
    TargetKBRequirements struct {
        Components          []string
        NeedConfigDefaults  bool
        NeedSystemVariables bool
        NeedUpgradeLogic    bool  // For forced changes
    }
}
```

## RuleContext

The `RuleContext` provides all the data needed for rule evaluation:

```go
type RuleContext struct {
    // SourceClusterSnapshot: Actual sampled data from running cluster
    SourceClusterSnapshot *collector.ClusterSnapshot
    
    // SourceVersion: Current cluster version
    SourceVersion string
    
    // TargetVersion: Target version for upgrade
    TargetVersion string
    
    // SourceDefaults: Default values for source version (CLEANED - filtered parameters removed)
    SourceDefaults map[string]map[string]interface{}
    
    // TargetDefaults: Default values for target version (CLEANED - filtered parameters removed)
    TargetDefaults map[string]map[string]interface{}
    
    // UpgradeLogic: Forced changes and upgrade logic
    UpgradeLogic map[string]interface{}
    
    // ParameterNotes: Special notes for parameters
    ParameterNotes map[string]interface{}
}
```

## Helper Methods

The `RuleContext` provides several helper methods:

- `GetSourceDefault(component, paramName)`: Get default value for source version
- `GetTargetDefault(component, paramName)`: Get default value for target version
- `GetForcedChangeMetadata(component, paramName, currentValue)`: Get forced change metadata
- `GetParameterNote(component, paramName, paramType, targetDefault)`: Get special note for parameter

**Note**: To get current runtime values, access `SourceClusterSnapshot.Components[componentName].Config` or `SourceClusterSnapshot.Components[componentName].Variables` directly.

## Example: Parameter Default Change Rule

```go
package myrules

import (
    "context"
    "fmt"
    "github.com/pingcap/tidb-upgrade-precheck/pkg/analyzer/rules"
    "github.com/pingcap/tidb-upgrade-precheck/pkg/collector"
)

type MyParameterRule struct {
    *rules.BaseRule
    component string
}

func NewMyParameterRule(component string) rules.Rule {
    return &MyParameterRule{
        BaseRule: rules.NewBaseRule(
            "MY_PARAM_RULE",
            "Check for parameter changes",
            "upgrade_difference",
        ),
        component: component,
    }
}

func (r *MyParameterRule) DataRequirements() rules.DataSourceRequirement {
    return rules.DataSourceRequirement{
        SourceClusterRequirements: struct {
            Components          []string
            NeedConfig          bool
            NeedSystemVariables bool
            NeedAllTikvNodes    bool
        }{
            Components: []string{r.component},
            NeedConfig: true,
        },
        TargetKBRequirements: struct {
            Components          []string
            NeedConfigDefaults  bool
            NeedSystemVariables bool
            NeedUpgradeLogic    bool
        }{
            Components:         []string{r.component},
            NeedConfigDefaults: true,
            NeedUpgradeLogic:   true,
        },
    }
}

func (r *MyParameterRule) Evaluate(ctx context.Context, ruleCtx *rules.RuleContext) ([]rules.CheckResult, error) {
    var results []rules.CheckResult
    
    // Get current runtime value
    var currentValue interface{}
    if snapshot := ruleCtx.SourceClusterSnapshot; snapshot != nil {
        if comp, ok := snapshot.Components[r.component]; ok {
            if paramValue, ok := comp.Config["my_param"]; ok {
                currentValue = paramValue.Value
            }
        }
    }
    
    // Get target default (from cleaned defaults - filtered parameters already removed)
    targetDefault := ruleCtx.GetTargetDefault(r.component, "my_param")
    
    // Check if target default is nil (should not happen after preprocessing, but check anyway)
    if targetDefault == nil {
        return nil, fmt.Errorf("targetDefault for parameter my_param in component %s is nil", r.component)
    }
    
    // Check if current value differs from target default
    if currentValue != nil {
        if !rules.CompareValues(currentValue, targetDefault) {
            // Check for forced change
            forcedMetadata := ruleCtx.GetForcedChangeMetadata(r.component, "my_param", currentValue)
            if forcedMetadata != nil {
                // Parameter will be forcibly changed
                results = append(results, rules.CheckResult{
                    RuleID:        r.Name(),
                    Category:      r.Category(),
                    Component:     r.component,
                    ParameterName: "my_param",
                    ParamType:     "config",
                    Severity:      "warning",
                    RiskLevel:     rules.RiskLevelMedium,
                    Message:       fmt.Sprintf("Parameter my_param will be forcibly changed during upgrade"),
                    Details:       fmt.Sprintf("Current: %v, Target: %v, Forced: %v", 
                        currentValue, targetDefault, forcedMetadata.ForcedValue),
                    CurrentValue:  currentValue,
                    TargetDefault: targetDefault,
                    ForcedValue:   forcedMetadata.ForcedValue,
                })
            } else {
                // Regular difference
                results = append(results, rules.CheckResult{
                    RuleID:        r.Name(),
                    Category:      r.Category(),
                    Component:     r.component,
                    ParameterName: "my_param",
                    ParamType:     "config",
                    Severity:      "info",
                    RiskLevel:     rules.RiskLevelLow,
                    Message:       fmt.Sprintf("Parameter my_param differs from target default"),
                    Details:       fmt.Sprintf("Current: %v, Target: %v", currentValue, targetDefault),
                    CurrentValue:  currentValue,
                    TargetDefault: targetDefault,
                })
            }
        }
    }
    
    return results, nil
}
```

## Key Points for Rule Development

### 1. No Filtering Needed

**Don't** check for path parameters, deployment-specific parameters, etc. These are already filtered in preprocessing:

```go
// ❌ DON'T DO THIS
if analyzer.IsPathParameter(paramName) {
    continue
}

// ✅ Rules receive cleaned defaults - just process them
for paramName, targetDefault := range targetDefaults {
    // Process parameter
}
```

### 2. Error Handling

Always check for nil values (though preprocessing should ensure they exist):

```go
targetDefault := ruleCtx.GetTargetDefault(component, paramName)
if targetDefault == nil {
    return nil, fmt.Errorf("targetDefault for parameter %s is nil", paramName)
}
```

### 3. Use Helper Methods

Use `GetForcedChangeMetadata` and `GetParameterNote` for special handling:

```go
// Check for forced change
forcedMetadata := ruleCtx.GetForcedChangeMetadata(component, paramName, currentValue)
if forcedMetadata != nil {
    // Handle forced change
}

// Get special note
note := ruleCtx.GetParameterNote(component, paramName, "config", targetDefault)
if note != "" {
    // Include note in details
}
```

### 4. Comparison Utilities

Use comparison utilities from `rules` package:

```go
import "github.com/pingcap/tidb-upgrade-precheck/pkg/analyzer/rules"

// Compare values
if !rules.CompareValues(currentValue, targetDefault) {
    // Values differ
}

// Format values
details := rules.FormatValueDiff(currentValue, targetDefault)

// For map types, use deep comparison
diffs := rules.CompareMapsDeep(currentValue, targetDefault, rules.CompareOptions{
    BasePath: paramName,
})
```

## Rule Categories

### 1. Upgrade Difference Rules
- Compare current vs target defaults
- Check for forced changes
- Category: `"upgrade_difference"`

### 2. User Modification Rules
- Detect user-modified parameters
- Category: `"user_modified"`

### 3. Consistency Rules
- Check parameter consistency across nodes
- Category: `"consistency"`

### 4. High Risk Rules
- Check for high-risk configurations
- Category: `"high_risk"`

## Best Practices

1. **Use BaseRule**: Embed `*rules.BaseRule` to reduce boilerplate
2. **Declare Data Requirements**: Always declare what data your rule needs
3. **Handle Errors**: Check for nil values and return errors for unexpected failures
4. **Provide Suggestions**: Always include actionable suggestions in CheckResult
5. **Set Appropriate Severity**: Use "info", "warning", "error", or "critical"
6. **Use Helper Methods**: Use `GetForcedChangeMetadata` and `GetParameterNote` for special handling
7. **Don't Filter**: Don't check for path parameters, deployment-specific parameters, etc. (already filtered)

## Testing

Create test files for your custom rules:

```go
func TestMyParameterRule(t *testing.T) {
    rule := NewMyParameterRule("tidb")
    
    // Create mock rule context with cleaned defaults
    ruleCtx := &rules.RuleContext{
        SourceClusterSnapshot: &collector.ClusterSnapshot{
            Components: map[string]collector.ComponentState{
                "tidb": {
                    Config: types.ConfigDefaults{
                        "my_param": types.ParameterValue{Value: 100, Type: "int"},
                    },
                },
            },
        },
        TargetDefaults: map[string]map[string]interface{}{
            "tidb": {
                "my_param": 200, // Cleaned defaults (filtered parameters removed)
            },
        },
    }
    
    // Evaluate
    results, err := rule.Evaluate(context.Background(), ruleCtx)
    
    // Assert results
    assert.NoError(t, err)
    assert.NotEmpty(t, results)
}
```

## Migration from Old Rules

If you're migrating from an older version:

1. **Remove Filtering Logic**: Remove all `IsPathParameter`, `IsIgnoredParameter` checks
2. **Simplify Comparison**: Rules now only compare current vs target (source comparison removed)
3. **Use Cleaned Defaults**: Rules receive cleaned defaults (filtered parameters already removed)
4. **Add Error Checks**: Add checks for nil `targetDefault` and `currentValue`
5. **Use Helper Methods**: Use `GetForcedChangeMetadata` and `GetParameterNote` instead of hardcoded special handling
