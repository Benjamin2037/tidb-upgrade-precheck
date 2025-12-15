package rules

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/pingcap/tidb-upgrade-precheck/pkg/collector"
	"github.com/pingcap/tidb-upgrade-precheck/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewHighRiskParamsRule(t *testing.T) {
	rule, err := NewHighRiskParamsRule("")
	assert.NoError(t, err)
	assert.NotNil(t, rule)
	assert.Equal(t, "HIGH_RISK_PARAMS", rule.Name())
	assert.Equal(t, "high_risk", rule.Category())
}

func TestNewHighRiskParamsRule_WithConfig(t *testing.T) {
	// Create a temporary config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "high_risk_params.json")

	config := HighRiskParamsConfig{
		TiDB: struct {
			Config         map[string]HighRiskParamConfig `json:"config,omitempty"`
			SystemVariables map[string]HighRiskParamConfig `json:"system_variables,omitempty"`
		}{
			Config: map[string]HighRiskParamConfig{
				"max-connections": {
					Severity:      "error",
					Description:   "Max connections is critical",
					CheckModified: true,
				},
			},
		},
	}

	data, err := json.Marshal(config)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(configPath, data, 0644))

	rule, err := NewHighRiskParamsRule(configPath)
	assert.NoError(t, err)
	assert.NotNil(t, rule)

	hrRule := rule.(*HighRiskParamsRule)
	assert.NotNil(t, hrRule.config)
	assert.NotEmpty(t, hrRule.config.TiDB.Config)
}

func TestNewHighRiskParamsRule_InvalidConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "invalid.json")

	// Write invalid JSON
	require.NoError(t, os.WriteFile(configPath, []byte("invalid json"), 0644))

	rule, err := NewHighRiskParamsRule(configPath)
	assert.Error(t, err)
	assert.Nil(t, rule)
}

func TestHighRiskParamsRule_DataRequirements(t *testing.T) {
	tests := []struct {
		name           string
		config         *HighRiskParamsConfig
		wantComponents []string
		wantSystemVars bool
	}{
		{
			name: "TiDB config only",
			config: &HighRiskParamsConfig{
				TiDB: struct {
					Config         map[string]HighRiskParamConfig `json:"config,omitempty"`
					SystemVariables map[string]HighRiskParamConfig `json:"system_variables,omitempty"`
				}{
					Config: map[string]HighRiskParamConfig{
						"param1": {Severity: "warning"},
					},
				},
			},
			wantComponents: []string{"tidb"},
			wantSystemVars: false,
		},
		{
			name: "TiDB system variables",
			config: &HighRiskParamsConfig{
				TiDB: struct {
					Config         map[string]HighRiskParamConfig `json:"config,omitempty"`
					SystemVariables map[string]HighRiskParamConfig `json:"system_variables,omitempty"`
				}{
					SystemVariables: map[string]HighRiskParamConfig{
						"var1": {Severity: "error"},
					},
				},
			},
			wantComponents: []string{"tidb"},
			wantSystemVars: true,
		},
		{
			name: "multiple components",
			config: &HighRiskParamsConfig{
				TiDB: struct {
					Config         map[string]HighRiskParamConfig `json:"config,omitempty"`
					SystemVariables map[string]HighRiskParamConfig `json:"system_variables,omitempty"`
				}{
					Config: map[string]HighRiskParamConfig{
						"param1": {Severity: "warning"},
					},
				},
				PD: struct {
					Config map[string]HighRiskParamConfig `json:"config,omitempty"`
				}{
					Config: map[string]HighRiskParamConfig{
						"param2": {Severity: "warning"},
					},
				},
			},
			wantComponents: []string{"tidb", "pd"},
			wantSystemVars: false,
		},
		{
			name:           "empty config",
			config:         &HighRiskParamsConfig{},
			wantComponents: []string{},
			wantSystemVars: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule := &HighRiskParamsRule{
				BaseRule: NewBaseRule("HIGH_RISK_PARAMS", "Test", "high_risk"),
				config:   tt.config,
			}

			req := rule.DataRequirements()

			assert.Equal(t, len(tt.wantComponents), len(req.SourceClusterRequirements.Components))
			for _, comp := range tt.wantComponents {
				assert.Contains(t, req.SourceClusterRequirements.Components, comp)
			}
			assert.Equal(t, tt.wantSystemVars, req.SourceClusterRequirements.NeedSystemVariables)
			assert.True(t, req.SourceKBRequirements.NeedConfigDefaults)
		})
	}
}

