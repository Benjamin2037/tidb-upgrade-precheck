package tikv

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/pelletier/go-toml/v2"
	"github.com/pingcap/tidb-upgrade-precheck/pkg/collector"
	defaultsTypes "github.com/pingcap/tidb-upgrade-precheck/pkg/types"
)

// TiKVCollector handles collection of TiKV configuration
type TiKVCollector interface {
	Collect(addrs []string, dataDirs map[string]string) ([]collector.ComponentState, error)
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

// Collect gathers configuration from TiKV instances
// dataDirs maps TiKV address to its data_dir path (from topology file)
func (c *tikvCollector) Collect(addrs []string, dataDirs map[string]string) ([]collector.ComponentState, error) {
	var states []collector.ComponentState

	for _, addr := range addrs {
		dataDir := dataDirs[addr]
		state, err := c.collectFromInstance(addr, dataDir)
		if err != nil {
			// Log error but continue with other instances
			fmt.Printf("Warning: failed to collect from TiKV instance %s: %v\n", addr, err)
			continue
		}
		states = append(states, *state)
	}

	return states, nil
}

func (c *tikvCollector) collectFromInstance(addr string, dataDir string) (*collector.ComponentState, error) {
	state := &collector.ComponentState{
		Type:      collector.TiKVComponent,
		Config:    make(defaultsTypes.ConfigDefaults),
		Variables: make(defaultsTypes.SystemVariables),
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

	// Collect configuration from last_tikv.toml file
	// This file contains the actual runtime configuration used by TiKV, including all user modifications
	if dataDir == "" {
		return nil, fmt.Errorf("data_dir not provided for TiKV instance %s (required to read last_tikv.toml)", addr)
	}

	config, err := c.getConfigFromFile(dataDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read last_tikv.toml from %s for TiKV instance %s: %w", dataDir, addr, err)
	}

	// Convert to pkg/types.ConfigDefaults format
	state.Config = collector.ConvertConfigToDefaults(config)

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
