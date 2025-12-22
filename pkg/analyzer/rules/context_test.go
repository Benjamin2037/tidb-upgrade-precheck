package rules

import (
	"testing"

	"github.com/pingcap/tidb-upgrade-precheck/pkg/collector"
	"github.com/stretchr/testify/assert"
)

func TestNewRuleContext(t *testing.T) {
	snapshot := &collector.ClusterSnapshot{
		Components: make(map[string]collector.ComponentState),
	}

	ruleCtx := NewRuleContext(
		snapshot,
		"v7.5.0",
		"v8.5.0",
		make(map[string]map[string]interface{}),
		make(map[string]map[string]interface{}),
		make(map[string]interface{}),
		140, // sourceBootstrapVersion
		160, // targetBootstrapVersion
		make(map[string]interface{}), // parameterNotes
	)

	assert.NotNil(t, ruleCtx)
	assert.Equal(t, snapshot, ruleCtx.SourceClusterSnapshot)
	assert.Equal(t, "v7.5.0", ruleCtx.SourceVersion)
	assert.Equal(t, "v8.5.0", ruleCtx.TargetVersion)
	assert.NotNil(t, ruleCtx.SourceDefaults)
	assert.NotNil(t, ruleCtx.TargetDefaults)
	assert.NotNil(t, ruleCtx.UpgradeLogic)
}

func TestRuleContext_GetForcedChanges(t *testing.T) {
	tests := []struct {
		name         string
		ruleCtx      *RuleContext
		wantLen      int
		checkVersion string
	}{
		{
			name: "no upgrade logic",
			ruleCtx: &RuleContext{
				SourceVersion: "v7.5.0",
				TargetVersion: "v8.5.0",
				UpgradeLogic:  make(map[string]interface{}),
			},
			wantLen: 0,
		},
		{
			name: "with forced changes in range",
			ruleCtx: &RuleContext{
				SourceVersion:          "v7.5.0",
				TargetVersion:          "v8.5.0",
				SourceBootstrapVersion: 140,
				TargetBootstrapVersion: 160,
				UpgradeLogic: map[string]interface{}{
					"tidb": map[string]interface{}{
						"changes": []interface{}{
							map[string]interface{}{
								"version":           "150", // Bootstrap version in range (140 < 150 <= 160)
								"bootstrap_version": 150,
								"name":              "tidb_mem_quota_query",
								"value":             2147483648,
							},
						},
					},
				},
			},
			wantLen: 1,
		},
		{
			name: "forced change outside range",
			ruleCtx: &RuleContext{
				SourceVersion:          "v7.5.0",
				TargetVersion:          "v8.5.0",
				SourceBootstrapVersion: 140,
				TargetBootstrapVersion: 160,
				UpgradeLogic: map[string]interface{}{
					"tidb": map[string]interface{}{
						"changes": []interface{}{
							map[string]interface{}{
								"version":           "130", // Bootstrap version outside range (130 <= 140)
								"bootstrap_version": 130,
								"name":              "tidb_mem_quota_query",
								"value":             1073741824,
							},
						},
					},
				},
			},
			wantLen: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			changes := tt.ruleCtx.GetForcedChanges("tidb")
			assert.Equal(t, tt.wantLen, len(changes))
		})
	}
}

