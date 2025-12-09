package kbgenerator

import (
	"os"
	"strings"
	"testing"
)

func TestCollectFromPDSource(t *testing.T) {
	// This is a basic test that just verifies the function structure
	// In a real test, we would use a mock PD repository
	
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "pd_kb_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)
	
	// Test with a non-existent repository (should fail)
	_, err = CollectFromPDSource("/non/existent/path", "v6.5.0")
	if err == nil {
		t.Error("Expected error when collecting from non-existent path")
	}
}

func TestParsePDConfigDefaults(t *testing.T) {
	// Test the parsing function with a non-existent file
	// This should return the default mock data
	defaults := parsePDConfigDefaults("/non/existent/path")
	
	if defaults == nil {
		t.Error("parsePDConfigDefaults returned nil")
	}
	if len(defaults) == 0 {
		t.Error("parsePDConfigDefaults returned empty map")
	}
	
	// Check some expected keys
	if _, exists := defaults["schedule.max-store-down-time"]; !exists {
		t.Error("defaults should contain schedule.max-store-down-time")
	}
	if _, exists := defaults["replication.max-replicas"]; !exists {
		t.Error("defaults should contain replication.max-replicas")
	}
	if _, exists := defaults["log.level"]; !exists {
		t.Error("defaults should contain log.level")
	}
}

func TestSavePDKBSnapshot(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "pd_kb_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)
	
	snapshot := &PDKBSnapshot{
		Version: "test-version",
		ConfigDefaults: map[string]interface{}{
			"test.param": "test-value",
		},
		BootstrapVersion: 1,
	}
	
	outputPath := tempDir + "/test_snapshot.json"
	err = SavePDKBSnapshot(snapshot, outputPath)
	if err != nil {
		t.Errorf("SavePDKBSnapshot failed: %v", err)
	}
	
	// Check that file was created
	_, err = os.Stat(outputPath)
	if err != nil {
		t.Errorf("Snapshot file was not created: %v", err)
	}
}

func TestComparePDConfigurations(t *testing.T) {
	from := map[string]interface{}{
		"param1": "value1",
		"param2": "value2",
		"param3": "old_value",
	}
	
	to := map[string]interface{}{
		"param2": "value2",     // unchanged
		"param3": "new_value",  // modified
		"param4": "value4",     // added
	}
	
	changes := comparePDConfigurations(from, to)
	if len(changes) != 3 {
		t.Errorf("Expected 3 changes, got %d", len(changes))
	}
	
	addedFound := false
	removedFound := false
	modifiedFound := false
	
	for _, change := range changes {
		if changeMap, ok := change.(map[string]interface{}); ok {
			changeType := changeMap["type"].(string)
			key := changeMap["key"].(string)
			
			switch changeType {
			case "added":
				if key == "param4" {
					addedFound = true
				}
			case "removed":
				if key == "param1" {
					removedFound = true
				}
			case "modified":
				if key == "param3" {
					modifiedFound = true
				}
			}
		}
	}
	
	if !addedFound {
		t.Error("Should find added parameter")
	}
	if !removedFound {
		t.Error("Should find removed parameter")
	}
	if !modifiedFound {
		t.Error("Should find modified parameter")
	}
}

func TestGeneratePDUpgradeScript(t *testing.T) {
	changes := []interface{}{
		map[string]interface{}{
			"type": "added",
			"key":  "new_param",
		},
		map[string]interface{}{
			"type": "removed",
			"key":  "old_param",
		},
	}
	
	script := GeneratePDUpgradeScript(changes)
	if !strings.Contains(script, "#!/bin/bash") {
		t.Error("Script should contain #!/bin/bash")
	}
	if !strings.Contains(script, "new_param") {
		t.Error("Script should contain new_param")
	}
	if !strings.Contains(script, "old_param") {
		t.Error("Script should contain old_param")
	}
}

