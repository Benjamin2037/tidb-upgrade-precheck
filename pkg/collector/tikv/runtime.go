package tikv

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/pelletier/go-toml/v2"
	"github.com/pingcap/tidb-upgrade-precheck/pkg/collector/tidb"
	"github.com/pingcap/tidb-upgrade-precheck/pkg/types"
)

// TiKVCollector handles collection of TiKV configuration
type TiKVCollector interface {
	// CollectWithTiDB collects TiKV configuration with optional TiDB connection
	// In upgrade precheck scenario, TiDB connection is always available
	// This collects from both last_tikv.toml and SHOW CONFIG, then merges them for the most complete configuration
	// If tidbAddr is empty, only collects from last_tikv.toml (for knowledge base generation)
	CollectWithTiDB(addrs []string, dataDirs map[string]string, tidbAddr, tidbUser, tidbPassword string) ([]types.ComponentState, error)
}

type tikvCollector struct {
	httpClient *http.Client
}

// NewTiKVCollector creates a new TiKV collector
func NewTiKVCollector() TiKVCollector {
	return &tikvCollector{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// CollectWithTiDB gathers configuration from TiKV instances with optional TiDB connection
// This matches the knowledge base generation approach:
// 1. Collects user-set configuration from last_tikv.toml
// 2. Collects runtime configuration via SHOW CONFIG WHERE type='tikv' AND instance='ip:port' for each instance (if TiDB connection available)
// 3. Merges them with priority: runtime values > user-set values
// dataDirs maps TiKV address to its data_dir path (from topology file)
func (c *tikvCollector) CollectWithTiDB(addrs []string, dataDirs map[string]string, tidbAddr, tidbUser, tidbPassword string) ([]types.ComponentState, error) {
	var states []types.ComponentState

	for _, addr := range addrs {
		dataDir := dataDirs[addr]
		state, err := c.collectFromInstance(addr, dataDir, tidbAddr, tidbUser, tidbPassword)
		if err != nil {
			// Log error but continue with other instances
			fmt.Printf("Warning: failed to collect from TiKV instance %s: %v\n", addr, err)
			continue
		}
		states = append(states, *state)
	}

	return states, nil
}

func (c *tikvCollector) collectFromInstance(addr string, dataDir string, tidbAddr, tidbUser, tidbPassword string) (*types.ComponentState, error) {
	state := &types.ComponentState{
		Type:      types.ComponentTiKV,
		Config:    make(types.ConfigDefaults),
		Variables: make(types.SystemVariables),
		Status:    make(map[string]interface{}),
	}

	// Store the address in Status for identification
	state.Status["address"] = addr

	// Get version (still use HTTP API for version, as it's lightweight)
	version, err := c.getVersion(addr)
	if err != nil {
		// If we can't get version, we still try to get config
		fmt.Printf("Warning: failed to get TiKV version from %s: %v\n", addr, err)
	}
	state.Version = version

	// Step 1: Collect user-set values from last_tikv.toml
	// This file contains the actual runtime configuration used by TiKV, including all user modifications
	userConfig := make(types.ConfigDefaults)
	if dataDir != "" {
		config, err := c.getConfigFromFile(dataDir)
		if err != nil {
			fmt.Printf("Warning: failed to read last_tikv.toml from %s for TiKV instance %s: %v\n", dataDir, addr, err)
		} else {
			userConfig = types.ConvertConfigToDefaults(config)
			fmt.Printf("Collected %d user-set parameters from last_tikv.toml for %s\n", len(userConfig), addr)
		}
	}

	// Step 2: Collect runtime configuration via SHOW CONFIG WHERE type='tikv' AND instance='ip:port' for this specific instance
	// This ensures we get all parameters (including optional ones like backup.*) for each instance
	var tikvConfigFromSHOW types.ConfigDefaults
	if tidbAddr != "" {
		var err error
		tikvConfigFromSHOW, err = c.collectTiKVConfigViaSHOWCONFIGForInstance(tidbAddr, tidbUser, tidbPassword, addr)
		if err != nil {
			fmt.Printf("Warning: failed to collect TiKV config via SHOW CONFIG for instance %s: %v\n", addr, err)
			// Continue without SHOW CONFIG data for this instance
			tikvConfigFromSHOW = make(types.ConfigDefaults)
		} else {
			fmt.Printf("Collected %d runtime parameters via SHOW CONFIG for instance %s\n", len(tikvConfigFromSHOW), addr)
		}
	}

	// Step 3: Merge with SHOW CONFIG data (if available)
	// Priority: SHOW CONFIG values > last_tikv.toml values
	// This matches the knowledge base generation approach
	state.Config = c.mergeConfigsWithPriority(userConfig, tikvConfigFromSHOW)

	return state, nil
}

func (c *tikvCollector) getVersion(addr string) (string, error) {
	resp, err := c.httpClient.Get(fmt.Sprintf("http://%s/status", addr))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP request failed with status: %d", resp.StatusCode)
	}

	var status struct {
		Version string `json:"version"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		return "", err
	}

	return status.Version, nil
}

// getConfigFromFile reads configuration from last_tikv.toml file
// This file contains the actual runtime configuration used by TiKV, including all user modifications
// The dataDir is provided from topology file (e.g., topology.yaml)
func (c *tikvCollector) getConfigFromFile(dataDir string) (map[string]interface{}, error) {
	// Construct path to last_tikv.toml
	lastConfigPath := filepath.Join(dataDir, "last_tikv.toml")

	// Check if file exists
	if _, err := os.Stat(lastConfigPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("last_tikv.toml not found at %s", lastConfigPath)
	}

	// Read and parse TOML file
	data, err := os.ReadFile(lastConfigPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read last_tikv.toml: %w", err)
	}

	// Parse TOML into map
	var config map[string]interface{}
	if err := toml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse last_tikv.toml: %w", err)
	}

	return config, nil
}

// collectTiKVConfigViaSHOWCONFIGForInstance collects TiKV config via SHOW CONFIG WHERE type='tikv' AND instance='ip:port'
// This gets the full parameter set for a specific TiKV instance
// instance should be in format "IP:port" (e.g., "192.168.1.101:20160")
func (c *tikvCollector) collectTiKVConfigViaSHOWCONFIGForInstance(tidbAddr, tidbUser, tidbPassword, instance string) (types.ConfigDefaults, error) {
	// Build DSN for TiDB connection
	dsn := fmt.Sprintf("%s:%s@tcp(%s)/", tidbUser, tidbPassword, tidbAddr)
	if tidbUser == "" {
		dsn = fmt.Sprintf("root@tcp(%s)/", tidbAddr)
	}

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database connection: %w", err)
	}
	defer db.Close()
	db.SetConnMaxLifetime(10 * time.Second)

	// Use TiDB collector's GetConfigByTypeAndInstance method to get TiKV config for specific instance
	collector := tidb.NewTiDBCollector()
	config, err := collector.GetConfigByTypeAndInstance(db, "tikv", instance)
	if err != nil {
		return nil, fmt.Errorf("failed to get TiKV config via SHOW CONFIG for instance %s: %w", instance, err)
	}

	// Convert map[string]interface{} to types.ConfigDefaults
	return types.ConvertConfigToDefaults(config), nil
}

// mergeConfigsWithPriority merges user-set and runtime configs with priority
// Priority: runtime values (from SHOW CONFIG) > user-set values (from last_tikv.toml)
// This matches the knowledge base generation approach
func (c *tikvCollector) mergeConfigsWithPriority(userConfig, runtimeConfig types.ConfigDefaults) types.ConfigDefaults {
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
