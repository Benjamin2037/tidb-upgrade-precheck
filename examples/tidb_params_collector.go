package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/pingcap/tidb-upgrade-precheck/pkg/kbgenerator"
)

func mainTidbParamsCollector() {
	// Command line flags
	tidbRepo := flag.String("repo", "", "TiDB repository path")
	tag := flag.String("tag", "", "Git tag to collect parameters from")
	method := flag.String("method", "source", "Collection method: source or binary")
	toolPath := flag.String("tool", "", "Path to the export tool (required for binary method)")
	output := flag.String("output", "", "Output file path (default: stdout)")
	flag.Parse()

	if *tidbRepo == "" {
		fmt.Fprintln(os.Stderr, "Error: -repo is required")
		os.Exit(1)
	}

	if *tag == "" {
		fmt.Fprintln(os.Stderr, "Error: -tag is required")
		os.Exit(1)
	}

	var snapshot *kbgenerator.KBSnapshot
	var err error

	// Collect based on method
	if *method == "binary" {
		if *toolPath == "" {
			fmt.Fprintln(os.Stderr, "Error: -tool is required for binary method")
			os.Exit(1)
		}
		// Convert KBSnapshot to ParamSnapshot for backward compatibility in this example
		ks, err := kbgenerator.CollectFromTidbBinary(*tidbRepo, *tag, *toolPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error collecting parameters: %v\n", err)
			os.Exit(1)
		}
		snapshot = ks
	} else {
		ks, err := kbgenerator.CollectFromTidbSource(*tidbRepo, *tag)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error collecting parameters: %v\n", err)
			os.Exit(1)
		}
		snapshot = ks
	}

	// Output
	var writer *os.File
	if *output != "" {
		writer, err = os.Create(*output)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error creating output file: %v\n", err)
			os.Exit(1)
		}
		defer writer.Close()
	} else {
		writer = os.Stdout
	}

	encoder := json.NewEncoder(writer)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(snapshot); err != nil {
		fmt.Fprintf(os.Stderr, "Error encoding output: %v\n", err)
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "Successfully collected parameters for %s\n", *tag)
}