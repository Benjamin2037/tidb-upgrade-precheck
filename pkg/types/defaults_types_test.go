package types

import (
	"os"
	"path/filepath"
	"testing"

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
				Component:      ComponentTiDB,
				Version:       "v7.5.0",
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
				Component:      ComponentPD,
				Version:       "v7.5.0",
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
						Version: "v7.6.0",
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
				Changes:  []UpgradeParamChange{},
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
				"port":           4000,
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

