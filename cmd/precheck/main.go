package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/pingcap/tidb-upgrade-precheck/pkg/precheck"
	"github.com/pingcap/tidb-upgrade-precheck/pkg/report"
	"github.com/pingcap/tidb-upgrade-precheck/pkg/runtime"
	"github.com/spf13/cobra"
)

func main() {
	var (
		targetVersion string
		outputFormat  string
		outputDir     string
		// Topology file (alternative to individual connection parameters)
		topologyFile string
		// Cluster connection parameters (provided by TiUP/Operator)
		// These are used if topology file is not provided
		tidbAddr     string
		tidbUser     string
		tidbPassword string
		tikvAddrs    string // Comma-separated list
		pdAddrs      string // Comma-separated list
	)

	rootCmd := &cobra.Command{
		Use:   "precheck",
		Short: "TiDB Upgrade Precheck Tool",
		Long: `A tool to check compatibility issues before upgrading TiDB cluster.

Connection information can be provided in two ways:
1. Topology file (recommended): Use --topology-file to specify a TiUP/TiDB Operator topology YAML file
2. Individual parameters: Use --tidb-addr, --tikv-addrs, --pd-addrs, etc.

Connection parameters are typically provided by TiUP or TiDB Operator.

The knowledge base is located at ./knowledge in the tidb-upgrade-precheck directory.
Source and target version numbers are used as keys to locate version-specific defaults.json files.`,
		Run: func(cmd *cobra.Command, args []string) {
			runPrecheck(targetVersion, outputFormat, outputDir,
				topologyFile, tidbAddr, tidbUser, tidbPassword, tikvAddrs, pdAddrs)
		},
	}

	// Required flags
	rootCmd.Flags().StringVar(&targetVersion, "target-version", "", "Target TiDB version for upgrade (required)")
	rootCmd.MarkFlagRequired("target-version")

	// Topology file (alternative to individual parameters)
	rootCmd.Flags().StringVar(&topologyFile, "topology-file", "", "Path to cluster topology YAML file (TiUP/TiDB Operator format)")

	// Cluster connection parameters (provided by TiUP/Operator)
	// These are used if topology file is not provided
	rootCmd.Flags().StringVar(&tidbAddr, "tidb-addr", "", "TiDB MySQL protocol endpoint (host:port)")
	rootCmd.Flags().StringVar(&tidbUser, "tidb-user", "", "TiDB MySQL username (provided by TiUP/Operator)")
	rootCmd.Flags().StringVar(&tidbPassword, "tidb-password", "", "TiDB MySQL password (provided by TiUP/Operator)")
	rootCmd.Flags().StringVar(&tikvAddrs, "tikv-addrs", "", "TiKV HTTP API endpoints (comma-separated, provided by TiUP/Operator)")
	rootCmd.Flags().StringVar(&pdAddrs, "pd-addrs", "", "PD HTTP API endpoints (comma-separated, provided by TiUP/Operator)")

	// Output options
	rootCmd.Flags().StringVar(&outputFormat, "format", "text", "Output format (text, markdown, html, json)")
	rootCmd.Flags().StringVar(&outputDir, "output-dir", ".", "Output directory for reports")

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func runPrecheck(targetVersion, outputFormat, outputDir,
	topologyFile, tidbAddr, tidbUser, tidbPassword, tikvAddrs, pdAddrs string) {

	// Knowledge base is fixed at ./knowledge in the tidb-upgrade-precheck directory
	// Source and target version numbers are used as keys to locate version-specific defaults.json files
	const knowledgeBasePath = "knowledge"

	var endpoints *runtime.ClusterEndpoints
	var err error

	// Step 0: Load cluster connection information
	// Priority: topology file > individual parameters
	if topologyFile != "" {
		// Load from topology file (TiUP/TiDB Operator format)
		fmt.Printf("Loading topology from file: %s\n", topologyFile)
		endpoints, err = runtime.LoadTopologyFromFile(topologyFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading topology file: %v\n", err)
			os.Exit(1)
		}

		// Override credentials if provided via command line (for security, passwords are not in topology)
		if tidbUser != "" {
			endpoints.TiDBUser = tidbUser
		}
		if tidbPassword != "" {
			endpoints.TiDBPassword = tidbPassword
		}
	} else {
		// Build ClusterEndpoints from individual command line arguments
		endpoints = &runtime.ClusterEndpoints{
			TiDBAddr:     tidbAddr,
			TiDBUser:     tidbUser,
			TiDBPassword: tidbPassword,
		}

		// Parse comma-separated addresses
		if tikvAddrs != "" {
			endpoints.TiKVAddrs = strings.Split(tikvAddrs, ",")
			for i := range endpoints.TiKVAddrs {
				endpoints.TiKVAddrs[i] = strings.TrimSpace(endpoints.TiKVAddrs[i])
			}
		}

		if pdAddrs != "" {
			endpoints.PDAddrs = strings.Split(pdAddrs, ",")
			for i := range endpoints.PDAddrs {
				endpoints.PDAddrs[i] = strings.TrimSpace(endpoints.PDAddrs[i])
			}
		}
	}

	// Validate that we have at least some connection information
	if endpoints.TiDBAddr == "" && len(endpoints.TiKVAddrs) == 0 && len(endpoints.PDAddrs) == 0 {
		fmt.Fprintf(os.Stderr, "Error: No cluster connection information provided.\n")
		fmt.Fprintf(os.Stderr, "Please provide either --topology-file or connection parameters (--tidb-addr, --tikv-addrs, --pd-addrs)\n")
		os.Exit(1)
	}

	// Step 1: Collect runtime configuration from cluster
	fmt.Println("Collecting cluster configuration...")
	collector := runtime.NewCollector()
	snapshot, err := collector.Collect(*endpoints)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error collecting cluster configuration: %v\n", err)
		os.Exit(1)
	}

	if snapshot == nil {
		fmt.Fprintf(os.Stderr, "Error: failed to collect cluster snapshot\n")
		os.Exit(1)
	}

	// Set target version
	snapshot.TargetVersion = targetVersion

	// Determine source version from collected snapshot
	if snapshot.SourceVersion == "" {
		fmt.Fprintf(os.Stderr, "Warning: could not determine source version from cluster. Using 'unknown'.\n")
		snapshot.SourceVersion = "unknown"
	}

	fmt.Printf("Cluster version: %s -> Target version: %s\n", snapshot.SourceVersion, targetVersion)

	// Step 2: Load knowledge base for source and target versions
	fmt.Println("Loading knowledge base...")
	sourceKB, err := precheck.LoadKnowledgeBase(knowledgeBasePath, "tidb", snapshot.SourceVersion)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to load source knowledge base: %v\n", err)
		sourceKB = make(map[string]interface{})
	}

	targetKB, err := precheck.LoadKnowledgeBase(knowledgeBasePath, "tidb", targetVersion)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to load target knowledge base: %v\n", err)
		fmt.Fprintf(os.Stderr, "Please ensure knowledge base is generated for version %s\n", targetVersion)
		os.Exit(1)
	}

	// Step 3: Create analyzer with knowledge base
	fmt.Println("Initializing analyzer...")
	factory := precheck.NewFactory()
	analyzer := factory.CreateAnalyzerWithKB(sourceKB, targetKB)

	// Step 4: Run analysis
	fmt.Println("Running compatibility checks...")
	ctx := context.Background()
	precheckReport, err := analyzer.Analyze(ctx, snapshot, targetVersion)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error running analysis: %v\n", err)
		os.Exit(1)
	}

	// Step 5: Generate report
	fmt.Println("Generating report...")
	generator := report.NewGenerator()
	options := &report.Options{
		Format:    report.Format(outputFormat),
		OutputDir: outputDir,
	}

	reportPath, err := generator.Generate(precheckReport, options)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error generating report: %v\n", err)
		os.Exit(1)
	}

	// Step 6: Print summary
	fmt.Printf("\n=== Precheck Summary ===\n")
	fmt.Printf("Total issues: %d\n", precheckReport.Summary.Total)
	fmt.Printf("Blocking issues: %d\n", precheckReport.Summary.Blocking)
	fmt.Printf("Warnings: %d\n", precheckReport.Summary.Warnings)
	fmt.Printf("Info: %d\n", precheckReport.Summary.Infos)

	if precheckReport.Summary.Blocking > 0 {
		fmt.Printf("\n⚠️  WARNING: %d blocking issue(s) found. Please review before upgrading.\n", precheckReport.Summary.Blocking)
	}

	fmt.Printf("\nReport generated successfully: %s\n", reportPath)
}
