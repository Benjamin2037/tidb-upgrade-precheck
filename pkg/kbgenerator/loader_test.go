package kbgenerator

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadKnowledgeBase(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(t *testing.T) string
		version string
		wantErr bool
		validate func(t *testing.T, kb map[string]interface{})
	}{
		{
			name: "non-existent path",
			setup: func(t *testing.T) string {
				return "/non/existent/path"
			},
			version: "v7.5.0",
			wantErr: false, // Returns empty KB, not an error
			validate: func(t *testing.T, kb map[string]interface{}) {
				assert.Empty(t, kb)
			},
		},
		{
			name: "valid knowledge base",
			setup: func(t *testing.T) string {
				tmpDir := t.TempDir()
				versionGroup := "v7.5"
				version := "v7.5.0"
				component := "tidb"

				// Create directory structure
				kbDir := filepath.Join(tmpDir, versionGroup, version, component)
				require.NoError(t, os.MkdirAll(kbDir, 0755))

				// Create defaults.json
				defaults := map[string]interface{}{
					"config_defaults": map[string]interface{}{
						"max-connections": 1000,
					},
					"system_variables": map[string]interface{}{
						"tidb_mem_quota_query": 1073741824,
					},
				}
				defaultsData, err := json.Marshal(defaults)
				require.NoError(t, err)
				require.NoError(t, os.WriteFile(filepath.Join(kbDir, "defaults.json"), defaultsData, 0644))

				// Create upgrade_logic.json
				upgradeLogicDir := filepath.Join(tmpDir, component)
				require.NoError(t, os.MkdirAll(upgradeLogicDir, 0755))
				upgradeLogic := map[string]interface{}{
					"changes": []interface{}{
						map[string]interface{}{
							"version": "v7.6.0",
							"name":    "tidb_mem_quota_query",
							"value":   2147483648,
						},
					},
				}
				upgradeLogicData, err := json.Marshal(upgradeLogic)
				require.NoError(t, err)
				require.NoError(t, os.WriteFile(filepath.Join(upgradeLogicDir, "upgrade_logic.json"), upgradeLogicData, 0644))

				return tmpDir
			},
			version: "v7.5.0",
			wantErr: false,
			validate: func(t *testing.T, kb map[string]interface{}) {
				assert.NotEmpty(t, kb)
				tidbKB, ok := kb["tidb"].(map[string]interface{})
				require.True(t, ok)
				assert.NotNil(t, tidbKB["config_defaults"])
				assert.NotNil(t, tidbKB["system_variables"])
				assert.NotNil(t, tidbKB["upgrade_logic"])
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kbPath := tt.setup(t)
			kb, err := LoadKnowledgeBase(kbPath, tt.version)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				if tt.validate != nil {
					tt.validate(t, kb)
				}
			}
		})
	}
}

func TestGetVersionGroup(t *testing.T) {
	tests := []struct {
		name    string
		version string
		want    string
	}{
		{
			name:    "full version",
			version:  "v7.5.0",
			want:    "v7.5",
		},
		{
			name:    "version with patch",
			version:  "v7.5.1",
			want:    "v7.5",
		},
		{
			name:    "version without v prefix",
			version:  "7.5.0",
			want:    "v7.5",
		},
		{
			name:    "two-digit version",
			version:  "v7.5",
			want:    "v7.5",
		},
		{
			name:    "single digit",
			version:  "v7",
			want:    "v7",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getVersionGroup(tt.version)
			assert.Equal(t, tt.want, result)
		})
	}
}

