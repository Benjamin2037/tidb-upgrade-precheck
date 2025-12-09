# Detailed Design for TiDB Upgrade Precheck System

This document provides a comprehensive overview of the TiDB upgrade precheck system's architecture, components, and implementation details.

## Table of Contents

1. [Overview](#overview)
2. [Architecture](#architecture)
3. [Core Components](#core-components)
   - [Knowledge Base Generator](#knowledge-base-generator)
   - [Runtime Collector](#runtime-collector)
   - [Analyzer](#analyzer)
   - [Report Generator](#report-generator)
4. [Data Structures](#data-structures)
5. [Implementation Plan](#implementation-plan)
6. [TiUP Integration](#tiup-integration)
7. [Testing Strategy](#testing-strategy)

## Overview

The TiDB upgrade precheck system is designed to identify potential compatibility issues before upgrading a TiDB cluster. It analyzes both static knowledge from TiDB source code and runtime configuration from the actual cluster to provide comprehensive risk assessment.

## Architecture

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
│   Analyzer   │  Report Generator  │  Rules Engine  │  Data Models   │
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
│  TiDB Source  │  TiDB Cluster  │  GitHub Metadata  │  Manual Input  │
└─────────────────────────────────────────────────────────────────────┘
```

## Core Components

### Knowledge Base Generator

The Knowledge Base Generator extracts historical parameter and upgrade logic information from TiDB source code. It consists of:

1. **Parameter History Collector**:
   - Extracts parameter definitions and their evolution across versions
   - Focuses on TiDB system variables, TiKV/PD configuration parameters
   - Outputs structured JSON with parameter history

2. **Upgrade Logic Collector**:
   - Parses upgrade.go to extract upgrade operations
   - Identifies INSERT/UPDATE/DELETE operations on system tables
   - Records forced value changes during upgrades

### Runtime Collector

The Runtime Collector connects to live TiDB clusters to collect current configuration state:

1. **TiDB Collector**:
   - Connects via MySQL protocol
   - Retrieves GLOBAL_VARIABLES
   - Collects configuration via SHOW CONFIG

2. **TiKV Collector**:
   - Connects via HTTP API
   - Retrieves configuration information

3. **PD Collector**:
   - Connects via HTTP API
   - Retrieves configuration information

### Analyzer

The Analyzer compares runtime configuration against the knowledge base to identify risks:

1. **Configuration Analysis**:
   - Compares current values with version-specific defaults
   - Identifies user-modified parameters
   - Detects parameters with changing defaults

2. **Upgrade Logic Analysis**:
   - Identifies parameters that will be forcibly changed
   - Warns about deprecated features
   - Checks for incompatible changes

### Report Generator

The Report Generator produces actionable reports in various formats:

1. **Text Reports**: Simple console output
2. **JSON Reports**: Structured data for programmatic consumption
3. **Markdown Reports**: Human-readable formatted output
4. **HTML Reports**: Rich interactive reports

## Data Structures

### Cluster Snapshot
```go
type ClusterSnapshot struct {
    Timestamp     time.Time              `json:"timestamp"`
    SourceVersion string                 `json:"source_version"`
    TargetVersion string                 `json:"target_version"`
    Components    map[string]ComponentState `json:"components"`
}

type ComponentState struct {
    Type      string                 `json:"type"`        
    Version   string                 `json:"version"`
    Config    map[string]interface{} `json:"config"`      
    Variables map[string]string      `json:"variables"`   
    Status    map[string]interface{} `json:"status"`      
}
```

### Knowledge Base
```json
{
  "parameters": [
    {
      "component": "tidb",
      "name": "tidb_enable_async_commit",
      "history": [
        {
          "version": "5.0.0",
          "default": "false",
          "type": "bool",
          "scope": "global",
          "dynamic": true,
          "description": "Enables async commit for the transaction"
        },
        {
          "version": "5.1.0",
          "default": "true",
          "type": "bool",
          "scope": "global",
          "dynamic": true,
          "description": "Enables async commit for the transaction"
        }
      ]
    }
  ],
  "upgrade_logic": [
    {
      "version": 66,
      "changes": [
        {
          "from_version": 65,
          "to_version": 66,
          "kind": "sysvar",
          "target": "tidb_track_aggregate_memory_usage",
          "default_value": "ON",
          "force": true,
          "summary": "Enable aggregate memory usage tracking",
          "scope": "global",
          "optional_hints": ["Confirm whether workloads rely on the legacy behavior"]
        }
      ]
    }
  ]
}
```

## Implementation Plan

See [Collector Implementation Plan](collector_implementation_plan.md) and [Analyzer Implementation Plan](analyzer_implementation_plan.md).

## TiUP Integration

There are multiple ways to integrate tidb-upgrade-precheck with TiUP:

1. [TiUP Integration Design](tiup_integration_design.md) - Creating a new dedicated command for prechecks
2. [TiUP Upgrade Command Integration](tiup_upgrade_integration.md) - Integrating prechecks into the existing `tiup cluster upgrade` command
3. [TiUP Cluster Precheck Command Design](tiup_cluster_precheck_command.md) - Detailed design of the precheck command in TiUP Cluster
4. [TiUP Integration Implementation Guide](tiup_integration_implementation_guide.md) - Detailed implementation guide for integrating tidb-upgrade-precheck into TiUP
5. [TiUP Implementation Steps](tiup_implementation_steps.md) - Step-by-step implementation guide for integrating tidb-upgrade-precheck into TiUP
6. [TiUP Integration Manual](tiup_integration_manual.md) - Complete manual for implementing TiUP integration

## Testing Strategy

The testing strategy includes:

1. **Unit Tests**:
   - Test individual functions and methods
   - Mock external dependencies
   - Achieve high code coverage

2. **Integration Tests**:
   - Test component interactions
   - Validate data flow between modules
   - Test with real cluster data

3. **End-to-End Tests**:
   - Full workflow testing
   - Validate output formats
   - Test error conditions

4. **Performance Tests**:
   - Measure collection performance
   - Validate scalability
   - Test with large clusters