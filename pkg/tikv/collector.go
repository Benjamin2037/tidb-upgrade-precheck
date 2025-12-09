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
)

// Collector is responsible for collecting TiKV configuration from source code
type Collector struct {
	sourcePath string
}

// Snapshot represents a TiKV configuration snapshot
type Snapshot struct {
	Version        string                 `json:"version"`
	ConfigDefaults map[string]interface{} `json:"config_defaults"`
}

// NewCollector creates a new TiKV collector
func NewCollector(sourcePath string) *Collector {
	return &Collector{
		sourcePath: sourcePath,
	}
}

// Collect collects TiKV configuration for a specific version
func (c *Collector) Collect(version string) (*Snapshot, error) {
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
		config["server.grpc-concurrency"] = float64(4)
	case "v7.1.0":
		config["storage.reserve-space"] = "0"
		config["raftstore.raft-entry-max-size"] = "8MB"
		config["rocksdb.defaultcf.block-cache-size"] = "1GB"
		config["server.grpc-concurrency"] = float64(5)
	case "v7.5.0":
		config["storage.reserve-space"] = "0"
		config["raftstore.raft-entry-max-size"] = "16MB"
		config["rocksdb.defaultcf.block-cache-size"] = "1GB"
		config["server.grpc-concurrency"] = float64(5)
	case "v8.1.0":
		config["storage.reserve-space"] = "0"
		config["raftstore.raft-entry-max-size"] = "16MB"
		config["rocksdb.defaultcf.block-cache-size"] = "1GB"
		config["server.grpc-concurrency"] = float64(5)
	case "v8.5.0":
		config["storage.reserve-space"] = "0"
		config["raftstore.raft-entry-max-size"] = "16MB"
		config["rocksdb.defaultcf.block-cache-size"] = "1GB"
		config["server.grpc-concurrency"] = float64(5)
	default:
		// Default configuration
		config["storage.reserve-space"] = "0"
		config["raftstore.raft-entry-max-size"] = "8MB"
		config["rocksdb.defaultcf.block-cache-size"] = "1GB"
		config["server.grpc-concurrency"] = float64(4)
	}
	
	snapshot := &Snapshot{
		Version:        version,
		ConfigDefaults: config,
	}
	
	return snapshot, nil
}

// Save saves a snapshot to a file
func (c *Collector) Save(snapshot *Snapshot, outputPath string) error {
	data, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal snapshot: %v", err)
	}
	
	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return fmt.Errorf("failed to create directory: %v", err)
	}
	
	if err := os.WriteFile(outputPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write file: %v", err)
	}
	
	return nil
}