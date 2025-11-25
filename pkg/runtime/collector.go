package runtime

import (
	"fmt"
	"time"
)

type collector struct {
	tidbCollector TiDBCollector
	tikvCollector TiKVCollector
	pdCollector   PDCollector
}

// NewCollector creates a new collector instance
func NewCollector() Collector {
	return &collector{
		tidbCollector: NewTiDBCollector(),
		tikvCollector: NewTiKVCollector(),
		pdCollector:   NewPDCollector(),
	}
}

// Collect gathers configuration information from all cluster components
func (c *collector) Collect(endpoints ClusterEndpoints) (*ClusterSnapshot, error) {
	snapshot := &ClusterSnapshot{
		Timestamp:  time.Now(),
		Components: make(map[string]ComponentState),
	}

	// Collect from TiDB
	if endpoints.TiDBAddr != "" {
		tidbState, err := c.tidbCollector.Collect(endpoints.TiDBAddr)
		if err != nil {
			return nil, fmt.Errorf("failed to collect TiDB info: %w", err)
		}
		snapshot.Components["tidb"] = *tidbState
	}

	// Collect from TiKV
	if len(endpoints.TiKVAddrs) > 0 {
		tikvStates, err := c.tikvCollector.Collect(endpoints.TiKVAddrs)
		if err != nil {
			return nil, fmt.Errorf("failed to collect TiKV info: %w", err)
		}
		for i, state := range tikvStates {
			snapshot.Components[fmt.Sprintf("tikv-%d", i)] = state
		}
	}

	// Collect from PD
	if len(endpoints.PDAddrs) > 0 {
		pdState, err := c.pdCollector.Collect(endpoints.PDAddrs)
		if err != nil {
			return nil, fmt.Errorf("failed to collect PD info: %w", err)
		}
		snapshot.Components["pd"] = *pdState
	}

	return snapshot, nil
}