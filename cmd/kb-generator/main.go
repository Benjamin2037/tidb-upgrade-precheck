package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/pingcap/tidb-upgrade-precheck/pkg/scan"
)

// Assuming collectFromTidbSource is available in the same project
// import "../collectparams" if package needs to be split

func main() {
	repo := flag.String("repo", "", "TiDB repository path")
	fromTag := flag.String("from-tag", "", "From tag (inclusive)")
	toTag := flag.String("to-tag", "", "To tag (inclusive)")
	singleTag := flag.String("tag", "", "Process a single tag")
	all := flag.Bool("all", false, "Collect knowledge for all tags")
	testTags := flag.Bool("test-tags", false, "Test tag retrieval")
	aggregate := flag.Bool("aggregate", false, "Aggregate collected knowledge")
	flag.Parse()

	if *singleTag != "" {
		fmt.Printf("Processing single tag: %s\n", *singleTag)
		
		// Create output directory
		outputDir := filepath.Join("knowledge", *singleTag)
		if err := os.MkdirAll(outputDir, 0755); err != nil {
			fmt.Fprintf(os.Stderr, "[ERROR] Failed to create output directory: %v\n", err)
			os.Exit(1)
		}
		
		outputFile := filepath.Join(outputDir, "defaults.json")
		if err := scan.ScanDefaults(*repo, outputFile, ""); err != nil {
			fmt.Fprintf(os.Stderr, "[ERROR] ScanDefaults failed: %v\n", err)
			os.Exit(1)
		}
		
		// For upgrade logic, we only need to scan once using the latest code
		if err := scan.ScanUpgradeLogic(*repo, *singleTag); err != nil {
			fmt.Fprintf(os.Stderr, "[ERROR] ScanUpgradeLogic failed: %v\n", err)
			os.Exit(1)
		}
		
		fmt.Println("Single tag processing completed")
		return
	}

	if *all {
		tags, err := scan.GetAllTags(*repo)
		if err != nil {
			fmt.Fprintf(os.Stderr, "[ERROR] GetAllTags failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Found %d LTS tags to process\n", len(tags))
		
		if err := scan.ScanAllAndAggregateParameters(*repo); err != nil {
			fmt.Fprintf(os.Stderr, "[ERROR] ScanAllAndAggregateParameters failed: %v\n", err)
			os.Exit(1)
		}
		
		// After all defaults are collected, generate global upgrade_logic.json
		// For upgrade logic, we only need to scan once using the latest code
		if err := scan.ScanUpgradeLogic(*repo, ""); err != nil {
			fmt.Fprintf(os.Stderr, "[ERROR] ScanUpgradeLogic failed: %v\n", err)
			os.Exit(1)
		}
		
		fmt.Println("Full collection completed")
		return
	}

	if *fromTag != "" && *toTag != "" {
		fmt.Println("[ERROR] Incremental mode not fully implemented yet")
		os.Exit(1)
	}

	if *testTags {
		tags, err := scan.GetAllTags(*repo)
		if err != nil {
			fmt.Fprintf(os.Stderr, "[ERROR] GetAllTags failed: %v\n", err)
			os.Exit(1)
		}
		for _, tag := range tags {
			fmt.Println(tag)
		}
		return
	}

	if *aggregate {
		fmt.Println("[ERROR] Aggregate mode not fully implemented yet")
		os.Exit(1)
	}

	fmt.Println("No mode specified. Use -help for usage information.")
}