// Package rules provides standardized rule definitions for upgrade precheck
package rules

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/pingcap/tidb-upgrade-precheck/pkg/collector"
	tidbCollector "github.com/pingcap/tidb-upgrade-precheck/pkg/collector/runtime/tidb"
	defaultsTypes "github.com/pingcap/tidb-upgrade-precheck/pkg/types"
)

// TikvConsistencyRule checks if TiKV nodes have consistent parameter values
// Rule 2.3: Compare parameter values across all TiKV nodes
type TikvConsistencyRule struct {
	*BaseRule
}

// NewTikvConsistencyRule creates a new TiKV consistency rule
func NewTikvConsistencyRule() Rule {
	return &TikvConsistencyRule{
		BaseRule: NewBaseRule(
			"TIKV_CONSISTENCY",
			"Check if TiKV nodes have consistent parameter values across the cluster",
			"consistency",
		),
	}
}

// DataRequirements returns the data requirements for this rule
func (r *TikvConsistencyRule) DataRequirements() DataSourceRequirement {
	return DataSourceRequirement{
		SourceClusterRequirements: struct {
			Components          []string `json:"components"`
			NeedConfig          bool     `json:"need_config"`
			NeedSystemVariables bool     `json:"need_system_variables"`
			NeedAllTikvNodes    bool     `json:"need_all_tikv_nodes"`
		}{
			Components:          []string{"tikv"},
			NeedConfig:          true,
			NeedSystemVariables: false, // TiKV doesn't have system variables
			NeedAllTikvNodes:    true,  // Need all TiKV nodes for consistency check
		},
		// SourceKBRequirements: Not needed for this rule
		// This rule only checks consistency across TiKV nodes in the current cluster
		// It does not need any knowledge base data (source or target)
		SourceKBRequirements: struct {
			Components          []string `json:"components"`
			NeedConfigDefaults  bool     `json:"need_config_defaults"`
			NeedSystemVariables bool     `json:"need_system_variables"`
			NeedUpgradeLogic    bool     `json:"need_upgrade_logic"`
		}{
			Components:          []string{},
			NeedConfigDefaults:  false,
			NeedSystemVariables: false,
			NeedUpgradeLogic:    false,
		},
		// TargetKBRequirements: Not needed for this rule
		// This rule only checks consistency across TiKV nodes in the current cluster
		// It does not need any knowledge base data (source or target)
		TargetKBRequirements: struct {
			Components          []string `json:"components"`
			NeedConfigDefaults  bool     `json:"need_config_defaults"`
			NeedSystemVariables bool     `json:"need_system_variables"`
			NeedUpgradeLogic    bool     `json:"need_upgrade_logic"`
		}{
			Components:          []string{},
			NeedConfigDefaults:  false,
			NeedSystemVariables: false,
			NeedUpgradeLogic:    false,
		},
	}
}

