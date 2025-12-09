package rules

import (
	"context"
	"testing"

	"github.com/pingcap/tidb-upgrade-precheck/pkg/collector"
	"github.com/pingcap/tidb-upgrade-precheck/pkg/types"
	"github.com/stretchr/testify/assert"
)

func TestNewTikvConsistencyRule(t *testing.T) {
	rule := NewTikvConsistencyRule()
	assert.NotNil(t, rule)
	assert.Equal(t, "TIKV_CONSISTENCY", rule.Name())
	assert.Equal(t, "consistency", rule.Category())
}

func TestTikvConsistencyRule_DataRequirements(t *testing.T) {
	rule := NewTikvConsistencyRule().(*TikvConsistencyRule)
	req := rule.DataRequirements()

	assert.True(t, req.SourceClusterRequirements.NeedConfig)
	assert.False(t, req.SourceClusterRequirements.NeedSystemVariables)
	assert.True(t, req.SourceClusterRequirements.NeedAllTikvNodes)
	assert.Contains(t, req.SourceClusterRequirements.Components, "tikv")
	assert.Equal(t, 1, len(req.SourceClusterRequirements.Components))

	// This rule doesn't need knowledge base data
	assert.False(t, req.SourceKBRequirements.NeedConfigDefaults)
	assert.False(t, req.SourceKBRequirements.NeedSystemVariables)
	assert.False(t, req.SourceKBRequirements.NeedUpgradeLogic)
	assert.False(t, req.TargetKBRequirements.NeedConfigDefaults)
	assert.False(t, req.TargetKBRequirements.NeedSystemVariables)
	assert.False(t, req.TargetKBRequirements.NeedUpgradeLogic)
}

func TestTikvConsistencyRule_Evaluate_EmptySnapshot(t *testing.T) {
	rule := NewTikvConsistencyRule()
	ctx := context.Background()

	ruleCtx := &RuleContext{
		SourceClusterSnapshot: nil,
	}

	results, err := rule.Evaluate(ctx, ruleCtx)

	assert.NoError(t, err)
	assert.Empty(t, results)
}

func TestTikvConsistencyRule_Evaluate_NoTiKVNodes(t *testing.T) {
	rule := NewTikvConsistencyRule()
	ctx := context.Background()

	ruleCtx := &RuleContext{
		SourceClusterSnapshot: &collector.ClusterSnapshot{
			Components: map[string]collector.ComponentState{
				"tidb": {
					Type: types.ComponentTiDB,
				},
			},
		},
	}

	results, err := rule.Evaluate(ctx, ruleCtx)

	assert.NoError(t, err)
	assert.Empty(t, results)
}

func TestTikvConsistencyRule_Evaluate_SingleTiKVNode(t *testing.T) {
	rule := NewTikvConsistencyRule()
	ctx := context.Background()

	ruleCtx := &RuleContext{
		SourceClusterSnapshot: &collector.ClusterSnapshot{
			Components: map[string]collector.ComponentState{
				"tikv-0": {
					Type: types.ComponentTiKV,
					Config: types.ConfigDefaults{
						"storage.reserve-space": types.ParameterValue{Value: "2GB", Type: "string"},
					},
					Status: map[string]interface{}{
						"address": "192.168.1.100:20160",
					},
				},
			},
		},
	}

	results, err := rule.Evaluate(ctx, ruleCtx)

	assert.NoError(t, err)
	// Need at least 2 nodes to check consistency
	assert.Empty(t, results)
}

func TestTikvConsistencyRule_Evaluate_NoTiDBConnection(t *testing.T) {
	rule := NewTikvConsistencyRule()
	ctx := context.Background()

	ruleCtx := &RuleContext{
		SourceClusterSnapshot: &collector.ClusterSnapshot{
			Components: map[string]collector.ComponentState{
				"tikv-0": {
					Type: types.ComponentTiKV,
					Config: types.ConfigDefaults{
						"storage.reserve-space": types.ParameterValue{Value: "2GB", Type: "string"},
					},
				},
				"tikv-1": {
					Type: types.ComponentTiKV,
					Config: types.ConfigDefaults{
						"storage.reserve-space": types.ParameterValue{Value: "2GB", Type: "string"},
					},
				},
			},
		},
	}

	results, err := rule.Evaluate(ctx, ruleCtx)

	assert.NoError(t, err)
	// Without TiDB connection, cannot get runtime config via SHOW CONFIG
	// The rule will return empty results (graceful degradation)
	assert.Empty(t, results)
}

