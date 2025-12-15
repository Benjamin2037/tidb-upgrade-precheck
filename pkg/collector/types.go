// Package collector provides types for the collector package
package collector

import (
	"github.com/pingcap/tidb-upgrade-precheck/pkg/types"
	defaultsTypes "github.com/pingcap/tidb-upgrade-precheck/pkg/types"
)

// Component type constants (aliases for pkg/types constants)
// Note: ComponentType is already defined in kbgenerator_types.go
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

// Type aliases for backward compatibility
// These types are now defined in pkg/types package
type (
	ComponentState   = defaultsTypes.ComponentState
	InstanceState    = defaultsTypes.InstanceState
	ClusterState     = defaultsTypes.ClusterState
	ClusterSnapshot  = defaultsTypes.ClusterSnapshot
	ClusterEndpoints = defaultsTypes.ClusterEndpoints
)

// ConvertConfigToDefaults converts a map[string]interface{} to pkg/types.ConfigDefaults
// This is used when collecting runtime configuration to maintain consistency with knowledge base format
func ConvertConfigToDefaults(config map[string]interface{}) types.ConfigDefaults {
	return defaultsTypes.ConvertConfigToDefaults(config)
}

// ConvertVariablesToSystemVariables converts a map[string]string to pkg/types.SystemVariables
// This is used when collecting runtime system variables to maintain consistency with knowledge base format
func ConvertVariablesToSystemVariables(variables map[string]string) types.SystemVariables {
	return defaultsTypes.ConvertVariablesToSystemVariables(variables)
}
