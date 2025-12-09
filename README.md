# tidb-upgrade-precheck

A precheck tool that provides TiDB cluster upgrade risk assessment and reporting to reduce overall risks for users during version upgrades.

## Overview

The project is currently in the initial architecture phase. It primarily uses parameter and system variable precheck to identify changes and incompatibilities before and after upgrades, forming the overall design and architecture of the TiDB Upgrade Precheck system.

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

> **Note**: This project is designed to be extensible. The current version (v1.0) focuses on parameter and system variable risk assessment as the initial implementation. Future versions will continuously add additional precheck capabilities.

## Future Roadmap

The TiDB Upgrade Precheck system is designed to be a comprehensive precheck platform. Future enhancements will include:

**Candidate Features:**
- **Execution Plan Jump Detection**: Detect execution plan changes that may impact query performance after upgrade
- **SQL Compatibility Checking**: Identify SQL statements that may behave differently in the target version
- **Schema Change Compatibility**: Check for schema changes that may affect application compatibility
- **Feature Deprecation Detection**: Identify deprecated features that will be removed in the target version
- **Performance Regression Analysis**: Analyze potential performance regressions based on version changes
- **Additional Risk Monitoring**: Continuously expand risk detection capabilities based on real-world upgrade scenarios

**Version Strategy:**
- **v1.0 (Current)**: Parameter and system variable risk assessment
- **v2.0 (Planning)**: Execution plan jump detection and SQL compatibility checking
- **v3.0+ (Future)**: Additional risk items and enhanced analysis capabilities

The system architecture is designed to support easy extension of new precheck rules and analysis capabilities.

## Quick Start

### Installation

```bash
make build
```

### Generate Knowledge Base

The knowledge base contains parameter defaults and upgrade logic for different TiDB versions. Generate it using:

```bash
# Generate for all LTS versions
./scripts/generate_knowledge.sh --serial

# Generate for specific version range
./scripts/generate_knowledge.sh --start-from=v7.5.0 --stop-at=v8.1.0 --serial
```

For detailed knowledge base generation guide, see [Knowledge Base Generation Guide](./doc/knowledge_generation_guide.md).

### Using Precheck

The precheck functionality is typically integrated into cluster management tools rather than run directly. The system is designed to be used through:

**TiUP Integration:**
```bash
# Standalone precheck command
tiup cluster upgrade-precheck <cluster-name> <version>

# Integrated into upgrade command
tiup cluster upgrade <cluster-name> <version> --precheck
```

**TiDB Operator Integration（TBD）:**
The precheck can be integrated into TiDB Operator upgrade workflows to automatically perform compatibility checks before upgrades.

**Direct Usage (Development/Testing):**
For development or testing purposes, you can run the precheck command directly:
```bash
./bin/precheck \
  --target-version=v8.1.0 \
  --topology-file=/path/to/topology.yaml \
  --format=html \
  --output-dir=./reports
```

For detailed integration guides, see [TiUP Integration Documents](./doc/design/tiup/).

## System Architecture

```
┌─────────────────────────────────────────────────────────────────────┐
│                        Consumer Layer                               │
├─────────────────────────────────────────────────────────────────────┤
│  TiUP CLI    │  TiDB Operator    │  Other Tools                     │
└─────────────────────────────────────────────────────────────────────┘
                                │
┌─────────────────────────────────────────────────────────────────────┐
│                       Integration Layer                             │
├─────────────────────────────────────────────────────────────────────┤
│                   tidb-upgrade-precheck Library                     │
└─────────────────────────────────────────────────────────────────────┘
                                │
┌─────────────────────────────────────────────────────────────────────┐
│                        Analysis Layer                               │
├─────────────────────────────────────────────────────────────────────┤
│   Analyzer   │  Report Generator  │  Rules Engine                   │
└─────────────────────────────────────────────────────────────────────┘
                                │
┌─────────────────────────────────────────────────────────────────────┐
│                        Collection Layer                             │
├─────────────────────────────────────────────────────────────────────┤
│           Runtime Collector          │        KB Generator          │
└─────────────────────────────────────────────────────────────────────┘
                                │
┌─────────────────────────────────────────────────────────────────────┐
│                        Data Sources                                 │
├─────────────────────────────────────────────────────────────────────┤
│  TiDB Source  │  TiUP Playground  │  Running Cluster                │
└─────────────────────────────────────────────────────────────────────┘
```

### Architecture Advantages

**Rule-Based Design for Rapid Extension:**

The system adopts a rule-based architecture that enables sustainable and rapid addition of new check rules. This design provides significant advantages:

- **Modular Rule System**: Each check rule is implemented as an independent module implementing the `Rule` interface, allowing new rules to be added without modifying existing code
- **Rapid Development**: New check rules can be developed and integrated quickly, typically requiring only implementing the `Rule` interface and adding the rule to the analyzer
- **Isolated Testing**: Each rule can be tested independently, ensuring reliability and maintainability
- **Flexible Configuration**: Rules can be enabled/disabled or configured independently, providing fine-grained control over the precheck process
- **Extensible Framework**: The rule-based architecture provides a solid foundation for continuously expanding precheck capabilities, from parameter checks to execution plan analysis and beyond

This architecture ensures that as new upgrade risks are identified, corresponding check rules can be quickly developed and integrated, making the system highly adaptable to evolving upgrade scenarios.

## Core Components

### 1. Collector

The collector consists of two parts:

- **Knowledge Base Generator (Offline)**: Generates parameter defaults and upgrade logic from TiUP playground clusters and source code
- **Runtime Collector (Online)**: Collects current configuration from running TiDB clusters

For detailed design and implementation, see [Collector Design](./doc/design/collector/README.md).

### 2. Analyzer

Compares runtime configuration against the knowledge base to identify risks using a rule-based architecture.

**Current Rules:**
- **User Modified Params Rule**: Detects parameters modified from defaults
- **Upgrade Differences Rule**: Detects forced parameter changes during upgrades
- **TiKV Consistency Rule**: Checks parameter consistency across TiKV nodes
- **High Risk Params Rule**: Validates manually specified high-risk parameters

For detailed design and implementation, including how to add new rules, see [Analyzer Design](./doc/design/analyzer/README.md).

### 3. Report Generator

Generates precheck reports in multiple formats (text, markdown, HTML, JSON).

For detailed design and implementation, see [Report Generator Design](./doc/design/reporter/README.md).

### 4. Knowledge Base

Stores parameter defaults and upgrade logic for different TiDB versions, organized by version and component.

For detailed knowledge base structure and generation process, see [knowledge base generator Guide](./doc/knowledge_generation_guide.md).

## Documentation

### High-Level Documentation

- **[System Design](./doc/design.md)** - System architecture and design overview
- **[Documentation Center](./doc/README.md)** - Complete documentation index

### Detailed Design Documents

- **[Parameter Comparison Design](./doc/design/parameter_comparison/)** - Detailed design and implementation of parameter comparison capabilities
- **[Collector Design](./doc/design/collector/README.md)** - Knowledge base generator and runtime collector design
- **[Analyzer Design](./doc/design/analyzer/README.md)** - Rule-based analyzer design and rule development guide
- **[Report Generator Design](./doc/design/reporter/README.md)** - Report generation design and format specifications

### User Guides

- **[Knowledge Base Generation Guide](./doc/knowledge_generation_guide.md)** - Detailed guide for knowledge base generation

## Contributing

Issues and pull requests are welcome.
