package precheck

import (
	"testing"

	"github.com/pingcap/tidb-upgrade-precheck/pkg/runtime"
)

func TestParamAnalyzer_AnalyzeParameterState(t *testing.T) {
	analyzer := NewParamAnalyzer()

	// Create a test snapshot
	snapshot := &runtime.ClusterSnapshot{
		Components: map[string]runtime.ComponentState{
			"tidb": {
				Type:    "tidb",
				Version: "v6.5.0",
				Config: map[string]interface{}{
					"performance.max-procs": 4, // User modified
				},
				Variables: map[string]string{
					"tidb_enable_clustered_index": "ON",  // User modified
					"max_connections":             "151", // Default value
				},
				Status: make(map[string]interface{}),
			},
		},
	}

	// Create source knowledge base
	sourceKB := map[string]interface{}{
		"config_defaults": map[string]interface{}{
			"performance.max-procs": 0, // Default value
		},
		"system_variables": map[string]interface{}{
			"tidb_enable_clustered_index": "INT_ONLY", // Default value
			"max_connections":             "151",       // Default value
		},
	}

	// Create target knowledge base
	targetKB := map[string]interface{}{
		"config_defaults": map[string]interface{}{
			"performance.max-procs": 0, // Same default
		},
		"system_variables": map[string]interface{}{
			"tidb_enable_clustered_index": "ON", // Changed default
			"max_connections":             "151", // Same default
		},
	}

	// Analyze parameter state
	analyses, err := analyzer.AnalyzeParameterState(snapshot, sourceKB, targetKB)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Check that we have analyses
	if len(analyses) == 0 {
		t.Fatal("Expected analyses, got none")
	}

	// Find specific parameters
	var maxProcsAnalysis *ParameterAnalysis
	var clusteredIndexAnalysis *ParameterAnalysis
	var maxConnectionsAnalysis *ParameterAnalysis

	for _, analysis := range analyses {
		switch analysis.Name {
		case "performance.max-procs":
			maxProcsAnalysis = analysis
		case "tidb_enable_clustered_index":
			clusteredIndexAnalysis = analysis
		case "max_connections":
			maxConnectionsAnalysis = analysis
		}
	}

	// Check performance.max-procs analysis
	if maxProcsAnalysis == nil {
		t.Error("Expected analysis for performance.max-procs")
	} else {
		if maxProcsAnalysis.State != UserSet {
			t.Errorf("Expected performance.max-procs to be UserSet, got %s", maxProcsAnalysis.State)
		}
		if maxProcsAnalysis.CurrentValue != 4 {
			t.Errorf("Expected performance.max-procs current value to be 4, got %v", maxProcsAnalysis.CurrentValue)
		}
		if maxProcsAnalysis.SourceDefault != 0 {
			t.Errorf("Expected performance.max-procs source default to be 0, got %v", maxProcsAnalysis.SourceDefault)
		}
	}

	// Check tidb_enable_clustered_index analysis
	if clusteredIndexAnalysis == nil {
		t.Error("Expected analysis for tidb_enable_clustered_index")
	} else {
		if clusteredIndexAnalysis.State != UserSet {
			t.Errorf("Expected tidb_enable_clustered_index to be UserSet, got %s", clusteredIndexAnalysis.State)
		}
		if clusteredIndexAnalysis.CurrentValue != "ON" {
			t.Errorf("Expected tidb_enable_clustered_index current value to be ON, got %v", clusteredIndexAnalysis.CurrentValue)
		}
		if clusteredIndexAnalysis.SourceDefault != "INT_ONLY" {
			t.Errorf("Expected tidb_enable_clustered_index source default to be INT_ONLY, got %v", clusteredIndexAnalysis.SourceDefault)
		}
		if clusteredIndexAnalysis.TargetDefault != "ON" {
			t.Errorf("Expected tidb_enable_clustered_index target default to be ON, got %v", clusteredIndexAnalysis.TargetDefault)
		}
	}

	// Check max_connections analysis
	if maxConnectionsAnalysis == nil {
		t.Error("Expected analysis for max_connections")
	} else {
		if maxConnectionsAnalysis.State != UseDefault {
			t.Errorf("Expected max_connections to be UseDefault, got %s", maxConnectionsAnalysis.State)
		}
		if maxConnectionsAnalysis.CurrentValue != "151" {
			t.Errorf("Expected max_connections current value to be 151, got %v", maxConnectionsAnalysis.CurrentValue)
		}
	}
}

func TestParamAnalyzer_IdentifyRisks(t *testing.T) {
	analyzer := NewParamAnalyzer()

	// Create test analyses
	analyses := []*ParameterAnalysis{
		{
			Name:          "tidb_enable_clustered_index",
			Component:     "tidb",
			CurrentValue:  "INT_ONLY",
			SourceDefault: "INT_ONLY",
			TargetDefault: "ON",
			State:         UseDefault,
		},
		{
			Name:          "performance.max-procs",
			Component:     "tidb",
			CurrentValue:  4,
			SourceDefault: 0,
			TargetDefault: 0,
			State:         UserSet,
		},
	}

	// Create forced changes
	forcedChanges := map[string]interface{}{
		"tidb_enable_clustered_index": "ON",
	}

	// Identify risks
	risks := analyzer.IdentifyRisks(analyses, forcedChanges)

	// Check that we have risks
	if len(risks) == 0 {
		t.Fatal("Expected risks, got none")
	}

	// Check specific risks
	var clusteredIndexRisk *RiskItem
	var maxProcsRisk *RiskItem

	for _, risk := range risks {
		switch risk.Parameter {
		case "tidb_enable_clustered_index":
			clusteredIndexRisk = risk
		case "performance.max-procs":
			maxProcsRisk = risk
		}
	}

	// Check tidb_enable_clustered_index risk (HIGH - forced change)
	if clusteredIndexRisk == nil {
		t.Error("Expected risk for tidb_enable_clustered_index")
	} else {
		if clusteredIndexRisk.Level != RiskHigh {
			t.Errorf("Expected tidb_enable_clustered_index risk level to be HIGH, got %s", clusteredIndexRisk.Level)
		}
	}

	// Check performance.max-procs risk (MEDIUM - user set with default change)
	// Note: This should actually be INFO since the default didn't change, let me fix the test
	if maxProcsRisk != nil {
		if maxProcsRisk.Level != RiskInfo {
			t.Errorf("Expected performance.max-procs risk level to be INFO, got %s", maxProcsRisk.Level)
		}
	}
}