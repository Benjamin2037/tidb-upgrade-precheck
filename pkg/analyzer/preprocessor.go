// Package analyzer provides risk analysis logic for upgrade precheck
package analyzer

import (
	"fmt"
	"strings"

	"github.com/pingcap/tidb-upgrade-precheck/pkg/analyzer/rules"
	"github.com/pingcap/tidb-upgrade-precheck/pkg/collector"
)

// extractValueFromDefault extracts the actual value from a ParameterValue structure
func extractValueFromDefault(defaultValue interface{}) interface{} {
	if defaultValue == nil {
		return nil
	}

	// If it's already a simple value, return it
	if paramValue, ok := defaultValue.(map[string]interface{}); ok {
		if value, ok := paramValue["value"]; ok {
			return value
		}
		// If no "value" key, return the whole map (might be a map type parameter)
		return defaultValue
	}

	return defaultValue
}

// preprocessParameters preprocesses parameters before rule evaluation:
// 1. Extracts and processes parameters that should be filtered (path parameters, deployment-specific, etc.)
// 2. Extracts and processes forced changes from upgrade_logic.json
// 3. Extracts and processes parameters with special notes from parameter_notes.json
// 4. Removes processed parameters from sourceDefaults and targetDefaults to reduce rule comparison overhead
// Returns: preprocessed results and cleaned defaults maps
func (a *Analyzer) preprocessParameters(
	snapshot *collector.ClusterSnapshot,
	sourceVersion, targetVersion string,
	sourceDefaults, targetDefaults map[string]map[string]interface{},
	upgradeLogic map[string]interface{},
	parameterNotes map[string]interface{},
	sourceBootstrapVersion, targetBootstrapVersion int64,
) ([]rules.CheckResult, map[string]map[string]interface{}, map[string]map[string]interface{}) {
	var preprocessedResults []rules.CheckResult

	// Create cleaned defaults maps (will remove processed parameters)
	cleanedSourceDefaults := make(map[string]map[string]interface{})
	cleanedTargetDefaults := make(map[string]map[string]interface{})

	// Process each component
	for compType := range sourceDefaults {
		cleanedSourceDefaults[compType] = make(map[string]interface{})
		if targetDefaults[compType] != nil {
			cleanedTargetDefaults[compType] = make(map[string]interface{})
		}

		// Get component data from snapshot
		var component *collector.ComponentState
		for compName, comp := range snapshot.Components {
			if string(comp.Type) == compType || strings.HasPrefix(compName, compType) {
				component = &comp
				break
			}
		}

		// Process all parameters in source defaults
		for paramName, sourceDefaultValue := range sourceDefaults[compType] {
			// Determine parameter type
			isSystemVar := strings.HasPrefix(paramName, "sysvar:")
			var displayName string
			var paramType string
			if isSystemVar {
				displayName = strings.TrimPrefix(paramName, "sysvar:")
				paramType = "system_variable"
			} else {
				displayName = paramName
				paramType = "config"
			}

			// Check if this parameter should be filtered (deployment-specific, path parameters, etc.)
			shouldFilter, filterReason := ShouldFilterParameter(displayName)
			if !shouldFilter {
				// Also check with full paramName (for system variables with "sysvar:" prefix)
				shouldFilter, filterReason = ShouldFilterParameter(paramName)
			}

			// Check if all three values are the same (no difference to report)
			if !shouldFilter && component != nil {
				var currentValue interface{}
				if isSystemVar {
					if varValue, ok := component.Variables[displayName]; ok {
						currentValue = varValue.Value
					}
				} else {
					if paramValue, ok := component.Config[displayName]; ok {
						currentValue = paramValue.Value
					}
				}

				if currentValue != nil {
					sourceDefault := extractValueFromDefault(sourceDefaultValue)
					var targetDefault interface{}
					if targetDefaults[compType] != nil {
						if targetDefaultValue, ok := targetDefaults[compType][paramName]; ok {
							targetDefault = extractValueFromDefault(targetDefaultValue)
						}
					}

					// If all three values are the same, filter
					if sourceDefault != nil && targetDefault != nil {
						if rules.CompareValues(currentValue, sourceDefault) &&
							rules.CompareValues(currentValue, targetDefault) &&
							rules.CompareValues(sourceDefault, targetDefault) {
							shouldFilter = true
							filterReason = "all values identical (no difference)"
						}
					}

					// Check resource-dependent parameters
					// If source default == target default, but current differs, filter (auto-tuned by system)
					if !shouldFilter && IsResourceDependentParameter(displayName) {
						if sourceDefault != nil && targetDefault != nil {
							if rules.CompareValues(sourceDefault, targetDefault) &&
								!rules.CompareValues(currentValue, sourceDefault) {
								// Source default == target default, but current differs
								// This is likely auto-tuned by TiKV/TiFlash based on system resources
								shouldFilter = true
								filterReason = "resource-dependent parameter (auto-tuned, source == target)"
							}
						}
					}
				}
			}

			// If should filter, generate CheckResult for filtered parameter and skip adding to cleaned defaults
			if shouldFilter {
				// Get current value and defaults for the filtered parameter
				var currentValue interface{}
				var sourceDefault interface{}
				var targetDefault interface{}

				if component != nil {
					if isSystemVar {
						if varValue, ok := component.Variables[displayName]; ok {
							currentValue = varValue.Value
						}
					} else {
						if paramValue, ok := component.Config[displayName]; ok {
							currentValue = paramValue.Value
						}
					}
				}

				sourceDefault = extractValueFromDefault(sourceDefaultValue)
				if targetDefaults[compType] != nil {
					if targetDefaultValue, ok := targetDefaults[compType][paramName]; ok {
						targetDefault = extractValueFromDefault(targetDefaultValue)
					}
				}

				// Determine severity based on filter reason
				severity := "info"
				if strings.Contains(filterReason, "deployment-specific") || strings.Contains(filterReason, "path parameter") {
					severity = "info" // Deployment-specific parameters are informational
				} else if strings.Contains(filterReason, "resource-dependent") {
					severity = "info" // Auto-tuned parameters are informational
				} else if strings.Contains(filterReason, "all values identical") {
					severity = "info" // No difference, informational
				}

				// Build details message
				details := fmt.Sprintf("This parameter has been filtered from detailed analysis.\nReason: %s", filterReason)
				if currentValue != nil {
					details += fmt.Sprintf("\n\nCurrent Value: %v", rules.FormatValue(currentValue))
				}
				if sourceDefault != nil {
					details += fmt.Sprintf("\nSource Default: %v", rules.FormatValue(sourceDefault))
				}
				if targetDefault != nil {
					details += fmt.Sprintf("\nTarget Default: %v", rules.FormatValue(targetDefault))
				}

				// Add note about why it's filtered
				if strings.Contains(filterReason, "deployment-specific") || strings.Contains(filterReason, "path parameter") {
					details += "\n\nNote: This parameter varies by deployment environment and does not require user action during upgrade."
				} else if strings.Contains(filterReason, "resource-dependent") {
					details += "\n\nNote: This parameter is automatically adjusted by the system based on available resources (CPU cores, memory, etc.)."
				} else if strings.Contains(filterReason, "all values identical") {
					details += "\n\nNote: All values (current, source default, target default) are identical. No action needed."
				}

				preprocessedResults = append(preprocessedResults, rules.CheckResult{
					RuleID:        "PARAMETER_PREPROCESSOR",
					Category:      "filtered",
					Component:     compType,
					ParameterName: displayName,
					ParamType:     paramType,
					Severity:      severity,
					RiskLevel:     rules.RiskLevelLow,
					Message:       fmt.Sprintf("Parameter %s in %s: %s", displayName, compType, filterReason),
					Details:       details,
					CurrentValue:  currentValue,
					SourceDefault: sourceDefault,
					TargetDefault: targetDefault,
					Metadata: map[string]interface{}{
						"filtered":      true,
						"filter_reason": filterReason,
					},
				})
				continue
			}

			// Check if this is a forced change (from upgrade_logic.json)
			// This will be handled by UPGRADE_DIFFERENCES rule, but we can preprocess it here
			// For now, we'll let the rule handle forced changes, but we can add preprocessing later if needed

			// Check if this parameter has special notes (from parameter_notes.json)
			// This will be handled by the rule using GetParameterNote, but we can preprocess it here
			// For now, we'll let the rule handle special notes

			// Add to cleaned defaults (not filtered)
			cleanedSourceDefaults[compType][paramName] = sourceDefaultValue
			if targetDefaults[compType] != nil {
				if targetDefaultValue, ok := targetDefaults[compType][paramName]; ok {
					cleanedTargetDefaults[compType][paramName] = targetDefaultValue
				}
			}
		}

		// Also process parameters that exist only in target defaults (new parameters)
		if targetDefaults[compType] != nil {
			for paramName, targetDefaultValue := range targetDefaults[compType] {
				// Skip if already processed
				if _, ok := sourceDefaults[compType][paramName]; ok {
					continue
				}

				// Determine parameter type
				isSystemVar := strings.HasPrefix(paramName, "sysvar:")
				var displayName string
				var paramType string
				if isSystemVar {
					displayName = strings.TrimPrefix(paramName, "sysvar:")
					paramType = "system_variable"
				} else {
					displayName = paramName
					paramType = "config"
				}

				// Check if should be filtered
				shouldFilter, filterReason := ShouldFilterParameter(displayName)
				if !shouldFilter {
					// Also check with full paramName (for system variables with "sysvar:" prefix)
					shouldFilter, filterReason = ShouldFilterParameter(paramName)
				}

				// Check if current value equals target default (no action needed)
				if !shouldFilter && component != nil {
					var currentValue interface{}
					if isSystemVar {
						if varValue, ok := component.Variables[displayName]; ok {
							currentValue = varValue.Value
						}
					} else {
						if paramValue, ok := component.Config[displayName]; ok {
							currentValue = paramValue.Value
						}
					}

					if currentValue != nil {
						targetDefault := extractValueFromDefault(targetDefaultValue)
						if targetDefault != nil && rules.CompareValues(currentValue, targetDefault) {
							// For PD, still report new parameters even if current == target
							if compType != "pd" {
								shouldFilter = true
							}
						}
					}
				}

				// If should filter, generate CheckResult for filtered parameter and skip adding to cleaned defaults
				if shouldFilter {
					// Get current value and target default for the filtered parameter
					var currentValue interface{}
					var targetDefault interface{}

					if component != nil {
						if isSystemVar {
							if varValue, ok := component.Variables[displayName]; ok {
								currentValue = varValue.Value
							}
						} else {
							if paramValue, ok := component.Config[displayName]; ok {
								currentValue = paramValue.Value
							}
						}
					}

					targetDefault = extractValueFromDefault(targetDefaultValue)

					// Determine filter reason and severity
					severity := "info"
					if filterReason == "" {
						if currentValue != nil && targetDefault != nil && rules.CompareValues(currentValue, targetDefault) {
							filterReason = "new parameter (current value equals target default, no action needed)"
						} else {
							filterReason = "new parameter filtered"
						}
					}

					// Build details message
					details := fmt.Sprintf("This new parameter has been filtered from detailed analysis.\nReason: %s", filterReason)
					if currentValue != nil {
						details += fmt.Sprintf("\n\nCurrent Value: %v", rules.FormatValue(currentValue))
					}
					if targetDefault != nil {
						details += fmt.Sprintf("\nTarget Default: %v", rules.FormatValue(targetDefault))
					}

					// Add note about why it's filtered
					if strings.Contains(filterReason, "deployment-specific") || strings.Contains(filterReason, "path parameter") {
						details += "\n\nNote: This parameter varies by deployment environment and does not require user action during upgrade."
					} else if strings.Contains(filterReason, "current value equals target default") {
						details += "\n\nNote: Current value already matches target default. No action needed after upgrade."
					}

					preprocessedResults = append(preprocessedResults, rules.CheckResult{
						RuleID:        "PARAMETER_PREPROCESSOR",
						Category:      "filtered",
						Component:     compType,
						ParameterName: displayName,
						ParamType:     paramType,
						Severity:      severity,
						RiskLevel:     rules.RiskLevelLow,
						Message:       fmt.Sprintf("New parameter %s in %s: %s", displayName, compType, filterReason),
						Details:       details,
						CurrentValue:  currentValue,
						TargetDefault: targetDefault,
						Metadata: map[string]interface{}{
							"filtered":      true,
							"filter_reason": filterReason,
							"is_new_param":  true,
						},
					})
					continue
				}

				// Add to cleaned defaults
				cleanedTargetDefaults[compType][paramName] = targetDefaultValue
			}
		}
	}

	return preprocessedResults, cleanedSourceDefaults, cleanedTargetDefaults
}
