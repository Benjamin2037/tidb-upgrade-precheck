# TiUP Integration and Deployment Guide

This document provides a complete guide for integrating and deploying `tidb-upgrade-precheck` with TiUP, including design, packaging, and implementation details.

## Overview

The precheck tool is packaged as a standalone TiUP component (`upgrade-precheck`) that is automatically installed and updated with TiUP. The component includes the complete knowledge base and provides runtime data storage for logs and user configurations.

## Integration Architecture

```
┌─────────────────┐
│   tiup-cluster  │
└────────┬────────┘
         │ calls
         ▼
┌─────────────────────────┐
│  tiup upgrade-precheck  │  (standalone component)
└────────┬─────────────────┘
         │ uses
         ▼
┌──────────────────────────┐
│  tidb-upgrade-precheck   │
│  - Knowledge Base         │
│  - Runtime Collector      │
│  - Analyzer               │
│  - Report Generator       │
└────────┬──────────────────┘
         │ connects to
         ▼
┌─────────────────┐
│  TiDB Cluster   │
└─────────────────┘
```

## Integration Approach

### Component-Based Integration

The precheck tool is packaged as a standalone TiUP component (`upgrade-precheck`), not as a library dependency. This approach:

- **Simplifies Integration**: `tiup-cluster` calls `tiup upgrade-precheck` as a subprocess
- **Automatic Installation**: Automatically installed and updated with TiUP
- **Self-Contained**: Component includes its own knowledge base
- **Independent Updates**: Component can be updated independently
- **Isolated Runtime Data**: Logs and configurations stored separately

### Command Integration

**Standalone Command**:
```bash
tiup upgrade-precheck <cluster-name> <target-version> [flags]
```

**Integrated into Upgrade Command**:
```bash
tiup cluster upgrade <cluster-name> <target-version> --precheck
```

## Deployment Model

### Directory Structure

**TiUP Component Installation**:
```
~/.tiup/components/upgrade-precheck/<version>/
├── tidb-upgrade-precheck     # Precheck executable binary
└── knowledge/                # Complete knowledge base (packaged with component)
    ├── v7.5/
    │   └── v7.5.1/
    │       ├── tidb/defaults.json
    │       ├── pd/defaults.json
    │       ├── tikv/defaults.json
    │       └── tiflash/defaults.json
    ├── v8.1/
    │   └── v8.1.0/
    │       └── ...
    └── tidb/
        └── upgrade_logic.json
```

**Runtime Data Storage** (persistent across component updates):
```
~/.tiup/storage/upgrade-precheck/
├── logs/                     # Execution logs
│   ├── precheck_20240101_120000.log
│   └── ...
└── config/                   # User configurations
    └── high_risk_params.json # User-defined high-risk parameters
```

### Advantages

1. **Automatic Installation**: Automatically installed and updated with TiUP, no manual installation required
2. **Self-Contained**: Complete knowledge base packaged with component, no separate deployment needed
3. **Version Management**: Each component version includes its compatible knowledge base
4. **Persistent Data**: Runtime logs and user configurations persist across component updates
5. **Isolated Storage**: Runtime data stored separately from component files, allowing safe updates

### Runtime Lookup Logic

When `tiup upgrade-precheck` runs, the lookup logic is as follows:

**Executable Binary**:
- Lookup from TiUP component directory: `~/.tiup/components/upgrade-precheck/<version>/tidb-upgrade-precheck`
- TiUP automatically manages version selection and execution

**Knowledge Base**:
- Primary location: `~/.tiup/components/upgrade-precheck/<version>/knowledge/`
- Environment variable `TIDB_UPGRADE_PRECHECK_KB` can override (for testing)
- Fallback to `knowledge/` in current working directory (for development)

**Runtime Data**:
- Logs: `~/.tiup/storage/upgrade-precheck/logs/`
- User Config: `~/.tiup/storage/upgrade-precheck/config/high_risk_params.json`
- Environment variable `TIDB_UPGRADE_PRECHECK_DATA_DIR` can override (for testing)

```go
// Get TiUP component directory
componentDir := spec.ProfilePath("components", "upgrade-precheck", version)

// Lookup executable
precheckBin := filepath.Join(componentDir, "tidb-upgrade-precheck")

// Lookup knowledge base (packaged with component)
knowledgeDir := filepath.Join(componentDir, "knowledge")

// Lookup runtime data directory (persistent)
dataDir := spec.ProfilePath("storage", "upgrade-precheck")
logsDir := filepath.Join(dataDir, "logs")
configDir := filepath.Join(dataDir, "config")
```

