package analyzer

import (
	"context"
	"testing"

	"github.com/pingcap/tidb-upgrade-precheck/pkg/analyzer/rules"
	"github.com/pingcap/tidb-upgrade-precheck/pkg/collector"
	"github.com/pingcap/tidb-upgrade-precheck/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewAnalyzer(t *testing.T) {
	tests := []struct {
		name    string
		options *AnalysisOptions
		wantErr bool
	}{
		{
			name:    "nil options",
			options: nil,
			wantErr: false,
		},
		{
			name:    "empty options",
			options: &AnalysisOptions{},
			wantErr: false,
		},
		{
			name: "with custom rules",
			options: &AnalysisOptions{
				Rules: []rules.Rule{
					rules.NewUserModifiedParamsRule(),
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			analyzer := NewAnalyzer(tt.options)
			assert.NotNil(t, analyzer)
			assert.NotNil(t, analyzer.options)
			assert.NotEmpty(t, analyzer.rules)
		})
	}
}

func TestAnalyzer_GetDataRequirements(t *testing.T) {
	analyzer := NewAnalyzer(nil)
	req := analyzer.GetDataRequirements()

	assert.NotNil(t, req)
	// Default rules should require some components
	assert.NotEmpty(t, req.SourceClusterRequirements.Components)
}

func TestAnalyzer_GetCollectionRequirements(t *testing.T) {
	analyzer := NewAnalyzer(nil)
	req := analyzer.GetCollectionRequirements()

	assert.NotNil(t, req)
	assert.NotEmpty(t, req.Components)
	assert.True(t, req.NeedConfig || req.NeedSystemVariables)
}

func TestAnalyzer_Analyze(t *testing.T) {
	tests := []struct {
		name          string
		snapshot      *collector.ClusterSnapshot
		sourceVersion string
		targetVersion string
		sourceKB      map[string]interface{}
		targetKB      map[string]interface{}
		wantErr       bool
	}{
		{
			name:          "nil snapshot",
			snapshot:       nil,
			sourceVersion:  "v7.5.0",
			targetVersion:  "v8.0.0",
			sourceKB:      make(map[string]interface{}),
			targetKB:      make(map[string]interface{}),
			wantErr:       true,
		},
		{
			name: "empty snapshot",
			snapshot: &collector.ClusterSnapshot{
				Components: make(map[string]collector.ComponentState),
			},
			sourceVersion: "v7.5.0",
			targetVersion: "v8.0.0",
			sourceKB:     make(map[string]interface{}),
			targetKB:     make(map[string]interface{}),
			wantErr:      false,
		},
		{
			name: "with TiDB component",
			snapshot: &collector.ClusterSnapshot{
				Components: map[string]collector.ComponentState{
					"tidb": {
						Type:    types.ComponentTiDB,
						Version: "v7.5.0",
						Config: types.ConfigDefaults{
							"max-connections": types.ParameterValue{Value: 1000, Type: "int"},
						},
						Variables: types.SystemVariables{
							"tidb_mem_quota_query": types.ParameterValue{Value: 1073741824, Type: "int"},
						},
					},
				},
			},
			sourceVersion: "v7.5.0",
			targetVersion: "v8.0.0",
			sourceKB: map[string]interface{}{
				"tidb": map[string]interface{}{
					"config_defaults": map[string]interface{}{
						"max-connections": 1000,
					},
					"system_variables": map[string]interface{}{
						"tidb_mem_quota_query": 1073741824,
					},
				},
			},
			targetKB: map[string]interface{}{
				"tidb": map[string]interface{}{
					"config_defaults": map[string]interface{}{
						"max-connections": 2000,
					},
					"system_variables": map[string]interface{}{
						"tidb_mem_quota_query": 2147483648,
					},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			analyzer := NewAnalyzer(nil)
			ctx := context.Background()

			result, err := analyzer.Analyze(ctx, tt.snapshot, tt.sourceVersion, tt.targetVersion, tt.sourceKB, tt.targetKB)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				assert.Equal(t, tt.sourceVersion, result.SourceVersion)
				assert.Equal(t, tt.targetVersion, result.TargetVersion)
			}
		})
	}
}

func TestAnalyzer_collectDataRequirements(t *testing.T) {
	analyzer := NewAnalyzer(nil)
	req := analyzer.collectDataRequirements()

	assert.NotNil(t, req)
	// Should have merged requirements from all default rules
	assert.NotEmpty(t, req.SourceClusterRequirements.Components)
}

func TestAnalyzer_loadKBFromRequirements(t *testing.T) {
	analyzer := NewAnalyzer(nil)

	tests := []struct {
		name                string
		kb                  map[string]interface{}
		components          []string
		needConfigDefaults  bool
		needSystemVariables bool
		want                map[string]map[string]interface{}
	}{
		{
			name:                "no requirements",
			kb:                  make(map[string]interface{}),
			components:          []string{"tidb"},
			needConfigDefaults:  false,
			needSystemVariables: false,
			want:                make(map[string]map[string]interface{}),
		},
		{
			name: "with config defaults",
			kb: map[string]interface{}{
				"tidb": map[string]interface{}{
					"config_defaults": map[string]interface{}{
						"max-connections": 1000,
					},
				},
			},
			components:          []string{"tidb"},
			needConfigDefaults:  true,
			needSystemVariables: false,
			want: map[string]map[string]interface{}{
				"tidb": {
					"max-connections": 1000,
				},
			},
		},
		{
			name: "with system variables",
			kb: map[string]interface{}{
				"tidb": map[string]interface{}{
					"system_variables": map[string]interface{}{
						"tidb_mem_quota_query": 1073741824,
					},
				},
			},
			components:          []string{"tidb"},
			needConfigDefaults:  false,
			needSystemVariables: true,
			want: map[string]map[string]interface{}{
				"tidb": {
					"sysvar:tidb_mem_quota_query": 1073741824,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, _ := analyzer.loadKBFromRequirements(tt.kb, tt.components, tt.needConfigDefaults, tt.needSystemVariables)
			assert.Equal(t, tt.want, result)
		})
	}
}

func TestAnalyzer_buildComponentMapping(t *testing.T) {
	analyzer := NewAnalyzer(nil)

	tests := []struct {
		name           string
		snapshot       *collector.ClusterSnapshot
		sourceDefaults map[string]map[string]interface{}
		want           map[string]string
	}{
		{
			name: "exact match",
			snapshot: &collector.ClusterSnapshot{
				Components: map[string]collector.ComponentState{
					"tidb": {
						Type: types.ComponentTiDB,
					},
				},
			},
			sourceDefaults: map[string]map[string]interface{}{
				"tidb": {"param1": "value1"},
			},
			want: map[string]string{
				"tidb": "tidb",
			},
		},
		{
			name: "TiKV prefix match",
			snapshot: &collector.ClusterSnapshot{
				Components: map[string]collector.ComponentState{
					"tikv-192-168-1-100-20160": {
						Type: types.ComponentTiKV,
					},
				},
			},
			sourceDefaults: map[string]map[string]interface{}{
				"tikv": {"param1": "value1"},
			},
			want: map[string]string{
				"tikv": "tikv-192-168-1-100-20160",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := analyzer.buildComponentMapping(tt.snapshot, tt.sourceDefaults)
			assert.Equal(t, tt.want, result)
		})
	}
}

func TestMergeStringSlices(t *testing.T) {
	tests := []struct {
		name   string
		slice1 []string
		slice2 []string
		want   []string
	}{
		{
			name:   "empty slices",
			slice1: []string{},
			slice2: []string{},
			want:   []string{},
		},
		{
			name:   "no duplicates",
			slice1: []string{"a", "b"},
			slice2: []string{"c", "d"},
			want:   []string{"a", "b", "c", "d"},
		},
		{
			name:   "with duplicates",
			slice1: []string{"a", "b"},
			slice2: []string{"b", "c"},
			want:   []string{"a", "b", "c"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mergeStringSlices(tt.slice1, tt.slice2)
			require.Equal(t, len(tt.want), len(result))
			for _, v := range tt.want {
				assert.Contains(t, result, v)
			}
		})
	}
}

