# Analyzer Rules Documentation

This directory contains detailed documentation for each analyzer rule in the `tidb-upgrade-precheck` system.

## Overview

The analyzer uses a rule-based architecture to perform various checks on TiDB cluster configurations and parameters. Each rule is an independent module that:

- Defines its data requirements (what cluster data and knowledge base data it needs)
- Implements evaluation logic (how it checks for issues)
- Returns standardized `CheckResult` objects (what issues it found)

## Available Rules

### 1. User Modified Parameters Rule

**File**: [`user_modified_params_rule.md`](./user_modified_params_rule.md)

Detects parameters that have been modified by the user from source version defaults.

- **Purpose**: Identify user customizations
- **Risk Level**: Low (Info)
- **Components**: TiDB, PD, TiKV, TiFlash

### 2. Upgrade Differences Rule

**File**: [`upgrade_differences_rule.md`](./upgrade_differences_rule.md)

Detects parameters that will differ after upgrade, including forced changes from upgrade logic.

- **Purpose**: Identify upgrade-related parameter changes
- **Risk Levels**: High (Error), Medium (Warning), Low (Info)
- **Components**: TiDB, PD, TiKV, TiFlash

### 3. TiKV Consistency Rule

**File**: [`tikv_consistency_rule.md`](./tikv_consistency_rule.md)

Compares all TiKV node parameters with source version knowledge base defaults.

- **Purpose**: Identify TiKV parameters that differ from defaults
- **Risk Level**: Medium (Warning)
- **Components**: TiKV only

### 4. High Risk Parameters Rule

**File**: [`high_risk_params_rule.md`](./high_risk_params_rule.md)

Checks for manually specified high-risk parameters across all components.

- **Purpose**: Custom high-risk parameter monitoring
- **Risk Levels**: Configurable (Error, Warning, Info)
- **Components**: TiDB, PD, TiKV, TiFlash

## Rule Architecture

### Rule Interface

All rules implement the `Rule` interface:

```go
type Rule interface {
    Name() string
    Category() string
    DataRequirements() DataSourceRequirement
    Evaluate(ctx context.Context, ruleCtx *RuleContext) ([]CheckResult, error)
}
```

### Data Requirements

Each rule specifies what data it needs:

- **Source Cluster Requirements**: What cluster data to collect
- **Source KB Requirements**: What source version knowledge base data to load
- **Target KB Requirements**: What target version knowledge base data to load

### Check Results

All rules return standardized `CheckResult` objects:

```go
type CheckResult struct {
    RuleID        string
    Category      string
    Component     string
    ParameterName string
    ParamType     string
    Severity      string
    RiskLevel     RiskLevel
    Message       string
    Details       string
    Suggestions   []string
    CurrentValue  interface{}
    SourceDefault interface{}
    TargetDefault interface{}
    ForcedValue   interface{}
}
```

## Rule Execution Flow

1. **Data Collection**: Analyzer collects required data based on all rules' requirements
2. **Rule Execution**: Each rule is executed with the collected data
3. **Result Aggregation**: All rule results are aggregated
4. **Report Generation**: Results are formatted into reports (text, JSON, HTML, Markdown)

## Adding New Rules

To add a new rule:

1. Create a new rule file in `pkg/analyzer/rules/`
2. Implement the `Rule` interface
3. Define data requirements
4. Implement evaluation logic
5. Create documentation in this directory
6. Add unit tests

See the [Analyzer README](../README.md) for more details on rule development.

## Rule Categories

Rules are organized by category:

- **`user_modified`**: User customization detection
- **`upgrade_difference`**: Upgrade-related changes
- **`consistency`**: Consistency checks
- **`high_risk`**: Custom high-risk monitoring

## Risk Levels

Rules assign risk levels to their findings:

- **High**: Critical issues that may cause upgrade failure or data loss
- **Medium**: Important issues that require attention
- **Low**: Informational issues for awareness

## Related Documentation

- [Analyzer Design](../README.md): Overall analyzer architecture
- [Rule Development Guide](../README.md#adding-new-rules): How to add new rules
- [Knowledge Base Generation](../../../knowledge_generation_guide.md): How knowledge base is generated

