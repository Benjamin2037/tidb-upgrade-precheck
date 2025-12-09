# TiUP Implementation Guide for tidb-upgrade-precheck Integration

## Overview

This document provides detailed instructions on how to implement the integration of tidb-upgrade-precheck into the existing `tiup cluster upgrade` command. This integration will allow users to automatically perform compatibility checks before upgrading a TiDB cluster.

## Implementation Steps in TiUP

### 1. Add Dependency

First, add tidb-upgrade-precheck as a dependency in TiUP's go.mod file:

```bash
go get github.com/pingcap/tidb-upgrade-precheck@latest
```

Or add it directly to go.mod:

```go
require (
    github.com/pingcap/tidb-upgrade-precheck v0.1.0
    // ... other dependencies
)
```

### 2. Extend Command Line Arguments

Modify the `upgrade` command to include new flags for precheck functionality. In the command definition file (typically in cmd/cluster/command/upgrade.go):

```go
// Add new flags to the upgrade command
cmd.Flags().Bool("precheck", false, "Perform compatibility check and ask user for confirmation")
cmd.Flags().Bool("without-precheck", false, "Skip compatibility check and proceed directly to upgrade")
cmd.Flags().Bool("precheck-only", false, "Only perform precheck, don't execute the actual upgrade")
cmd.Flags().String("precheck-fail-severity", "error", "Failure threshold for precheck (info, warning, error)")
cmd.Flags().String("precheck-format", "text", "Precheck report format (text, json, markdown, html)")
cmd.Flags().String("precheck-output-dir", "./", "Precheck report output directory")
cmd.Flags().Bool("precheck-strict", true, "Whether to abort upgrade if precheck fails")
```

### 3. Implement Precheck Logic

In the upgrade command execution function, add the precheck logic. This would typically be in a file like cmd/cluster/command/upgrade.go:

```go
import (
    "github.com/pingcap/tidb-upgrade-precheck/pkg/runtime"
    "github.com/pingcap/tidb-upgrade-precheck/pkg/precheck"
    "github.com/pingcap/tidb-upgrade-precheck/pkg/report"
)

func upgradeCluster(clusterName, version string, opt operator.Options) error {
    // Get cluster information using existing TiUP infrastructure
    clusterInst, err := cc.GetClusterManager().GetCluster(clusterName)
    if err != nil {
        return err
    }
    
    topo := clusterInst.Topology
    
    // Check if precheck should be skipped
    withoutPrecheck, err := cmd.Flags().GetBool("without-precheck")
    if err != nil {
        return err
    }
    
    // Check if explicit precheck is requested
    explicitPrecheck, err := cmd.Flags().GetBool("precheck")
    if err != nil {
        return err
    }
    
    // Run precheck if explicitly requested or if neither precheck nor without-precheck is specified (default behavior)
    if explicitPrecheck || (!explicitPrecheck && !withoutPrecheck) {
        if err := runPrecheck(topo, clusterName, version, opt); err != nil {
            precheckStrict, _ := cmd.Flags().GetBool("precheck-strict")
            if precheckStrict {
                return fmt.Errorf("precheck failed: %v", err)
            } else {
                log.Warnf("Precheck failed but continuing due to --precheck-strict=false: %v", err)
            }
        }
        
        // Check if only precheck is requested
        precheckOnly, err := cmd.Flags().GetBool("precheck-only")
        if err != nil {
            return err
        }
        
        if precheckOnly {
            fmt.Println("Precheck completed. Exiting due to --precheck-only flag.")
            return nil
        }
        
        // Ask for user confirmation if not explicitly requesting precheck
        if !explicitPrecheck {
            if !askUserConfirmation() {
                fmt.Println("Upgrade cancelled by user.")
                return nil
            }
        }
    }
    
    // Continue with the existing upgrade logic
    // ... existing upgrade code ...
    return nil
}
```

### 4. Implement the runPrecheck Function

Create a function to run the precheck process:

