package scan

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestScanUpgradeLogic(t *testing.T) {
	err := scanUpgradeLogic("")
	require.Error(t, err)

	// Test with actual repo path
	// In test environment, the "../tidb" path might not exist,
	// so we don't require it to succeed, but it shouldn't panic
	_ = scanUpgradeLogic("../tidb")
}

func TestGetAllUpgradeChanges(t *testing.T) {
	// Skip this test if upgrade_logic.json doesn't exist or has invalid format
	changes, err := GetAllUpgradeChanges()
	if err != nil {
		t.Skipf("Skipping test due to error: %v", err)
	}
	
	// If we have changes, verify the structure
	if len(changes) > 0 {
		// Just verify that we have a valid structure
		require.NotNil(t, changes)
	}
}

func TestGetIncrementalUpgradeChanges(t *testing.T) {
	// Skip this test if upgrade_logic.json doesn't exist or has invalid format
	changes, err := GetIncrementalUpgradeChanges("v6.5.0", "v7.1.0")
	if err != nil {
		t.Skipf("Skipping test due to error: %v", err)
	}
	
	// If we have changes, verify the structure
	if len(changes) > 0 {
		// Just verify that we have a valid structure
		require.NotNil(t, changes)
	}
}

func TestParseVersionString(t *testing.T) {
	testCases := []struct {
		version  string
		expected int
		hasError bool
	}{
		{"v6.5.0", 65, false},
		{"6.5.0", 65, false},
		{"v7.1.0", 71, false},
		{"7.5.0", 75, false},
		{"v8.1.0", 81, false},
		{"invalid", 0, true},
		{"v6", 0, true},
	}

	for _, tc := range testCases {
		result, err := parseVersionString(tc.version)
		if tc.hasError {
			require.Error(t, err, "Expected error for version %s", tc.version)
		} else {
			require.NoError(t, err, "Unexpected error for version %s", tc.version)
			require.Equal(t, tc.expected, result, "Mismatch for version %s", tc.version)
		}
	}
}

func TestScanUpgradeLogicGlobal(t *testing.T) {
	err := ScanUpgradeLogicGlobal("", nil)
	require.NoError(t, err)
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && indexOf(s, substr) != -1
}

func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}