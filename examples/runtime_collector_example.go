package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/pingcap/tidb-upgrade-precheck/pkg/runtime"
)

func mainRuntimeCollector() {
	// Command line flags for cluster configuration
	tidbAddr := flag.String("tidb-addr", "127.0.0.1:4000", "TiDB server address")
	tikvAddrs := flag.String("tikv-addrs", "127.0.0.1:20180", "TiKV addresses (comma separated)")
	pdAddrs := flag.String("pd-addrs", "127.0.0.1:2379", "PD addresses (comma separated)")
	// outputFormat := flag.String("format", "json", "Output format: json")
	
	flag.Parse()

	// Create collector
	c := runtime.NewCollector()

	// Parse addresses from command line flags
	tikvAddrList := parseAddressesRuntime(*tikvAddrs)
	pdAddrList := parseAddressesRuntime(*pdAddrs)

	// Define cluster endpoints
	endpoints := runtime.ClusterEndpoints{
		TiDBAddr:  *tidbAddr,
		TiKVAddrs: tikvAddrList,
		PDAddrs:   pdAddrList,
	}

	// Collect cluster snapshot
	snapshot, err := c.Collect(endpoints)
	if err != nil {
		fmt.Printf("Error collecting cluster info: %v\n", err)
		os.Exit(1)
	}

	// Output as JSON
	data, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		fmt.Printf("Error marshaling snapshot: %v\n", err)
		os.Exit(1)
	}

	fmt.Println(string(data))
}

// parseAddressesRuntime parses a comma-separated list of addresses
func parseAddressesRuntime(addrStr string) []string {
	if addrStr == "" {
		return []string{}
	}
	
	// Split by comma and trim spaces
	addrs := strings.Split(addrStr, ",")
	for i, addr := range addrs {
		addrs[i] = strings.TrimSpace(addr)
	}
	
	return addrs
}