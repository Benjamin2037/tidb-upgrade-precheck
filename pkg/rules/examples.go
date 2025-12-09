package rules

import (
	"context"
	"fmt"
)

// Example: ParameterDefaultChangeRule
// This rule checks if parameter defaults will change between source and target versions
// and warns if user has modified the parameter (which might conflict with new default)
type ParameterDefaultChangeRule struct {
	*BaseRule
	component string
}

// NewParameterDefaultChangeRule creates a rule to check parameter default changes
func NewParameterDefaultChangeRule(component string) *ParameterDefaultChangeRule {
	return &ParameterDefaultChangeRule{
		BaseRule: NewBaseRule(
			fmt.Sprintf("PARAM_DEFAULT_CHANGE_%s", component),
			fmt.Sprintf("Check for parameter default value changes in %s", component),
		),
		component: component,
	}
}

// Evaluate compares source defaults vs target defaults
func (r *ParameterDefaultChangeRule) Evaluate(ctx context.Context, ruleCtx *RuleContext) ([]CheckResult, error) {
	var results []CheckResult

	// Get all parameters from source defaults
	sourceDefaults := ruleCtx.SourceDefaults[r.component]
	if sourceDefaults == nil {
		return results, nil
	}

	// Check each parameter
	for paramName, sourceDefault := range sourceDefaults {
		// Check if default will change
		if ruleCtx.WillDefaultChange(r.component, paramName) {
			targetDefault := ruleCtx.GetTargetDefault(r.component, paramName)
			sampledValue := ruleCtx.GetSampledValue(r.component, paramName)
			isUserModified := ruleCtx.IsUserModified(r.component, paramName)

			severity := "info"
			message := fmt.Sprintf("Parameter %s default will change from %v to %v",
				paramName, sourceDefault, targetDefault)

			// If user has modified the value, this is more critical
			if isUserModified {
				severity = "warning"
				message = fmt.Sprintf("Parameter %s default will change (user modified: %v -> target default: %v)",
					paramName, sampledValue, targetDefault)
			}

			// If the change is forced, it's even more critical
			if ruleCtx.WillBeForced(r.component, paramName) {
				severity = "error"
				message = fmt.Sprintf("Parameter %s will be FORCED to change from %v to %v (user value: %v will be overridden)",
					paramName, sourceDefault, targetDefault, sampledValue)
			}

			results = append(results, CheckResult{
				RuleID:      r.Name(),
				Description: r.Description(),
				Severity:    severity,
				Message:     message,
				Details: fmt.Sprintf("Source default: %v, Target default: %v, Current value: %v",
					sourceDefault, targetDefault, sampledValue),
				Suggestions: []string{
					"Review the parameter change in release notes",
					"Test the new default value in a staging environment",
					"Consider adjusting your configuration if needed",
				},
			})
		}
	}

	return results, nil
}

// Example: SystemVariableForcedChangeRule
// This rule checks for system variables that will be forcibly changed during upgrade
type SystemVariableForcedChangeRule struct {
	*BaseRule
	component string
}

// NewSystemVariableForcedChangeRule creates a rule to check forced system variable changes
func NewSystemVariableForcedChangeRule(component string) *SystemVariableForcedChangeRule {
	return &SystemVariableForcedChangeRule{
		BaseRule: NewBaseRule(
			fmt.Sprintf("SYSVAR_FORCED_CHANGE_%s", component),
			fmt.Sprintf("Check for system variables that will be forcibly changed in %s", component),
		),
		component: component,
	}
}

// Evaluate checks for forced system variable changes
func (r *SystemVariableForcedChangeRule) Evaluate(ctx context.Context, ruleCtx *RuleContext) ([]CheckResult, error) {
	var results []CheckResult

	// Get source defaults for the component
	sourceDefaults := ruleCtx.SourceDefaults[r.component]
	if sourceDefaults == nil {
		return results, nil
	}

	// Check each parameter/variable (system variables are prefixed with "sysvar:")
	for paramName := range sourceDefaults {
		// Only check system variables (prefixed with "sysvar:")
		if len(paramName) > 7 && paramName[:7] == "sysvar:" {
			varName := paramName[7:] // Remove "sysvar:" prefix

			// Check if this variable will be forced (check in upgrade logic)
			if ruleCtx.WillBeForced(r.component, varName) {
				sampledValue := ruleCtx.GetSampledValue(r.component, varName)
				targetDefault := ruleCtx.GetTargetDefault(r.component, paramName)

				results = append(results, CheckResult{
					RuleID:      r.Name(),
					Description: r.Description(),
					Severity:    "error",
					Message:     fmt.Sprintf("System variable %s will be FORCED to change", varName),
					Details:     fmt.Sprintf("Current value: %v will be changed to: %v", sampledValue, targetDefault),
					Suggestions: []string{
						"This change cannot be prevented - it will happen during upgrade",
						"Review the impact of this change on your workload",
						"Test your application with the new value before upgrading",
					},
				})
			}
		}
	}

	return results, nil
}

// Example: QueryPlanChangeRule
// This rule compares query execution plans between source and target versions
type QueryPlanChangeRule struct {
	*BaseRule
	queries []string // List of queries to check
}

// NewQueryPlanChangeRule creates a rule to compare query plans
func NewQueryPlanChangeRule(queries []string) *QueryPlanChangeRule {
	return &QueryPlanChangeRule{
		BaseRule: NewBaseRule(
			"QUERY_PLAN_CHANGE",
			"Compare query execution plans between source and target versions",
		),
		queries: queries,
	}
}

// Evaluate compares query plans
func (r *QueryPlanChangeRule) Evaluate(ctx context.Context, ruleCtx *RuleContext) ([]CheckResult, error) {
	var results []CheckResult

	for _, query := range r.queries {
		sourcePlan := ruleCtx.GetQueryPlanSource(query)
		targetPlan := ruleCtx.GetQueryPlanTarget(query)

		if sourcePlan == nil || targetPlan == nil {
			continue
		}

		// Compare plans (simplified - actual comparison would be more sophisticated)
		if !plansEqual(sourcePlan, targetPlan) {
			severity := "warning"
			message := fmt.Sprintf("Query execution plan will change for query: %s", query)

			// If the change might cause performance regression, mark as error
			if mightCauseRegression(sourcePlan, targetPlan) {
				severity = "error"
				message = fmt.Sprintf("Query execution plan change may cause performance regression: %s", query)
			}

			results = append(results, CheckResult{
				RuleID:      r.Name(),
				Description: r.Description(),
				Severity:    severity,
				Message:     message,
				Details:     fmt.Sprintf("Source plan: %v, Target plan: %v", sourcePlan, targetPlan),
				Suggestions: []string{
					"Review the query plan changes",
					"Test query performance with target version",
					"Consider adding indexes if needed",
				},
			})
		}
	}

	return results, nil
}

// plansEqual compares two query plans (simplified implementation)
func plansEqual(plan1, plan2 interface{}) bool {
	// In production, this would do a proper comparison of execution plans
	// For now, just do a simple equality check
	return fmt.Sprintf("%v", plan1) == fmt.Sprintf("%v", plan2)
}

// mightCauseRegression checks if plan change might cause performance regression
func mightCauseRegression(sourcePlan, targetPlan interface{}) bool {
	// In production, this would analyze the plans to detect potential regressions
	// For example: index scan -> table scan, hash join -> nested loop join, etc.
	return false
}
