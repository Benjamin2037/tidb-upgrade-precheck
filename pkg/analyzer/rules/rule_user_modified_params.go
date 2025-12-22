// Package rules provides standardized rule definitions for upgrade precheck
package rules

import (
	"context"
	"fmt"
	"strings"

	"github.com/pingcap/tidb-upgrade-precheck/pkg/collector"
)

// UserModifiedParamsRule detects parameters that have been modified by the user
// Rule 2.1: Compare current cluster values with source version defaults
// to determine if user has modified any parameters
type UserModifiedParamsRule struct {
	*BaseRule
}

// NewUserModifiedParamsRule creates a new user modified parameters rule
func NewUserModifiedParamsRule() Rule {
	return &UserModifiedParamsRule{
		BaseRule: NewBaseRule(
			"USER_MODIFIED_PARAMS",
			"Detect parameters that have been modified by the user from source version defaults",
			"user_modified",
		),
	}
}

// DataRequirements returns the data requirements for this rule
func (r *UserModifiedParamsRule) DataRequirements() DataSourceRequirement {
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
			NeedAllTikvNodes:    false, // Only need one instance per component for this check
		},
		SourceKBRequirements: struct {
			Components          []string `json:"components"`
			NeedConfigDefaults  bool     `json:"need_config_defaults"`
			NeedSystemVariables bool     `json:"need_system_variables"`
			NeedUpgradeLogic    bool     `json:"need_upgrade_logic"`
		}{
			Components:          []string{"tidb", "pd", "tikv", "tiflash"},
			NeedConfigDefaults:  true,
			NeedSystemVariables: true,
			NeedUpgradeLogic:    false,
		},
	}
}

