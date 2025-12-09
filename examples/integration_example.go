package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/pingcap/tidb-upgrade-precheck/pkg/report"
	"github.com/pingcap/tidb-upgrade-precheck/pkg/rules"
	"github.com/pingcap/tidb-upgrade-precheck/pkg/runtime"
)

// This example demonstrates how to integrate all components of the tidb-upgrade-precheck system
func main() {
	// Command line flags for cluster configuration
	tidbAddr := flag.String("tidb-addr", "127.0.0.1:4000", "TiDB server address")
	tikvAddrs := flag.String("tikv-addrs", "127.0.0.1:20180", "TiKV addresses (comma separated)")
	pdAddrs := flag.String("pd-addrs", "127.0.0.1:2379", "PD addresses (comma separated)")
	clusterName := flag.String("cluster-name", "example-cluster", "Cluster name for report")
	
	flag.Parse()

	fmt.Println("TiDB Upgrade Precheck Integration Example")
	fmt.Println("=========================================")

	// Step 1: Collect cluster configuration
	fmt.Println("Step 1: Collecting cluster configuration...")
	collector := runtime.NewCollector()

	// Parse addresses from command line flags
	tikvAddrList := parseAddresses(*tikvAddrs)
	pdAddrList := parseAddresses(*pdAddrs)

	endpoints := runtime.ClusterEndpoints{
		TiDBAddr:  *tidbAddr,
		TiKVAddrs: tikvAddrList,
		PDAddrs:   pdAddrList,
	}

	snapshot, err := collector.Collect(endpoints)
	if err != nil {
		fmt.Printf("Error collecting cluster info: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Collected configuration from %d components\n", len(snapshot.Components))

	// Step 2: Run compatibility checks
	fmt.Println("\nStep 2: Running compatibility checks...")
	checkers := []rules.Checker{
		rules.NewConfigChecker(),
		rules.NewSysVarChecker(),
	}

	checkRunner := rules.NewCheckRunner(checkers)
	results, err := checkRunner.Run(snapshot)
	if err != nil {
		fmt.Printf("Error running checks: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Found %d potential issues\n", len(results))

	// Step 3: Generate report
	fmt.Println("\nStep 3: Generating report...")
	reportData := &report.Report{
		ClusterName: *clusterName,
		UpgradePath: "v7.5.0 -> v8.0.0",
		Summary:     make(map[report.RiskLevel]int),
		Risks:       []report.RiskItem{},
		Audits:      []report.AuditItem{},
	}

	// Initialize summary counts
	reportData.Summary[report.RiskHigh] = 0
	reportData.Summary[report.RiskMedium] = 0
	reportData.Summary[report.RiskInfo] = 0

	// Convert results to report items (simplified)
	for _, result := range results {
		var level report.RiskLevel
		switch result.Severity {
		case "critical", "error":
			level = report.RiskHigh
			reportData.Summary[report.RiskHigh]++
		case "warning":
			level = report.RiskMedium
			reportData.Summary[report.RiskMedium]++
		case "info":
			level = report.RiskInfo
			reportData.Summary[report.RiskInfo]++
		default:
			level = report.RiskInfo
			reportData.Summary[report.RiskInfo]++
		}

		riskItem := report.RiskItem{
			Component:  "tidb",
			Parameter:  result.RuleID,
			Current:    "unknown",
			Target:     "unknown",
			Level:      level,
			Impact:     result.Message,
			Suggestion: "Review the configuration",
			RDComment:  result.Details,
		}
		reportData.Risks = append(reportData.Risks, riskItem)
	}

	// Add audit items
	for componentName, component := range snapshot.Components {
		for key, value := range component.Config {
			var valueStr string
			if s, ok := value.(string); ok {
				valueStr = s
			} else {
				valueStr = fmt.Sprintf("%v", value)
			}

			auditItem := report.AuditItem{
				Component: component.Type,
				Parameter: key,
				Current:   valueStr,
				Target:    "default",
				Status:    fmt.Sprintf("From %s", componentName),
			}
			reportData.Audits = append(reportData.Audits, auditItem)
		}
	}

	// Generate markdown report
	generator := report.NewGenerator()
	reportPath, err := generator.Generate(reportData, &report.Options{
		Format:    report.MarkdownFormat,
		OutputDir: "./out",
	})
	if err != nil {
		fmt.Printf("Error generating report: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Report generated successfully: %s\n", reportPath)
	fmt.Println("\nIntegration example completed!")
}

// parseAddresses parses a comma-separated list of addresses
func parseAddresses(addrStr string) []string {
	if addrStr == "" {
		return []string{}
	}
	
	// Split by comma and trim spaces
	addrs := strings.Split(addrStr, ",")
	for i, addr := range addrs {
		addrs[i] = strings.TrimSpace(addr)
	}
	
	return addrs
}