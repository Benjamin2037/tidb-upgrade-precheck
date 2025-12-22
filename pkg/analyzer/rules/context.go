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

	// ParameterNotes contains special notes/descriptions for parameters
	// Structure: map[component]map[param_type]map[param_name]note_info
	// Only loaded if needed
	ParameterNotes map[string]interface{}
}

// NewRuleContext creates a new rule context
func NewRuleContext(
	sourceSnapshot *collector.ClusterSnapshot,
	sourceVersion, targetVersion string,
	sourceDefaults, targetDefaults map[string]map[string]interface{},
	upgradeLogic map[string]interface{},
	sourceBootstrapVersion, targetBootstrapVersion int64,
	parameterNotes map[string]interface{},
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
		ParameterNotes:         parameterNotes,
	}
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

// GetForcedChanges extracts forced changes from upgrade logic
// Filters changes by bootstrap version range: (sourceBootstrapVersion, targetBootstrapVersion]
// The upgrade_logic.json contains changes with bootstrap version numbers (e.g., "68", "71")
// We filter changes where: sourceBootstrapVersion < changeBootstrapVersion <= targetBootstrapVersion
// Returns a map of parameter name to forced value
func (ctx *RuleContext) GetForcedChanges(component string) map[string]interface{} {
	result := make(map[string]interface{})

	if len(ctx.UpgradeLogic) == 0 {
		return result
	}

	if logic, ok := ctx.UpgradeLogic[component]; ok {
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
							if changeBootstrapVersion > ctx.SourceBootstrapVersion && changeBootstrapVersion <= ctx.TargetBootstrapVersion {
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
							}
						} else {
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
		}
	}

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

// ForcedChangeMetadata contains special handling metadata for a forced change
type ForcedChangeMetadata struct {
	DetailsNote    string   // Additional note to append to details message
	Suggestions    []string // Custom suggestions (if nil, use default)
	ReportSeverity string   // Override report severity: "error", "warning", "info" (if empty, use default)
}

// GetForcedChangeMetadata gets special handling metadata for a forced change
// Returns metadata if found, nil otherwise
func (ctx *RuleContext) GetForcedChangeMetadata(component, paramName string, currentValue interface{}) *ForcedChangeMetadata {
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

						// Check if from_value matches current value (if specified)
						if fromValue, ok := changeMap["from_value"]; ok {
							fromValueStr := fmt.Sprintf("%v", fromValue)
							if fromValueStr != currentValueStr {
								// from_value doesn't match current value, skip this entry
								continue
							}
						}

						// Extract metadata
						metadata := &ForcedChangeMetadata{}
						hasMetadata := false

						// Extract details_note
						if detailsNote, ok := changeMap["details_note"].(string); ok && detailsNote != "" {
							metadata.DetailsNote = detailsNote
							hasMetadata = true
						}

						// Extract suggestions
						if suggestions, ok := changeMap["suggestions"].([]interface{}); ok && len(suggestions) > 0 {
							metadata.Suggestions = make([]string, 0, len(suggestions))
							for _, s := range suggestions {
								if str, ok := s.(string); ok {
									metadata.Suggestions = append(metadata.Suggestions, str)
								}
							}
							if len(metadata.Suggestions) > 0 {
								hasMetadata = true
							}
						}

						// Extract report_severity
						if reportSeverity, ok := changeMap["report_severity"].(string); ok && reportSeverity != "" {
							metadata.ReportSeverity = reportSeverity
							hasMetadata = true
						}

						// Return metadata if any field is set
						if hasMetadata {
							return metadata
						}
					}
				}
			}
		}
	}

	return nil
}

// GetParameterNote gets special note/description for a parameter from knowledge base
// Returns the note if found and conditions match, empty string otherwise
func (ctx *RuleContext) GetParameterNote(component, paramName, paramType string, targetDefault interface{}) string {
	if len(ctx.ParameterNotes) == 0 {
		return ""
	}

	compNotes, ok := ctx.ParameterNotes[component]
	if !ok {
		return ""
	}

	compNotesMap, ok := compNotes.(map[string]interface{})
	if !ok {
		return ""
	}

	// Get param type map (config or system_variables)
	var typeMap map[string]interface{}
	if paramType == "config" {
		if configMap, ok := compNotesMap["config"].(map[string]interface{}); ok {
			typeMap = configMap
		}
	} else if paramType == "system_variable" {
		if sysVarMap, ok := compNotesMap["system_variables"].(map[string]interface{}); ok {
			typeMap = sysVarMap
		}
	}

	if typeMap == nil {
		return ""
	}

	// Get parameter note info
	paramNoteInfo, ok := typeMap[paramName]
	if !ok {
		return ""
	}

	noteInfoMap, ok := paramNoteInfo.(map[string]interface{})
	if !ok {
		return ""
	}

	// Check conditions if specified
	if condition, ok := noteInfoMap["condition"].(map[string]interface{}); ok {
		// Check target_default_value condition
		if targetDefaultValue, ok := condition["target_default_value"]; ok {
			if !CompareValues(targetDefault, targetDefaultValue) {
				return "" // Condition not met
			}
		}

		// Check target_version_min condition
		if targetVersionMin, ok := condition["target_version_min"].(string); ok {
			if compareVersions(ctx.TargetVersion, targetVersionMin) < 0 {
				return "" // Target version is before minimum
			}
		}
	}

	// Extract details_note
	if detailsNote, ok := noteInfoMap["details_note"].(string); ok && detailsNote != "" {
		return detailsNote
	}

	return ""
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
