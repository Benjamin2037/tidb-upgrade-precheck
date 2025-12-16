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
	tidbCollector "github.com/pingcap/tidb-upgrade-precheck/pkg/collector/tidb"
	defaultsTypes "github.com/pingcap/tidb-upgrade-precheck/pkg/types"
)

// TikvConsistencyRule compares TiKV node parameters with source version knowledge base
// Rule 2.3: Compare all TiKV node parameters with source version defaults
// Reports differences as medium risk (warning)
type TikvConsistencyRule struct {
	*BaseRule
}

// NewTikvConsistencyRule creates a new TiKV consistency rule
func NewTikvConsistencyRule() Rule {
	return &TikvConsistencyRule{
		BaseRule: NewBaseRule(
			"TIKV_CONSISTENCY",
			"Compare TiKV node parameters with source version knowledge base defaults",
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
			Components:          []string{"tikv"},
			NeedConfigDefaults:  false, // This rule doesn't need knowledge base data
			NeedSystemVariables: false,
			NeedUpgradeLogic:    false,
		},
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
// Logic:
// 1. For each TiKV node, collect parameters (last_tikv.toml + SHOW CONFIG, merged with runtime priority)
// 2. Compare with source version knowledge base defaults
// 3. Report differences as medium risk (warning)
// 4. Each node-parameter combination is one entry
func (r *TikvConsistencyRule) Evaluate(ctx context.Context, ruleCtx *RuleContext) ([]CheckResult, error) {
	var results []CheckResult

	if ruleCtx.SourceClusterSnapshot == nil {
		return results, nil
	}

	// Get source version defaults for TiKV
	sourceDefaults := ruleCtx.SourceDefaults["tikv"]
	if sourceDefaults == nil {
		// No source defaults available, skip
		return results, nil
	}

	// Get target version defaults for TiKV (optional, for reference)
	targetDefaults := ruleCtx.TargetDefaults["tikv"]

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
			instance := address

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

	if len(tikvNodes) == 0 {
		return results, nil
	}

	// Connect to TiDB to get runtime configs via SHOW CONFIG (if available)
	var db *sql.DB
	var collector tidbCollector.TiDBCollector // TiDBCollector is an interface, not a pointer
	if tidbAddr != "" {
		var err error
		db, err = sql.Open("mysql", fmt.Sprintf("%s:%s@tcp(%s)/", tidbUser, tidbPassword, tidbAddr))
		if err == nil {
			defer db.Close()
			db.SetConnMaxLifetime(10 * time.Second)
			collector = tidbCollector.NewTiDBCollector() // Returns TiDBCollector interface
		}
	}

	// Process each TiKV node
	for _, node := range tikvNodes {
		// Step 1: Start with user-set values from last_tikv.toml
		mergedConfig := make(defaultsTypes.ConfigDefaults)
		for k, v := range node.userSetConfig {
			mergedConfig[k] = v
		}

		// Step 2: Get runtime values via SHOW CONFIG WHERE type='tikv' AND instance='...' (if available)
		if db != nil && collector != nil {
			runtimeConfig, err := collector.GetConfigByTypeAndInstance(db, "tikv", node.instance)
			if err == nil {
				// Step 3: Merge with priority: runtime values > user-set values
				for k, v := range runtimeConfig {
					// Convert to ParameterValue format
					mergedConfig[k] = defaultsTypes.ParameterValue{
						Value: v,
						Type:  determineValueType(v),
					}
				}
			}
		}

		// Step 4: Compare merged config with source version defaults
		for paramName, paramValue := range mergedConfig {
			currentValue := paramValue.Value

			// Get source default
			sourceDefaultValue, existsInSource := sourceDefaults[paramName]
			if !existsInSource {
				// Parameter not in source version KB, skip (handled by other rules)
				continue
			}

			sourceDefault := extractValueFromDefault(sourceDefaultValue)
			if sourceDefault == nil {
				continue
			}

			// Get target default (if available)
			var targetDefault interface{}
			if targetDefaults != nil {
				if targetDefaultValue, existsInTarget := targetDefaults[paramName]; existsInTarget {
					targetDefault = extractValueFromDefault(targetDefaultValue)
				}
			}

			// For map types, use deep comparison to show only differing fields
			if IsMapType(currentValue) && IsMapType(sourceDefault) {
				opts := CompareOptions{
					IgnoredParams: nil, // Don't ignore any fields for consistency checks
					BasePath:      paramName,
				}
				diffs := CompareMapsDeep(currentValue, sourceDefault, opts)

				// Only report if there are differences
				if len(diffs) > 0 {
					// Create a separate CheckResult for each differing field
					for fieldPath, diff := range diffs {
						fieldDetails := FormatValueDiff(diff.Current, diff.Source) // Current vs Source
						if targetDefault != nil && IsMapType(targetDefault) {
							// Try to get target value for this field
							targetMap := ConvertToMapStringInterface(targetDefault)
							if targetMap != nil {
								fieldKeys := strings.Split(fieldPath, ".")
								targetFieldValue := getNestedMapValue(targetMap, fieldKeys)
								if targetFieldValue != nil {
									fieldDetails += fmt.Sprintf("\nTarget Default: %v", FormatValue(targetFieldValue))
								}
							}
						}

						results = append(results, CheckResult{
							RuleID:        r.Name(),
							Category:      r.Category(),
							Component:     "tikv",
							ParameterName: fmt.Sprintf("%s.%s", paramName, fieldPath),
							ParamType:     "config",
							Severity:      "warning",
							RiskLevel:     RiskLevelMedium,
							Message:       fmt.Sprintf("Parameter %s.%s in TiKV node %s differs from source version default", paramName, fieldPath, node.name),
							Details:       fmt.Sprintf("Node: %s (instance: %s)\n%s", node.name, node.instance, fieldDetails),
							CurrentValue:  diff.Current,
							SourceDefault: diff.Source,
							TargetDefault: getNestedMapValue(ConvertToMapStringInterface(targetDefault), strings.Split(fieldPath, ".")),
							Suggestions: []string{
								"This parameter differs from the source version default",
								"Review if this difference is intentional",
								"Ensure the value is compatible with target version",
							},
							Metadata: map[string]interface{}{
								"node_name":      node.name,
								"node_instance":  node.instance,
								"config_sources": []string{"last_tikv.toml", "SHOW CONFIG WHERE type='tikv' AND instance='...'"},
							},
						})
					}
				}
			} else {
				// For non-map types, use simple comparison
				if fmt.Sprintf("%v", currentValue) != fmt.Sprintf("%v", sourceDefault) {
					// Difference found: medium risk (warning)
					details := FormatValueDiff(currentValue, sourceDefault)
					if targetDefault != nil {
						details += fmt.Sprintf("\nTarget Default: %v", FormatValue(targetDefault))
					}

					results = append(results, CheckResult{
						RuleID:        r.Name(),
						Category:      r.Category(),
						Component:     "tikv",
						ParameterName: paramName,
						ParamType:     "config",
						Severity:      "warning",
						RiskLevel:     RiskLevelMedium,
						Message:       fmt.Sprintf("Parameter %s in TiKV node %s differs from source version default", paramName, node.name),
						Details:       fmt.Sprintf("Node: %s (instance: %s)\n%s", node.name, node.instance, details),
						CurrentValue:  currentValue,
						SourceDefault: sourceDefault,
						TargetDefault: targetDefault,
						Suggestions: []string{
							"This parameter differs from the source version default",
							"Review if this difference is intentional",
							"Ensure the value is compatible with target version",
						},
						Metadata: map[string]interface{}{
							"node_name":      node.name,
							"node_instance":  node.instance,
							"config_sources": []string{"last_tikv.toml", "SHOW CONFIG WHERE type='tikv' AND instance='...'"},
						},
					})
				}
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

// getNestedMapValue gets a value from a nested map using a path (e.g., ["backup", "num-threads"])
func getNestedMapValue(m map[string]interface{}, path []string) interface{} {
	if m == nil || len(path) == 0 {
		return nil
	}

	current := m
	for i, key := range path {
		if i == len(path)-1 {
			// Last key, return the value
			return current[key]
		}
		// Not the last key, go deeper
		if nextMap, ok := current[key].(map[string]interface{}); ok {
			current = nextMap
		} else {
			return nil
		}
	}
	return nil
}
