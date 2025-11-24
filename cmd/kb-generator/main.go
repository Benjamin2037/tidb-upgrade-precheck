package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/pingcap/tidb-upgrade-precheck/pkg/scan"
)

// Assuming collectFromTidbSource is available in the same project
// import "../collectparams" if package needs to be split

func main() {
	repo := flag.String("repo", "../tidb", "TiDB source code root directory")
	fromTag := flag.String("from-tag", "", "Starting tag (incremental mode)")
	toTag := flag.String("to-tag", "", "Target tag (incremental mode)")
	singleTag := flag.String("tag", "", "Single tag to process")
	all := flag.Bool("all", false, "Full rebuild mode")
	aggregate := flag.Bool("aggregate", false, "Aggregate parameter history")
	testTags := flag.Bool("test-tags", false, "Test tag retrieval")
	flag.Parse()

	if *singleTag != "" {
		fmt.Printf("Processing single tag: %s\n", *singleTag)
		
		if err := scan.ScanDefaults(*repo, *singleTag); err != nil {
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