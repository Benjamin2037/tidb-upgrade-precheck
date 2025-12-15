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

		// Compare all source defaults with current runtime values
		// Iterate through source defaults map
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
				// Since validateComponentMapping already verified one-to-one correspondence,
				// this should always exist. If not, it's an unexpected error.
				varName := strings.TrimPrefix(paramName, "sysvar:")
				if varValue, ok := component.Variables[varName]; ok {
					currentValue = varValue.Value
				} else {
					// This should not happen if validateComponentMapping worked correctly
					// Report as an error for investigation
					results = append(results, CheckResult{
						RuleID:        r.Name(),
						Category:      r.Category(),
						Component:     compType,
						ParameterName: varName,
						ParamType:     "system_variable",
						Severity:      "error",
						Message:       fmt.Sprintf("System variable %s in %s was expected but not found in runtime (validation mismatch)", varName, compType),
						Details:       fmt.Sprintf("Source KB has default for %s, but runtime cluster does not have this variable. This indicates a validation issue.", varName),
						Suggestions: []string{
							"Verify that validateComponentMapping detected this mismatch",
							"Check if variable was removed or renamed",
						},
					})
					continue
				}
			} else {
				// Config parameter: check in Config
				// Since validateComponentMapping already verified one-to-one correspondence,
				// this should always exist. If not, it's an unexpected error.
				if paramValue, ok := component.Config[paramName]; ok {
					currentValue = paramValue.Value
				} else {
					// This should not happen if validateComponentMapping worked correctly
					// Report as an error for investigation
					results = append(results, CheckResult{
						RuleID:        r.Name(),
						Category:      r.Category(),
						Component:     compType,
						ParameterName: paramName,
						ParamType:     "config",
						Severity:      "error",
						Message:       fmt.Sprintf("Parameter %s in %s was expected but not found in runtime (validation mismatch)", paramName, compType),
						Details:       fmt.Sprintf("Source KB has default for %s, but runtime cluster does not have this parameter. This indicates a validation issue.", paramName),
						Suggestions: []string{
							"Verify that validateComponentMapping detected this mismatch",
							"Check if parameter was removed or renamed",
						},
					})
					continue
				}
			}

			// Compare values: current cluster value vs source version default
			// If different, it means user has modified this parameter
			if fmt.Sprintf("%v", currentValue) != fmt.Sprintf("%v", sourceDefault) {
				// User has modified this parameter from source version default
				paramType := "config"
				displayName := paramName
				if isSystemVar {
					paramType = "system_variable"
					displayName = strings.TrimPrefix(paramName, "sysvar:")
				}

				results = append(results, CheckResult{
					RuleID:        r.Name(),
					Category:      r.Category(),
					Component:     compType,
					ParameterName: displayName,
					ParamType:     paramType,
					Severity:      "info",       // Info level: user modified parameters are informational
					RiskLevel:     RiskLevelLow, // Low risk: just informational
					Message:       fmt.Sprintf("Parameter %s in %s has been modified by user (differs from source version default)", displayName, compType),
					Details:       fmt.Sprintf("Current cluster value: %v, Source version default: %v", currentValue, sourceDefault),
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

	return results, nil
}
