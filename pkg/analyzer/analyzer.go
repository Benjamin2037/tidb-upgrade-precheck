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

package analyzer

import (
	"fmt"
	"path/filepath"

	"github.com/pingcap/tidb-upgrade-precheck/pkg/kbgenerator"
	"github.com/pingcap/tidb-upgrade-precheck/pkg/runtime"
)

// ComponentType represents the type of a TiDB cluster component
type ComponentType string

const (
	// TiDBComponent represents a TiDB component
	TiDBComponent ComponentType = "tidb"
	// PDComponent represents a PD component
	PDComponent ComponentType = "pd"
	// TiKVComponent represents a TiKV component
	TiKVComponent ComponentType = "tikv"
)

// RiskLevel represents the risk level of a change or inconsistency
type RiskLevel string

const (
	// HighRisk represents a high risk change
	HighRisk RiskLevel = "high"
	// MediumRisk represents a medium risk change
	MediumRisk = "medium"
	// LowRisk represents a low risk change
	LowRisk = "low"
	// InfoLevel represents an informational change
	InfoLevel = "info"
)

// ParameterChange represents a parameter change between versions
type ParameterChange struct {
	// Name is the name of the parameter
	Name string `json:"name"`
	// Type is the type of the parameter
	Type string `json:"type"`
	// FromVersion is the source version
	FromVersion string `json:"from_version"`
	// ToVersion is the target version
	ToVersion string `json:"to_version"`
	// FromValue is the value in the source version
	FromValue interface{} `json:"from_value"`
	// ToValue is the value in the target version
	ToValue interface{} `json:"to_value"`
	// Description is a description of the change
	Description string `json:"description"`
	// RiskLevel is the risk level of the change
	RiskLevel RiskLevel `json:"risk_level"`
}

// InconsistentConfig represents an inconsistent configuration across instances
type InconsistentConfig struct {
	// ParameterName is the name of the parameter
	ParameterName string `json:"parameter_name"`
	// Values are the different values of the parameter across instances
	Values []ParameterValue `json:"values"`
	// RiskLevel is the risk level of the inconsistency
	RiskLevel RiskLevel `json:"risk_level"`
	// Description is a description of the inconsistency
	Description string `json:"description"`
}

// ParameterValue represents a parameter value on a specific instance
type ParameterValue struct {
	// InstanceAddress is the address of the instance
	InstanceAddress string `json:"instance_address"`
	// Value is the value of the parameter
	Value interface{} `json:"value"`
}

// Recommendation represents a recommendation for handling a change or inconsistency
type Recommendation struct {
	// ParameterName is the name of the parameter
	ParameterName string `json:"parameter_name"`
	// RiskLevel is the risk level of the recommendation
	RiskLevel RiskLevel `json:"risk_level"`
	// Description is a description of the recommendation
	Description string `json:"description"`
	// Recommendation is the recommended action
	Recommendation string `json:"recommendation"`
	// AffectedComponents are the components affected by this recommendation
	AffectedComponents []string `json:"affected_components"`
}

// AnalysisReport represents a report of the analysis
type AnalysisReport struct {
	// Component is the component being analyzed
	Component ComponentType `json:"component"`
	// VersionFrom is the source version
	VersionFrom string `json:"version_from"`
	// VersionTo is the target version
	VersionTo string `json:"version_to"`
	// Parameters is the list of parameter changes
	Parameters []ParameterChange `json:"parameters"`
	// Summary contains summary information
	Summary Summary `json:"summary"`
}

// ClusterAnalysisReport represents a report of cluster configuration analysis
type ClusterAnalysisReport struct {
	// Instances is the list of instances in the cluster
	Instances []runtime.InstanceState `json:"instances"`
	// InconsistentConfigs are the configurations that are inconsistent across instances
	InconsistentConfigs []InconsistentConfig `json:"inconsistent_configs"`
	// Recommendations are the recommendations for handling inconsistencies
	Recommendations []Recommendation `json:"recommendations"`
}

