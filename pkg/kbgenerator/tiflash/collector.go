// Package tiflash provides tools for generating TiFlash knowledge base from playground clusters
// This package collects runtime configuration directly from tiup playground clusters
// TiFlash configuration is collected from:
// 1. tiflash.toml (default values) - at ~/.tiup/data/{tag}/tiflash-{port}/tiflash.toml
// 2. SHOW CONFIG WHERE type='tiflash' (runtime values) - via TiDB connection
// These are merged with priority: runtime values > file values
// System variables are collected by TiDB collector and do not need to be collected separately here
package tiflash

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/pelletier/go-toml/v2"
	tidbRuntimeCollector "github.com/pingcap/tidb-upgrade-precheck/pkg/collector/runtime/tidb"
	"github.com/pingcap/tidb-upgrade-precheck/pkg/kbgenerator"
)

// Collect collects TiFlash knowledge base from a tiup playground cluster
// This function:
// 1. Collects TiFlash configuration from tiflash.toml file (default values)
// 2. Collects TiFlash runtime configuration via SHOW CONFIG WHERE type='tiflash' (runtime values)
// 3. Merges them with priority: runtime values > file values
// System variables are collected by TiDB collector and do not need to be collected separately here.
func Collect(tiflashRoot, version string, tidbPort int, tag string) (*kbgenerator.KBSnapshot, error) {
	fmt.Printf("Collecting TiFlash runtime configuration from playground...\n")

	// Step 1: Collect default values from tiflash.toml file
	fmt.Printf("Collecting TiFlash default configuration from tiflash.toml...\n")
	fileConfig := make(kbgenerator.ConfigDefaults)
	configFromFile, err := collectTiFlashConfigFromFile(tag)
	if err != nil {
		fmt.Printf("Warning: failed to collect from tiflash.toml: %v\n", err)
	} else {
		fileConfig = configFromFile
		fmt.Printf("Collected %d parameters from tiflash.toml\n", len(fileConfig))
	}

	// Step 2: Collect runtime values via SHOW CONFIG WHERE type='tiflash'
	fmt.Printf("Collecting TiFlash runtime configuration via SHOW CONFIG...\n")
	runtimeConfig, err := collectTiFlashConfigViaSHOWCONFIG(tidbPort)
	if err != nil {
		fmt.Printf("Warning: failed to collect via SHOW CONFIG: %v\n", err)
		runtimeConfig = make(kbgenerator.ConfigDefaults)
	} else {
		fmt.Printf("Collected %d runtime parameters via SHOW CONFIG\n", len(runtimeConfig))
	}

	// Step 3: Merge with priority: runtime values > file values
	mergedConfig := mergeConfigsWithPriority(fileConfig, runtimeConfig)

	fmt.Printf("Merged configuration: %d total parameters (file: %d, runtime: %d)\n",
		len(mergedConfig), len(fileConfig), len(runtimeConfig))

	snapshot := &kbgenerator.KBSnapshot{
		Component:        kbgenerator.ComponentTiFlash,
		Version:          version,
		ConfigDefaults:   mergedConfig,
		SystemVariables:  make(kbgenerator.SystemVariables), // Empty - system variables are collected by TiDB collector
		BootstrapVersion: 0,
	}

	return snapshot, nil
}

// findTiFlashConfigPath finds TiFlash config file path from playground tag
// TiFlash config file is typically at ~/.tiup/data/{tag}/tiflash-{port}/tiflash.toml
func findTiFlashConfigPath(tag string) (string, error) {
	// Get TIUP_HOME from environment or use default
	tiupHome := os.Getenv("TIUP_HOME")
	if tiupHome == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to get user home directory: %w", err)
		}
		tiupHome = filepath.Join(homeDir, ".tiup")
	}

	// Try to find TiFlash config directory
	// TiFlash config file is typically at ~/.tiup/data/{tag}/tiflash-{port}/tiflash.toml
	dataBaseDir := filepath.Join(tiupHome, "data", tag)

	// List directories to find tiflash instance
	entries, err := os.ReadDir(dataBaseDir)
	if err != nil {
		return "", fmt.Errorf("failed to read playground data directory %s: %w", dataBaseDir, err)
	}

	for _, entry := range entries {
		if entry.IsDir() && strings.HasPrefix(entry.Name(), "tiflash-") {
			// Try tiflash.toml first (correct path in data directory)
			configPath := filepath.Join(dataBaseDir, entry.Name(), "tiflash.toml")
			if _, err := os.Stat(configPath); err == nil {
				return configPath, nil
			}
			// Fallback to conf/tiflash.toml (for backward compatibility)
			configPath = filepath.Join(dataBaseDir, entry.Name(), "conf", "tiflash.toml")
			if _, err := os.Stat(configPath); err == nil {
				return configPath, nil
			}
		}
	}

	return "", fmt.Errorf("TiFlash config file (tiflash.toml) not found in %s", dataBaseDir)
}

