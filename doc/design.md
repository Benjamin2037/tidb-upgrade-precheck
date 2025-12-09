# TiDB Upgrade Precheck System Design

## Overview

The TiDB Upgrade Precheck system identifies potential compatibility risks before upgrading a TiDB cluster by analyzing configuration parameters and system variables. It compares the current cluster configuration against a knowledge base of historical parameter defaults and upgrade logic.

## Current Scope (v1.0)

**This is the initial version of the TiDB Upgrade Precheck system, focusing on parameter and system variable risk assessment.**

**Current Capabilities:**
- Parameter default value changes
- System variable forced changes during upgrades
- Parameter deprecation and removal
- Configuration compatibility analysis

**Supported Components:**
- TiDB: Configuration parameters and system variables
- TiKV: Configuration parameters
- PD: Configuration parameters
- TiFlash: Configuration parameters

> **Important**: This project is designed to be extensible. The current version (v1.0) focuses on parameter and system variable risk assessment as the initial implementation. Future versions will continuously add additional precheck capabilities to build a comprehensive upgrade precheck platform.

## System Architecture

```
┌─────────────────────────────────────────────────────────┐
│              TiDB Upgrade Precheck System               │
├─────────────────────────────────────────────────────────┤
│                                                         │
│  ┌─────────────────────────────────────────────────┐    │
│  │              Collector                          │    │
│  │  - Knowledge Base Generator (offline)           │    │
│  │  - Runtime Collector (online)                   │    │
│  └──────────────────┬──────────────────────────────┘    │
│                     │                                   │
│  ┌──────────────────▼──────────────────────────────┐    │
│  │              Analyzer                           │    │
│  │  - Rule-based risk assessment                   │    │
│  │  - Pluggable rule system                        │    │
│  │  - Configuration comparison                     │    │
│  └──────────────────┬──────────────────────────────┘    │
│                     │                                   │
│  ┌──────────────────▼──────────────────────────────┐    │
│  │         Report Generator                        │    │
│  │  - Multiple formats (text/markdown/html/json)   │    │
│  └────────────────────────────────────────────────┘     │
│                                                         │
│  ┌─────────────────────────────────────────────────┐    │
│  │              Knowledge Base                     │    │
│  │  - Parameter defaults by version                │    │
│  │  - Upgrade logic                                │    │
│  └─────────────────────────────────────────────────┘    │
└─────────────────────────────────────────────────────────┘
```

## Core Components

### Collector

The collector consists of two parts: knowledge base generation (offline) and runtime collection (online).

#### Knowledge Base Generator (Offline)

Generates parameter defaults and upgrade logic from TiUP playground clusters and source code.

**Process:**
1. Start TiUP playground cluster for target version
2. Collect runtime configuration via `SHOW CONFIG` and `SHOW GLOBAL VARIABLES`
3. Extract bootstrap version from source code
4. Extract upgrade logic from `upgrade.go` (TiDB only)
5. Save to knowledge base directory

**Component-Specific Collection:**

**TiDB:**
- Runtime config: `SHOW CONFIG WHERE type='tidb'`
- System variables: `SHOW GLOBAL VARIABLES`
- Bootstrap version: Extracted from `pkg/session/upgrade.go` or `session/upgrade.go`
- Upgrade logic: Extracted from `upgradeToVerXX` functions in `pkg/session/upgrade.go`

**PD:**
- Runtime config: HTTP API `/pd/api/v1/config/default`

**TiKV:**
- User-set config: `last_tikv.toml` from playground data directory
- Runtime config: `SHOW CONFIG WHERE type='tikv'`
- Merged with priority: runtime > user-set

**TiFlash:**
- Default config: `tiflash.toml` from playground installation directory
- Runtime config: `SHOW CONFIG WHERE type='tiflash'`
- Merged with priority: runtime > default

**Output Structure:**
```
knowledge/
├── v6.5/
│   ├── v6.5.0/
│   │   ├── tidb/defaults.json
│   │   ├── pd/defaults.json
│   │   ├── tikv/defaults.json
│   │   └── tiflash/defaults.json
│   └── ...
├── tidb/
│   └── upgrade_logic.json
└── ...
```