// Summary contains summary information about the analysis
type Summary struct {
	// TotalChanges is the total number of changes
	TotalChanges int `json:"total_changes"`
	// Added is the number of added parameters
	Added int `json:"added"`
	// Removed is the number of removed parameters
	Removed int `json:"removed"`
	// Modified is the number of modified parameters
	Modified int `json:"modified"`
	// HighRisk is the number of high risk changes
	HighRisk int `json:"high_risk"`
	// MediumRisk is the number of medium risk changes
	MediumRisk int `json:"medium_risk"`
	// LowRisk is the number of low risk changes
	LowRisk int `json:"low_risk"`
	// InfoLevel is the number of info level changes
	InfoLevel int `json:"info_level"`
}

// Analyzer is responsible for analyzing parameter changes and cluster configurations
type Analyzer struct {
	// knowledgeBasePath is the path to the knowledge base
	knowledgeBasePath string
}

// NewAnalyzer creates a new analyzer
func NewAnalyzer(knowledgeBasePath string) *Analyzer {
	return &Analyzer{
		knowledgeBasePath: knowledgeBasePath,
	}
}

// AnalyzeUpgrade analyzes parameter changes between two versions of a component
func (a *Analyzer) AnalyzeUpgrade(component ComponentType, fromVersion, toVersion string) (*AnalysisReport, error) {
	switch component {
	case TiDBComponent:
		return a.analyzeTiDBUpgrade(fromVersion, toVersion)
	case PDComponent:
		return a.analyzePDUpgrade(fromVersion, toVersion)
	case TiKVComponent:
		return a.analyzeTiKVUpgrade(fromVersion, toVersion)
	default:
		return nil, fmt.Errorf("unsupported component type: %s", component)
	}
}

// AnalyzeCluster analyzes the configuration consistency of a cluster
func (a *Analyzer) AnalyzeCluster(clusterState *runtime.ClusterState) (*ClusterAnalysisReport, error) {
	// Group instances by component type
	tidbInstances := []runtime.InstanceState{}
	pdInstances := []runtime.InstanceState{}
	tikvInstances := []runtime.InstanceState{}

	for _, instance := range clusterState.Instances {
		switch instance.State.Type {
		case runtime.TiDBComponent:
			tidbInstances = append(tidbInstances, instance)
		case runtime.PDComponent:
			pdInstances = append(pdInstances, instance)
		case runtime.TiKVComponent:
			tikvInstances = append(tikvInstances, instance)
		}
	}

	// Analyze each component type
	var allInconsistentConfigs []InconsistentConfig
	var allRecommendations []Recommendation

	// Analyze TiDB instances
	if len(tidbInstances) > 1 {
		inconsistentConfigs := a.findInconsistentConfigs(tidbInstances)
		allInconsistentConfigs = append(allInconsistentConfigs, inconsistentConfigs...)
		
		recommendations := a.generateRecommendations(inconsistentConfigs)
		allRecommendations = append(allRecommendations, recommendations...)
	}

	// Analyze PD instances
	if len(pdInstances) > 1 {
		inconsistentConfigs := a.findInconsistentConfigs(pdInstances)
		allInconsistentConfigs = append(allInconsistentConfigs, inconsistentConfigs...)
		
		recommendations := a.generateRecommendations(inconsistentConfigs)
		allRecommendations = append(allRecommendations, recommendations...)
	}

	// Analyze TiKV instances
	if len(tikvInstances) > 1 {
		inconsistentConfigs := a.findInconsistentConfigs(tikvInstances)
		allInconsistentConfigs = append(allInconsistentConfigs, inconsistentConfigs...)
		
		recommendations := a.generateRecommendations(inconsistentConfigs)
		allRecommendations = append(allRecommendations, recommendations...)
	}

	report := &ClusterAnalysisReport{
		Instances:           clusterState.Instances,
		InconsistentConfigs: allInconsistentConfigs,
		Recommendations:     allRecommendations,
	}

	return report, nil
}

