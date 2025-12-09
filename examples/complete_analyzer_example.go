package main

import (
	"context"
	"fmt"
	"log"

	"github.com/pingcap/tidb-upgrade-precheck/pkg/precheck"
	"github.com/pingcap/tidb-upgrade-precheck/pkg/runtime"
)

func main() {
	// Create a mock cluster snapshot
	snapshot := createMockSnapshot()

	// Create mock knowledge base data
	sourceKB := createMockSourceKB()
	targetKB := createMockTargetKB()

	// Create analyzer with knowledge base
	factory := precheck.NewFactory()
	analyzer := factory.CreateAnalyzerWithKB(sourceKB, targetKB)

	// Analyze the snapshot
	ctx := context.Background()
	report, err := analyzer.Analyze(ctx, snapshot, "v7.0.0")
	if err != nil {
		log.Fatalf("Error analyzing snapshot: %v", err)
	}

	// Display results
	fmt.Println("=== Analysis Report ===")
	fmt.Printf("Analysis completed in: %v\n", report.FinishedAt.Sub(report.StartedAt))
	fmt.Printf("Total issues: %d\n", report.Summary.Total)
	fmt.Printf("Blocking issues: %d\n", report.Summary.Blocking)
	fmt.Printf("Warnings: %d\n", report.Summary.Warnings)
	fmt.Printf("Info items: %d\n", report.Summary.Infos)

	if len(report.Items) > 0 {
		fmt.Println("\n=== Detailed Issues ===")
		for _, item := range report.Items {
			fmt.Printf("[%s] %s: %s\n", item.Severity, item.Rule, item.Message)
			if len(item.Details) > 0 {
				for _, detail := range item.Details {
					fmt.Printf("  - %s\n", detail)
				}
			}
			fmt.Println()
		}
	}

	// Also demonstrate parameter analyzer directly
	fmt.Println("=== Parameter Analysis ===")
	paramAnalyzer := precheck.NewParamAnalyzer()
	analyses, err := paramAnalyzer.AnalyzeParameterState(snapshot, sourceKB, targetKB)
	if err != nil {
		log.Fatalf("Error analyzing parameters: %v", err)
	}

	forcedChanges := paramAnalyzer.GetForcedChanges(targetKB)
	risks := paramAnalyzer.IdentifyRisks(analyses, forcedChanges)

	if len(risks) > 0 {
		fmt.Printf("Identified %d risks:\n", len(risks))
		for _, risk := range risks {
			fmt.Printf("[%s] %s.%s: %s\n", risk.Level, risk.Component, risk.Parameter, risk.Message)
			fmt.Printf("  Details: %s\n", risk.Details)
		}
	} else {
		fmt.Println("No risks identified")
	}

	// Show parameter analysis details
	fmt.Println("\n=== Parameter Details ===")
	for _, analysis := range analyses {
		fmt.Printf("%s.%s: Current=%v, SourceDefault=%v, TargetDefault=%v, State=%s\n",
			analysis.Component, analysis.Name, analysis.CurrentValue,
			analysis.SourceDefault, analysis.TargetDefault, analysis.State)
	}
}

func createMockSnapshot() *runtime.ClusterSnapshot {
	return &runtime.ClusterSnapshot{
		Components: map[string]runtime.ComponentState{
			"tidb": {
				Type:    "tidb",
				Version: "v6.5.0",
				Config: map[string]interface{}{
					"performance.max-procs":     8,
					"log.level":                 "info",
					"prepared-plan-cache.enabled": true,
				},
				Variables: map[string]string{
					"tidb_enable_clustered_index": "ON",
					"max_connections":             "200",
					"tidb_mem_quota_query":       "1073741824", // 1GB
					"tidb_enable_async_commit":   "ON",
				},
				Status: make(map[string]interface{}),
			},
		},
	}
}

func createMockSourceKB() map[string]interface{} {
	return map[string]interface{}{
		"system_variables": map[string]interface{}{
			"tidb_enable_clustered_index": "INT_ONLY",
			"max_connections":             "151",
			"tidb_mem_quota_query":       "1073741824", // 1GB
			"tidb_enable_async_commit":   "OFF",
		},
		"config_defaults": map[string]interface{}{
			"performance.max-procs":     0,
			"log.level":                 "info",
			"prepared-plan-cache.enabled": false,
		},
	}
}

func createMockTargetKB() map[string]interface{} {
	return map[string]interface{}{
		"system_variables": map[string]interface{}{
			"tidb_enable_clustered_index": "ON", // Changed default
			"max_connections":             "151",
			"tidb_mem_quota_query":       "1073741824", // 1GB
			"tidb_enable_async_commit":   "ON",
		},
		"config_defaults": map[string]interface{}{
			"performance.max-procs":     0,
			"log.level":                 "info",
			"prepared-plan-cache.enabled": true, // Changed default
		},
		"upgrade_logic": []interface{}{
			map[string]interface{}{
				"variable":     "tidb_enable_clustered_index",
				"forced_value": "ON",
				"type":         "set_global",
			},
		},
	}
}