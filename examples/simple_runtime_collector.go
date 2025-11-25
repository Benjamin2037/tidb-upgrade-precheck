package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/pingcap/tidb-upgrade-precheck/pkg/runtime"
)

func main() {
	// Create collector
	collector := runtime.NewCollector()

	// Define cluster endpoints (using localhost as example)
	// In a real scenario, these would be the actual addresses of your TiDB cluster components
	endpoints := runtime.ClusterEndpoints{
		TiDBAddr:  "", // Empty to avoid connection attempts in this example
		TiKVAddrs: []string{}, // Empty to avoid connection attempts in this example
		PDAddrs:   []string{}, // Empty to avoid connection attempts in this example
	}

	// Collect cluster snapshot
	fmt.Println("Collecting cluster information...")
	snapshot, err := collector.Collect(endpoints)
	if err != nil {
		log.Printf("Warning: Error collecting cluster info: %v", err)
		// Continue anyway to show partial results
	}

	// Check that we got a snapshot
	if snapshot == nil {
		// Create an empty snapshot for demonstration
		snapshot = &runtime.ClusterSnapshot{
			Timestamp:  time.Now(),
			Components: make(map[string]runtime.ComponentState),
		}
		fmt.Println("Created empty snapshot for demonstration")
	}

	// Display results
	fmt.Printf("Timestamp: %v\n", snapshot.Timestamp)
	fmt.Printf("Number of components collected: %d\n", len(snapshot.Components))

	// Display component details
	for name, component := range snapshot.Components {
		fmt.Printf("\nComponent: %s\n", name)
		fmt.Printf("  Type: %s\n", component.Type)
		fmt.Printf("  Version: %s\n", component.Version)
		fmt.Printf("  Config items: %d\n", len(component.Config))
		fmt.Printf("  Variables: %d\n", len(component.Variables))
	}

	// Create a mock component for demonstration
	mockComponent := runtime.ComponentState{
		Type:      "tidb",
		Version:   "v6.5.0",
		Config:    map[string]interface{}{"performance.max-procs": 0, "log.level": "info"},
		Variables: map[string]string{"tidb_enable_clustered_index": "ON", "max_connections": "151"},
		Status:    make(map[string]interface{}),
	}
	
	snapshot.Components["tidb-mock"] = mockComponent
	fmt.Println("\nAdded mock TiDB component for demonstration")

	// Display results again with mock data
	fmt.Printf("\nUpdated results:\n")
	fmt.Printf("Timestamp: %v\n", snapshot.Timestamp)
	fmt.Printf("Number of components collected: %d\n", len(snapshot.Components))

	// Display component details
	for name, component := range snapshot.Components {
		fmt.Printf("\nComponent: %s\n", name)
		fmt.Printf("  Type: %s\n", component.Type)
		fmt.Printf("  Version: %s\n", component.Version)
		fmt.Printf("  Config items: %d\n", len(component.Config))
		fmt.Printf("  Variables: %d\n", len(component.Variables))
	}

	// Output full snapshot as JSON for detailed inspection
	fmt.Println("\nFull snapshot (JSON format):")
	data, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		log.Fatalf("Error marshaling snapshot: %v", err)
	}

	// Write to file
	err = os.WriteFile("cluster_snapshot.json", data, 0644)
	if err != nil {
		log.Fatalf("Error writing to file: %v", err)
	}

	fmt.Println("\nSnapshot written to cluster_snapshot.json")
}