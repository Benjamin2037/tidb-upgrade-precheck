package runtime

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPDCollector(t *testing.T) {
	// Create a test server to simulate PD API
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
	defer server.Close()

	collector := NewPDCollector()
	
	// Test collecting from a single instance
	state, err := collector.Collect([]string{server.Listener.Addr().String()})
	if err != nil {
		t.Fatalf("Failed to collect from PD: %v", err)
	}
	
	if state.Type != "pd" {
		t.Errorf("Expected type 'pd', got %s", state.Type)
	}
	
	if state.Version != "v6.5.0" {
		t.Errorf("Expected version 'v6.5.0', got %s", state.Version)
	}
	
	if len(state.Config) == 0 {
		t.Error("Expected config to be populated")
	}
}