package main

import (
	"encoding/json"
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run collect_pd_simple.go <version>")
		os.Exit(1)
	}

	version := os.Args[1]
	
	// Collect default parameters
	defaults := collectPDDefaults()
	
	// Create output structure
	output := map[string]interface{}{
		"version": version,
		"config_defaults": defaults,
	}
	
	// Output as JSON
	data, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		fmt.Printf("Error marshaling JSON: %v\n", err)
		os.Exit(1)
	}
	
	// Write to file
	filename := fmt.Sprintf("pd_defaults_%s.json", version)
	err = os.WriteFile(filename, data, 0644)
	if err != nil {
		fmt.Printf("Error writing to file: %v\n", err)
		os.Exit(1)
	}
	
	fmt.Printf("PD defaults for version %s written to %s\n", version, filename)
}

func collectPDDefaults() map[string]interface{} {
	defaults := make(map[string]interface{})
	
	// Schedule config defaults
	defaults["schedule.max-snapshot-count"] = 64
	defaults["schedule.max-pending-peer-count"] = 64
	defaults["schedule.max-merge-region-size"] = 54
	defaults["schedule.split-merge-interval"] = "1h0m0s"
	defaults["schedule.max-store-down-time"] = "30m0s"
	defaults["schedule.leader-schedule-limit"] = 4
	defaults["schedule.region-schedule-limit"] = 2048
	defaults["schedule.replica-schedule-limit"] = 64
	defaults["schedule.merge-schedule-limit"] = 8
	defaults["schedule.hot-region-schedule-limit"] = 4
	defaults["schedule.tolerant-size-ratio"] = 0.0
	defaults["schedule.low-space-ratio"] = 0.8
	defaults["schedule.high-space-ratio"] = 0.7
	defaults["schedule.enable-joint-consensus"] = true
	defaults["schedule.enable-tikv-split-region"] = true
	defaults["schedule.enable-cross-table-merge"] = true
	defaults["schedule.enable-diagnostic"] = true
	defaults["schedule.enable-witness"] = false
	
	// Replication config defaults
	defaults["replication.max-replicas"] = 3
	defaults["replication.location-labels"] = []string{}
	defaults["replication.strictly-match-label"] = false
	defaults["replication.enable-placement-rules"] = true
	defaults["replication.enable-placement-rules-cache"] = false
	
	// Other important parameters
	defaults["log.level"] = "info"
	defaults["lease"] = 5
	defaults["quota-backend-bytes"] = 8589934592 // 8GB
	
	return defaults
}