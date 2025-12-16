package rules

import (
	"context"
	"testing"

	"github.com/pingcap/tidb-upgrade-precheck/pkg/collector"
	"github.com/pingcap/tidb-upgrade-precheck/pkg/types"
	"github.com/stretchr/testify/assert"
)

func TestNewUserModifiedParamsRule(t *testing.T) {
	rule := NewUserModifiedParamsRule()
	assert.NotNil(t, rule)
	assert.Equal(t, "USER_MODIFIED_PARAMS", rule.Name())
}

func TestUserModifiedParamsRule_DataRequirements(t *testing.T) {
	rule := NewUserModifiedParamsRule().(*UserModifiedParamsRule)
	req := rule.DataRequirements()

	assert.True(t, req.SourceClusterRequirements.NeedConfig)
	assert.True(t, req.SourceClusterRequirements.NeedSystemVariables)
	assert.False(t, req.SourceClusterRequirements.NeedAllTikvNodes)
	assert.Contains(t, req.SourceClusterRequirements.Components, "tidb")
	assert.Contains(t, req.SourceClusterRequirements.Components, "pd")
	assert.Contains(t, req.SourceClusterRequirements.Components, "tikv")
	assert.Contains(t, req.SourceClusterRequirements.Components, "tiflash")

	assert.True(t, req.SourceKBRequirements.NeedConfigDefaults)
	assert.True(t, req.SourceKBRequirements.NeedSystemVariables)
	assert.False(t, req.SourceKBRequirements.NeedUpgradeLogic)
}

func TestUserModifiedParamsRule_Evaluate(t *testing.T) {
	tests := []struct {
		name     string
		ruleCtx  *RuleContext
		wantErr  bool
		wantLen  int
	}{
		{
			name: "nil snapshot",
			ruleCtx: &RuleContext{
				SourceClusterSnapshot: nil,
			},
			wantErr: false,
			wantLen: 0,
		},
		{
			name: "empty snapshot",
			ruleCtx: &RuleContext{
				SourceClusterSnapshot: &collector.ClusterSnapshot{
					Components: make(map[string]collector.ComponentState),
				},
				SourceDefaults: make(map[string]map[string]interface{}),
			},
			wantErr: false,
			wantLen: 0,
		},
		{
			name: "modified config parameter",
			ruleCtx: &RuleContext{
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
				SourceDefaults: map[string]map[string]interface{}{
					"tidb": {
						"max-connections": 1000,
					},
				},
			},
			wantErr: false,
			wantLen: 1,
		},
		{
			name: "unmodified config parameter",
			ruleCtx: &RuleContext{
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
				SourceDefaults: map[string]map[string]interface{}{
					"tidb": {
						"max-connections": 1000,
					},
				},
			},
			wantErr: false,
			wantLen: 0,
		},
		{
			name: "modified system variable",
			ruleCtx: &RuleContext{
				SourceClusterSnapshot: &collector.ClusterSnapshot{
					Components: map[string]collector.ComponentState{
						"tidb": {
							Type: types.ComponentTiDB,
							Variables: types.SystemVariables{
								"tidb_mem_quota_query": types.ParameterValue{Value: 2147483648, Type: "int"},
							},
						},
					},
				},
				SourceDefaults: map[string]map[string]interface{}{
					"tidb": {
						"sysvar:tidb_mem_quota_query": 1073741824,
					},
				},
			},
			wantErr: false,
			wantLen: 1,
		},
		{
			name: "map parameter with nested differences - only differing fields reported",
			ruleCtx: &RuleContext{
				SourceClusterSnapshot: &collector.ClusterSnapshot{
					Components: map[string]collector.ComponentState{
						"tikv": {
							Type: types.ComponentTiKV,
							Config: types.ConfigDefaults{
								"storage": types.ParameterValue{
									Value: map[string]interface{}{
										"data-dir": "/data/tikv",
										"reserve-space": "5GiB",
										"reserve-raft-space": "1GiB",
										"block-cache": map[string]interface{}{
											"capacity": "7373835KiB",
											"high-pri-pool-ratio": 0.8,
										},
									},
									Type: "map",
								},
							},
						},
					},
				},
				SourceDefaults: map[string]map[string]interface{}{
					"tikv": {
						"storage": map[string]interface{}{
							"data-dir": "/default/tikv",
							"reserve-space": "0KiB",
							"reserve-raft-space": "0KiB",
							"block-cache": map[string]interface{}{
								"capacity": "23192823398B",
								"high-pri-pool-ratio": 0.8,
							},
						},
					},
				},
			},
			wantErr: false,
			wantLen: 4, // reserve-space, reserve-raft-space, block-cache.capacity, storage.data-dir (nested)
		},
		{
			name: "map parameter with no differences - nothing reported",
			ruleCtx: &RuleContext{
				SourceClusterSnapshot: &collector.ClusterSnapshot{
					Components: map[string]collector.ComponentState{
						"tikv": {
							Type: types.ComponentTiKV,
							Config: types.ConfigDefaults{
								"storage": types.ParameterValue{
									Value: map[string]interface{}{
										"reserve-space": "5GiB",
										"reserve-raft-space": "1GiB",
									},
									Type: "map",
								},
							},
						},
					},
				},
				SourceDefaults: map[string]map[string]interface{}{
					"tikv": {
						"storage": map[string]interface{}{
							"reserve-space": "5GiB",
							"reserve-raft-space": "1GiB",
						},
					},
				},
			},
			wantErr: false,
			wantLen: 0, // No differences, nothing reported
		},
		{
			name: "top-level path parameter ignored",
			ruleCtx: &RuleContext{
				SourceClusterSnapshot: &collector.ClusterSnapshot{
					Components: map[string]collector.ComponentState{
						"tidb": {
							Type: types.ComponentTiDB,
							Config: types.ConfigDefaults{
								"data-dir": types.ParameterValue{Value: "/custom/data", Type: "string"},
							},
						},
					},
				},
				SourceDefaults: map[string]map[string]interface{}{
					"tidb": {
						"data-dir": "/default/data",
					},
				},
			},
			wantErr: false,
			wantLen: 0, // Top-level data-dir is ignored
		},
		{
			name: "nested path parameter in map NOT ignored",
			ruleCtx: &RuleContext{
				SourceClusterSnapshot: &collector.ClusterSnapshot{
					Components: map[string]collector.ComponentState{
						"tikv": {
							Type: types.ComponentTiKV,
							Config: types.ConfigDefaults{
								"storage": types.ParameterValue{
									Value: map[string]interface{}{
										"data-dir": "/custom/tikv/data",
									},
									Type: "map",
								},
							},
						},
					},
				},
				SourceDefaults: map[string]map[string]interface{}{
					"tikv": {
						"storage": map[string]interface{}{
							"data-dir": "/default/tikv/data",
						},
					},
				},
			},
			wantErr: false,
			wantLen: 1, // Nested storage.data-dir is NOT ignored
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule := NewUserModifiedParamsRule()
			ctx := context.Background()

			results, err := rule.Evaluate(ctx, tt.ruleCtx)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.wantLen, len(results), "Expected %d results, got %d. Results: %+v", tt.wantLen, len(results), results)
			}
		})
	}
}

