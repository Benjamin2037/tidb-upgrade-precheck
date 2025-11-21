package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/pingcap/tidb-upgrade-precheck/pkg/collectparams"
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
	if err := collectparams.CollectUpgradeLogic(bootstrapPath, outputPath); err != nil {
		fmt.Println("collect failed:", err)
		os.Exit(2)
	}
	fmt.Println("collect success:", outputPath)
}
