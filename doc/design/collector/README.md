# Collector Design

This document describes the detailed design and implementation of the collector module, including both knowledge base generation (offline) and runtime collection (online).

## Overview

The collector module is responsible for collecting configuration parameters and system variables from TiDB cluster components. It consists of two parts:

1. **Knowledge Base Generator (Offline)**: Generates parameter defaults and upgrade logic from TiUP playground clusters and source code
2. **Runtime Collector (Online)**: Collects current configuration from running TiDB clusters

## Knowledge Base Generator

### Architecture

The knowledge base generator uses TiUP playground clusters to collect runtime configuration, then extracts additional metadata from source code.

**Key Design Principles:**
- Collect runtime configuration directly from TiUP playground clusters (most accurate)
- Extract bootstrap version from source code (needed for upgrade logic filtering)
- Extract upgrade logic from source code (TiDB only, from `upgradeToVerXX` functions)
- No code-based parameter extraction needed (playground provides complete defaults)

### Process Flow

1. **Version Selection**: Select target versions to collect
2. **Playground Start**: Start TiUP playground cluster for target version (managed by `cmd/kb_generator/main.go`)
3. **Runtime Collection**: Collect parameters via `SHOW CONFIG` and `SHOW GLOBAL VARIABLES`
4. **Code Extraction**: Extract bootstrap version and upgrade logic from source code
5. **Storage**: Store results in knowledge base directory structure

### Component-Specific Implementation

#### TiDB Collector

**Location**: `pkg/kbgenerator/tidb/collector.go`

**Collection Methods:**
- Runtime config: `SHOW CONFIG WHERE type='tidb'` (via runtime collector)
- System variables: `SHOW GLOBAL VARIABLES` (via runtime collector)
- Bootstrap version: Extracted from `pkg/session/upgrade.go` or `session/upgrade.go` (via `extractBootstrapVersion`)
- Upgrade logic: Extracted from `upgradeToVerXX` functions in `pkg/session/upgrade.go` (via `CollectUpgradeLogicFromSource`)

**Key Functions:**
- `Collect(tidbRoot, version, tag)`: Main collection function (assumes playground is already running)
- `StartPlayground(version, tag)`: Start TiUP playground cluster (exported for use by main.go)
- `WaitForClusterReady(tag, port)`: Wait for cluster to be ready (exported for use by main.go)
- `StopPlayground(tag)`: Stop and clean up playground cluster (exported for use by main.go)
- `extractBootstrapVersion(tidbRoot, version)`: Extract bootstrap version from source code

**Note**: Playground lifecycle (start/stop/wait) is managed by `cmd/kb_generator/main.go`. The `Collect` function assumes the playground is already running and ready.

#### PD Collector

**Location**: `pkg/kbgenerator/pd/collector.go`

**Collection Methods:**
- Runtime config: HTTP API `/pd/api/v1/config/default` (via runtime collector)

**Key Functions:**
- `Collect(pdRoot, version, pdAddr)`: Main collection function
- Uses `runtimeCollector.NewPDCollector().CollectDefaults()` directly

**Note**: PD doesn't use bootstrap version for upgrade logic, so `BootstrapVersion` is set to 0.

#### TiKV Collector

**Location**: `pkg/kbgenerator/tikv/collector.go`

**Collection Methods:**
- User-set config: `last_tikv.toml` from playground data directory (`~/.tiup/data/{tag}/tikv-{port}/data/last_tikv.toml`)
- Runtime config: `SHOW CONFIG WHERE type='tikv'` (via TiDB connection)
- Merged with priority: runtime > user-set

**Key Functions:**
- `Collect(tikvRoot, version, tidbPort, tag)`: Main collection function
- `collectTiKVConfigFromFile(tikvDataDir)`: Collect from `last_tikv.toml`
- `collectTiKVConfigViaSHOWCONFIG(tidbPort)`: Collect via `SHOW CONFIG`
- `mergeConfigsWithPriority(userConfig, runtimeConfig)`: Merge configurations

#### TiFlash Collector

**Location**: `pkg/kbgenerator/tiflash/collector.go`

**Collection Methods:**
- Default config: `tiflash.toml` from playground installation directory
- Runtime config: `SHOW CONFIG WHERE type='tiflash'` (via TiDB connection)
- Merged with priority: runtime > default

