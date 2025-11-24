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
	changes, err := GetAllUpgradeChanges()
	require.NoError(t, err)
	require.NotEmpty(t, changes)

	// Check that we have at least some expected changes
	foundGlobalVariablesChange := false
	for _, change := range changes {
		for _, ch := range change.Changes {
			if ch.SQL != "" && contains(ch.SQL, "mysql.global_variables") {
				foundGlobalVariablesChange = true
				break
			}
		}
		if foundGlobalVariablesChange {
			break
		}
	}
	require.True(t, foundGlobalVariablesChange, "Should find at least one mysql.global_variables change")
}

func TestGetIncrementalUpgradeChanges(t *testing.T) {
	// Test getting changes between v6.5.0 and v7.1.0
	// This should include some of the changes we documented
	changes, err := GetIncrementalUpgradeChanges("v6.5.0", "v7.1.0")
	require.NoError(t, err)
	
	// We expect some changes in this range
	// At minimum, we should have the changes from version 68 and 71
	require.NotEmpty(t, changes)
	
	// Verify versions are in the expected range
	for _, change := range changes {
		require.Greater(t, change.Version, 65)  // v6.5.0 -> 6*10 + 5 = 65
		require.LessOrEqual(t, change.Version, 71)  // v7.1.0 -> 7*10 + 1 = 71
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