// Package rules provides standardized rule definitions for upgrade precheck
package rules

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
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

			// Check if this parameter should be ignored
			displayName := paramName
			if isSystemVar {
				displayName = strings.TrimPrefix(paramName, "sysvar:")
			}
			if ignoredParamsForUserModification[displayName] || ignoredParamsForUserModification[paramName] {
				continue
			}

			// For map types, do deep comparison to find only differing fields
			if isMapType(currentValue) && isMapType(sourceDefault) {
				differingFields := compareMapsDeep(currentValue, sourceDefault, paramName, compType)
				for fieldPath, diff := range differingFields {
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
						Details:       formatValueDiff(diff.Current, diff.Source),
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
				if fmt.Sprintf("%v", currentValue) != fmt.Sprintf("%v", sourceDefault) {
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
						Details:       formatValueDiff(currentValue, sourceDefault),
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

// isMapType checks if a value is a map type
func isMapType(v interface{}) bool {
	if v == nil {
		return false
	}
	val := reflect.ValueOf(v)
	return val.Kind() == reflect.Map
}

// convertToMapStringInterface converts various map types to map[string]interface{}
func convertToMapStringInterface(v interface{}) map[string]interface{} {
	if v == nil {
		return nil
	}

	// Direct conversion
	if m, ok := v.(map[string]interface{}); ok {
		return m
	}

	// Try map[interface{}]interface{} (common from YAML unmarshaling)
	if m, ok := v.(map[interface{}]interface{}); ok {
		result := make(map[string]interface{})
		for k, val := range m {
			key := fmt.Sprintf("%v", k)
			result[key] = val
		}
		return result
	}

	// Use reflection for other map types
	val := reflect.ValueOf(v)
	if val.Kind() != reflect.Map {
		return nil
	}

	result := make(map[string]interface{})
	for _, key := range val.MapKeys() {
		keyStr := fmt.Sprintf("%v", key.Interface())
		result[keyStr] = val.MapIndex(key).Interface()
	}
	return result
}

// mapDiff represents a difference between two map values
type mapDiff struct {
	Current interface{}
	Source  interface{}
}

// compareMapsDeep compares two maps and returns only the differing fields
// Returns a map of field paths to their differences
func compareMapsDeep(current, source interface{}, basePath, component string) map[string]mapDiff {
	result := make(map[string]mapDiff)

	// Convert to map[string]interface{} if possible
	currentMap := convertToMapStringInterface(current)
	sourceMap := convertToMapStringInterface(source)

	if currentMap == nil || sourceMap == nil {
		// If not both maps, fall back to simple comparison
		if fmt.Sprintf("%v", current) != fmt.Sprintf("%v", source) {
			result[""] = mapDiff{Current: current, Source: source}
		}
		return result
	}

	// Check all fields in current map
	for key, currentVal := range currentMap {
		sourceVal, exists := sourceMap[key]
		// Only check top-level parameter name for ignore list (not nested map fields)
		// For nested maps, we want to show all differences
		if basePath == "" && ignoredParamsForUserModification[key] {
			continue
		}

		// Build field path for recursive calls (but don't use for ignore checks)
		fieldPath := key
		if basePath != "" {
			fieldPath = fmt.Sprintf("%s.%s", basePath, key)
		}

		if !exists {
			// Field exists in current but not in source
			result[key] = mapDiff{Current: currentVal, Source: nil}
		} else if isMapType(currentVal) && isMapType(sourceVal) {
			// Recursively compare nested maps
			nestedDiffs := compareMapsDeep(currentVal, sourceVal, fieldPath, component)
			for nestedKey, nestedDiff := range nestedDiffs {
				if nestedKey == "" {
					result[key] = nestedDiff
				} else {
					result[fmt.Sprintf("%s.%s", key, nestedKey)] = nestedDiff
				}
			}
		} else if fmt.Sprintf("%v", currentVal) != fmt.Sprintf("%v", sourceVal) {
			// Simple value comparison
			result[key] = mapDiff{Current: currentVal, Source: sourceVal}
		}
	}

	// Check fields in source map that don't exist in current
	for key, sourceVal := range sourceMap {
		if _, exists := currentMap[key]; !exists {
			// Only check top-level parameter name for ignore list (not nested map fields)
			if basePath == "" && ignoredParamsForUserModification[key] {
				continue
			}
			result[key] = mapDiff{Current: nil, Source: sourceVal}
		}
	}

	return result
}

// formatValueDiff formats the difference between current and source values in a clear, readable way
func formatValueDiff(current, source interface{}) string {
	var currentStr, sourceStr string

	// Format current value
	if current == nil {
		currentStr = "<not set>"
	} else {
		currentStr = formatValue(current)
	}

	// Format source value
	if source == nil {
		sourceStr = "<not set>"
	} else {
		sourceStr = formatValue(source)
	}

	// For simple values (non-map, non-slice), use compact format
	if !isMapType(current) && !isMapType(source) && !isSliceType(current) && !isSliceType(source) {
		return fmt.Sprintf("Current: %s | Source Default: %s", currentStr, sourceStr)
	}

	// For complex types, use multi-line format
	return fmt.Sprintf("Current Value:\n%s\n\nSource Default:\n%s", currentStr, sourceStr)
}

// isSliceType checks if a value is a slice or array type
func isSliceType(v interface{}) bool {
	if v == nil {
		return false
	}
	val := reflect.ValueOf(v)
	return val.Kind() == reflect.Slice || val.Kind() == reflect.Array
}

// formatValue formats a value in a readable way
func formatValue(v interface{}) string {
	if v == nil {
		return "<nil>"
	}

	// For map/slice types, use JSON formatting for readability
	val := reflect.ValueOf(v)
	switch val.Kind() {
	case reflect.Map, reflect.Slice, reflect.Array:
		jsonBytes, err := json.MarshalIndent(v, "", "  ")
		if err == nil {
			return string(jsonBytes)
		}
		// Fall back to simple format if JSON marshaling fails
		return fmt.Sprintf("%v", v)
	case reflect.String:
		return fmt.Sprintf("%q", v)
	default:
		return fmt.Sprintf("%v", v)
	}
}
