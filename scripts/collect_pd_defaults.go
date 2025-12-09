package main

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/tikv/pd/pkg/utils/typeutil"
	scheduleconfig "github.com/tikv/pd/pkg/schedule/config"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run collect_pd_defaults.go <version>")
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
	defaults["schedule.max-snapshot-count"] = uint64(64)
	defaults["schedule.max-pending-peer-count"] = uint64(64)
	defaults["schedule.max-merge-region-size"] = uint64(54)
	defaults["schedule.split-merge-interval"] = typeutil.NewDuration(time.Hour)
	defaults["schedule.max-store-down-time"] = typeutil.NewDuration(30 * time.Minute)
	defaults["schedule.leader-schedule-limit"] = uint64(4)
	defaults["schedule.region-schedule-limit"] = uint64(2048)
	defaults["schedule.replica-schedule-limit"] = uint64(64)
	defaults["schedule.merge-schedule-limit"] = uint64(8)
	defaults["schedule.hot-region-schedule-limit"] = uint64(4)
	defaults["schedule.tolerant-size-ratio"] = float64(0)
	defaults["schedule.low-space-ratio"] = float64(0.8)
	defaults["schedule.high-space-ratio"] = float64(0.7)
	defaults["schedule.enable-joint-consensus"] = true
	defaults["schedule.enable-tikv-split-region"] = true
	
	// Replication config defaults
	defaults["replication.max-replicas"] = uint64(3)
	defaults["replication.location-labels"] = []string{}
	defaults["replication.strictly-match-label"] = false
	defaults["replication.enable-placement-rules"] = true
	
	// Other important parameters
	defaults["log.level"] = "info"
	defaults["lease"] = int64(5)
	defaults["quota-backend-bytes"] = int64(8 * 1024 * 1024 * 1024) // 8GB
	
	return defaults
}

// 查找PD版本升级中的强制变化参数
func findMandatoryChanges() map[string][]map[string]interface{} {
	changes := make(map[string][]map[string]interface{})
	
	// Example of mandatory changes between versions
	// These would normally be determined by examining release notes and source code
	
	// Changes from v6.5 to v7.0
	changes["v6.5->v7.0"] = []map[string]interface{}{
		{
			"name":        "schedule.max-store-down-time",
			"from":        "30m",
			"to":          "1h",
			"type":        "modified",
			"mandatory":   true,
			"description": "Increased default max store down time to reduce unnecessary replica operations",
		},
	}
	
	// Changes from v7.0 to v7.1
	changes["v7.0->v7.1"] = []map[string]interface{}{
		{
			"name":        "schedule.enable-joint-consensus",
			"from":        false,
			"to":          true,
			"type":        "modified",
			"mandatory":   true,
			"description": "Enabled joint consensus by default for better scheduling performance",
		},
	}
	
	return changes
}