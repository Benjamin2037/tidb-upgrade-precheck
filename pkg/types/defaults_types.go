// Package types provides common types used across the tidb-upgrade-precheck project
// These types are shared by kbgenerator, collector, analyzer, and reporter modules
package types

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
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
	SystemVariables  SystemVariables `json:"system_variables,omitempty"` // Only for TiDB and TiFlash
	BootstrapVersion int64           `json:"bootstrap_version"`          // Always include, even if 0 (extraction failed)
}

// UpgradeParamChange represents a forced parameter change during upgrade
// This unified structure is used for both config parameters and system variables
// across all components (TiDB, PD, TiKV, TiFlash)
type UpgradeParamChange struct {
	Version       string      `json:"version"`                 // Version when this change was introduced (e.g., "v7.5.0")
	Name          string      `json:"name"`                    // Parameter name (config key or system variable name)
	VarName       string      `json:"var_name,omitempty"`      // Alias for Name (TiDB compatibility)
	Value         interface{} `json:"value"`                   // New value that will be forced
	FromValue     interface{} `json:"from_value,omitempty"`    // Old value that will be mapped to new value (for value migration)
	Description   string      `json:"description,omitempty"`   // Description of the change
	Comment       string      `json:"comment,omitempty"`       // Alias for Description (TiDB compatibility)
	Force         bool        `json:"force"`                   // Always true for upgrade logic changes
	Type          string      `json:"type,omitempty"`          // "config" or "system_variable"
	FuncName      string      `json:"func_name,omitempty"`     // Function name where change occurs (TiDB-specific)
	Method        string      `json:"method,omitempty"`        // Method used to apply change (TiDB-specific)
	Severity      string      `json:"severity,omitempty"`      // Risk severity: "medium" (UPDATE/REPLACE - default value behavior changed), "low-medium" (DELETE - deprecated)
	DetailsNote   string      `json:"details_note,omitempty"`  // Additional note to append to details message
	Suggestions   []string    `json:"suggestions,omitempty"`   // Custom suggestions for this parameter (overrides default)
	ReportSeverity string     `json:"report_severity,omitempty"` // Override default report severity: "error", "warning", "info"
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

// ComponentState represents the state of a component including its configuration
// This type is used by the collector package to represent runtime component state
type ComponentState struct {
	// Type is the type of the component (tidb, pd, tikv, tiflash)
	Type ComponentType `json:"type"`
	// Version is the version of the component
	Version string `json:"version"`
	// Config is the configuration of the component
	// Uses ConfigDefaults to maintain consistency with knowledge base format
	// Runtime values are converted to ParameterValue format
	Config ConfigDefaults `json:"config"`
	// Variables are system variables (for TiDB only)
	// Uses SystemVariables to maintain consistency with knowledge base format
	Variables SystemVariables `json:"variables,omitempty"`
	// Status is the status information of the component
	Status map[string]interface{} `json:"status"`
}

// InstanceState represents the state of a component instance
type InstanceState struct {
	// Address is the address of the component instance
	Address string `json:"address"`
	// State is the state of the instance
	State ComponentState `json:"state"`
}

// ClusterState represents the state of a TiDB cluster
type ClusterState struct {
	// Instances is the list of component instances
	Instances []InstanceState `json:"instances"`
}

// ClusterSnapshot represents the complete configuration state of a cluster
type ClusterSnapshot struct {
	// Timestamp is when the snapshot was collected
	Timestamp time.Time `json:"timestamp"`
	// SourceVersion is the current version of the cluster
	SourceVersion string `json:"source_version"`
	// TargetVersion is the target version for upgrade (optional, set by caller)
	TargetVersion string `json:"target_version,omitempty"`
	// Components contains the state of each component
	Components map[string]ComponentState `json:"components"`
}

// ClusterEndpoints contains connection information for cluster components
// This structure is typically populated by external tools like TiUP or TiDB Operator
// that have access to the cluster topology and credentials
type ClusterEndpoints struct {
	// TiDBAddr is the MySQL protocol endpoint (host:port)
	TiDBAddr string `json:"tidb_addr,omitempty"`
	// TiDBUser is the MySQL username for TiDB connection (provided by TiUP/Operator)
	TiDBUser string `json:"tidb_user,omitempty"`
	// TiDBPassword is the MySQL password for TiDB connection (provided by TiUP/Operator)
	TiDBPassword string `json:"tidb_password,omitempty"`
	// TiKVAddrs are HTTP API endpoints for TiKV instances
	TiKVAddrs []string `json:"tikv_addrs,omitempty"`
	// TiKVDataDirs maps TiKV address to its data_dir path (from topology file)
	// This is required to read last_tikv.toml file which contains actual runtime configuration
	TiKVDataDirs map[string]string `json:"tikv_data_dirs,omitempty"`
	// PDAddrs are HTTP API endpoints for PD instances
	PDAddrs []string `json:"pd_addrs,omitempty"`
	// TiFlashAddrs are HTTP API endpoints for TiFlash instances
	TiFlashAddrs []string `json:"tiflash_addrs,omitempty"`
	// SourceVersion is the version extracted from topology file (if available)
	// This can be used as a fallback when cluster version detection fails
	SourceVersion string `json:"source_version,omitempty"`
}
