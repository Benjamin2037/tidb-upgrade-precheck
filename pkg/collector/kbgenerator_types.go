// Package collector provides knowledge base generation functionality
// Common types are now in pkg/types package
package collector

import (
	"github.com/pingcap/tidb-upgrade-precheck/pkg/types"
)

// Type aliases for backward compatibility
// These are now defined in pkg/types package
type (
	ComponentType        = types.ComponentType
	ParameterValue       = types.ParameterValue
	ConfigDefaults       = types.ConfigDefaults
	SystemVariables      = types.SystemVariables
	KBSnapshot           = types.KBSnapshot
	UpgradeParamChange   = types.UpgradeParamChange
	UpgradeLogicSnapshot = types.UpgradeLogicSnapshot
)

// Constants for backward compatibility
const (
	ComponentTiDB    = types.ComponentTiDB
	ComponentPD      = types.ComponentPD
	ComponentTiKV    = types.ComponentTiKV
	ComponentTiFlash = types.ComponentTiFlash
)

// SaveKBSnapshot saves a KB snapshot to a file
func SaveKBSnapshot(snapshot *KBSnapshot, outputPath string) error {
	return types.SaveKBSnapshot(snapshot, outputPath)
}

// SaveUpgradeLogic saves upgrade logic to a file
func SaveUpgradeLogic(snapshot *UpgradeLogicSnapshot, outputPath string) error {
	return types.SaveUpgradeLogic(snapshot, outputPath)
}