#### Runtime Collector (Online)

Collects current configuration from running TiDB clusters.

**Collection Methods:**
- **TiDB**: `SHOW CONFIG WHERE type='tidb'` and `SHOW GLOBAL VARIABLES`
- **TiKV**: `SHOW CONFIG WHERE type='tikv'` and `last_tikv.toml`
- **PD**: HTTP API `/pd/api/v1/config/default`
- **TiFlash**: `SHOW CONFIG WHERE type='tiflash'` and `tiflash.toml`

**Connection Management:**
- Connection parameters (addresses, credentials) are provided by external tools (TiUP, TiDB Operator, etc.)
- The collector receives cluster topology information containing:
  - TiDB MySQL endpoint and credentials
  - TiKV HTTP API endpoints
  - PD HTTP API endpoints

### Analyzer

Compares runtime configuration against the knowledge base to identify risks using a rule-based architecture.

**Rule-Based Architecture Design:**

The analyzer adopts a pluggable rule-based architecture that enables sustainable and rapid addition of new check rules. This design provides significant architectural advantages:

- **Modular Rule System**: Each check rule is implemented as an independent module implementing the `Rule` interface, allowing new rules to be added without modifying existing code
- **Rapid Development**: New check rules can be developed and integrated quickly, typically requiring only implementing the `Rule` interface and adding the rule to the analyzer
- **Isolated Testing**: Each rule can be tested independently, ensuring reliability and maintainability
- **Flexible Configuration**: Rules can be enabled/disabled or configured independently, providing fine-grained control over the precheck process
- **Extensible Framework**: The rule-based architecture provides a solid foundation for continuously expanding precheck capabilities, from parameter checks to execution plan analysis and beyond

This architecture ensures that as new upgrade risks are identified, corresponding check rules can be quickly developed and integrated, making the system highly adaptable to evolving upgrade scenarios.

**Core Functions:**
- Risk assessment rules establishment
- Risk parameter identification
- Configuration comparison and compatibility analysis

**Current Rules:**

1. **User Modified Params Rule**
   - Detects parameters that differ from default values
   - Identifies user-customized configurations

2. **Upgrade Differences Rule**
   - Detects forced parameter changes during upgrades
   - Filters changes by bootstrap version range `(source, target]`
   - Categorizes by operation type (UPDATE, REPLACE, DELETE) with severity levels

3. **TiKV Consistency Rule**
   - Checks parameter consistency across TiKV nodes
   - Uses `last_tikv.toml` and `SHOW CONFIG WHERE type='tikv' AND instance='IP:port'`
   - Merges with priority: runtime > user-set

4. **High Risk Params Rule**
   - Validates manually specified high-risk parameters
   - Supports version range filtering
   - Checks against allowed values and source defaults

**Adding New Rules:**

To add a new check rule, developers simply need to:
1. Implement the `Rule` interface in `pkg/analyzer/rules/`
2. Add the rule to the analyzer's rule list
3. The rule will automatically be executed during precheck

This process typically takes minimal time and requires no changes to existing code, demonstrating the architecture's rapid extensibility.

**Process:**
1. Load knowledge base for source and target versions
2. Apply risk assessment rules to compare runtime configuration with knowledge base
3. Identify forced parameter changes
4. Detect deprecated or removed parameters
5. Assess compatibility risks

### Report Generator

Generates unified, standardized precheck reports in various formats.

**Supported Formats:**
- Text (console output)
- Markdown
- HTML
- JSON

**Report Contents:**
- Executive summary
- Risk assessment by severity
- Parameter change details
- Upgrade recommendations
- Component-specific analysis

## Workflow

### Knowledge Base Generation (Offline)

```
Source Code → TiUP Playground → Runtime Collection → Knowledge Base
```

