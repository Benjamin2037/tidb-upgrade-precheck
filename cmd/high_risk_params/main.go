package main

import (
	"fmt"
	"os"
)

func main() {
	rootCmd := newHighRiskParamsCmd()
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
