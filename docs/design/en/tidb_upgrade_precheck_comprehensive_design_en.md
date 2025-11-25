// TRANSLATION REQUIRED: # TiDB Upgrade Precheck System Design
// TRANSLATION REQUIRED: 
// TRANSLATION REQUIRED: ## 🎯 Project Goals and Core Value
// TRANSLATION REQUIRED: 
// TRANSLATION REQUIRED: ### Project Name
// TRANSLATION REQUIRED: tidb-upgrade-precheck
// TRANSLATION REQUIRED: 
// TRANSLATION REQUIRED: ### Core Problem
// TRANSLATION REQUIRED: Hidden risks in TiDB cluster upgrades caused by changes in "configuration parameters" or "system variables" in the target version (such as default value changes, deprecation, forced overrides).
// TRANSLATION REQUIRED: 
// TRANSLATION REQUIRED: ### Core Value
// TRANSLATION REQUIRED: 1. **Risk Prevention**: Automatically identify P0/P1/P2 level risks before upgrade.
// TRANSLATION REQUIRED: 2. **Decision Support**: Provide unified "planning", "confirmation" and "skip" modes.
// TRANSLATION REQUIRED: 3. **Zero Intrusion**: No need to modify TiDB code; rely on external scanners to extract knowledge.
// TRANSLATION REQUIRED: 4. **Knowledge Transfer**: Automatically transfer domain knowledge from R&D side (through comments) to Operators.
// TRANSLATION REQUIRED: 5. **Audit and Archiving**: [New] Generate formal reports in Markdown/HTML format containing risk summaries and full configuration audits.
// TRANSLATION REQUIRED: 
// TRANSLATION REQUIRED: ---
// TRANSLATION REQUIRED: 
// TRANSLATION REQUIRED: ## 1. 🏛️ Core Architecture (Three-tier Architecture Based on Scanner)
// TRANSLATION REQUIRED: 
// TRANSLATION REQUIRED: We adopt a three-tier architecture: Producer -> Packager -> Consumer.
// TRANSLATION REQUIRED: 
// TRANSLATION REQUIRED: ```mermaid
// TRANSLATION REQUIRED: graph TD
// TRANSLATION REQUIRED:     subgraph "Tier 1: Producer (pingcap/tidb)"
// TRANSLATION REQUIRED:         A[TiDB Source Code]
// TRANSLATION REQUIRED:         A1[GitHub PRs / Issues Metadata]
// TRANSLATION REQUIRED:     end
// TRANSLATION REQUIRED:     
// TRANSLATION REQUIRED:     subgraph "Tier 2: Packager (tidb-upgrade-precheck)"
// TRANSLATION REQUIRED:         direction TB
// TRANSLATION REQUIRED:         B[CI Pipeline]
// TRANSLATION REQUIRED:         
// TRANSLATION REQUIRED:         %% Path 1: Defaults
// TRANSLATION REQUIRED:         B -- "1. Import pkg/config" --> C(Defaults Generator)
// TRANSLATION REQUIRED:         C --> D[defaults.json]
// TRANSLATION REQUIRED:         
// TRANSLATION REQUIRED:         %% Path 2: Scanner Logic (P0 Risks)
// TRANSLATION REQUIRED:         B -- "2. Scan AST / Git Diff / PRs" --> E(Compatibility Scanner)
// TRANSLATION REQUIRED:         A --> E
// TRANSLATION REQUIRED:         A1 -.-> |"Pull Metadata"| E
// TRANSLATION REQUIRED:         E -- "3. Extract Logic & Comments" --> F[upgrade_logic.json]
// TRANSLATION REQUIRED: 
// TRANSLATION REQUIRED:         %% Packaging
// TRANSLATION REQUIRED:         D --> G[knowledge_base/ Directory]
// TRANSLATION REQUIRED:         F --> G
// TRANSLATION REQUIRED:         
// TRANSLATION REQUIRED:         %% Runtime Components
// TRANSLATION REQUIRED:         G -.-> H[Analyzer Engine]
// TRANSLATION REQUIRED:         H --> I[Reporter Engine]
// TRANSLATION REQUIRED:         I --> |Render| J[HTML/MD/Console]
// TRANSLATION REQUIRED:         
// TRANSLATION REQUIRED:         %% Final Product
// TRANSLATION REQUIRED:         G & H & I & J -- "4. go:embed + build" --> K[Release Go Module]
// TRANSLATION REQUIRED:     end
// TRANSLATION REQUIRED: 
// TRANSLATION REQUIRED:     subgraph "Tier 3: Consumer"
// TRANSLATION REQUIRED:         K --> L[tiup / tioperator]
// TRANSLATION REQUIRED:     end
// TRANSLATION REQUIRED: ```
// TRANSLATION REQUIRED: 注：目前暂时没有需要在生产者这边实现的功能，目前主要是通过 packager 端的 kb-generator 工具来实现知识库的生成。目前我们依然将生产者部分保留在设计中，方便未来有需要时进行扩展
// TRANSLATION REQUIRED: ---
// TRANSLATION REQUIRED: 
// TRANSLATION REQUIRED: ## Part I: R&D Side Implementation (Production and Packaging)
// TRANSLATION REQUIRED: 
// TRANSLATION REQUIRED: ### 2. 🧬 Knowledge Base Generation Strategy
// TRANSLATION REQUIRED: 
// TRANSLATION REQUIRED: The tidb-upgrade-precheck repository contains a core CLI tool: kb-generator.
// TRANSLATION REQUIRED: 
// TRANSLATION REQUIRED: ### 3.1. Task 1: Generate defaults.json (P1/P2 Risks)
// TRANSLATION REQUIRED: 
// TRANSLATION REQUIRED: - **Strategy**: Runtime Import.
// TRANSLATION REQUIRED: - **Mechanism**: Directly import the target version package and serialize default values.
// TRANSLATION REQUIRED: - **Metadata**: Extract internal version number CurrentBootstrapVersion (e.g.: 218).
// TRANSLATION REQUIRED: 
// TRANSLATION REQUIRED: ### 3.2. Task 2: Generate upgrade_logic.json (P0 Risks)
// TRANSLATION REQUIRED: 
// TRANSLATION REQUIRED: - **Strategy**: External intelligent scanning (zero intrusion).
// TRANSLATION REQUIRED: - **Mechanism**:
// TRANSLATION REQUIRED:   - AST Analysis: Parse pkg/domain/upgrade.go to find SET GLOBAL call patterns.
// TRANSLATION REQUIRED:   - GitHub Mining: Automatically extract comments from PR descriptions.
// TRANSLATION REQUIRED: 
// TRANSLATION REQUIRED: ### 3.3. Command Examples
// TRANSLATION REQUIRED: 
// TRANSLATION REQUIRED: ```bash
// TRANSLATION REQUIRED: # Incremental (Release)
// TRANSLATION REQUIRED: kb-generator scan --repo=... --from-tag=v7.5.0 --to-tag=v8.1.0
// TRANSLATION REQUIRED: 
// TRANSLATION REQUIRED: # Full rebuild (Bootstrap)
// TRANSLATION REQUIRED: kb-generator scan --repo=... --all
// TRANSLATION REQUIRED: ```
// TRANSLATION REQUIRED: 
// TRANSLATION REQUIRED: Full collection starts from version 6.5.0 to get all variable and configuration parameter default values as the baseline for each version, and then collect the content of variables that are forcibly changed during upgrades.
// TRANSLATION REQUIRED: 
// TRANSLATION REQUIRED: ### 3.4. Task 3: Generate upgrade knowledge base for TiKV, PD, TiFlash modules (P0 Risks)
// TRANSLATION REQUIRED: ToDo
// TRANSLATION REQUIRED: ---
// TRANSLATION REQUIRED: 
// TRANSLATION REQUIRED: ## Part II: Runtime and Integration (User Side)
// TRANSLATION REQUIRED: 
// TRANSLATION REQUIRED: To minimize code development workload and code maintenance difficulty, we recommend that all core code be in the tidb-upgrade-precheck repository:
// TRANSLATION REQUIRED: 
// TRANSLATION REQUIRED: 1. All collector capabilities:
// TRANSLATION REQUIRED:    1. Truly implement collection of corresponding data from source clusters to be upgraded by users
// TRANSLATION REQUIRED:    2. Implement collection of configuration parameters and variables for core modules such as TiKV, PD, TiFlash
// TRANSLATION REQUIRED: 
// TRANSLATION REQUIRED: 2. Main implementation of analyzer:
// TRANSLATION REQUIRED:    1. Analysis rules, risk rule application, risk identification
// TRANSLATION REQUIRED:    2. Implementation of analyzers for core modules such as TiKV, PD, TiFlash
// TRANSLATION REQUIRED: 
// TRANSLATION REQUIRED: 3. Unified report output mode
// TRANSLATION REQUIRED: 
// TRANSLATION REQUIRED: 4. Other tools such as tiup and tioperator only need to call the corresponding functions:
// TRANSLATION REQUIRED:    1. Support corresponding command line parameter entry knowledge to integrate the corresponding precheck command, maintain cluster topology, access database information in commands and output format for the main program to call
// TRANSLATION REQUIRED:    2. Tools only keep the minimum code related to tool commands
// TRANSLATION REQUIRED: 
// TRANSLATION REQUIRED: > **Important Note**: tidb-upgrade-precheck does not directly generate a precheck command. This command needs to be extended by tiup and tioperator themselves, and then executed by calling our API library in a similar way.
// TRANSLATION REQUIRED: 
// TRANSLATION REQUIRED: ---
// TRANSLATION REQUIRED: 
// TRANSLATION REQUIRED: ## 3. ⚙️ Runtime Workflow
// TRANSLATION REQUIRED: 
// TRANSLATION REQUIRED: ### 4.1. Collector
// TRANSLATION REQUIRED: 
// TRANSLATION REQUIRED: - Fetch real-time configuration of current cluster (GLOBAL_VARIABLES, /config).
// TRANSLATION REQUIRED: - Detailed design of upgrade knowledge base collection tool
// TRANSLATION REQUIRED: 
// TRANSLATION REQUIRED: ### 4.2. Analyzer - Core Logic and Audit
// TRANSLATION REQUIRED: 
// TRANSLATION REQUIRED: Compare Current_State (current), Source_KB (source) and Target_KB (target).
// TRANSLATION REQUIRED: 
// TRANSLATION REQUIRED: **Logic Step 1: Source Determination**
// TRANSLATION REQUIRED: - If current value == source default → UseDefault
// TRANSLATION REQUIRED: - If current value != source default → UserSet
// TRANSLATION REQUIRED: 
// TRANSLATION REQUIRED: **Logic Step 2: Risk and Audit Matrix**
// TRANSLATION REQUIRED: 
// TRANSLATION REQUIRED: | Source State | Target State | Risk Level | Action |
// TRANSLATION REQUIRED: |--------------|--------------|------------|--------|
// TRANSLATION REQUIRED: | UseDefault | Default Changed | MEDIUM | Recommendation |
// TRANSLATION REQUIRED: | UseDefault | Forced Upgrade | HIGH | Must Handle |
// TRANSLATION REQUIRED: | UserSet | Default Changed | INFO | Configuration Audit |
// TRANSLATION REQUIRED: | UserSet | Forced Upgrade | HIGH | Must Handle |
// TRANSLATION REQUIRED: ### 4.3. Reporter [New]
// TRANSLATION REQUIRED: 
// TRANSLATION REQUIRED: Support rendering the following formats:
// TRANSLATION REQUIRED: - Console: Brief summary + high/medium/low details.
// TRANSLATION REQUIRED: - Markdown (.md): Contains risk summary table + full configuration audit table.
// TRANSLATION REQUIRED: - HTML (.html): Web report with styling.
// TRANSLATION REQUIRED: 
// TRANSLATION REQUIRED: ---
// TRANSLATION REQUIRED: 
// TRANSLATION REQUIRED: ## 4. 🤝 Integration Blueprint
// TRANSLATION REQUIRED: 
// TRANSLATION REQUIRED: ### 5.1. tiup Integration
// TRANSLATION REQUIRED: 
// TRANSLATION REQUIRED: - New parameters: `--report-format=md,html` and `--report-dir=/tmp`.
// TRANSLATION REQUIRED: - Logic:
// TRANSLATION REQUIRED:   - Mode 1 (Execute): Check -> Terminal Report -> Interactive Confirmation (y/N) -> Upgrade.
// TRANSLATION REQUIRED:   - Mode 2 (Planning): Check -> Terminal Report -> Generate MD/HTML File -> Exit.
// TRANSLATION REQUIRED:   - Mode 3 (Skip): Skip Check -> Upgrade.
// TRANSLATION REQUIRED: 
// TRANSLATION REQUIRED: ### 5.2. tioperator Integration
// TRANSLATION REQUIRED: 
// TRANSLATION REQUIRED: - Logic:
// TRANSLATION REQUIRED:   - Mode 1 (Execute): Pause Upgrade -> Write Report to Status/Events -> Wait for Annotation.
// TRANSLATION REQUIRED:   - Mode 2 (Planning): Write Report to Status -> Do Not Upgrade.
// TRANSLATION REQUIRED: 
// TRANSLATION REQUIRED: ---
// TRANSLATION REQUIRED: 
// TRANSLATION REQUIRED: ## 5. 🖥️ Appendix: Output Examples (DEMO)
// TRANSLATION REQUIRED: 
// TRANSLATION REQUIRED: ### 6.1. Markdown Report (report.md)
// TRANSLATION REQUIRED: 
// TRANSLATION REQUIRED: ```

