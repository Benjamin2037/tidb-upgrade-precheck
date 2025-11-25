package rules

import (
	"context"
	"fmt"

	"github.com/pingcap/tidb-upgrade-precheck/pkg/precheck"
)

// SysVarCheckRule checks for system variable-related issues
type SysVarCheckRule struct {
	sourceKB       map[string]interface{}
	targetKB       map[string]interface{}
	forcedChanges  map[string]interface{}
}

// NewSysVarCheckRule creates a new system variable check rule
func NewSysVarCheckRule(sourceKB, targetKB map[string]interface{}, forcedChanges map[string]interface{}) precheck.Rule {
	return &SysVarCheckRule{
		sourceKB:      sourceKB,
		targetKB:      targetKB,
		forcedChanges: forcedChanges,
	}
}

// Name returns the rule name
func (r *SysVarCheckRule) Name() string {
	return "sysvar-check"
}

// Evaluate evaluates the rule against a snapshot
func (r *SysVarCheckRule) Evaluate(ctx context.Context, snapshot precheck.Snapshot) ([]precheck.ReportItem, error) {
	var items []precheck.ReportItem

	// Check global system variables
	items = append(items, r.checkGlobalSysVars(snapshot.GlobalSysVars)...)

	return items, nil
}

func (r *SysVarCheckRule) checkGlobalSysVars(vars map[string]string) []precheck.ReportItem {
	var items []precheck.ReportItem

	// Get source and target system variable defaults
	sourceSysVarDefaults := make(map[string]interface{})
	targetSysVarDefaults := make(map[string]interface{})

	if sourceSysVars, ok := r.sourceKB["system_variables"].(map[string]interface{}); ok {
		sourceSysVarDefaults = sourceSysVars
	}

	if targetSysVars, ok := r.targetKB["system_variables"].(map[string]interface{}); ok {
		targetSysVarDefaults = targetSysVars
	}

	// Check each system variable
	for name, currentValue := range vars {
		sourceDefault, sourceExists := sourceSysVarDefaults[name]
		targetDefault, targetExists := targetSysVarDefaults[name]

		// Check if parameter is forcibly changed during upgrade (HIGH risk)
		if _, isForced := r.forcedChanges[name]; isForced {
			items = append(items, precheck.ReportItem{
				Rule:     "sysvar-check",
				Severity: precheck.SeverityBlocker,
				Message:  fmt.Sprintf("System variable '%s' will be forcibly changed during upgrade", name),
				Details: []string{
					fmt.Sprintf("Current value: '%v', will be changed to: '%v'", currentValue, r.forcedChanges[name]),
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
				items = append(items, precheck.ReportItem{
					Rule:     "sysvar-check",
					Severity: precheck.SeverityWarning,
					Message:  fmt.Sprintf("System variable '%s' default value changed and has custom value", name),
					Details: []string{
						fmt.Sprintf("Default value changed from '%v' to '%v'", sourceDefault, targetDefault),
						fmt.Sprintf("Current custom value: '%v'", currentValue),
						"You have customized this parameter and the default is changing in the target version",
					},
				})
			} else {
				// Using default value but default is changing
				items = append(items, precheck.ReportItem{
					Rule:     "sysvar-check",
					Severity: precheck.SeverityInfo,
					Message:  fmt.Sprintf("System variable '%s' default value will change", name),
					Details: []string{
						fmt.Sprintf("Default value changing from '%v' to '%v'", sourceDefault, targetDefault),
						"The default value for this parameter is changing in the target version",
					},
				})
			}
		} else if sourceExists && fmt.Sprintf("%v", currentValue) != fmt.Sprintf("%v", sourceDefault) {
			// User has customized the parameter but default is not changing
			items = append(items, precheck.ReportItem{
				Rule:     "sysvar-check",
				Severity: precheck.SeverityInfo,
				Message:  fmt.Sprintf("System variable '%s' has custom value", name),
				Details: []string{
					fmt.Sprintf("Current value: '%v', default value: '%v'", currentValue, sourceDefault),
					"You have customized this parameter",
				},
			})
		}
	}

	return items
}