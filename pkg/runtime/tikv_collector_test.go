package runtime

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestTiKVCollector(t *testing.T) {
	// Create a test server to simulate TiKV API
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
	defer server.Close()

	collector := NewTiKVCollector()
	
	// Test collecting from a single instance
	states, err := collector.Collect([]string{server.Listener.Addr().String()})
	if err != nil {
		t.Fatalf("Failed to collect from TiKV: %v", err)
	}
	
	if len(states) != 1 {
		t.Fatalf("Expected 1 state, got %d", len(states))
	}
	
	state := states[0]
	if state.Type != "tikv" {
		t.Errorf("Expected type 'tikv', got %s", state.Type)
	}
	
	if state.Version != "v6.5.0" {
		t.Errorf("Expected version 'v6.5.0', got %s", state.Version)
	}
	
	if len(state.Config) == 0 {
		t.Error("Expected config to be populated")
	}
}