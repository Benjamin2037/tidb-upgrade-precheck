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
	"reflect"
	"testing"

	"github.com/pingcap/tidb-upgrade-precheck/pkg/runtime"
)

func TestClusterAnalyzer_AnalyzeCluster(t *testing.T) {
	// Create test instances
	instances := []runtime.InstanceState{
		{
			Address: "127.0.0.1:20160",
			State: runtime.ComponentState{
				Type:    runtime.TiKVComponent,
				Version: "v6.5.0",
				Config: map[string]interface{}{
					"storage.reserve-space": "2GB",
				},
				Status: map[string]interface{}{},
			},
		},
		{
			Address: "127.0.0.1:20161",
			State: runtime.ComponentState{
				Type:    runtime.TiKVComponent,
				Version: "v6.5.0",
				Config: map[string]interface{}{
					"storage.reserve-space": "2GB",
				},
				Status: map[string]interface{}{},
			},
		},
	}

	// Create analyzer
	analyzer := NewClusterAnalyzer()

	// Test cluster analysis
	reports := analyzer.AnalyzeCluster(instances)

	// We expect some reports even if configs are consistent
	if reports == nil {
		t.Error("Expected non-nil reports")
	}
}

func TestClusterAnalyzer_AnalyzeScaleOut(t *testing.T) {
	// Create existing instances
	existingInstances := []runtime.InstanceState{
		{
			Address: "127.0.0.1:20160",
			State: runtime.ComponentState{
				Type:    runtime.TiKVComponent,
				Version: "v6.5.0",
				Config: map[string]interface{}{
					"storage.reserve-space": "2GB",
				},
				Status: map[string]interface{}{},
			},
		},
	}

	// Create new instances
	newInstances := []runtime.InstanceState{
		{
			Address: "127.0.0.1:20161",
			State: runtime.ComponentState{
				Type:    runtime.TiKVComponent,
				Version: "v6.5.0",
				Config: map[string]interface{}{
					"storage.reserve-space": "1GB", // Different value
				},
				Status: map[string]interface{}{},
			},
		},
	}

	// Create analyzer
	analyzer := NewClusterAnalyzer()

	// Test scale-out analysis
	reports := analyzer.AnalyzeScaleOut(existingInstances, newInstances)

	// We expect some reports for the scale-out scenario
	if reports == nil {
		t.Error("Expected non-nil reports")
	}
}

func TestClusterAnalyzer_AnalyzeScaleIn(t *testing.T) {
	// Create remaining instances
	remainingInstances := []runtime.InstanceState{
		{
			Address: "127.0.0.1:20160",
			State: runtime.ComponentState{
				Type:    runtime.TiKVComponent,
				Version: "v6.5.0",
				Config: map[string]interface{}{
					"storage.reserve-space": "2GB",
				},
				Status: map[string]interface{}{},
			},
		},
		{
			Address: "127.0.0.1:20161",
			State: runtime.ComponentState{
				Type:    runtime.TiKVComponent,
				Version: "v6.5.0",
				Config: map[string]interface{}{
					"storage.reserve-space": "2GB",
				},
				Status: map[string]interface{}{},
			},
		},
	}

	// Create removed instance IDs
	removedInstanceIDs := []string{"127.0.0.1:20162"}

	// Create analyzer
	analyzer := NewClusterAnalyzer()

	// Test scale-in analysis
	reports := analyzer.AnalyzeScaleIn(remainingInstances, removedInstanceIDs)

	// We expect some reports for the scale-in scenario
	if reports == nil {
		t.Error("Expected non-nil reports")
	}
}

func TestClusterAnalyzer_determineSeverity(t *testing.T) {
	analyzer := NewClusterAnalyzer()

	tests := []struct {
		name       string
		paramName  string
		want       Severity
	}{
		{
			name:      "High severity parameter",
			paramName: "storage.reserve-space",
			want:      High,
		},
		{
			name:      "Medium severity parameter",
			paramName: "log.level",
			want:      Medium,
		},
		{
			name:      "Low severity parameter",
			paramName: "readpool.coprocessor.normal-concurrency",
			want:      Low,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := analyzer.determineSeverity(tt.paramName); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ClusterAnalyzer.determineSeverity() = %v, want %v", got, tt.want)
			}
		})
	}
}