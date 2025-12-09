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

package reporter

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/pingcap/tidb-upgrade-precheck/pkg/analyzer"
	"github.com/pingcap/tidb-upgrade-precheck/pkg/runtime"
)

func TestReporter_GenerateUpgradeReport(t *testing.T) {
	report := &analyzer.AnalysisReport{
		Component:   analyzer.TiDBComponent,
		VersionFrom: "v6.5.0",
		VersionTo:   "v7.1.0",
		Parameters: []analyzer.ParameterChange{
			{
				Name:        "performance.run-auto-analyze",
				Type:        "bool",
				FromVersion: "v6.5.0",
				ToVersion:   "v7.1.0",
				FromValue:   false,
				ToValue:     true,
				Description: "Parameter performance.run-auto-analyze was modified from false to true",
				RiskLevel:   analyzer.LowRisk,
			},
		},
		Summary: analyzer.Summary{
			TotalChanges: 1,
			Added:        0,
			Removed:      0,
			Modified:     1,
			HighRisk:     0,
			MediumRisk:   0,
			LowRisk:      1,
			InfoLevel:    0,
		},
	}

	tests := []struct {
		format   ReportFormat
		expected string
	}{
		{JSONFormat, "application/json"},
		{TextFormat, "text/plain"},
		{HTMLFormat, "text/html"},
	}

	for _, test := range tests {
		reporter := NewReporter(test.format)
		data, err := reporter.GenerateUpgradeReport(report)
		if err != nil {
			t.Fatalf("Failed to generate %s report: %v", test.format, err)
		}

		if len(data) == 0 {
			t.Errorf("Expected non-empty %s report", test.format)
		}

		// Verify the content type based on format
		contentType := ""
		switch test.format {
		case JSONFormat:
			contentType = "application/json"
			// Verify it's valid JSON
			var result map[string]interface{}
			if err := json.Unmarshal(data, &result); err != nil {
				t.Errorf("Generated JSON is invalid: %v", err)
			}
		case TextFormat:
			contentType = "text/plain"
			if !strings.Contains(string(data), "UPGRADE ANALYSIS REPORT") {
				t.Errorf("Text report does not contain expected header")
			}
		case HTMLFormat:
			contentType = "text/html"
			if !strings.Contains(string(data), "<html>") {
				t.Errorf("HTML report does not contain expected HTML tag")
			}
		}

		// Just print the content type for verification
		t.Logf("Generated %s report with content type: %s", test.format, contentType)
	}
}

func TestReporter_GenerateClusterReport(t *testing.T) {
	report := &analyzer.ClusterAnalysisReport{
		Instances: []runtime.InstanceState{
			{
				Address: "127.0.0.1:4000",
				State: runtime.ComponentState{
					Type:    runtime.TiDBComponent,
					Version: "v6.5.0",
					Config: map[string]interface{}{
						"log.level": "info",
					},
					Status: map[string]interface{}{},
				},
			},
		},
		InconsistentConfigs: []analyzer.InconsistentConfig{
			{
				ParameterName: "log.level",
				Values: []analyzer.ParameterValue{
					{InstanceAddress: "127.0.0.1:4000", Value: "info"},
					{InstanceAddress: "127.0.0.1:4001", Value: "debug"},
				},
				RiskLevel:   analyzer.MediumRisk,
				Description: "Parameter log.level has different values across instances",
			},
		},
		Recommendations: []analyzer.Recommendation{
			{
				ParameterName:      "log.level",
				RiskLevel:          analyzer.MediumRisk,
				Description:        "Parameter log.level has different values across instances",
				Recommendation:     "Consider aligning the value of log.level across all instances for optimal performance and behavior consistency",
				AffectedComponents: []string{"cluster"},
			},
		},
	}

	tests := []struct {
		format   ReportFormat
		expected string
	}{
		{JSONFormat, "application/json"},
		{TextFormat, "text/plain"},
		{HTMLFormat, "text/html"},
	}

	for _, test := range tests {
		reporter := NewReporter(test.format)
		data, err := reporter.GenerateClusterReport(report)
		if err != nil {
			t.Fatalf("Failed to generate %s report: %v", test.format, err)
		}

		if len(data) == 0 {
			t.Errorf("Expected non-empty %s report", test.format)
		}

		// Verify the content type based on format
		contentType := ""
		switch test.format {
		case JSONFormat:
			contentType = "application/json"
			// Verify it's valid JSON
			var result map[string]interface{}
			if err := json.Unmarshal(data, &result); err != nil {
				t.Errorf("Generated JSON is invalid: %v", err)
			}
		case TextFormat:
			contentType = "text/plain"
			if !strings.Contains(string(data), "CLUSTER CONFIGURATION ANALYSIS REPORT") {
				t.Errorf("Text report does not contain expected header")
			}
		case HTMLFormat:
			contentType = "text/html"
			if !strings.Contains(string(data), "<html>") {
				t.Errorf("HTML report does not contain expected HTML tag")
			}
		}

		// Just print the content type for verification
		t.Logf("Generated %s report with content type: %s", test.format, contentType)
	}
}

func TestReporter_UnsupportedFormat(t *testing.T) {
	reporter := &Reporter{format: "unsupported"}

	upgradeReport := &analyzer.AnalysisReport{}
	_, err := reporter.GenerateUpgradeReport(upgradeReport)
	if err == nil {
		t.Error("Expected error for unsupported format")
	}

	clusterReport := &analyzer.ClusterAnalysisReport{}
	_, err = reporter.GenerateClusterReport(clusterReport)
	if err == nil {
		t.Error("Expected error for unsupported format")
	}
}