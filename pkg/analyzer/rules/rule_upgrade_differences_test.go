package rules

import (
	"context"
	"testing"

	"github.com/pingcap/tidb-upgrade-precheck/pkg/collector"
	"github.com/pingcap/tidb-upgrade-precheck/pkg/types"
	"github.com/stretchr/testify/assert"
)

func TestNewUpgradeDifferencesRule(t *testing.T) {
	rule := NewUpgradeDifferencesRule()
	assert.NotNil(t, rule)
	assert.Equal(t, "UPGRADE_DIFFERENCES", rule.Name())
	assert.Equal(t, "upgrade_difference", rule.Category())
}

func TestUpgradeDifferencesRule_DataRequirements(t *testing.T) {
	rule := NewUpgradeDifferencesRule().(*UpgradeDifferencesRule)
	req := rule.DataRequirements()

	assert.True(t, req.SourceClusterRequirements.NeedConfig)
	assert.True(t, req.SourceClusterRequirements.NeedSystemVariables)
	assert.False(t, req.SourceClusterRequirements.NeedAllTikvNodes)
	assert.Contains(t, req.SourceClusterRequirements.Components, "tidb")
	assert.Contains(t, req.SourceClusterRequirements.Components, "pd")
	assert.Contains(t, req.SourceClusterRequirements.Components, "tikv")
	assert.Contains(t, req.SourceClusterRequirements.Components, "tiflash")

	assert.True(t, req.TargetKBRequirements.NeedConfigDefaults)
	assert.True(t, req.TargetKBRequirements.NeedSystemVariables)
	assert.True(t, req.TargetKBRequirements.NeedUpgradeLogic)
}

func TestUpgradeDifferencesRule_Evaluate_ForcedConfigChange(t *testing.T) {
	rule := NewUpgradeDifferencesRule()
	ctx := context.Background()

	ruleCtx := &RuleContext{
		SourceClusterSnapshot: &collector.ClusterSnapshot{
			Components: map[string]collector.ComponentState{
				"tidb": {
					Type: types.ComponentTiDB,
					Config: types.ConfigDefaults{
						"max-connections": types.ParameterValue{Value: 1000, Type: "int"},
					},
				},
			},
		},
		SourceVersion: "v7.5.0",
		TargetVersion: "v8.5.0",
		SourceDefaults: map[string]map[string]interface{}{
			"tidb": {
				"max-connections": 1000,
			},
		},
		TargetDefaults: map[string]map[string]interface{}{
			"tidb": {
				"max-connections": 2000,
			},
		},
		UpgradeLogic: map[string]interface{}{
			"tidb": map[string]interface{}{
				"changes": []interface{}{
					map[string]interface{}{
						"version":           "150", // Bootstrap version as string
						"bootstrap_version": 150,   // Bootstrap version in range (140 < 150 <= 160)
						"name":              "max-connections",
						"value":             3000,
						"operation":         "UPDATE",
						"severity":          "medium",
					},
				},
			},
		},
	}

	// Mock bootstrap version for source and target
	ruleCtx.SourceBootstrapVersion = 140
	ruleCtx.TargetBootstrapVersion = 160

	results, err := rule.Evaluate(ctx, ruleCtx)

	assert.NoError(t, err)
	assert.NotEmpty(t, results)
	
	// Should detect forced change
	found := false
	for _, result := range results {
		if result.ParameterName == "max-connections" && result.ForcedValue != nil {
			found = true
			assert.Equal(t, "error", result.Severity) // TiDB forced changes are error
			assert.Equal(t, 3000, result.ForcedValue)
			assert.Equal(t, 1000, result.CurrentValue)
			break
		}
	}
	assert.True(t, found, "Should detect forced config change")
}