```go
func runPrecheck(topo *spec.Specification, clusterName, targetVersion string, opt operator.Options) error {
    // Convert TiUP topology to endpoints format required by tidb-upgrade-precheck
    endpoints := convertToEndpoint(topo)
    
    // Initialize collector
    collector := runtime.NewCollector()
    
    // Collect cluster snapshot
    snapshot, err := collector.Collect(endpoints)
    if err != nil {
        return fmt.Errorf("failed to collect cluster information: %v", err)
    }
    
    // Set version information
    snapshot.SourceVersion = getCurrentVersion(topo) // Implement this function to get current version
    snapshot.TargetVersion = targetVersion
    
    // Run precheck analysis
    reportData := precheck.FromClusterSnapshot(snapshot)
    
    // Generate report
    precheckFormat, _ := cmd.Flags().GetString("precheck-format")
    precheckOutputDir, _ := cmd.Flags().GetString("precheck-output-dir")
    
    generator := report.NewGenerator()
    options := &report.Options{
        Format:    report.Format(precheckFormat),
        OutputDir: precheckOutputDir,
    }
    
    reportPath, err := generator.Generate(reportData, options)
    if err != nil {
        return fmt.Errorf("failed to generate precheck report: %v", err)
    }
    
    fmt.Printf("Precheck report generated: %s\n", reportPath)
    
    // Check if there are issues above the failure threshold
    precheckFailSeverity, _ := cmd.Flags().GetString("precheck-fail-severity")
    if hasIssuesAboveSeverity(reportData, precheckFailSeverity) {
        return fmt.Errorf("precheck found issues with severity level %s or higher", precheckFailSeverity)
    }
    
    return nil
}
```

### 5. Implement Helper Functions

Implement the helper functions needed for the integration:

```go
// Convert TiUP topology to endpoints
func convertToEndpoint(topo *spec.Specification) runtime.ClusterEndpoints {
    endpoints := runtime.ClusterEndpoints{}
    
    // Get TiDB address
    for _, comp := range topo.ComponentsByStartOrder() {
        if comp.Name() == spec.ComponentTiDB {
            inst := comp.Instances()[0] // Get the first instance for simplicity
            endpoints.TiDBAddr = fmt.Sprintf("%s:%d", inst.GetHost(), inst.GetPort())
            break
        }
    }
    
    // Get TiKV addresses
    for _, comp := range topo.ComponentsByStartOrder() {
        if comp.Name() == spec.ComponentTiKV {
            for _, inst := range comp.Instances() {
                endpoints.TiKVAddrs = append(endpoints.TiKVAddrs, 
                    fmt.Sprintf("%s:%d", inst.GetHost(), inst.GetPort()))
            }
        }
    }
    
    // Get PD addresses
    for _, comp := range topo.ComponentsByStartOrder() {
        if comp.Name() == spec.ComponentPD {
            for _, inst := range comp.Instances() {
                endpoints.PDAddrs = append(endpoints.PDAddrs, 
                    fmt.Sprintf("%s:%d", inst.GetHost(), inst.GetPort()))
            }
        }
    }
    
    return endpoints
}

// Get current cluster version
func getCurrentVersion(topo *spec.Specification) string {
    // Implementation to get current version from topology
    // This might involve querying the cluster or checking component versions
    // For now, return a placeholder
    return "unknown"
}

// Check if there are issues above the specified severity
func hasIssuesAboveSeverity(reportData *precheck.Report, severity string) bool {
    // Implementation to check if report contains issues above threshold
    // This is a simplified example
    switch severity {
    case "info":
        return len(reportData.Items) > 0
    case "warning":
        for _, item := range reportData.Items {
            if item.Severity == "warning" || item.Severity == "error" {
                return true
            }
        }
        return false
    case "error":
        for _, item := range reportData.Items {
            if item.Severity == "error" {
                return true
            }
        }
        return false
    default:
        return false
    }
}
```

### 6. Handle User Interaction

If issues are found, you might want to prompt the user for confirmation before proceeding:

```go
func askUserConfirmation() bool {
    reader := bufio.NewReader(os.Stdin)
    fmt.Print("Precheck found no critical issues. Do you want to continue with the upgrade? (Y/n): ")
    response, err := reader.ReadString('\n')
    if err != nil {
        return false
    }
    
    response = strings.ToLower(strings.TrimSpace(response))
    return response == "y" || response == "yes" || response == ""
}
```

### 7. Error Handling and Logging

Ensure proper error handling and logging throughout the precheck process:

```go
func runPrecheckWithLogging(...) error {
    log.Infof("Starting precheck for cluster upgrade from %s to %s", 
        snapshot.SourceVersion, snapshot.TargetVersion)
    
    // ... precheck logic ...
    
    if err != nil {
        log.Errorf("Precheck failed: %v", err)
        return err
    }
    
    log.Infof("Precheck completed successfully. Report saved to %s", reportPath)
    return nil
}
```

## Testing Considerations

### Unit Tests

Create unit tests for the new functions:

```go
func TestConvertToEndpoint(t *testing.T) {
    // Create a mock topology
    topo := createMockTopology()
    
    // Convert to endpoints
    endpoints := convertToEndpoint(top