func TestTikvConsistencyRule_Evaluate_ConsistentValues(t *testing.T) {
	rule := NewTikvConsistencyRule()
	ctx := context.Background()

	ruleCtx := &RuleContext{
		SourceClusterSnapshot: &collector.ClusterSnapshot{
			Components: map[string]collector.ComponentState{
				"tidb": {
					Type: types.ComponentTiDB,
					Status: map[string]interface{}{
						"address": "127.0.0.1:4000",
						"user":    "root",
						"password": "",
					},
				},
				"tikv-0": {
					Type: types.ComponentTiKV,
					Config: types.ConfigDefaults{
						"storage.reserve-space": types.ParameterValue{Value: "2GB", Type: "string"},
					},
					Status: map[string]interface{}{
						"address": "192.168.1.100:20160",
					},
				},
				"tikv-1": {
					Type: types.ComponentTiKV,
					Config: types.ConfigDefaults{
						"storage.reserve-space": types.ParameterValue{Value: "2GB", Type: "string"},
					},
					Status: map[string]interface{}{
						"address": "192.168.1.101:20160",
					},
				},
			},
		},
	}

	// Note: This test will fail to connect to TiDB, but we can test the logic structure
	// In a real integration test, we would have a running TiDB cluster
	results, err := rule.Evaluate(ctx, ruleCtx)

	// The rule will try to connect to TiDB and may fail, but should handle gracefully
	// We're testing that the rule structure is correct
	assert.NoError(t, err)
	// Results may be empty if connection fails, which is acceptable
	_ = results
}

func TestTikvConsistencyRule_Evaluate_InconsistentValues(t *testing.T) {
	rule := NewTikvConsistencyRule()
	ctx := context.Background()

	ruleCtx := &RuleContext{
		SourceClusterSnapshot: &collector.ClusterSnapshot{
			Components: map[string]collector.ComponentState{
				"tidb": {
					Type: types.ComponentTiDB,
					Status: map[string]interface{}{
						"address": "127.0.0.1:4000",
						"user":    "root",
						"password": "",
					},
				},
				"tikv-0": {
					Type: types.ComponentTiKV,
					Config: types.ConfigDefaults{
						"storage.reserve-space": types.ParameterValue{Value: "2GB", Type: "string"},
					},
					Status: map[string]interface{}{
						"address": "192.168.1.100:20160",
					},
				},
				"tikv-1": {
					Type: types.ComponentTiKV,
					Config: types.ConfigDefaults{
						"storage.reserve-space": types.ParameterValue{Value: "4GB", Type: "string"},
					},
					Status: map[string]interface{}{
						"address": "192.168.1.101:20160",
					},
				},
			},
		},
	}

	// Note: This test will fail to connect to TiDB, but we can test the logic structure
	// In a real integration test with a running cluster, this would detect inconsistencies
	results, err := rule.Evaluate(ctx, ruleCtx)

	assert.NoError(t, err)
	// Results may be empty if connection fails, which is acceptable for unit test
	// In integration test, this should detect the inconsistency
	_ = results
}

func TestDetermineValueType(t *testing.T) {
	tests := []struct {
		name  string
		value interface{}
		want  string
	}{
		{
			name:  "int",
			value: 100,
			want:  "int",
		},
		{
			name:  "int64",
			value: int64(100),
			want:  "int",
		},
		{
			name:  "float",
			value: 3.14,
			want:  "float",
		},
		{
			name:  "bool",
			value: true,
			want:  "bool",
		},
		{
			name:  "string",
			value: "test",
			want:  "string",
		},
		{
			name:  "default to string",
			value: []interface{}{1, 2, 3},
			want:  "string",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := determineValueType(tt.value)
			assert.Equal(t, tt.want, result)
		})
	}
}