// Evaluate performs the rule check
// It compares all source version defaults with current cluster runtime values
// by iterating through the source defaults map and comparing with runtime values
func (r *UserModifiedParamsRule) Evaluate(ctx context.Context, ruleCtx *RuleContext) ([]CheckResult, error) {
	var results []CheckResult

	if ruleCtx.SourceClusterSnapshot == nil {
		return results, nil
	}

	// Iterate through all components in source defaults
	for compType, sourceDefaults := range ruleCtx.SourceDefaults {
		// Find the corresponding component in the cluster snapshot
		var component *collector.ComponentState
		var compName string

		// Try to find component by type
		for name, comp := range ruleCtx.SourceClusterSnapshot.Components {
			if string(comp.Type) == compType {
				component = &comp
				compName = name
				break
			}
			// Also check by name prefix for TiKV/TiFlash nodes
			if (compType == "tikv" && strings.HasPrefix(name, "tikv")) ||
				(compType == "tiflash" && strings.HasPrefix(name, "tiflash")) {
				// For TiKV/TiFlash, use the first instance found
				if component == nil {
					component = &comp
					compName = name
				}
			}
		}

		if component == nil {
			// Component not found in cluster snapshot, skip
			continue
		}

		// For TiKV, only check the first instance to avoid duplicate results
		if compType == "tikv" && compName != "tikv" && !strings.HasPrefix(compName, "tikv-") {
			continue
		}

		// Build runtime parameter maps for reverse lookup (cluster → KB)
		runtimeConfigMap := make(map[string]bool)
		runtimeVarsMap := make(map[string]bool)
		for paramName := range component.Config {
			runtimeConfigMap[paramName] = true
		}
		for varName := range component.Variables {
			runtimeVarsMap[varName] = true
		}

		// Compare all source defaults with current runtime values
		// Iterate through source defaults map (KB → Cluster)
		for paramName, sourceDefaultValue := range sourceDefaults {
			// Extract actual value from ParameterValue structure
			sourceDefault := extractValueFromDefault(sourceDefaultValue)
			if sourceDefault == nil {
				continue
			}

			// Determine if this is a system variable (prefixed with "sysvar:")
			isSystemVar := strings.HasPrefix(paramName, "sysvar:")
			var currentValue interface{}

			if isSystemVar {
				// System variable: remove "sysvar:" prefix and check in Variables
				varName := strings.TrimPrefix(paramName, "sysvar:")
				if varValue, ok := component.Variables[varName]; ok {
					currentValue = varValue.Value
					// Mark as processed
					delete(runtimeVarsMap, varName)
				} else {
					// Variable exists in KB but not in runtime - report as mismatch
					displayName := varName
					// Note: Filtering of ignored parameters is done at report generation time, not here
					// This ensures all parameters are properly categorized before filtering
					results = append(results, CheckResult{
						RuleID:        r.Name(),
						Category:      r.Category(),
						Component:     compType,
						ParameterName: displayName,
						ParamType:     "system_variable",
						Severity:      "warning",
						RiskLevel:     RiskLevelMedium,
						Message:       fmt.Sprintf("System variable %s exists in source KB (v%s) but not found in runtime cluster", displayName, ruleCtx.SourceVersion),
						Details:       fmt.Sprintf("Source KB default: %s | Runtime: <not found>", FormatValue(sourceDefault)),
						SourceDefault: sourceDefault,
						Suggestions: []string{
							"This system variable exists in source version knowledge base but is missing in runtime cluster",
							"Verify if this variable was removed or renamed in the current cluster version",
							"Check if this is expected behavior or a data collection issue",
						},
					})
					continue
				}
			} else {
				// Config parameter: check in Config
				if paramValue, ok := component.Config[paramName]; ok {
					currentValue = paramValue.Value
					// Mark as processed
					delete(runtimeConfigMap, paramName)
				} else {
					// Parameter exists in KB but not in runtime - report as mismatch
					// Note: Filtering of ignored parameters is done at report generation time, not here
					// This ensures all parameters are properly categorized before filtering
					results = append(results, CheckResult{
						RuleID:        r.Name(),
						Category:      r.Category(),
						Component:     compType,
						ParameterName: paramName,
						ParamType:     "config",
						Severity:      "warning",
						RiskLevel:     RiskLevelMedium,
						Message:       fmt.Sprintf("Parameter %s exists in source KB (v%s) but not found in runtime cluster", paramName, ruleCtx.SourceVersion),
						Details:       fmt.Sprintf("Source KB default: %s | Runtime: <not found>", FormatValue(sourceDefault)),
						SourceDefault: sourceDefault,
						Suggestions: []string{
							"This parameter exists in source version knowledge base but is missing in runtime cluster",
							"Verify if this parameter was removed or renamed in the current cluster version",
							"Check if this is expected behavior or a data collection issue",
						},
					})
					continue
				}
			}

			// Get display name for parameter
			displayName := paramName
			if isSystemVar {
				displayName = strings.TrimPrefix(paramName, "sysvar:")
			}

			// For map types, do deep comparison to find only differing fields
			if IsMapType(currentValue) && IsMapType(sourceDefault) {
				opts := CompareOptions{
					BasePath: paramName,
				}
				differingFields := CompareMapsDeep(currentValue, sourceDefault, opts)
				for fieldPath, diff := range differingFields {
					// Note: Resource-dependent parameter filtering is done at report generation time, not here
					// This ensures all parameters are properly categorized before filtering

					// Show all differences in map, don't ignore nested fields
					paramType := "config"
					if isSystemVar {
						paramType = "system_variable"
					}
					results = append(results, CheckResult{
						RuleID:        r.Name(),
						Category:      r.Category(),
						Component:     compType,
						ParameterName: fmt.Sprintf("%s.%s", displayName, fieldPath),
						ParamType:     paramType,
						Severity:      "info",
						RiskLevel:     RiskLevelLow,
						Message:       fmt.Sprintf("Parameter %s.%s in %s has been modified by user (differs from source version default)", displayName, fieldPath, compType),
						Details:       FormatValueDiff(diff.Current, diff.Source),
						CurrentValue:  diff.Current,
						SourceDefault: diff.Source,
						Suggestions: []string{
							"This parameter has been modified from the source version default",
							"Review if this modification is intentional and appropriate",
							"Ensure the modified value is compatible with target version",
						},
					})
				}
			} else {
				// For non-map types, do simple comparison
				// For filename-only parameters, compare by filename only (ignore path)
				var differs bool
				if IsFilenameOnlyParameter(displayName) || IsFilenameOnlyParameter(paramName) {
					differs = !CompareFileNames(currentValue, sourceDefault)
				} else {
					// Use proper value comparison to avoid scientific notation issues
					differs = !CompareValues(currentValue, sourceDefault)
				}

				if differs {
					paramType := "config"
					if isSystemVar {
						paramType = "system_variable"
					}
					results = append(results, CheckResult{
						RuleID:        r.Name(),
						Category:      r.Category(),
						Component:     compType,
						ParameterName: displayName,
						ParamType:     paramType,
						Severity:      "info",
						RiskLevel:     RiskLevelLow,
						Message:       fmt.Sprintf("Parameter %s in %s has been modified by user (differs from source version default)", displayName, compType),
						Details:       FormatValueDiff(currentValue, sourceDefault),
						CurrentValue:  currentValue,
						SourceDefault: sourceDefault,
						Suggestions: []string{
							"This parameter has been modified from the source version default",
							"Review if this modification is intentional and appropriate",
							"Ensure the modified value is compatible with target version",
						},
					})
				}
			}
		}

		// Check reverse direction: Cluster → KB
		// Report parameters that exist in runtime but not in source KB
		for paramName := range runtimeConfigMap {
			// Get current value
			paramValue, ok := component.Config[paramName]
			if !ok {
				continue
			}
			results = append(results, CheckResult{
				RuleID:        r.Name(),
				Category:      r.Category(),
				Component:     compType,
				ParameterName: paramName,
				ParamType:     "config",
				Severity:      "warning",
				RiskLevel:     RiskLevelMedium,
				Message:       fmt.Sprintf("Parameter %s exists in runtime cluster but not found in source KB (v%s)", paramName, ruleCtx.SourceVersion),
				Details:       fmt.Sprintf("Runtime value: %s | Source KB: <not found>", FormatValue(paramValue.Value)),
				CurrentValue:  paramValue.Value,
				Suggestions: []string{
					"This parameter exists in runtime cluster but is missing in source version knowledge base",
					"Verify if this parameter was added in a newer version or is a custom parameter",
					"Check if this is expected behavior or a knowledge base collection issue",
				},
			})
		}

		for varName := range runtimeVarsMap {
			// Get current value
			varValue, ok := component.Variables[varName]
			if !ok {
				continue
			}
			results = append(results, CheckResult{
				RuleID:        r.Name(),
				Category:      r.Category(),
				Component:     compType,
				ParameterName: varName,
				ParamType:     "system_variable",
				Severity:      "warning",
				RiskLevel:     RiskLevelMedium,
				Message:       fmt.Sprintf("System variable %s exists in runtime cluster but not found in source KB (v%s)", varName, ruleCtx.SourceVersion),
				Details:       fmt.Sprintf("Runtime value: %s | Source KB: <not found>", FormatValue(varValue.Value)),
				CurrentValue:  varValue.Value,
				Suggestions: []string{
					"This system variable exists in runtime cluster but is missing in source version knowledge base",
					"Verify if this variable was added in a newer version or is a custom variable",
					"Check if this is expected behavior or a knowledge base collection issue",
				},
			})
		}
	}

	return results, nil
}
