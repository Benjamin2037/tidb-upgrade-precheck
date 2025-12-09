package runtime

import (
	"testing"
)

func TestCollector_Collect(t *testing.T) {
	endpoints := []string{"127.0.0.1:2379"}
	collector := NewCollector(endpoints)

	clusterState, err := collector.Collect()
	if err != nil {
		t.Fatalf("Failed to collect cluster state: %v", err)
	}

	if len(clusterState.Instances) == 0 {
		t.Error("Expected at least one instance in cluster state")
	}

	// Check that we have instances of each type
	hasTiDB := false
	hasPD := false
	hasTiKV := false

	for _, instance := range clusterState.Instances {
		switch instance.State.Type {
		case TiDBComponent:
			hasTiDB = true
		case PDComponent:
			hasPD = true
		case TiKVComponent:
			hasTiKV = true
		}
	}

	if !hasTiDB {
		t.Error("Expected TiDB instance in cluster state")
	}

	if !hasPD {
		t.Error("Expected PD instance in cluster state")
	}

	if !hasTiKV {
		t.Error("Expected TiKV instance in cluster state")
	}
}

func TestCollector_CollectFromInstance(t *testing.T) {
	endpoints := []string{"127.0.0.1:2379"}
	collector := NewCollector(endpoints)

	// Test collecting from TiDB instance
	tidbInstance, err := collector.CollectFromInstance("127.0.0.1:4000", TiDBComponent)
	if err != nil {
		t.Fatalf("Failed to collect from TiDB instance: %v", err)
	}

	if tidbInstance.State.Type != TiDBComponent {
		t.Errorf("Expected TiDB component type, got %s", tidbInstance.State.Type)
	}

	// Test collecting from PD instance
	pdInstance, err := collector.CollectFromInstance("127.0.0.1:2379", PDComponent)
	if err != nil {
		t.Fatalf("Failed to collect from PD instance: %v", err)
	}

	if pdInstance.State.Type != PDComponent {
		t.Errorf("Expected PD component type, got %s", pdInstance.State.Type)
	}

	// Test collecting from TiKV instance
	tikvInstance, err := collector.CollectFromInstance("127.0.0.1:20160", TiKVComponent)
	if err != nil {
		t.Fatalf("Failed to collect from TiKV instance: %v", err)
	}

	if tikvInstance.State.Type != TiKVComponent {
		t.Errorf("Expected TiKV component type, got %s", tikvInstance.State.Type)
	}
}

