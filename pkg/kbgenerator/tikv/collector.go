// Package tikv provides tools for generating TiKV knowledge base from playground clusters
// This package collects runtime configuration from tiup playground clusters
// TiKV configuration is collected from:
// 1. last_tikv.toml (user-set values) - at ~/.tiup/data/{tag}/tikv-{port}/data/last_tikv.toml
// 2. SHOW CONFIG WHERE type='tikv' (runtime values) - via TiDB connection
// These are merged with priority: runtime values > user-set values
package tikv

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
	tidbRuntimeCollector "github.com/pingcap/tidb-upgrade-precheck/pkg/collector/runtime/tidb"
	tikvRuntimeCollector "github.com/pingcap/tidb-upgrade-precheck/pkg/collector/runtime/tikv"
	"github.com/pingcap/tidb-upgrade-precheck/pkg/kbgenerator"
)

const (
	defaultTiDBHost = "127.0.0.1"
	defaultTiDBPort = 4000
	defaultTiDBUser = "root"
	defaultTiDBPass = ""
)

// Collect collects TiKV knowledge base from a tiup playground cluster
// This function:
// 1. Collects user-set configuration from last_tikv.toml (user-set values)
// 2. Collects runtime configuration via SHOW CONFIG WHERE type='tikv' (runtime values)
// 3. Merges them with priority: runtime values > user-set values
func Collect(tikvRoot, version string, tidbPort int, tag string) (*kbgenerator.KBSnapshot, error) {
	fmt.Printf("Collecting TiKV runtime configuration from playground...\n")

	// Step 1: Collect user-set values from last_tikv.toml
	fmt.Printf("Collecting TiKV user-set configuration from last_tikv.toml...\n")
	userConfig := make(kbgenerator.ConfigDefaults)
	tikvDataDir, err := findTiKVDataDir(tag)
	if err == nil {
		configFromFile, err := collectTiKVConfigFromFile(tikvDataDir)
		if err == nil {
			userConfig = configFromFile
			fmt.Printf("Collected %d user-set parameters from last_tikv.toml\n", len(userConfig))
		} else {
			fmt.Printf("Warning: failed to collect from last_tikv.toml: %v\n", err)
		}
	} else {
		fmt.Printf("Warning: TiKV data directory not found: %v\n", err)
	}

	// Step 2: Collect runtime values via SHOW CONFIG WHERE type='tikv'
	fmt.Printf("Collecting TiKV runtime configuration via SHOW CONFIG...\n")
	runtimeConfig, err := collectTiKVConfigViaSHOWCONFIG(tidbPort)
	if err != nil {
		fmt.Printf("Warning: failed to collect via SHOW CONFIG: %v\n", err)
		runtimeConfig = make(kbgenerator.ConfigDefaults)
	} else {
		fmt.Printf("Collected %d runtime parameters via SHOW CONFIG\n", len(runtimeConfig))
	}

	// Step 3: Merge with priority: runtime values > user-set values
	mergedConfig := mergeConfigsWithPriority(userConfig, runtimeConfig)

	fmt.Printf("Merged configuration: %d total parameters (user-set: %d, runtime: %d)\n",
		len(mergedConfig), len(userConfig), len(runtimeConfig))

	snapshot := &kbgenerator.KBSnapshot{
		Component:        kbgenerator.ComponentTiKV,
		Version:          version,
		ConfigDefaults:   mergedConfig,
		BootstrapVersion: 0, // TiKV doesn't have explicit bootstrap version
	}

	return snapshot, nil
}

// findTiKVDataDir finds TiKV data directory from playground tag
// TiKV data directory is typically at ~/.tiup/data/{tag}/tikv-{port}/data
func findTiKVDataDir(tag string) (string, error) {
	// Get TIUP_HOME from environment or use default
	tiupHome := os.Getenv("TIUP_HOME")
	if tiupHome == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to get user home directory: %w", err)
		}
		tiupHome = filepath.Join(homeDir, ".tiup")
	}

	// Try to find TiKV data directory
	// Playground stores data at ~/.tiup/data/{tag}/tikv-{port}/data
	dataBaseDir := filepath.Join(tiupHome, "data", tag)

	// List directories to find tikv instance
	entries, err := os.ReadDir(dataBaseDir)
	if err != nil {
		return "", fmt.Errorf("failed to read playground data directory %s: %w", dataBaseDir, err)
	}

	for _, entry := range entries {
		if entry.IsDir() && strings.HasPrefix(entry.Name(), "tikv-") {
			dataDir := filepath.Join(dataBaseDir, entry.Name(), "data")
			lastConfigPath := filepath.Join(dataDir, "last_tikv.toml")
			if _, err := os.Stat(lastConfigPath); err == nil {
				return dataDir, nil
			}
		}
	}

	return "", fmt.Errorf("TiKV data directory not found in %s", dataBaseDir)
}

// collectTiKVConfigFromFile reads configuration from last_tikv.toml file (user-set values)
func collectTiKVConfigFromFile(dataDir string) (kbgenerator.ConfigDefaults, error) {
	// Use runtime collector for consistency (same logic as real cluster collection)
	collector := tikvRuntimeCollector.NewTiKVCollector()

	// Use Collect method which internally calls getConfigFromFile
	states, err := collector.Collect([]string{"dummy"}, map[string]string{"dummy": dataDir})
	if err != nil || len(states) == 0 {
		return nil, fmt.Errorf("failed to collect TiKV config from last_tikv.toml: %w", err)
	}

	// Convert to knowledge base format
	kbConfig := make(kbgenerator.ConfigDefaults)
	for k, v := range states[0].Config {
		kbConfig[k] = kbgenerator.ParameterValue{
			Value: v.Value,
			Type:  v.Type,
		}
	}

	return kbConfig, nil
}

// collectTiKVConfigViaSHOWCONFIG collects TiKV config via SHOW CONFIG WHERE type='tikv'
// This gets runtime values from the running cluster
func collectTiKVConfigViaSHOWCONFIG(tidbPort int) (kbgenerator.ConfigDefaults, error) {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/", defaultTiDBUser, defaultTiDBPass, defaultTiDBHost, tidbPort)

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database connection: %w", err)
	}
	defer db.Close()

	db.SetConnMaxLifetime(10 * time.Second)

	// Use runtime collector's GetConfigByType method
	collector := tidbRuntimeCollector.NewTiDBCollector()
	config, err := collector.GetConfigByType(db, "tikv")
	if err != nil {
		return nil, fmt.Errorf("failed to get TiKV config via SHOW CONFIG: %w", err)
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

// Note: Code extraction functions have been removed.
// TiKV configuration is now collected from:
// 1. last_tikv.toml (user-set values)
// 2. SHOW CONFIG WHERE type='tikv' (runtime values)
// These sources provide complete configuration without needing source code extraction.

// mergeConfigsWithPriority merges user-set and runtime configs with priority
// Priority: runtime values > user-set values
func mergeConfigsWithPriority(userConfig, runtimeConfig kbgenerator.ConfigDefaults) kbgenerator.ConfigDefaults {
	merged := make(kbgenerator.ConfigDefaults)

	// Step 1: Start with user-set values (lower priority)
	for k, v := range userConfig {
		merged[k] = v
	}

	// Step 2: Override with runtime values (higher priority)
	for k, v := range runtimeConfig {
		merged[k] = v
	}

	return merged
}

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
