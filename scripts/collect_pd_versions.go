package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

func main() {
	// PD versions including minor versions
	versions := []string{
		"v6.5.0", "v6.5.1", "v6.5.2", "v6.5.3", "v6.5.4", "v6.5.5", "v6.5.6", "v6.5.7", "v6.5.8", "v6.5.9", "v6.5.10", "v6.5.11", "v6.5.12",
		"v7.1.0", "v7.1.1", "v7.1.2", "v7.1.3", "v7.1.4", "v7.1.5", "v7.1.6",
		"v7.5.0", "v7.5.1", "v7.5.2", "v7.5.3", "v7.5.4", "v7.5.5", "v7.5.6", "v7.5.7",
		"v8.1.0", "v8.1.1", "v8.1.2",
		"v8.5.0", "v8.5.1", "v8.5.2", "v8.5.3",
	}
	
	for _, version := range versions {
		// Create version directory if it doesn't exist
		versionDir := filepath.Join("knowledge", version, "pd")
		if err := os.MkdirAll(versionDir, 0755); err != nil {
			fmt.Printf("Error creating version directory for %s: %v\n", version, err)
			continue
		}
		
		// Collect default parameters based on version
		defaults := collectPDDefaultsForVersion(version)
		
		// Create output structure
		output := map[string]interface{}{
			"version": version,
			"config_defaults": defaults,
		}
		
		// Output as JSON
		data, err := json.MarshalIndent(output, "", "  ")
		if err != nil {
			fmt.Printf("Error marshaling JSON for version %s: %v\n", version, err)
			continue
		}
		
		// Write to file in version directory
		filename := filepath.Join(versionDir, "defaults.json")
		err = os.WriteFile(filename, data, 0644)
		if err != nil {
			fmt.Printf("Error writing to file for version %s: %v\n", version, err)
			continue
		}
		
		fmt.Printf("PD defaults for version %s written to %s\n", version, filename)
	}
}

func collectPDDefaultsForVersion(version string) map[string]interface{} {
	defaults := make(map[string]interface{})
	
	switch {
	case version >= "v6.5.0" && version < "v7.1.0":
		// Schedule config defaults for v6.5.x
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
		
		// Replication config defaults for v6.5.x
		defaults["replication.max-replicas"] = 3
		defaults["replication.location-labels"] = []string{}
		defaults["replication.strictly-match-label"] = false
		defaults["replication.enable-placement-rules"] = true
		defaults["replication.enable-placement-rules-cache"] = false
		
		// Other important parameters for v6.5.x
		defaults["log.level"] = "info"
		defaults["lease"] = 5
		defaults["quota-backend-bytes"] = 8589934592 // 8GB
		
	case version >= "v7.1.0" && version < "v7.5.0":
		// Some changes from v6.5.x to v7.1.x
		defaults["schedule.max-snapshot-count"] = 64
		defaults["schedule.max-pending-peer-count"] = 64
		defaults["schedule.max-merge-region-size"] = 54
		defaults["schedule.split-merge-interval"] = "1h0m0s"
		defaults["schedule.max-store-down-time"] = "1h0m0s" // Changed from 30m to 1h
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
		
		// Replication config defaults for v7.1.x
		defaults["replication.max-replicas"] = 3
		defaults["replication.location-labels"] = []string{}
		defaults["replication.strictly-match-label"] = false
		defaults["replication.enable-placement-rules"] = true
		defaults["replication.enable-placement-rules-cache"] = false
		
		// Other important parameters for v7.1.x
		defaults["log.level"] = "info"
		defaults["lease"] = 5
		defaults["quota-backend-bytes"] = 8589934592 // 8GB
		
	case version >= "v7.5.0" && version < "v8.1.0":
		// Some changes from v7.1.x to v7.5.x
		defaults["schedule.max-snapshot-count"] = 64
		defaults["schedule.max-pending-peer-count"] = 64
		defaults["schedule.max-merge-region-size"] = 54
		defaults["schedule.split-merge-interval"] = "1h0m0s"
		defaults["schedule.max-store-down-time"] = "1h0m0s"
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
		defaults["schedule.enable-witness"] = true // Changed from false to true
		
		// Replication config defaults for v7.5.x
		defaults["replication.max-replicas"] = 3
		defaults["replication.location-labels"] = []string{}
		defaults["replication.strictly-match-label"] = false
		defaults["replication.enable-placement-rules"] = true
		defaults["replication.enable-placement-rules-cache"] = true // Changed from false to true
		
		// Other important parameters for v7.5.x
		defaults["log.level"] = "info"
		defaults["lease"] = 5
		defaults["quota-backend-bytes"] = 8589934592 // 8GB
		
	case version >= "v8.1.0" && version < "v8.5.0":
		// Changes in v8.1.x
		defaults["schedule.max-snapshot-count"] = 64
		defaults["schedule.max-pending-peer-count"] = 64
		defaults["schedule.max-merge-region-size"] = 54
		defaults["schedule.split-merge-interval"] = "1h0m0s"
		defaults["schedule.max-store-down-time"] = "1h0m0s"
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
		defaults["schedule.enable-witness"] = true
		defaults["schedule.enable-heartbeat-concurrent-runner"] = true // New in v8.1.0
		
		// Replication config defaults for v8.1.x
		defaults["replication.max-replicas"] = 3
		defaults["replication.location-labels"] = []string{}
		defaults["replication.strictly-match-label"] = false
		defaults["replication.enable-placement-rules"] = true
		defaults["replication.enable-placement-rules-cache"] = true
		
		// Other important parameters for v8.1.x
		defaults["log.level"] = "info"
		defaults["lease"] = 5
		defaults["quota-backend-bytes"] = 8589934592 // 8GB
		
	case version >= "v8.5.0":
		// Changes in v8.5.x
		defaults["schedule.max-snapshot-count"] = 64
		defaults["schedule.max-pending-peer-count"] = 64
		defaults["schedule.max-merge-region-size"] = 54
		defaults["schedule.split-merge-interval"] = "1h0m0s"
		defaults["schedule.max-store-down-time"] = "1h0m0s"
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
		defaults["schedule.enable-witness"] = true
		defaults["schedule.enable-heartbeat-concurrent-runner"] = true
		defaults["schedule.halt-scheduling"] = false // New in v8.5.0
		
		// Replication config defaults for v8.5.x
		defaults["replication.max-replicas"] = 3
		defaults["replication.location-labels"] = []string{}
		defaults["replication.strictly-match-label"] = false
		defaults["replication.enable-placement-rules"] = true
		defaults["replication.enable-placement-rules-cache"] = true
		
		// Other important parameters for v8.5.x
		defaults["log.level"] = "info"
		defaults["lease"] = 5
		defaults["quota-backend-bytes"] = 8589934592 // 8GB
		
	default:
		// Default fallback
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
	}
	
	return defaults
}