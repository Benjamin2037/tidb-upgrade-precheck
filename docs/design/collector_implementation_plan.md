# Collector Implementation Plan

## 1. Overview

This document outlines the implementation plan for the collector module that will gather current configuration and system variables from a running TiDB cluster.

## 2. Module Structure

```
pkg/
└── collector/
    ├── collector.go          # Main collector interface
    ├── tidb_collector.go     # TiDB-specific collection logic
    ├── tikv_collector.go     # TiKV-specific collection logic
    ├── pd_collector.go       # PD-specific collection logic
    ├── types.go              # Data structures
    └── utils.go              # Utility functions
```

## 3. Implementation Steps

### 3.1. Define Data Structures (types.go)

```go
package collector

import (
    "time"
)

// ClusterSnapshot represents the complete configuration state of a cluster
type ClusterSnapshot struct {
    Timestamp     time.Time              `json:"timestamp"`
    SourceVersion string                 `json:"source_version"`
    TargetVersion string                 `json:"target_version"`
    Components    map[string]ComponentState `json:"components"`
}

// ComponentState represents the configuration state of a single component
type ComponentState struct {
    Type      string                 `json:"type"`        // tidb, tikv, pd, tiflash
    Version   string                 `json:"version"`
    Config    map[string]interface{} `json:"config"`      // Configuration parameters
    Variables map[string]string      `json:"variables"`   // System variables (for TiDB)
    Status    map[string]interface{} `json:"status"`      // Runtime status information
}

// ClusterEndpoints contains connection information for cluster components
type ClusterEndpoints struct {
    TiDBAddr string   // MySQL protocol endpoint (host:port)
    TiKVAddrs []string // HTTP API endpoints
    PDAddrs   []string // HTTP API endpoints
}

// Collector defines the interface for collecting cluster information
type Collector interface {
    Collect(endpoints ClusterEndpoints) (*ClusterSnapshot, error)
}
```

### 3.2. Main Collector Interface (collector.go)

```go
package collector

import (
    "context"
    "fmt"
)

type collector struct {
    tidbCollector TiDBCollector
    tikvCollector TiKVCollector
    pdCollector   PDCollector
}

// NewCollector creates a new collector instance
func NewCollector() Collector {
    return &collector{
        tidbCollector: NewTiDBCollector(),
        tikvCollector: NewTiKVCollector(),
        pdCollector:   NewPDCollector(),
    }
}

// Collect gathers configuration information from all cluster components
func (c *collector) Collect(endpoints ClusterEndpoints) (*ClusterSnapshot, error) {
    snapshot := &ClusterSnapshot{
        Timestamp:  time.Now(),
        Components: make(map[string]ComponentState),
    }

    // Collect from TiDB
    if endpoints.TiDBAddr != "" {
        tidbState, err := c.tidbCollector.Collect(endpoints.TiDBAddr)
        if err != nil {
            return nil, fmt.Errorf("failed to collect TiDB info: %w", err)
        }
        snapshot.Components["tidb"] = *tidbState
    }

    // Collect from TiKV
    if len(endpoints.TiKVAddrs) > 0 {
        tikvStates, err := c.tikvCollector.Collect(endpoints.TiKVAddrs)
        if err != nil {
            return nil, fmt.Errorf("failed to collect TiKV info: %w", err)
        }
        for i, state := range tikvStates {
            snapshot.Components[fmt.Sprintf("tikv-%d", i)] = state
        }
    }

    // Collect from PD
    if len(endpoints.PDAddrs) > 0 {
        pdState, err := c.pdCollector.Collect(endpoints.PDAddrs)
        if err != nil {
            return nil, fmt.Errorf("failed to collect PD info: %w", err)
        }
        snapshot.Components["pd"] = *pdState
    }

    return snapshot, nil
}
```

### 3.3. TiDB Collector (tidb_collector.go)

