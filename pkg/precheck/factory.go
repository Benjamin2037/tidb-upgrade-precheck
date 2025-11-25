package precheck

import (
	"context"
	"fmt"
)

// Factory creates precheck analyzers with predefined rule sets
type Factory struct{}

// NewFactory creates a new precheck factory
func NewFactory() *Factory {
	return &Factory{}
}

// CreateDefaultAnalyzer creates an analyzer with default rules
func (f *Factory) CreateDefaultAnalyzer() *Analyzer {
	analyzer := NewAnalyzer()
	
	// Register default rules
	// TODO: Add actual rule implementations
	// analyzer.AddRule(NewConfigCheckRule())
	// analyzer.AddRule(NewSysVarCheckRule())
	
	return analyzer
}

// CreateAnalyzerWithRules creates an analyzer with specific rules
func (f *Factory) CreateAnalyzerWithRules(rules []Rule) *Analyzer {
	analyzer := NewAnalyzer()
	
	for _, rule := range rules {
		analyzer.AddRule(rule)
	}
	
	return analyzer
}

// RuntimeRuleAdapter adapts a runtime checker to a precheck rule
type RuntimeRuleAdapter struct {
	name    string
	checker func(context.Context, Snapshot) ([]ReportItem, error)
}

// Name returns the rule name
func (r *RuntimeRuleAdapter) Name() string {
	return r.name
}

// Evaluate evaluates the rule against a snapshot
func (r *RuntimeRuleAdapter) Evaluate(ctx context.Context, snapshot Snapshot) ([]ReportItem, error) {
	return r.checker(ctx, snapshot)
}

// CreateAnalyzerWithKB creates an analyzer with knowledge base aware rules
func (f *Factory) CreateAnalyzerWithKB(sourceKB, targetKB map[string]interface{}) *Analyzer {
	analyzer := NewAnalyzer()
	
	// Create rules with knowledge base data
	configRule := NewRuleFunc("config-check", func(ctx context.Context, snapshot Snapshot) ([]ReportItem, error) {
		return checkConfig(sourceKB, targetKB, snapshot)
	})
	
	sysVarRule := NewRuleFunc("sysvar-check", func(ctx context.Context, snapshot Snapshot) ([]ReportItem, error) {
		forcedChanges := getForcedChanges(targetKB)
		return checkSysVars(sourceKB, targetKB, forcedChanges, snapshot)
	})
	
	analyzer.AddRule(configRule)
	analyzer.AddRule(sysVarRule)
	
	return analyzer
}