func TestGetPDParameterChanges(t *testing.T) {
	// 创建临时参数历史文件用于测试
	tempHistory := `{
		"parameters": [
			{
				"name": "schedule.enable-diagnostic",
				"type": "bool",
				"history": [
					{
						"version": "v6.5.0",
						"default": false,
						"description": ""
					},
					{
						"version": "v7.1.0",
						"default": true,
						"description": ""
					}
				]
			},
			{
				"name": "schedule.enable-joint-consensus",
				"type": "bool",
				"history": [
					{
						"version": "v6.5.0",
						"default": true,
						"description": ""
					}
				]
			}
		]
	}`

	// 写入临时文件
	tempFile, err := os.CreateTemp("", "pd_parameters_history_*.json")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tempFile.Name())

	if _, err := tempFile.Write([]byte(tempHistory)); err != nil {
		t.Fatalf("Failed to write to temp file: %v", err)
	}
	tempFile.Close()

	// 测试获取参数变更
	changes, err := GetPDParameterChanges(tempFile.Name(), "v6.5.0", "v7.1.0")
	if err != nil {
		t.Fatalf("Failed to get parameter changes: %v", err)
	}

	// 验证结果 - 应该有2个变更：1个修改的参数和1个被移除的参数
	if len(changes) != 2 {
		t.Errorf("Expected 2 changes, got %d", len(changes))
		t.Logf("Changes: %+v", changes)
		return
	}

	// 查找修改的参数
	modifiedFound := false
	for _, change := range changes {
		if change.Name == "schedule.enable-diagnostic" && change.Type == "modified" {
			modifiedFound = true
			if change.FromValue != false {
				t.Errorf("Expected from value false, got %v", change.FromValue)
			}
			if change.ToValue != true {
				t.Errorf("Expected to value true, got %v", change.ToValue)
			}
		}
	}

	if !modifiedFound {
		t.Error("Expected to find modified parameter 'schedule.enable-diagnostic'")
	}
}

func TestGetPDParameterChangesAddedRemoved(t *testing.T) {
	// 创建临时参数历史文件用于测试添加和删除的参数
	tempHistory := `{
		"parameters": [
			{
				"name": "schedule.existing-param",
				"type": "bool",
				"history": [
					{
						"version": "v6.5.0",
						"default": true,
						"description": ""
					},
					{
						"version": "v7.1.0",
						"default": true,
						"description": ""
					}
				]
			},
			{
				"name": "schedule.added-param",
				"type": "bool",
				"history": [
					{
						"version": "v7.1.0",
						"default": false,
						"description": ""
					}
				]
			},
			{
				"name": "schedule.removed-param",
				"type": "bool",
				"history": [
					{
						"version": "v6.5.0",
						"default": true,
						"description": ""
					}
				]
			}
		]
	}`

	// 写入临时文件
	tempFile, err := os.CreateTemp("", "pd_parameters_history_*.json")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tempFile.Name())

	if _, err := tempFile.Write([]byte(tempHistory)); err != nil {
		t.Fatalf("Failed to write to temp file: %v", err)
	}
	tempFile.Close()

	// 测试获取参数变更
	changes, err := GetPDParameterChanges(tempFile.Name(), "v6.5.0", "v7.1.0")
	if err != nil {
		t.Fatalf("Failed to get parameter changes: %v", err)
	}

	// 验证结果
	if len(changes) != 2 {
		t.Errorf("Expected 2 changes, got %d", len(changes))
		t.Logf("Changes: %+v", changes)
	}

	// 查找添加和删除的参数
	var addedChange, removedChange *PDParameterChange
	for _, change := range changes {
		if change.Type == "added" {
			addedChange = &change
		} else if change.Type == "removed" {
			removedChange = &change
		}
	}

	if addedChange == nil {
		t.Error("Expected to find added parameter change")
	} else {
		if addedChange.Name != "schedule.added-param" {
			t.Errorf("Expected added parameter name 'schedule.added-param', got '%s'", addedChange.Name)
		}
		if addedChange.FromValue != nil {
			t.Errorf("Expected added parameter from value nil, got %v", addedChange.FromValue)
		}
		if addedChange.ToValue != false {
			t.Errorf("Expected added parameter to value false, got %v", addedChange.ToValue)
		}
	}

	if removedChange == nil {
		t.Error("Expected to find removed parameter change")
	} else {
		if removedChange.Name != "schedule.removed-param" {
			t.Errorf("Expected removed parameter name 'schedule.removed-param', got '%s'", removedChange.Name)
		}
		if removedChange.FromValue != true {
			t.Errorf("Expected removed parameter from value true, got %v", removedChange.FromValue)
		}
		if removedChange.ToValue != nil {
			t.Errorf("Expected removed parameter to value nil, got %v", removedChange.ToValue)
		}
	}
}