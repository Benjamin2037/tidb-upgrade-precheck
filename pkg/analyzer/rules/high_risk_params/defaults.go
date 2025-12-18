// Package high_risk_params provides default high-risk parameters definitions
package high_risk_params

import (
	"github.com/pingcap/tidb-upgrade-precheck/pkg/analyzer/rules"
)

// GetDefaultHighRiskParams returns the default high-risk parameters configuration
// organized by component and version range
func GetDefaultHighRiskParams() *rules.HighRiskParamsConfig {
	config := &rules.HighRiskParamsConfig{}

	// Initialize maps
	config.TiDB.Config = make(map[string]rules.HighRiskParamConfig)
	config.TiDB.SystemVariables = make(map[string]rules.HighRiskParamConfig)
	config.PD.Config = make(map[string]rules.HighRiskParamConfig)
	config.TiKV.Config = make(map[string]rules.HighRiskParamConfig)
	config.TiFlash.Config = make(map[string]rules.HighRiskParamConfig)

	// TiDB System Variables
	// ANALYZE concurrency parameter change in v8.5+
	// Old parameter: tidb_distsql_scan_concurrency
	// New parameter: tidb_analyze_distsql_scan_concurrency
	// The old parameter is still available but only for non-ANALYZE scenarios
	config.TiDB.SystemVariables["tidb_distsql_scan_concurrency"] = rules.HighRiskParamConfig{
		Severity:      "warning",
		Description:   "In v8.5+, ANALYZE operations use a separate parameter 'tidb_analyze_distsql_scan_concurrency'. The current parameter 'tidb_distsql_scan_concurrency' now only controls concurrency for non-ANALYZE scenarios. If you have customized this parameter for ANALYZE operations, you may need to set 'tidb_analyze_distsql_scan_concurrency' separately.",
		CheckModified: true,
		FromVersion:   "v8.5.0",
		ToVersion:     "", // Applies to all versions after v8.5.0
	}

	// TiKV Config Parameters
	// gRPC parameters default value change may cause performance regression in <=16 core environments
	config.TiKV.Config["grpc-raft-conn-num"] = rules.HighRiskParamConfig{
		Severity:      "warning",
		Description:   "Default value change for this parameter may cause performance regression in environments with 16 cores or less. Please review the new default value and consider adjusting if your TiKV nodes have ≤16 CPU cores.",
		CheckModified: true,
		FromVersion:   "v8.5.0",
		ToVersion:     "", // Applies to all versions after v8.5.0
	}

	config.TiKV.Config["grpc-concurrency"] = rules.HighRiskParamConfig{
		Severity:      "warning",
		Description:   "Default value change for this parameter may cause performance regression in environments with 16 cores or less. Please review the new default value and consider adjusting if your TiKV nodes have ≤16 CPU cores.",
		CheckModified: true,
		FromVersion:   "v8.5.0",
		ToVersion:     "", // Applies to all versions after v8.5.0
	}

	// Region-split-size default value changed from 96MB to 256MB
	// But if not explicitly set before upgrade, it will keep the old default (96MB) after upgrade
	config.TiKV.Config["coprocessor.region-split-size"] = rules.HighRiskParamConfig{
		Severity:      "warning",
		Description:   "The default value has changed from 96MB to 256MB in the target version. However, since this parameter was not explicitly set in your current cluster (using default 96MB), it will continue to use 96MB after upgrade, NOT the new default 256MB. If you want to use the new default (256MB), you need to explicitly set it after upgrade.",
		CheckModified: false, // Report even if using default value
		FromVersion:   "v8.5.0",
		ToVersion:     "", // Applies to all versions after v8.5.0
	}

	return config
}