// analyzeTiDBUpgrade analyzes parameter changes between two TiDB versions
func (a *Analyzer) analyzeTiDBUpgrade(fromVersion, toVersion string) (*AnalysisReport, error) {
	// This is a placeholder implementation
	// A real implementation would compare the parameters between the two versions
	report := &AnalysisReport{
		Component:   TiDBComponent,
		VersionFrom: fromVersion,
		VersionTo:   toVersion,
		Parameters:  []ParameterChange{},
		Summary: Summary{
			TotalChanges: 0,
			Added:        0,
			Removed:      0,
			Modified:     0,
			HighRisk:     0,
			MediumRisk:   0,
			LowRisk:      0,
			InfoLevel:    0,
		},
	}

	// For now, we'll return mock data
	// A real implementation would compare actual parameter changes
	switch {
	case fromVersion == "v6.5.0" && toVersion == "v7.1.0":
		report.Parameters = append(report.Parameters, ParameterChange{
			Name:        "performance.run-auto-analyze",
			Type:        "bool",
			FromVersion: "v6.5.0",
			ToVersion:   "v7.1.0",
			FromValue:   false,
			ToValue:     true,
			Description: "Parameter performance.run-auto-analyze was modified from false to true",
			RiskLevel:   LowRisk,
		})
		report.Summary.TotalChanges = 1
		report.Summary.Modified = 1
		report.Summary.LowRisk = 1
	}

	return report, nil
}

// analyzePDUpgrade analyzes parameter changes between two PD versions
func (a *Analyzer) analyzePDUpgrade(fromVersion, toVersion string) (*AnalysisReport, error) {
	// This is a placeholder implementation
	report := &AnalysisReport{
		Component:   PDComponent,
		VersionFrom: fromVersion,
		VersionTo:   toVersion,
		Parameters:  []ParameterChange{},
		Summary: Summary{
			TotalChanges: 0,
			Added:        0,
			Removed:      0,
			Modified:     0,
			HighRisk:     0,
			MediumRisk:   0,
			LowRisk:      0,
			InfoLevel:    0,
		},
	}

	// For now, we'll return mock data
	switch {
	case fromVersion == "v6.5.0" && toVersion == "v7.1.0":
		report.Parameters = append(report.Parameters, ParameterChange{
			Name:        "schedule.max-store-down-time",
			Type:        "string",
			FromVersion: "v6.5.0",
			ToVersion:   "v7.1.0",
			FromValue:   "30m",
			ToValue:     "1h",
			Description: "Parameter schedule.max-store-down-time was modified from 30m to 1h",
			RiskLevel:   MediumRisk,
		})
		report.Summary.TotalChanges = 1
		report.Summary.Modified = 1
		report.Summary.MediumRisk = 1
	}

	return report, nil
}

// analyzeTiKVUpgrade analyzes parameter changes between two TiKV versions
func (a *Analyzer) analyzeTiKVUpgrade(fromVersion, toVersion string) (*AnalysisReport, error) {
	// Load parameter history
	historyFile := filepath.Join(a.knowledgeBasePath, "tikv", "parameters-history.json")
	changes, err := kbgenerator.GetTikvParameterChanges(historyFile, fromVersion, toVersion)
	if err != nil {
		return nil, fmt.Errorf("failed to get TiKV parameter changes: %v", err)
	}

	// Convert to analysis report format
	parameters := []ParameterChange{}
	for _, change := range changes {
		parameters = append(parameters, ParameterChange{
			Name:        change.Name,
			Type:        change.Type,
			FromVersion: change.FromVersion,
			ToVersion:   change.ToVersion,
			FromValue:   change.FromValue,
			ToValue:     change.ToValue,
			Description: change.Description,
			RiskLevel:   a.assessTiKVRisk(change.Name, change.FromValue, change.ToValue),
		})
	}

	report := &AnalysisReport{
		Component:   TiKVComponent,
		VersionFrom: fromVersion,
		VersionTo:   toVersion,
		Parameters:  parameters,
		Summary:     a.calculateSummary(parameters),
	}

	return report, nil
}

// findInconsistentConfigs finds configurations that are inconsistent across instances
func (a *Analyzer) findInconsistentConfigs(instances []runtime.InstanceState) []InconsistentConfig {
	inconsistentConfigs := []InconsistentConfig{}

	if len(instances) <= 1 {
		return inconsistentConfigs
	}

	// Create a map of parameter name to values across instances
	paramValues := make(map[string][]ParameterValue)

	for _, instance := range instances {
		for paramName, paramValue := range instance.State.Config {
			pv := ParameterValue{
				InstanceAddress: instance.Address,
				Value:           paramValue,
			}
			paramValues[paramName] = append(paramValues[paramName], pv)
		}
	}

	// Check each parameter for inconsistencies
	for paramName, values := range paramValues {
		if len(values) != len(instances) {
			// Parameter doesn't exist on all instances
			inconsistentConfigs = append(inconsistentConfigs, InconsistentConfig{
				ParameterName: paramName,
				Values:        values,
				RiskLevel:     MediumRisk,
				Description:   fmt.Sprintf("Parameter %s is not configured on all instances", paramName),
			})
			continue
		}

		// Check if all values are the same
		firstValue := values[0].Value
		isConsistent := true
		for _, value := range values[1:] {
			if firstValue != value.Value {
				isConsistent = false
				break
			}
		}

		if !isConsistent {
			// Assess risk level for this inconsistency
			riskLevel := a.assessInconsistencyRisk(paramName, values)

			inconsistentConfigs = append(inconsistentConfigs, InconsistentConfig{
				ParameterName: paramName,
				Values:        values,
				RiskLevel:     riskLevel,
				Description:   fmt.Sprintf("Parameter %s has different values across instances", paramName),
			})
		}
	}

	return inconsistentConfigs
}

