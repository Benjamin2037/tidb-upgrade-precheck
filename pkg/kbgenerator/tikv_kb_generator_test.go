package kbgenerator

import (
	"os"
	"testing"
)

func TestCollectFromTikvSource(t *testing.T) {
	tests := []struct {
		name       string
		repoPath   string
		version    string
		wantErr    bool
	}{
		{
			name:     "Non-existent repository path",
			repoPath: "/non/existent/path",
			version:  "v6.5.0",
			wantErr:  true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := CollectFromTikvSource(tt.repoPath, tt.version)
			if (err != nil) != tt.wantErr {
				t.Errorf("CollectFromTikvSource() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSaveTikvKBSnapshot(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "tikv_kb_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	tests := []struct {
		name       string
		snapshot   *TikvKBSnapshot
		outputPath string
		wantErr    bool
	}{
		{
			name: "Valid snapshot",
			snapshot: &TikvKBSnapshot{
				Version: "test-version",
				ConfigDefaults: map[string]interface{}{
					"test.param": "test-value",
				},
			},
			outputPath: tempDir + "/test_snapshot.json",
			wantErr:    false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := SaveTikvKBSnapshot(tt.snapshot, tt.outputPath)
			if (err != nil) != tt.wantErr {
				t.Errorf("SaveTikvKBSnapshot() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			
			// If no error expected, verify file exists
			if !tt.wantErr {
				_, err = os.Stat(tt.outputPath)
				if err != nil {
					t.Errorf("Snapshot file was not created: %v", err)
				}
			}
		})
	}
}

func TestGetTikvParameterChanges(t *testing.T) {
	tests := []struct {
		name        string
		repoPath    string
		fromVersion string
		toVersion   string
		wantErr     bool
	}{
		{
			name:        "Valid version comparison",
			repoPath:    "/non/existent/path",
			fromVersion: "v6.5.0",
			toVersion:   "v7.1.0",
			wantErr:     false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			changes, err := GetTikvParameterChanges(tt.repoPath, tt.fromVersion, tt.toVersion)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetTikvParameterChanges() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			
			// Only check results if no error expected
			if !tt.wantErr {
				// We expect at least one change
				if len(changes) == 0 {
					t.Error("Expected at least one parameter change")
					return
				}

				// Check the first change
				change := changes[0]
				if change.Name != "storage.reserve-space" {
					t.Errorf("Expected parameter name 'storage.reserve-space', got '%s'", change.Name)
				}
				if change.FromValue != "2GB" {
					t.Errorf("Expected from value '2GB', got '%v'", change.FromValue)
				}
				if change.ToValue != "0" {
					t.Errorf("Expected to value '0', got '%v'", change.ToValue)
				}
			}
		})
	}
}