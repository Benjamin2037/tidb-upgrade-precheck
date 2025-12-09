# TiUP Integration Implementation Manual

## Overview

This manual provides step-by-step instructions for integrating tidb-upgrade-precheck into the TiUP project. Following these steps will enable TiUP users to perform compatibility checks before upgrading TiDB clusters.

## Prerequisites

1. Access to the TiUP repository
2. Go development environment (version 1.18 or higher)
3. Basic understanding of TiUP codebase
4. tidb-upgrade-precheck module published and accessible

## Implementation Steps

### Step 1: Add Dependency

1. Navigate to the TiUP project root directory
2. Edit the `go.mod` file to add the tidb-upgrade-precheck dependency:
   ```
   require (
       github.com/pingcap/tidb-upgrade-precheck v0.1.0
       // ... other dependencies
   )
   ```
3. Run `go mod tidy` to update dependencies

### Step 2: Update Upgrade Command

1. Open `cmd/cluster/command/upgrade.go`
2. Add the necessary imports:
   ```go
   import (
       precheckRuntime "github.com/pingcap/tidb-upgrade-precheck/pkg/runtime"
       precheckEngine "github.com/pingcap/tidb-upgrade-precheck/pkg/precheck"
       precheckReport "github.com/pingcap/tidb-upgrade-precheck/pkg/report"
       "github.com/pingcap/tidb-upgrade-precheck/pkg/runtime/types"
   )
   ```

3. Add command-line flags for precheck:
   ```go
   // Add precheck flags
   cmd.Flags().Bool("precheck", false, "Perform compatibility check and ask user for confirmation")
   cmd.Flags().Bool("without-precheck", false, "Skip compatibility check and proceed directly to upgrade")
   ```

### Step 3: Integrate Precheck Logic

1. In the `upgradeCluster` function, add the precheck logic before the existing upgrade process:
   ```go
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
           if err := runPrecheck(topo, clusterName, version); err != nil {
                   return fmt.Errorf("precheck failed: %v", err)
           }

           // Ask for user confirmation if not explicitly requesting precheck
           if !explicitPrecheck {
                   if !askUserConfirmation() {
                           fmt.Println("Upgrade cancelled by user.")
                           return nil
                   }
           }
   }
   ```

### Step 4: Implement Helper Functions

1. Add the following functions to handle precheck operations:
   ```go
   func runPrecheck(topo *spec.Specification, clusterName, targetVersion string) error {
           // Convert TiUP topology to endpoints format required by tidb-upgrade-precheck
           endpoints := convertToEndpoint(topo)

           // Initialize collector
           collector := precheckRuntime.NewCollector()

           // Collect cluster snapshot
           snapshot, err := collector.Collect(endpoints)
           if err != nil {
                   return fmt.Errorf("failed to collect cluster information: %v", err)
           }

           // Set version information
           snapshot.SourceVersion = getCurrentVersion(topo) // Implement this function to get current version
           snapshot.TargetVersion = targetVersion

           // Run precheck analysis
           reportData := precheckEngine.FromClusterSnapshot(snapshot)

           // Generate report
           generator := precheckReport.NewGenerator()
           options := &precheckReport.Options{
                   Format:    precheckReport.TextFormat,
                   OutputDir: "./",
           }

           reportPath, err := generator.Generate(reportData, options)
           if err != nil {
                   return fmt.Errorf("failed to generate precheck report: %v", err)
           }

           fmt.Printf("Precheck report generated: %s\n", reportPath)

           return nil
   }

   // Convert TiUP topology to endpoints
   func convertToEndpoint(topo *spec.Specification) types.ClusterEndpoints {
           endpoints := types.ClusterEndpoints{}

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

   // Ask user for confirmation
   func askUserConfirmation() bool {
           reader := bufio.NewReader(os.Stdin)
           fmt.Print("Precheck completed. Do you want to continue with the upgrade? (Y/n): ")
           response, err := reader.ReadString('\n')
           if err != nil {
                   return false
           }

           response = strings.ToLower(strings.TrimSpace(response))
           return response == "y" || response == "yes" || response == ""
   }
   ```

### Step 5: Add Required Imports

Make sure to add the following imports at the top of the file:
```go
import (
    "bufio"
    "fmt"
    "os"
    "strings"
    
    precheckRuntime "github.com/pingcap/tidb-upgrade-precheck/pkg/runtime"
    precheckEngine "github.com/pingcap/tidb-upgrade-precheck/pkg/precheck"
    precheckReport "github.com/pingcap/tidb-upgrade-precheck/pkg/report"
    "github.com/pingcap/tidb-upgrade-precheck/pkg/runtime/types"
)
```

## Testing

### Unit Tests

1. Create unit tests for the helper functions:
   ```go
   func TestConvertToEndpoint(t *testing.T) {
       // Create mock topology
       topo := createMockTopology()
       
       // Convert to endpoints
       endpoints := convertToEndpoint(topo)
       
       // Verify results
       assert.NotEmpty(t, endpoints.TiDBAddr)
       assert.Equal(t, "127.0.0.1:4000", endpoints.TiDBAddr)
       
       assert.NotEmpty(t, endpoints.TiKVAddrs)
       assert.Contains(t, endpoints.TiKVAddrs, "127.0.0.1:20160")
       
       assert.NotEmpty(t, endpoints.PDAddrs)
       assert.Contains(t, endpoints.PDAddrs, "127.0.0.1:2379")
   }
   ```

### Integration Tests

1. Create integration tests to verify the end-to-end functionality:
   ```go
   func TestPrecheckIntegration(t *testing.T) {
       // Setup test cluster
       // Execute precheck
       // Verify results
   }
   ```

## Verification

1. Build the TiUP project:
   ```bash
   make
   ```

2. Test the new command-line flags:
   ```bash
   ./bin/tiup-cluster upgrade --help
   ```

3. Run precheck on a test cluster:
   ```bash
   ./bin/tiup-cluster upgrade my-cluster v7.5.0 --precheck
   ```

## Troubleshooting

1. If you encounter dependency issues, try running `go mod tidy`
2. If you encounter compilation errors, check import paths
3. If you encounter runtime errors, check logs for detailed error messages

## Conclusion

By following these steps, you can successfully integrate tidb-upgrade-precheck into TiUP, providing users with the ability to perform compatibility checks before upgrading TiDB clusters. This integration helps reduce upgrade risks and improves system stability.