**Key Functions:**
- `Collect(tiflashRoot, version, tidbPort, tag)`: Main collection function
- `collectTiFlashConfigFromFile(tiflashInstallDir)`: Collect from `tiflash.toml`
- `collectTiFlashConfigViaSHOWCONFIG(tidbPort)`: Collect via `SHOW CONFIG`
- `mergeConfigsWithPriority(defaultConfig, runtimeConfig)`: Merge configurations

**Note**: TiFlash doesn't collect system variables separately, as TiDB handles that uniformly.

## Runtime Collector

### Architecture

The runtime collector connects to live TiDB clusters to collect current configuration state. It supports optimized collection based on data requirements.

**Key Design Principles:**
- **Optimized Collection**: Supports `CollectDataRequirements` to collect only necessary data
- **Component-Specific Collectors**: Each component has its own collector implementation
- **Unified Interface**: All collectors implement consistent interfaces

### Collection Optimization

The runtime collector supports collecting only the data needed by the analyzer:

```go
type CollectDataRequirements struct {
    Components          []string // Which components to collect
    NeedConfig          bool     // Whether config parameters are needed
    NeedSystemVariables bool     // Whether system variables are needed
    NeedAllTikvNodes    bool     // Whether all TiKV nodes are needed (for consistency checks)
}
```

This allows the analyzer to optimize collection by only gathering data required by active rules.

### Component-Specific Implementation

#### TiDB Runtime Collector

**Location**: `pkg/collector/runtime/tidb/collector.go`

**Collection Methods:**
- `SHOW CONFIG WHERE type='tidb'`
- `SHOW GLOBAL VARIABLES`

**Key Functions:**
- `Collect(addr, user, password)`: Main collection function
- `GetConfigByType(addr, user, password, componentType)`: Get configuration by component type (exported for reuse)
- `GetConfigByTypeAndInstance(addr, user, password, componentType, instance)`: Get configuration for specific instance (for TiKV consistency checks)

#### PD Runtime Collector

**Location**: `pkg/collector/runtime/pd/collector.go`

**Collection Methods:**
- HTTP API `/pd/api/v1/config/default` (for default values)
- HTTP API `/pd/api/v1/config` (for current values)

**Key Functions:**
- `Collect(addrs)`: Collect from PD instances
- `CollectDefaults(addrs)`: Collect default configuration only

#### TiKV Runtime Collector

**Location**: `pkg/collector/runtime/tikv/collector.go`

**Collection Methods:**
- HTTP API endpoints for TiKV status and configuration

**Key Functions:**
- `Collect(addrs, dataDirs)`: Collect from TiKV instances
- Supports collecting from multiple TiKV nodes for consistency checks

#### TiFlash Runtime Collector

**Location**: `pkg/collector/runtime/tiflash/collector.go`

**Collection Methods:**
- HTTP API endpoints for TiFlash status and configuration

**Key Functions:**
- `Collect(addrs)`: Collect from TiFlash instances

### Unified Collector Interface

**Location**: `pkg/collector/runtime/collector.go`

The unified `Collector` provides a single interface for collecting from all components:

```go
type Collector struct {
    tidbCollector    tidb.TiDBCollector
    pdCollector      pd.PDCollector
    tikvCollector    tikv.TiKVCollector
    tiflashCollector tiflash.TiFlashCollector
}

func (c *Collector) Collect(endpoints ClusterEndpoints, req *CollectDataRequirements) (*ClusterSnapshot, error)
```

## Data Structures

See [Types Definition](../../../pkg/types/defaults_types.go) for detailed data structures.

**Key Types:**
- `KBSnapshot`: Knowledge base snapshot for a specific version and component
- `ComponentState`: Runtime state of a component (config, variables, version)
- `ClusterSnapshot`: Complete snapshot of a cluster's runtime state

## Implementation Plan

- **[Collector Implementation Plan](./collector_implementation_plan.md)** - Detailed implementation plan for the collector module, including data structures, interfaces, and component-specific collection logic

## Related Documents

- [Parameter Comparison Design](../parameter_comparison/) - Parameter comparison capabilities
- [Knowledge Base Generation Guide](../../knowledge_generation_guide.md) - User guide for knowledge base generation