1. **Version Selection**: Select target versions to collect
2. **Playground Start**: Start TiUP playground cluster for target version
3. **Runtime Collection**: Collect parameters via `SHOW CONFIG` and `SHOW GLOBAL VARIABLES`
4. **Code Extraction**: Extract bootstrap version and upgrade logic from source code
5. **Storage**: Store results in knowledge base directory structure

### Runtime Precheck (Online)

```
Running Cluster → Configuration Collection → Analysis → Report Generation
```

1. **Cluster Connection**: Connect to TiDB cluster components
2. **Configuration Collection**: Collect current configuration and system variables
3. **Knowledge Base Loading**: Load relevant knowledge base files
4. **Analysis**: Compare runtime configuration with knowledge base
5. **Report Generation**: Generate risk assessment report

## Knowledge Base Directory Structure

```
knowledge/
├── v6.5/                      # Version group (to second digit)
│   ├── v6.5.0/                # Specific version (to third digit)
│   │   ├── tidb/
│   │   │   └── defaults.json  # TiDB parameters and system variables
│   │   ├── tikv/
│   │   │   └── defaults.json  # TiKV parameters
│   │   ├── pd/
│   │   │   └── defaults.json  # PD parameters
│   │   └── tiflash/
│   │       └── defaults.json  # TiFlash parameters
│   └── v6.5.1/                # Next patch version
│       └── ...
├── tidb/                      # Component directory
│   └── upgrade_logic.json     # TiDB upgrade logic (forced changes)
└── ...
```

## Integration

### TiUP Integration

The system can be integrated into TiUP to provide precheck functionality during cluster upgrades.

**Integration Points:**
- `tiup cluster upgrade-precheck` - Standalone precheck command
- `tiup cluster upgrade --precheck` - Integrated into upgrade flow

## Future Roadmap

The TiDB Upgrade Precheck system is designed to be a comprehensive precheck platform. The current version (v1.0) focuses on parameter and system variable risk assessment as the initial implementation. Future versions will continuously add additional precheck capabilities.

### Planned Enhancements

**v2.0 - Execution Plan Jump Detection:**
- Detect execution plan changes that may impact query performance after upgrade
- Analyze query plan differences between source and target versions
- Identify potential performance regressions from plan changes
- SQL compatibility checking for statements that may behave differently

**v3.0+ - Additional Risk Items:**
- **Schema Change Compatibility**: Check for schema changes that may affect application compatibility
- **Feature Deprecation Detection**: Identify deprecated features that will be removed in the target version
- **Performance Regression Analysis**: Analyze potential performance regressions based on version changes
- **Additional Risk Monitoring**: Continuously expand risk detection capabilities based on real-world upgrade scenarios

### Extensibility

The system architecture is designed to support easy extension of new precheck rules and analysis capabilities, with the rule-based architecture as the core advantage:

- **Modular Rule System**: New analysis rules can be added by implementing the `Rule` interface, enabling rapid development and integration without modifying existing code
- **Rapid Extension**: The rule-based design allows new check rules to be developed and integrated quickly, typically requiring minimal code changes
- **Isolated Development**: Each rule is independent, allowing parallel development and isolated testing
- **Pluggable Collectors**: Additional data collection methods can be integrated through the collector interface
- **Extensible Knowledge Base**: Knowledge base structure supports adding new types of historical data
- **Flexible Report Generation**: Report formats can be extended to support new analysis results

The rule-based architecture ensures that as new upgrade risks are identified (execution plan changes, SQL compatibility issues, schema changes, etc.), corresponding check rules can be quickly developed and integrated, making the system highly adaptable and future-proof.

### Version Strategy

- **v1.0 (Current)**: Parameter and system variable risk assessment
- **v2.0 (Planning)**: Execution plan jump detection and SQL compatibility checking
- **v3.0+ (Future)**: Additional risk items and enhanced analysis capabilities

The project will continuously evolve based on real-world upgrade scenarios and user feedback.

## Related Documents

- **[Knowledge Base Generator Guide](./knowledge_generation_guide.md)** - Detailed guide for knowledge base generation
- **[Documentation Center](./README.md)** - Complete documentation index

---

**Last Updated**: 2025
