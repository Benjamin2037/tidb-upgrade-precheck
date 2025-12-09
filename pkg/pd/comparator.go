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

// Comparator is responsible for comparing PD configuration parameters between versions
type Comparator struct {
	collector *Collector
}

// NewComparator creates a new PD comparator
func NewComparator(sourcePath string) *Comparator {
	return &Comparator{
		collector: NewCollector(sourcePath),
	}
}

// Compare compares PD parameters between two versions
func (c *Comparator) Compare(fromVersion, toVersion string) (*ComparisonReport, error) {
	fromParams, err := c.collector.Collect(fromVersion)
	if err != nil {
		return nil, fmt.Errorf("failed to collect parameters for version %s: %v", fromVersion, err)
	}
	
	toParams, err := c.collector.Collect(toVersion)
	if err != nil {
		return nil, fmt.Errorf("failed to collect parameters for version %s: %v", toVersion, err)
	}
	
	changes := c.compareParameters(fromParams, toParams)
	summary := c.generateSummary(changes)
	
	return &ComparisonReport{
		Component:   "pd",
		VersionFrom: fromVersion,
		VersionTo:   toVersion,
		Parameters:  changes,
		Summary:     summary,
	}, nil
}

// compareParameters compares parameters between two versions
func (c *Comparator) compareParameters(from, to *VersionParameters) []ParameterChange {
	changes := []ParameterChange{}
	
	// Convert parameters to maps for easier lookup
	fromMap := make(map[string]Parameter)
	toMap := make(map[string]Parameter)
	
	for _, param := range from.Parameters {
		fromMap[param.Name] = param
	}
	
	for _, param := range to.Parameters {
		toMap[param.Name] = param
	}
	
	// Find added parameters (in to but not in from)
	for name, param := range toMap {
		if _, exists := fromMap[name]; !exists {
			changes = append(changes, ParameterChange{
				Name:        param.Name,
				Type:        param.Type,
				FromValue:   "",
				ToValue:     param.Value,
				ChangeType:  "added",
				RiskLevel:   c.assessRisk(param, "", "added"),
				Description: param.Description,
				Category:    param.Category,
			})
		}
	}
	
	// Find removed parameters (in from but not in to)
	for name, param := range fromMap {
		if _, exists := toMap[name]; !exists {
			changes = append(changes, ParameterChange{
				Name:        param.Name,
				Type:        param.Type,
				FromValue:   param.Value,
				ToValue:     "",
				ChangeType:  "removed",
				RiskLevel:   c.assessRisk(param, param.Value, "removed"),
				Description: param.Description,
				Category:    param.Category,
			})
		}
	}
	
	// Find modified parameters (in both but with different values)
	for name, fromParam := range fromMap {
		if toParam, exists := toMap[name]; exists {
			if fromParam.Value != toParam.Value || fromParam.Type != toParam.Type {
				changes = append(changes, ParameterChange{
					Name:        fromParam.Name,
					Type:        toParam.Type,
					FromValue:   fromParam.Value,
					ToValue:     toParam.Value,
					ChangeType:  "modified",
					RiskLevel:   c.assessRisk(toParam, fromParam.Value, "modified"),
					Description: toParam.Description,
					Category:    toParam.Category,
				})
			}
		}
	}
	
	return changes
}

// assessRisk assesses the risk level of a parameter change
func (c *Comparator) assessRisk(param Parameter, oldValue, changeType string) string {
	// This is a simplified risk assessment
	// A real implementation would have more sophisticated rules
	
	// High risk parameters
	highRiskParams := map[string]bool{
		"schedule.max-store-down-time": true,
		"schedule.replica-schedule-limit": true,
		"schedule.region-schedule-limit": true,
	}
	
	// Medium risk parameters
	mediumRiskParams := map[string]bool{
		"replication.location-labels": true,
		"replication.max-replicas": true,
		"schedule.leader-schedule-limit": true,
	}
	
	switch changeType {
	case "added":
		if highRiskParams[param.Name] {
			return "high"
		} else if mediumRiskParams[param.Name] {
			return "medium"
		}
		return "low"
		
	case "removed":
		if highRiskParams[param.Name] {
			return "high"
		} else if mediumRiskParams[param.Name] {
			return "medium"
		}
		return "low"
		
	case "modified":
		if highRiskParams[param.Name] {
			return "high"
		} else if mediumRiskParams[param.Name] {
			return "medium"
		}
		// Check for significant value changes
		if c.isSignificantChange(param, oldValue) {
			return "medium"
		}
		return "low"
	}
	
	return "info"
}

// isSignificantChange determines if a parameter value change is significant
func (c *Comparator) isSignificantChange(param Parameter, oldValue string) bool {
	// This is a simplified implementation
	// A real implementation would have more sophisticated logic
	
	// For duration parameters, consider changes of an order of magnitude significant
	if param.Type == "duration" {
		// Simplified check - in a real implementation we would parse and compare durations
		return len(oldValue) != len(param.Value)
	}
	
	// For numeric parameters, consider large percentage changes significant
	if param.Type == "integer" || param.Type == "float" {
		// Simplified check - in a real implementation we would parse and compare numbers
		return oldValue != param.Value
	}
	
	return oldValue != param.Value
}

// generateSummary generates a summary of parameter changes
func (c *Comparator) generateSummary(changes []ParameterChange) Summary {
	summary := Summary{}
	
	for _, change := range changes {
		summary.TotalChanges++
		
		switch change.ChangeType {
		case "added":
			summary.Added++
		case "removed":
			summary.Removed++
		case "modified":
			summary.Modified++
		}
		
		switch change.RiskLevel {
		case "high":
			summary.HighRisk++
		case "medium":
			summary.MediumRisk++
		case "low":
			summary.LowRisk++
		case "info":
			summary.Info++
		}
	}
	
	return summary
}

// GetParameterDetails retrieves detailed information about a specific parameter
func (c *Comparator) GetParameterDetails(version, paramName string) (*Parameter, error) {
	return c.collector.GetParameter(version, paramName)
}

// ListSupportedVersions returns all PD versions supported by the comparator
func (c *Comparator) ListSupportedVersions() ([]string, error) {
	return c.collector.ListSupportedVersions()
}