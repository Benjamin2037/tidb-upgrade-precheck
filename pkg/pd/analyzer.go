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
	"fmt"
)

// Analyzer is responsible for analyzing PD parameter changes and providing recommendations
type Analyzer struct {
	comparator *Comparator
}

// Recommendation represents a recommendation for handling a parameter change
type Recommendation struct {
	// ParameterName is the name of the parameter
	ParameterName string `json:"parameter_name"`
	
	// RiskLevel is the risk level of the change
	RiskLevel string `json:"risk_level"`
	
	// Description is a description of the change
	Description string `json:"description"`
	
	// Recommendation is the recommended action
	Recommendation string `json:"recommendation"`
	
	// AffectedComponents are the components affected by this change
	AffectedComponents []string `json:"affected_components"`
}

// AnalysisResult represents the result of a PD parameter analysis
type AnalysisResult struct {
	// Report is the comparison report
	Report *ComparisonReport `json:"report"`
	
	// Recommendations are the recommendations for handling the changes
	Recommendations []Recommendation `json:"recommendations"`
	
	// OverallRisk is the overall risk level of the upgrade
	OverallRisk string `json:"overall_risk"`
}

// NewAnalyzer creates a new PD analyzer
func NewAnalyzer(sourcePath string) *Analyzer {
	return &Analyzer{
		comparator: NewComparator(sourcePath),
	}
}

// Analyze analyzes PD parameter changes between two versions
func (a *Analyzer) Analyze(fromVersion, toVersion string) (*AnalysisResult, error) {
	report, err := a.comparator.Compare(fromVersion, toVersion)
	if err != nil {
		return nil, fmt.Errorf("failed to compare parameters: %v", err)
	}
	
	recommendations := a.generateRecommendations(report)
	overallRisk := a.assessOverallRisk(report)
	
	return &AnalysisResult{
		Report:         report,
		Recommendations: recommendations,
		OverallRisk:    overallRisk,
	}, nil
}

// generateRecommendations generates recommendations for handling parameter changes
func (a *Analyzer) generateRecommendations(report *ComparisonReport) []Recommendation {
	recommendations := []Recommendation{}
	
	for _, change := range report.Parameters {
		recommendation := Recommendation{
			ParameterName: change.Name,
			RiskLevel:     change.RiskLevel,
			Description:   change.Description,
			AffectedComponents: []string{"pd"},
		}
		
		switch change.RiskLevel {
		case "high":
			recommendation.Recommendation = "Carefully review this parameter change. Test thoroughly in a staging environment before applying to production."
		case "medium":
			recommendation.Recommendation = "Review this parameter change and consider if any action is needed based on your cluster configuration."
		case "low":
			recommendation.Recommendation = "This is a low-impact change. Review for completeness but no action is typically required."
		case "info":
			recommendation.Recommendation = "Informational change. No action required."
		}
		
		// Special handling for specific parameters
		switch change.Name {
		case "schedule.max-store-down-time":
			recommendation.Recommendation = "This parameter affects how PD treats down stores. Increasing this value gives more time for recovery but may delay failover. Decreasing it speeds up failover but may cause unnecessary replica creation. Review your typical store downtime patterns."
			recommendation.AffectedComponents = []string{"pd", "tikv"}
		case "replication.location-labels":
			recommendation.Recommendation = "Changing this parameter affects how PD schedules replicas for disaster recovery. Ensure your deployment topology matches the new label configuration."
			recommendation.AffectedComponents = []string{"pd", "tikv"}
		}
		
		recommendations = append(recommendations, recommendation)
	}
	
	return recommendations
}

// assessOverallRisk assesses the overall risk level of the upgrade
func (a *Analyzer) assessOverallRisk(report *ComparisonReport) string {
	// Simple algorithm: if there are any high-risk changes, overall risk is high
	// If there are medium-risk changes but no high-risk changes, overall risk is medium
	// Otherwise, overall risk is low
	
	if report.Summary.HighRisk > 0 {
		return "high"
	}
	
	if report.Summary.MediumRisk > 0 {
		return "medium"
	}
	
	return "low"
}

// GetParameterImpact analyzes the impact of a specific parameter change
func (a *Analyzer) GetParameterImpact(versionFrom, versionTo, paramName string) (*Recommendation, error) {
	report, err := a.comparator.Compare(versionFrom, versionTo)
	if err != nil {
		return nil, fmt.Errorf("failed to compare parameters: %v", err)
	}
	
	for _, change := range report.Parameters {
		if change.Name == paramName {
			recommendations := a.generateRecommendations(report)
			for _, rec := range recommendations {
				if rec.ParameterName == paramName {
					return &rec, nil
				}
			}
		}
	}
	
	return nil, fmt.Errorf("parameter %s not found in the comparison report", paramName)
}