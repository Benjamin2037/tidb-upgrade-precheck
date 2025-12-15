// Package pd provides tools for generating PD knowledge base from playground clusters
// This package collects runtime configuration from tiup playground clusters via HTTP API
package pd

import (
	"fmt"
	"strings"

	"github.com/pingcap/tidb-upgrade-precheck/pkg/types"
)

// Collect collects PD knowledge base from a tiup playground cluster
// This function collects default configuration directly from PD HTTP API:
// - /pd/api/v1/config/default: Returns complete default configuration
// Since PD provides a complete default config API, we directly use runtime collector.
func Collect(pdRoot, version string, pdAddr string) (*types.KBSnapshot, error) {
	// Collect default configuration via HTTP API using runtime collector
	// PD's /pd/api/v1/config/default endpoint provides complete default configuration
	fmt.Printf("Collecting PD default configuration from %s via HTTP API...\n", pdAddr)

	// Clean address (remove http:// prefix if present)
	cleanAddr := strings.TrimPrefix(pdAddr, "http://")
	cleanAddr = strings.TrimPrefix(cleanAddr, "https://")

	// Use runtime collector directly to get default values
	collector := NewPDCollector()
	state, err := collector.CollectDefaults([]string{cleanAddr})
	if err != nil {
		return nil, fmt.Errorf("failed to collect PD default config: %w", err)
	}

	snapshot := &types.KBSnapshot{
		Component:        types.ComponentPD,
		Version:          version,
		ConfigDefaults:   state.Config, // Direct assignment - types are compatible
		BootstrapVersion: 0,            // PD doesn't use bootstrap version for upgrade logic
	}

	return snapshot, nil
}
