// Package rules provides standardized rule definitions for upgrade precheck
package rules

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/pingcap/tidb-upgrade-precheck/pkg/collector"
	defaultsTypes "github.com/pingcap/tidb-upgrade-precheck/pkg/types"
)

// RuleContext provides all the data needed for rule evaluation
// The data is loaded based on rules' DataRequirements
type RuleContext struct {
	// SourceClusterSnapshot contains the actual sampled data from the running cluster
	// Only contains data for components specified in rules' requirements
	SourceClusterSnapshot *collector.ClusterSnapshot

	// SourceVersion is the current cluster version
	SourceVersion string

	// TargetVersion is the target version for upgrade
	TargetVersion string

	// SourceBootstrapVersion is the bootstrap version of the source version
	// This is used to filter upgrade logic changes by bootstrap version range (X, Y]
	SourceBootstrapVersion int64

	// TargetBootstrapVersion is the bootstrap version of the target version
	// This is used to filter upgrade logic changes by bootstrap version range (X, Y]
	TargetBootstrapVersion int64

	// SourceDefaults contains the source version default values from knowledge base
	// Structure: map[component]map[param_name]default_value
	// Only contains data for components and types specified in rules' requirements
	SourceDefaults map[string]map[string]interface{}

	// TargetDefaults contains the target version default values from knowledge base
	// Structure: map[component]map[param_name]default_value
	// Only contains data for components and types specified in rules' requirements
	TargetDefaults map[string]map[string]interface{}

	// UpgradeLogic contains all forced changes and upgrade logic for all components
	// Structure: map[component]upgrade_logic_data
	// Each component's upgrade_logic contains all historical changes with version tags
	// Only loaded if rules require it
	// Changes are filtered by version range (sourceVersion, targetVersion] during evaluation
	UpgradeLogic map[string]interface{}
}

// NewRuleContext creates a new rule context
func NewRuleContext(
	sourceSnapshot *collector.ClusterSnapshot,
	sourceVersion, targetVersion string,
	sourceDefaults, targetDefaults map[string]map[string]interface{},
	upgradeLogic map[string]interface{},
	sourceBootstrapVersion, targetBootstrapVersion int64,
) *RuleContext {
	return &RuleContext{
		SourceClusterSnapshot:  sourceSnapshot,
		SourceVersion:          sourceVersion,
		TargetVersion:          targetVersion,
		SourceDefaults:         sourceDefaults,
		TargetDefaults:         targetDefaults,
		UpgradeLogic:           upgradeLogic,
		SourceBootstrapVersion: sourceBootstrapVersion,
		TargetBootstrapVersion: targetBootstrapVersion,
	}
}

// GetSampledValue gets a value from the actual sampled cluster data (current runtime value)
// component: "tidb", "pd", "tikv", "tiflash" or component name with address (e.g., "tikv-192-168-1-100-20160")
// paramName: parameter name or config key
// Returns the actual value currently configured in the cluster
func (ctx *RuleContext) GetSampledValue(component, paramName string) interface{} {
	if ctx.SourceClusterSnapshot == nil {
		return nil
	}

	// Try exact match first
	if comp, ok := ctx.SourceClusterSnapshot.Components[component]; ok {
		// Check in config (Config is ConfigDefaults, need to extract Value from ParameterValue)
		if paramValue, ok := comp.Config[paramName]; ok {
			return paramValue.Value
		}
		// Check in variables (for TiDB system variables, Variables is SystemVariables, need to extract Value)
		if paramValue, ok := comp.Variables[paramName]; ok {
			return paramValue.Value
		}
	}

	// Try to find by component type prefix (for TiKV nodes with address-based keys)
	for compName, comp := range ctx.SourceClusterSnapshot.Components {
		// Check if this component matches the requested component type
		if string(comp.Type) == component ||
			(component == "tikv" && (comp.Type == collector.TiKVComponent ||
				(len(compName) > 4 && compName[:4] == "tikv"))) {
			if paramValue, ok := comp.Config[paramName]; ok {
				return paramValue.Value
			}
			if paramValue, ok := comp.Variables[paramName]; ok {
				return paramValue.Value
			}
		}
	}

	return nil
}

