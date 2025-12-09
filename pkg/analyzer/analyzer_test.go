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
	"testing"

	"github.com/pingcap/tidb-upgrade-precheck/pkg/runtime"
)

func TestAnalyzer_AnalyzeUpgrade(t *testing.T) {
	type args struct {
		component   ComponentType
		fromVersion string
		toVersion   string
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "TiDB upgrade analysis",
			args: args{
				component:   TiDBComponent,
				fromVersion: "v6.5.0",
				toVersion:   "v7.1.0",
			},
			wantErr: false,
		},
		{
			name: "PD upgrade analysis",
			args: args{
				component:   PDComponent,
				fromVersion: "v6.5.0",
				toVersion:   "v7.1.0",
			},
			wantErr: false,
		},
		{
			name: "TiKV upgrade analysis",
			args: args{
				component:   TiKVComponent,
				fromVersion: "v6.5.0",
				toVersion:   "v7.1.0",
			},
			wantErr: false,
		},
		{
			name: "Unsupported component",
			args: args{
				component:   "unsupported",
				fromVersion: "v6.5.0",
				toVersion:   "v7.1.0",
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			analyzer := NewAnalyzer("../knowledge")
			_, err := analyzer.AnalyzeUpgrade(tt.args.component, tt.args.fromVersion, tt.args.toVersion)
			if (err != nil) != tt.wantErr {
				t.Errorf("Analyzer.AnalyzeUpgrade() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}

func TestAnalyzer_AnalyzeCluster(t *testing.T) {
	// Create a mock cluster state
	mockClusterState := &runtime.ClusterState{
		Instances: []runtime.InstanceState{
			{
				Address: "127.0.0.1:4000",
				State: runtime.ComponentState{
					Type:    runtime.TiDBComponent,
					Version: "v6.5.0",
					Config: map[string]interface{}{
						"performance.max-procs": 0,
						"log.level":             "info",
					},
					Status: map[string]interface{}{},
				},
			},
			{
				Address: "127.0.0.1:2379",
				State: runtime.ComponentState{
					Type:    runtime.PDComponent,
					Version: "v6.5.0",
					Config: map[string]interface{}{
						"schedule.max-store-down-time": "30m",
					},
					Status: map[string]interface{}{},
				},
			},
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
		},
	}

	analyzer := NewAnalyzer("../knowledge")
	report, err := analyzer.AnalyzeCluster(mockClusterState)
	if err != nil {
		t.Fatalf("Analyzer.AnalyzeCluster() error = %v", err)
	}

	if report == nil {
		t.Error("Expected non-nil report")
	}
}

func TestAnalyzer_assessPDRisk(t *testing.T) {
	analyzer := NewAnalyzer("../../knowledge")

	tests := []struct {
		paramName string
		expected  RiskLevel
	}{
		{"schedule.max-store-down-time", HighRisk},
		{"schedule.leader-schedule-limit", HighRisk},
		{"schedule.region-schedule-limit", HighRisk},
		{"schedule.replica-schedule-limit", MediumRisk},
		{"replication.max-replicas", MediumRisk},
		{"log.level", LowRisk},
	}

	for _, test := range tests {
		actual := analyzer.assessPDRisk(test.paramName, "from", "to")
		if actual != test.expected {
			t.Errorf("Expected risk level %s for parameter %s, got %s", test.expected, test.paramName, actual)
		}
	}
}

func TestAnalyzer_assessTiKVRisk(t *testing.T) {
	analyzer := NewAnalyzer("../../knowledge")

	tests := []struct {
		paramName string
		expected  RiskLevel
	}{
		{"storage.reserve-space", HighRisk},
		{"raftstore.raft-entry-max-size", HighRisk},
		{"rocksdb.defaultcf.block-cache-size", HighRisk},
		{"server.grpc-concurrency", MediumRisk},
		{"readpool.storage.high-concurrency", MediumRisk},
		{"readpool.storage.normal-concurrency", MediumRisk},
		{"readpool.storage.low-concurrency", MediumRisk},
		{"log.level", LowRisk},
	}

	for _, test := range tests {
		actual := analyzer.assessTiKVRisk(test.paramName, "from", "to")
		if actual != test.expected {
			t.Errorf("Expected risk level %s for parameter %s, got %s", test.expected, test.paramName, actual)
		}
	}
}

func TestAnalyzer_assessInconsistencyRisk(t *testing.T) {
	analyzer := NewAnalyzer("../../knowledge")

	values := []ParameterValue{
		{InstanceAddress: "127.0.0.1:4000", Value: "info"},
		{InstanceAddress: "127.0.0.1:4001", Value: "debug"},
	}

	tests := []struct {
		paramName string
		expected  RiskLevel
	}{
		{"storage.reserve-space", HighRisk},
		{"raftstore.raft-entry-max-size", HighRisk},
		{"rocksdb.defaultcf.block-cache-size", HighRisk},
		{"schedule.max-store-down-time", HighRisk},
		{"schedule.leader-schedule-limit", HighRisk},
		{"schedule.region-schedule-limit", HighRisk},
		{"server.grpc-concurrency", MediumRisk},
		{"readpool.storage.high-concurrency", MediumRisk},
		{"readpool.storage.normal-concurrency", MediumRisk},
		{"readpool.storage.low-concurrency", MediumRisk},
		{"schedule.replica-schedule-limit", MediumRisk},
		{"replication.max-replicas", MediumRisk},
		{"log.level", LowRisk},
	}

	for _, test := range tests {
		actual := analyzer.assessInconsistencyRisk(test.paramName, values)
		if actual != test.expected {
			t.Errorf("Expected risk level %s for parameter %s, got %s", test.expected, test.paramName, actual)
		}
	}
}