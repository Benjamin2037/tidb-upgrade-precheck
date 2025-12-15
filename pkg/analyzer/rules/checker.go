package rules

import (
	"context"
)

// RiskLevel represents the risk level of a check result
type RiskLevel string

const (
	RiskLevelHigh   RiskLevel = "high"   // critical, error
	RiskLevelMedium RiskLevel = "medium" // warning
	RiskLevelLow    RiskLevel = "low"    // info
)

// GetRiskLevel determines the risk level from severity
func GetRiskLevel(severity string) RiskLevel {
	switch severity {
	case "critical", "error":
		return RiskLevelHigh
	case "warning":
		return RiskLevelMedium
	case "info":
		return RiskLevelLow
	default:
		return RiskLevelLow
	}
}

// CheckResult represents the result of a single check
type CheckResult struct {
	RuleID        string                 `json:"rule_id"`
	Category      string                 `json:"category,omitempty"`       // Category/group of this rule
	Component     string                 `json:"component,omitempty"`      // Component this result relates to
	ParameterName string                 `json:"parameter_name,omitempty"` // Parameter or system variable name
	ParamType     string                 `json:"param_type,omitempty"`     // "config" or "system_variable"
	Description   string                 `json:"description"`
	Severity      string                 `json:"severity"`             // "info", "warning", "error", "critical"
	RiskLevel     RiskLevel              `json:"risk_level,omitempty"` // Risk level: "high", "medium", "low" (auto-set from severity if not provided)
	Message       string                 `json:"message"`
	Details       string                 `json:"details,omitempty"`
	Suggestions   []string               `json:"suggestions,omitempty"` // Optional suggestions for fixing the issue
	CurrentValue  interface{}            `json:"current_value,omitempty"`
	SourceDefault interface{}            `json:"source_default,omitempty"`
	TargetDefault interface{}            `json:"target_default,omitempty"`
	ForcedValue   interface{}            `json:"forced_value,omitempty"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"` // Additional metadata
}

// RuleRunner orchestrates the execution of all rules with full context
type RuleRunner struct {
	rules []Rule
}

// NewRuleRunner creates a new rule runner
func NewRuleRunner(rules []Rule) *RuleRunner {
	return &RuleRunner{
		rules: rules,
	}
}

// Run executes all rules with the provided context and returns combined results
func (r *RuleRunner) Run(ctx context.Context, ruleCtx *RuleContext) ([]CheckResult, error) {
	var allResults []CheckResult

	for _, rule := range r.rules {
		if ctx.Err() != nil {
			break
		}

		results, err := rule.Evaluate(ctx, ruleCtx)
		if err != nil {
			// Create an error result for this rule
			allResults = append(allResults, CheckResult{
				RuleID:      rule.Name(),
				Description: rule.Description(),
				Severity:    "error",
				Message:     "Rule execution failed",
				Details:     err.Error(),
			})
			continue
		}

		// Ensure all results have the rule ID, category, and risk level set
		for i := range results {
			if results[i].RuleID == "" {
				results[i].RuleID = rule.Name()
			}
			if results[i].Category == "" {
				results[i].Category = rule.Category()
			}
			if results[i].Description == "" {
				results[i].Description = rule.Description()
			}
			// Auto-set risk level from severity if not already set
			if results[i].RiskLevel == "" {
				results[i].RiskLevel = GetRiskLevel(results[i].Severity)
			}
		}

		allResults = append(allResults, results...)
	}

	return allResults, nil
}