```go
package collector

import (
    "database/sql"
    "encoding/json"
    "fmt"
    "net/http"
    "time"

    _ "github.com/go-sql-driver/mysql"
)

// TiDBCollector handles collection of TiDB configuration and variables
type TiDBCollector interface {
    Collect(addr string) (*ComponentState, error)
}

type tidbCollector struct {
    httpClient *http.Client
}

// NewTiDBCollector creates a new TiDB collector
func NewTiDBCollector() TiDBCollector {
    return &tidbCollector{
        httpClient: &http.Client{
            Timeout: 30 * time.Second,
        },
    }
}

// Collect gathers configuration and variables from a TiDB instance
func (c *tidbCollector) Collect(addr string) (*ComponentState, error) {
    state := &ComponentState{
        Type:      "tidb",
        Config:    make(map[string]interface{}),
        Variables: make(map[string]string),
        Status:    make(map[string]interface{}),
    }

    // Get version
    version, err := c.getVersion(addr)
    if err != nil {
        return nil, fmt.Errorf("failed to get TiDB version: %w", err)
    }
    state.Version = version

    // Collect configuration
    config, err := c.getConfig(addr)
    if err != nil {
        return nil, fmt.Errorf("failed to get TiDB config: %w", err)
    }
    state.Config = config

    // Collect variables
    variables, err := c.getVariables(addr)
    if err != nil {
        return nil, fmt.Errorf("failed to get TiDB variables: %w", err)
    }
    state.Variables = variables

    return state, nil
}

func (c *tidbCollector) getVersion(addr string) (string, error) {
    db, err := sql.Open("mysql", fmt.Sprintf("root@tcp(%s)/", addr))
    if err != nil {
        return "", err
    }
    defer db.Close()

    var version string
    err = db.QueryRow("SELECT VERSION()").Scan(&version)
    if err != nil {
        return "", err
    }

    return version, nil
}

func (c *tidbCollector) getConfig(addr string) (map[string]interface{}, error) {
    host, port, err := splitHostPort(addr)
    if err != nil {
        return nil, fmt.Errorf("invalid address format: %w", err)
    }

    // Try to connect to HTTP API (usually port+10000)
    httpAddr := fmt.Sprintf("http://%s:%d/config", host, port+10000)
    
    resp, err := c.httpClient.Get(httpAddr)
    if err != nil {
        return nil, fmt.Errorf("failed to get config from %s: %w", httpAddr, err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        return nil, fmt.Errorf("unexpected status code %d from %s", resp.StatusCode, httpAddr)
    }

    var config map[string]interface{}
    if err := json.NewDecoder(resp.Body).Decode(&config); err != nil {
        return nil, fmt.Errorf("failed to decode config JSON: %w", err)
    }

    return config, nil
}

func (c *tidbCollector) getVariables(addr string) (map[string]string, error) {
    db, err := sql.Open("mysql", fmt.Sprintf("root@tcp(%s)/", addr))
    if err != nil {
        return nil, err
    }
    defer db.Close()

    rows, err := db.Query("SHOW GLOBAL VARIABLES")
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    variables := make(map[string]string)
    for rows.Next() {
        var name, value string
        if err := rows.Scan(&name, &value); err != nil {
            return nil, err
        }
        variables[name] = value
    }

    return variables, nil
}

// splitHostPort parses a network address of the form "host:port" into host and port.
// It handles both IPv4 and IPv6 addresses.
func splitHostPort(addr string) (host string, port int, err error) {
    // Handle IPv6 addresses with square brackets
    if strings.HasPrefix(addr, "[") {
        end := strings.Index(addr, "]")
        if end == -1 {
            return "", 0, fmt.Errorf("missing closing bracket in address: %s", addr)
        }
        host = addr[1:end]
        if len(addr) > end+1 && addr[end+1] == ':' {
            portStr := addr[end+2:]
            portNum, err := strconv.Atoi(portStr)
            if err != nil {
                return "", 0, fmt.Errorf("invalid port number: %s", portStr)
            }
            port = portNum
        } else {
            return "", 0, fmt.Errorf("missing port after IPv6 address: %s", addr)
        }
    } else {
        // Handle IPv4 addresses
        parts := strings.Split(addr, ":")
        if len(parts) != 2 {
            return "", 0, fmt.Errorf("address must be in format host:port: %s", addr)
        }
        host = parts[0]
        portNum, err := strconv.Atoi(parts[1])
        if err != nil {
            return "", 0, fmt.Errorf("invalid port number: %s", parts[1])
        }
        port = portNum
    }
    return host, port, nil
}
```

### 3.4. TiKV Collector (tikv_collector.go)

```go
package collector

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
        return nil, fmt.Errorf("failed to get TiKV version: %w", err)
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
```

### 3.5. PD Collector (pd_collector.go)

```go
package collector

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
        return nil, fmt.Errorf("failed to get PD version: %w", err)
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
```

## 4. TiUP Integration Considerations

### 4.1. Integration Approach

The collector module is designed to be used as a library by TiUP. TiUP should:

1. Obtain cluster topology information from its inventory
2. Extract connection information for TiDB, TiKV, and PD components
3. Call the collector APIs to gather configuration data
4. Pass the collected data to the analyzer module

### 4.2. Connection Information Handling

TiUP will need to provide the following connection information:

- TiDB MySQL protocol address (host:port)
- TiKV HTTP API addresses (host:port list)
- PD HTTP API addresses (host:port list)

The collector will handle:
- Converting TiDB MySQL port to HTTP API port (typically MySQL port + 10000)
- Trying multiple PD instances until one responds
- Gracefully handling unreachable or misconfigured components

### 4.3. Error Handling

The collector is designed to be resilient:
- Failure to collect from one component doesn't prevent collection from others
- Detailed error messages are returned for troubleshooting
- Timeouts are enforced to prevent hanging on unreachable components

### 4.4. Data Format

The collector returns data in a standardized format that can be:
- Serialized to JSON for file storage or transmission
- Used directly in-memory by the analyzer
- Extended in the future without breaking existing integrations

## 5. Testing Plan

### 5.1. Unit Tests

- Test data structure serialization/deserialization
- Test component collectors with mock HTTP servers
- Test error handling for various failure scenarios
- Test IPv6 address handling

### 5.2. Integration Tests

- Test end-to-end collection with real TiDB components
- Test behavior with partially available clusters
- Test performance with large configurations

### 5.3. Mock Servers

Create mock HTTP servers for TiKV and PD to enable testing without a full TiDB cluster:

```go
// Example mock server for testing
func TestTiKVCollector(t *testing.T) {
    // Start mock server
    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        switch r.URL.Path {
        case "/config":
            json.NewEncoder(w).Encode(mockConfig)
        case "/status":
            json.NewEncoder(w).Encode(mockStatus)
        }
    }))
    defer server.Close()

    // Test collector
    collector := NewTiKVCollector()
    states, err := collector.Collect([]string{server.URL})
    // Assert results
}
```

## 6. Implementation Considerations

### 6.1. Error Handling
- Network timeouts and retries
- Authentication failures
- Partial collection (some components succeed, others fail)
- Graceful degradation

### 6.2. Configuration
- Configurable timeouts
- Multiple endpoint support
- Secure connection support (TLS)

### 6.3. Performance
- Parallel collection from components
- Connection reuse
- Efficient data structures

### 6.4. Security
- Secure credential handling
- TLS support
- Minimal privilege requirements