package precheck

import (
	"context"
	"testing"
	"time"

	"github.com/pingcap/tidb-upgrade-precheck/pkg/runtime"
)

func TestFactory_CreateAnalyzerWithKB(t *testing.T) {
	// Create mock knowledge base data
	sourceKB := map[string]interface{}{
		"system_variables": map[string]interface{}{
			"tidb_enable_clustered_index": "INT_ONLY",
			"max_connections":             "151",
		},
		"config_defaults": map[string]interface{}{
			"performance.max-procs": 0,
			"log.level":             "info",
		},
	}

	targetKB := map[string]interface{}{
		"system_variables": map[string]interface{}{
			"tidb_enable_clustered_index": "ON",
			"max_connections":             "151",
		},
		"config_defaults": map[string]interface{}{
			"performance.max-procs": 0,
			"log.level":             "info",
		},
		"upgrade_logic": []interface{}{
			map[string]interface{}{
				"variable":     "tidb_enable_clustered_index",
				"forced_value": "ON",
				"type":         "set_global",
			},
		},
	}

	// Create factory and analyzer
	factory := NewFactory()
	analyzer := factory.CreateAnalyzerWithKB(sourceKB, targetKB)

	// Create a test snapshot
	snapshot := &runtime.ClusterSnapshot{
		Timestamp: time.Now(),
		Components: map[string]runtime.ComponentState{
			"tidb": {
				Type:    "tidb",
				Version: "v6.5.0",
				Config: map[string]interface{}{
					"performance.max-procs": 4,
				},
				Variables: map[string]string{
					"tidb_enable_clustered_index": "ON",
					"max_connections":             "200",
				},
				Status: make(map[string]interface{}),
			},
		},
	}

	// Run analysis
	ctx := context.Background()
	report, err := analyzer.Analyze(ctx, snapshot, "v7.0.0")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Check report
	if report == nil {
		t.Fatal("Expected report, got nil")
	}

	// Check that we have items
	if len(report.Items) == 0 {
		t.Error("Expected report items")
	}

	// Check summary
	if report.Summary.Total == 0 {
		t.Error("Expected summary total to be greater than 0")
	}

	// Check for specific items
	var hasForcedChange bool
	var hasConfigCustomValue bool

	for _, item := range report.Items {
		if item.Rule == "sysvar-check" && item.Severity == SeverityBlocker {
			hasForcedChange = true
		}
		if item.Rule == "config-check" && item.Severity == SeverityInfo {
			hasConfigCustomValue = true
		}
	}

	if !hasForcedChange {
		t.Error("Expected to find forced change item")
	}

	if !hasConfigCustomValue {
		t.Error("Expected to find config custom value item")
	}
}

func TestGetForcedChanges(t *testing.T) {
	targetKB := map[string]interface{}{
		"upgrade_logic": []interface{}{
			map[string]interface{}{
				"variable":     "tidb_enable_clustered_index",
				"forced_value": "ON",
				"type":         "set_global",
			},
		},
	}

	forcedChanges := getForcedChanges(targetKB)

	if len(forcedChanges) != 1 {
		t.Fatalf("Expected 1 forced change, got %d", len(forcedChanges))
	}

	value, exists := forcedChanges["tidb_enable_clustered_index"]
	if !exists {
		t.Fatal("Expected tidb_enable_clustered_index in forced changes")
	}

	if value != "ON" {
		t.Errorf("Expected forced value to be ON, got %v", value)
	}
}