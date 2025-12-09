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

package runtime

import (
	"fmt"
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

// ComponentState represents the state of a component including its configuration
type ComponentState struct {
	// Type is the type of the component (tidb, pd, tikv)
	Type ComponentType `json:"type"`
	// Version is the version of the component
	Version string `json:"version"`
	// Config is the configuration of the component
	Config map[string]interface{} `json:"config"`
	// Status is the status information of the component
	Status map[string]interface{} `json:"status"`
}

// InstanceState represents the state of a component instance
type InstanceState struct {
	// Address is the address of the component instance
	Address string `json:"address"`
	// State is the state of the instance
	State ComponentState `json:"state"`
}

// ClusterState represents the state of a TiDB cluster
type ClusterState struct {
	// Instances is the list of component instances
	Instances []InstanceState `json:"instances"`
}

// Collector is responsible for collecting runtime configuration from a TiDB cluster
type Collector struct {
	// endpoints are the addresses of the cluster components
	endpoints []string
}

// NewCollector creates a new runtime collector
func NewCollector(endpoints []string) *Collector {
	return &Collector{
		endpoints: endpoints,
	}
}

// Collect collects the runtime configuration from the cluster
func (c *Collector) Collect() (*ClusterState, error) {
	// This is a placeholder implementation
	// A real implementation would connect to the cluster components and collect their configurations
	clusterState := &ClusterState{
		Instances: []InstanceState{},
	}

	// For now, we'll return mock data
	// A real implementation would collect actual configuration from the cluster
	mockTiDB := InstanceState{
		Address: "127.0.0.1:4000",
		State: ComponentState{
			Type:    TiDBComponent,
			Version: "v6.5.0",
			Config: map[string]interface{}{
				"performance.max-procs":         0,
				"performance.max-memory":        float64(0),
				"performance.cross-join":        true,
				"performance.feedback-probability": 0.0,
				"log.level":                     "info",
				"log.slow-threshold":            float64(300),
			},
			Status: map[string]interface{}{},
		},
	}

	mockPD := InstanceState{
		Address: "127.0.0.1:2379",
		State: ComponentState{
			Type:    PDComponent,
			Version: "v6.5.0",
			Config: map[string]interface{}{
				"schedule.max-store-down-time":        "30m",
				"schedule.leader-schedule-limit":      float64(4),
				"schedule.region-schedule-limit":      float64(2048),
				"schedule.replica-schedule-limit":     float64(64),
				"replication.max-replicas":            float64(3),
				"replication.location-labels":         []interface{}{},
			},
			Status: map[string]interface{}{},
		},
	}

	mockTiKV := InstanceState{
		Address: "127.0.0.1:20160",
		State: ComponentState{
			Type:    TiKVComponent,
			Version: "v6.5.0",
			Config: map[string]interface{}{
				"storage.reserve-space":               "2GB",
				"raftstore.raft-entry-max-size":      "8MB",
				"rocksdb.defaultcf.block-cache-size": "1GB",
				"server.grpc-concurrency":            float64(4),
			},
			Status: map[string]interface{}{},
		},
	}

	clusterState.Instances = append(clusterState.Instances, mockTiDB, mockPD, mockTiKV)

	return clusterState, nil
}

// CollectFromInstance collects the runtime configuration from a specific instance
func (c *Collector) CollectFromInstance(address string, componentType ComponentType) (*InstanceState, error) {
	// This is a placeholder implementation
	// A real implementation would connect to the specific instance and collect its configuration
	switch componentType {
	case TiDBComponent:
		return &InstanceState{
			Address: address,
			State: ComponentState{
				Type:    TiDBComponent,
				Version: "v6.5.0",
				Config: map[string]interface{}{
					"performance.max-procs":         0,
					"performance.max-memory":        float64(0),
					"performance.cross-join":        true,
					"performance.feedback-probability": 0.0,
					"log.level":                     "info",
					"log.slow-threshold":            float64(300),
				},
				Status: map[string]interface{}{},
			},
		}, nil
	case PDComponent:
		return &InstanceState{
			Address: address,
			State: ComponentState{
				Type:    PDComponent,
				Version: "v6.5.0",
				Config: map[string]interface{}{
					"schedule.max-store-down-time":        "30m",
					"schedule.leader-schedule-limit":      float64(4),
					"schedule.region-schedule-limit":      float64(2048),
					"schedule.replica-schedule-limit":     float64(64),
					"replication.max-replicas":            float64(3),
					"replication.location-labels":         []interface{}{},
				},
				Status: map[string]interface{}{},
			},
		}, nil
	case TiKVComponent:
		return &InstanceState{
			Address: address,
			State: ComponentState{
				Type:    TiKVComponent,
				Version: "v6.5.0",
				Config: map[string]interface{}{
					"storage.reserve-space":               "2GB",
					"raftstore.raft-entry-max-size":      "8MB",
					"rocksdb.defaultcf.block-cache-size": "1GB",
					"server.grpc-concurrency":            float64(4),
				},
				Status: map[string]interface{}{},
			},
		}, nil
	default:
		return nil, fmt.Errorf("unsupported component type: %s", componentType)
	}
}