## Command Interface

### Standalone Precheck Command

```bash
tiup upgrade-precheck <cluster-name> <target-version> \
  --format=html \
  --output-dir=./reports \
  --high-risk-params-config=~/.tiup/storage/upgrade-precheck/config/high_risk_params.json
```

### Upgrade Command Integration

```bash
tiup cluster upgrade <cluster-name> <target-version> \
  --precheck \
  --precheck-format=html \
  --precheck-output-dir=./reports
```

## Knowledge Base Packaging

The knowledge base is packaged directly with the component, ensuring each component version includes its compatible knowledge base.

### Generation Steps

1. **Generate Full Knowledge Base**:
   ```bash
   cd /path/to/tidb-upgrade-precheck
   bash scripts/generate_knowledge.sh --serial
   ```

   This generates knowledge bases for all LTS versions to the `./knowledge/` directory.

2. **Verify Knowledge Base**:
   ```bash
   ls -la knowledge/
   # Should see a structure similar to:
   # knowledge/
   #   v7.5/
   #     v7.5.1/
   #       tidb/defaults.json
   #       pd/defaults.json
   #       tikv/defaults.json
   #       tiflash/defaults.json
   #   tidb/upgrade_logic.json
   ```

### Packaging with Component

The knowledge base directory must be included in the component package:

```
upgrade-precheck/
├── tidb-upgrade-precheck     # Binary
└── knowledge/                # Knowledge base (entire directory)
    └── ...
```

## Packaging Process

### In tidb-upgrade-precheck Repository

```bash
# 1. Generate knowledge base
bash scripts/generate_knowledge.sh --serial

# 2. Build precheck binary
make build
# or
go build -o bin/tidb-upgrade-precheck ./cmd/precheck

# 3. Prepare component package directory
mkdir -p package/upgrade-precheck
cp bin/tidb-upgrade-precheck package/upgrade-precheck/
cp -r knowledge package/upgrade-precheck/

# 4. Package for TiUP
tiup package package/upgrade-precheck \
  --name upgrade-precheck \
  --release v1.0.0 \
  --entry tidb-upgrade-precheck \
  --desc "TiDB upgrade precheck tool with knowledge base"
```

### Component Package Structure

The packaged component should have this structure:

```
upgrade-precheck-v1.0.0.tar.gz
└── upgrade-precheck/
    ├── tidb-upgrade-precheck     # Executable
    └── knowledge/               # Complete knowledge base
        ├── v7.5/
        ├── v8.1/
        └── tidb/
```

### CI/CD Integration

**GitHub Actions Example**:

```yaml
name: Release

on:
  push:
    tags:
      - 'v*'

jobs:
  release:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      
      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: '1.21'
      
      - name: Generate knowledge base
        run: |
          bash scripts/generate_knowledge.sh --serial
      
      - name: Build binary
        run: |
          go build -o bin/tidb-upgrade-precheck ./cmd/precheck
      
      - name: Prepare package
        run: |
          mkdir -p package/upgrade-precheck
          cp bin/tidb-upgrade-precheck package/upgrade-precheck/
          cp -r knowledge package/upgrade-precheck/
      
      - name: Package for TiUP
        run: |
          if ! command -v tiup &> /dev/null; then
            curl --proto '=https' --tlsv1.2 -sSf https://tiup-mirrors.pingcap.com/install.sh | sh
          fi
          
          VERSION=${GITHUB_REF#refs/tags/}
          tiup package package/upgrade-precheck \
            --name upgrade-precheck \
            --release $VERSION \
            --entry tidb-upgrade-precheck \
            --desc "TiDB upgrade precheck tool with knowledge base"
      
      - name: Upload release assets
        uses: actions/upload-release-asset@v1
        with:
          upload_url: ${{ github.event.release.upload_url }}
          asset_path: ./package/upgrade-precheck-*.tar.gz
          asset_name: upgrade-precheck-${{ github.ref_name }}.tar.gz
          asset_content_type: application/gzip
```

## Runtime Data Management

### Logs Storage

Execution logs are stored in `~/.tiup/storage/upgrade-precheck/logs/`:

