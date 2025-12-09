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

func TestComparator_Compare(t *testing.T) {
	comparator := NewComparator("")
	
	report, err := comparator.Compare("v6.5.0", "v7.1.0")
	assert.NoError(t, err)
	assert.NotNil(t, report)
	
	assert.Equal(t, "pd", report.Component)
	assert.Equal(t, "v6.5.0", report.VersionFrom)
	assert.Equal(t, "v7.1.0", report.VersionTo)
	
	// We're using mock data, so we may not have parameter changes
	// Just check that the report structure is correct
	assert.NotNil(t, report.Parameters)
	assert.NotNil(t, report.Summary)
}

func TestComparator_compareParameters(t *testing.T) {
	comparator := NewComparator("")
	
	fromParams := &VersionParameters{
		Version: "v6.5.0",
		Parameters: []Parameter{
			{
				Name:        "schedule.max-store-down-time",
				Type:        "duration",
				Value:       "30m",
				Description: "Maximum downtime for a store before it is considered unavailable",
				Category:    "schedule",
			},
			{
				Name:        "replication.location-labels",
				Type:        "string array",
				Value:       "",
				Description: "Location labels used to describe the isolation level of nodes",
				Category:    "replication",
			},
		},
	}
	
	toParams := &VersionParameters{
		Version: "v7.1.0",
		Parameters: []Parameter{
			{
				Name:        "schedule.max-store-down-time",
				Type:        "duration",
				Value:       "1h",
				Description: "Maximum downtime for a store before it is considered unavailable",
				Category:    "schedule",
			},
			{
				Name:        "security.cacert-path",
				Type:        "string",
				Value:       "",
				Description: "Path to the CA certificate file",
				Category:    "security",
			},
		},
	}
	
	changes := comparator.compareParameters(fromParams, toParams)
	
	// Should have 3 changes:
	// 1. Modified: schedule.max-store-down-time (value changed from 30m to 1h)
	// 2. Removed: replication.location-labels
	// 3. Added: security.cacert-path
	
	assert.Len(t, changes, 3)
	
	// Check modified parameter
	modifiedFound := false
	addedFound := false
	removedFound := false
	
	for _, change := range changes {
		if change.Name == "schedule.max-store-down-time" && change.ChangeType == "modified" {
			assert.Equal(t, "30m", change.FromValue)
			assert.Equal(t, "1h", change.ToValue)
			modifiedFound = true
		}
		
		if change.Name == "security.cacert-path" && change.ChangeType == "added" {
			assert.Equal(t, "", change.FromValue)
			assert.Equal(t, "", change.ToValue)
			addedFound = true
		}
		
		if change.Name == "replication.location-labels" && change.ChangeType == "removed" {
			assert.Equal(t, "", change.FromValue)
			assert.Equal(t, "", change.ToValue)
			removedFound = true
		}
	}
	
	assert.True(t, modifiedFound, "Should find modified parameter")
	assert.True(t, addedFound, "Should find added parameter")
	assert.True(t, removedFound, "Should find removed parameter")
}

func TestComparator_assessRisk(t *testing.T) {
	comparator := NewComparator("")
	
	// Test high risk parameter
	highRiskParam := Parameter{
		Name:  "schedule.max-store-down-time",
		Type:  "duration",
		Value: "1h",
	}
	
	risk := comparator.assessRisk(highRiskParam, "", "added")
	assert.Equal(t, "high", risk)
	
	// Test medium risk parameter
	mediumRiskParam := Parameter{
		Name:  "replication.location-labels",
		Type:  "string array",
		Value: "zone,rack,host",
	}
	
	risk = comparator.assessRisk(mediumRiskParam, "", "added")
	assert.Equal(t, "medium", risk)
	
	// Test low risk parameter
	lowRiskParam := Parameter{
		Name:  "log.level",
		Type:  "string",
		Value: "info",
	}
	
	risk = comparator.assessRisk(lowRiskParam, "", "added")
	assert.Equal(t, "low", risk)
}

func TestComparator_generateSummary(t *testing.T) {
	comparator := NewComparator("")
	
	changes := []ParameterChange{
		{Name: "param1", ChangeType: "added", RiskLevel: "high"},
		{Name: "param2", ChangeType: "removed", RiskLevel: "medium"},
		{Name: "param3", ChangeType: "modified", RiskLevel: "low"},
		{Name: "param4", ChangeType: "added", RiskLevel: "info"},
		{Name: "param5", ChangeType: "modified", RiskLevel: "high"},
	}
	
	summary := comparator.generateSummary(changes)
	
	assert.Equal(t, 5, summary.TotalChanges)
	assert.Equal(t, 2, summary.Added)
	assert.Equal(t, 1, summary.Removed)
	assert.Equal(t, 2, summary.Modified)
	assert.Equal(t, 2, summary.HighRisk)
	assert.Equal(t, 1, summary.MediumRisk)
	assert.Equal(t, 1, summary.LowRisk)
	assert.Equal(t, 1, summary.Info)
}