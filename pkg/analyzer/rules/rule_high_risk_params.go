// Package rules provides standardized rule definitions for upgrade precheck
package rules

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/pingcap/tidb-upgrade-precheck/pkg/collector"
)

// HighRiskParamConfig defines configuration for a high-risk parameter
type HighRiskParamConfig struct {
	// Severity is the severity level when this parameter is found/modified
	// Values: "error", "warning", "info"
	Severity string `json:"severity"`
	// Description is a human-readable description of why this parameter is high-risk
	Description string `json:"description,omitempty"`
	// AllowedValues is an optional list of allowed values
	// If empty, any modification from default is considered risky
	// If specified, only values in this list are allowed
	AllowedValues []interface{} `json:"allowed_values,omitempty"`
	// CheckModified indicates whether to check if the parameter has been modified from default
	// If true, only report if the parameter value differs from source default
	// If false, always report if the parameter exists
	CheckModified bool `json:"check_modified,omitempty"`
	// FromVersion is the minimum version from which this parameter is considered high-risk
	// Format: "v6.5.0", "v7.5.0", etc.
	// If empty, applies to all versions
	// The rule will only check this parameter if sourceVersion >= FromVersion
	FromVersion string `json:"from_version,omitempty"`
	// ToVersion is the maximum version until which this parameter is considered high-risk
	// Format: "v7.5.0", "v8.5.0", etc.
	// If empty, applies to all versions after FromVersion
	// The rule will only check this parameter if sourceVersion <= ToVersion (if specified)
	ToVersion string `json:"to_version,omitempty"`
}

// HighRiskParamsConfig defines the structure for high-risk parameters configuration
// This allows developers to manually specify high-risk parameters for each component
type HighRiskParamsConfig struct {
	// TiDB high-risk parameters
	TiDB struct {
		Config          map[string]HighRiskParamConfig `json:"config,omitempty"`
		SystemVariables map[string]HighRiskParamConfig `json:"system_variables,omitempty"`
	} `json:"tidb,omitempty"`
	// PD high-risk parameters
	PD struct {
		Config map[string]HighRiskParamConfig `json:"config,omitempty"`
	} `json:"pd,omitempty"`
	// TiKV high-risk parameters
	TiKV struct {
		Config map[string]HighRiskParamConfig `json:"config,omitempty"`
	} `json:"tikv,omitempty"`
	// TiFlash high-risk parameters
	TiFlash struct {
		Config map[string]HighRiskParamConfig `json:"config,omitempty"`
	} `json:"tiflash,omitempty"`
}

// HighRiskParamsRule checks for high-risk parameters that have been manually specified
// This rule allows developers to define custom high-risk parameters for each component
type HighRiskParamsRule struct {
	*BaseRule
	config *HighRiskParamsConfig
}

// NewHighRiskParamsRule creates a new high-risk parameters rule
// If configPath is provided, it will load the configuration from the file
// If configPath is empty, it will use an empty configuration (no high-risk params)
func NewHighRiskParamsRule(configPath string) (Rule, error) {
	rule := &HighRiskParamsRule{
		BaseRule: NewBaseRule(
			"HIGH_RISK_PARAMS",
			"Check for manually specified high-risk parameters across all components",
			"high_risk",
		),
		config: &HighRiskParamsConfig{},
	}

	// Load configuration from file if provided
	if configPath != "" {
		if err := rule.loadConfig(configPath); err != nil {
			return nil, fmt.Errorf("failed to load high-risk params config: %w", err)
		}
	}

	return rule, nil
}

// loadConfig loads high-risk parameters configuration from a JSON file
func (r *HighRiskParamsRule) loadConfig(configPath string) error {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	if err := json.Unmarshal(data, r.config); err != nil {
		return fmt.Errorf("failed to parse config file: %w", err)
	}

	return nil
}

