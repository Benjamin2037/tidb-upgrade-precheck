// Package rules provides standardized rule definitions for upgrade precheck
package rules

import (
	"context"
	"fmt"
	"strings"

	"github.com/pingcap/tidb-upgrade-precheck/pkg/collector"
)

// Parameters that should be ignored when comparing user modifications
// These are typically deployment/environment-specific and not user modifications
// Note: Only top-level parameter names are ignored, not nested map fields
var ignoredParamsForUserModification = map[string]bool{
	// Path-related parameters (deployment-specific, top-level only)
	"data-dir":   true,
	"log-dir":    true,
	"deploy-dir": true,

	// Compile-time platform information (not user-configurable)
	"version_compile_machine": true, // Compilation machine architecture (e.g., amd64, arm64)
	"version_compile_os":      true, // Compilation OS (e.g., linux, darwin)
}

// isResourceDependentParameter is a wrapper for the shared IsResourceDependentParameter function
// Kept for backward compatibility with existing code
func isResourceDependentParameter(paramName string) bool {
	return IsResourceDependentParameter(paramName)
}

// isAutoTuneParameter is kept for backward compatibility
// Deprecated: Use isResourceDependentParameter instead
func isAutoTuneParameter(paramName string) bool {
	return IsResourceDependentParameter(paramName)
}

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
				// If variable doesn't exist in runtime, skip it
				// validateComponentMapping already reports variable mismatches as warnings
				// We can't compare a variable that doesn't exist, so just skip it
				varName := strings.TrimPrefix(paramName, "sysvar:")
				if varValue, ok := component.Variables[varName]; ok {
					currentValue = varValue.Value
				} else {
					// Variable doesn't exist in runtime - skip it
					// This is normal for optional variables or variables removed in certain versions
					// validateComponentMapping will report this as a warning if needed
					continue
				}
			} else {
				// Config parameter: check in Config
				// If parameter doesn't exist in runtime, skip it
				// validateComponentMapping already reports parameter mismatches as warnings
				// We can't compare a parameter that doesn't exist, so just skip it
				if paramValue, ok := component.Config[paramName]; ok {
					currentValue = paramValue.Value
				} else {
					// Parameter doesn't exist in runtime - skip it
					// This is normal for optional parameters or parameters removed in certain versions
					// validateComponentMapping will report this as a warning if needed
					continue
				}
			}

			// Check if this parameter should be ignored
			displayName := paramName
			if isSystemVar {
				displayName = strings.TrimPrefix(paramName, "sysvar:")
			}
			if ignoredParamsForUserModification[displayName] || ignoredParamsForUserModification[paramName] {
				continue
			}

			// PD Component: All current values will be kept during upgrade (compatible)
			// Filter all PD config parameters (not system variables) as they will be kept
			if compType == "pd" && !isSystemVar && currentValue != nil {
				// PD maintains existing configuration, upgrade is compatible
				// Skip reporting as current value will be kept
				continue
			}

			// Note: IsPathParameter filtering is done at report generation time, not here
			// This ensures all parameters are properly categorized before filtering

			// Check if this is a resource-dependent parameter
			// If source default == target default but current differs, it's likely adjusted by TiKV/TiFlash
			// based on system resources (CPU cores, memory, etc.)
			// Skip reporting these as "user modified" to avoid false positives
			if isResourceDependentParameter(displayName) || isResourceDependentParameter(paramName) {
				targetDefault := ruleCtx.GetTargetDefault(compType, paramName)
				if targetDefault != nil {
					sourceEqualsTarget := CompareValues(sourceDefault, targetDefault)
					if sourceEqualsTarget {
						// Source default == target default, but current differs
						// This is likely auto-tuned by TiKV based on system resources
						// Skip reporting as "user modified"
						continue
					}
				}
			}

			// For map types, do deep comparison to find only differing fields
			if IsMapType(currentValue) && IsMapType(sourceDefault) {
				opts := CompareOptions{
					IgnoredParams: ignoredParamsForUserModification,
					BasePath:      paramName,
				}
				differingFields := CompareMapsDeep(currentValue, sourceDefault, opts)
				for fieldPath, diff := range differingFields {
					// Check if this nested field is a resource-dependent parameter
					// For nested fields like "backup.auto-tune-remain-threads" or "backup.num-threads", check the full path
					fullFieldPath := fmt.Sprintf("%s.%s", displayName, fieldPath)
					if isResourceDependentParameter(fieldPath) || isResourceDependentParameter(fullFieldPath) {
						// Get target default for the parent map
						targetDefaultMap := ruleCtx.GetTargetDefault(compType, paramName)
						if targetDefaultMap != nil {
							// Extract the nested field value from target default
							targetDefaultValue := getNestedMapValue(ConvertToMapStringInterface(targetDefaultMap), strings.Split(fieldPath, "."))
							if targetDefaultValue != nil {
								sourceEqualsTarget := CompareValues(diff.Source, targetDefaultValue)
								if sourceEqualsTarget {
									// Source default == target default, but current differs
									// This is likely auto-tuned by TiKV based on system resources
									// Skip reporting as "user modified"
									continue
								}
							}
						}
					}

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
				if filenameOnlyParams[displayName] || filenameOnlyParams[paramName] {
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
	}

	return results, nil
}
