// Package rules provides standardized rule definitions for upgrade precheck
package rules

import (
	"context"
	"fmt"
	"strings"
)

// UpgradeDifferencesRule detects parameters that will differ after upgrade
// Rule 2.2: Compare current cluster values with target version defaults
// and identify forced changes from upgrade logic
type UpgradeDifferencesRule struct {
	*BaseRule
}

// NewUpgradeDifferencesRule creates a new upgrade differences rule
func NewUpgradeDifferencesRule() Rule {
	return &UpgradeDifferencesRule{
		BaseRule: NewBaseRule(
			"UPGRADE_DIFFERENCES",
			"Detect parameters that will differ after upgrade, including forced changes",
			"upgrade_difference",
		),
	}
}

// DataRequirements returns the data requirements for this rule
func (r *UpgradeDifferencesRule) DataRequirements() DataSourceRequirement {
	return DataSourceRequirement{
		SourceClusterRequirements: struct {
			Components          []string `json:"components"`
			NeedConfig          bool     `json:"need_config"`
			NeedSystemVariables bool     `json:"need_system_variables"`
			NeedAllTikvNodes    bool     `json:"need_all_tikv_nodes"`
		}{
			Components:          []string{"tidb", "pd", "tikv", "tiflash"},
			NeedConfig:          true,
			NeedSystemVariables: true,
			NeedAllTikvNodes:    false,
		},
		TargetKBRequirements: struct {
			Components          []string `json:"components"`
			NeedConfigDefaults  bool     `json:"need_config_defaults"`
			NeedSystemVariables bool     `json:"need_system_variables"`
			NeedUpgradeLogic    bool     `json:"need_upgrade_logic"`
		}{
			Components:          []string{"tidb", "pd", "tikv", "tiflash"},
			NeedConfigDefaults:  true,
			NeedSystemVariables: true,
			NeedUpgradeLogic:    true, // Need upgrade logic for forced changes
		},
	}
}

