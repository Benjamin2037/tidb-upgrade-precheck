// Package collector provides types for the collector package
package collector

import (
	"time"

	defaultsTypes "github.com/pingcap/tidb-upgrade-precheck/pkg/types"
)

// ComponentType is an alias for pkg/types.ComponentType
type ComponentType = defaultsTypes.ComponentType

// Component type constants (aliases for pkg/types constants)
const (
	// TiDBComponent represents a TiDB component
	TiDBComponent = defaultsTypes.ComponentTiDB
	// PDComponent represents a PD component
	PDComponent = defaultsTypes.ComponentPD
	// TiKVComponent represents a TiKV component
	TiKVComponent = defaultsTypes.ComponentTiKV
	// TiFlashComponent represents a TiFlash component
	TiFlashComponent = defaultsTypes.ComponentTiFlash
)

// ComponentState represents the state of a component including its configuration
type ComponentState struct {
	// Type is the type of the component (tidb, pd, tikv, tiflash)
	Type ComponentType `json:"type"`
	// Version is the version of the component
	Version string `json:"version"`
	// Config is the configuration of the component
	// Uses pkg/types.ConfigDefaults to maintain consistency with knowledge base format
	// Runtime values are converted to ParameterValue format
	Config defaultsTypes.ConfigDefaults `json:"config"`
	// Variables are system variables (for TiDB only)
	// Uses pkg/types.SystemVariables to maintain consistency with knowledge base format
	Variables defaultsTypes.SystemVariables `json:"variables,omitempty"`
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

// ConvertConfigToDefaults converts a map[string]interface{} to pkg/types.ConfigDefaults
// This is used when collecting runtime configuration to maintain consistency with knowledge base format
func ConvertConfigToDefaults(config map[string]interface{}) defaultsTypes.ConfigDefaults {
	return defaultsTypes.ConvertConfigToDefaults(config)
}

// ConvertVariablesToSystemVariables converts a map[string]string to pkg/types.SystemVariables
// This is used when collecting runtime system variables to maintain consistency with knowledge base format
func ConvertVariablesToSystemVariables(variables map[string]string) defaultsTypes.SystemVariables {
	return defaultsTypes.ConvertVariablesToSystemVariables(variables)
}
