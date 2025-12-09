# Detailed System Design

This document provides a comprehensive overview of the TiDB Upgrade Precheck system's architecture, components, and implementation details.

## Overview

The TiDB Upgrade Precheck system identifies potential compatibility issues before upgrading a TiDB cluster. It analyzes both static knowledge from TiDB source code and runtime configuration from the actual cluster to provide comprehensive risk assessment.

## System Architecture

```
┌─────────────────────────────────────────────────────────────────────┐
│                        Consumer Layer                              │
├─────────────────────────────────────────────────────────────────────┤
│  TiUP CLI    │  TiDB Operator    │  Other Tools                     │
└─────────────────────────────────────────────────────────────────────┘
                                │
┌─────────────────────────────────────────────────────────────────────┐
│                       Integration Layer                            │
├─────────────────────────────────────────────────────────────────────┤
│                   tidb-upgrade-precheck Library                    │
└─────────────────────────────────────────────────────────────────────┘
                                │
┌─────────────────────────────────────────────────────────────────────┐
│                        Analysis Layer                              │
├─────────────────────────────────────────────────────────────────────┤
│   Analyzer   │  Report Generator  │  Rules Engine                   │
└─────────────────────────────────────────────────────────────────────┘
                                │
┌─────────────────────────────────────────────────────────────────────┐
│                        Collection Layer                           │
├─────────────────────────────────────────────────────────────────────┤
│           Runtime Collector          │        KB Generator         │
└─────────────────────────────────────────────────────────────────────┘
                                │
┌─────────────────────────────────────────────────────────────────────┐
│                        Data Sources                                │
├─────────────────────────────────────────────────────────────────────┤
│  TiDB Source  │  TiUP Playground  │  Running Cluster               │
└─────────────────────────────────────────────────────────────────────┘
```

## Core Components

### Knowledge Base Generator

**Location**: `pkg/kbgenerator/`

The Knowledge Base Generator collects parameter defaults and upgrade logic from TiUP playground clusters and source code.

**Key Features:**
- Uses TiUP playground clusters to collect runtime configuration (most accurate)
- Extracts bootstrap version from source code (for upgrade logic filtering)
- Extracts upgrade logic from source code (TiDB only, from `upgradeToVerXX` functions)
- Supports TiDB, PD, TiKV, and TiFlash components

**Process:**
1. Start TiUP playground cluster for target version
2. Collect runtime configuration via `SHOW CONFIG` and `SHOW GLOBAL VARIABLES`
3. Extract bootstrap version from source code
4. Extract upgrade logic from `upgrade.go` (TiDB only)
5. Store results in knowledge base directory structure

For detailed design, see [Collector Design](../knowledge_generation_guide.md).

### Runtime Collector

**Location**: `pkg/collector/runtime/`

The Runtime Collector connects to live TiDB clusters to collect current configuration state.

**Key Features:**
- Supports optimized collection based on data requirements
- Component-specific collectors for TiDB, PD, TiKV, TiFlash
- Unified interface for all components

**Collection Methods:**
- **TiDB**: `SHOW CONFIG WHERE type='tidb'` and `SHOW GLOBAL VARIABLES`
- **PD**: HTTP API `/pd/api/v1/config/default`
- **TiKV**: HTTP API endpoints and `last_tikv.toml`
- **TiFlash**: HTTP API endpoints and `tiflash.toml`

For detailed design, see [Collector Design](../collector/README.md).

### Analyzer

**Location**: `pkg/analyzer/`

The Analyzer compares runtime configuration against the knowledge base to identify risks using a rule-based architecture.

**Key Features:**
- Rule-based architecture for rapid extension
- Optimized data loading (only loads data needed by active rules)
- Shared rule context for efficient data access
- Results organized by category and severity

**Default Rules:**
- User Modified Params Rule
- Upgrade Differences Rule
- TiKV Consistency Rule

For detailed design, see [Analyzer Design](../analyzer/).

### Report Generator

**Location**: `pkg/reporter/`

The Report Generator produces actionable reports in various formats.

**Supported Formats:**
- **Text**: Simple console output
- **Markdown**: Human-readable formatted output
- **HTML**: Rich interactive reports
- **JSON**: Structured data for programmatic consumption

For detailed design, see [Report Generator Design](../reporter/).

## Data Structures

### Cluster Snapshot

```go
type ClusterSnapshot struct {
    Timestamp     time.Time
    SourceVersion string
    Components    map[string]ComponentState
}

type ComponentState struct {
    Type      string
    Version   string
    Config    ConfigDefaults
    Variables SystemVariables
    Status    map[string]interface{}
}
```

### Knowledge Base Snapshot

```go
type KBSnapshot struct {
    Component        string
    Version          string
    BootstrapVersion int64
    ConfigDefaults   ConfigDefaults
    SystemVariables  SystemVariables
}
```

### Analysis Result

```go
type AnalysisResult struct {
    SourceVersion string
    TargetVersion string
    Timestamp     time.Time
    Categories    map[string]CategoryResult
}

type CategoryResult struct {
    Category string
    Severity string
    Items    []CheckResult
}
```

For detailed type definitions, see [Types Definition](../../../pkg/types/defaults_types.go).

## Workflow

### Knowledge Base Generation (Offline)

```
Source Code → TiUP Playground → Runtime Collection → Knowledge Base
```

1. Select target versions
2. Start TiUP playground cluster
3. Collect runtime configuration
4. Extract bootstrap version and upgrade logic from source code
5. Store in knowledge base directory

### Runtime Precheck (Online)

```
Running Cluster → Configuration Collection → Analysis → Report Generation
```

1. Connect to TiDB cluster components
2. Collect current configuration and system variables
3. Load relevant knowledge base files
4. Execute rules to compare configuration
5. Generate risk assessment report

## Related Documents

- [System Design Overview](../../design.md) - High-level system architecture
- [Collector Design](../collector/) - Knowledge base generator and runtime collector
- [Analyzer Design](../analyzer/) - Rule-based analyzer
- [Report Generator Design](../reporter/) - Report generation
- [Parameter Comparison Design](../parameter_comparison/) - Parameter comparison capabilities
- [TiUP Integration](../tiup/) - TiUP integration design and guides

---

**Last Updated**: 2024
