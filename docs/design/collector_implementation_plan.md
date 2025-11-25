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
    // Assuming TiDB HTTP API is available on the same host but different port
    hostPort := addr
    // TODO: Determine HTTP API port from MySQL port or configuration
    // For now, assuming standard setup where MySQL is on 4000 and HTTP on 10080
    // This needs to be configurable
    
    resp, err := c.httpClient.Get(fmt.Sprintf("http://%s/config", hostPort))
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

    return variables, rows.Err()
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

## 4. Usage Example

```go
package main

import (
    "encoding/json"
    "fmt"
    "os"

    "github.com/pingcap/tidb-upgrade-precheck/pkg/collector"
)

func main() {
    // Create collector
    c := collector.NewCollector()

    // Define cluster endpoints
    endpoints := collector.ClusterEndpoints{
        TiDBAddr:  "127.0.0.1:4000",
        TiKVAddrs: []string{"127.0.0.1:20180"},
        PDAddrs:   []string{"127.0.0.1:2379"},
    }

    // Collect cluster snapshot
    snapshot, err := c.Collect(endpoints)
    if err != nil {
        fmt.Printf("Error collecting cluster info: %v\n", err)
        os.Exit(1)
    }

    // Output as JSON
    data, err := json.MarshalIndent(snapshot, "", "  ")
    if err != nil {
        fmt.Printf("Error marshaling snapshot: %v\n", err)
        os.Exit(1)
    }

    fmt.Println(string(data))
}
```

## 5. Implementation Considerations

### 5.1. Error Handling
- Network timeouts and retries
- Authentication failures
- Partial collection (some components succeed, others fail)
- Graceful degradation

### 5.2. Configuration
- Configurable timeouts
- Multiple endpoint support
- Secure connection support (TLS)

### 5.3. Performance
- Parallel collection from components
- Connection reuse
- Efficient data structures

### 5.4. Security
- Secure credential handling
- TLS support
- Minimal privilege requirements