func TestUpgradeDifferencesRule_Evaluate_ForcedSystemVariableChange(t *testing.T) {
	rule := NewUpgradeDifferencesRule()
	ctx := context.Background()

	ruleCtx := &RuleContext{
		SourceClusterSnapshot: &collector.ClusterSnapshot{
			Components: map[string]collector.ComponentState{
				"tidb": {
					Type: types.ComponentTiDB,
					Variables: types.SystemVariables{
						"tidb_mem_quota_query": types.ParameterValue{Value: 1073741824, Type: "int"},
					},
				},
			},
		},
		SourceVersion: "v7.5.0",
		TargetVersion: "v8.5.0",
		SourceDefaults: map[string]map[string]interface{}{
			"tidb": {
				"sysvar:tidb_mem_quota_query": 1073741824,
			},
		},
		TargetDefaults: map[string]map[string]interface{}{
			"tidb": {
				"sysvar:tidb_mem_quota_query": 2147483648,
			},
		},
		UpgradeLogic: map[string]interface{}{
			"tidb": map[string]interface{}{
				"changes": []interface{}{
					map[string]interface{}{
						"version":           "150", // Bootstrap version as string
						"bootstrap_version": 150,   // Bootstrap version in range (140 < 150 <= 160)
						"name":              "tidb_mem_quota_query", // System variable name (without sysvar: prefix)
						"value":             4294967296,
						"operation":         "SET @@GLOBAL",
						"severity":          "medium",
					},
				},
			},
		},
	}

	ruleCtx.SourceBootstrapVersion = 140
	ruleCtx.TargetBootstrapVersion = 160

	results, err := rule.Evaluate(ctx, ruleCtx)

	assert.NoError(t, err)
	assert.NotEmpty(t, results)
	
	// Should detect forced system variable change
	found := false
	for _, result := range results {
		if result.ParameterName == "tidb_mem_quota_query" && result.ForcedValue != nil {
			found = true
			assert.Equal(t, "error", result.Severity) // Forced system variable changes are critical
			assert.Equal(t, "system_variable", result.ParamType)
			assert.Equal(t, 4294967296, result.ForcedValue)
			break
		}
	}
	assert.True(t, found, "Should detect forced system variable change")
}

func TestUpgradeDifferencesRule_Evaluate_DefaultValueChanged(t *testing.T) {
	rule := NewUpgradeDifferencesRule()
	ctx := context.Background()

	ruleCtx := &RuleContext{
		SourceClusterSnapshot: &collector.ClusterSnapshot{
			Components: map[string]collector.ComponentState{
				"tidb": {
					Type: types.ComponentTiDB,
					Config: types.ConfigDefaults{
						"max-connections": types.ParameterValue{Value: 1000, Type: "int"},
					},
				},
			},
		},
		SourceVersion: "v7.5.0",
		TargetVersion: "v8.5.0",
		SourceDefaults: map[string]map[string]interface{}{
			"tidb": {
				"max-connections": 1000,
			},
		},
		TargetDefaults: map[string]map[string]interface{}{
			"tidb": {
				"max-connections": 2000,
			},
		},
		UpgradeLogic: map[string]interface{}{},
	}

	results, err := rule.Evaluate(ctx, ruleCtx)

	assert.NoError(t, err)
	assert.NotEmpty(t, results)
	
	// Should detect that value will differ from target default
	found := false
	for _, result := range results {
		if result.ParameterName == "max-connections" && result.ForcedValue == nil {
			found = true
			assert.Equal(t, "warning", result.Severity)
			assert.Equal(t, 1000, result.CurrentValue)
			assert.Equal(t, 2000, result.TargetDefault)
			break
		}
	}
	assert.True(t, found, "Should detect default value change")
}

func TestUpgradeDifferencesRule_Evaluate_PDCompatibilityHandling(t *testing.T) {
	rule := NewUpgradeDifferencesRule()
	ctx := context.Background()

	ruleCtx := &RuleContext{
		SourceClusterSnapshot: &collector.ClusterSnapshot{
			Components: map[string]collector.ComponentState{
				"pd": {
					Type: types.ComponentPD,
					Config: types.ConfigDefaults{
						"max-request-size": types.ParameterValue{Value: 100, Type: "int"},
					},
				},
			},
		},
		SourceVersion: "v7.5.0",
		TargetVersion: "v8.5.0",
		SourceDefaults: map[string]map[string]interface{}{
			"pd": {
				"max-request-size": 100,
			},
		},
		TargetDefaults: map[string]map[string]interface{}{
			"pd": {
				"max-request-size": 200,
			},
		},
		UpgradeLogic: map[string]interface{}{},
	}

	results, err := rule.Evaluate(ctx, ruleCtx)

	assert.NoError(t, err)
	assert.NotEmpty(t, results)
	
	// PD should have special handling: current value will be preserved
	found := false
	for _, result := range results {
		if result.ParameterName == "max-request-size" {
			found = true
			assert.Equal(t, "info", result.Severity) // PD compatibility handling uses info
			assert.Contains(t, result.Details, "will be kept")
			assert.Contains(t, result.Details, "PD maintains existing configuration")
			break
		}
	}
	assert.True(t, found, "Should detect PD compatibility handling")
}

