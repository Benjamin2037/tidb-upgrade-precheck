// Package tidb provides tools for generating TiDB knowledge base from playground clusters
// This package collects runtime configuration and system variables directly from tiup playground clusters
package tidb

import (
	"fmt"

	"github.com/pingcap/tidb-upgrade-precheck/pkg/types"
)

const (
	defaultTiDBHost = "127.0.0.1"
	defaultTiDBPort = 4000
	defaultTiDBUser = "root"
	defaultTiDBPass = ""
)

// Collect collects TiDB knowledge base from an existing tiup playground cluster
// This function assumes the playground cluster is already running and ready.
// Playground lifecycle (start/stop/wait) is managed by the caller (main.go).
// This function only:
// 1. Collects runtime configuration and system variables directly from the cluster via SHOW CONFIG and SHOW GLOBAL VARIABLES
// 2. Extracts bootstrap version from source code (needed for upgrade logic)
func Collect(tidbRoot, version, tag string) (*types.KBSnapshot, error) {
	if tag == "" {
		return nil, fmt.Errorf("tag is required: playground cluster must be started by caller")
	}

	// Collect runtime configuration and system variables from cluster
	// Since playground cluster provides complete default config and variables,
	// we directly use runtime collector without code extraction
	fmt.Printf("Collecting runtime configuration and system variables from cluster...\n")

	// Use runtime collector directly with connection info
	tidbCollector := NewTiDBCollector()
	addr := fmt.Sprintf("%s:%d", defaultTiDBHost, defaultTiDBPort)
	state, err := tidbCollector.Collect(addr, defaultTiDBUser, defaultTiDBPass)
	if err != nil {
		return nil, fmt.Errorf("failed to collect runtime configuration: %w", err)
	}

	// Extract bootstrap version from code (still needed for upgrade logic)
	// Note: We need to ensure TiDB repository is checked out to the correct version
	// The extractBootstrapVersion function will read from the repository, so it should
	// be called after the repository is in the correct state (or we need to checkout first)
	bootstrapVersion := extractBootstrapVersion(tidbRoot, version)
	if bootstrapVersion == 0 {
		fmt.Printf("Warning: Failed to extract bootstrap version for %s (returned 0). This may indicate the TiDB repository is not checked out to the correct version.\n", version)
	}

	snapshot := &types.KBSnapshot{
		Component:        types.ComponentTiDB,
		Version:          version,
		ConfigDefaults:   state.Config,    // Direct assignment - types are compatible
		SystemVariables:  state.Variables, // Direct assignment - types are compatible
		BootstrapVersion: bootstrapVersion,
	}

	return snapshot, nil
}
