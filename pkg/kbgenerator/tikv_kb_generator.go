package kbgenerator

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// TikvKBSnapshot represents a TiKV knowledge base snapshot
type TikvKBSnapshot struct {
	Version        string                 `json:"version"`
	ConfigDefaults map[string]interface{} `json:"config_defaults"`
}

// TikvParameterChange represents a TiKV parameter change
type TikvParameterChange struct {
	Name        string      `json:"name"`
	Type        string      `json:"type"`
	FromVersion string      `json:"from_version"`
	ToVersion   string      `json:"to_version"`
	FromValue   interface{} `json:"from_value"`
	ToValue     interface{} `json:"to_value"`
	Description string      `json:"description"`
}

// TikvParameterHistory represents the history of a TiKV parameter
type TikvParameterHistory struct {
	Version     string      `json:"version"`
	Default     interface{} `json:"default"`
	Description string      `json:"description"`
}

// TikvParameterHistoryEntry represents a TiKV parameter with its history
type TikvParameterHistoryEntry struct {
	Name    string                  `json:"name"`
	Type    string                  `json:"type"`
	History []TikvParameterHistory `json:"history"`
}

// TikvParameterHistoryFile represents the TiKV parameter history file structure
type TikvParameterHistoryFile struct {
	Component  string                     `json:"component"`
	Parameters []TikvParameterHistoryEntry `json:"parameters"`
}

// CollectFromTikvSource collects TiKV configuration from source code
func CollectFromTikvSource(repoRoot, version string) (*TikvKBSnapshot, error) {
	// Checkout to the specific version
	cmd := exec.Command("git", "checkout", version)
	cmd.Dir = repoRoot
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

	snapshot := &TikvKBSnapshot{
		Version:        version,
		ConfigDefaults: config,
	}

	return snapshot, nil
}

// SaveTiKVSnapshot saves a TiKV KB snapshot to a file
func SaveTiKVSnapshot(snapshot *TikvKBSnapshot, outputPath string) error {
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

// CollectTikvUpgradeLogic collects upgrade logic between two TiKV versions
func CollectTikvUpgradeLogic(repoRoot, fromVersion, toVersion string) ([]interface{}, error) {
	// For now, we'll return mock data
	// A real implementation would analyze the source code changes between versions
	changes := []interface{}{}

	switch {
	case fromVersion == "v6.5.0" && toVersion == "v7.1.0":
		changes = append(changes, map[string]interface{}{
			"type":        "modified",
			"key":         "storage.reserve-space",
			"from_value":  "2GB",
			"to_value":    "0",
			"description": "Parameter storage.reserve-space was modified from 2GB to 0",
		})
	case fromVersion == "v7.1.0" && toVersion == "v7.5.0":
		changes = append(changes, map[string]interface{}{
			"type":        "modified",
			"key":         "raftstore.raft-entry-max-size",
			"from_value":  "8MB",
			"to_value":    "16MB",
			"description": "Parameter raftstore.raft-entry-max-size was modified from 8MB to 16MB",
		})
	}

	return changes, nil
}

// GenerateTikvUpgradeScript generates an upgrade script template
func GenerateTikvUpgradeScript(changes []interface{}) string {
	var sb strings.Builder

	sb.WriteString("#!/bin/bash\n\n")
	sb.WriteString("# TiKV Upgrade Script\n")
	sb.WriteString("# Auto-generated - review and modify as needed\n\n")

	sb.WriteString("echo \"Starting TiKV upgrade...\"\n\n")

	for _, change := range changes {
		if changeMap, ok := change.(map[string]interface{}); ok {
			changeType := changeMap["type"].(string)
			key := changeMap["key"].(string)

			switch changeType {
			case "added":
				sb.WriteString(fmt.Sprintf("# TODO: Handle added parameter %s\n", key))
			case "removed":
				sb.WriteString(fmt.Sprintf("# TODO: Handle removed parameter %s\n", key))
			case "modified":
				fromValue := changeMap["from_value"]
				toValue := changeMap["to_value"]
				sb.WriteString(fmt.Sprintf("# TODO: Handle modified parameter %s (from %v to %v)\n", key, fromValue, toValue))
			}
		}
	}

	sb.WriteString("\necho \"Upgrade script completed. Review the changes and adjust as needed.\"\n")

	return sb.String()
}

// GetTikvParameterChanges gets parameter changes between two TiKV versions
func GetTikvParameterChanges(historyFile, fromVersion, toVersion string) ([]TikvParameterChange, error) {
	// For now, we'll return mock data
	// A real implementation would parse the history file and extract changes
	changes := []TikvParameterChange{}

	switch {
	case fromVersion == "v6.5.0" && toVersion == "v7.1.0":
		changes = append(changes, TikvParameterChange{
			Name:        "storage.reserve-space",
			Type:        "modified",
			FromVersion: "v6.5.0",
			ToVersion:   "v7.1.0",
			FromValue:   "2GB",
			ToValue:     "0",
			Description: "Parameter storage.reserve-space was modified from 2GB to 0",
		})
	case fromVersion == "v7.1.0" && toVersion == "v7.5.0":
		changes = append(changes, TikvParameterChange{
			Name:        "raftstore.raft-entry-max-size",
			Type:        "modified",
			FromVersion: "v7.1.0",
			ToVersion:   "v7.5.0",
			FromValue:   "8MB",
			ToValue:     "16MB",
			Description: "Parameter raftstore.raft-entry-max-size was modified from 8MB to 16MB",
		})
	}

	return changes, nil
}

// SaveTikvParameterHistory saves TiKV parameter history to a file
func SaveTikvParameterHistory(history *TikvParameterHistoryFile, outputPath string) error {
	data, err := json.MarshalIndent(history, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal parameter history: %v", err)
	}

	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return fmt.Errorf("failed to create directory: %v", err)
	}

	if err := os.WriteFile(outputPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write file: %v", err)
	}

	return nil
}

// DetermineTikvParameterType determines the type of a TiKV parameter
func DetermineTikvParameterType(value interface{}) string {
	switch value.(type) {
	case string:
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