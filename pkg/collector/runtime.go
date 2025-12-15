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

package collector

import (
	"fmt"
	"strings"
	"time"

	"github.com/pingcap/tidb-upgrade-precheck/pkg/collector/pd"
	"github.com/pingcap/tidb-upgrade-precheck/pkg/collector/tidb"
	"github.com/pingcap/tidb-upgrade-precheck/pkg/collector/tiflash"
	"github.com/pingcap/tidb-upgrade-precheck/pkg/collector/tikv"
)

// CollectDataRequirements defines what data needs to be collected from the cluster
// This is used to optimize collection by only gathering necessary data
type CollectDataRequirements struct {
	// Components specifies which components' data is needed
	Components []string `json:"components"`
	// NeedConfig indicates if configuration parameters are needed
	NeedConfig bool `json:"need_config"`
	// NeedSystemVariables indicates if system variables are needed (mainly for TiDB)
	NeedSystemVariables bool `json:"need_system_variables"`
	// NeedAllTikvNodes indicates if all TiKV nodes' data is needed (for consistency checks)
	NeedAllTikvNodes bool `json:"need_all_tikv_nodes"`
}

// Collector is responsible for collecting runtime configuration from a TiDB cluster
type Collector struct {
	// tidbCollector handles TiDB collection
	tidbCollector tidb.TiDBCollector
	// pdCollector handles PD collection
	pdCollector pd.PDCollector
	// tikvCollector handles TiKV collection
	tikvCollector tikv.TiKVCollector
	// tiflashCollector handles TiFlash collection
	tiflashCollector tiflash.TiFlashCollector
}

// NewCollector creates a new runtime collector
func NewCollector() *Collector {
	return &Collector{
		tidbCollector:    tidb.NewTiDBCollector(),
		pdCollector:      pd.NewPDCollector(),
		tikvCollector:    tikv.NewTiKVCollector(),
		tiflashCollector: tiflash.NewTiFlashCollector(),
	}
}

// Collect collects the runtime configuration from the cluster
// If req is nil, collects all components with all data types (default behavior)
// If req is provided, collects only the required components and data types (optimized)
func (c *Collector) Collect(endpoints ClusterEndpoints, req *CollectDataRequirements) (*ClusterSnapshot, error) {
	// If no requirements specified, collect everything
	if req == nil {
		defaultReq := CollectDataRequirements{
			Components:          []string{"tidb", "pd", "tikv", "tiflash"},
			NeedConfig:          true,
			NeedSystemVariables: true,
			NeedAllTikvNodes:    true, // Collect all TiKV nodes by default
		}
		return c.collectWithRequirements(endpoints, defaultReq)
	}
	return c.collectWithRequirements(endpoints, *req)
}

// collectWithRequirements is the internal implementation that collects cluster data based on requirements
// This allows optimizing collection by only gathering necessary data
func (c *Collector) collectWithRequirements(endpoints ClusterEndpoints, req CollectDataRequirements) (*ClusterSnapshot, error) {
	snapshot := &ClusterSnapshot{
		Timestamp:  time.Now(),
		Components: make(map[string]ComponentState),
	}

	// Collect from TiDB if needed
	if contains(req.Components, "tidb") && endpoints.TiDBAddr != "" {
		if req.NeedConfig || req.NeedSystemVariables {
			tidbState, err := c.tidbCollector.Collect(endpoints.TiDBAddr, endpoints.TiDBUser, endpoints.TiDBPassword)
			if err != nil {
				return nil, fmt.Errorf("failed to collect from TiDB: %w", err)
			}
			snapshot.Components["tidb"] = *tidbState
			if snapshot.SourceVersion == "" && tidbState.Version != "" {
				snapshot.SourceVersion = tidbState.Version
			}
		}
	}

	// Collect from PD if needed
	if contains(req.Components, "pd") && len(endpoints.PDAddrs) > 0 {
		if req.NeedConfig {
			pdState, err := c.pdCollector.Collect(endpoints.PDAddrs)
			if err != nil {
				fmt.Printf("Warning: failed to collect from PD: %v\n", err)
			} else {
				snapshot.Components["pd"] = *pdState
				if snapshot.SourceVersion == "" && pdState.Version != "" {
					snapshot.SourceVersion = pdState.Version
				}
			}
		}
	}

	// Collect from TiKV if needed
	if contains(req.Components, "tikv") && len(endpoints.TiKVAddrs) > 0 {
		if req.NeedConfig {
			// Prepare data_dir mapping for TiKV collector
			dataDirs := endpoints.TiKVDataDirs
			if dataDirs == nil {
				dataDirs = make(map[string]string)
			}
			tikvStates, err := c.tikvCollector.Collect(endpoints.TiKVAddrs, dataDirs)
			if err != nil {
				fmt.Printf("Warning: failed to collect from TiKV: %v\n", err)
			} else {
				// Store TiKV instances
				// If NeedAllTikvNodes is false, only store the first one
				// If true, store all nodes
				for i, state := range tikvStates {
					if !req.NeedAllTikvNodes && i > 0 {
						break // Only need first instance
					}

					addr := endpoints.TiKVAddrs[i]
					if addrFromStatus, ok := state.Status["address"].(string); ok && addrFromStatus != "" {
						addr = addrFromStatus
					}

					key := fmt.Sprintf("tikv-%s", addr)
					key = strings.ReplaceAll(key, ":", "-")
					key = strings.ReplaceAll(key, ".", "-")

					if i == 0 {
						snapshot.Components["tikv"] = state
					}
					snapshot.Components[key] = state

					if snapshot.SourceVersion == "" && state.Version != "" {
						snapshot.SourceVersion = state.Version
					}
				}
			}
		}
	}

	// Collect from TiFlash if needed
	if contains(req.Components, "tiflash") && len(endpoints.TiFlashAddrs) > 0 {
		if req.NeedConfig {
			tiflashStates, err := c.tiflashCollector.Collect(endpoints.TiFlashAddrs)
			if err != nil {
				fmt.Printf("Warning: failed to collect from TiFlash: %v\n", err)
			} else {
				for i, state := range tiflashStates {
					addr := endpoints.TiFlashAddrs[i]
					if addrFromStatus, ok := state.Status["address"].(string); ok && addrFromStatus != "" {
						addr = addrFromStatus
					}

					key := fmt.Sprintf("tiflash-%s", addr)
					key = strings.ReplaceAll(key, ":", "-")
					key = strings.ReplaceAll(key, ".", "-")

					if i == 0 {
						snapshot.Components["tiflash"] = state
					}
					snapshot.Components[key] = state

					if snapshot.SourceVersion == "" && state.Version != "" {
						snapshot.SourceVersion = state.Version
					}
				}
			}
		}
	}

	return snapshot, nil
}

// Helper function to check if a string slice contains a value
func contains(slice []string, value string) bool {
	for _, s := range slice {
		if s == value {
			return true
		}
	}
	return false
}
