package tiflash

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/pingcap/tidb-upgrade-precheck/pkg/collector/tidb"
	"github.com/pingcap/tidb-upgrade-precheck/pkg/types"
)

// TiFlashCollector handles collection of TiFlash configuration
type TiFlashCollector interface {
	Collect(addrs []string) ([]types.ComponentState, error)
	// CollectWithTiDB collects TiFlash configuration with optional TiDB connection
	// This allows supplementing HTTP API config with SHOW CONFIG data for consistency
	// with knowledge base generation approach
	CollectWithTiDB(addrs []string, tidbAddr, tidbUser, tidbPassword string) ([]types.ComponentState, error)
}

type tiflashCollector struct {
	httpClient *http.Client
}

// NewTiFlashCollector creates a new TiFlash collector
func NewTiFlashCollector() TiFlashCollector {
	return &tiflashCollector{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Collect gathers configuration from TiFlash instances
// This method only collects from HTTP API (for backward compatibility)
func (c *tiflashCollector) Collect(addrs []string) ([]types.ComponentState, error) {
	return c.CollectWithTiDB(addrs, "", "", "")
}

// CollectWithTiDB gathers configuration from TiFlash instances with optional TiDB connection
// This matches the knowledge base generation approach:
// 1. Collects configuration from HTTP API /config endpoint
// 2. Collects runtime configuration via SHOW CONFIG WHERE type='tiflash' (if TiDB connection available)
// 3. Merges them with priority: runtime values > HTTP API values
func (c *tiflashCollector) CollectWithTiDB(addrs []string, tidbAddr, tidbUser, tidbPassword string) ([]types.ComponentState, error) {
	var states []types.ComponentState

	// Collect TiFlash config via SHOW CONFIG if TiDB connection is available
	// This ensures we get all parameters (including optional ones)
	var tiflashConfigFromSHOW types.ConfigDefaults
	if tidbAddr != "" {
		var err error
		tiflashConfigFromSHOW, err = c.collectTiFlashConfigViaSHOWCONFIG(tidbAddr, tidbUser, tidbPassword)
		if err != nil {
			fmt.Printf("Warning: failed to collect TiFlash config via SHOW CONFIG: %v\n", err)
			// Continue without SHOW CONFIG data
			tiflashConfigFromSHOW = make(types.ConfigDefaults)
		} else {
			fmt.Printf("Collected %d runtime parameters via SHOW CONFIG\n", len(tiflashConfigFromSHOW))
		}
	}

	for _, addr := range addrs {
		state, err := c.collectFromInstance(addr, tiflashConfigFromSHOW)
		if err != nil {
			// Log error but continue with other instances
			fmt.Printf("Warning: failed to collect from TiFlash instance %s: %v\n", addr, err)
			continue
		}
		states = append(states, *state)
	}

	return states, nil
}

func (c *tiflashCollector) collectFromInstance(addr string, tiflashConfigFromSHOW types.ConfigDefaults) (*types.ComponentState, error) {
	state := &types.ComponentState{
		Type:      types.ComponentTiFlash,
		Config:    make(types.ConfigDefaults),
		Variables: make(types.SystemVariables),
		Status:    make(map[string]interface{}),
	}

	// Store the address in Status for identification
	state.Status["address"] = addr

	// Get version
	version, err := c.getVersion(addr)
	if err != nil {
		// If we can't get version, we still try to get config
		fmt.Printf("Warning: failed to get TiFlash version from %s: %v\n", addr, err)
	}
	state.Version = version

	// Step 1: Collect configuration from HTTP API /config endpoint
	// This provides the current runtime configuration
	httpConfig := make(types.ConfigDefaults)
	config, err := c.getConfig(addr)
	if err != nil {
		fmt.Printf("Warning: failed to get TiFlash config from HTTP API for %s: %v\n", addr, err)
	} else {
		httpConfig = types.ConvertConfigToDefaults(config)
		fmt.Printf("Collected %d parameters from HTTP API for %s\n", len(httpConfig), addr)
	}

	// Step 2: Merge with SHOW CONFIG data (if available)
	// Priority: SHOW CONFIG values > HTTP API values
	// This matches the knowledge base generation approach
	state.Config = c.mergeConfigsWithPriority(httpConfig, tiflashConfigFromSHOW)

	// Collect status information
	status, err := c.getStatus(addr)
	if err != nil {
		// Log warning but continue - status might not be available
		fmt.Printf("Warning: failed to get TiFlash status from %s: %v\n", addr, err)
	} else {
		state.Status = status
	}

	return state, nil
}

func (c *tiflashCollector) getVersion(addr string) (string, error) {
	// TiFlash typically exposes version via /status endpoint
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

func (c *tiflashCollector) getConfig(addr string) (map[string]interface{}, error) {
	// TiFlash typically exposes config via /config endpoint
	resp, err := c.httpClient.Get(fmt.Sprintf("http://%s/config", addr))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP request failed with status: %d", resp.StatusCode)
	}

	var config map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&config); err != nil {
		return nil, err
	}

	return config, nil
}

func (c *tiflashCollector) getStatus(addr string) (map[string]interface{}, error) {
	// TiFlash typically exposes status via /status endpoint
	resp, err := c.httpClient.Get(fmt.Sprintf("http://%s/status", addr))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP request failed with status: %d", resp.StatusCode)
	}

	var status map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		return nil, err
	}

	return status, nil
}

// collectTiFlashConfigViaSHOWCONFIG collects TiFlash config via SHOW CONFIG WHERE type='tiflash'
// This matches the knowledge base generation approach for consistency
func (c *tiflashCollector) collectTiFlashConfigViaSHOWCONFIG(tidbAddr, tidbUser, tidbPassword string) (types.ConfigDefaults, error) {
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

	// Use TiDB collector's GetConfigByType method to get TiFlash config
	collector := tidb.NewTiDBCollector()
	config, err := collector.GetConfigByType(db, "tiflash")
	if err != nil {
		return nil, fmt.Errorf("failed to get TiFlash config via SHOW CONFIG: %w", err)
	}

	// Convert map[string]interface{} to types.ConfigDefaults
	return types.ConvertConfigToDefaults(config), nil
}

// mergeConfigsWithPriority merges HTTP API config and SHOW CONFIG with priority
// Priority: SHOW CONFIG values > HTTP API values
// This matches the knowledge base generation approach
func (c *tiflashCollector) mergeConfigsWithPriority(httpConfig, runtimeConfig types.ConfigDefaults) types.ConfigDefaults {
	merged := make(types.ConfigDefaults)

	// Step 1: Start with HTTP API values (lower priority)
	for k, v := range httpConfig {
		merged[k] = v
	}

	// Step 2: Override with runtime values from SHOW CONFIG (higher priority)
	for k, v := range runtimeConfig {
		merged[k] = v
	}

	return merged
}
