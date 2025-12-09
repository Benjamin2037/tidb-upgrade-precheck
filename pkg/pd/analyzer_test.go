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

package pd

import (
	"testing"
	
	"github.com/stretchr/testify/assert"
)

func TestAnalyzer_Analyze(t *testing.T) {
	analyzer := NewAnalyzer("")
	
	result, err := analyzer.Analyze("v6.5.0", "v7.1.0")
	assert.NoError(t, err)
	assert.NotNil(t, result)
	
	assert.NotNil(t, result.Report)
	assert.NotNil(t, result.Recommendations)
	assert.NotEmpty(t, result.OverallRisk)
}

func TestAnalyzer_generateRecommendations(t *testing.T) {
	analyzer := NewAnalyzer("")
	
	report := &ComparisonReport{
		Component:   "pd",
		VersionFrom: "v6.5.0",
		VersionTo:   "v7.1.0",
		Parameters: []ParameterChange{
			{
				Name:        "schedule.max-store-down-time",
				Type:        "duration",
				FromValue:   "30m",
				ToValue:     "1h",
				ChangeType:  "modified",
				RiskLevel:   "medium",
				Description: "Maximum downtime for a store before it is considered unavailable",
				Category:    "schedule",
			},
			{
				Name:        "log.level",
				Type:        "string",
				FromValue:   "info",
				ToValue:     "debug",
				ChangeType:  "modified",
				RiskLevel:   "low",
				Description: "Log level",
				Category:    "log",
			},
		},
	}
	
	recommendations := analyzer.generateRecommendations(report)
	assert.Len(t, recommendations, 2)
	
	// Check that special handling for max-store-down-time works
	foundSpecial := false
	for _, rec := range recommendations {
		if rec.ParameterName == "schedule.max-store-down-time" {
			assert.Contains(t, rec.Recommendation, "This parameter affects how PD treats down stores")
			assert.Contains(t, rec.AffectedComponents, "pd")
			assert.Contains(t, rec.AffectedComponents, "tikv")
			foundSpecial = true
		}
	}
	assert.True(t, foundSpecial, "Should find special recommendation for max-store-down-time")
}

func TestAnalyzer_assessOverallRisk(t *testing.T) {
	analyzer := NewAnalyzer("")
	
	// Test high risk
	reportHigh := &ComparisonReport{
		Summary: Summary{
			HighRisk: 1,
		},
	}
	
	risk := analyzer.assessOverallRisk(reportHigh)
	assert.Equal(t, "high", risk)
	
	// Test medium risk
	reportMedium := &ComparisonReport{
		Summary: Summary{
			MediumRisk: 1,
		},
	}
	
	risk = analyzer.assessOverallRisk(reportMedium)
	assert.Equal(t, "medium", risk)
	
	// Test low risk
	reportLow := &ComparisonReport{
		Summary: Summary{
			LowRisk: 1,
		},
	}
	
	risk = analyzer.assessOverallRisk(reportLow)
	assert.Equal(t, "low", risk)
}