package runtime

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// TiKVCollector handles collection of TiKV configuration
type TiKVCollector interface {
	Collect(addrs []string) ([]ComponentState, error)
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
func (c *tikvCollector) Collect(addrs []string) ([]ComponentState, error) {
	var states []ComponentState

	for _, addr := range addrs {
		state, err := c.collectFromInstance(addr)
		if err != nil {
			// Log error but continue with other instances
			fmt.Printf("Warning: failed to collect from TiKV instance %s: %v\n", addr, err)
			continue
		}
		states = append(states, *state)
	}

	return states, nil
}

func (c *tikvCollector) collectFromInstance(addr string) (*ComponentState, error) {
	state := &ComponentState{
		Type:   "tikv",
		Config: make(map[string]interface{}),
		Status: make(map[string]interface{}),
	}

	// Get version
	version, err := c.getVersion(addr)
	if err != nil {
		// If we can't get version, we still try to get config
		fmt.Printf("Warning: failed to get TiKV version from %s: %v\n", addr, err)
	}
	state.Version = version

	// Collect configuration
	config, err := c.getConfig(addr)
	if err != nil {
		return nil, fmt.Errorf("failed to get TiKV config: %w", err)
	}
	state.Config = config

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

func (c *tikvCollector) getConfig(addr string) (map[string]interface{}, error) {
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