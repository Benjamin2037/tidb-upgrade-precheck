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
				assert.Equal(t, tt.wantLen, len(results))
			}
		})
	}
}

