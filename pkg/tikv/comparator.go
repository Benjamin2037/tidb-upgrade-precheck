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
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
)

// Comparator is responsible for comparing TiKV configurations between versions
type Comparator struct {
	sourcePath string
}

// ComparisonReport represents a report of configuration differences between versions
type ComparisonReport struct {
	Component    string           `json:"component"`
	VersionFrom  string           `json:"version_from"`
	VersionTo    string           `json:"version_to"`
	Parameters   []ParameterDiff  `json:"parameters"`
	FeatureGates []FeatureGateDiff `json:"feature_gates"`
	Summary      Summary          `json:"summary"`
}

// ParameterDiff represents a difference in a single parameter
type ParameterDiff struct {
	Name        string      `json:"name"`
	Type        string      `json:"type"`
	FromValue   interface{} `json:"from_value"`
	ToValue     interface{} `json:"to_value"`
	ChangeType  string      `json:"change_type"` // added, removed, modified
	RiskLevel   string      `json:"risk_level"`  // info, low, medium, high
	Description string      `json:"description"`
}

// FeatureGateDiff represents a difference in a feature gate
type FeatureGateDiff struct {
	Name        string `json:"name"`
	StatusFrom  string `json:"status_from"`
	StatusTo    string `json:"status_to"`
	RiskLevel   string `json:"risk_level"`
	Description string `json:"description"`
}

// Summary provides a summary of the changes
type Summary struct {
	TotalChanges       int `json:"total_changes"`
	Added              int `json:"added"`
	Removed            int `json:"removed"`
	Modified           int `json:"modified"`
	HighRisk           int `json:"high_risk"`
	MediumRisk         int `json:"medium_risk"`
	LowRisk            int `json:"low_risk"`
	FeatureGateChanges int `json:"feature_gate_changes"`
}

// NewComparator creates a new TiKV comparator
func NewComparator(sourcePath string) *Comparator {
	return &Comparator{
		sourcePath: sourcePath,
	}
}

// Compare compares TiKV configurations between two versions
func (c *Comparator) Compare(fromVersion, toVersion string) (*ComparisonReport, error) {
	fromConfig, err := c.getConfiguration(fromVersion)
	if err != nil {
		return nil, fmt.Errorf("failed to get configuration for version %s: %v", fromVersion, err)
	}
	
	toConfig, err := c.getConfiguration(toVersion)
	if err != nil {
		return nil, fmt.Errorf("failed to get configuration for version %s: %v", toVersion, err)
	}
	
	diff := c.compareConfigurations(fromConfig, toConfig)
	
	report := &ComparisonReport{
		Component:   "tikv",
		VersionFrom: fromVersion,
		VersionTo:   toVersion,
		Parameters:  diff.Parameters,
		Summary:     diff.Summary,
	}
	
	// For now, we don't have feature gate information in our simple collector
	// In a full implementation, we would also compare feature gates
	report.FeatureGates = []FeatureGateDiff{}
	
	return report, nil
}

// getConfiguration retrieves the configuration for a specific version
func (c *Comparator) getConfiguration(version string) (map[string]interface{}, error) {
	// Check if we have a knowledge base file for this version
	kbFile := filepath.Join("knowledge", "tikv", version, "tikv_defaults.json")
	if _, err := os.Stat(kbFile); err == nil {
		// Load from knowledge base
		data, err := os.ReadFile(kbFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read knowledge base file: %v", err)
		}
		
		var kb struct {
			ConfigDefaults map[string]interface{} `json:"config_defaults"`
		}
		
		if err := json.Unmarshal(data, &kb); err != nil {
			return nil, fmt.Errorf("failed to parse knowledge base file: %v", err)
		}
		
		return kb.ConfigDefaults, nil
	}
	
	// Fallback to collecting from source
	return c.collectFromSource(version)
}

// collectFromSource collects configuration from TiKV source code
func (c *Comparator) collectFromSource(version string) (map[string]interface{}, error) {
	// Checkout to the specific version
	cmd := exec.Command("git", "checkout", version)
	cmd.Dir = c.sourcePath
	if out, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("git checkout failed: %v, output: %s", err, out)
	}
	
	// For now, we'll return mock data
	// A real implementation would parse the TiKV configuration files
	config := map[string]interface{}{}
	
	switch version {
	case "v6.5.0":
		config["storage.reserve-space"] = "2GB"
		config["raftstore.raft-entry-max-size"] = "8MB"
		config["rocksdb.defaultcf.block-cache-size"] = "1GB"
	case "v7.1.0":
		config["storage.reserve-space"] = "0"
		config["raftstore.raft-entry-max-size"] = "8MB"
		config["rocksdb.defaultcf.block-cache-size"] = "1GB"
	case "v7.5.0":
		config["storage.reserve-space"] = "0"
		config["raftstore.raft-entry-max-size"] = "16MB"
		config["rocksdb.defaultcf.block-cache-size"] = "1GB"
	default:
		// Default configuration
		config["storage.reserve-space"] = "0"
		config["raftstore.raft-entry-max-size"] = "8MB"
		config["rocksdb.defaultcf.block-cache-size"] = "1GB"
	}
	
	return config, nil
}

