 # tidb-upgrade-precheck

`tidb-upgrade-precheck` 提供用于收集和分析 TiDB 集群配置的工具，以识别版本升级期间的潜在风险。该系统包含两个主要组件：

1. **知识库生成器** - 从 TiDB 源代码中收集参数默认值和升级逻辑，构建知识库
2. **运行时采集器** - 从正在运行的集群中收集当前配置，用于风险分析

## Key Features
- Automated risk scanning of TiDB cluster parameters, configuration, and compatibility before upgrade.
- Multiple report output formats: text, markdown, html. 
- Aggregated parameter history

## Typical Scenarios

- Pre-upgrade risk assessment and archiving
- Integration into automated O&M workflows
- Change review and compliance traceability

---

For rule extension, custom report templates, or integration into your own system, please refer to the source code and README.

Issues and pull requests are welcome.

## Knowledge Base Generation Tool (kb-generator)

This repository includes tools to automatically generate TiDB parameter defaults (defaults.json) and upgrade logic (upgrade_logic.json) across versions. This is used to build a knowledge base for upgrade compatibility checking.

### Directory Structure

```
tidb-upgrade-precheck/
├── cmd/
│   └── kb-generator/         # Knowledge base generator CLI
├── pkg/
│   └── kbgenerator/          # Core parameter collection logic
│       ├── kb_generator.go   # Parameter collection from source code
│       └── collect_upgrade_logic.go # Upgrade logic analysis
├── knowledge/                # Output directory (not version controlled)
├── doc/                      # Documentation
│   ├── parameter_collection_design.md    # Technical design document
│   ├── parameter_collection_guide.md     # Operation guide
│   ├── parameter_collection_design_zh.md # Technical design document (Chinese)
│   └── parameter_collection_guide_zh.md  # Operation guide (Chinese)
└── Makefile                  # Build and run commands
```

### Knowledge Base Generation Features

1. **Parameter Collection (P1/P2 Risks)**
   - Strategy: Runtime Import or Static Analysis
   - Mechanism:
     - Switch to target tag (e.g., v7.5.0)
     - Two collection methods supported:
       - Source code parsing (static analysis)
       - Binary execution (runtime import)
     - Extract `CurrentBootstrapVersion` as metadata
     - Output as `knowledge/<version>/defaults.json`

2. **Upgrade Logic Collection (P0 Risks)**
   - Strategy: External intelligent scanning (zero-intrusion)
   - Mechanism:
     - AST parsing of `session/bootstrap.go` to extract all `SET GLOBAL` changes
     - Records forced parameter changes that occur during the upgrade process
     - Output as `knowledge/upgrade_logic.json` - part of the knowledge base

3. **Version Management**
   - Automatically tracks generated versions to avoid re-generation
   - Stores version information in `knowledge/generated_versions.json`
   - Can skip or force re-generation of specific versions

### Usage

#### Dependencies
- Go 1.18+
- Git
- TiDB source code cloned to `../tidb` (can be specified with `--repo` flag)

#### Full Collection (All LTS versions)
```bash
# Collect only non-generated versions (default behavior)
make collect
# or
go run cmd/kb-generator/main.go --all --repo=/path/to/tidb

# Collect all versions including already generated ones
make collect-all
# or
go run cmd/kb-generator/main.go --all --skip-generated=false --repo=/path/to/tidb
```

#### Single Tag Collection
```bash
# Using source code parsing (default)
go run cmd/kb-generator/main.go --tag=v7.5.0 --repo=/path/to/tidb

# Using binary execution method
go run cmd/kb-generator/main.go --tag=v7.5.0 --method=binary --tool=/path/to/export_tool.go --repo=/path/to/tidb
```

#### Incremental Collection (Version range)
```bash
go run cmd/kb-generator/main.go --from-tag=v7.5.0 --to-tag=v8.1.0 --repo=/path/to/tidb
```

#### Parameter History Aggregation
```bash
make aggregate
# or
go run cmd/kb-generator/main.go --aggregate --repo=/path/to/tidb
```

#### Clean Generated Records
```bash
make clean-generated
# or manually remove knowledge/generated_versions.json
```

### Contents of Knowledge Base

The knowledge base contains three main components:

1. **Parameter Defaults**: Default values for each version
   - Stored in `knowledge/<version>/defaults.json`
   - Contains configuration defaults and system variable defaults

2. **Upgrade Logic**: Forced parameter changes during upgrades
   - Stored in `knowledge/upgrade_logic.json`
   - Contains records of all forced parameter changes that happen during the upgrade process
   - Used to identify P0 risks during upgrade precheck

3. **Parameter History**: Aggregated parameter history across versions
   - Stored in `knowledge/parameters-history.json`
   - Shows how parameters have changed across versions

### Key Technical Points

1. Multi-version source switching: Automatically checkout git tags to ensure accuracy
2. Dual collection methods: Support both static analysis and runtime import for parameter collection
3. AST static analysis: Automatically extract upgrade changes to avoid manual omissions
4. Standardized output: All outputs are JSON for easy validation and comparison
5. Fault tolerance and logging: Each tag is collected independently, failures don't affect others
6. Version management: Avoids re-generating already processed versions for efficiency

### TiDB Parameters in Knowledge Base

The knowledge base contains two main categories of TiDB parameters:

1. **Configuration Defaults**: Values defined in the `config.Config` struct, including:
   - Server settings (port, status, log, etc.)
   - Performance tuning parameters
   - Security settings
   - Plugin configurations

2. **System Variables**: Global session variables that can be tuned, including:
   - SQL behavior settings
   - Optimizer parameters
   - Transaction settings
   - Storage engine parameters

Both categories are essential for upgrade compatibility checking as they define the baseline behavior of TiDB at each version.

## Runtime Collection Tool

The runtime collection tool collects current configuration from running TiDB clusters. This is used to compare against the knowledge base to identify potential upgrade risks.

### Directory Structure

```
tidb-upgrade-precheck/
├── pkg/
│   └── runtime/              # Runtime collection logic
│       ├── types.go          # Data structures
│       ├── collector.go      # Main collector
│       ├── tidb_collector.go # TiDB collector
│       ├── tikv_collector.go # TiKV collector
│       └── pd_collector.go   # PD collector
└── examples/
    └── runtime_collector_example.go # Runtime collection example
```

### Runtime Collection Features

1. **Real-time Configuration Collection**
   - Collects current TiDB configuration via HTTP API
   - Collects current system variables via MySQL protocol
   - Collects TiKV and PD configuration via HTTP API

2. **Multi-component Support**
   - TiDB: Configuration and system variables
   - TiKV: Configuration parameters
   - PD: Configuration parameters

### Runtime Collection Usage

```bash
# Collect from a running cluster
go run examples/runtime_collector_example.go \
  --tidb-addr=127.0.0.1:4000 \
  --tikv-addrs=127.0.0.1:20180 \
  --pd-addrs=127.0.0.1:2379
```

### Collected Runtime Information

The runtime collector gathers:

1. **Current Configuration Values**: Actual values set in the running cluster
2. **User-modified Parameters**: Parameters that differ from defaults
3. **Component Versions**: Version information for each component

This information is compared against the knowledge base to identify potential upgrade risks.