// Evaluate performs the rule check
// This rule uses:
// 1. last_tikv.toml (user-set values) - from SourceClusterSnapshot
// 2. SHOW CONFIG WHERE type='tikv' AND instance='IP:port' (runtime values) - via TiDB connection
// These are merged with priority: runtime values > user-set values
func (r *TikvConsistencyRule) Evaluate(ctx context.Context, ruleCtx *RuleContext) ([]CheckResult, error) {
	var results []CheckResult

	if ruleCtx.SourceClusterSnapshot == nil {
		return results, nil
	}

	// Find TiDB component to get connection info
	var tidbAddr string
	var tidbUser, tidbPassword string
	for compName, component := range ruleCtx.SourceClusterSnapshot.Components {
		if component.Type == collector.TiDBComponent || strings.HasPrefix(compName, "tidb") {
			if addr, ok := component.Status["address"].(string); ok {
				tidbAddr = addr
			} else {
				tidbAddr = compName
			}
			// Try to get user and password from status
			if user, ok := component.Status["user"].(string); ok {
				tidbUser = user
			} else {
				tidbUser = "root" // Default
			}
			if password, ok := component.Status["password"].(string); ok {
				tidbPassword = password
			} else {
				tidbPassword = "" // Default
			}
			break
		}
	}

	if tidbAddr == "" {
		// Cannot check consistency without TiDB connection
		return results, nil
	}

	// Collect all TiKV nodes with their instance addresses (IP:port)
	type tikvNodeInfo struct {
		name          string
		address       string                       // HTTP address (from status)
		instance      string                       // Instance format: IP:port (for SHOW CONFIG)
		userSetConfig defaultsTypes.ConfigDefaults // From last_tikv.toml
	}

	var tikvNodes []tikvNodeInfo

	for compName, component := range ruleCtx.SourceClusterSnapshot.Components {
		if component.Type == collector.TiKVComponent || strings.HasPrefix(compName, "tikv") {
			// Get HTTP address from status or use component name
			address := compName
			if addr, ok := component.Status["address"].(string); ok {
				address = addr
			}

			// Extract instance (IP:port) from address
			// Address format is typically "IP:port" or "hostname:port"
			// For SHOW CONFIG, we need the actual IP:port format
			instance := address
			// If address contains hostname, we might need to resolve it
			// For now, assume address is already in IP:port format or can be used directly

			// User-set config from last_tikv.toml (already in component.Config)
			userSetConfig := component.Config

			tikvNodes = append(tikvNodes, tikvNodeInfo{
				name:          compName,
				address:       address,
				instance:      instance,
				userSetConfig: userSetConfig,
			})
		}
	}

	if len(tikvNodes) < 2 {
		// Need at least 2 nodes to check consistency
		return results, nil
	}

	// Connect to TiDB to get runtime configs via SHOW CONFIG
	db, err := sql.Open("mysql", fmt.Sprintf("%s:%s@tcp(%s)/", tidbUser, tidbPassword, tidbAddr))
	if err != nil {
		return results, fmt.Errorf("failed to connect to TiDB: %w", err)
	}
	defer db.Close()
	db.SetConnMaxLifetime(10 * time.Second)

	collector := tidbCollector.NewTiDBCollector()

	// Collect merged configs for each node (runtime > user-set)
	nodeConfigs := make(map[string]defaultsTypes.ConfigDefaults) // instance -> merged config
	for _, node := range tikvNodes {
		// Step 1: Start with user-set values from last_tikv.toml
		mergedConfig := make(defaultsTypes.ConfigDefaults)
		for k, v := range node.userSetConfig {
			mergedConfig[k] = v
		}

		// Step 2: Get runtime values via SHOW CONFIG WHERE type='tikv' AND instance='...'
		runtimeConfig, err := collector.GetConfigByTypeAndInstance(db, "tikv", node.instance)
		if err != nil {
			// Log warning but continue with user-set config only
			fmt.Printf("Warning: failed to get runtime config for TiKV instance %s: %v\n", node.instance, err)
		} else {
			// Step 3: Merge with priority: runtime values > user-set values
			for k, v := range runtimeConfig {
				// Convert to ParameterValue format
				mergedConfig[k] = defaultsTypes.ParameterValue{
					Value: v,
					Type:  determineValueType(v),
				}
			}
		}

		nodeConfigs[node.instance] = mergedConfig
	}

	// Collect all parameter names from all nodes
	paramNames := make(map[string]bool)
	for _, config := range nodeConfigs {
		for paramName := range config {
			paramNames[paramName] = true
		}
	}

	// Check each parameter for consistency
	for paramName := range paramNames {
		// Get values from all nodes
		nodeValues := make(map[interface{}][]string) // value -> []instances
		for instance, config := range nodeConfigs {
			if paramValue, ok := config[paramName]; ok {
				value := paramValue.Value
				nodeValues[value] = append(nodeValues[value], instance)
			}
		}

		// If there are different values, report inconsistency
		if len(nodeValues) > 1 {
			// Find the most common value (if any)
			var mostCommonValue interface{}
			maxCount := 0
			for value, addresses := range nodeValues {
				if len(addresses) > maxCount {
					maxCount = len(addresses)
					mostCommonValue = value
				}
			}

			// Build details
			var details strings.Builder
			details.WriteString(fmt.Sprintf("Parameter %s has inconsistent values across TiKV nodes:\n", paramName))
			details.WriteString("Configuration sources: last_tikv.toml (user-set) merged with SHOW CONFIG (runtime)\n")
			for value, instances := range nodeValues {
				if value == mostCommonValue {
					details.WriteString(fmt.Sprintf("  Value %v (most common): instances %v\n", value, instances))
				} else {
					details.WriteString(fmt.Sprintf("  Value %v (different): instances %v\n", value, instances))
				}
			}

			// Determine severity based on parameter importance
			severity := "warning"
			// Some parameters are more critical if inconsistent
			criticalParams := []string{
				"storage.reserve-space",
				"raftstore.raft-entry-max-size",
				"rocksdb.defaultcf.block-cache-size",
			}
			for _, critical := range criticalParams {
				if paramName == critical {
					severity = "error"
					break
				}
			}

			results = append(results, CheckResult{
				RuleID:        r.Name(),
				Category:      r.Category(),
				Component:     "tikv",
				ParameterName: paramName,
				ParamType:     "config",
				Severity:      severity,
				Message:       fmt.Sprintf("Parameter %s has different values across TiKV nodes", paramName),
				Details:       details.String(),
				Suggestions: []string{
					"Align parameter values across all TiKV nodes for optimal performance",
					"Review why different values are configured",
					"Consider using consistent configuration for all nodes",
				},
				Metadata: map[string]interface{}{
					"param_name":         paramName,
					"node_count":         len(tikvNodes),
					"value_count":        len(nodeValues),
					"inconsistent_nodes": len(nodeValues) - 1,
					"config_sources":     []string{"last_tikv.toml", "SHOW CONFIG WHERE type='tikv' AND instance='...'"},
				},
			})
		}
	}

	return results, nil
}

// determineValueType determines the type of a value
func determineValueType(v interface{}) string {
	switch v.(type) {
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return "int"
	case float32, float64:
		return "float"
	case bool:
		return "bool"
	case string:
		return "string"
	default:
		return "string"
	}
}
