// Package rules provides standardized rule definitions for upgrade precheck
package rules

import (
	"context"
	"fmt"
	"strings"
)

// ignoredParamsForUpgradeDifferences contains parameters that should be ignored
// when reporting default value changes. These are typically deployment-specific
// parameters (paths, hostnames, etc.) that differ between environments but don't
// represent actual configuration changes that users need to be aware of.
var ignoredParamsForUpgradeDifferences = map[string]bool{
	// Deployment-specific path parameters (TiDB)
	"host":                 true, // Host binding address (deployment-specific)
	"path":                 true, // TiDB storage path (deployment-specific)
	"socket":               true, // Socket file path (deployment-specific)
	"temp-dir":             true, // Temporary directory (deployment-specific)
	"tmp-storage-path":     true, // Temporary storage path (deployment-specific)
	"log.file.filename":    true, // Log file path (deployment-specific)
	"log.slow-query-file":  true, // Slow query log file path (deployment-specific)
	"log.file.max-size":    true, // Log file max size (deployment-specific, may vary)
	"log.file.max-days":    true, // Log file max days (deployment-specific, may vary)
	"log.file.max-backups": true, // Log file max backups (deployment-specific, may vary)

	// Deployment-specific path parameters (TiKV)
	"data-dir":   true, // Data directory (deployment-specific)
	"log-file":   true, // Log file path (deployment-specific)
	"deploy-dir": true, // Deploy directory (deployment-specific)
	"log-dir":    true, // Log directory (deployment-specific)

	// Deployment-specific path parameters (PD)
	// data-dir, log-file, deploy-dir, log-dir are already covered above

	// Deployment-specific path parameters (TiFlash)
	"tmp_path":           true, // Temporary path (deployment-specific)
	"storage.main.dir":   true, // Storage main directory (deployment-specific)
	"storage.latest.dir": true, // Storage latest directory (deployment-specific)
	"storage.raft.dir":   true, // Storage raft directory (deployment-specific)

	// Other parameters to ignore
	"deprecate-integer-display-length": true, // Deprecated parameter, no need to report
}

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
		SourceKBRequirements: struct {
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
// 2. Source version has, target version doesn't: info (deprecated)
// 3. Source version doesn't have, target version has: info (new parameter)
// 4. Target default != source default: info (default value changed)
// 5. If source default == target default: skip (no difference, don't show)
// 6. Otherwise (consistent): skip (don't show)
func (r *UpgradeDifferencesRule) Evaluate(ctx context.Context, ruleCtx *RuleContext) ([]CheckResult, error) {
	var results []CheckResult
	// Track statistics: total parameters compared and skipped
	totalCompared := 0
	totalSkipped := 0

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
		sourceDefaults := ruleCtx.SourceDefaults[compType]

		// Track which parameters we've processed
		processedParams := make(map[string]bool)

		// 1. Check parameters that exist in target version (compare with current cluster)
		for paramName, targetDefaultValue := range targetDefaults {
			processedParams[paramName] = true
			totalCompared++

			// Extract actual value from ParameterValue structure
			targetDefault := extractValueFromDefault(targetDefaultValue)
			sourceDefault := ruleCtx.GetSourceDefault(compType, paramName)

			// Filter: If source default == target default, skip (no difference)
			if sourceDefault != nil && targetDefault != nil {
				sourceDefaultStr := fmt.Sprintf("%v", sourceDefault)
				targetDefaultStr := fmt.Sprintf("%v", targetDefault)
				if sourceDefaultStr == targetDefaultStr {
					// Source and target defaults are the same, skip unless it's a forced change or current differs
					// We'll check forced changes and current value differences below, but if all are same, skip
				}
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
				} else {
					// System variable not in current cluster, skip
					continue
				}
			} else {
				displayName = paramName
				paramType = "config"
				if paramValue, ok := component.Config[paramName]; ok {
					currentValue = paramValue.Value
				} else {
					// Config parameter not in current cluster, skip
					continue
				}
			}

			// Skip ignored parameters (deployment-specific paths, etc.)
			if ignoredParamsForUpgradeDifferences[displayName] || ignoredParamsForUpgradeDifferences[paramName] {
				totalSkipped++
				continue
			}

			// Compare target default with current cluster value
			targetDiffersFromCurrent := fmt.Sprintf("%v", targetDefault) != fmt.Sprintf("%v", currentValue)
			sourceDefaultStr := fmt.Sprintf("%v", sourceDefault)
			targetDefaultStr := fmt.Sprintf("%v", targetDefault)

			// Filter: If source default == target default and current == target, skip (no difference)
			// Exception: forced changes should always be reported
			if sourceDefault != nil && targetDefault != nil && sourceDefaultStr == targetDefaultStr {
				currentValueStr := fmt.Sprintf("%v", currentValue)
				if currentValueStr == targetDefaultStr {
					// All values are the same: source == target == current
					// Check if it's a forced change - if not, skip
					if _, isForced := forcedChanges[displayName]; !isForced {
						totalSkipped++
						continue // Skip: no difference between source and target
					}
				}
			}

			// Check if this parameter is in upgrade_logic.json (forced change)
			if forcedValue, isForced := forcedChanges[displayName]; isForced {
				// This parameter is in upgrade_logic.json
				forcedValueStr := fmt.Sprintf("%v", forcedValue)
				currentValueStr := fmt.Sprintf("%v", currentValue)

				if forcedValueStr != currentValueStr {
					// Forced value differs from current value: error severity for TiDB (critical), warning for others
					severity := "warning"
					riskLevel := RiskLevelMedium
					if compType == "tidb" {
						severity = "error"
						riskLevel = RiskLevelHigh
					}
					details := FormatDefaultChangeDiff(currentValue, sourceDefault, targetDefault, nil)
					// Add forced value information
					forcedStr := FormatValue(forcedValue)
					if !strings.Contains(details, "Will be forced to") {
						details = fmt.Sprintf("Will be forced to: %s\n\n%s", forcedStr, details)
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
						SourceDefault: sourceDefault,
						ForcedValue:   forcedValue,
						Suggestions: []string{
							"This parameter will be forcibly changed during upgrade",
							"Review the forced change and its impact",
							"Test the new value in a staging environment",
							"Plan for the change before upgrading",
						},
					})
				} else {
					// Forced value equals current value: info (default value changed)
					details := FormatDefaultChangeDiff(currentValue, sourceDefault, targetDefault, nil)
					if !strings.Contains(details, "matches forced value") {
						details = "Current value matches forced value.\n\n" + details
					}
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
						SourceDefault: sourceDefault,
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
				if IsMapType(sourceDefault) && IsMapType(targetDefault) {
					opts := CompareOptions{
						IgnoredParams: ignoredParamsForUpgradeDifferences, // Use ignore list for nested fields
						BasePath:      displayName,
					}
					sourceTargetDiffs := CompareMapsDeep(sourceDefault, targetDefault, opts)

					// Convert currentValue to map for field extraction
					currentMap := ConvertToMapStringInterface(currentValue)

					for fieldPath, diff := range sourceTargetDiffs {
						// Extract current value for this specific field from the map
						var currentFieldValue interface{}
						if currentMap != nil {
							fieldKeys := strings.Split(fieldPath, ".")
							currentFieldValue = getNestedMapValue(currentMap, fieldKeys)
						} else {
							currentFieldValue = currentValue // Fallback to full value if not a map
						}

						fieldMessage := fmt.Sprintf("Parameter %s.%s in %s: %s", displayName, fieldPath, compType, baseMessage)
						// Format details with current field value
						fieldDetails := FormatValueDiff(currentFieldValue, diff.Source) + " → " + FormatValue(diff.Current)

						// Check if user has modified this parameter (current != source)
						currentValueStr := fmt.Sprintf("%v", currentFieldValue)
						sourceValueStr := fmt.Sprintf("%v", diff.Source)
						targetValueStr := fmt.Sprintf("%v", diff.Current)
						
						// If current value differs from both source and target, indicate user modification
						if currentValueStr != sourceValueStr && currentValueStr != targetValueStr {
							fieldDetails += fmt.Sprintf("\n\n⚠️ User Modified: Current value (%v) differs from both source default (%v) and target default (%v)", 
								FormatValue(currentFieldValue), FormatValue(diff.Source), FormatValue(diff.Current))
						} else if currentValueStr != sourceValueStr {
							fieldDetails += fmt.Sprintf("\n\n⚠️ User Modified: Current value (%v) differs from source default (%v)", 
								FormatValue(currentFieldValue), FormatValue(diff.Source))
						}

						// Add component-specific note
						if compType == "pd" && paramType == "config" {
							fieldDetails += "\n\nPD maintains existing configuration"
						} else if compType == "tidb" && paramType == "system_variable" {
							fieldDetails += "\n\nTiDB system variables keep old values"
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
							CurrentValue:  currentFieldValue, // Extract field value from current map
							TargetDefault: diff.Current,      // Target value for this field
							SourceDefault: diff.Source,       // Source value for this field
							Suggestions: []string{
								"Default value has changed in target version",
								"Review if the new default is acceptable",
							},
						})
					}
				} else {
					// For non-map types, use simple format
					details := FormatDefaultChangeDiff(currentValue, sourceDefault, targetDefault, nil)
					if compType == "pd" && paramType == "config" {
						details += "\n\nPD maintains existing configuration"
					} else if compType == "tidb" && paramType == "system_variable" {
						details += "\n\nTiDB system variables keep old values"
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
						SourceDefault: sourceDefault,
						Suggestions: []string{
							"Default value has changed in target version",
							"Review if the new default is acceptable",
						},
					})
				}
			} else if sourceDefault != nil && targetDefault != nil {
				// Target default == current value, but source default != target default: info (default value changed)
				// Note: We already checked sourceDefaultStr == targetDefaultStr above and skipped, so this case is source != target
				sourceDefaultStr := fmt.Sprintf("%v", sourceDefault)
				targetDefaultStr := fmt.Sprintf("%v", targetDefault)
				if sourceDefaultStr != targetDefaultStr {
					// For map types, create separate CheckResult for each differing field
					if IsMapType(sourceDefault) && IsMapType(targetDefault) {
						opts := CompareOptions{
							IgnoredParams: ignoredParamsForUpgradeDifferences, // Use ignore list for nested fields
							BasePath:      displayName,
						}
						sourceTargetDiffs := CompareMapsDeep(sourceDefault, targetDefault, opts)

						// Convert currentValue to map for field extraction
						currentMap := ConvertToMapStringInterface(currentValue)

						for fieldPath, diff := range sourceTargetDiffs {
							// Extract current value for this specific field from the map
							var currentFieldValue interface{}
							if currentMap != nil {
								fieldKeys := strings.Split(fieldPath, ".")
								currentFieldValue = getNestedMapValue(currentMap, fieldKeys)
							} else {
								currentFieldValue = currentValue // Fallback to full value if not a map
							}

							fieldDetails := FormatValueDiff(diff.Source, diff.Current) // Source -> Target
							fieldDetails = fmt.Sprintf("Current: %s (matches target default)\n\n", FormatValue(currentFieldValue)) + fieldDetails

							results = append(results, CheckResult{
								RuleID:        r.Name(),
								Category:      r.Category(),
								Component:     compType,
								ParameterName: fmt.Sprintf("%s.%s", displayName, fieldPath),
								ParamType:     paramType,
								Severity:      "info",
								RiskLevel:     RiskLevelLow,
								Message:       fmt.Sprintf("Parameter %s.%s in %s: default value changed between source and target versions", displayName, fieldPath, compType),
								Details:       fieldDetails,
								CurrentValue:  currentFieldValue, // Extract field value from current map
								TargetDefault: diff.Current,
								SourceDefault: diff.Source,
								Suggestions: []string{
									"Default value has changed between source and target versions",
									"Your current value matches the new target default",
								},
							})
						}
					} else {
						// For non-map types
						details := FormatDefaultChangeDiff(currentValue, sourceDefault, targetDefault, nil)
						if !strings.Contains(details, "matches target default") {
							details = "Current value matches target default.\n\n" + details
						}
						results = append(results, CheckResult{
							RuleID:        r.Name(),
							Category:      r.Category(),
							Component:     compType,
							ParameterName: displayName,
							ParamType:     paramType,
							Severity:      "info",
							RiskLevel:     RiskLevelLow,
							Message:       fmt.Sprintf("Parameter %s in %s: default value changed between source and target versions", displayName, compType),
							Details:       details,
							CurrentValue:  currentValue,
							TargetDefault: targetDefault,
							SourceDefault: sourceDefault,
							Suggestions: []string{
								"Default value has changed between source and target versions",
								"Your current value matches the new target default",
							},
						})
					}
				}
				// Otherwise: target default == current value == source default, skip (consistent)
			}
		}

		// 2. Check parameters that exist in source version but not in target version (deprecated)
		for paramName, sourceDefaultValue := range sourceDefaults {
			if processedParams[paramName] {
				continue // Already processed
			}

			// Source has, target doesn't: deprecated
			sourceDefault := extractValueFromDefault(sourceDefaultValue)

			// Get current cluster value
			var currentValue interface{}
			var paramType string
			var displayName string

			isSystemVar := strings.HasPrefix(paramName, "sysvar:")
			if isSystemVar {
				varName := strings.TrimPrefix(paramName, "sysvar:")
				displayName = varName
				paramType = "system_variable"
				if varValue, ok := component.Variables[varName]; ok {
					currentValue = varValue.Value
				} else {
					continue // Not in current cluster
				}
			} else {
				displayName = paramName
				paramType = "config"
				if paramValue, ok := component.Config[paramName]; ok {
					currentValue = paramValue.Value
				} else {
					continue // Not in current cluster
				}
			}

			// Skip ignored parameters (deployment-specific paths, etc.)
			if ignoredParamsForUpgradeDifferences[displayName] || ignoredParamsForUpgradeDifferences[paramName] {
				continue
			}

			// Parameter deprecated: low risk (info)
			results = append(results, CheckResult{
				RuleID:        r.Name(),
				Category:      r.Category(),
				Component:     compType,
				ParameterName: displayName,
				ParamType:     paramType,
				Severity:      "info",
				RiskLevel:     RiskLevelLow,
				Message:       fmt.Sprintf("Parameter %s in %s is deprecated (exists in source version but removed in target version)", displayName, compType),
				Details:       fmt.Sprintf("Current cluster value: %v, Source default: %v. This parameter will be removed in target version.", currentValue, sourceDefault),
				CurrentValue:  currentValue,
				SourceDefault: sourceDefault,
				Suggestions: []string{
					"This parameter is deprecated and will be removed in target version",
					"Review if this parameter is still needed",
					"Plan for migration if necessary",
				},
			})
		}

		// 3. Check parameters that exist in target version but not in source version (new parameter)
		for paramName, targetDefaultValue := range targetDefaults {
			// Skip if already processed in step 1 (exists in current cluster)
			if processedParams[paramName] {
				continue
			}

			// Check if this parameter exists in source version
			sourceDefault := ruleCtx.GetSourceDefault(compType, paramName)
			if sourceDefault != nil {
				continue // Exists in source version, already processed or skipped
			}

			// Source doesn't have, target has: new parameter
			targetDefault := extractValueFromDefault(targetDefaultValue)

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
			Description:   fmt.Sprintf("Compared %d parameters, skipped %d (source == target)", totalCompared, totalSkipped),
			Severity:      "info",
			RiskLevel:     RiskLevelLow,
		})
	}

	return results, nil
}
