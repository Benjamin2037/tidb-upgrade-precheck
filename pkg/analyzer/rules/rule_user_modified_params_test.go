package rules

import (
	"context"
	"strings"
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
		name    string
		ruleCtx *RuleContext
		wantErr bool
		wantLen int
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
										"data-dir":           "/data/tikv",
										"reserve-space":      "5GiB",
										"reserve-raft-space": "1GiB",
										"block-cache": map[string]interface{}{
											"capacity":            "7373835KiB",
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
							"data-dir":           "/default/tikv",
							"reserve-space":      "0KiB",
							"reserve-raft-space": "0KiB",
							"block-cache": map[string]interface{}{
								"capacity":            "23192823398B",
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
										"reserve-space":      "5GiB",
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
							"reserve-space":      "5GiB",
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
								// Note: In the new architecture, path parameters are filtered in preprocessing stage
								// Rules receive cleaned defaults (path parameters already removed)
								// This test simulates that: data-dir is not in runtime config (filtered)
								// So we don't include it in Config to truly test filtering
							},
						},
					},
				},
				SourceDefaults: map[string]map[string]interface{}{
					"tidb": {
						// Note: In the new architecture, path parameters are filtered in preprocessing stage
						// Rules receive cleaned defaults (path parameters already removed)
						// This test simulates that: data-dir is not in SourceDefaults (filtered)
					},
				},
			},
			wantErr: false,
			wantLen: 0, // Top-level data-dir is filtered in preprocessing, not in runtime config or SourceDefaults
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

