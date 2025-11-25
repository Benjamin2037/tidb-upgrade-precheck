package runtime

import (
	"testing"
)

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