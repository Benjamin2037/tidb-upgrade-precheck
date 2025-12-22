# Analyzer Design

This document describes the detailed design and implementation of the analyzer module, including the rule-based architecture and how to develop new rules.

## Overview

The analyzer compares runtime configuration against the knowledge base to identify risks using a rule-based architecture. This design enables sustainable and rapid addition of new check rules.

## Rule-Based Architecture

### Design Principles

- **Modular Design**: Each rule is an independent module implementing the `Rule` interface
- **Rapid Extension**: New rules can be added quickly without modifying existing code
- **Isolated Testing**: Each rule can be tested independently
- **Flexible Configuration**: Rules can be enabled/disabled or configured independently
- **Future-Proof**: The architecture supports continuous expansion of precheck capabilities
- **Optimized Data Loading**: Rules declare data requirements, analyzer loads only necessary data

### Rule Interface

All rules must implement the `Rule` interface defined in `pkg/analyzer/rules/rule.go`:

```go
type Rule interface {
    Name() string
    Description() string
    Category() string
    DataRequirements() DataSourceRequirement
    Evaluate(ctx context.Context, ruleCtx *RuleContext) ([]CheckResult, error)
}
```

### Data Requirements

Each rule declares what data it needs through `DataRequirements()`:

```go
type DataSourceRequirement struct {
    SourceClusterRequirements struct {
        Components          []string
        NeedConfig          bool
        NeedSystemVariables bool
        NeedAllTikvNodes    bool
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
        NeedUpgradeLogic    bool
    }
}
```

The analyzer merges requirements from all rules and loads only the necessary data, optimizing performance.

### Rule Context

The `RuleContext` provides access to:
- Source and target cluster snapshots
- Knowledge base data (source and target defaults, upgrade logic)
- Component configurations
- Bootstrap versions (for upgrade logic filtering)
- Helper methods for accessing defaults and comparing values

## Current Rules

The analyzer includes several rules for checking different aspects of cluster configuration and upgrade compatibility. Each rule is documented in detail in the [Rules Documentation](./rules/) directory.

### Default Rules

The analyzer includes three default rules (defined in `pkg/analyzer/analyzer.go`):

1. **[User Modified Params Rule](./rules/user_modified_params_rule.md)** - Detects parameters modified by users
2. **[Upgrade Differences Rule](./rules/upgrade_differences_rule.md)** - Detects parameters that will change after upgrade
3. **[TiKV Consistency Rule](./rules/tikv_consistency_rule.md)** - Compares TiKV parameters with source defaults

### Optional Rules

4. **[High Risk Params Rule](./rules/high_risk_params_rule.md)** - Validates manually specified high-risk parameters

**Note**: The High Risk Params Rule is not included in default rules. It must be explicitly added when creating the analyzer with a custom rule list.

### Quick Reference

| Rule | Purpose | Risk Level | Components |
|------|---------|------------|------------|
| User Modified Params | Detect user customizations | Low | All |
| Upgrade Differences | Detect upgrade changes | High/Medium/Low | All |
| TiKV Consistency | Compare TiKV with defaults | Medium | TiKV |
| High Risk Params | Custom risk monitoring | Configurable | All |

For detailed documentation on each rule, including logic, implementation details, and examples, see the [Rules Documentation](./rules/) directory.

## Analyzer Workflow

The analyzer follows this workflow:

1. **Collect Data Requirements**: Merge requirements from all rules
2. **Load Knowledge Base**: Load only necessary data based on merged requirements
3. **Preprocess Parameters**: Filter deployment-specific, resource-dependent, and identical parameters
4. **Build Component Mapping**: Map KB components to runtime components
5. **Create Rule Context**: Create shared context with cleaned data (filtered parameters removed)
6. **Execute Rules**: Run all rules with the shared context (rules receive only necessary parameters)
7. **Organize Results**: Group results by category and severity

## Preprocessing Stage