// DataRequirements returns the data requirements for this rule
func (r *HighRiskParamsRule) DataRequirements() DataSourceRequirement {
	// Determine which components are needed based on config
	components := []string{}
	if len(r.config.TiDB.Config) > 0 || len(r.config.TiDB.SystemVariables) > 0 {
		components = append(components, "tidb")
	}
	if len(r.config.PD.Config) > 0 {
		components = append(components, "pd")
	}
	if len(r.config.TiKV.Config) > 0 {
		components = append(components, "tikv")
	}
	if len(r.config.TiFlash.Config) > 0 {
		components = append(components, "tiflash")
	}

	needSystemVars := len(r.config.TiDB.SystemVariables) > 0

	return DataSourceRequirement{
		SourceClusterRequirements: struct {
			Components          []string `json:"components"`
			NeedConfig          bool     `json:"need_config"`
			NeedSystemVariables bool     `json:"need_system_variables"`
			NeedAllTikvNodes    bool     `json:"need_all_tikv_nodes"`
		}{
			Components:          components,
			NeedConfig:          true,
			NeedSystemVariables: needSystemVars,
			NeedAllTikvNodes:    false, // Only need one instance per component
		},
		SourceKBRequirements: struct {
			Components          []string `json:"components"`
			NeedConfigDefaults  bool     `json:"need_config_defaults"`
			NeedSystemVariables bool     `json:"need_system_variables"`
			NeedUpgradeLogic    bool     `json:"need_upgrade_logic"`
		}{
			Components:          components,
			NeedConfigDefaults:  true, // Need defaults to check if modified
			NeedSystemVariables: needSystemVars,
			NeedUpgradeLogic:    false,
		},
	}
}

// Evaluate performs the rule check
func (r *HighRiskParamsRule) Evaluate(ctx context.Context, ruleCtx *RuleContext) ([]CheckResult, error) {
	var results []CheckResult

	if ruleCtx.SourceClusterSnapshot == nil {
		return results, nil
	}

	// Check TiDB high-risk parameters
	if len(r.config.TiDB.Config) > 0 || len(r.config.TiDB.SystemVariables) > 0 {
		tidbResults := r.checkComponent(
			ruleCtx,
			"tidb",
			r.config.TiDB.Config,
			r.config.TiDB.SystemVariables,
		)
		results = append(results, tidbResults...)
	}

	// Check PD high-risk parameters
	if len(r.config.PD.Config) > 0 {
		pdResults := r.checkComponent(
			ruleCtx,
			"pd",
			r.config.PD.Config,
			nil,
		)
		results = append(results, pdResults...)
	}

	// Check TiKV high-risk parameters
	if len(r.config.TiKV.Config) > 0 {
		tikvResults := r.checkComponent(
			ruleCtx,
			"tikv",
			r.config.TiKV.Config,
			nil,
		)
		results = append(results, tikvResults...)
	}

	// Check TiFlash high-risk parameters
	if len(r.config.TiFlash.Config) > 0 {
		tiflashResults := r.checkComponent(
			ruleCtx,
			"tiflash",
			r.config.TiFlash.Config,
			nil,
		)
		results = append(results, tiflashResults...)
	}

	return results, nil
}

// checkComponent checks high-risk parameters for a specific component
func (r *HighRiskParamsRule) checkComponent(
	ruleCtx *RuleContext,
	compType string,
	configParams map[string]HighRiskParamConfig,
	systemVarParams map[string]HighRiskParamConfig,
) []CheckResult {
	var results []CheckResult

	// Find the component in cluster snapshot
	var component *collector.ComponentState
	var compName string

	for name, comp := range ruleCtx.SourceClusterSnapshot.Components {
		if string(comp.Type) == compType {
			component = &comp
			compName = name
			break
		}
		// Also check by name prefix for TiKV/TiFlash nodes
		if (compType == "tikv" && strings.HasPrefix(name, "tikv")) ||
			(compType == "tiflash" && strings.HasPrefix(name, "tiflash")) {
			if component == nil {
				component = &comp
				compName = name
			}
		}
	}

	if component == nil {
		// Component not found, skip
		return results
	}

	// For TiKV, only check the first instance to avoid duplicates
	if compType == "tikv" && compName != "tikv" && !strings.HasPrefix(compName, "tikv-") {
		return results
	}

	// Check config parameters
	for paramName, paramConfig := range configParams {
		// Convert ConfigDefaults to map for checkParameter
		configMap := make(map[string]interface{})
		if paramValue, ok := component.Config[paramName]; ok {
			configMap[paramName] = map[string]interface{}{
				"value": paramValue.Value,
				"type":  paramValue.Type,
			}
		}
		result := r.checkParameter(
			ruleCtx,
			component,
			compType,
			paramName,
			"config",
			paramConfig,
			configMap,
		)
		if result != nil {
			results = append(results, *result)
		}
	}

	// Check system variables (for TiDB)
	if systemVarParams != nil && compType == "tidb" {
		for varName, varConfig := range systemVarParams {
			// Convert SystemVariables to map for checkParameter
			varMap := make(map[string]interface{})
			if varValue, ok := component.Variables[varName]; ok {
				varMap[varName] = map[string]interface{}{
					"value": varValue.Value,
					"type":  varValue.Type,
				}
			}
			result := r.checkParameter(
				ruleCtx,
				component,
				compType,
				varName,
				"system_variable",
				varConfig,
				varMap,
			)
			if result != nil {
				results = append(results, *result)
			}
		}
	}

	return results
}

