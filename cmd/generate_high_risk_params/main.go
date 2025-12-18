package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

func main() {
	sourcePath := flag.String("source", "./pkg/analyzer/rules/high_risk_params/default.json", "Source path for high-risk parameters default config")
	outputPath := flag.String("output", "./knowledge/high_risk_params/default.json", "Output path for high-risk parameters default config")
	flag.Parse()

	// Copy default.json from pkg directory to knowledge directory
	sourceFile, err := os.Open(*sourcePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to open source file %s: %v\n", *sourcePath, err)
		os.Exit(1)
	}
	defer sourceFile.Close()

	// Create output directory if it doesn't exist
	outputDir := filepath.Dir(*outputPath)
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to create output directory %s: %v\n", outputDir, err)
		os.Exit(1)
	}

	// Create output file
	outputFile, err := os.Create(*outputPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to create output file %s: %v\n", *outputPath, err)
		os.Exit(1)
	}
	defer outputFile.Close()

	// Copy file contents
	if _, err := io.Copy(outputFile, sourceFile); err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to copy file: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("High-risk parameters default config copied from %s to %s\n", *sourcePath, *outputPath)
}

