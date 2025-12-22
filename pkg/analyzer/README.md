# Analyzer Package

The `analyzer` package provides comprehensive risk analysis logic for TiDB upgrade precheck. It performs analysis based on rules, which define what data to collect and how to compare.

## Architecture Overview

The analyzer package is organized into several key components:

```
analyzer/
├── analyzer.go          # Main analyzer orchestrator
├── preprocessor.go      # Parameter preprocessing and filtering
├── filters.go           # Centralized filter configuration
├── result.go            # Analysis result types
└── rules/               # Rule definitions
    ├── rule.go          # Rule interface
    ├── context.go       # Rule context (data provider)
    ├── compare.go       # Comparison utilities
    ├── checker.go       # Check result utilities
    └── rule_*.go        # Individual rule implementations
```

## Core Components

### 1. Analyzer (`analyzer.go`)

The main orchestrator that:
- Collects data requirements from all rules
- Loads necessary data from knowledge base
- Executes preprocessing stage
- Runs all rules
- Aggregates and returns results

**Key Methods:**
- `NewAnalyzer(options)`: Creates a new analyzer instance
- `Analyze(ctx, snapshot, sourceVersion, targetVersion, sourceKB, targetKB)`: Performs comprehensive analysis
- `GetDataRequirements()`: Returns merged data requirements from all rules
- `GetCollectionRequirements()`: Returns collection requirements for runtime collector

### 2. Preprocessor (`preprocessor.go`)

The preprocessing stage that runs **before** rule evaluation:
- Filters deployment-specific parameters (paths, hostnames, etc.)
- Filters resource-dependent parameters (auto-tuned by system)
- Filters parameters with identical values (no difference to report)
- Removes filtered parameters from `sourceDefaults` and `targetDefaults` maps
- Generates `CheckResult` entries for filtered parameters (for reporting)

**Key Benefits:**
- Reduces comparison overhead in rules (rules only process necessary parameters)
- Centralizes filtering logic (no duplicate filtering in each rule)
- Improves maintainability (add new filters in one place)

**Filtering Criteria:**
1. **Path Parameters**: Deployment-specific paths (data-dir, log-dir, etc.)
2. **Host/Network Parameters**: Deployment-specific addresses (host, port, etc.)
3. **Resource-Dependent Parameters**: Auto-tuned based on system resources
4. **Identical Values**: Parameters where current == source == target (no action needed)
5. **New Parameters**: New parameters in target version where current == target default

### 3. Filters (`filters.go`)

Centralized filter configuration that defines:
- **ExactMatchParams**: Parameters filtered by exact name match
- **PathKeywords**: Keywords indicating path-related parameters
- **HostKeywords**: Keywords indicating host/network parameters
- **ResourceDependentKeywords**: Keywords indicating resource-dependent parameters
- **Exceptions**: Parameters that contain keywords but should NOT be filtered
- **FilenameOnlyParams**: Parameters that need special comparison (filename only, not path)

**Key Functions:**
- `ShouldFilterParameter(paramName)`: Returns (shouldFilter, filterReason)
- `IsResourceDependentParameter(paramName)`: Checks if parameter is resource-dependent
- `IsFilenameOnlyParameter(paramName)`: Checks if parameter should be compared by filename only

### 4. Rules (`rules/`)

Individual rule implementations that perform specific comparisons. Each rule:
- Defines its data requirements (what data it needs)
- Receives preprocessed data (filtered parameters already removed)
- Focuses on core comparison logic (no filtering needed)

**Default Rules:**
- `UserModifiedParamsRule`: Detects user-modified parameters
- `UpgradeDifferencesRule`: Compares current vs target defaults and forced changes
- `TikvConsistencyRule`: Compares TiKV node parameters for consistency
- `HighRiskParamsRule`: Checks for high-risk parameter configurations

## Data Flow

```
1. Analyzer collects data requirements from all rules
   ↓
2. Loads data from knowledge base (source/target defaults, upgrade logic, parameter notes)
   ↓
3. Preprocessor filters parameters:
   - Removes deployment-specific parameters
   - Removes resource-dependent parameters (if source == target)
   - Removes identical parameters (current == source == target)
   - Generates CheckResults for filtered parameters
   ↓
4. Rules receive cleaned defaults (only necessary parameters)
   ↓
5. Rules perform comparisons and generate CheckResults
   ↓
6. Analyzer aggregates all results and returns AnalysisResult
```

## Usage Example

```go
import (
    "context"
    "github.com/pingcap/tidb-upgrade-precheck/pkg/analyzer"
    "github.com/pingcap/tidb-upgrade-precheck/pkg/collector"
)

// Create analyzer with default rules
analyzer := analyzer.NewAnalyzer(nil)

// Perform analysis
result, err := analyzer.Analyze(
    context.Background(),
    snapshot,           // Cluster snapshot from collector
    "v7.5.0",          // Source version
    "v8.0.0",          // Target version
    sourceKB,          // Source version knowledge base
    targetKB,           // Target version knowledge base
)

if err != nil {
    // Handle error
}

// Access results
for _, checkResult := range result.AllCheckResults {
    // Process each check result
}
```

## Custom Rules

To create custom rules, see [rules/README.md](./rules/README.md) for detailed documentation.

## Testing

- `analyzer_test.go`: Tests for analyzer orchestration
- `preprocessor_test.go`: Tests for parameter preprocessing
- `filters_test.go`: Tests for filter configuration
- `rules/*_test.go`: Tests for individual rules

## Key Design Decisions

1. **Preprocessing Stage**: Centralizes filtering to reduce rule complexity and improve performance
2. **Filter Configuration**: Single source of truth for all filtering logic
3. **Rule Interface**: Standardized interface for easy extension
4. **Data Requirements**: Rules declare what they need, analyzer loads only necessary data
5. **CheckResult**: Standardized result format for all rules

## Migration Notes

If you're migrating from an older version:

1. **Filtering Logic**: Filtering is now done in preprocessor, not in individual rules
2. **Rule Simplification**: Rules no longer need to check `IsPathParameter`, `IsIgnoredParameter`, etc.
3. **Default Maps**: Rules receive cleaned defaults (filtered parameters already removed)
4. **Filtered Results**: Filtered parameters are still reported (as `CheckResult` with category "filtered") but don't go through rule evaluation

