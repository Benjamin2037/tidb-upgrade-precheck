# TiDB Upgrade Precheck Documentation

Welcome to the TiDB Upgrade Precheck project documentation.

## Quick Start

- **[Main README](../README.md)** - Project overview, quick start, and usage guide
- **[System Design](./design.md)** - System architecture and design overview
- **[Knowledge Base Generator Guide](./knowledge_generation_guide.md)** - How to generate the knowledge base

## Documentation Structure

### Core Documentation

- **[System Design](./design.md)** - High-level system architecture, core components, and workflow
- **[Knowledge Base Generation Guide](./knowledge_generation_guide.md)** - Detailed guide for knowledge base generation
- **[Deployment Guide](./deployment.md)** - Deployment options for different platforms (TiUP, TiDB Operator)

### Design Documents

- **[Design Documents Index](./design/README.md)** - Complete index of detailed design documents
  - [Parameter Comparison Design](./design/parameter_comparison/) - Parameter comparison capabilities
  - [Collector Design](./design/collector/) - Knowledge base generator and runtime collector
  - [Analyzer Design](./design/analyzer/) - Rule-based analyzer and rule development
  - [Report Generator Design](./design/reporter/) - Report generation and formats
  - [Architecture Design](./design/architecture/) - Detailed architecture documents

### Integration Documents

- **[TiUP Integration](./tiup/)** - TiUP integration design and implementation guides

## System Overview

The TiDB Upgrade Precheck system consists of three main components:

1. **Collector** - Collects configuration parameters and system variables
   - Knowledge Base Generator (offline): Generates parameter defaults from TiUP playground clusters
   - Runtime Collector (online): Collects current configuration from running clusters

2. **Analyzer** - Analyzes collected configuration against the knowledge base to identify risks
   - Rule-based risk assessment
   - Configuration comparison

3. **Report Generator** - Generates unified precheck reports in multiple formats (text, markdown, html, json)

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

> **Note**: This project is designed to be extensible. The current version (v1.0) focuses on parameter and system variable risk assessment as the initial implementation. Future versions will continuously add additional precheck capabilities, including execution plan jump detection, SQL compatibility checking, and other risk monitoring features. See the [Main README](../README.md#future-roadmap) and [System Design](./design.md#future-roadmap) for the complete roadmap.

## Documentation Reading Paths

### For New Users
1. Read the [Main README](../README.md) to understand the project overview
2. Read the [System Design](./design.md) for overall architecture
3. Read the [Knowledge Base Generation Guide](./knowledge_generation_guide.md) to get started

### For Developers
1. Read the [System Design](./design.md) for overall architecture
2. Read module-specific designs:
   - [Collector Design](./design/collector/) - Knowledge base generation and runtime collection
   - [Analyzer Design](./design/analyzer/) - Rule-based analysis
   - [Report Generator Design](./design/reporter/) - Report generation
3. Refer to [Detailed Design Documents](./design/README.md) for implementation details

### For Integration Developers
1. Read the [System Design](./design.md) to understand the system
2. Read [Deployment Guide](./deployment.md) for deployment options
3. Choose the appropriate deployment guide:
   - [TiUP Deployment Guide](./tiup_deployment.md) - For TiUP integration
   - [TiDB Operator Deployment Guide](./tidb_operator_deployment.md) - For TiDB Operator integration (TBD)
4. Refer to [TiUP Integration Documents](./tiup/) for detailed integration guides

## Related Resources

- [Main README](../README.md) - Project overview and usage guide
- [System Design](./design.md) - High-level system architecture

---

**Last Updated**: 2024
