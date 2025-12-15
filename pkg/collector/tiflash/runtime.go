package tiflash

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/pingcap/tidb-upgrade-precheck/pkg/types"
)

// TiFlashCollector handles collection of TiFlash configuration
type TiFlashCollector interface {
	Collect(addrs []string) ([]types.ComponentState, error)
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
func (c *tiflashCollector) Collect(addrs []string) ([]types.ComponentState, error) {
	var states []types.ComponentState

	for _, addr := range addrs {
		state, err := c.collectFromInstance(addr)
		if err != nil {
			// Log error but continue with other instances
			fmt.Printf("Warning: failed to collect from TiFlash instance %s: %v\n", addr, err)
			continue
		}
		states = append(states, *state)
	}

	return states, nil
}

func (c *tiflashCollector) collectFromInstance(addr string) (*types.ComponentState, error) {
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

	// Collect configuration
	config, err := c.getConfig(addr)
	if err != nil {
		return nil, fmt.Errorf("failed to get TiFlash config: %w", err)
	}
	// Convert to pkg/types.ConfigDefaults format
	state.Config = types.ConvertConfigToDefaults(config)

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
