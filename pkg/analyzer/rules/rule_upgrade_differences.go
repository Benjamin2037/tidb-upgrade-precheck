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
// Logic:
// 1. Compare target version defaults with current cluster values
//   - If in upgrade_logic.json (forced change):
//   - If forced value != current value: warning (medium risk)
//   - If forced value == current value: info (default value changed)
//   - If not in upgrade_logic.json:
//   - If target default != current value: info (default value changed)
//
// 2. If parameter exists in target version but not in current cluster: info (new parameter)
// Note: Source version comparison is handled by USER_MODIFIED_PARAMS rule, not here
func (r *UpgradeDifferencesRule) Evaluate(ctx context.Context, ruleCtx *RuleContext) ([]CheckResult, error) {
	var results []CheckResult
	// Track statistics: total parameters compared
	totalCompared := 0
	totalFiltered := 0

	if ruleCtx.SourceClusterSnapshot == nil {
		return results, nil
	}

	// Get forced changes for each component
	forcedChangesByComponent := make(map[string]map[string]interface{})
	for _, comp := range []string{"tidb", "pd", "tikv", "tiflash"} {
		forcedChanges := ruleCtx.GetForcedChanges(comp)
		forcedChangesByComponent[comp] = forcedChanges
	}

	// Process each component
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

		// Get all parameters from target version knowledge base
		targetDefaults := ruleCtx.TargetDefaults[compType]
		if targetDefaults == nil {
			return nil, fmt.Errorf("targetDefaults for component %s is nil - this indicates a knowledge base loading issue. Please check if the target version knowledge base was loaded correctly. Component: %s, SourceVersion: %s, TargetVersion: %s", compType, compType, ruleCtx.SourceVersion, ruleCtx.TargetVersion)
		}

		// Track which parameters we've processed
		processedParams := make(map[string]bool)

		// 1. Check parameters that exist in target version (compare with current cluster)
		for paramName, targetDefaultValue := range targetDefaults {
			processedParams[paramName] = true
			totalCompared++

			// Extract actual value from ParameterValue structure
			targetDefault := extractValueFromDefault(targetDefaultValue)

			// If targetDefault is nil/None, this indicates a knowledge base loading issue
			// targetDefault must have a value - forced changes are upgrade compatibility matters and don't affect this requirement
			if targetDefault == nil {
				return nil, fmt.Errorf("targetDefault for parameter %s in component %s is nil - this indicates a knowledge base loading issue. Component: %s, Parameter: %s, SourceVersion: %s, TargetVersion: %s. Please check if the target version knowledge base was loaded correctly", paramName, compType, compType, paramName, ruleCtx.SourceVersion, ruleCtx.TargetVersion)
			}

			// Get current cluster value
			var currentValue interface{}
			var paramType string
			var displayName string

			// Determine if this is a system variable (prefixed with "sysvar:")
			isSystemVar := strings.HasPrefix(paramName, "sysvar:")
			if isSystemVar {
				varName := strings.TrimPrefix(paramName, "sysvar:")
				displayName = varName
				paramType = "system_variable"
				if varValue, ok := component.Variables[varName]; ok {
					currentValue = varValue.Value
					if currentValue == nil {
						return nil, fmt.Errorf("system variable %s in component %s has nil value - this indicates a data collection issue. Component: %s, Parameter: %s, SourceVersion: %s, TargetVersion: %s", varName, compType, compType, varName, ruleCtx.SourceVersion, ruleCtx.TargetVersion)
					}
				} else {
					// System variable not in current cluster - this is a new parameter, will be handled in Step 2
					// Skip here to avoid duplicate reporting
					continue
				}
			} else {
				displayName = paramName
				paramType = "config"
				if paramValue, ok := component.Config[paramName]; ok {
					currentValue = paramValue.Value
					if currentValue == nil {
						return nil, fmt.Errorf("config parameter %s in component %s has nil value - this indicates a data collection issue. Component: %s, Parameter: %s, SourceVersion: %s, TargetVersion: %s", paramName, compType, compType, paramName, ruleCtx.SourceVersion, ruleCtx.TargetVersion)
					}
				} else {
					// Config parameter not in current cluster - this is a new parameter, will be handled in Step 2
					// Skip here to avoid duplicate reporting
					continue
				}
			}

			// Compare target default with current cluster value
			// Use proper value comparison to avoid scientific notation issues
			targetDiffersFromCurrent := !CompareValues(targetDefault, currentValue)

			// Check if this parameter is in upgrade_logic.json (forced change)
			// First check if there's a forced change entry for this parameter
			fallbackForcedValue, hasForcedChange := forcedChanges[displayName]

			// Get the forced value that matches the current value (using from_value matching)
			forcedValue := ruleCtx.GetForcedChangeForValue(compType, displayName, currentValue)

			// If no matching from_value found, use fallback value (for entries without from_value)
			if forcedValue == nil && hasForcedChange {
				forcedValue = fallbackForcedValue
			}

			if hasForcedChange && forcedValue != nil {
				// This parameter is in upgrade_logic.json and we found a matching entry
				// Use proper value comparison to avoid scientific notation issues
				if !CompareValues(forcedValue, currentValue) {
					// Get special handling metadata from knowledge base
					metadata := ruleCtx.GetForcedChangeMetadata(compType, displayName, currentValue)

					// Determine severity: use metadata override if available, otherwise use default logic
					severity := "warning"
					riskLevel := RiskLevelMedium
					if metadata != nil && metadata.ReportSeverity != "" {
						// Use severity from knowledge base
						severity = metadata.ReportSeverity
						switch severity {
						case "error":
							riskLevel = RiskLevelHigh
						case "warning":
							riskLevel = RiskLevelMedium
						case "info":
							riskLevel = RiskLevelLow
						default:
							riskLevel = RiskLevelMedium
						}
					} else if compType == "tidb" {
						// Default: Most TiDB forced changes are error
						severity = "error"
						riskLevel = RiskLevelHigh
					}

					// Build details for forced change
					forcedStr := FormatValue(forcedValue)
					currentStr := FormatValue(currentValue)
					targetStr := FormatValue(targetDefault)
					details := fmt.Sprintf("Will be forced to: %s\n\nCurrent: %s\nTarget Default: %s", forcedStr, currentStr, targetStr)

					// Add details note from knowledge base if available
					if metadata != nil && metadata.DetailsNote != "" {
						details += "\n\n" + metadata.DetailsNote
					}

					// Get suggestions: use metadata if available, otherwise use default
					var suggestions []string
					if metadata != nil && len(metadata.Suggestions) > 0 {
						suggestions = metadata.Suggestions
					} else {
						// Default suggestions for forced changes
						suggestions = []string{
							"This parameter will be forcibly changed during upgrade",
							"Review the forced change and its impact",
							"Test the new value in a staging environment",
							"Plan for the change before upgrading",
						}
					}

					results = append(results, CheckResult{
						RuleID:        r.Name(),
						Category:      r.Category(),
						Component:     compType,
						ParameterName: displayName,
						ParamType:     paramType,
						Severity:      severity,
						RiskLevel:     riskLevel,
						Message:       fmt.Sprintf("Parameter %s in %s will be forcibly changed during upgrade (forced value differs from current)", displayName, compType),
						Details:       details,
						CurrentValue:  currentValue,
						TargetDefault: targetDefault,
						ForcedValue:   forcedValue,
						Suggestions:   suggestions,
					})
				} else {
					// Forced value equals current value: info (default value changed)
					currentStr := FormatValue(currentValue)
					targetStr := FormatValue(targetDefault)
					details := fmt.Sprintf("Current value matches forced value.\n\nCurrent: %s\nTarget Default: %s", currentStr, targetStr)
					results = append(results, CheckResult{
						RuleID:        r.Name(),
						Category:      r.Category(),
						Component:     compType,
						ParameterName: displayName,
						ParamType:     paramType,
						Severity:      "info",
						RiskLevel:     RiskLevelLow,
						Message:       fmt.Sprintf("Parameter %s in %s: default value changed (forced change matches current value)", displayName, compType),
						Details:       details,
						CurrentValue:  currentValue,
						TargetDefault: targetDefault,
						ForcedValue:   forcedValue,
						Suggestions: []string{
							"Default value has changed in target version",
							"Your current value matches the forced value, so no change will occur",
						},
					})
				}
			} else if targetDiffersFromCurrent {
				// Not in upgrade_logic.json, but target default differs from current
				// Special handling for PD and system variables
				severity := "warning"
				riskLevel := RiskLevelMedium
				baseMessage := "default value changed (target default differs from current)"

				// PD maintains existing configuration
				if compType == "pd" && paramType == "config" {
					severity = "info"
					riskLevel = RiskLevelLow
					baseMessage = "default value changed (current value will be kept)"
				}

				// TiDB system variables keep old values unless forced
				if compType == "tidb" && paramType == "system_variable" {
					severity = "info"
					riskLevel = RiskLevelLow
					baseMessage = "default value changed (current value will be kept)"
				}

				// For map types, create separate CheckResult for each differing field
				if IsMapType(currentValue) && IsMapType(targetDefault) {
					opts := CompareOptions{
						BasePath: displayName,
					}
					currentTargetDiffs := CompareMapsDeep(currentValue, targetDefault, opts)

					// Convert currentValue to map for field extraction
					currentMap := ConvertToMapStringInterface(currentValue)
					targetMap := ConvertToMapStringInterface(targetDefault)

					for fieldPath := range currentTargetDiffs {
						// Extract current value for this specific field from the map
						var currentFieldValue interface{}
						if currentMap != nil {
							fieldKeys := strings.Split(fieldPath, ".")
							currentFieldValue = getNestedMapValue(currentMap, fieldKeys)
						} else {
							currentFieldValue = currentValue // Fallback to full value if not a map
						}

						// Extract target default value for this field
						var targetFieldValue interface{}
						if targetMap != nil {
							fieldKeys := strings.Split(fieldPath, ".")
							targetFieldValue = getNestedMapValue(targetMap, fieldKeys)
						} else {
							targetFieldValue = targetDefault
						}

						fieldMessage := fmt.Sprintf("Parameter %s.%s in %s: %s", displayName, fieldPath, compType, baseMessage)
						// Format details: Current vs Target
						fieldDetails := fmt.Sprintf("Current: %s\nTarget Default: %s", FormatValue(currentFieldValue), FormatValue(targetFieldValue))

						// Add component-specific note
						if compType == "pd" && paramType == "config" {
							fieldDetails += "\n\nCurrent value will be kept.\n\nPD maintains existing configuration"
						} else if compType == "tidb" && paramType == "system_variable" {
							fieldDetails += "\n\nCurrent value will be kept.\n\nTiDB system variables keep old values"
						}

						// Get special note from knowledge base for this field (if it's a top-level parameter, use displayName; if nested, use fieldPath)
						// For map types, check if fieldPath matches a parameter note
						paramNote := ruleCtx.GetParameterNote(compType, fieldPath, paramType, targetFieldValue)
						if paramNote == "" {
							// Try with full parameter path (displayName.fieldPath)
							fullParamPath := fmt.Sprintf("%s.%s", displayName, fieldPath)
							paramNote = ruleCtx.GetParameterNote(compType, fullParamPath, paramType, targetFieldValue)
						}
						if paramNote != "" {
							fieldDetails += "\n\n" + paramNote
						}

						results = append(results, CheckResult{
							RuleID:        r.Name(),
							Category:      r.Category(),
							Component:     compType,
							ParameterName: fmt.Sprintf("%s.%s", displayName, fieldPath),
							ParamType:     paramType,
							Severity:      severity,
							RiskLevel:     riskLevel,
							Message:       fieldMessage,
							Details:       fieldDetails,
							CurrentValue:  currentFieldValue,
							TargetDefault: targetFieldValue,
							Suggestions: []string{
								"Default value has changed in target version",
								"Review if the new default is acceptable",
							},
						})
					}
				} else {
					// For non-map types, use simple format
					currentStr := FormatValue(currentValue)
					targetStr := FormatValue(targetDefault)
					details := fmt.Sprintf("Current: %s\nTarget Default: %s", currentStr, targetStr)
					if compType == "pd" && paramType == "config" {
						details += "\n\nCurrent value will be kept.\n\nPD maintains existing configuration"
					} else if compType == "tidb" && paramType == "system_variable" {
						details += "\n\nCurrent value will be kept.\n\nTiDB system variables keep old values"
					}

					// Get special note from knowledge base
					paramNote := ruleCtx.GetParameterNote(compType, displayName, paramType, targetDefault)
					if paramNote != "" {
						details += "\n\n" + paramNote
					}

					results = append(results, CheckResult{
						RuleID:        r.Name(),
						Category:      r.Category(),
						Component:     compType,
						ParameterName: displayName,
						ParamType:     paramType,
						Severity:      severity,
						RiskLevel:     riskLevel,
						Message:       fmt.Sprintf("Parameter %s in %s: %s", displayName, compType, baseMessage),
						Details:       details,
						CurrentValue:  currentValue,
						TargetDefault: targetDefault,
						Suggestions: []string{
							"Default value has changed in target version",
							"Review if the new default is acceptable",
						},
					})
				}
			}
			// Otherwise: target default == current value, skip (no difference)
		}

		// 2. Check parameters that exist in target version but not in current cluster (new parameter)
		// Note: Deprecated parameters (exist in source but not in target) are not checked here
		// as source version comparison is handled by USER_MODIFIED_PARAMS rule
		for paramName, targetDefaultValue := range targetDefaults {
			// Skip if already processed in step 1 (exists in current cluster)
			if processedParams[paramName] {
				continue
			}

			// Extract target default value
			targetDefault := extractValueFromDefault(targetDefaultValue)
			if targetDefault == nil {
				return nil, fmt.Errorf("targetDefault for parameter %s in component %s is nil - this indicates a knowledge base loading issue. Component: %s, Parameter: %s, SourceVersion: %s, TargetVersion: %s. Please check if the target version knowledge base was loaded correctly", paramName, compType, compType, paramName, ruleCtx.SourceVersion, ruleCtx.TargetVersion)
			}

			var paramType string
			var displayName string

			isSystemVar := strings.HasPrefix(paramName, "sysvar:")
			if isSystemVar {
				displayName = strings.TrimPrefix(paramName, "sysvar:")
				paramType = "system_variable"
			} else {
				displayName = paramName
				paramType = "config"
			}

			// Note: Deployment-specific parameters have already been filtered in preprocessor
			// This rule only processes parameters that passed the preprocessor filter

			// Check if this new parameter exists in current cluster
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

			// Filter: If current value equals target default, skip (no action needed after upgrade)
			if currentValue != nil && targetDefault != nil {
				if CompareValues(currentValue, targetDefault) {
					// For PD component, still report new parameters even if current == target
					if compType == "pd" && paramType == "config" {
						// Don't filter PD new parameters, let them be reported
					} else {
						// Current value equals target default, no action needed
						totalFiltered++
						continue
					}
				}
			}

			// New parameter: info
			message := fmt.Sprintf("Parameter %s in %s is new (added in target version)", displayName, compType)
			details := fmt.Sprintf("Target default: %v. This is a new parameter in target version.", targetDefault)
			if currentValue != nil {
				message = fmt.Sprintf("Parameter %s in %s is new (added in target version, already configured in cluster)", displayName, compType)
				details = fmt.Sprintf("Current cluster value: %v, Target default: %v. This is a new parameter in target version.", currentValue, targetDefault)
			}

			results = append(results, CheckResult{
				RuleID:        r.Name(),
				Category:      r.Category(),
				Component:     compType,
				ParameterName: displayName,
				ParamType:     paramType,
				Severity:      "info",
				RiskLevel:     RiskLevelLow,
				Message:       message,
				Details:       details,
				CurrentValue:  currentValue,
				TargetDefault: targetDefault,
				Suggestions: []string{
					"This is a new parameter in target version",
					"Review the new parameter and its default value",
					"Consider configuring it if needed",
				},
			})
		}
	}

	// Add a special CheckResult to pass statistics (if we compared any parameters)
	// This will be filtered out in the reporter, but analyzer can extract it
	if totalCompared > 0 {
		results = append(results, CheckResult{
			RuleID:        r.Name() + "_STATS",
			Category:      r.Category(),
			Component:     "",
			ParameterName: "__statistics__",
			Description:   fmt.Sprintf("Compared %d parameters, filtered %d (deployment-specific)", totalCompared, totalFiltered),
			Severity:      "info",
			RiskLevel:     RiskLevelLow,
		})
	}

	return results, nil
}