// GetSourceDefault gets the default value for a parameter in source version
// component: "tidb", "pd", "tikv", "tiflash"
// paramName: parameter name (for system variables, use "sysvar:variable_name")
// Returns the default value from source version knowledge base
func (ctx *RuleContext) GetSourceDefault(component, paramName string) interface{} {
	if comp, ok := ctx.SourceDefaults[component]; ok {
		if val, ok := comp[paramName]; ok {
			return extractValueFromDefault(val)
		}
	}
	return nil
}

// GetTargetDefault gets the default value for a parameter in target version
// component: "tidb", "pd", "tikv", "tiflash"
// paramName: parameter name (for system variables, use "sysvar:variable_name")
// Returns the default value from target version knowledge base (what it will be after upgrade)
func (ctx *RuleContext) GetTargetDefault(component, paramName string) interface{} {
	if comp, ok := ctx.TargetDefaults[component]; ok {
		if val, ok := comp[paramName]; ok {
			return extractValueFromDefault(val)
		}
	}
	return nil
}

// IsUserModified checks if a parameter has been modified by the user
// Returns true if the sampled value differs from the source default
func (ctx *RuleContext) IsUserModified(component, paramName string) bool {
	sampled := ctx.GetSampledValue(component, paramName)
	sourceDefault := ctx.GetSourceDefault(component, paramName)

	if sampled == nil && sourceDefault == nil {
		return false
	}
	if sampled == nil || sourceDefault == nil {
		return true
	}

	// Simple comparison - in production, might need type-aware comparison
	return fmt.Sprintf("%v", sampled) != fmt.Sprintf("%v", sourceDefault)
}

// WillDefaultChange checks if the default value will change between source and target versions
// Returns true if source default != target default
func (ctx *RuleContext) WillDefaultChange(component, paramName string) bool {
	sourceDefault := ctx.GetSourceDefault(component, paramName)
	targetDefault := ctx.GetTargetDefault(component, paramName)

	if sourceDefault == nil && targetDefault == nil {
		return false
	}
	if sourceDefault == nil || targetDefault == nil {
		return true
	}

	return fmt.Sprintf("%v", sourceDefault) != fmt.Sprintf("%v", targetDefault)
}

