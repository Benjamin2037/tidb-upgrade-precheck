// Package rules provides standardized rule definitions for upgrade precheck
package rules

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/pingcap/tidb-upgrade-precheck/pkg/analyzer"
	"github.com/pingcap/tidb-upgrade-precheck/pkg/collector"
	tidbCollector "github.com/pingcap/tidb-upgrade-precheck/pkg/collector/tidb"
	defaultsTypes "github.com/pingcap/tidb-upgrade-precheck/pkg/types"
)

// TikvConsistencyRule compares all TiKV node parameters for consistency
// Rule: Compare all TiKV node parameters with the first TiKV node (baseline)
// Reports differences as medium risk (warning)
// This rule is used for TiKV scale out precheck to ensure all TiKV nodes have consistent parameters
type TikvConsistencyRule struct {
	*BaseRule
}

// NewTikvConsistencyRule creates a new TiKV consistency rule
func NewTikvConsistencyRule() Rule {
	return &TikvConsistencyRule{
		BaseRule: NewBaseRule(
			"TIKV_CONSISTENCY",
			"Compare all TiKV node parameters for consistency (all nodes vs first node)",
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
			NeedAllTikvNodes:    true,  // Need all TiKV nodes
		},
		SourceKBRequirements: struct {
			Components          []string `json:"components"`
			NeedConfigDefaults  bool     `json:"need_config_defaults"`
			NeedSystemVariables bool     `json:"need_system_variables"`
			NeedUpgradeLogic    bool     `json:"need_upgrade_logic"`
		}{
			Components:          []string{}, // This rule doesn't need knowledge base data
			NeedConfigDefaults:  false,
			NeedSystemVariables: false,
			NeedUpgradeLogic:    false,
		},
		TargetKBRequirements: struct {
			Components          []string `json:"components"`
			NeedConfigDefaults  bool     `json:"need_config_defaults"`
			NeedSystemVariables bool     `json:"need_system_variables"`
			NeedUpgradeLogic    bool     `json:"need_upgrade_logic"`
		}{
			Components:          []string{}, // This rule doesn't need knowledge base data
			NeedConfigDefaults:  false,
			NeedSystemVariables: false,
			NeedUpgradeLogic:    false,
		},
	}
}

