# Custom Rules Development Guide

This guide explains how to create custom rules for the TiDB upgrade precheck system.

## Overview

The rule system is designed around a simple concept: **each rule compares source cluster sampled values vs target cluster target values**.

### Core Concept

Every rule performs comparisons between:
- **Source Sampled Values**: Actual values collected from the running cluster (what is currently configured)
- **Source Defaults**: Default values for the source version (from knowledge base)
- **Target Defaults**: Default values for the target version (what will be after upgrade)
- **Target Simulation Results**: Simulated results from target version (query plans, performance predictions, etc.)

### Key Comparisons

1. **Parameter/System Variable Default Changes**: Compare source defaults vs target defaults
2. **User Modifications**: Check if user has modified values (sampled value != source default)
3. **Forced Changes**: Identify parameters that will be forcibly changed during upgrade
4. **Query Plan Changes**: Compare query execution plans between source and target versions

### Key Concepts

1. **Source Sampled Values**: Actual values collected from the running cluster (what is currently configured)
2. **Source Defaults**: Default values for the source version (from knowledge base)
3. **Target Defaults**: Default values for the target version (what will be after upgrade)
4. **Target Simulation Results**: Simulated results from target version (query plans, performance predictions, etc.)

## Rule Interface

All rules must implement the `Rule` interface:

```go
type Rule interface {
    Name() string
    Description() string
    Evaluate(ctx context.Context, ruleCtx *RuleContext) ([]CheckResult, error)
}
```

## RuleContext

The `RuleContext` provides all the data needed for rule evaluation:

```go
type RuleContext struct {
    // SourceClusterSnapshot: Actual sampled data from running cluster
    SourceClusterSnapshot *types.ClusterSnapshot
    
    // SourceDefaults: Default values for source version
    SourceDefaults map[string]map[string]interface{}
    
    // TargetDefaults: Default values for target version
    TargetDefaults map[string]map[string]interface{}
    
    // SourceUpgradeLogic: Upgrade logic for source version
    SourceUpgradeLogic map[string]interface{}
    
    // TargetUpgradeLogic: Forced changes and upgrade logic for target version
    TargetUpgradeLogic map[string]interface{}
    
    // TargetSimulationResults: Simulated results (query plans, etc.)
    TargetSimulationResults map[string]map[string]interface{}
}
```

## Helper Methods

The `RuleContext` provides several helper methods:

- `GetSampledValue(component, paramName)`: Get actual value from running cluster
- `GetSourceDefault(component, paramName)`: Get default value for source version
- `GetTargetDefault(component, paramName)`: Get default value for target version
- `IsUserModified(component, paramName)`: Check if user modified the parameter
- `WillDefaultChange(component, paramName)`: Check if default will change
- `WillBeForced(component, paramName)`: Check if parameter will be forcibly changed
- `GetQueryPlanSource(query)`: Get query plan from source cluster
- `GetQueryPlanTarget(query)`: Get simulated query plan for target version

## Example: Parameter Default Change Rule

```go
package myrules

import (
    "context"
    "fmt"
    "github.com/pingcap/tidb-upgrade-precheck/pkg/rules"
)

type MyParameterRule struct {
    *rules.BaseRule
    component string
}

func NewMyParameterRule(component string) *MyParameterRule {
    return &MyParameterRule{
        BaseRule: rules.NewBaseRule(
            "MY_PARAM_RULE",
            "Check for parameter changes",
        ),
        component: component,
    }
}

func (r *MyParameterRule) Evaluate(ctx context.Context, ruleCtx *rules.RuleContext) ([]rules.CheckResult, error) {
    var results []rules.CheckResult
    
    // Get sampled value (what is currently configured)
    sampledValue := ruleCtx.GetSampledValue(r.component, "my_param")
    
    // Get source default (what it should be in source version)
    sourceDefault := ruleCtx.GetSourceDefault(r.component, "my_param")
    
    // Get target default (what it will be after upgrade)
    targetDefault := ruleCtx.GetTargetDefault(r.component, "my_param")
    
    // Check if default will change
    if ruleCtx.WillDefaultChange(r.component, "my_param") {
        // Check if user has modified the value
        if ruleCtx.IsUserModified(r.component, "my_param") {
            results = append(results, rules.CheckResult{
                RuleID:      r.Name(),
                Description: r.Description(),
                Severity:    "warning",
                Message:     fmt.Sprintf("Parameter my_param default will change and user has modified it"),
                Details:     fmt.Sprintf("Current: %v, Source default: %v, Target default: %v", 
                    sampledValue, sourceDefault, targetDefault),
                Suggestions: []string{
                    "Review the parameter change",
                    "Test with new default value",
                },
            })
        }
    }
    
    return results, nil
}
```

