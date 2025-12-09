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
	"testing"
)

func TestAnalyzer_Analyze(t *testing.T) {
	analyzer := NewAnalyzer("/fake/path")
	
	// Test analyzing between two versions
	result, err := analyzer.Analyze("v6.5.0", "v7.1.0")
	if err != nil {
		t.Fatalf("Failed to analyze: %v", err)
	}
	
	if result.Report.Component != "tikv" {
		t.Errorf("Expected component 'tikv', got %s", result.Report.Component)
	}
	
	if result.Report.VersionFrom != "v6.5.0" {
		t.Errorf("Expected versionFrom 'v6.5.0', got %s", result.Report.VersionFrom)
	}
	
	if result.Report.VersionTo != "v7.1.0" {
		t.Errorf("Expected versionTo 'v7.1.0', got %s", result.Report.VersionTo)
	}
	
	// Check that we have some parameters
	if len(result.Report.Parameters) == 0 {
		t.Error("Expected some parameters in the report")
	}
	
	// Check that we have recommendations
	if len(result.Recommendations) == 0 {
		t.Error("Expected some recommendations")
	}
}

func TestAnalyzer_GetParameterImpact(t *testing.T) {
	analyzer := NewAnalyzer("/fake/path")
	
	// Test getting impact for a specific parameter
	impact, err := analyzer.GetParameterImpact("v6.5.0", "v7.1.0", "storage.reserve-space")
	if err != nil {
		t.Fatalf("Failed to get parameter impact: %v", err)
	}
	
	if impact.ParameterName != "storage.reserve-space" {
		t.Errorf("Expected parameter name 'storage.reserve-space', got %s", impact.ParameterName)
	}
	
	if impact.RiskLevel != "high" {
		t.Errorf("Expected risk level 'high', got %s", impact.RiskLevel)
	}
}