package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/pingcap/tidb-upgrade-precheck/pkg/kbgenerator"
	"github.com/pingcap/tidb-upgrade-precheck/pkg/scan"
)

// Knowledge base generation main entry

func main() {
	repo := flag.String("repo", "", "TiDB repository path")
	fromTag := flag.String("from-tag", "", "From tag (inclusive)")
	toTag := flag.String("to-tag", "", "To tag (inclusive)")
	singleTag := flag.String("tag", "", "Process a single tag")
	all := flag.Bool("all", false, "Collect knowledge for all tags")
	testTags := flag.Bool("test-tags", false, "Test tag retrieval")
	aggregate := flag.Bool("aggregate", false, "Aggregate collected knowledge")
	method := flag.String("method", "source", "Collection method: source or binary")
	toolPath := flag.String("tool", "", "Path to the export tool (required for binary method)")
	flag.Parse()

	if *singleTag != "" {
		fmt.Printf("Processing single tag: %s\n", *singleTag)

		// Create output directory
		outputDir := filepath.Join("knowledge", *singleTag)
		if err := os.MkdirAll(outputDir, 0755); err != nil {
			fmt.Fprintf(os.Stderr, "[ERROR] Failed to create output directory: %v\n", err)
			os.Exit(1)
		}

		var snapshot *kbgenerator.KBSnapshot
		var err error

		// Collect based on method
		if *method == "binary" {
			if *toolPath == "" {
				fmt.Fprintf(os.Stderr, "[ERROR] Tool path is required for binary method\n")
				os.Exit(1)
			}
			snapshot, err = kbgenerator.CollectFromTidbBinary(*repo, *singleTag, *toolPath)
		} else {
			snapshot, err = kbgenerator.CollectFromTidbSource(*repo, *singleTag)
		}

		if err != nil {
			fmt.Fprintf(os.Stderr, "[ERROR] Collection failed: %v\n", err)
			os.Exit(1)
		}

		// Write parameters to file
		outputFile := filepath.Join(outputDir, "defaults.json")
		file, err := os.Create(outputFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "[ERROR] Failed to create output file: %v\n", err)
			os.Exit(1)
		}
		defer file.Close()

		encoder := json.NewEncoder(file)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(snapshot); err != nil {
			fmt.Fprintf(os.Stderr, "[ERROR] Failed to write snapshot: %v\n", err)
			os.Exit(1)
		}

		// For upgrade logic, we only need to scan once using the latest code
		// This collects the upgrade logic that will be part of our knowledge base
		bootstrapPath := filepath.Join(*repo, "pkg", "session", "bootstrap.go")
		upgradeLogic, err := kbgenerator.CollectUpgradeLogicFromSource(bootstrapPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "[ERROR] Failed to collect upgrade logic: %v\n", err)
			os.Exit(1)
		}
		
		// Save upgrade logic to knowledge base
		upgradeOutputPath := filepath.Join("knowledge", "upgrade_logic.json")
		if err := kbgenerator.SaveUpgradeLogic(upgradeLogic, upgradeOutputPath); err != nil {
			fmt.Fprintf(os.Stderr, "[ERROR] Failed to save upgrade logic: %v\n", err)
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
		// This collects the upgrade logic that will be part of our knowledge base
		bootstrapPath := filepath.Join(*repo, "pkg", "session", "bootstrap.go")
		upgradeLogic, err := kbgenerator.CollectUpgradeLogicFromSource(bootstrapPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "[ERROR] Failed to collect upgrade logic: %v\n", err)
			os.Exit(1)
		}
		
		// Save upgrade logic to knowledge base
		upgradeOutputPath := filepath.Join("knowledge", "upgrade_logic.json")
		if err := kbgenerator.SaveUpgradeLogic(upgradeLogic, upgradeOutputPath); err != nil {
			fmt.Fprintf(os.Stderr, "[ERROR] Failed to save upgrade logic: %v\n", err)
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