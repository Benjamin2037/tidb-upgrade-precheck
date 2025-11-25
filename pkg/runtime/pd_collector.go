package runtime

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// PDCollector handles collection of PD configuration
type PDCollector interface {
	Collect(addrs []string) (*ComponentState, error)
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
func (c *pdCollector) Collect(addrs []string) (*ComponentState, error) {
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

func (c *pdCollector) collectFromInstance(addr string) (*ComponentState, error) {
	state := &ComponentState{
		Type:   "pd",
		Config: make(map[string]interface{}),
		Status: make(map[string]interface{}),
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
	state.Config = config

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