// getForcedChanges extracts forced changes from target knowledge base
func getForcedChanges(targetKB map[string]interface{}) map[string]interface{} {
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

// checkConfig checks configuration parameters
func checkConfig(sourceKB, targetKB map[string]interface{}, snapshot Snapshot) ([]ReportItem, error) {
	var items []ReportItem

	// Check TiDB config parameters
	if tidbSnapshot, exists := snapshot.Components["tidb"]; exists {
		// Get source and target config defaults
		sourceConfigDefaults := make(map[string]interface{})
		targetConfigDefaults := make(map[string]interface{})

		if sourceConfig, ok := sourceKB["config_defaults"].(map[string]interface{}); ok {
			sourceConfigDefaults = sourceConfig
		}

		if targetConfig, ok := targetKB["config_defaults"].(map[string]interface{}); ok {
			targetConfigDefaults = targetConfig
		}

		// Check each config parameter
		for name, currentValue := range tidbSnapshot.Config {
			sourceDefault, sourceExists := sourceConfigDefaults[name]
			targetDefault, targetExists := targetConfigDefaults[name]

			// Check if parameter default changed
			if sourceExists && targetExists && 
				fmt.Sprintf("%v", sourceDefault) != fmt.Sprintf("%v", targetDefault) {
				items = append(items, ReportItem{
					Rule:     "config-check",
					Severity: SeverityWarning,
					Message:  fmt.Sprintf("TiDB config parameter '%s' default value changed", name),
					Details: []string{
						fmt.Sprintf("Default value changed from '%v' to '%v'", sourceDefault, targetDefault),
					},
				})
			}

			// Check if user has customized the parameter
			if sourceExists && fmt.Sprintf("%v", currentValue) != fmt.Sprintf("%v", sourceDefault) {
				items = append(items, ReportItem{
					Rule:     "config-check",
					Severity: SeverityInfo,
					Message:  fmt.Sprintf("TiDB config parameter '%s' has custom value", name),
					Details: []string{
						fmt.Sprintf("Current value: '%v', default value: '%v'", currentValue, sourceDefault),
					},
				})
			}
		}
	}

	return items, nil
}

// checkSysVars checks system variables
func checkSysVars(sourceKB, targetKB, forcedChanges map[string]interface{}, snapshot Snapshot) ([]ReportItem, error) {
	var items []ReportItem

	// Get source and target system variable defaults
	sourceSysVarDefaults := make(map[string]interface{})
	targetSysVarDefaults := make(map[string]interface{})

	if sourceSysVars, ok := sourceKB["system_variables"].(map[string]interface{}); ok {
		sourceSysVarDefaults = sourceSysVars
	}

	if targetSysVars, ok := targetKB["system_variables"].(map[string]interface{}); ok {
		targetSysVarDefaults = targetSysVars
	}

	// Check each system variable
	for name, currentValue := range snapshot.GlobalSysVars {
		sourceDefault, sourceExists := sourceSysVarDefaults[name]
		targetDefault, targetExists := targetSysVarDefaults[name]

		// Check if parameter is forcibly changed during upgrade (HIGH risk)
		if _, isForced := forcedChanges[name]; isForced {
			items = append(items, ReportItem{
				Rule:     "sysvar-check",
				Severity: SeverityBlocker,
				Message:  fmt.Sprintf("System variable '%s' will be forcibly changed during upgrade", name),
				Details: []string{
					fmt.Sprintf("Current value: '%v', will be changed to: '%v'", currentValue, forcedChanges[name]),
					"This is a forced change during the upgrade process and cannot be overridden",
				},
			})
			continue // Skip other checks for forcibly changed variables
		}

		// Check if default value changes (MEDIUM risk for user-set parameters)
		if sourceExists && targetExists && 
			fmt.Sprintf("%v", sourceDefault) != fmt.Sprintf("%v", targetDefault) {
			// Check if user has customized the parameter
			if fmt.Sprintf("%v", currentValue) != fmt.Sprintf("%v", sourceDefault) {
				items = append(items, ReportItem{
					Rule:     "sysvar-check",
					Severity: SeverityWarning,
					Message:  fmt.Sprintf("System variable '%s' default value changed and has custom value", name),
					Details: []string{
						fmt.Sprintf("Default value changed from '%v' to '%v'", sourceDefault, targetDefault),
						fmt.Sprintf("Current custom value: '%v'", currentValue),
						"You have customized this parameter and the default is changing in the target version",
					},
				})
			} else {
				// Using default value but default is changing
				items = append(items, ReportItem{
					Rule:     "sysvar-check",
					Severity: SeverityInfo,
					Message:  fmt.Sprintf("System variable '%s' default value will change", name),
					Details: []string{
						fmt.Sprintf("Default value changing from '%v' to '%v'", sourceDefault, targetDefault),
						"The default value for this parameter is changing in the target version",
					},
				})
			}
		} else if sourceExists && fmt.Sprintf("%v", currentValue) != fmt.Sprintf("%v", sourceDefault) {
			// User has customized the parameter but default is not changing
			items = append(items, ReportItem{
				Rule:     "sysvar-check",
				Severity: SeverityInfo,
				Message:  fmt.Sprintf("System variable '%s' has custom value", name),
				Details: []string{
					fmt.Sprintf("Current value: '%v', default value: '%v'", currentValue, sourceDefault),
					"You have customized this parameter",
				},
			})
		}
	}

	return items, nil
}