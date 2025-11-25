package runtime

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCollector(t *testing.T) {
	// Create test servers to simulate cluster components
	tikvServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/status":
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"version": "v6.5.0"}`))
		case "/config":
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"server": {"grpc-concurrency": 5}}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer tikvServer.Close()

	pdServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/pd/api/v1/status":
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"version": "v6.5.0"}`))
		case "/pd/api/v1/config":
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"schedule": {"max-snapshot-count": 3}}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer pdServer.Close()

	// Create collector
	c := NewCollector()

	// Define cluster endpoints
	endpoints := ClusterEndpoints{
		TiDBAddr:  "", // Empty to avoid trying to connect
		TiKVAddrs: []string{tikvServer.Listener.Addr().String()},
		PDAddrs:   []string{pdServer.Listener.Addr().String()},
	}

	// Collect cluster snapshot
	snapshot, err := c.Collect(endpoints)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Check that we got a snapshot
	if snapshot == nil {
		t.Fatal("Expected snapshot, got nil")
	}

	// Check timestamp
	if snapshot.Timestamp.IsZero() {
		t.Error("Expected timestamp to be set")
	}

	// Check that we have at least some components
	if len(snapshot.Components) == 0 {
		t.Error("Expected at least some components")
	}

	// Check TiKV component
	if tikvComponent, exists := snapshot.Components["tikv-0"]; exists {
		if tikvComponent.Type != "tikv" {
			t.Errorf("Expected TiKV type to be 'tikv', got %s", tikvComponent.Type)
		}
		if len(tikvComponent.Config) == 0 {
			t.Error("Expected TiKV config to be populated")
		}
	} else {
		t.Error("Expected TiKV component")
	}

	// Check PD component
	if pdComponent, exists := snapshot.Components["pd"]; exists {
		if pdComponent.Type != "pd" {
			t.Errorf("Expected PD type to be 'pd', got %s", pdComponent.Type)
		}
		if len(pdComponent.Config) == 0 {
			t.Error("Expected PD config to be populated")
		}
	} else {
		t.Error("Expected PD component")
	}
}

func TestCollectorWithNoEndpoints(t *testing.T) {
	// Create collector
	c := NewCollector()

	// Define empty cluster endpoints
	endpoints := ClusterEndpoints{
		TiDBAddr:  "",
		TiKVAddrs: []string{},
		PDAddrs:   []string{},
	}

	// Collect cluster snapshot
	snapshot, err := c.Collect(endpoints)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Check that we got a snapshot
	if snapshot == nil {
		t.Fatal("Expected snapshot, got nil")
	}

	// Check that we have no components
	if len(snapshot.Components) != 0 {
		t.Errorf("Expected no components, got %d", len(snapshot.Components))
	}
}