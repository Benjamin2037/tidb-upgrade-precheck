// Package types provides common types used across the tidb-upgrade-precheck project
// These types are shared by kbgenerator, collector, analyzer, and reporter modules
package types

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// ComponentType represents the type of component
type ComponentType string

const (
	// ComponentTiDB represents a TiDB component
	ComponentTiDB ComponentType = "tidb"
	// ComponentPD represents a PD component
	ComponentPD ComponentType = "pd"
	// ComponentTiKV represents a TiKV component
	ComponentTiKV ComponentType = "tikv"
	// ComponentTiFlash represents a TiFlash component
	ComponentTiFlash ComponentType = "tiflash"
)

// ParameterValue represents a parameter value with its type information
// This is used for both configuration parameters and system variables
type ParameterValue struct {
	Value       interface{} `json:"value"`
	Type        string      `json:"type"` // "string", "int", "float", "bool", "duration", "size", "array", "map"
	Description string      `json:"description,omitempty"`
}

// ConfigDefaults represents configuration parameter defaults for a component
type ConfigDefaults map[string]ParameterValue

// SystemVariables represents system variable defaults for a component (mainly for TiDB)
type SystemVariables map[string]ParameterValue

// KBSnapshot represents a knowledge base snapshot for any component
// This is a generic structure that can be used by TiDB, PD, TiKV, TiFlash, etc.
type KBSnapshot struct {
	Component        ComponentType   `json:"component"`
	Version          string          `json:"version"`
	ConfigDefaults   ConfigDefaults  `json:"config_defaults"`
	SystemVariables  SystemVariables `json:"system_variables,omitempty"` // Only for TiDB
	BootstrapVersion int64           `json:"bootstrap_version"`          // Always include, even if 0 (extraction failed)
}

// UpgradeParamChange represents a forced parameter change during upgrade
// This unified structure is used for both config parameters and system variables
// across all components (TiDB, PD, TiKV, TiFlash)
type UpgradeParamChange struct {
	Version     string      `json:"version"`               // Version when this change was introduced (e.g., "v7.5.0")
	Name        string      `json:"name"`                  // Parameter name (config key or system variable name)
	VarName     string      `json:"var_name,omitempty"`    // Alias for Name (TiDB compatibility)
	Value       interface{} `json:"value"`                 // New value that will be forced
	FromValue   interface{} `json:"from_value,omitempty"`  // Old value that will be mapped to new value (for value migration)
	Description string      `json:"description,omitempty"` // Description of the change
	Comment     string      `json:"comment,omitempty"`     // Alias for Description (TiDB compatibility)
	Force       bool        `json:"force"`                 // Always true for upgrade logic changes
	Type        string      `json:"type,omitempty"`        // "config" or "system_variable"
	FuncName    string      `json:"func_name,omitempty"`   // Function name where change occurs (TiDB-specific)
	Method      string      `json:"method,omitempty"`      // Method used to apply change (TiDB-specific)
	Severity    string      `json:"severity,omitempty"`    // Risk severity: "medium" (UPDATE/REPLACE - default value behavior changed), "low-medium" (DELETE - deprecated)
}

// UpgradeLogicSnapshot represents upgrade logic for a component
// Changes contains forced parameter changes for the component
type UpgradeLogicSnapshot struct {
	Component ComponentType        `json:"component"`
	Changes   []UpgradeParamChange `json:"changes"`
}

// SaveKBSnapshot saves a KB snapshot to a file
func SaveKBSnapshot(snapshot *KBSnapshot, outputPath string) error {
	return saveJSON(snapshot, outputPath)
}

// SaveUpgradeLogic saves upgrade logic to a file
func SaveUpgradeLogic(snapshot *UpgradeLogicSnapshot, outputPath string) error {
	return saveJSON(snapshot, outputPath)
}

// Helper function to save JSON
func saveJSON(data interface{}, outputPath string) error {
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	if err := os.WriteFile(outputPath, jsonData, 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

// ConvertConfigToDefaults converts a map[string]interface{} to ConfigDefaults
// This is used when collecting runtime configuration to maintain consistency with knowledge base format
func ConvertConfigToDefaults(config map[string]interface{}) ConfigDefaults {
	defaults := make(ConfigDefaults)
	for k, v := range config {
		defaults[k] = ParameterValue{
			Value: v,
			Type:  determineValueType(v),
		}
	}
	return defaults
}

// ConvertVariablesToSystemVariables converts a map[string]string to SystemVariables
// This is used when collecting runtime system variables to maintain consistency with knowledge base format
func ConvertVariablesToSystemVariables(variables map[string]string) SystemVariables {
	sysVars := make(SystemVariables)
	for k, v := range variables {
		sysVars[k] = ParameterValue{
			Value: v,
			Type:  "string", // System variables are typically strings
		}
	}
	return sysVars
}

// determineValueType determines the type of a value
func determineValueType(v interface{}) string {
	switch v.(type) {
	case string:
		return "string"
	case int, int8, int16, int32, int64:
		return "int"
	case uint, uint8, uint16, uint32, uint64:
		return "int"
	case float32, float64:
		return "float"
	case bool:
		return "bool"
	case []interface{}:
		return "array"
	case map[string]interface{}:
		return "map"
	default:
		return "string"
	}
}
