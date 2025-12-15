package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pingcap/tidb-upgrade-precheck/pkg/analyzer"
	"github.com/pingcap/tidb-upgrade-precheck/pkg/analyzer/rules"
	"github.com/pingcap/tidb-upgrade-precheck/pkg/collector"
	"github.com/pingcap/tidb-upgrade-precheck/pkg/reporter"
	"github.com/spf13/cobra"
)

func main() {
	var (
		sourceVersion string // Optional: if not provided, will be detected from cluster
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
		// High-risk parameters configuration
		highRiskParamsConfig string
	)

	rootCmd := &cobra.Command{
		Use:   "precheck",
		Short: "TiDB Upgrade Precheck Tool",
		Long: `A tool to check compatibility issues before upgrading TiDB cluster.

Connection information can be provided in two ways:
1. Topology file (recommended): Use --topology-file to specify a TiUP/TiDB Operator topology YAML file
2. Individual parameters: Use --tidb-addr, --tikv-addrs, --pd-addrs, etc.

Connection parameters are typically provided by TiUP or TiDB Operator.

The knowledge base is automatically located:
- When running as TiUP component: ~/.tiup/components/tidb-upgrade-precheck/<version>/knowledge/
- When running standalone: ./knowledge relative to the binary location

Source and target version numbers are used as keys to locate version-specific defaults.json files.`,
		Run: func(cmd *cobra.Command, args []string) {
			runPrecheck(sourceVersion, targetVersion, outputFormat, outputDir,
				topologyFile, tidbAddr, tidbUser, tidbPassword, tikvAddrs, pdAddrs, highRiskParamsConfig)
		},
	}

	// Version flags
	rootCmd.Flags().StringVar(&sourceVersion, "source-version", "", "Source TiDB version (current cluster version). If not provided, will be detected from cluster")
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

	// High-risk parameters configuration
	rootCmd.Flags().StringVar(&highRiskParamsConfig, "high-risk-params-config", "", "Path to high-risk parameters configuration file (JSON format). If not specified, will try to load from default locations")

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func runPrecheck(sourceVersion, targetVersion, outputFormat, outputDir,
	topologyFile, tidbAddr, tidbUser, tidbPassword, tikvAddrs, pdAddrs, highRiskParamsConfig string) {

	// Knowledge base is fixed at ./knowledge in the tidb-upgrade-precheck directory
	// Source and target version numbers are used as keys to locate version-specific defaults.json files
	// Try multiple locations:
	// 1. Environment variable TIDB_UPGRADE_PRECHECK_KNOWLEDGE_BASE
	// 2. Relative to executable (for TiUP component installation)
	// 3. Current working directory
	// 4. Relative paths from executable (go up to find tidb-upgrade-precheck directory)
	var knowledgeBasePath string
	if envPath := os.Getenv("TIDB_UPGRADE_PRECHECK_KNOWLEDGE_BASE"); envPath != "" {
		knowledgeBasePath = envPath
	} else {
		// Try multiple locations
		candidates := []string{
			"knowledge", // Current working directory
		}

		// Try relative to executable
		if execPath, execErr := os.Executable(); execErr == nil {
			execDir := filepath.Dir(execPath)
			candidates = append(candidates,
				filepath.Join(execDir, "knowledge"),                                // Same dir as executable
				filepath.Join(execDir, "..", "knowledge"),                          // Parent dir
				filepath.Join(execDir, "..", "tidb-upgrade-precheck", "knowledge"), // Go up to find tidb-upgrade-precheck
			)
		}

		// Find first existing path
		for _, candidate := range candidates {
			if absPath, absErr := filepath.Abs(candidate); absErr == nil {
				if _, statErr := os.Stat(absPath); statErr == nil {
					knowledgeBasePath = absPath
					break
				}
			}
		}

		// Final fallback
		if knowledgeBasePath == "" {
			if absPath, absErr := filepath.Abs("knowledge"); absErr == nil {
				knowledgeBasePath = absPath
			} else {
				knowledgeBasePath = "knowledge"
			}
		}
	}
	fmt.Printf("[DEBUG] Using knowledge base path: %s\n", knowledgeBasePath)

	var endpoints *collector.ClusterEndpoints
	var err error

	// Step 0: Load cluster connection information
	// Priority: topology file > individual parameters
	if topologyFile != "" {
		// Load from topology file (TiUP/TiDB Operator format)
		fmt.Printf("Loading topology from file: %s\n", topologyFile)
		endpoints, err = collector.LoadTopologyFromFile(topologyFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading topology file: %v\n", err)
			os.Exit(1)
		}

		// Extract source version from topology if available
		if sourceVersion == "" && endpoints.SourceVersion != "" {
			sourceVersion = endpoints.SourceVersion
			fmt.Printf("Extracted source version from topology: %s\n", sourceVersion)
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
		endpoints = &collector.ClusterEndpoints{
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

	// Step 1: Create analyzer with default rules to determine data requirements
	fmt.Println("Initializing analyzer...")

	// Build rules list
	var rulesList []rules.Rule

	// Add default rules
	rulesList = append(rulesList,
		rules.NewUserModifiedParamsRule(),
		rules.NewUpgradeDifferencesRule(),
		rules.NewTikvConsistencyRule(),
	)

	// Add high-risk parameters rule if config is provided
	if highRiskParamsConfig != "" {
		fmt.Printf("Loading high-risk parameters from: %s\n", highRiskParamsConfig)
		highRiskRule, err := rules.NewHighRiskParamsRule(highRiskParamsConfig)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to load high-risk params rule: %v\n", err)
			fmt.Fprintf(os.Stderr, "Continuing without high-risk parameters check...\n")
		} else {
			rulesList = append(rulesList, highRiskRule)
			fmt.Printf("High-risk parameters rule loaded successfully\n")
		}
	} else {
		// Try default locations
		homeDir, err := os.UserHomeDir()
		if err == nil {
			defaultPaths := []string{
				filepath.Join(homeDir, ".tiup", "high_risk_params.json"),
				filepath.Join(homeDir, ".tidb-upgrade-precheck", "high_risk_params.json"),
			}
			for _, defaultPath := range defaultPaths {
				if _, err := os.Stat(defaultPath); err == nil {
					fmt.Printf("Loading high-risk parameters from default location: %s\n", defaultPath)
					highRiskRule, err := rules.NewHighRiskParamsRule(defaultPath)
					if err != nil {
						fmt.Fprintf(os.Stderr, "Warning: failed to load high-risk params rule from %s: %v\n", defaultPath, err)
						continue
					}
					rulesList = append(rulesList, highRiskRule)
					fmt.Printf("High-risk parameters rule loaded successfully\n")
					break
				}
			}
		}
	}

	analyzerOptions := &analyzer.AnalysisOptions{
		Rules: rulesList,
	}
	analyzerInstance := analyzer.NewAnalyzer(analyzerOptions)

	// Step 2: Get collection requirements from rules
	// This allows us to optimize collection by only gathering necessary data
	fmt.Println("Determining data requirements from rules...")
	analyzerCollectReq := analyzerInstance.GetCollectionRequirements()

	// Step 3: Collect runtime configuration from cluster based on requirements
	fmt.Println("Collecting cluster configuration...")
	collectorInstance := collector.NewCollector()
	// Convert analyzer's CollectionRequirements to collector's CollectDataRequirements
	// (They have the same structure, so we can convert directly)
	collectReq := collector.CollectDataRequirements{
		Components:          analyzerCollectReq.Components,
		NeedConfig:          analyzerCollectReq.NeedConfig,
		NeedSystemVariables: analyzerCollectReq.NeedSystemVariables,
		NeedAllTikvNodes:    analyzerCollectReq.NeedAllTikvNodes,
	}
	snapshot, err := collectorInstance.Collect(*endpoints, &collectReq)
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

	// Determine source version: priority: user input > topology file > cluster detection
	if sourceVersion != "" {
		// Use user-provided source version (highest priority)
		snapshot.SourceVersion = sourceVersion
		fmt.Printf("Using provided source version: %s\n", sourceVersion)
	} else if snapshot.SourceVersion != "" {
		// Use version detected from cluster (from topology file or runtime detection)
		fmt.Printf("Detected source version from cluster: %s\n", snapshot.SourceVersion)
	} else {
		// Neither user input, topology file, nor cluster detection provided a version
		fmt.Fprintf(os.Stderr, "Error: could not determine source version.\n")
		fmt.Fprintf(os.Stderr, "Please provide --source-version, ensure topology file contains version, or ensure cluster connection is working.\n")
		os.Exit(1)
	}

	fmt.Printf("Cluster version: %s -> Target version: %s\n", snapshot.SourceVersion, targetVersion)

	// Step 4: Load knowledge base for source and target versions based on requirements
	fmt.Println("Loading knowledge base...")
	sourceKB, err := collector.LoadKnowledgeBase(knowledgeBasePath, snapshot.SourceVersion)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to load source knowledge base: %v\n", err)
		sourceKB = make(map[string]interface{})
	}

	targetKB, err := collector.LoadKnowledgeBase(knowledgeBasePath, targetVersion)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to load target knowledge base: %v\n", err)
		fmt.Fprintf(os.Stderr, "Please ensure knowledge base is generated for version %s\n", targetVersion)
		os.Exit(1)
	}

	// Step 5: Run analysis using rules
	fmt.Println("Running compatibility checks...")
	ctx := context.Background()
	analysisResult, err := analyzerInstance.Analyze(ctx, snapshot, snapshot.SourceVersion, targetVersion, sourceKB, targetKB)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error running analysis: %v\n", err)
		os.Exit(1)
	}

	// Step 5: Generate report
	fmt.Println("Generating report...")
	generator := reporter.NewGenerator()
	options := &reporter.Options{
		Format:    reporter.Format(outputFormat),
		OutputDir: outputDir,
	}

	reportPath, err := generator.GenerateFromAnalysisResult(analysisResult, options)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error generating report: %v\n", err)
		os.Exit(1)
	}

	// Step 6: Print summary
	fmt.Printf("\n=== Precheck Summary ===\n")
	fmt.Printf("Modified Parameters: %d\n", countModifiedParams(analysisResult.ModifiedParams))
	fmt.Printf("TiKV Inconsistencies: %d\n", len(analysisResult.TikvInconsistencies))
	fmt.Printf("Upgrade Differences: %d\n", countUpgradeDifferences(analysisResult.UpgradeDifferences))
	fmt.Printf("Forced Changes: %d\n", countForcedChanges(analysisResult.ForcedChanges))
	fmt.Printf("Focus Parameters: %d\n", countFocusParams(analysisResult.FocusParams))
	fmt.Printf("Check Results: %d\n", len(analysisResult.CheckResults))

	// Count critical issues
	criticalCount := 0
	for _, check := range analysisResult.CheckResults {
		if check.Severity == "critical" || check.Severity == "error" {
			criticalCount++
		}
	}

	if criticalCount > 0 {
		fmt.Printf("\n⚠️  WARNING: %d critical issue(s) found. Please review before upgrading.\n", criticalCount)
	}

	fmt.Printf("\nReport generated successfully: %s\n", reportPath)
}

// Helper functions for summary
func countModifiedParams(modifiedParams map[string]map[string]analyzer.ModifiedParamInfo) int {
	count := 0
	for _, params := range modifiedParams {
		count += len(params)
	}
	return count
}

func countUpgradeDifferences(differences map[string]map[string]analyzer.UpgradeDifference) int {
	count := 0
	for _, params := range differences {
		count += len(params)
	}
	return count
}

func countForcedChanges(forcedChanges map[string]map[string]analyzer.ForcedChange) int {
	count := 0
	for _, params := range forcedChanges {
		count += len(params)
	}
	return count
}

func countFocusParams(focusParams map[string]map[string]analyzer.FocusParamInfo) int {
	count := 0
	for _, params := range focusParams {
		count += len(params)
	}
	return count
}