// GetForcedChanges extracts forced changes from upgrade logic
// Filters changes by bootstrap version range: (sourceBootstrapVersion, targetBootstrapVersion]
// The upgrade_logic.json contains changes with bootstrap version numbers (e.g., "68", "71")
// We filter changes where: sourceBootstrapVersion < changeBootstrapVersion <= targetBootstrapVersion
// Returns a map of parameter name to forced value
func (ctx *RuleContext) GetForcedChanges(component string) map[string]interface{} {
	result := make(map[string]interface{})

	// Debug: Check if upgrade_logic is loaded
	if len(ctx.UpgradeLogic) == 0 {
		fmt.Printf("[DEBUG GetForcedChanges] No upgrade_logic loaded for any component\n")
		return result
	}

	if logic, ok := ctx.UpgradeLogic[component]; ok {
		fmt.Printf("[DEBUG GetForcedChanges] Found upgrade_logic for component %s, SourceBootstrap=%d, TargetBootstrap=%d\n", component, ctx.SourceBootstrapVersion, ctx.TargetBootstrapVersion)
		// Parse upgrade logic structure
		// Expected structure: UpgradeLogicSnapshot with Changes array
		// Each change has a Version field that contains bootstrap version number (e.g., "68", "71")
		if logicMap, ok := logic.(map[string]interface{}); ok {
			// Try UpgradeLogicSnapshot format: {"component": "...", "changes": [...]}
			if changes, ok := logicMap["changes"].([]interface{}); ok {
				for _, change := range changes {
					if changeMap, ok := change.(map[string]interface{}); ok {
						// Get bootstrap version from change
						// Version field in upgrade_logic.json is bootstrap version (e.g., "68", "71")
						var changeBootstrapVersion int64
						if versionStr, ok := changeMap["version"].(string); ok {
							// Version is a string like "68" or "71"
							if versionNum, err := strconv.ParseInt(versionStr, 10, 64); err == nil {
								changeBootstrapVersion = versionNum
							} else {
								continue
							}
						} else if versionNum, ok := changeMap["version"].(float64); ok {
							// Version is a number (JSON unmarshaled as float64)
							changeBootstrapVersion = int64(versionNum)
						} else {
							continue
						}

						// Check if bootstrap version is in range (sourceBootstrapVersion, targetBootstrapVersion]
						// This means: sourceBootstrapVersion < changeBootstrapVersion <= targetBootstrapVersion
						if ctx.SourceBootstrapVersion > 0 && ctx.TargetBootstrapVersion > 0 {
							// Debug: Check if version is in range
							if changeBootstrapVersion > ctx.SourceBootstrapVersion && changeBootstrapVersion <= ctx.TargetBootstrapVersion {
								fmt.Printf("[DEBUG GetForcedChanges] Change version %d is in range (%d, %d]\n", changeBootstrapVersion, ctx.SourceBootstrapVersion, ctx.TargetBootstrapVersion)
								// Extract parameter name and value
								var paramName string
								var forcedValue interface{}

								// Try different field names for parameter name
								if name, ok := changeMap["name"].(string); ok {
									paramName = name
								} else if varName, ok := changeMap["var_name"].(string); ok {
									paramName = varName
								} else if target, ok := changeMap["target"].(string); ok {
									paramName = target
								} else {
									continue
								}

								// Extract forced value
								if value, ok := changeMap["value"]; ok {
									forcedValue = value
								} else if defaultVal, ok := changeMap["default_value"]; ok {
									forcedValue = defaultVal
								} else {
									continue
								}

								result[paramName] = forcedValue
								fmt.Printf("[DEBUG GetForcedChanges] Added forced change: %s = %v (version: %d)\n", paramName, forcedValue, changeBootstrapVersion)
							} else {
								fmt.Printf("[DEBUG GetForcedChanges] Change version %d is NOT in range (%d, %d]\n", changeBootstrapVersion, ctx.SourceBootstrapVersion, ctx.TargetBootstrapVersion)
							}
						} else {
							fmt.Printf("[DEBUG GetForcedChanges] Bootstrap versions not set (Source=%d, Target=%d), using fallback\n", ctx.SourceBootstrapVersion, ctx.TargetBootstrapVersion)
							// Fallback to release version comparison if bootstrap versions are not available
							// This maintains backward compatibility
							changeVersion := fmt.Sprintf("%d", changeBootstrapVersion)
							if isVersionInRange(changeVersion, ctx.SourceVersion, ctx.TargetVersion) {
								var paramName string
								var forcedValue interface{}

								if name, ok := changeMap["name"].(string); ok {
									paramName = name
								} else if varName, ok := changeMap["var_name"].(string); ok {
									paramName = varName
								} else {
									continue
								}

								if value, ok := changeMap["value"]; ok {
									forcedValue = value
								} else {
									continue
								}

								result[paramName] = forcedValue
							}
						}
					}
				}
			}
		} else {
			fmt.Printf("[DEBUG GetForcedChanges] upgrade_logic for component %s is not a map[string]interface{}\n", component)
		}
	} else {
		fmt.Printf("[DEBUG GetForcedChanges] No upgrade_logic found for component %s (available components: %v)\n", component, getMapKeys(ctx.UpgradeLogic))
	}

	fmt.Printf("[DEBUG GetForcedChanges] Returning %d forced changes for component %s\n", len(result), component)
	return result
}