## Example: Query Plan Comparison Rule

```go
type MyQueryPlanRule struct {
    *rules.BaseRule
    queries []string
}

func (r *MyQueryPlanRule) Evaluate(ctx context.Context, ruleCtx *rules.RuleContext) ([]rules.CheckResult, error) {
    var results []rules.CheckResult
    
    for _, query := range r.queries {
        // Get query plan from source cluster
        sourcePlan := ruleCtx.GetQueryPlanSource(query)
        
        // Get simulated query plan for target version
        targetPlan := ruleCtx.GetQueryPlanTarget(query)
        
        // Compare plans
        if sourcePlan != nil && targetPlan != nil {
            if !plansEqual(sourcePlan, targetPlan) {
                results = append(results, rules.CheckResult{
                    RuleID:      r.Name(),
                    Description: r.Description(),
                    Severity:    "warning",
                    Message:     fmt.Sprintf("Query plan will change for: %s", query),
                    Details:     fmt.Sprintf("Source plan: %v, Target plan: %v", sourcePlan, targetPlan),
                })
            }
        }
    }
    
    return results, nil
}
```

## Using Custom Rules

### Option 1: Using RuleRunner

```go
import (
    "github.com/pingcap/tidb-upgrade-precheck/pkg/rules"
    myrules "your-package/rules"
)

// Create custom rules
customRules := []rules.Rule{
    myrules.NewMyParameterRule("tidb"),
    myrules.NewMyQueryPlanRule([]string{"SELECT * FROM t1"}),
}

// Create rule context
ruleCtx := rules.NewRuleContext(
    snapshot,
    sourceVersion, targetVersion,
    sourceDefaults, targetDefaults,
    sourceUpgradeLogic, targetUpgradeLogic,
)

// Run rules
runner := rules.NewRuleRunner(customRules)
results, err := runner.Run(ctx, ruleCtx)
```

### Option 2: Using Analyzer

```go
import (
    "github.com/pingcap/tidb-upgrade-precheck/pkg/precheck"
    "github.com/pingcap/tidb-upgrade-precheck/pkg/rules"
)

// Create analyzer
analyzer := precheck.NewAnalyzer()

// Create rule context
ruleCtx := precheck.CreateRuleContextFromKB(
    snapshot,
    sourceVersion, targetVersion,
    sourceKB, targetKB,
)

// Create custom rules
customRules := []rules.Rule{
    myrules.NewMyParameterRule("tidb"),
}

// Analyze with custom rules
report, err := analyzer.AnalyzeWithRules(ctx, snapshot, targetVersion, customRules, ruleCtx)
```

## Rule Categories

### 1. Parameter Default Change Rules
- Compare source defaults vs target defaults
- Check if user-modified values will conflict
- Identify forced changes

### 2. System Variable Rules
- Check for forced system variable changes
- Compare system variable defaults
- Identify deprecated variables

### 3. Query Plan Rules
- Compare query execution plans
- Detect potential performance regressions
- Identify plan changes that might affect workload

### 4. Configuration Rules
- Check configuration consistency across instances
- Identify deprecated configuration options
- Validate configuration compatibility

## Best Practices

1. **Use BaseRule**: Embed `*rules.BaseRule` to reduce boilerplate
2. **Provide Suggestions**: Always include actionable suggestions in CheckResult
3. **Set Appropriate Severity**: Use "info", "warning", "error", or "critical"
4. **Handle Errors Gracefully**: Return errors only for unexpected failures
5. **Document Your Rules**: Add clear descriptions of what each rule checks

## Testing

Create test files for your custom rules:

```go
func TestMyParameterRule(t *testing.T) {
    // Create mock rule context
    ruleCtx := &rules.RuleContext{
        // ... setup context
    }
    
    // Create rule
    rule := NewMyParameterRule("tidb")
    
    // Evaluate
    results, err := rule.Evaluate(context.Background(), ruleCtx)
    
    // Assert results
    assert.NoError(t, err)
    assert.NotEmpty(t, results)
}
```