// Evaluate performs the rule check
func (r *UpgradeDifferencesRule) Evaluate(ctx context.Context, ruleCtx *RuleContext) ([]CheckResult, error) {
	var results []CheckResult

	if ruleCtx.SourceClusterSnapshot == nil {
		return results, nil
	}

	// Get forced changes for each component
	forcedChangesByComponent := make(map[string]map[string]interface{})
	for _, comp := range []string{"tidb", "pd", "tikv", "tiflash"} {
		forcedChangesByComponent[comp] = ruleCtx.GetForcedChanges(comp)
	}

	// Check each component
	for compName, component := range ruleCtx.SourceClusterSnapshot.Components {
		// Determine component type
		compType := string(component.Type)
		if compType == "" {
			if strings.HasPrefix(compName, "tidb") {
				compType = "tidb"
			} else if strings.HasPrefix(compName, "pd") {
				compType = "pd"
			} else if strings.HasPrefix(compName, "tikv") {
				compType = "tikv"
			} else if strings.HasPrefix(compName, "tiflash") {
				compType = "tiflash"
			} else {
				continue
			}
		}

		// For TiKV, only check the first instance to avoid duplicates
		if compType == "tikv" && compName != "tikv" && !strings.HasPrefix(compName, "tikv-") {
			continue
		}

		forcedChanges := forcedChangesByComponent[compType]

		// Check config parameters
		for paramName, paramValue := range component.Config {
			currentValue := paramValue.Value
			targetDefault := ruleCtx.GetTargetDefault(compType, paramName)
			sourceDefault := ruleCtx.GetSourceDefault(compType, paramName)

			// Check if this parameter will be forced
			if forcedValue, isForced := forcedChanges[paramName]; isForced {
				// This is a forced change
				severity := "warning"
				if compType == "tidb" {
					severity = "error" // TiDB forced changes are more critical
				}

				results = append(results, CheckResult{
					RuleID:        r.Name(),
					Category:      r.Category(),
					Component:     compType,
					ParameterName: paramName,
					ParamType:     "config",
					Severity:      severity,
					Message:       fmt.Sprintf("Parameter %s in %s will be forcibly changed during upgrade", paramName, compType),
					Details:       fmt.Sprintf("Current: %v, Will be forced to: %v", currentValue, forcedValue),
					CurrentValue:  currentValue,
					TargetDefault: targetDefault,
					SourceDefault: sourceDefault,
					ForcedValue:   forcedValue,
					Suggestions: []string{
						"Review the forced change and its impact",
						"Test the new value in a staging environment",
						"Plan for the change before upgrading",
					},
				})
			} else if targetDefault != nil {
				// Check if default value changed between source and target versions
				defaultChanged := sourceDefault != nil && fmt.Sprintf("%v", sourceDefault) != fmt.Sprintf("%v", targetDefault)

				// Check if current value will differ from target default after upgrade
				valueWillDiffer := fmt.Sprintf("%v", currentValue) != fmt.Sprintf("%v", targetDefault)

				if valueWillDiffer {
					// Special handling for PD: if default changed but current value will be kept
					if compType == "pd" && defaultChanged {
						// PD maintains current configuration during upgrade (compatibility handling)
						// This is informational: default changed, but current value will be preserved
						results = append(results, CheckResult{
							RuleID:        r.Name(),
							Category:      r.Category(),
							Component:     compType,
							ParameterName: paramName,
							ParamType:     "config",
							Severity:      "info",
							Message:       fmt.Sprintf("Parameter %s in %s: default value changed, but current setting will be preserved during upgrade", paramName, compType),
							Details:       fmt.Sprintf("Current: %v (will be kept), Target default: %v, Source default: %v. PD maintains existing configuration during upgrade for compatibility.", currentValue, targetDefault, sourceDefault),
							CurrentValue:  currentValue,
							TargetDefault: targetDefault,
							SourceDefault: sourceDefault,
							Suggestions: []string{
								"The default value has changed in the target version",
								"Your current configuration will be preserved during upgrade",
								"Consider reviewing the new default value and adjusting if needed after upgrade",
							},
						})
					} else {
						// For other components or when default didn't change, use warning
						severity := "warning"
						// If user modified it, it's more important
						if ruleCtx.IsUserModified(compType, paramName) {
							severity = "warning"
						}

						results = append(results, CheckResult{
							RuleID:        r.Name(),
							Category:      r.Category(),
							Component:     compType,
							ParameterName: paramName,
							ParamType:     "config",
							Severity:      severity,
							Message:       fmt.Sprintf("Parameter %s in %s will differ after upgrade", paramName, compType),
							Details:       fmt.Sprintf("Current: %v, Target default: %v, Source default: %v", currentValue, targetDefault, sourceDefault),
							CurrentValue:  currentValue,
							TargetDefault: targetDefault,
							SourceDefault: sourceDefault,
							Suggestions: []string{
								"Review if the new default is acceptable",
								"Consider adjusting configuration if needed",
							},
						})
					}
				} else if defaultChanged && compType == "pd" {
					// PD default changed, but current value matches new default
					// Still provide info for awareness
					results = append(results, CheckResult{
						RuleID:        r.Name(),
						Category:      r.Category(),
						Component:     compType,
						ParameterName: paramName,
						ParamType:     "config",
						Severity:      "info",
						Message:       fmt.Sprintf("Parameter %s in %s: default value changed in target version", paramName, compType),
						Details:       fmt.Sprintf("Current: %v, Target default: %v, Source default: %v. Your current value matches the new default.", currentValue, targetDefault, sourceDefault),
						CurrentValue:  currentValue,
						TargetDefault: targetDefault,
						SourceDefault: sourceDefault,
						Suggestions: []string{
							"The default value has changed in the target version",
							"Your current configuration matches the new default",
						},
					})
				}
			}
		}

		// Check system variables (for TiDB)
		// TiDB system variables have special behavior: they keep old values after upgrade
		// (either user-set values or source defaults), unless forced by upgrade logic
		if compType == "tidb" {
			for varName, varValue := range component.Variables {
				currentValue := varValue.Value
				targetDefault := ruleCtx.GetTargetDefault(compType, "sysvar:"+varName)
				sourceDefault := ruleCtx.GetSourceDefault(compType, "sysvar:"+varName)

				// Check if this system variable will be forced
				if forcedValue, isForced := forcedChanges[varName]; isForced {
					// This is a forced change - very important for TiDB
					// After upgrade, this variable will be forced to forcedValue
					results = append(results, CheckResult{
						RuleID:        r.Name(),
						Category:      r.Category(),
						Component:     compType,
						ParameterName: varName,
						ParamType:     "system_variable",
						Severity:      "error", // Forced system variable changes are critical
						Message:       fmt.Sprintf("System variable %s in %s will be forcibly changed during upgrade", varName, compType),
						Details:       fmt.Sprintf("Current: %v, Will be forced to: %v (Target default: %v, Source default: %v)", currentValue, forcedValue, targetDefault, sourceDefault),
						CurrentValue:  currentValue,
						TargetDefault: targetDefault,
						SourceDefault: sourceDefault,
						ForcedValue:   forcedValue,
						Suggestions: []string{
							"CRITICAL: This system variable will be forcibly changed",
							"Review the forced change and its impact on your workload",
							"Test the new value in a staging environment",
							"Plan for the change before upgrading",
						},
					})
				} else {
					// For TiDB system variables, if not forced, they keep the current value after upgrade
					// So we compare: current value vs target default
					// If they differ, it means the current value (which will be kept) differs from target default
					// This is informational - the value won't change, but it's different from target default
					if targetDefault != nil {
						if fmt.Sprintf("%v", currentValue) != fmt.Sprintf("%v", targetDefault) {
							// Current value will be kept, but it's different from target default
							// This is just informational - the value won't actually change
							severity := "info"
							// If user modified it, it's more important to note
							if ruleCtx.IsUserModified(compType, "sysvar:"+varName) {
								severity = "info" // Still info, as it won't change
							}

							results = append(results, CheckResult{
								RuleID:        r.Name(),
								Category:      r.Category(),
								Component:     compType,
								ParameterName: varName,
								ParamType:     "system_variable",
								Severity:      severity,
								Message:       fmt.Sprintf("System variable %s in %s will keep current value after upgrade (differs from target default)", varName, compType),
								Details:       fmt.Sprintf("Current: %v (will be kept), Target default: %v, Source default: %v. Note: TiDB system variables keep old values unless forced.", currentValue, targetDefault, sourceDefault),
								CurrentValue:  currentValue,
								TargetDefault: targetDefault,
								SourceDefault: sourceDefault,
								Suggestions: []string{
									"This variable will keep its current value after upgrade",
									"The target default value is different but won't be applied (unless forced by upgrade logic)",
									"Consider reviewing if the current value is still appropriate for the target version",
								},
							})
						}
					}
				}
			}
		}
	}

	return results, nil
}
