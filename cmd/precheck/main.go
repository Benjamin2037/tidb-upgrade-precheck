package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/pingcap/tidb-upgrade-precheck/pkg/metadata"
	"github.com/pingcap/tidb-upgrade-precheck/pkg/precheck"
	reportpkg "github.com/pingcap/tidb-upgrade-precheck/pkg/report"
	"github.com/pingcap/tidb-upgrade-precheck/pkg/rules"
)

var (
	snapshotPath = flag.String("snapshot", "", "snapshot JSON file path")
	timeout      = flag.Duration("timeout", 30*time.Second, "maximum execution time")
	metadataPath = flag.String("upgrade-metadata", "", "path to TiDB upgrade metadata JSON (tools/upgrade-metadata/upgrade_changes.json)")
	reportFormat = flag.String("report-format", "", "report format: md or html (optional)")
	reportDir    = flag.String("report-dir", "", "directory to write the report file (optional)")
)

func main() {
	flag.Parse()

	snapshot, err := loadSnapshot()
	if err != nil {
		fatal(err)
	}

	if err := precheck.ValidateSnapshot(snapshot); err != nil {
		fatal(fmt.Errorf("invalid snapshot: %w", err))
	}

	ruleSet := []precheck.Rule{
		rules.NewTargetVersionOrderRule(),
	}

	var catalog *metadata.Catalog
	if *metadataPath != "" {
		cat, err := metadata.LoadCatalog(*metadataPath)
		if err != nil {
			fatal(err)
		}
		catalog = cat
	}
	if rule := rules.NewForcedGlobalSysvarsRule(catalog); rule != nil {
		ruleSet = append(ruleSet, rule)
	}
	e := precheck.NewEngine(ruleSet...)

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	report := e.Run(ctx, snapshot)

	// Print raw JSON to console
	output, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		fatal(err)
	}
	fmt.Println(string(output))

	// Generate structured report
	if *reportFormat != "" && *reportDir != "" {
		clusterName := "prod-cluster" // Can be set based on actual snapshot/parameters
		upgradePath := snapshot.SourceVersion + " -> " + snapshot.TargetVersion
		r := reportpkg.ConvertEngineReportToReport(clusterName, upgradePath, &report)
		file, err := reportpkg.WriteReportToFile(r, *reportFormat, *reportDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to write report: %v\n", err)
		} else {
			fmt.Fprintf(os.Stderr, "Report written to: %s\n", file)
		}
	}

	if report.HasBlocking() {
		os.Exit(2)
	}
}

func loadSnapshot() (precheck.Snapshot, error) {
	if *snapshotPath == "" {
		return precheck.Snapshot{}, fmt.Errorf("--snapshot is required")
	}
	data, err := os.ReadFile(*snapshotPath)
	if err != nil {
		return precheck.Snapshot{}, err
	}
	var snapshot precheck.Snapshot
	if err := json.Unmarshal(data, &snapshot); err != nil {
		return precheck.Snapshot{}, err
	}
	return snapshot, nil
}

func fatal(err error) {
	fmt.Fprintf(os.Stderr, "precheck: %v\n", err)
	os.Exit(1)
}
