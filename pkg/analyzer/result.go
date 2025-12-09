// Package analyzer provides risk analysis logic for upgrade precheck
package analyzer

import (
	"github.com/pingcap/tidb-upgrade-precheck/pkg/analyzer/rules"
)

// AnalysisResult contains the complete analysis results
// This structure is designed for reporter to display
type AnalysisResult struct {
	// SourceVersion is the current cluster version
	SourceVersion string `json:"source_version"`
	// TargetVersion is the target version for upgrade
	TargetVersion string `json:"target_version"`

	// ModifiedParams contains parameters that have been modified from source defaults
	// Structure: map[component]map[param_name]ModifiedParamInfo
	ModifiedParams map[string]map[string]ModifiedParamInfo `json:"modified_params"`

	// TikvInconsistencies contains TiKV nodes with inconsistent parameters
	// Structure: map[param_name][]InconsistentNode
	TikvInconsistencies map[string][]InconsistentNode `json:"tikv_inconsistencies"`

	// UpgradeDifferences contains parameters that will differ after upgrade
	// Structure: map[component]map[param_name]UpgradeDifference
	UpgradeDifferences map[string]map[string]UpgradeDifference `json:"upgrade_differences"`

	// ForcedChanges contains parameters that will be forcibly changed during upgrade
	// Structure: map[component]map[param_name]ForcedChange
	ForcedChanges map[string]map[string]ForcedChange `json:"forced_changes"`

	// FocusParams contains focus parameters specified by user
	// These are always reported regardless of changes
	// Structure: map[component]map[param_name]FocusParamInfo
	FocusParams map[string]map[string]FocusParamInfo `json:"focus_params"`

	// CheckResults contains all rule check results
	CheckResults []rules.CheckResult `json:"check_results"`
}

// ModifiedParamInfo contains information about a modified parameter
type ModifiedParamInfo struct {
	// Component is the component name (tidb, pd, tikv, tiflash)
	Component string `json:"component"`
	// ParamName is the parameter name
	ParamName string `json:"param_name"`
	// CurrentValue is the current value in the cluster
	CurrentValue interface{} `json:"current_value"`
	// SourceDefault is the default value in source version
	SourceDefault interface{} `json:"source_default"`
	// ParamType is "config" or "system_variable"
	ParamType string `json:"param_type"`
}

// InconsistentNode represents a TiKV node with inconsistent parameter value
type InconsistentNode struct {
	// NodeAddress is the address of the TiKV node
	NodeAddress string `json:"node_address"`
	// Value is the parameter value on this node
	Value interface{} `json:"value"`
}

// UpgradeDifference contains information about parameter differences after upgrade
type UpgradeDifference struct {
	// Component is the component name
	Component string `json:"component"`
	// ParamName is the parameter name
	ParamName string `json:"param_name"`
	// CurrentValue is the current value in the cluster
	CurrentValue interface{} `json:"current_value"`
	// TargetDefault is the default value in target version
	TargetDefault interface{} `json:"target_default"`
	// SourceDefault is the default value in source version
	SourceDefault interface{} `json:"source_default"`
	// ParamType is "config" or "system_variable"
	ParamType string `json:"param_type"`
}

// ForcedChange contains information about a forced parameter change during upgrade
type ForcedChange struct {
	// Component is the component name
	Component string `json:"component"`
	// ParamName is the parameter name
	ParamName string `json:"param_name"`
	// CurrentValue is the current value in the cluster
	CurrentValue interface{} `json:"current_value"`
	// ForcedValue is the value that will be forced during upgrade
	ForcedValue interface{} `json:"forced_value"`
	// SourceDefault is the default value in source version
	SourceDefault interface{} `json:"source_default"`
	// ParamType is "config" or "system_variable"
	ParamType string `json:"param_type"`
	// Summary is the summary of the forced change
	Summary string `json:"summary,omitempty"`
	// Scope is the scope of the change (global, session, etc.)
	Scope string `json:"scope,omitempty"`
}

// FocusParamInfo contains information about a focus parameter
type FocusParamInfo struct {
	// Component is the component name
	Component string `json:"component"`
	// ParamName is the parameter name
	ParamName string `json:"param_name"`
	// CurrentValue is the current value in the cluster
	CurrentValue interface{} `json:"current_value"`
	// SourceDefault is the default value in source version
	SourceDefault interface{} `json:"source_default"`
	// TargetDefault is the default value in target version
	TargetDefault interface{} `json:"target_default"`
	// ParamType is "config" or "system_variable"
	ParamType string `json:"param_type"`
	// IsModified indicates if the parameter has been modified from source default
	IsModified bool `json:"is_modified"`
	// WillChange indicates if the parameter will change after upgrade
	WillChange bool `json:"will_change"`
}

