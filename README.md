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
│       ├── pd_kb_generator.go # PD parameter collection from source code
│       └── collect_upgrade_logic.go # Upgrade logic analysis
├── knowledge/                # Output directory (not version controlled)
│   ├── tidb/                 # TiDB knowledge base files
│   ├── pd/                   # PD knowledge base files
│   │   ├── v6.5.0/           # PD knowledge for v6.5.0
│   │   │   └── pd_defaults.json # PD parameter defaults for v6.5.0
│   │   └── v8.5.0/           # PD knowledge for v8.5.0
│   │       └── pd_defaults.json # PD parameter defaults for v8.5.0
│   └── upgrade_logic.json    # Cross-version upgrade logic
├── doc/                      # Documentation
│   ├── parameter_collection_design.md    # Technical design document
│   ├── parameter_collection_guide.md     # Operation guide
│   ├── parameter_collection_design_zh.md # Technical design document (Chinese)
│   ├── parameter_collection_guide_zh.md  # Operation guide (Chinese)
│   ├── pd_knowledge_base_generation.md   # PD knowledge base generation guide
│   ├── pd_parameter_upgrade_comparison_design.md  # PD parameter upgrade comparison design
│   ├── pd_mandatory_changes.md           # PD mandatory parameter changes
│   └── tikv_parameter_upgrade_comparison_design.md # TiKV parameter upgrade comparison design
└── Makefile                  # Build and run commands
```

### Knowledge Base Generation Features

1. **Parameter Collection (P1/P2 Risks)**
   - Strategy: Runtime Import or Static Analysis
   
2. **PD Configuration Collection**
   - Strategy: Static Analysis of PD source code
   - Extracts PD configuration parameters and their default values
   - Generates knowledge base for PD parameter comparison

### Building and Running

Build the tools:

```bash
make build
```

Generate knowledge base for TiDB:

```bash
make gen-kb-tidb REPO_ROOT=../tidb TAG=v6.5.0
```

Generate knowledge base for PD:

```bash
make gen-kb-pd PD_REPO_ROOT=../pd TAG=v6.5.0
```

Generate upgrade logic for TiDB:

```bash
make gen-ul-tidb REPO_ROOT=../tidb FROM_TAG=v6.5.0 TO_TAG=v7.1.0
```

Generate upgrade logic for PD:

```bash
make gen-ul-pd PD_REPO_ROOT=../pd FROM_TAG=v6.5.0 TO_TAG=v7.1.0
```

See `make help` for more details.

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