// Evaluate performs the rule check
// Logic:
// 1. Collect all TiKV node parameters (last_tikv.toml + SHOW CONFIG, merged with runtime priority)
// 2. Use the first TiKV node as baseline
// 3. Compare all other TiKV nodes with the baseline node
// 4. Report differences as medium risk (warning)
// 5. Each node-parameter combination is one entry
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

	// Collect all TiKV nodes with their instance addresses (IP:port) and merged configs
	type tikvNodeInfo struct {
		name         string
		address      string                       // HTTP address (from status)
		instance     string                       // Instance format: IP:port (for SHOW CONFIG)
		mergedConfig defaultsTypes.ConfigDefaults // Merged config (last_tikv.toml + SHOW CONFIG)
	}

	var tikvNodes []tikvNodeInfo

	// Connect to TiDB to get runtime configs via SHOW CONFIG (if available)
	var db *sql.DB
	var collector tidbCollector.TiDBCollector
	if tidbAddr != "" {
		var err error
		db, err = sql.Open("mysql", fmt.Sprintf("%s:%s@tcp(%s)/", tidbUser, tidbPassword, tidbAddr))
		if err == nil {
			defer db.Close()
			db.SetConnMaxLifetime(10 * time.Second)
			collector = tidbCollector.NewTiDBCollector()
		}
	}

	// Collect all TiKV nodes
	for compName, component := range ruleCtx.SourceClusterSnapshot.Components {
		if component.Type == collector.TiKVComponent || strings.HasPrefix(compName, "tikv") {
			// Get HTTP address from status or use component name
			address := compName
			if addr, ok := component.Status["address"].(string); ok {
				address = addr
			}

			// Extract instance (IP:port) from address
			instance := address

			// Step 1: Start with user-set values from last_tikv.toml
			mergedConfig := make(defaultsTypes.ConfigDefaults)
			for k, v := range component.Config {
				mergedConfig[k] = v
			}

			// Step 2: Get runtime values via SHOW CONFIG WHERE type='tikv' AND instance='...' (if available)
			if db != nil && collector != nil {
				runtimeConfig, err := collector.GetConfigByTypeAndInstance(db, "tikv", instance)
				if err == nil {
					// Step 3: Merge with priority: runtime values > user-set values
					for k, v := range runtimeConfig {
						mergedConfig[k] = defaultsTypes.ParameterValue{
							Value: v,
							Type:  determineValueType(v),
						}
					}
				}
			}

			tikvNodes = append(tikvNodes, tikvNodeInfo{
				name:         compName,
				address:      address,
				instance:     instance,
				mergedConfig: mergedConfig,
			})
		}
	}

	if len(tikvNodes) == 0 {
		return results, nil
	}

	// If there's only one TiKV node, skip consistency check (no other nodes to compare with)
	if len(tikvNodes) == 1 {
		return results, nil
	}

	// Use the first TiKV node as baseline
	baselineNode := tikvNodes[0]
	baselineConfig := baselineNode.mergedConfig

	// Compare all other TiKV nodes with the baseline node
	// Note: Deployment-specific parameters have already been filtered in preprocessor
	// This rule only processes parameters that passed the preprocessor filter
	for i := 1; i < len(tikvNodes); i++ {
		node := tikvNodes[i]
		nodeConfig := node.mergedConfig

		// Compare each parameter in the node with the baseline
		for paramName, paramValue := range nodeConfig {
			nodeValue := paramValue.Value

			// Get baseline value
			baselineParamValue, existsInBaseline := baselineConfig[paramName]
			if !existsInBaseline {
				// Parameter exists in this node but not in baseline - report as difference
				results = append(results, CheckResult{
					RuleID:        r.Name(),
					Category:      r.Category(),
					Component:     "tikv",
					ParameterName: paramName,
					ParamType:     "config",
					Severity:      "warning",
					RiskLevel:     RiskLevelMedium,
					Message:       fmt.Sprintf("Parameter %s exists in TiKV node %s but not in baseline node %s", paramName, node.name, baselineNode.name),
					Details:       fmt.Sprintf("Node: %s (instance: %s)\nBaseline Node: %s (instance: %s)\n\nThis parameter is present in node %s but missing in the baseline node.\nCurrent Value: %v", node.name, node.instance, baselineNode.name, baselineNode.instance, node.name, FormatValue(nodeValue)),
					CurrentValue:  nodeValue,
					Suggestions: []string{
						"This parameter exists in this node but not in the baseline node",
						"Review if this parameter should be added to the baseline node or removed from this node",
						"Ensure all TiKV nodes have consistent parameters for scale out",
					},
					Metadata: map[string]interface{}{
						"node_name":         node.name,
						"node_instance":     node.instance,
						"baseline_name":     baselineNode.name,
						"baseline_instance": baselineNode.instance,
						"config_sources":    []string{"last_tikv.toml", "SHOW CONFIG WHERE type='tikv' AND instance='...'"},
					},
				})
				continue
			}

			baselineValue := baselineParamValue.Value

			// For map types, use deep comparison to show only differing fields
			nodeMap := ConvertToMapStringInterface(nodeValue)
			baselineMap := ConvertToMapStringInterface(baselineValue)

			if nodeMap != nil && baselineMap != nil {
				// Both are maps, use deep comparison to show only differing fields
				opts := CompareOptions{
					BasePath: paramName,
				}
				diffs := CompareMapsDeep(nodeValue, baselineValue, opts)

				// Only report if there are differences
				if len(diffs) > 0 {
					// Create a separate CheckResult for each differing field
					for fieldPath, diff := range diffs {
						fieldDetails := FormatValueDiff(diff.Current, diff.Source) // Current (node) vs Source (baseline)

						results = append(results, CheckResult{
							RuleID:        r.Name(),
							Category:      r.Category(),
							Component:     "tikv",
							ParameterName: fmt.Sprintf("%s.%s", paramName, fieldPath),
							ParamType:     "config",
							Severity:      "warning",
							RiskLevel:     RiskLevelMedium,
							Message:       fmt.Sprintf("Parameter %s.%s in TiKV node %s differs from baseline node %s", paramName, fieldPath, node.name, baselineNode.name),
							Details:       fmt.Sprintf("Node: %s (instance: %s)\nBaseline Node: %s (instance: %s)\n%s", node.name, node.instance, baselineNode.name, baselineNode.instance, fieldDetails),
							CurrentValue:  diff.Current,
							SourceDefault: diff.Source, // Baseline value
							Suggestions: []string{
								"This parameter differs between TiKV nodes",
								"Review if this difference is intentional",
								"Ensure all TiKV nodes have consistent parameters for scale out",
							},
							Metadata: map[string]interface{}{
								"node_name":         node.name,
								"node_instance":     node.instance,
								"baseline_name":     baselineNode.name,
								"baseline_instance": baselineNode.instance,
								"config_sources":    []string{"last_tikv.toml", "SHOW CONFIG WHERE type='tikv' AND instance='...'"},
							},
						})
					}
				}
				// Skip reporting the entire map - we only report individual fields
				continue
			} else {
				// For non-map types, use simple comparison
				// For filename-only parameters, compare by filename only (ignore path)
				var differs bool
				if analyzer.IsFilenameOnlyParameter(paramName) {
					differs = !CompareFileNames(nodeValue, baselineValue)
				} else {
					// Use proper value comparison to avoid scientific notation issues
					differs = !CompareValues(nodeValue, baselineValue)
				}

				if differs {
					// Difference found: medium risk (warning)
					details := FormatValueDiff(nodeValue, baselineValue)

					results = append(results, CheckResult{
						RuleID:        r.Name(),
						Category:      r.Category(),
						Component:     "tikv",
						ParameterName: paramName,
						ParamType:     "config",
						Severity:      "warning",
						RiskLevel:     RiskLevelMedium,
						Message:       fmt.Sprintf("Parameter %s in TiKV node %s differs from baseline node %s", paramName, node.name, baselineNode.name),
						Details:       fmt.Sprintf("Node: %s (instance: %s)\nBaseline Node: %s (instance: %s)\n%s", node.name, node.instance, baselineNode.name, baselineNode.instance, details),
						CurrentValue:  nodeValue,
						SourceDefault: baselineValue, // Baseline value
						Suggestions: []string{
							"This parameter differs between TiKV nodes",
							"Review if this difference is intentional",
							"Ensure all TiKV nodes have consistent parameters for scale out",
						},
						Metadata: map[string]interface{}{
							"node_name":         node.name,
							"node_instance":     node.instance,
							"baseline_name":     baselineNode.name,
							"baseline_instance": baselineNode.instance,
							"config_sources":    []string{"last_tikv.toml", "SHOW CONFIG WHERE type='tikv' AND instance='...'"},
						},
					})
				}
			}
		}

		// Also check for parameters that exist in baseline but not in this node
		for paramName, baselineParamValue := range baselineConfig {
			if _, existsInNode := nodeConfig[paramName]; !existsInNode {
				// Parameter exists in baseline but not in this node - report as difference
				baselineValue := baselineParamValue.Value
				results = append(results, CheckResult{
					RuleID:        r.Name(),
					Category:      r.Category(),
					Component:     "tikv",
					ParameterName: paramName,
					ParamType:     "config",
					Severity:      "warning",
					RiskLevel:     RiskLevelMedium,
					Message:       fmt.Sprintf("Parameter %s exists in baseline node %s but not in TiKV node %s", paramName, baselineNode.name, node.name),
					Details:       fmt.Sprintf("Node: %s (instance: %s)\nBaseline Node: %s (instance: %s)\n\nThis parameter is present in the baseline node but missing in node %s.\nBaseline Value: %v", node.name, node.instance, baselineNode.name, baselineNode.instance, node.name, FormatValue(baselineValue)),
					CurrentValue:  nil,
					SourceDefault: baselineValue,
					Suggestions: []string{
						"This parameter exists in the baseline node but not in this node",
						"Review if this parameter should be added to this node or removed from the baseline node",
						"Ensure all TiKV nodes have consistent parameters for scale out",
					},
					Metadata: map[string]interface{}{
						"node_name":         node.name,
						"node_instance":     node.instance,
						"baseline_name":     baselineNode.name,
						"baseline_instance": baselineNode.instance,
						"config_sources":    []string{"last_tikv.toml", "SHOW CONFIG WHERE type='tikv' AND instance='...'"},
					},
				})
			}
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
