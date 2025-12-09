# TiUP Implementation Guide

This document provides step-by-step instructions for integrating tidb-upgrade-precheck into TiUP.

## Overview

The integration uses a component-based approach where `tiup-cluster` calls the `upgrade-precheck` component as a subprocess. This simplifies integration and allows independent component updates.

## Prerequisites

1. The `upgrade-precheck` component must be installed in TiUP
2. Component includes complete knowledge base
3. Runtime data directory is initialized

## Implementation Steps

### 1. Install upgrade-precheck Component

The component is automatically installed with TiUP, or can be manually installed:

```bash
tiup install upgrade-precheck
```

### 2. Call Precheck Component from tiup-cluster

In the `tiup-cluster` upgrade command, call the precheck component:

```go
import (
    "os/exec"
    "fmt"
)

func runPrecheck(clusterName, targetVersion string, opt *Options) error {
    // Build command to call upgrade-precheck component
    args := []string{
        "upgrade-precheck",
        clusterName,
        targetVersion,
    }
    
    // Add optional flags
    if opt.PrecheckFormat != "" {
        args = append(args, "--format", opt.PrecheckFormat)
    }
    if opt.PrecheckOutputDir != "" {
        args = append(args, "--output-dir", opt.PrecheckOutputDir)
    }
    if opt.HighRiskParamsConfig != "" {
        args = append(args, "--high-risk-params-config", opt.HighRiskParamsConfig)
    }
    
    // Execute precheck component
    cmd := exec.Command("tiup", args...)
    cmd.Stdout = os.Stdout
    cmd.Stderr = os.Stderr
    
    if err := cmd.Run(); err != nil {
        return fmt.Errorf("precheck failed: %v", err)
    }
    
    return nil
}
```

### 3. Integrate into Upgrade Command

Add precheck execution to the upgrade command flow:

```go
func upgradeCluster(clusterName, targetVersion string, opt *Options) error {
    // Check if precheck should be skipped
    if !opt.WithoutPrecheck {
        // Run precheck
        if err := runPrecheck(clusterName, targetVersion, opt); err != nil {
            if opt.PrecheckStrict {
                return fmt.Errorf("precheck failed, aborting upgrade: %v", err)
            } else {
                log.Warnf("Precheck failed but continuing: %v", err)
            }
        }
        
        // Check if only precheck is requested
        if opt.PrecheckOnly {
            fmt.Println("Precheck completed. Exiting due to --precheck-only flag.")
            return nil
        }
        
        // Ask for user confirmation
        if !askUserConfirmation() {
            return fmt.Errorf("upgrade cancelled by user")
        }
    }
    
    // Continue with upgrade
    // ... existing upgrade logic ...
    return nil
}
```

### 4. Command Line Flags

Add flags to the upgrade command:

```go
cmd.Flags().Bool("precheck", false, "Execute precheck before upgrade")
cmd.Flags().Bool("without-precheck", false, "Skip precheck and proceed directly")
cmd.Flags().Bool("precheck-only", false, "Only run precheck, don't upgrade")
cmd.Flags().String("precheck-format", "text", "Precheck report format")
cmd.Flags().String("precheck-output-dir", "./", "Precheck report output directory")
cmd.Flags().Bool("precheck-strict", true, "Abort upgrade if precheck fails")
```

### 5. Error Handling

Handle different error scenarios:

```go
func runPrecheckWithErrorHandling(clusterName, targetVersion string) error {
    cmd := exec.Command("tiup", "upgrade-precheck", clusterName, targetVersion)
    
    output, err := cmd.CombinedOutput()
    if err != nil {
        // Check exit code
        if exitError, ok := err.(*exec.ExitError); ok {
            exitCode := exitError.ExitCode()
            switch exitCode {
            case 1:
                return fmt.Errorf("precheck found issues: %s", string(output))
            case 2:
                return fmt.Errorf("precheck execution error: %s", string(output))
            default:
                return fmt.Errorf("precheck failed with code %d: %s", exitCode, string(output))
            }
        }
        return fmt.Errorf("failed to execute precheck: %v", err)
    }
    
    fmt.Println(string(output))
    return nil
}
```

## Testing

### Test Component Availability

```go
func isPrecheckComponentAvailable() bool {
    cmd := exec.Command("tiup", "list", "upgrade-precheck")
    err := cmd.Run()
    return err == nil
}
```

### Test Precheck Execution

```go
func TestPrecheckExecution(t *testing.T) {
    if !isPrecheckComponentAvailable() {
        t.Skip("upgrade-precheck component not available")
    }
    
    err := runPrecheck("test-cluster", "v8.1.0", &Options{})
    if err != nil {
        t.Fatalf("Precheck failed: %v", err)
    }
}
```

## Related Documents

- [TiUP Integration Design](./tiup_integration_design.md) - Integration architecture
- [TiUP Deployment Guide](../tiup_deployment.md) - Component packaging and deployment
- [TiUP Integration Patch](./tiup_integration_patch.diff) - Example code changes

---

**Last Updated**: 2025
