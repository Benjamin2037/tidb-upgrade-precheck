package precheck

import (
	"context"
	"testing"
	"time"

	"github.com/pingcap/tidb-upgrade-precheck/pkg/runtime"
)

func TestAnalyzer(t *testing.T) {
	// Create a simple rule for testing
	rule := NewRuleFunc("test-rule", func(ctx context.Context, snapshot Snapshot) ([]ReportItem, error) {
		return []ReportItem{
			{
				Rule:     "test-rule",
				Severity: SeverityInfo,
				Message:  "Test message",
			},
		}, nil
	})

	// Create analyzer with the test rule
	analyzer := NewAnalyzer(rule)

	// Create a test snapshot
	snapshot := &runtime.ClusterSnapshot{
		Timestamp: time.Now(),
		Components: map[string]runtime.ComponentState{
			"tidb": {
				Type:    "tidb",
				Version: "v6.5.0",
				Config: map[string]interface{}{
					"performance.max-procs": 0,
				},
				Variables: map[string]string{
					"tidb_enable_clustered_index": "ON",
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
}

func TestGetClusterVersion(t *testing.T) {
	tests := []struct {
		name     string
		snapshot *runtime.ClusterSnapshot
		expected string
	}{
		{
			name: "TiDB version available",
			snapshot: &runtime.ClusterSnapshot{
				Components: map[string]runtime.ComponentState{
					"tidb": {
						Version: "v6.5.0",
					},
				},
			},
			expected: "v6.5.0",
		},
		{
			name: "Other component version",
			snapshot: &runtime.ClusterSnapshot{
				Components: map[string]runtime.ComponentState{
					"tikv": {
						Version: "v6.5.0",
					},
				},
			},
			expected: "v6.5.0",
		},
		{
			name: "No version available",
			snapshot: &runtime.ClusterSnapshot{
				Components: map[string]runtime.ComponentState{},
			},
			expected: "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			version := getClusterVersion(tt.snapshot)
			if version != tt.expected {
				t.Errorf("Expected version %s, got %s", tt.expected, version)
			}
		})
	}
}