func TestUpgradeDifferencesRule_Evaluate_SystemVariableKeepsValue(t *testing.T) {
	rule := NewUpgradeDifferencesRule()
	ctx := context.Background()

	ruleCtx := &RuleContext{
		SourceClusterSnapshot: &collector.ClusterSnapshot{
			Components: map[string]collector.ComponentState{
				"tidb": {
					Type: types.ComponentTiDB,
					Variables: types.SystemVariables{
						"tidb_mem_quota_query": types.ParameterValue{Value: 1073741824, Type: "int"},
					},
				},
			},
		},
		SourceVersion: "v7.5.0",
		TargetVersion: "v8.5.0",
		SourceDefaults: map[string]map[string]interface{}{
			"tidb": {
				"sysvar:tidb_mem_quota_query": 1073741824,
			},
		},
		TargetDefaults: map[string]map[string]interface{}{
			"tidb": {
				"sysvar:tidb_mem_quota_query": 2147483648,
			},
		},
		UpgradeLogic: map[string]interface{}{}, // No forced change
	}

	results, err := rule.Evaluate(ctx, ruleCtx)

	assert.NoError(t, err)
	assert.NotEmpty(t, results)
	
	// TiDB system variables keep old values unless forced
	found := false
	for _, result := range results {
		if result.ParameterName == "tidb_mem_quota_query" && result.ForcedValue == nil {
			found = true
			assert.Equal(t, "info", result.Severity) // Informational: value will be kept
			assert.Contains(t, result.Details, "will be kept")
			assert.Contains(t, result.Details, "TiDB system variables keep old values")
			break
		}
	}
	assert.True(t, found, "Should detect system variable will keep value")
}

func TestUpgradeDifferencesRule_Evaluate_EmptySnapshot(t *testing.T) {
	rule := NewUpgradeDifferencesRule()
	ctx := context.Background()

	ruleCtx := &RuleContext{
		SourceClusterSnapshot: nil,
	}

	results, err := rule.Evaluate(ctx, ruleCtx)

	assert.NoError(t, err)
	assert.Empty(t, results)
}

func TestUpgradeDifferencesRule_Evaluate_BootstrapVersionFiltering(t *testing.T) {
	rule := NewUpgradeDifferencesRule()
	ctx := context.Background()

	ruleCtx := &RuleContext{
		SourceClusterSnapshot: &collector.ClusterSnapshot{
			Components: map[string]collector.ComponentState{
				"tidb": {
					Type: types.ComponentTiDB,
					Config: types.ConfigDefaults{
						"param1": types.ParameterValue{Value: 100, Type: "int"},
						"param2": types.ParameterValue{Value: 200, Type: "int"},
					},
				},
			},
		},
		SourceVersion: "v7.5.0",
		TargetVersion: "v8.5.0",
		SourceBootstrapVersion: 140,
		TargetBootstrapVersion: 160,
		SourceDefaults: map[string]map[string]interface{}{
			"tidb": {
				"param1": 100,
				"param2": 200,
			},
		},
		TargetDefaults: map[string]map[string]interface{}{
			"tidb": {
				"param1": 100,
				"param2": 200,
			},
		},
		UpgradeLogic: map[string]interface{}{
			"tidb": map[string]interface{}{
				"changes": []interface{}{
					// This change is before source bootstrap version (should be filtered out)
					map[string]interface{}{
						"version":           "130",
						"bootstrap_version": 130,
						"name":              "param1",
						"value":             150,
						"operation":         "UPDATE",
						"severity":          "medium",
					},
					// This change is in range (140 < 150 <= 160) (should be included)
					map[string]interface{}{
						"version":           "150",
						"bootstrap_version": 150,
						"name":              "param2",
						"value":             250,
						"operation":         "UPDATE",
						"severity":          "medium",
					},
					// This change is after target bootstrap version (should be filtered out)
					map[string]interface{}{
						"version":           "170",
						"bootstrap_version": 170,
						"name":              "param3",
						"value":             300,
						"operation":         "UPDATE",
						"severity":          "medium",
					},
				},
			},
		},
	}

	results, err := rule.Evaluate(ctx, ruleCtx)

	assert.NoError(t, err)
	
	// Should only detect param2 (bootstrap version 150 is in range)
	param2Found := false
	param1Found := false
	param3Found := false
	
	for _, result := range results {
		if result.ParameterName == "param1" && result.ForcedValue != nil {
			param1Found = true
		}
		if result.ParameterName == "param2" && result.ForcedValue != nil {
			param2Found = true
			assert.Equal(t, 250, result.ForcedValue)
		}
		if result.ParameterName == "param3" && result.ForcedValue != nil {
			param3Found = true
		}
	}
	
	assert.False(t, param1Found, "param1 change should be filtered out (before source bootstrap version)")
	assert.True(t, param2Found, "param2 change should be included (in bootstrap version range)")
	assert.False(t, param3Found, "param3 change should be filtered out (after target bootstrap version)")
}

