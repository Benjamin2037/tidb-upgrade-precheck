// Copyright 2024 PingCAP, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// See the License for the specific language governing permissions and
// limitations under the License.

package tikv

import (
	"os"
	"testing"
)

func TestCollector_Collect(t *testing.T) {
	collector := NewCollector("/fake/path")
	
	// Test collecting for a specific version
	snapshot, err := collector.Collect("v6.5.0")
	if err != nil {
		t.Fatalf("Failed to collect: %v", err)
	}
	
	if snapshot.Version != "v6.5.0" {
		t.Errorf("Expected version 'v6.5.0', got %s", snapshot.Version)
	}
	
	if len(snapshot.ConfigDefaults) == 0 {
		t.Error("Expected some config defaults")
	}
	
	// Check for specific parameters
	if _, exists := snapshot.ConfigDefaults["storage.reserve-space"]; !exists {
		t.Error("Expected storage.reserve-space in config defaults")
	}
}

func TestCollector_Save(t *testing.T) {
	collector := NewCollector("/fake/path")
	
	snapshot := &Snapshot{
		Version: "test-version",
		ConfigDefaults: map[string]interface{}{
			"test.param": "test-value",
		},
	}
	
	// Create a temporary file
	tmpFile, err := os.CreateTemp("", "tikv_test_snapshot.json")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()
	
	// Test saving the snapshot
	err = collector.Save(snapshot, tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to save snapshot: %v", err)
	}
	
	// Check that file was created
	if _, err := os.Stat(tmpFile.Name()); err != nil {
		t.Errorf("Snapshot file was not created: %v", err)
	}
}