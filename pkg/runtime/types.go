package runtime

import (
	"time"
)

// ClusterSnapshot represents the complete configuration state of a cluster
type ClusterSnapshot struct {
	Timestamp     time.Time              `json:"timestamp"`
	SourceVersion string                 `json:"source_version"`
	TargetVersion string                 `json:"target_version"`
	Components    map[string]ComponentState `json:"components"`
}

// ComponentState represents the configuration state of a single component
type ComponentState struct {
	Type      string                 `json:"type"`        // tidb, tikv, pd, tiflash
	Version   string                 `json:"version"`
	Config    map[string]interface{} `json:"config"`      // Configuration parameters
	Variables map[string]string      `json:"variables"`   // System variables (for TiDB)
	Status    map[string]interface{} `json:"status"`      // Runtime status information
}

// ClusterEndpoints contains connection information for cluster components
type ClusterEndpoints struct {
	TiDBAddr string   // MySQL protocol endpoint (host:port)
	TiKVAddrs []string // HTTP API endpoints
	PDAddrs   []string // HTTP API endpoints
}

// Collector defines the interface for collecting cluster information
type Collector interface {
	Collect(endpoints ClusterEndpoints) (*ClusterSnapshot, error)
}