// checkParameter checks a single parameter against high-risk configuration
func (r *HighRiskParamsRule) checkParameter(
	ruleCtx *RuleContext,
	component *collector.ComponentState,
	compType, paramName, paramType string,
	paramConfig HighRiskParamConfig,
	paramMap map[string]interface{}, // Pre-converted map with ParameterValue structure
) *CheckResult {
	// Check version range first
	// The parameter should be checked if the upgrade path (sourceVersion -> targetVersion)
	// overlaps with the configured version range (fromVersion -> toVersion)
	if !r.isVersionApplicableForUpgrade(ruleCtx.SourceVersion, ruleCtx.TargetVersion, paramConfig.FromVersion, paramConfig.ToVersion) {
		// This parameter is not applicable for the upgrade path, skip
		return nil
	}

	// Extract value from parameter map
	pv, exists := paramMap[paramName]
	if !exists {
		// Parameter not found, skip (might be optional or removed)
		return nil
	}

	// Extract value from ParameterValue structure
	var currentValue interface{}
	if pvMap, ok := pv.(map[string]interface{}); ok {
		if v, ok := pvMap["value"]; ok {
			currentValue = v
		} else {
			return nil
		}
	} else {
		// Fallback: try to use as-is
		currentValue = pv
	}

	// If CheckModified is true, only report if parameter differs from default
	if paramConfig.CheckModified {
		// For system variables, use "sysvar:" prefix
		lookupName := paramName
		if paramType == "system_variable" {
			lookupName = "sysvar:" + paramName
		}
		sourceDefault := ruleCtx.GetSourceDefault(compType, lookupName)
		if sourceDefault == nil {
			// No default found, skip
			return nil
		}
		// Compare values using proper comparison to avoid scientific notation issues
		if CompareValues(currentValue, sourceDefault) {
			// Value matches default, skip
			return nil
		}
	}

	// Check if value is in allowed list (if specified)
	if len(paramConfig.AllowedValues) > 0 {
		valueAllowed := false
		for _, allowedValue := range paramConfig.AllowedValues {
			if CompareValues(currentValue, allowedValue) {
				valueAllowed = true
				break
			}
		}
		if valueAllowed {
			// Value is allowed, skip
			return nil
		}
	}

	// Build message and details
	message := fmt.Sprintf("High-risk parameter %s found in %s", paramName, compType)
	details := fmt.Sprintf("Current value: %v", currentValue)
	if paramConfig.Description != "" {
		details = fmt.Sprintf("%s\nReason: %s", details, paramConfig.Description)
	}
	if paramConfig.CheckModified {
		// For system variables, use "sysvar:" prefix
		lookupName := paramName
		if paramType == "system_variable" {
			lookupName = "sysvar:" + paramName
		}
		sourceDefault := ruleCtx.GetSourceDefault(compType, lookupName)
		if sourceDefault != nil {
			details = fmt.Sprintf("%s\nSource default: %v", details, sourceDefault)
		}
	}
	if len(paramConfig.AllowedValues) > 0 {
		details = fmt.Sprintf("%s\nAllowed values: %v", details, paramConfig.AllowedValues)
	}

	// Determine severity (use config severity or default to warning)
	severity := paramConfig.Severity
	if severity == "" {
		severity = "warning"
	}

	return &CheckResult{
		RuleID:        r.Name(),
		Category:      r.Category(),
		Component:     compType,
		ParameterName: paramName,
		ParamType:     paramType,
		Severity:      severity,
		Message:       message,
		Details:       details,
		CurrentValue:  currentValue,
		Suggestions: []string{
			"Review this high-risk parameter and its current value",
			"Ensure the value is appropriate for your workload",
			"Consider consulting with the development team if unsure",
		},
		Metadata: map[string]interface{}{
			"param_name":    paramName,
			"param_type":    paramType,
			"is_high_risk":  true,
			"config_source": "manual",
			"from_version":  paramConfig.FromVersion,
			"to_version":    paramConfig.ToVersion,
		},
	}
}

