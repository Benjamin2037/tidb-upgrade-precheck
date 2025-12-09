package runtime

import (
	"testing"
)

func TestTiDBCollector_Collect(t *testing.T) {
	// Create collector
	collector := &TiDBCollector{
		address: "127.0.0.1:4000",
	}

	// Test collection
	instanceState, err := collector.Collect()
	if err != nil {
		t.Fatalf("Failed to collect from TiDB: %v", err)
	}

	// Check basic properties
	if instanceState.Address != "127.0.0.1:4000" {
		t.Errorf("Expected address 127.0.0.1:4000, got %s", instanceState.Address)
	}

	if instanceState.State.Type != TiDBComponent {
		t.Errorf("Expected TiDB component type, got %s", instanceState.State.Type)
	}
}

func TestTiDBCollector_ConnectString(t *testing.T) {
	// Create collector
	collector := &TiDBCollector{
		address: "127.0.0.1:4000",
	}

	// Test connection string generation
	connectStr := collector.ConnectString("root", "")
	expected := "root:@tcp(127.0.0.1:4000)/information_schema"
	if connectStr != expected {
		t.Errorf("Expected connection string %s, got %s", expected, connectStr)
	}
}

func TestSplitHostPort(t *testing.T) {
	tests := []struct {
		name        string
		addr        string
		expectedHost string
		expectedPort string
		expectError  bool
	}{
		{
			name:         "IPv4 with port",
			addr:         "127.0.0.1:4000",
			expectedHost: "127.0.0.1",
			expectedPort: "4000",
			expectError:  false,
		},
		{
			name:         "Hostname with port",
			addr:         "localhost:4000",
			expectedHost: "localhost",
			expectedPort: "4000",
			expectError:  false,
		},
		{
			name:         "IPv4 without port",
			addr:         "127.0.0.1",
			expectedHost: "127.0.0.1",
			expectedPort: "4000",
			expectError:  false,
		},
		{
			name:         "IPv6 with port",
			addr:         "[::1]:4000",
			expectedHost: "::1",
			expectedPort: "4000",
			expectError:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			host, port, err := splitHostPort(tt.addr)
			
			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
				return
			}
			
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			
			if host != tt.expectedHost {
				t.Errorf("Expected host %s, got %s", tt.expectedHost, host)
			}
			
			if port != tt.expectedPort {
				t.Errorf("Expected port %s, got %s", tt.expectedPort, port)
			}
		})
	}
}