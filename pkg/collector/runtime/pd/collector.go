package pd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/pingcap/tidb-upgrade-precheck/pkg/collector"
	defaultsTypes "github.com/pingcap/tidb-upgrade-precheck/pkg/types"
)

// PDCollector handles collection of PD configuration
type PDCollector interface {
	Collect(addrs []string) (*collector.ComponentState, error)
	CollectDefaults(addrs []string) (*collector.ComponentState, error) // For knowledge base generation
}

type pdCollector struct {
	httpClient *http.Client
}

// NewPDCollector creates a new PD collector
func NewPDCollector() PDCollector {
	return &pdCollector{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Collect gathers configuration from PD instances
func (c *pdCollector) Collect(addrs []string) (*collector.ComponentState, error) {
	// Try each address until one succeeds
	var lastErr error
	for _, addr := range addrs {
		state, err := c.collectFromInstance(addr)
		if err == nil {
			return state, nil
		}
		lastErr = err
		fmt.Printf("Warning: failed to collect from PD instance %s: %v\n", addr, err)
	}

	return nil, fmt.Errorf("failed to collect from any PD instance: %w", lastErr)
}

// CollectDefaults gathers default configuration from PD instances
// This is used for knowledge base generation to get default values
func (c *pdCollector) CollectDefaults(addrs []string) (*collector.ComponentState, error) {
	// Try each address until one succeeds
	var lastErr error
	for _, addr := range addrs {
		state, err := c.collectDefaultsFromInstance(addr)
		if err == nil {
			return state, nil
		}
		lastErr = err
		fmt.Printf("Warning: failed to collect defaults from PD instance %s: %v\n", addr, err)
	}

	return nil, fmt.Errorf("failed to collect defaults from any PD instance: %w", lastErr)
}

func (c *pdCollector) collectDefaultsFromInstance(addr string) (*collector.ComponentState, error) {
	state := &collector.ComponentState{
		Type:      collector.PDComponent,
		Config:    make(defaultsTypes.ConfigDefaults),
		Variables: make(defaultsTypes.SystemVariables),
		Status:    make(map[string]interface{}),
	}

	// Get version
	version, err := c.getVersion(addr)
	if err != nil {
		fmt.Printf("Warning: failed to get PD version from %s: %v\n", addr, err)
	}
	state.Version = version

	// Collect default configuration using /pd/api/v1/config/default
	config, err := c.getDefaultConfig(addr)
	if err != nil {
		return nil, fmt.Errorf("failed to get PD default config: %w", err)
	}
	// Convert to pkg/types.ConfigDefaults format
	state.Config = collector.ConvertConfigToDefaults(config)

	return state, nil
}

func (c *pdCollector) collectFromInstance(addr string) (*collector.ComponentState, error) {
	state := &collector.ComponentState{
		Type:      collector.PDComponent,
		Config:    make(defaultsTypes.ConfigDefaults),
		Variables: make(defaultsTypes.SystemVariables),
		Status:    make(map[string]interface{}),
	}

	// Get version
	version, err := c.getVersion(addr)
	if err != nil {
		// If we can't get version, we still try to get config
		fmt.Printf("Warning: failed to get PD version from %s: %v\n", addr, err)
	}
	state.Version = version

	// Collect configuration
	config, err := c.getConfig(addr)
	if err != nil {
		return nil, fmt.Errorf("failed to get PD config: %w", err)
	}
	// Convert to pkg/types.ConfigDefaults format
	state.Config = collector.ConvertConfigToDefaults(config)

	return state, nil
}

func (c *pdCollector) getVersion(addr string) (string, error) {
	resp, err := c.httpClient.Get(fmt.Sprintf("http://%s/pd/api/v1/status", addr))
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

// getConfig gets PD configuration via HTTP API
// For knowledge base generation, use getDefaultConfig to get default values
// For runtime collection, use this method to get current values
func (c *pdCollector) getConfig(addr string) (map[string]interface{}, error) {
	resp, err := c.httpClient.Get(fmt.Sprintf("http://%s/pd/api/v1/config", addr))
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

// getDefaultConfig gets PD default configuration via HTTP API
// This is used for knowledge base generation to get default values for each version
func (c *pdCollector) getDefaultConfig(addr string) (map[string]interface{}, error) {
	resp, err := c.httpClient.Get(fmt.Sprintf("http://%s/pd/api/v1/config/default", addr))
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
