package types

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSaveKBSnapshot(t *testing.T) {
	tests := []struct {
		name     string
		snapshot *KBSnapshot
		wantErr  bool
	}{
		{
			name: "valid snapshot",
			snapshot: &KBSnapshot{
				Component: ComponentTiDB,
				Version:   "v7.5.0",
				ConfigDefaults: ConfigDefaults{
					"max-connections": ParameterValue{
						Value: 1000,
						Type:  "int",
					},
				},
				SystemVariables: SystemVariables{
					"tidb_mem_quota_query": ParameterValue{
						Value: 1073741824,
						Type:  "int",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "snapshot without system variables",
			snapshot: &KBSnapshot{
				Component: ComponentPD,
				Version:   "v7.5.0",
				ConfigDefaults: ConfigDefaults{
					"max-connections": ParameterValue{
						Value: 1000,
						Type:  "int",
					},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			outputPath := filepath.Join(tmpDir, "defaults.json")

			err := SaveKBSnapshot(tt.snapshot, outputPath)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				// Verify file exists
				_, statErr := os.Stat(outputPath)
				assert.NoError(t, statErr)

				// Verify file content
				data, readErr := os.ReadFile(outputPath)
				require.NoError(t, readErr)
				assert.NotEmpty(t, data)
			}
		})
	}
}

func TestSaveUpgradeLogic(t *testing.T) {
	tests := []struct {
		name     string
		snapshot *UpgradeLogicSnapshot
		wantErr  bool
	}{
		{
			name: "valid upgrade logic",
			snapshot: &UpgradeLogicSnapshot{
				Component: ComponentTiDB,
				Changes: []UpgradeParamChange{
					{
						Version: "v8.5.0",
						Name:    "tidb_mem_quota_query",
						Value:   2147483648,
						Force:   true,
					},
				},
			},
			wantErr: false,
		},
		{
			name: "empty changes",
			snapshot: &UpgradeLogicSnapshot{
				Component: ComponentTiDB,
				Changes:   []UpgradeParamChange{},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			outputPath := filepath.Join(tmpDir, "upgrade_logic.json")

			err := SaveUpgradeLogic(tt.snapshot, outputPath)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				// Verify file exists
				_, statErr := os.Stat(outputPath)
				assert.NoError(t, statErr)

				// Verify file content
				data, readErr := os.ReadFile(outputPath)
				require.NoError(t, readErr)
				assert.NotEmpty(t, data)
			}
		})
	}
}

func TestConvertConfigToDefaults(t *testing.T) {
	tests := []struct {
		name   string
		config map[string]interface{}
		want   ConfigDefaults
	}{
		{
			name:   "empty config",
			config: make(map[string]interface{}),
			want:   make(ConfigDefaults),
		},
		{
			name: "with values",
			config: map[string]interface{}{
				"max-connections": 1000,
				"port":            4000,
			},
			want: ConfigDefaults{
				"max-connections": ParameterValue{Value: 1000, Type: "int"},
				"port":            ParameterValue{Value: 4000, Type: "int"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ConvertConfigToDefaults(tt.config)
			assert.Equal(t, len(tt.want), len(result))
			for k, v := range tt.want {
				assert.Equal(t, v.Value, result[k].Value)
			}
		})
	}
}

func TestConvertVariablesToSystemVariables(t *testing.T) {
	tests := []struct {
		name      string
		variables map[string]string
		want      SystemVariables
	}{
		{
			name:      "empty variables",
			variables: make(map[string]string),
			want:      make(SystemVariables),
		},
		{
			name: "with values",
			variables: map[string]string{
				"tidb_mem_quota_query": "1073741824",
			},
			want: SystemVariables{
				"tidb_mem_quota_query": ParameterValue{Value: "1073741824", Type: "string"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ConvertVariablesToSystemVariables(tt.variables)
			assert.Equal(t, len(tt.want), len(result))
			for k, v := range tt.want {
				assert.Equal(t, v.Value, result[k].Value)
			}
		})
	}
}

func TestComponentState_JSON(t *testing.T) {
	tests := []struct {
		name    string
		state   ComponentState
		wantErr bool
	}{
		{
			name: "valid component state",
			state: ComponentState{
				Type:    ComponentTiDB,
				Version: "v7.5.0",
				Config: ConfigDefaults{
					"max-connections": ParameterValue{Value: 1000, Type: "int"},
				},
				Variables: SystemVariables{
					"tidb_mem_quota_query": ParameterValue{Value: "1073741824", Type: "string"},
				},
				Status: map[string]interface{}{
					"status": "running",
				},
			},
			wantErr: false,
		},
		{
			name: "component state without variables",
			state: ComponentState{
				Type:    ComponentPD,
				Version: "v7.5.0",
				Config: ConfigDefaults{
					"max-request-size": ParameterValue{Value: 100, Type: "int"},
				},
				Status: make(map[string]interface{}),
			},
			wantErr: false,
		},
		{
			name: "empty component state",
			state: ComponentState{
				Type:    ComponentTiKV,
				Version: "",
				Config:  make(ConfigDefaults),
				Status:  make(map[string]interface{}),
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test JSON marshaling
			data, err := json.Marshal(tt.state)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotEmpty(t, data)

				// Test JSON unmarshaling
				var unmarshaled ComponentState
				err = json.Unmarshal(data, &unmarshaled)
				assert.NoError(t, err)
				assert.Equal(t, tt.state.Type, unmarshaled.Type)
				assert.Equal(t, tt.state.Version, unmarshaled.Version)
				assert.Equal(t, len(tt.state.Config), len(unmarshaled.Config))
				assert.Equal(t, len(tt.state.Variables), len(unmarshaled.Variables))
			}
		})
	}
}

func TestClusterSnapshot_JSON(t *testing.T) {
	tests := []struct {
		name     string
		snapshot ClusterSnapshot
		wantErr  bool
	}{
		{
			name: "valid cluster snapshot",
			snapshot: ClusterSnapshot{
				Timestamp:     time.Now(),
				SourceVersion: "v7.5.0",
				TargetVersion: "v8.5.0",
				Components: map[string]ComponentState{
					"tidb": {
						Type:    ComponentTiDB,
						Version: "v7.5.0",
						Config: ConfigDefaults{
							"max-connections": ParameterValue{Value: 1000, Type: "int"},
						},
						Variables: SystemVariables{
							"tidb_mem_quota_query": ParameterValue{Value: "1073741824", Type: "string"},
						},
						Status: make(map[string]interface{}),
					},
					"pd": {
						Type:    ComponentPD,
						Version: "v7.5.0",
						Config: ConfigDefaults{
							"max-request-size": ParameterValue{Value: 100, Type: "int"},
						},
						Status: make(map[string]interface{}),
					},
				},
			},
			wantErr: false,
		},
		{
			name: "cluster snapshot without target version",
			snapshot: ClusterSnapshot{
				Timestamp:     time.Now(),
				SourceVersion: "v7.5.0",
				Components: map[string]ComponentState{
					"tidb": {
						Type:    ComponentTiDB,
						Version: "v7.5.0",
						Config:  make(ConfigDefaults),
						Status:  make(map[string]interface{}),
					},
				},
			},
			wantErr: false,
		},
		{
			name: "empty cluster snapshot",
			snapshot: ClusterSnapshot{
				Timestamp:     time.Now(),
				SourceVersion: "",
				Components:    make(map[string]ComponentState),
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test JSON marshaling
			data, err := json.Marshal(tt.snapshot)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotEmpty(t, data)

				// Test JSON unmarshaling
				var unmarshaled ClusterSnapshot
				err = json.Unmarshal(data, &unmarshaled)
				assert.NoError(t, err)
				assert.Equal(t, tt.snapshot.SourceVersion, unmarshaled.SourceVersion)
				assert.Equal(t, tt.snapshot.TargetVersion, unmarshaled.TargetVersion)
				assert.Equal(t, len(tt.snapshot.Components), len(unmarshaled.Components))
			}
		})
	}
}

func TestClusterEndpoints_JSON(t *testing.T) {
	tests := []struct {
		name      string
		endpoints ClusterEndpoints
		wantErr   bool
	}{
		{
			name: "valid cluster endpoints",
			endpoints: ClusterEndpoints{
				TiDBAddr:     "127.0.0.1:4000",
				TiDBUser:     "root",
				TiDBPassword: "password",
				TiKVAddrs:    []string{"127.0.0.1:20160", "127.0.0.1:20161"},
				TiKVDataDirs: map[string]string{
					"127.0.0.1:20160": "/data/tikv1",
					"127.0.0.1:20161": "/data/tikv2",
				},
				PDAddrs:       []string{"127.0.0.1:2379"},
				TiFlashAddrs:  []string{"127.0.0.1:3930"},
				SourceVersion: "v7.5.0",
			},
			wantErr: false,
		},
		{
			name: "cluster endpoints with minimal fields",
			endpoints: ClusterEndpoints{
				TiDBAddr: "127.0.0.1:4000",
			},
			wantErr: false,
		},
		{
			name:      "empty cluster endpoints",
			endpoints: ClusterEndpoints{},
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test JSON marshaling
			data, err := json.Marshal(tt.endpoints)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotEmpty(t, data)

				// Test JSON unmarshaling
				var unmarshaled ClusterEndpoints
				err = json.Unmarshal(data, &unmarshaled)
				assert.NoError(t, err)
				assert.Equal(t, tt.endpoints.TiDBAddr, unmarshaled.TiDBAddr)
				assert.Equal(t, tt.endpoints.TiDBUser, unmarshaled.TiDBUser)
				assert.Equal(t, len(tt.endpoints.TiKVAddrs), len(unmarshaled.TiKVAddrs))
				assert.Equal(t, len(tt.endpoints.PDAddrs), len(unmarshaled.PDAddrs))
				assert.Equal(t, len(tt.endpoints.TiFlashAddrs), len(unmarshaled.TiFlashAddrs))
			}
		})
	}
}