func TestHighRiskParamsRule_Evaluate_ModifiedConfig(t *testing.T) {
	rule := &HighRiskParamsRule{
		BaseRule: NewBaseRule("HIGH_RISK_PARAMS", "Test", "high_risk"),
		config: &HighRiskParamsConfig{
			TiDB: struct {
				Config         map[string]HighRiskParamConfig `json:"config,omitempty"`
				SystemVariables map[string]HighRiskParamConfig `json:"system_variables,omitempty"`
			}{
				Config: map[string]HighRiskParamConfig{
					"max-connections": {
						Severity:      "error",
						Description:   "Max connections is critical",
						CheckModified: true,
					},
				},
			},
		},
	}

	ctx := context.Background()

	ruleCtx := &RuleContext{
		SourceClusterSnapshot: &collector.ClusterSnapshot{
			Components: map[string]collector.ComponentState{
				"tidb": {
					Type: types.ComponentTiDB,
					Config: types.ConfigDefaults{
						"max-connections": types.ParameterValue{Value: 2000, Type: "int"},
					},
				},
			},
		},
		SourceVersion: "v7.5.0",
		SourceDefaults: map[string]map[string]interface{}{
			"tidb": {
				"max-connections": 1000, // Default is 1000, current is 2000 (modified)
			},
		},
	}

	results, err := rule.Evaluate(ctx, ruleCtx)

	assert.NoError(t, err)
	assert.NotEmpty(t, results)

	found := false
	for _, result := range results {
		if result.ParameterName == "max-connections" {
			found = true
			assert.Equal(t, "error", result.Severity)
			assert.Equal(t, 2000, result.CurrentValue)
			assert.Contains(t, result.Details, "Max connections is critical")
			break
		}
	}
	assert.True(t, found, "Should detect modified high-risk parameter")
}

func TestHighRiskParamsRule_Evaluate_UnmodifiedConfig(t *testing.T) {
	rule := &HighRiskParamsRule{
		BaseRule: NewBaseRule("HIGH_RISK_PARAMS", "Test", "high_risk"),
		config: &HighRiskParamsConfig{
			TiDB: struct {
				Config         map[string]HighRiskParamConfig `json:"config,omitempty"`
				SystemVariables map[string]HighRiskParamConfig `json:"system_variables,omitempty"`
			}{
				Config: map[string]HighRiskParamConfig{
					"max-connections": {
						Severity:      "error",
						CheckModified: true, // Only check if modified
					},
				},
			},
		},
	}

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
		SourceDefaults: map[string]map[string]interface{}{
			"tidb": {
				"max-connections": 1000, // Same as default, not modified
			},
		},
	}

	results, err := rule.Evaluate(ctx, ruleCtx)

	assert.NoError(t, err)
	// Should not report if value matches default and CheckModified is true
	assert.Empty(t, results)
}

func TestHighRiskParamsRule_Evaluate_AllowedValues(t *testing.T) {
	rule := &HighRiskParamsRule{
		BaseRule: NewBaseRule("HIGH_RISK_PARAMS", "Test", "high_risk"),
		config: &HighRiskParamsConfig{
			TiDB: struct {
				Config         map[string]HighRiskParamConfig `json:"config,omitempty"`
				SystemVariables map[string]HighRiskParamConfig `json:"system_variables,omitempty"`
			}{
				Config: map[string]HighRiskParamConfig{
					"max-connections": {
						Severity:      "error",
						CheckModified: false, // Check regardless of modification
						AllowedValues: []interface{}{1000, 2000, 3000},
					},
				},
			},
		},
	}

	ctx := context.Background()

	ruleCtx := &RuleContext{
		SourceClusterSnapshot: &collector.ClusterSnapshot{
			Components: map[string]collector.ComponentState{
				"tidb": {
					Type: types.ComponentTiDB,
					Config: types.ConfigDefaults{
						"max-connections": types.ParameterValue{Value: 2000, Type: "int"}, // Allowed value
					},
				},
			},
		},
		SourceVersion: "v7.5.0",
		SourceDefaults: map[string]map[string]interface{}{
			"tidb": {
				"max-connections": 1000,
			},
		},
	}

	results, err := rule.Evaluate(ctx, ruleCtx)

	assert.NoError(t, err)
	// Value is in allowed list, should not report
	assert.Empty(t, results)
}

