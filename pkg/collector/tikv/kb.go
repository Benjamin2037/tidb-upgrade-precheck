// Package tikv provides tools for generating TiKV knowledge base from playground clusters
// This package collects runtime configuration from tiup playground clusters
// TiKV configuration is collected from:
// 1. last_tikv.toml (user-set values) - at ~/.tiup/data/{tag}/tikv-{port}/data/last_tikv.toml
// 2. SHOW CONFIG WHERE type='tikv' (runtime values) - via TiDB connection
// These are merged with priority: runtime values > user-set values
package tikv

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pingcap/tidb-upgrade-precheck/pkg/collector/common"
	"github.com/pingcap/tidb-upgrade-precheck/pkg/types"
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
func Collect(tikvRoot, version string, tidbPort int, tag string) (*types.KBSnapshot, error) {
	fmt.Printf("Collecting TiKV runtime configuration from playground...\n")

	// Step 1: Collect user-set values from last_tikv.toml
	fmt.Printf("Collecting TiKV user-set configuration from last_tikv.toml...\n")
	userConfig := make(types.ConfigDefaults)
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

	// Step 2: Collect runtime values via SHOW CONFIG WHERE type='tikv' AND instance='ip:port'
	// Use runtime collector's method for consistency
	fmt.Printf("Collecting TiKV runtime configuration via SHOW CONFIG...\n")
	runtimeConfig, err := collectTiKVConfigViaSHOWCONFIG(tidbPort, tag)
	if err != nil {
		fmt.Printf("Warning: failed to collect via SHOW CONFIG: %v\n", err)
		runtimeConfig = make(types.ConfigDefaults)
	} else {
		fmt.Printf("Collected %d runtime parameters via SHOW CONFIG\n", len(runtimeConfig))
	}

	// Step 3: Merge with priority: runtime values > user-set values
	mergedConfig := mergeConfigsWithPriority(userConfig, runtimeConfig)

	fmt.Printf("Merged configuration: %d total parameters (user-set: %d, runtime: %d)\n",
		len(mergedConfig), len(userConfig), len(runtimeConfig))

	snapshot := &types.KBSnapshot{
		Component:        types.ComponentTiKV,
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
// Uses runtime collector for consistency with real cluster collection
func collectTiKVConfigFromFile(dataDir string) (types.ConfigDefaults, error) {
	collector := NewTiKVCollector()
	// Use CollectWithTiDB with empty TiDB connection to only collect from last_tikv.toml
	states, err := collector.CollectWithTiDB([]string{"dummy"}, map[string]string{"dummy": dataDir}, "", "", "")
	if err != nil || len(states) == 0 {
		return nil, fmt.Errorf("failed to collect TiKV config from last_tikv.toml: %w", err)
	}
	// ComponentState.Config is already types.ConfigDefaults, no conversion needed
	return states[0].Config, nil
}

// collectTiKVConfigViaSHOWCONFIG collects TiKV config via SHOW CONFIG WHERE type='tikv' AND instance='ip:port'
// Uses runtime collector's method for consistency
func collectTiKVConfigViaSHOWCONFIG(tidbPort int, tag string) (types.ConfigDefaults, error) {
	// Find TiKV instance address from playground directory
	tikvAddr, err := common.FindPlaygroundInstanceAddr("tikv", tag)
	if err != nil {
		return nil, fmt.Errorf("failed to find TiKV instance address: %w", err)
	}

	// Use runtime collector's method for consistency
	tikvCollector := NewTiKVCollector()
	tidbAddr := fmt.Sprintf("%s:%d", defaultTiDBHost, tidbPort)

	// Use the runtime collector's method to get config for specific instance
	// This matches the approach used in runtime collection
	states, err := tikvCollector.CollectWithTiDB(
		[]string{tikvAddr},
		map[string]string{}, // dataDirs not needed for SHOW CONFIG
		tidbAddr, defaultTiDBUser, defaultTiDBPass)
	if err != nil {
		return nil, fmt.Errorf("failed to collect TiKV config via SHOW CONFIG: %w", err)
	}
	if len(states) == 0 {
		return nil, fmt.Errorf("no TiKV state collected")
	}

	// Return the config from the collected state
	return states[0].Config, nil
}

// TiKV configuration is now collected from:
// 1. last_tikv.toml (user-set values)
// 2. SHOW CONFIG WHERE type='tikv' (runtime values)
// These sources provide complete configuration without needing source code extraction.

// mergeConfigsWithPriority merges user-set and runtime configs with priority
// Priority: runtime values > user-set values
func mergeConfigsWithPriority(userConfig, runtimeConfig types.ConfigDefaults) types.ConfigDefaults {
	merged := make(types.ConfigDefaults)

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