func TestInstanceState_JSON(t *testing.T) {
	tests := []struct {
		name     string
		instance InstanceState
		wantErr  bool
	}{
		{
			name: "valid instance state",
			instance: InstanceState{
				Address: "127.0.0.1:4000",
				State: ComponentState{
					Type:    ComponentTiDB,
					Version: "v7.5.0",
					Config: ConfigDefaults{
						"max-connections": ParameterValue{Value: 1000, Type: "int"},
					},
					Status: make(map[string]interface{}),
				},
			},
			wantErr: false,
		},
		{
			name: "instance state with empty address",
			instance: InstanceState{
				Address: "",
				State: ComponentState{
					Type:    ComponentPD,
					Version: "v7.5.0",
					Config:  make(ConfigDefaults),
					Status:  make(map[string]interface{}),
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test JSON marshaling
			data, err := json.Marshal(tt.instance)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotEmpty(t, data)

				// Test JSON unmarshaling
				var unmarshaled InstanceState
				err = json.Unmarshal(data, &unmarshaled)
				assert.NoError(t, err)
				assert.Equal(t, tt.instance.Address, unmarshaled.Address)
				assert.Equal(t, tt.instance.State.Type, unmarshaled.State.Type)
				assert.Equal(t, tt.instance.State.Version, unmarshaled.State.Version)
			}
		})
	}
}

