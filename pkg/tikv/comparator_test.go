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

func TestComparator_Compare(t *testing.T) {
	comparator := NewComparator("/fake/path")
	
	// Test comparing two versions
	report, err := comparator.Compare("v6.5.0", "v7.1.0")
	if err != nil {
		t.Fatalf("Failed to compare: %v", err)
	}
	
	if report.Component != "tikv" {
		t.Errorf("Expected component 'tikv', got %s", report.Component)
	}
	
	if report.VersionFrom != "v6.5.0" {
		t.Errorf("Expected versionFrom 'v6.5.0', got %s", report.VersionFrom)
	}
	
	if report.VersionTo != "v7.1.0" {
		t.Errorf("Expected versionTo 'v7.1.0', got %s", report.VersionTo)
	}
	
	// Check that we have some parameters
	if len(report.Parameters) == 0 {
		t.Error("Expected some parameters in the report")
	}
	
	// Verify that storage.reserve-space is in the report and marked as modified
	found := false
	for _, param := range report.Parameters {
		if param.Name == "storage.reserve-space" && param.ChangeType == "modified" {
			found = true
			break
		}
	}
	
	if !found {
		t.Error("Expected storage.reserve-space to be in the report as modified")
	}
}

func TestComparator_getParameterType(t *testing.T) {
	comparator := NewComparator("/fake/path")
	
	tests := []struct {
		value    interface{}
		expected string
	}{
		{"1GB", "size"},
		{"30m", "duration"},
		{"normal string", "string"},
		{true, "bool"},
		{float64(10), "number"},
	}
	
	for _, test := range tests {
		actual := comparator.getParameterType(test.value)
		if actual != test.expected {
			t.Errorf("Expected type %s for value %v, got %s", test.expected, test.value, actual)
		}
	}
}