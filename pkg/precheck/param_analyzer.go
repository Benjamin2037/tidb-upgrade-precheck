package precheck

import (
	"fmt"

	"github.com/pingcap/tidb-upgrade-precheck/pkg/runtime"
)

// ParameterState represents the state of a parameter
type ParameterState string

const (
	UseDefault ParameterState = "use_default" // Using default value
	UserSet    ParameterState = "user_set"    // Modified by user
)

// ParameterAnalysis represents the analysis result of a parameter
type ParameterAnalysis struct {
	Name          string          `json:"name"`
	Component     string          `json:"component"`
	CurrentValue  interface{}     `json:"current_value"`
	SourceDefault interface{}     `json:"source_default"`
	TargetDefault interface{}     `json:"target_default"`
	State         ParameterState  `json:"state"`
}

// ParamAnalyzer analyzes parameter states and identifies upgrade risks
type ParamAnalyzer struct {
}

// NewParamAnalyzer creates a new parameter analyzer
func NewParamAnalyzer() *ParamAnalyzer {
	return &ParamAnalyzer{}
}

// AnalyzeParameterState determines whether each parameter is using its default value or has been modified by the user
func (pa *ParamAnalyzer) AnalyzeParameterState(
	snapshot *runtime.ClusterSnapshot,
	sourceKB map[string]interface{}, // source version knowledge base
	targetKB map[string]interface{}, // target version knowledge base
) ([]*ParameterAnalysis, error) {
	if snapshot == nil {
		return nil, fmt.Errorf("snapshot cannot be nil")
	}

	var analyses []*ParameterAnalysis

	// Analyze TiDB parameters
	if tidbComponent, exists := snapshot.Components["tidb"]; exists {
		// Analyze system variables
		sourceSysVarDefaults := make(map[string]interface{})
		targetSysVarDefaults := make(map[string]interface{})
		
		if sourceSysVars, ok := sourceKB["system_variables"].(map[string]interface{}); ok {
			sourceSysVarDefaults = sourceSysVars
		}
		
		if targetSysVars, ok := targetKB["system_variables"].(map[string]interface{}); ok {
			targetSysVarDefaults = targetSysVars
		}
		
		for name, currentValue := range tidbComponent.Variables {
			analysis := &ParameterAnalysis{
				Name:         name,
				Component:    "tidb",
				CurrentValue: currentValue,
				State:        UseDefault, // default assumption
			}

			// Get source and target defaults
			if sourceDefault, exists := sourceSysVarDefaults[name]; exists {
				analysis.SourceDefault = sourceDefault
			}

			if targetDefault, exists := targetSysVarDefaults[name]; exists {
				analysis.TargetDefault = targetDefault
			}

			// Determine if parameter is user-set
			if analysis.SourceDefault != nil && fmt.Sprintf("%v", analysis.SourceDefault) != fmt.Sprintf("%v", currentValue) {
				analysis.State = UserSet
			}

			analyses = append(analyses, analysis)
		}

		// Analyze configuration parameters
		sourceConfigDefaults := make(map[string]interface{})
		targetConfigDefaults := make(map[string]interface{})
		
		if sourceConfigs, ok := sourceKB["config_defaults"].(map[string]interface{}); ok {
			sourceConfigDefaults = sourceConfigs
		}
		
		if targetConfigs, ok := targetKB["config_defaults"].(map[string]interface{}); ok {
			targetConfigDefaults = targetConfigs
		}
		
		for name, currentValue := range tidbComponent.Config {
			analysis := &ParameterAnalysis{
				Name:         name,
				Component:    "tidb",
				CurrentValue: currentValue,
				State:        UseDefault, // default assumption
			}

			// Get source and target defaults
			if sourceDefault, exists := sourceConfigDefaults[name]; exists {
				analysis.SourceDefault = sourceDefault
			}

			if targetDefault, exists := targetConfigDefaults[name]; exists {
				analysis.TargetDefault = targetDefault
			}

			// Determine if parameter is user-set
			if analysis.SourceDefault != nil && fmt.Sprintf("%v", analysis.SourceDefault) != fmt.Sprintf("%v", currentValue) {
				analysis.State = UserSet
			}

			analyses = append(analyses, analysis)
		}
	}

	// TODO: Analyze TiKV and PD parameters in a similar way

	return analyses, nil
}

// IdentifyRisks identifies risks based on parameter analysis
func (pa *ParamAnalyzer) IdentifyRisks(analyses []*ParameterAnalysis, forcedChanges map[string]interface{}) []*RiskItem {
	var risks []*RiskItem

	for _, analysis := range analyses {
		risk := pa.assessRisk(analysis, forcedChanges)
		if risk != nil {
			risks = append(risks, risk)
		}
	}

	return risks
}

// assessRisk assesses the risk level of a parameter according to the risk matrix
func (pa *ParamAnalyzer) assessRisk(analysis *ParameterAnalysis, forcedChanges map[string]interface{}) *RiskItem {
	// Check if parameter is forcibly changed during upgrade (HIGH risk)
	if _, isForced := forcedChanges[analysis.Name]; isForced {
		return &RiskItem{
			Level:     RiskHigh,
			Parameter: analysis.Name,
			Component: analysis.Component,
			Message:   fmt.Sprintf("Parameter will be forcibly changed during upgrade from '%v' to '%v'", analysis.CurrentValue, forcedChanges[analysis.Name]),
			Details:   "This is a forced change during the upgrade process and cannot be overridden",
		}
	}

	// Check if default value changes (MEDIUM risk for user-set parameters)
	if analysis.State == UserSet && 
	   analysis.SourceDefault != nil && 
	   analysis.TargetDefault != nil &&
	   fmt.Sprintf("%v", analysis.SourceDefault) != fmt.Sprintf("%v", analysis.TargetDefault) {
		return &RiskItem{
			Level:     RiskMedium,
			Parameter: analysis.Name,
			Component: analysis.Component,
			Message:   fmt.Sprintf("Default value changes from '%v' to '%v' in target version", analysis.SourceDefault, analysis.TargetDefault),
			Details:   "You have customized this parameter and the default is changing in the target version",
		}
	}

	// Check if default value changes (INFO for default parameters)
	if analysis.State == UseDefault && 
	   analysis.SourceDefault != nil && 
	   analysis.TargetDefault != nil &&
	   fmt.Sprintf("%v", analysis.SourceDefault) != fmt.Sprintf("%v", analysis.TargetDefault) {
		return &RiskItem{
			Level:     RiskInfo,
			Parameter: analysis.Name,
			Component: analysis.Component,
			Message:   fmt.Sprintf("Default value changes from '%v' to '%v' in target version", analysis.SourceDefault, analysis.TargetDefault),
			Details:   "The default value for this parameter is changing in the target version",
		}
	}

	// No significant risk identified
	return nil
}

// GetForcedChanges extracts forced changes from knowledge base
func (pa *ParamAnalyzer) GetForcedChanges(targetKB map[string]interface{}) map[string]interface{} {
	forcedChanges := make(map[string]interface{})
	
	// Extract forced changes from upgrade logic if available
	if upgradeLogic, ok := targetKB["upgrade_logic"].([]interface{}); ok {
		for _, change := range upgradeLogic {
			if changeMap, ok := change.(map[string]interface{}); ok {
				if variable, ok := changeMap["variable"].(string); ok {
					if forcedValue, ok := changeMap["forced_value"]; ok {
						forcedChanges[variable] = forcedValue
					}
				}
			}
		}
	}
	
	return forcedChanges
}