func TestClusterState_JSON(t *testing.T) {
	tests := []struct {
		name         string
		clusterState ClusterState
		wantErr      bool
	}{
		{
			name: "valid cluster state",
			clusterState: ClusterState{
				Instances: []InstanceState{
					{
						Address: "127.0.0.1:4000",
						State: ComponentState{
							Type:    ComponentTiDB,
							Version: "v7.5.0",
							Config:  make(ConfigDefaults),
							Status:  make(map[string]interface{}),
						},
					},
					{
						Address: "127.0.0.1:2379",
						State: ComponentState{
							Type:    ComponentPD,
							Version: "v7.5.0",
							Config:  make(ConfigDefaults),
							Status:  make(map[string]interface{}),
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "empty cluster state",
			clusterState: ClusterState{
				Instances: []InstanceState{},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test JSON marshaling
			data, err := json.Marshal(tt.clusterState)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotEmpty(t, data)

				// Test JSON unmarshaling
				var unmarshaled ClusterState
				err = json.Unmarshal(data, &unmarshaled)
				assert.NoError(t, err)
				assert.Equal(t, len(tt.clusterState.Instances), len(unmarshaled.Instances))
			}
		})
	}
}

func TestConvertConfigToDefaults_EdgeCases(t *testing.T) {
	tests := []struct {
		name   string
		config map[string]interface{}
	}{
		{
			name: "with different value types",
			config: map[string]interface{}{
				"string_val": "test",
				"int_val":    100,
				"float_val":  3.14,
				"bool_val":   true,
				"array_val":  []interface{}{1, 2, 3},
				"map_val":    map[string]interface{}{"key": "value"},
				"nil_val":    nil,
			},
		},
		{
			name: "with zero values",
			config: map[string]interface{}{
				"zero_int":     0,
				"zero_float":   0.0,
				"empty_string": "",
				"false_bool":   false,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ConvertConfigToDefaults(tt.config)
			assert.Equal(t, len(tt.config), len(result))
			for k, v := range tt.config {
				assert.Contains(t, result, k)
				assert.Equal(t, v, result[k].Value)
				// Verify type is determined correctly
				assert.NotEmpty(t, result[k].Type)
			}
		})
	}
}

func TestConvertVariablesToSystemVariables_EdgeCases(t *testing.T) {
	tests := []struct {
		name      string
		variables map[string]string
	}{
		{
			name: "with empty strings",
			variables: map[string]string{
				"empty":     "",
				"non_empty": "value",
			},
		},
		{
			name: "with special characters",
			variables: map[string]string{
				"special": "value with spaces and !@#$%",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ConvertVariablesToSystemVariables(tt.variables)
			assert.Equal(t, len(tt.variables), len(result))
			for k, v := range tt.variables {
				assert.Contains(t, result, k)
				assert.Equal(t, v, result[k].Value)
				assert.Equal(t, "string", result[k].Type)
			}
		})
	}
}
