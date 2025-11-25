package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/pingcap/tidb-upgrade-precheck/pkg/kbgenerator"
)

func main() {
	var bootstrapPath string
	var outputPath string
	flag.StringVar(&bootstrapPath, "bootstrap", "", "Path to bootstrap.go")
	flag.StringVar(&outputPath, "output", "./knowledge/upgrade_logic.json", "Path to output upgrade_logic.json (default: ./knowledge/upgrade_logic.json)")
	flag.Parse()
	if bootstrapPath == "" {
		fmt.Println("Usage: upgrade-logic-collector -bootstrap /path/to/bootstrap.go [-output ./knowledge/upgrade_logic.json]")
		os.Exit(1)
	}
	
	// Collect upgrade logic
	upgradeLogic, err := kbgenerator.CollectUpgradeLogicFromSource(bootstrapPath)
	if err != nil {
		fmt.Println("collect failed:", err)
		os.Exit(2)
	}
	
	// Create output directory if it doesn't exist
	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		fmt.Println("failed to create output directory:", err)
		os.Exit(2)
	}
	
	// Save upgrade logic
	if err := kbgenerator.SaveUpgradeLogic(upgradeLogic, outputPath); err != nil {
		fmt.Println("failed to save upgrade logic:", err)
		os.Exit(2)
	}
	
	fmt.Println("collect success:", outputPath)
}