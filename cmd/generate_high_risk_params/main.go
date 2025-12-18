package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/pingcap/tidb-upgrade-precheck/pkg/analyzer/rules/high_risk_params"
)

func main() {
	outputPath := flag.String("output", "./knowledge/high_risk_params/default.json", "Output path for high-risk parameters default config")
	flag.Parse()

	// Get knowledge base path from output path
	// If output is ./knowledge/high_risk_params/default.json, kbPath should be ./knowledge
	kbPath := filepath.Dir(filepath.Dir(*outputPath))
	if kbPath == "" || kbPath == "." {
		kbPath = "./knowledge"
	}

	// Generate knowledge base config
	if err := high_risk_params.GenerateKnowledgeBaseConfig(kbPath); err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to generate high-risk parameters config: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("High-risk parameters default config generated successfully at: %s\n", *outputPath)
}

