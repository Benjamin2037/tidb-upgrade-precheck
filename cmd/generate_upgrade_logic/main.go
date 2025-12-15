package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	kbgenerator "github.com/pingcap/tidb-upgrade-precheck/pkg/collector"
	tidbkb "github.com/pingcap/tidb-upgrade-precheck/pkg/collector/tidb"
)

func main() {
	tidbRepoRoot := flag.String("tidb-repo", "", "Path to TiDB repository root")
	outputPath := flag.String("output", "", "Output path for upgrade_logic.json")
	flag.Parse()

	if *tidbRepoRoot == "" {
		log.Fatalf("Error: --tidb-repo is required")
	}
	if *outputPath == "" {
		log.Fatalf("Error: --output is required")
	}

	fmt.Printf("Collecting TiDB upgrade logic from source code...\n")
	fmt.Printf("Repository: %s\n", *tidbRepoRoot)

	upgradeLogic, err := tidbkb.CollectUpgradeLogicFromSource(*tidbRepoRoot)
	if err != nil {
		log.Fatalf("Failed to collect TiDB upgrade logic: %v", err)
	}

	// Ensure output directory exists
	if err := os.MkdirAll(filepath.Dir(*outputPath), 0755); err != nil {
		log.Fatalf("Failed to create output directory: %v", err)
	}

	if err := kbgenerator.SaveUpgradeLogic(upgradeLogic, *outputPath); err != nil {
		log.Fatalf("Failed to save TiDB upgrade logic: %v", err)
	}

	totalChanges := 0
	if upgradeLogic != nil && upgradeLogic.Changes != nil {
		totalChanges = len(upgradeLogic.Changes)
	}
	fmt.Printf("Successfully generated upgrade_logic.json with %d total forced changes\n", totalChanges)
	fmt.Printf("Saved to: %s\n", *outputPath)
}

