# Detailed Design Documents

This directory contains detailed design documents for the TiDB Upgrade Precheck system.

> **Note**: For a high-level overview of the system architecture and module organization, see the [System Design Overview](../design.md).

## 📚 Documentation Index

### 🏗️ Architecture Design

- **[Architecture Design Documents](./architecture/detailed_design.md)** - High-level architecture and comprehensive design documents

### 🛠️ Core Module Designs

- **[Parameter Comparison Design](./parameter_comparison/README.md)** - Detailed design and implementation of parameter comparison capabilities
- **[Collector Design](./collector/README.md)** - Knowledge base generator and runtime collector design
- **[Analyzer Design](./analyzer/README.md)** - Rule-based analyzer design and rule development guide
- **[Report Generator Design](./reporter/README.md)** - Report generation design and format specifications

### 🔌 TiUP Integration

- **[TiUP Integration Documents](../tiup/README.md)** - Complete TiUP integration design and implementation guides


## 🗺️ Documentation Reading Recommendations

### Understanding System Architecture
1. Start with the [System Design Overview](../design.md) to understand the overall architecture and module organization
2. Read [Architecture Design Documents](./architecture/) for comprehensive system design
3. Read component-specific designs:
   - [Collector Design](./collector/) - Component-specific collection methods
   - [Parameter Comparison Design](./parameter_comparison/) - Parameter comparison capabilities

### Implementing Core Modules
1. Read the corresponding core module design:
   - [Collector Design](./collector/) - For knowledge base generation and runtime collection
   - [Analyzer Design](./analyzer/) - For risk analysis and rule development
   - [Report Generator Design](./reporter/) - For report generation
2. Check the implementation plan documents in each module directory
3. Refer to component-specific designs for detailed implementation details

### Integrating with TiUP
1. Start with [TiUP Integration and Deployment Guide](../tiup_deployment.md) to understand the overall solution
2. Follow [TiUP Implementation Guide](../tiup/tiup_implementation_guide.md) for step-by-step implementation
3. Refer to [TiUP Integration Patch](../tiup/tiup_integration_patch.diff) for code change examples

## 📁 Documentation Organization

```
doc/
├── design.md                    # Main design document
│
└── design/                      # Detailed design documents
    ├── README.md                # This document
    │
    ├── architecture/            # Architecture design documents
    │   └── detailed_design.md
    │
    ├── parameter_comparison/    # Parameter comparison design
    │   └── README.md
    │
    ├── collector/               # Collector design
    │   └── README.md
    │
    ├── analyzer/                # Analyzer design
    │   └── README.md
    │
    └── reporter/                # Reporter design
        └── README.md
```

## 🔗 Related Documents

- [Documentation Center README](../README.md) - Unified entry point for all documentation
- [Main README](../../README.md) - Project overview and usage guide
- [System Design Overview](../design.md) - High-level system architecture and module overview

## 📝 Documentation Maintenance

Design documents are continuously updated as the system evolves. If you find inconsistencies between documentation and implementation, please:
1. Submit an Issue to report it
2. Or submit a PR to update the documentation

---

**Last Updated**: 2024