func TestHighRiskParamsRule_Evaluate_NotAllowedValue(t *testing.T) {
	rule := &HighRiskParamsRule{
		BaseRule: NewBaseRule("HIGH_RISK_PARAMS", "Test", "high_risk"),
		config: &HighRiskParamsConfig{
			TiDB: struct {
				Config         map[string]HighRiskParamConfig `json:"config,omitempty"`
				SystemVariables map[string]HighRiskParamConfig `json:"system_variables,omitempty"`
			}{
				Config: map[string]HighRiskParamConfig{
					"max-connections": {
						Severity:      "error",
						CheckModified: false,
						AllowedValues: []interface{}{1000, 2000, 3000},
					},
				},
			},
		},
	}

	ctx := context.Background()

	ruleCtx := &RuleContext{
		SourceClusterSnapshot: &collector.ClusterSnapshot{
			Components: map[string]collector.ComponentState{
				"tidb": {
					Type: types.ComponentTiDB,
					Config: types.ConfigDefaults{
						"max-connections": types.ParameterValue{Value: 5000, Type: "int"}, // Not in allowed list
					},
				},
			},
		},
		SourceVersion: "v7.5.0",
		SourceDefaults: map[string]map[string]interface{}{
			"tidb": {
				"max-connections": 1000,
			},
		},
	}

	results, err := rule.Evaluate(ctx, ruleCtx)

	assert.NoError(t, err)
	assert.NotEmpty(t, results)

	found := false
	for _, result := range results {
		if result.ParameterName == "max-connections" {
			found = true
			assert.Equal(t, "error", result.Severity)
			assert.Equal(t, 5000, result.CurrentValue)
			assert.Contains(t, result.Details, "Allowed values")
			break
		}
	}
	assert.True(t, found, "Should detect value not in allowed list")
}

func TestHighRiskParamsRule_Evaluate_VersionRange(t *testing.T) {
	rule := &HighRiskParamsRule{
		BaseRule: NewBaseRule("HIGH_RISK_PARAMS", "Test", "high_risk"),
		config: &HighRiskParamsConfig{
			TiDB: struct {
				Config         map[string]HighRiskParamConfig `json:"config,omitempty"`
				SystemVariables map[string]HighRiskParamConfig `json:"system_variables,omitempty"`
			}{
				Config: map[string]HighRiskParamConfig{
					"param1": {
						Severity:      "error",
						CheckModified: false,
						FromVersion:   "v7.5.0",
						ToVersion:     "v8.5.0",
					},
				},
			},
		},
	}

	ctx := context.Background()

	tests := []struct {
		name          string
		sourceVersion string
		wantResults   bool
	}{
		{
			name:          "version in range",
			sourceVersion: "v7.5.0",
			wantResults:   true,
		},
		{
			name:          "version before range",
			sourceVersion: "v6.5.0",
			wantResults:   false,
		},
		{
			name:          "version after range",
			sourceVersion: "v8.5.0",
			wantResults:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ruleCtx := &RuleContext{
				SourceClusterSnapshot: &collector.ClusterSnapshot{
					Components: map[string]collector.ComponentState{
						"tidb": {
							Type: types.ComponentTiDB,
							Config: types.ConfigDefaults{
								"param1": types.ParameterValue{Value: 100, Type: "int"},
							},
						},
					},
				},
				SourceVersion: tt.sourceVersion,
				// Don't set TargetVersion - test checks if sourceVersion is in range
				SourceDefaults: map[string]map[string]interface{}{
					"tidb": {
						"param1": 100,
					},
				},
			}

			results, err := rule.Evaluate(ctx, ruleCtx)

			assert.NoError(t, err)
			if tt.wantResults {
				assert.NotEmpty(t, results)
			} else {
				assert.Empty(t, results)
			}
		})
	}
}

func TestHighRiskParamsRule_Evaluate_SystemVariable(t *testing.T) {
	rule := &HighRiskParamsRule{
		BaseRule: NewBaseRule("HIGH_RISK_PARAMS", "Test", "high_risk"),
		config: &HighRiskParamsConfig{
			TiDB: struct {
				Config         map[string]HighRiskParamConfig `json:"config,omitempty"`
				SystemVariables map[string]HighRiskParamConfig `json:"system_variables,omitempty"`
			}{
				SystemVariables: map[string]HighRiskParamConfig{
					"tidb_enable_async_commit": {
						Severity:      "high",
						Description:   "Async commit may cause data inconsistency",
						CheckModified: true,
					},
				},
			},
		},
	}

	ctx := context.Background()

	ruleCtx := &RuleContext{
		SourceClusterSnapshot: &collector.ClusterSnapshot{
			Components: map[string]collector.ComponentState{
				"tidb": {
					Type: types.ComponentTiDB,
					Variables: types.SystemVariables{
						"tidb_enable_async_commit": types.ParameterValue{Value: "ON", Type: "string"},
					},
				},
			},
		},
		SourceVersion: "v7.5.0",
		SourceDefaults: map[string]map[string]interface{}{
			"tidb": {
				"sysvar:tidb_enable_async_commit": "OFF", // Default is OFF, current is ON (modified)
			},
		},
	}

	results, err := rule.Evaluate(ctx, ruleCtx)

	assert.NoError(t, err)
	assert.NotEmpty(t, results)

	found := false
	for _, result := range results {
		if result.ParameterName == "tidb_enable_async_commit" {
			found = true
			assert.Equal(t, "high", result.Severity)
			assert.Equal(t, "system_variable", result.ParamType)
			assert.Equal(t, "ON", result.CurrentValue)
			break
		}
	}
	assert.True(t, found, "Should detect modified high-risk system variable")
}

