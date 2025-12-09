# PD Knowledge Base Generation

## Introduction

This document describes how to generate a knowledge base for PD (Placement Driver) component, which includes collecting default configuration parameters and upgrade logic across different versions. The knowledge base is essential for the PD parameter upgrade comparison functionality.

## Prerequisites

1. Access to the PD source code repository
2. Git installed and configured
3. Go development environment

## Knowledge Base Structure

The PD knowledge base is organized as follows:

```
knowledge/
└── pd/
    ├── v6.5.0/
    │   └── pd_defaults.json
    ├── v6.5.1/
    │   └── pd_defaults.json
    ├── v7.0.0/
    │   └── pd_defaults.json
    ├── upgrade_logic.json
    ├── upgrade_script.sh
    └── parameters-history.json
```

Each version directory contains a `pd_defaults.json` file with the configuration parameters and their default values for that version.

The `parameters-history.json` file contains the history of all parameters across all supported versions, enabling flexible comparison between any two versions.

## Generating Knowledge Base

### Single Version Collection

To collect the knowledge base for a single PD version:

```bash
# Build the tools first
make build

# Generate knowledge base for a specific PD version
make gen-kb-pd PD_REPO_ROOT=/path/to/pd/repo TAG=v6.5.0
```

This will create a `knowledge/pd/v6.5.0/pd_defaults.json` file containing the PD configuration parameters and their default values for version v6.5.0.

### Upgrade Logic Collection

To collect upgrade logic between two PD versions:

```bash
# Generate upgrade logic between two versions
make gen-ul-pd PD_REPO_ROOT=/path/to/pd/repo FROM_TAG=v6.5.0 TO_TAG=v7.1.0
```

This will create:
1. `knowledge/pd/upgrade_logic.json` - Contains information about parameter changes between versions
2. `knowledge/pd/upgrade_script.sh` - A script template for handling upgrade changes

### Parameter History Generation

To generate the parameter history across all supported versions:

```bash
# Generate parameter history for all supported versions
./bin/kb-generator -pd-repo /path/to/pd/repo -gen-history
```

This will create `knowledge/pd/parameters-history.json` containing the history of all parameters across all supported versions.

### Aggregating Knowledge Base

To aggregate all collected knowledge:

```bash
# Aggregate all PD knowledge
make agg-kb-pd
```

## Knowledge Base Format

### PD Defaults Format

The `pd_defaults.json` file follows this structure:

```json
{
  "version": "v6.5.0",
  "config_defaults": {
    "schedule.max-store-down-time": "30m",
    "schedule.leader-schedule-limit": 4,
    "replication.max-replicas": 3,
    "log.level": "info"
  },
  "bootstrap_version": 0
}
```

### Upgrade Logic Format

The `upgrade_logic.json` file follows this structure:

```json
[
  {
    "type": "modified",
    "key": "schedule.max-store-down-time",
    "from_value": "30m",
    "to_value": "1h",
    "description": "Parameter schedule.max-store-down-time was modified"
  },
  {
    "type": "added",
    "key": "security.encryption",
    "from_value": null,
    "to_value": {},
    "description": "Parameter security.encryption was added"
  }
]
```

### Parameter History Format

The `parameters-history.json` file follows this structure:

```json
{
  "component": "pd",
  "parameters": [
    {
      "name": "schedule.enable-diagnostic",
      "type": "bool",
      "history": [
        {
          "version": "v6.5.0",
          "default": false,
          "description": "Enable diagnostic mode for scheduling"
        },
        {
          "version": "v7.1.0",
          "default": true,
          "description": "Enable diagnostic mode for scheduling"
        }
      ]
    },
    {
      "name": "schedule.enable-cross-table-merge",
      "type": "bool",
      "history": [
        {
          "version": "v6.5.0",
          "default": true,
          "description": "Enable cross table merge"
        },
        {
          "version": "v7.5.0",
          "default": null,
          "description": "Removed in v7.5.0"
        }
      ]
    }
  ]
}
```

## Implementation Details

### Configuration Collection

The knowledge base generator analyzes the PD source code to extract configuration parameters:

1. Parses `server/config/config.go` to extract struct definitions
2. Extracts field tags to determine parameter names
3. Extracts default values from struct initialization
4. Collects comments as parameter descriptions

### Upgrade Logic Collection

The upgrade logic collector:

1. Compares configuration parameters between two versions
2. Identifies added, removed, and modified parameters
3. Generates descriptive messages for each change
4. Creates an upgrade script template with TODOs for handling changes

### Parameter History Management

The parameter history management system:

1. Collects parameter values from all supported versions
2. Organizes parameters by name with their values across versions
3. Enables efficient querying of parameter changes between any two versions
4. Supports detection of added, removed, and modified parameters

## Extending the Knowledge Base

To extend the knowledge base for additional PD versions:

1. Add new version tags to the PD repository
2. Run the knowledge base generation for each new version
3. Update the upgrade logic when new versions are added

## Future Improvements

1. **Enhanced Parsing**: Implement more sophisticated parsing of PD source code to extract detailed parameter information
2. **Dynamic Configuration**: Track dynamically configurable parameters that can be changed at runtime
3. **Feature Gates**: Monitor feature gates and experimental features across versions
4. **Cross-component Dependencies**: Identify parameters that interact with TiDB or TiKV settings
5. **Validation Rules**: Implement parameter value validation based on constraints