// isVersionApplicableForUpgrade checks if the parameter configuration is applicable for the upgrade path
// Returns true if the upgrade path (sourceVersion -> targetVersion) overlaps with the configured version range (fromVersion -> toVersion)
//
// Examples:
//   - Config: fromVersion=v7.5.0, toVersion=v8.5.0
//     Upgrade: v7.5.0 -> v8.5.0 -> Should check (overlap: v7.5.0 to v8.5.0)
//   - Config: fromVersion=v7.5.0, toVersion=v8.5.0
//     Upgrade: v6.5.0 -> v8.5.0 -> Should check (overlap: v7.5.0 to v8.5.0)
//   - Config: fromVersion=v7.5.0, toVersion=v8.5.0
//     Upgrade: v6.5.0 -> v7.5.0 -> Should not check (no overlap)
func (r *HighRiskParamsRule) isVersionApplicableForUpgrade(sourceVersion, targetVersion, fromVersion, toVersion string) bool {
	// Normalize versions (remove 'v' prefix if present)
	sourceVersion = strings.TrimPrefix(sourceVersion, "v")
	targetVersion = strings.TrimPrefix(targetVersion, "v")
	fromVersion = strings.TrimPrefix(fromVersion, "v")
	toVersion = strings.TrimPrefix(toVersion, "v")

	// If both fromVersion and toVersion are empty, applies to all versions
	if fromVersion == "" && toVersion == "" {
		return true
	}

	// If targetVersion is not specified, check if sourceVersion is in the version range
	// This is used for testing and simple version checks
	if targetVersion == "" {
		// Check if sourceVersion is in [fromVersion, toVersion)
		// fromVersion is inclusive, toVersion is exclusive
		if fromVersion != "" {
			if compareVersions(sourceVersion, fromVersion) < 0 {
				return false // sourceVersion is before fromVersion
			}
		}
		if toVersion != "" {
			if compareVersions(sourceVersion, toVersion) >= 0 {
				return false // sourceVersion is at or after toVersion (exclusive)
			}
		}
		return true
	}

	// If fromVersion is empty but toVersion is specified
	// Check if upgrade path starts before toVersion (toVersion is exclusive)
	// If sourceVersion == toVersion, it's not in range (after the range)
	if fromVersion == "" {
		return compareVersions(sourceVersion, toVersion) < 0
	}

	// If toVersion is empty but fromVersion is specified
	// Check if upgrade path ends at or after fromVersion
	if toVersion == "" {
		return compareVersions(targetVersion, fromVersion) >= 0
	}

	// Both fromVersion and toVersion are specified
	// Check if upgrade path overlaps with [fromVersion, toVersion)
	// Overlap exists if:
	//   - sourceVersion < toVersion (sourceVersion is before toVersion, exclusive) AND targetVersion >= fromVersion
	sourceBeforeTo := compareVersions(sourceVersion, toVersion) < 0
	targetAfterFrom := compareVersions(targetVersion, fromVersion) >= 0

	return sourceBeforeTo && targetAfterFrom
}