// GetForcedChangeForValue gets the forced change value for a specific parameter and current value
// This method matches the from_value field in upgrade_logic.json to determine the correct forced value
// Returns the forced value if a match is found, nil otherwise
func (ctx *RuleContext) GetForcedChangeForValue(component, paramName string, currentValue interface{}) interface{} {
	if len(ctx.UpgradeLogic) == 0 {
		return nil
	}

	if logic, ok := ctx.UpgradeLogic[component]; ok {
		if logicMap, ok := logic.(map[string]interface{}); ok {
			if changes, ok := logicMap["changes"].([]interface{}); ok {
				currentValueStr := fmt.Sprintf("%v", currentValue)
				
				for _, change := range changes {
					if changeMap, ok := change.(map[string]interface{}); ok {
						// Get bootstrap version from change
						var changeBootstrapVersion int64
						if versionStr, ok := changeMap["version"].(string); ok {
							if versionNum, err := strconv.ParseInt(versionStr, 10, 64); err == nil {
								changeBootstrapVersion = versionNum
							} else {
								continue
							}
						} else if versionNum, ok := changeMap["version"].(float64); ok {
							changeBootstrapVersion = int64(versionNum)
						} else {
							continue
						}

						// Check if bootstrap version is in range
						var versionInRange bool
						if ctx.SourceBootstrapVersion > 0 && ctx.TargetBootstrapVersion > 0 {
							versionInRange = changeBootstrapVersion > ctx.SourceBootstrapVersion && changeBootstrapVersion <= ctx.TargetBootstrapVersion
						} else {
							// Fallback to release version comparison
							changeVersion := fmt.Sprintf("%d", changeBootstrapVersion)
							versionInRange = isVersionInRange(changeVersion, ctx.SourceVersion, ctx.TargetVersion)
						}

						if !versionInRange {
							continue
						}

						// Check if parameter name matches
						var changeParamName string
						if name, ok := changeMap["name"].(string); ok {
							changeParamName = name
						} else if varName, ok := changeMap["var_name"].(string); ok {
							changeParamName = varName
						} else {
							continue
						}

						if changeParamName != paramName {
							continue
						}

						// Check if from_value matches current value
						if fromValue, ok := changeMap["from_value"]; ok {
							fromValueStr := fmt.Sprintf("%v", fromValue)
							if fromValueStr != currentValueStr {
								// from_value doesn't match current value, skip this entry
								continue
							}
						}

						// Extract forced value
						if value, ok := changeMap["value"]; ok {
							return value
						} else if defaultVal, ok := changeMap["default_value"]; ok {
							return defaultVal
						}
					}
				}
			}
		}
	}

	return nil
}

// Helper function to get map keys for debugging
func getMapKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// isVersionInRange checks if a version is in the range (sourceVersion, targetVersion]
// Returns true if sourceVersion < changeVersion <= targetVersion
func isVersionInRange(changeVersion, sourceVersion, targetVersion string) bool {
	// Normalize versions (remove 'v' prefix if present)
	changeVersion = strings.TrimPrefix(changeVersion, "v")
	sourceVersion = strings.TrimPrefix(sourceVersion, "v")
	targetVersion = strings.TrimPrefix(targetVersion, "v")

	// Compare versions
	// If changeVersion > sourceVersion && changeVersion <= targetVersion, return true
	compareToSource := compareVersions(changeVersion, sourceVersion)
	compareToTarget := compareVersions(changeVersion, targetVersion)

	// changeVersion > sourceVersion && changeVersion <= targetVersion
	return compareToSource > 0 && compareToTarget <= 0
}

// compareVersions compares two version strings
// Returns: -1 if v1 < v2, 0 if v1 == v2, 1 if v1 > v2
func compareVersions(v1, v2 string) int {
	// Split versions by '.'
	parts1 := strings.Split(v1, ".")
	parts2 := strings.Split(v2, ".")

	// Compare each part
	maxLen := len(parts1)
	if len(parts2) > maxLen {
		maxLen = len(parts2)
	}

	for i := 0; i < maxLen; i++ {
		var num1, num2 int
		if i < len(parts1) {
			num1, _ = strconv.Atoi(parts1[i])
		}
		if i < len(parts2) {
			num2, _ = strconv.Atoi(parts2[i])
		}

		if num1 < num2 {
			return -1
		} else if num1 > num2 {
			return 1
		}
	}

	return 0
}

// Helper function to extract value from default (handles ParameterValue structures)
func extractValueFromDefault(defaultValue interface{}) interface{} {
	if defaultValue == nil {
		return nil
	}

	// If it's a ParameterValue, extract the Value field
	if paramValue, ok := defaultValue.(defaultsTypes.ParameterValue); ok {
		return paramValue.Value
	}

	// If it's a map with "value" key (JSON unmarshaled ParameterValue)
	if paramMap, ok := defaultValue.(map[string]interface{}); ok {
		if value, ok := paramMap["value"]; ok {
			return value
		}
	}

	// Otherwise, return as-is
	return defaultValue
}