Before rules are evaluated, the analyzer runs a preprocessing stage (`pkg/analyzer/preprocessor.go`) that:

1. **Filters Deployment-Specific Parameters**: 
   - Path parameters (data-dir, log-dir, etc.)
   - Host/network parameters (host, port, etc.)
   - Platform information (version_compile_machine, etc.)
   - Timezone parameters (system_time_zone, etc.)

2. **Filters Resource-Dependent Parameters**:
   - Parameters auto-tuned by system (num-threads, concurrency, etc.)
   - Only filtered if source default == target default (difference is due to deployment environment)

3. **Filters Identical Parameters**:
   - Parameters where current == source == target (no difference to report)

4. **Removes Filtered Parameters**: 
   - Removes filtered parameters from `sourceDefaults` and `targetDefaults` maps
   - Rules receive cleaned defaults (only parameters that need comparison)

5. **Generates CheckResults**: 
   - Creates `CheckResult` entries for filtered parameters (for reporting)
   - These results are included in the final report but don't go through rule evaluation

### Filter Configuration

Filtering logic is centralized in `pkg/analyzer/filters.go`:

- **FilterConfig**: Single source of truth for all filtering rules
- **ShouldFilterParameter()**: Unified function to check if a parameter should be filtered
- **IsResourceDependentParameter()**: Checks if parameter is resource-dependent
- **IsFilenameOnlyParameter()**: Checks if parameter needs special comparison (filename only)

### Benefits

- **Reduced Overhead**: Rules only process parameters that need actual comparison
- **Centralized Logic**: All filtering in one place, easy to maintain
- **Simplified Rules**: Rules don't need to check for path parameters, deployment-specific parameters, etc.
- **Better Performance**: Fewer parameters to compare in rules

## Adding New Rules

### Step-by-Step Guide

1. **Create Rule File**: Create a new file in `pkg/analyzer/rules/` (e.g., `my_new_rule.go`)

2. **Implement Rule Interface**:
   ```go
   type MyNewRule struct {
       *BaseRule
   }
   
   func NewMyNewRule() *MyNewRule {
       return &MyNewRule{
           BaseRule: NewBaseRule("MyNewRule", "Description", "Category"),
       }
   }
   
   func (r *MyNewRule) DataRequirements() DataSourceRequirement {
       // Declare what data this rule needs
   }
   
   func (r *MyNewRule) Evaluate(ctx context.Context, ruleCtx *RuleContext) ([]CheckResult, error) {
       // Implementation
   }
   ```

3. **Add to Analyzer**: 
   - For default rules: Add to `getDefaultRules()` in `pkg/analyzer/analyzer.go`
   - For custom rules: Pass when creating analyzer: `NewAnalyzer(&AnalysisOptions{Rules: []rules.Rule{...}})`

4. **Write Tests**: Create test file `pkg/analyzer/rules/my_new_rule_test.go`

5. **Document**: Add documentation in the [Rules Documentation](./rules/) directory

### Best Practices

- **Keep rules focused on a single responsibility**
- **Declare accurate data requirements** to optimize loading
- **Don't filter parameters** - filtering is done in preprocessing stage
- **Use cleaned defaults** - rules receive defaults with filtered parameters already removed
- **Use the `RuleContext` helper methods** for accessing data
- **Check for nil values** - always validate targetDefault and currentValue
- **Use helper methods** - `GetForcedChangeMetadata()` and `GetParameterNote()` for special handling
- **Provide clear error messages** in `CheckResult`
- **Write comprehensive tests** - test with cleaned defaults
- **Document rule behavior** and configuration options

## Implementation Plan

- **[Analyzer Implementation Plan](./analyzer_implementation_plan.md)** - Detailed implementation plan for the analyzer module, including data structures, interfaces, and risk evaluation logic

## Related Documents

- [Parameter Comparison Design](../parameter_comparison/) - Parameter comparison capabilities
- [Rule Interface Documentation](../../../pkg/analyzer/rules/README.md) - Detailed rule interface documentation