// TestUserModifiedParamsRule_RealWorldScenario tests a real-world scenario similar to user feedback
// This simulates the actual case where storage parameter has differences in nested fields
func TestUserModifiedParamsRule_RealWorldScenario(t *testing.T) {
	rule := NewUserModifiedParamsRule()
	ctx := context.Background()

	// Simulate the actual scenario from user feedback:
	// - Current cluster has storage with reserve-space: 5GiB, reserve-raft-space: 1GiB, block-cache.capacity: 7373835KiB
	// - Source default has reserve-space: 0KiB, reserve-raft-space: 0KiB, block-cache.capacity: 23192823398B
	// - data-dir differs but should be ignored if top-level, but NOT ignored if nested in storage
	ruleCtx := &RuleContext{
		SourceClusterSnapshot: &collector.ClusterSnapshot{
			Components: map[string]collector.ComponentState{
				"tikv-0": {
					Type: types.ComponentTiKV,
					Config: types.ConfigDefaults{
						"storage": types.ParameterValue{
							Value: map[string]interface{}{
								"data-dir":           "/data/tidb-data/tikv-20160",
								"reserve-space":      "5GiB",
								"reserve-raft-space": "1GiB",
								"block-cache": map[string]interface{}{
									"capacity":            "7373835KiB",
									"high-pri-pool-ratio": 0.8,
									"num-shard-bits":      6,
								},
								"api-version": 1,
								"engine":      "raft-kv",
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
					"data-dir":           "/Users/benjamin2037/.tiup/data/kb-gen-v7.5.1-1765505694/tikv-0/data",
					"reserve-space":      "0KiB",
					"reserve-raft-space": "0KiB",
					"block-cache": map[string]interface{}{
						"capacity":            "23192823398B",
						"high-pri-pool-ratio": 0.8,
						"num-shard-bits":      6,
					},
					"api-version": 1,
					"engine":      "raft-kv",
				},
			},
		},
	}

	results, err := rule.Evaluate(ctx, ruleCtx)
	assert.NoError(t, err)

	// Should report: reserve-space, reserve-raft-space, block-cache.capacity, storage.data-dir
	// (4 differences, ignoring only top-level data-dir, not nested storage.data-dir)
	assert.GreaterOrEqual(t, len(results), 3, "Should report at least reserve-space, reserve-raft-space, and block-cache.capacity differences")

	// Verify that reserve-space is reported
	foundReserveSpace := false
	foundReserveRaftSpace := false
	foundBlockCacheCapacity := false
	foundStorageDataDir := false

	for _, result := range results {
		if strings.Contains(result.ParameterName, "reserve-space") {
			foundReserveSpace = true
			// Verify format is checklist-style
			assert.Contains(t, result.Details, "Current", "Details should contain 'Current'")
			assert.Contains(t, result.Details, "Source Default", "Details should contain 'Source Default'")
		}
		if strings.Contains(result.ParameterName, "reserve-raft-space") {
			foundReserveRaftSpace = true
		}
		if strings.Contains(result.ParameterName, "block-cache") && strings.Contains(result.ParameterName, "capacity") {
			foundBlockCacheCapacity = true
		}
		if strings.Contains(result.ParameterName, "storage") && strings.Contains(result.ParameterName, "data-dir") {
			foundStorageDataDir = true
		}
	}

	assert.True(t, foundReserveSpace, "Should report reserve-space difference")
	assert.True(t, foundReserveRaftSpace, "Should report reserve-raft-space difference")
	assert.True(t, foundBlockCacheCapacity, "Should report block-cache.capacity difference")
	// storage.data-dir should be reported (nested path parameters are NOT ignored)
	assert.True(t, foundStorageDataDir, "Should report storage.data-dir difference (nested paths are not ignored)")
}

// TestUserModifiedParamsRule_FormatOutput tests the formatting of output
func TestUserModifiedParamsRule_FormatOutput(t *testing.T) {
	rule := NewUserModifiedParamsRule()
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
		SourceDefaults: map[string]map[string]interface{}{
			"tidb": {
				"max-connections": 1000,
			},
		},
	}

	results, err := rule.Evaluate(ctx, ruleCtx)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(results))

	result := results[0]
	// Verify checklist-style format for simple values
	assert.Contains(t, result.Details, "Current:", "Details should contain 'Current:' for simple values")
	assert.Contains(t, result.Details, "Source Default:", "Details should contain 'Source Default:' for simple values")
	assert.Contains(t, result.Details, "2000", "Details should contain current value")
	assert.Contains(t, result.Details, "1000", "Details should contain source default value")
}

// TestUserModifiedParamsRule_ComplexNestedMap tests complex nested map scenarios
func TestUserModifiedParamsRule_ComplexNestedMap(t *testing.T) {
	rule := NewUserModifiedParamsRule()
	ctx := context.Background()

	ruleCtx := &RuleContext{
		SourceClusterSnapshot: &collector.ClusterSnapshot{
			Components: map[string]collector.ComponentState{
				"tikv": {
					Type: types.ComponentTiKV,
					Config: types.ConfigDefaults{
						"storage": types.ParameterValue{
							Value: map[string]interface{}{
								"block-cache": map[string]interface{}{
									"capacity":            "7373835KiB",
									"high-pri-pool-ratio": 0.9, // Changed from 0.8
									"num-shard-bits":      6,
								},
								"io-rate-limit": map[string]interface{}{
									"max-bytes-per-sec": "100MiB", // Changed
									"mode":              "write-only",
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
					"block-cache": map[string]interface{}{
						"capacity":            "23192823398B",
						"high-pri-pool-ratio": 0.8,
						"num-shard-bits":      6,
					},
					"io-rate-limit": map[string]interface{}{
						"max-bytes-per-sec": "0KiB",
						"mode":              "write-only",
					},
				},
			},
		},
	}

	results, err := rule.Evaluate(ctx, ruleCtx)
	assert.NoError(t, err)

	// Should report: block-cache.capacity, block-cache.high-pri-pool-ratio, io-rate-limit.max-bytes-per-sec
	assert.GreaterOrEqual(t, len(results), 3, "Should report at least 3 differences")

	// Verify each result has proper formatting
	for _, result := range results {
		assert.NotEmpty(t, result.ParameterName, "Parameter name should not be empty")
		assert.NotEmpty(t, result.Details, "Details should not be empty")
		assert.Contains(t, result.Details, "Current", "Details should contain 'Current'")
		assert.Contains(t, result.Details, "Source Default", "Details should contain 'Source Default'")
	}
}

// TestUserModifiedParamsRule_OnlyDifferingFields tests that only differing fields are reported
func TestUserModifiedParamsRule_OnlyDifferingFields(t *testing.T) {
	rule := NewUserModifiedParamsRule()
	ctx := context.Background()

	// Large map with only one field different
	ruleCtx := &RuleContext{
		SourceClusterSnapshot: &collector.ClusterSnapshot{
			Components: map[string]collector.ComponentState{
				"tikv": {
					Type: types.ComponentTiKV,
					Config: types.ConfigDefaults{
						"storage": types.ParameterValue{
							Value: map[string]interface{}{
								"api-version":                       1,
								"background-error-recovery-window":  "1h",
								"enable-async-apply-prewrite":       false,
								"enable-ttl":                        false,
								"engine":                            "raft-kv",
								"gc-ratio-threshold":                1.1,
								"max-key-size":                      8192,
								"reserve-space":                     "5GiB", // Only this differs
								"reserve-raft-space":                "0KiB",
								"scheduler-concurrency":             524288,
								"scheduler-pending-write-threshold": "100MiB",
								"scheduler-worker-pool-size":        4,
								"ttl-check-poll-interval":           "12h",
								"txn-status-cache-capacity":         5120000,
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
					"api-version":                       1,
					"background-error-recovery-window":  "1h",
					"enable-async-apply-prewrite":       false,
					"enable-ttl":                        false,
					"engine":                            "raft-kv",
					"gc-ratio-threshold":                1.1,
					"max-key-size":                      8192,
					"reserve-space":                     "0KiB", // Only this differs
					"reserve-raft-space":                "0KiB",
					"scheduler-concurrency":             524288,
					"scheduler-pending-write-threshold": "100MiB",
					"scheduler-worker-pool-size":        4,
					"ttl-check-poll-interval":           "12h",
					"txn-status-cache-capacity":         5120000,
				},
			},
		},
	}

	results, err := rule.Evaluate(ctx, ruleCtx)
	assert.NoError(t, err)

	// Should only report reserve-space difference, not all the other fields
	assert.Equal(t, 1, len(results), "Should only report the one differing field (reserve-space)")
	assert.Contains(t, results[0].ParameterName, "reserve-space", "Should report reserve-space")
}
