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
	"fmt"
)

// Analyzer is responsible for analyzing TiKV parameter changes and providing recommendations
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

// AnalysisResult represents the result of a TiKV parameter analysis
type AnalysisResult struct {
	// Report is the comparison report
	Report *ComparisonReport `json:"report"`
	
	// Recommendations are the recommendations for handling the changes
	Recommendations []Recommendation `json:"recommendations"`
	
	// OverallRisk is the overall risk level of the upgrade
	OverallRisk string `json:"overall_risk"`
}

// NewAnalyzer creates a new TiKV analyzer
func NewAnalyzer(sourcePath string) *Analyzer {
	return &Analyzer{
		comparator: NewComparator(sourcePath),
	}
}

// Analyze analyzes TiKV parameter changes between two versions
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
			AffectedComponents: []string{"tikv"},
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
		case "storage.reserve-space":
			recommendation.Recommendation = "This parameter controls how much disk space is reserved for non-TiKV purposes. Setting it to 0 disables the reservation mechanism. Make sure you have adequate monitoring for disk usage if changing this parameter."
			recommendation.AffectedComponents = []string{"tikv"}
		case "raftstore.raft-entry-max-size":
			recommendation.Recommendation = "This parameter affects the maximum size of a Raft log entry. Larger values may improve performance for large transactions but increase memory usage. Review your typical transaction sizes before changing."
			recommendation.AffectedComponents = []string{"tikv"}
		case "rocksdb.defaultcf.block-cache-size":
			recommendation.Recommendation = "This parameter controls the amount of memory allocated for RocksDB block cache. Larger values improve read performance but reduce memory available for other operations. Adjust based on your read workload and total system memory."
			recommendation.AffectedComponents = []string{"tikv"}
		}
		
		recommendations = append(recommendations, recommendation)
	}
	
	// Handle feature gate changes
	for _, fg := range report.FeatureGates {
		recommendation := Recommendation{
			ParameterName: fmt.Sprintf("feature-gate:%s", fg.Name),
			RiskLevel:     fg.RiskLevel,
			Description:   fg.Description,
			AffectedComponents: []string{"tikv"},
		}
		
		switch fg.StatusTo {
		case "deprecated":
			recommendation.Recommendation = fmt.Sprintf("Feature gate '%s' is deprecated in the target version. Plan to migrate away from this feature.", fg.Name)
		case "removed":
			recommendation.Recommendation = fmt.Sprintf("Feature gate '%s' is removed in the target version. You must migrate away from this feature before upgrading.", fg.Name)
			recommendation.RiskLevel = "high"
		default:
			recommendation.Recommendation = fmt.Sprintf("Feature gate '%s' status changed from %s to %s.", fg.Name, fg.StatusFrom, fg.StatusTo)
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
	
	// Check feature gates
	for _, fg := range report.FeatureGates {
		fgName := fmt.Sprintf("feature-gate:%s", fg.Name)
		if fgName == paramName {
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