// collectTiFlashConfigFromFile collects TiFlash configuration from tiflash.toml file (default values)
func collectTiFlashConfigFromFile(tag string) (kbgenerator.ConfigDefaults, error) {
	// Find TiFlash config file path
	configPath, err := findTiFlashConfigPath(tag)
	if err != nil {
		return nil, fmt.Errorf("failed to find TiFlash config file: %w", err)
	}

	// Read and parse TOML file
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read tiflash.toml: %w", err)
	}

	// Parse TOML into map
	var config map[string]interface{}
	if err := toml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse tiflash.toml: %w", err)
	}

	// Flatten nested config structure
	flattened := flattenConfig(config, "")

	// Convert to knowledge base format
	kbConfig := make(kbgenerator.ConfigDefaults)
	for k, v := range flattened {
		kbConfig[k] = kbgenerator.ParameterValue{
			Value: v,
			Type:  determineValueType(v),
		}
	}

	return kbConfig, nil
}

// collectTiFlashConfigViaSHOWCONFIG collects TiFlash config via SHOW CONFIG WHERE type='tiflash'
// This gets runtime values from the running cluster
func collectTiFlashConfigViaSHOWCONFIG(tidbPort int) (kbgenerator.ConfigDefaults, error) {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/", "root", "", "127.0.0.1", tidbPort)

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database connection: %w", err)
	}
	defer db.Close()

	db.SetConnMaxLifetime(10 * time.Second)

	// Use runtime collector's GetConfigByType method
	collector := tidbRuntimeCollector.NewTiDBCollector()
	config, err := collector.GetConfigByType(db, "tiflash")
	if err != nil {
		return nil, fmt.Errorf("failed to get TiFlash config via SHOW CONFIG: %w", err)
	}

	// Convert to knowledge base format
	kbConfig := make(kbgenerator.ConfigDefaults)
	for k, v := range config {
		kbConfig[k] = kbgenerator.ParameterValue{
			Value: v,
			Type:  determineValueType(v),
		}
	}

	return kbConfig, nil
}

// mergeConfigsWithPriority merges file config and runtime config with priority
// Priority: runtime values > file values
func mergeConfigsWithPriority(fileConfig, runtimeConfig kbgenerator.ConfigDefaults) kbgenerator.ConfigDefaults {
	merged := make(kbgenerator.ConfigDefaults)

	// Step 1: Start with file values (lower priority)
	for k, v := range fileConfig {
		merged[k] = v
	}

	// Step 2: Override with runtime values (higher priority)
	for k, v := range runtimeConfig {
		merged[k] = v
	}

	return merged
}

// flattenConfig flattens a nested map structure using dot notation
func flattenConfig(config map[string]interface{}, prefix string) map[string]interface{} {
	result := make(map[string]interface{})

	for k, v := range config {
		key := k
		if prefix != "" {
			key = prefix + "." + k
		}

		switch val := v.(type) {
		case map[string]interface{}:
			// Recursively flatten nested maps
			nested := flattenConfig(val, key)
			for nk, nv := range nested {
				result[nk] = nv
			}
		case []interface{}:
			// For arrays, convert to JSON string
			if jsonBytes, err := json.Marshal(val); err == nil {
				result[key] = string(jsonBytes)
			} else {
				result[key] = fmt.Sprintf("%v", val)
			}
		default:
			result[key] = v
		}
	}

	return result
}

// Note: Code extraction, merging, and system variable collection functions have been removed.
// TiFlash playground cluster provides complete default configuration via tiflash.toml file,
// so we no longer need to extract from source code or merge with code definitions.
// System variables are collected by TiDB collector and do not need to be collected separately here.
// This simplifies the collection process significantly, similar to PD and TiDB's approach.

// determineValueType determines the type of a value
func determineValueType(v interface{}) string {
	switch v.(type) {
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return "int"
	case float32, float64:
		return "float"
	case bool:
		return "bool"
	case string:
		return "string"
	default:
		return "string"
	}
}
