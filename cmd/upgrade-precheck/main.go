// Copyright 2024 PingCAP, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/pingcap/tidb-upgrade-precheck/pkg/analyzer"
	"github.com/pingcap/tidb-upgrade-precheck/pkg/reporter"
	"github.com/pingcap/tidb-upgrade-precheck/pkg/runtime"
)

var (
	endpoints     = flag.String("endpoints", "", "Comma-separated list of cluster endpoints (e.g., 127.0.0.1:2379)")
	component     = flag.String("component", "", "Component to analyze (tidb, pd, tikv)")
	fromVersion   = flag.String("from-version", "", "Source version for upgrade analysis")
	toVersion     = flag.String("to-version", "", "Target version for upgrade analysis")
	reportFormat  = flag.String("format", "text", "Report format (json, text, html)")
	knowledgePath = flag.String("knowledge-path", "./knowledge", "Path to knowledge base")
	output        = flag.String("output", "", "Output file path (default: stdout)")
)

func main() {
	flag.Parse()

	if *endpoints == "" && (*component == "" || *fromVersion == "" || *toVersion == "") {
		fmt.Println("Usage:")
		fmt.Println("  For upgrade analysis:")
		fmt.Println("    upgrade-precheck --component=tidb|pd|tikv --from-version=VERSION --to-version=VERSION [--format=json|text|html] [--knowledge-path=PATH] [--output=FILE]")
		fmt.Println("")
		fmt.Println("  For cluster configuration analysis:")
		fmt.Println("    upgrade-precheck --endpoints=ENDPOINTS [--format=json|text|html] [--knowledge-path=PATH] [--output=FILE]")
		fmt.Println("")
		fmt.Println("Examples:")
		fmt.Println("  upgrade-precheck --component=tidb --from-version=v6.5.0 --to-version=v7.1.0")
		fmt.Println("  upgrade-precheck --endpoints=127.0.0.1:2379,127.0.0.1:2380")
		os.Exit(1)
	}

	// Parse report format
	var format reporter.ReportFormat
	switch strings.ToLower(*reportFormat) {
	case "json":
		format = reporter.JSONFormat
	case "html":
		format = reporter.HTMLFormat
	default:
		format = reporter.TextFormat
	}

	// Create reporter
	rep := reporter.NewReporter(format)

	if *endpoints != "" {
		// Cluster configuration analysis
		if err := runClusterAnalysis(rep, *endpoints, *knowledgePath, *output); err != nil {
			log.Fatalf("Cluster analysis failed: %v", err)
		}
	} else {
		// Upgrade analysis
		if err := runUpgradeAnalysis(rep, *component, *fromVersion, *toVersion, *knowledgePath, *output); err != nil {
			log.Fatalf("Upgrade analysis failed: %v", err)
		}
	}
}

// runUpgradeAnalysis performs upgrade analysis
func runUpgradeAnalysis(rep *reporter.Reporter, component, fromVersion, toVersion, knowledgePath, output string) error {
	// Parse component type
	var compType analyzer.ComponentType
	switch strings.ToLower(component) {
	case "tidb":
		compType = analyzer.TiDBComponent
	case "pd":
		compType = analyzer.PDComponent
	case "tikv":
		compType = analyzer.TiKVComponent
	default:
		return fmt.Errorf("unsupported component: %s", component)
	}

	// Create analyzer
	analy := analyzer.NewAnalyzer(knowledgePath)

	// Perform analysis
	report, err := analy.AnalyzeUpgrade(compType, fromVersion, toVersion)
	if err != nil {
		return fmt.Errorf("failed to analyze upgrade: %v", err)
	}

	// Generate report
	data, err := rep.GenerateUpgradeReport(report)
	if err != nil {
		return fmt.Errorf("failed to generate report: %v", err)
	}

	// Output report
	if output != "" {
		if err := os.WriteFile(output, data, 0644); err != nil {
			return fmt.Errorf("failed to write report to file: %v", err)
		}
		fmt.Printf("Report written to %s\n", output)
	} else {
		fmt.Print(string(data))
	}

	return nil
}

// runClusterAnalysis performs cluster configuration analysis
func runClusterAnalysis(rep *reporter.Reporter, endpoints, knowledgePath, output string) error {
	// Parse endpoints
	endpointList := strings.Split(endpoints, ",")

	// Create collector
	collector := runtime.NewCollector(endpointList)

	// Collect cluster state
	clusterState, err := collector.Collect()
	if err != nil {
		return fmt.Errorf("failed to collect cluster state: %v", err)
	}

	// Create analyzer
	analy := analyzer.NewAnalyzer(knowledgePath)

	// Perform analysis
	report, err := analy.AnalyzeCluster(clusterState)
	if err != nil {
		return fmt.Errorf("failed to analyze cluster: %v", err)
	}

	// Generate report
	data, err := rep.GenerateClusterReport(report)
	if err != nil {
		return fmt.Errorf("failed to generate report: %v", err)
	}

	// Output report
	if output != "" {
		if err := os.WriteFile(output, data, 0644); err != nil {
			return fmt.Errorf("failed to write report to file: %v", err)
		}
		fmt.Printf("Report written to %s\n", output)
	} else {
		fmt.Print(string(data))
	}

	return nil
}