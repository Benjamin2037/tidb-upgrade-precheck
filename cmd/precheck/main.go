package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/pingcap/tidb-upgrade-precheck/pkg/report"
	"github.com/pingcap/tidb-upgrade-precheck/pkg/rules"
	"github.com/pingcap/tidb-upgrade-precheck/pkg/runtime"
)

func main() {
	tidbAddr := flag.String("tidb-addr", "127.0.0.1:4000", "TiDB server address")
	tikvAddrs := flag.String("tikv-addrs", "127.0.0.1:20180", "TiKV addresses (comma separated)")
	pdAddrs := flag.String("pd-addrs", "127.0.0.1:2379", "PD addresses (comma separated)")
	outputFormat := flag.String("format", "text", "Output format: text, json, markdown, html")
	reportDir := flag.String("report-dir", "", "Report output directory")
	clusterName := flag.String("cluster-name", "unknown", "Cluster name for report")
	flag.Parse()

	// Parse addresses
	tikvAddrList := parseAddresses(*tikvAddrs)
	pdAddrList := parseAddresses(*pdAddrs)

	// Create runtime collector
	collector := runtime.NewCollector()

	// Define cluster endpoints
	endpoints := runtime.ClusterEndpoints{
		TiDBAddr:  *tidbAddr,
		TiKVAddrs: tikvAddrList,
		PDAddrs:   pdAddrList,
	}

	// Collect cluster snapshot
	fmt.Println("Collecting cluster configuration...")
	snapshot, err := collector.Collect(endpoints)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error collecting cluster info: %v\n", err)
		os.Exit(1)
	}

	// Create checkers
	checkers := []rules.Checker{
		rules.NewConfigChecker(),
		rules.NewSysVarChecker(),
		// Add more checkers here as they are implemented
	}

	// Create check runner
	checkRunner := rules.NewCheckRunner(checkers)

	// Run checks
	fmt.Println("Running upgrade compatibility checks...")
	results, err := checkRunner.Run(snapshot)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error running checks: %v\n", err)
		os.Exit(1)
	}

	// Convert results to report
	r := convertResultsToReport(results, *clusterName, snapshot)

	// Generate report based on format
	switch *outputFormat {
	case "json":
		outputResultsJSON(results)
	case "markdown", "html":
		// Generate report using report generator
		generator := report.NewGenerator()
		format := report.TextFormat
		if *outputFormat == "markdown" {
			format = report.MarkdownFormat
		} else if *outputFormat == "html" {
			format = report.HTMLFormat
		}
		
		reportResult, err := generator.Generate(r, &report.Options{
			Format:    format,
			OutputDir: *reportDir,
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error generating report: %v\n", err)
			os.Exit(1)
		}
		
		if reportResult.Path != "" {
			fmt.Printf("Report generated at: %s\n", reportResult.Path)
		} else {
			fmt.Println(reportResult.Data)
		}
	default:
		outputResultsText(results)
	}
}

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

func outputResultsText(results []rules.CheckResult) {
	if len(results) == 0 {
		fmt.Println("No compatibility issues found.")
		return
	}

	fmt.Printf("Found %d compatibility issues:\n\n", len(results))
	
	for i, result := range results {
		fmt.Printf("%d. [%s] %s\n", i+1, result.Severity, result.Message)
		if result.Details != "" {
			fmt.Printf("   Details: %s\n", result.Details)
		}
		fmt.Println()
	}
}

func outputResultsJSON(results []rules.CheckResult) {
	data, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error marshaling results: %v\n", err)
		os.Exit(1)
	}
	
	fmt.Println(string(data))
}

func convertResultsToReport(results []rules.CheckResult, clusterName string, snapshot *runtime.ClusterSnapshot) *report.Report {
	// Create a basic report structure
	r := &report.Report{
		ClusterName: clusterName,
		UpgradePath: "unknown -> unknown", // In a real implementation, this would be determined from the snapshot
		Summary:     make(map[report.RiskLevel]int),
		Risks:       []report.RiskItem{},
		Audits:      []report.AuditItem{},
	}
	
	// Initialize summary counts
	r.Summary[report.RiskHigh] = 0
	r.Summary[report.RiskMedium] = 0
	r.Summary[report.RiskInfo] = 0
	
	// Convert results to report items
	for _, result := range results {
		// Map severity to risk level
		var level report.RiskLevel
		switch result.Severity {
		case "critical", "error":
			level = report.RiskHigh
			r.Summary[report.RiskHigh]++
		case "warning":
			level = report.RiskMedium
			r.Summary[report.RiskMedium]++
		case "info":
			level = report.RiskInfo
			r.Summary[report.RiskInfo]++
		default:
			level = report.RiskInfo
			r.Summary[report.RiskInfo]++
		}
		
		// Add to risks list
		riskItem := report.RiskItem{
			Component:  "unknown", // In a real implementation, this would be determined from the result
			Parameter:  result.RuleID,
			Current:    "unknown",
			Target:     "unknown",
			Level:      level,
			Impact:     result.Message,
			Suggestion: "Review the configuration",
			RDComment:  result.Details,
		}
		r.Risks = append(r.Risks, riskItem)
	}
	
	// Add some basic audit items from the snapshot
	for componentName, component := range snapshot.Components {
		// Add config items to audit
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
			r.Audits = append(r.Audits, auditItem)
		}
		
		// Add variable items to audit
		for key, value := range component.Variables {
			auditItem := report.AuditItem{
				Component: component.Type,
				Parameter: key,
				Current:   value,
				Target:    "default",
				Status:    fmt.Sprintf("From %s", componentName),
			}
			r.Audits = append(r.Audits, auditItem)
		}
	}
	
	return r
}