func TestHighRiskParamsRule_Evaluate_EmptySnapshot(t *testing.T) {
	rule := &HighRiskParamsRule{
		BaseRule: NewBaseRule("HIGH_RISK_PARAMS", "Test", "high_risk"),
		config: &HighRiskParamsConfig{
			TiDB: struct {
				Config         map[string]HighRiskParamConfig `json:"config,omitempty"`
				SystemVariables map[string]HighRiskParamConfig `json:"system_variables,omitempty"`
			}{
				Config: map[string]HighRiskParamConfig{
					"param1": {Severity: "error"},
				},
			},
		},
	}

	ctx := context.Background()

	ruleCtx := &RuleContext{
		SourceClusterSnapshot: nil,
	}

	results, err := rule.Evaluate(ctx, ruleCtx)

	assert.NoError(t, err)
	assert.Empty(t, results)
}

// TestHighRiskParamsRule_VersionRange tests version range filtering through Evaluate
// Since isVersionApplicable is a private method, we test it indirectly through Evaluate
func TestHighRiskParamsRule_VersionRange(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name          string
		sourceVersion string
		fromVersion   string
		toVersion     string
		wantResults   bool
	}{
		{
			name:          "no version range",
			sourceVersion: "v7.5.0",
			fromVersion:   "",
			toVersion:     "",
			wantResults:   true,
		},
		{
			name:          "in range",
			sourceVersion: "v7.5.0",
			fromVersion:   "v7.5.0",
			toVersion:     "v8.5.0",
			wantResults:   true,
		},
		{
			name:          "before fromVersion",
			sourceVersion: "v6.5.0",
			fromVersion:   "v7.5.0",
			toVersion:     "",
			wantResults:   false,
		},
		{
			name:          "after toVersion",
			sourceVersion: "v8.5.0",
			fromVersion:   "v7.5.0",
			toVersion:     "v8.5.0",
			wantResults:   false,
		},
		{
			name:          "only toVersion",
			sourceVersion: "v7.5.0",
			fromVersion:   "",
			toVersion:     "v8.5.0",
			wantResults:   true,
		},
		{
			name:          "only toVersion, after",
			sourceVersion: "v8.5.0",
			fromVersion:   "",
			toVersion:     "v8.5.0",
			wantResults:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule := &HighRiskParamsRule{
				BaseRule: NewBaseRule("HIGH_RISK_PARAMS", "Test", "high_risk"),
				config: &HighRiskParamsConfig{
					TiDB: struct {
						Config         map[string]HighRiskParamConfig `json:"config,omitempty"`
						SystemVariables map[string]HighRiskParamConfig `json:"system_variables,omitempty"`
					}{
						Config: map[string]HighRiskParamConfig{
							"param1": {
								Severity:      "error",
								CheckModified: false,
								FromVersion:   tt.fromVersion,
								ToVersion:     tt.toVersion,
							},
						},
					},
				},
			}

			ruleCtx := &RuleContext{
				SourceClusterSnapshot: &collector.ClusterSnapshot{
					Components: map[string]collector.ComponentState{
						"tidb": {
							Type: types.ComponentTiDB,
							Config: types.ConfigDefaults{
								"param1": types.ParameterValue{Value: 100, Type: "int"},
							},
						},
					},
				},
				SourceVersion: tt.sourceVersion,
				// Don't set TargetVersion - test checks if sourceVersion is in range
				SourceDefaults: map[string]map[string]interface{}{
					"tidb": {
						"param1": 100,
					},
				},
			}

			results, err := rule.Evaluate(ctx, ruleCtx)
			assert.NoError(t, err)
			if tt.wantResults {
				assert.NotEmpty(t, results)
			} else {
				assert.Empty(t, results)
			}
		})
	}
}