// compareConfigurations compares two configurations and returns the differences
func (c *Comparator) compareConfigurations(from, to map[string]interface{}) *ComparisonReport {
	report := &ComparisonReport{
		Parameters: []ParameterDiff{},
		Summary: Summary{
			TotalChanges: 0,
			Added:        0,
			Removed:      0,
			Modified:     0,
			HighRisk:     0,
			MediumRisk:   0,
			LowRisk:      0,
		},
	}
	
	// Check for added and modified parameters
	for key, toValue := range to {
		if fromValue, exists := from[key]; exists {
			// Parameter exists in both versions - check if it's modified
			if !reflect.DeepEqual(fromValue, toValue) {
				diff := ParameterDiff{
					Name:       key,
					FromValue:  fromValue,
					ToValue:    toValue,
					ChangeType: "modified",
					Type:       c.getParameterType(toValue),
				}
				
				diff.Description = fmt.Sprintf("Parameter %s was modified from %v to %v", key, fromValue, toValue)
				diff.RiskLevel = c.assessRisk(key, fromValue, toValue)
				
				report.Parameters = append(report.Parameters, diff)
				report.Summary.Modified++
				report.Summary.TotalChanges++
				
				switch diff.RiskLevel {
				case "high":
					report.Summary.HighRisk++
				case "medium":
					report.Summary.MediumRisk++
				case "low":
					report.Summary.LowRisk++
				}
			}
		} else {
			// Parameter only exists in 'to' version - it's added
			diff := ParameterDiff{
				Name:       key,
				ToValue:    toValue,
				ChangeType: "added",
				Type:       c.getParameterType(toValue),
			}
			
			diff.Description = fmt.Sprintf("Parameter %s was added", key)
			diff.RiskLevel = c.assessRisk(key, nil, toValue)
			
			report.Parameters = append(report.Parameters, diff)
			report.Summary.Added++
			report.Summary.TotalChanges++
			
			switch diff.RiskLevel {
			case "high":
				report.Summary.HighRisk++
			case "medium":
				report.Summary.MediumRisk++
			case "low":
				report.Summary.LowRisk++
			}
		}
	}
	
	// Check for removed parameters
	for key, fromValue := range from {
		if _, exists := to[key]; !exists {
			// Parameter only exists in 'from' version - it's removed
			diff := ParameterDiff{
				Name:       key,
				FromValue:  fromValue,
				ChangeType: "removed",
				Type:       c.getParameterType(fromValue),
			}
			
			diff.Description = fmt.Sprintf("Parameter %s was removed", key)
			diff.RiskLevel = c.assessRisk(key, fromValue, nil)
			
			report.Parameters = append(report.Parameters, diff)
			report.Summary.Removed++
			report.Summary.TotalChanges++
			
			switch diff.RiskLevel {
			case "high":
				report.Summary.HighRisk++
			case "medium":
				report.Summary.MediumRisk++
			case "low":
				report.Summary.LowRisk++
			}
		}
	}
	
	return report
}

// getParameterType determines the type of a parameter value
func (c *Comparator) getParameterType(value interface{}) string {
	switch value.(type) {
	case string:
		// Check if it's a size or duration
		str := value.(string)
		if strings.HasSuffix(str, "B") || strings.HasSuffix(str, "KB") || strings.HasSuffix(str, "MB") || strings.HasSuffix(str, "GB") {
			return "size"
		}
		if strings.HasSuffix(str, "s") || strings.HasSuffix(str, "m") || strings.HasSuffix(str, "h") {
			return "duration"
		}
		return "string"
	case bool:
		return "bool"
	case float64:
		return "number"
	default:
		return "unknown"
	}
}

// assessRisk assesses the risk level of a parameter change
func (c *Comparator) assessRisk(name string, from, to interface{}) string {
	// High risk parameters
	highRiskParams := map[string]bool{
		"storage.reserve-space":             true,
		"raftstore.raft-entry-max-size":     true,
		"rocksdb.defaultcf.block-cache-size": true,
		"rocksdb.writecf.block-cache-size":  true,
		"storage.capacity":                  true,
	}
	
	if highRiskParams[name] {
		return "high"
	}
	
	// Medium risk parameters
	mediumRiskParams := map[string]bool{
		"server.grpc-concurrency":        true,
		"server.connections-per-ip":     true,
		"readpool.storage.high-concurrency": true,
		"readpool.storage.normal-concurrency": true,
		"readpool.storage.low-concurrency": true,
	}
	
	if mediumRiskParams[name] {
		return "medium"
	}
	
	// Check for significant value changes
	if from != nil && to != nil {
		// For size parameters, check if the change is significant
		if c.getParameterType(from) == "size" && c.getParameterType(to) == "size" {
			fromStr := from.(string)
			toStr := to.(string)
			
			// Very basic size comparison - in a real implementation, we'd parse and compare actual values
			if fromStr != toStr {
				// If it's a size parameter and changed, consider it at least medium risk
				return "medium"
			}
		}
	}
	
	// Low risk by default
	return "low"
}