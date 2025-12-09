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

// Parameter represents a PD configuration parameter
type Parameter struct {
	// Name is the full parameter name using dot notation
	Name string `json:"name"`
	
	// Type is the parameter type (string, integer, boolean, duration, size, etc.)
	Type string `json:"type"`
	
	// Value is the parameter value
	Value string `json:"value"`
	
	// Description is the human-readable description of the parameter
	Description string `json:"description"`
	
	// Category is the parameter category (e.g., schedule, replication, security)
	Category string `json:"category"`
}

// VersionParameters represents parameters for a specific PD version
type VersionParameters struct {
	// Version is the PD version
	Version string `json:"version"`
	
	// Parameters is the list of parameters for this version
	Parameters []Parameter `json:"parameters"`
}

// ParameterChange represents a change in a parameter between versions
type ParameterChange struct {
	// Name is the full parameter name using dot notation
	Name string `json:"name"`
	
	// Type is the parameter type
	Type string `json:"type"`
	
	// FromValue is the parameter value in the source version
	FromValue string `json:"from_value"`
	
	// ToValue is the parameter value in the target version
	ToValue string `json:"to_value"`
	
	// ChangeType is the type of change (added, removed, modified)
	ChangeType string `json:"change_type"` // added, removed, modified
	
	// RiskLevel is the risk level of the change (info, low, medium, high)
	RiskLevel string `json:"risk_level"` // info, low, medium, high
	
	// Description is the human-readable description of the parameter
	Description string `json:"description"`
	
	// Category is the parameter category
	Category string `json:"category"`
}

// ComparisonReport represents the report of parameter changes between versions
type ComparisonReport struct {
	// Component is the component name (pd)
	Component string `json:"component"`
	
	// VersionFrom is the source version
	VersionFrom string `json:"version_from"`
	
	// VersionTo is the target version
	VersionTo string `json:"version_to"`
	
	// Parameters is the list of parameter changes
	Parameters []ParameterChange `json:"parameters"`
	
	// Summary provides a summary of changes
	Summary Summary `json:"summary"`
}

// Summary provides a summary of parameter changes
type Summary struct {
	// TotalChanges is the total number of parameter changes
	TotalChanges int `json:"total_changes"`
	
	// Added is the number of added parameters
	Added int `json:"added"`
	
	// Removed is the number of removed parameters
	Removed int `json:"removed"`
	
	// Modified is the number of modified parameters
	Modified int `json:"modified"`
	
	// HighRisk is the number of high-risk changes
	HighRisk int `json:"high_risk"`
	
	// MediumRisk is the number of medium-risk changes
	MediumRisk int `json:"medium_risk"`
	
	// LowRisk is the number of low-risk changes
	LowRisk int `json:"low_risk"`
	
	// Info is the number of info-level changes
	Info int `json:"info"`
}