// generateRecommendations generates recommendations for inconsistent configurations
func (a *Analyzer) generateRecommendations(inconsistentConfigs []InconsistentConfig) []Recommendation {
	recommendations := []Recommendation{}

	for _, ic := range inconsistentConfigs {
		rec := Recommendation{
			ParameterName:      ic.ParameterName,
			RiskLevel:          ic.RiskLevel,
			Description:        ic.Description,
			AffectedComponents: []string{"cluster"}, // This could be more specific
		}

		switch ic.RiskLevel {
		case HighRisk:
			rec.Recommendation = fmt.Sprintf("Align the value of %s across all instances to prevent potential data inconsistency or cluster instability", ic.ParameterName)
		case MediumRisk:
			rec.Recommendation = fmt.Sprintf("Consider aligning the value of %s across all instances for optimal performance and behavior consistency", ic.ParameterName)
		case LowRisk:
			rec.Recommendation = fmt.Sprintf("Parameter %s has different values across instances. This is generally acceptable but consider standardizing for easier management", ic.ParameterName)
		}

		recommendations = append(recommendations, rec)
	}

	return recommendations
}


// assessTiKVRisk assesses the risk level of a TiKV parameter change
func (a *Analyzer) assessTiKVRisk(paramName string, fromValue, toValue interface{}) RiskLevel {
	// High risk parameters
	highRiskParams := map[string]bool{
		"storage.reserve-space": true,
		"raftstore.raft-entry-max-size": true,
		"rocksdb.defaultcf.block-cache-size": true,
	}

	if highRiskParams[paramName] {
		return HighRisk
	}

	// Medium risk parameters
	mediumRiskParams := map[string]bool{
		"server.grpc-concurrency": true,
		"readpool.storage.high-concurrency": true,
		"readpool.storage.normal-concurrency": true,
		"readpool.storage.low-concurrency": true,
	}

	if mediumRiskParams[paramName] {
		return MediumRisk
	}

	return LowRisk
}

// assessInconsistencyRisk assesses the risk level of a configuration inconsistency
func (a *Analyzer) assessInconsistencyRisk(paramName string, values []ParameterValue) RiskLevel {
	// High risk parameters
	highRiskParams := map[string]bool{
		"storage.reserve-space": true,
		"raftstore.raft-entry-max-size": true,
		"rocksdb.defaultcf.block-cache-size": true,
		"schedule.max-store-down-time": true,
		"schedule.leader-schedule-limit": true,
		"schedule.region-schedule-limit": true,
	}

	if highRiskParams[paramName] {
		return HighRisk
	}

	// Medium risk parameters
	mediumRiskParams := map[string]bool{
		"server.grpc-concurrency": true,
		"readpool.storage.high-concurrency": true,
		"readpool.storage.normal-concurrency": true,
		"readpool.storage.low-concurrency": true,
		"schedule.replica-schedule-limit": true,
		"replication.max-replicas": true,
	}

	if mediumRiskParams[paramName] {
		return MediumRisk
	}

	return LowRisk
}

// calculateSummary calculates the summary of parameter changes
func (a *Analyzer) calculateSummary(parameters []ParameterChange) Summary {
	summary := Summary{}

	for _, param := range parameters {
		summary.TotalChanges++
		switch param.RiskLevel {
		case HighRisk:
			summary.HighRisk++
		case MediumRisk:
			summary.MediumRisk++
		case LowRisk:
			summary.LowRisk++
		case InfoLevel:
			summary.InfoLevel++
		}
	}

	return summary
}