- **Log Naming**: `precheck_YYYYMMDD_HHMMSS.log`
- **Log Rotation**: Automatic rotation based on size or time
- **Retention**: Configurable retention policy (default: 30 days)

### User Configuration Storage

User-defined high-risk parameters are stored in `~/.tiup/storage/upgrade-precheck/config/high_risk_params.json`:

```json
{
  "tidb": [
    {
      "type": "system_variable",
      "name": "tidb_enable_async_commit",
      "severity": "high",
      "description": "Async commit may cause data inconsistency",
      "from_version": "v7.0.0",
      "to_version": "",
      "check_modified": true,
      "allowed_values": []
    }
  ]
}
```

**Configuration Management**:
- Created automatically on first use
- Persists across component updates
- Can be edited manually or via CLI commands

### Data Directory Initialization

The runtime data directory is automatically initialized on first run:

```go
// Initialize runtime data directory
dataDir := spec.ProfilePath("storage", "upgrade-precheck")
logsDir := filepath.Join(dataDir, "logs")
configDir := filepath.Join(dataDir, "config")

// Create directories if they don't exist
os.MkdirAll(logsDir, 0755)
os.MkdirAll(configDir, 0755)

// Initialize default config file if it doesn't exist
configFile := filepath.Join(configDir, "high_risk_params.json")
if _, err := os.Stat(configFile); os.IsNotExist(err) {
    // Create empty config file
    os.WriteFile(configFile, []byte("{}"), 0644)
}
```

## Integration Points in tiup-cluster

### Calling Precheck Component

```go
// In tiup-cluster upgrade command
func runPrecheck(clusterName, targetVersion string) error {
    // Call upgrade-precheck component
    cmd := exec.Command("tiup", "upgrade-precheck", clusterName, targetVersion,
        "--format", "text",
        "--output-dir", "./reports",
    )
    
    output, err := cmd.CombinedOutput()
    if err != nil {
        return fmt.Errorf("precheck failed: %v\n%s", err, output)
    }
    
    // Parse and display results
    // Ask user for confirmation
    return nil
}
```

### Error Handling

- Precheck failures can be configured to block or warn
- User can skip precheck with `--without-precheck` flag
- Precheck results are logged for audit purposes

## Version Management

- **Component Version**: Independent versioning for `upgrade-precheck` component
- **Knowledge Base Version**: Included in component package, versioned with component
- **Compatibility**: Each component version includes knowledge base compatible with that version
- **Updates**: Component updates automatically include updated knowledge base

## Update Mechanism

### Component Updates

- **Automatic Updates**: TiUP automatically checks for and installs component updates
- **Knowledge Base Updates**: Included in component package, updated with component
- **Data Preservation**: Runtime data (logs, config) persists across updates

### Manual Updates

Users can manually update the component:

```bash
# Update to latest version
tiup update upgrade-precheck

# Update to specific version
tiup update upgrade-precheck@v1.0.0
```

## Implementation Points

1. **Component Packaging**: Knowledge base must be included in component package
2. **Data Persistence**: Runtime data stored in `storage/` directory, separate from component files
3. **Path Resolution**: Use TiUP's `spec.ProfilePath()` for consistent path resolution
4. **Error Handling**: Provide clear error messages if knowledge base or data directory is not accessible
5. **Logging**: All execution logs saved to persistent storage directory
6. **Configuration Management**: User configurations persist across component updates

## Notes

1. **Knowledge Base Size**: Knowledge base may be large (contains data for multiple versions). Ensure component package size is acceptable.

2. **Data Directory Permissions**: Ensure runtime data directory has proper permissions for read/write operations.

3. **Log Rotation**: Implement log rotation to prevent disk space issues from accumulating logs.

4. **Configuration Backup**: Consider providing backup/restore functionality for user configurations.

5. **CI Automation**: Automate knowledge base generation and component packaging in CI to ensure each release includes the latest knowledge base.

6. **Version Compatibility**: Ensure knowledge base version is compatible with component version. Document version compatibility matrix if needed.

## Related Documents

- [Deployment Guide](./deployment.md) - Overview of all deployment options
- [Knowledge Base Generation Guide](./knowledge_generation_guide.md) - Detailed knowledge base generation guide
- [TiUP Implementation Guide](./tiup/tiup_implementation_guide.md) - Detailed implementation steps
- [System Design Overview](./design.md) - System architecture

---

**Last Updated**: 2025
