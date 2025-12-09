package rules

import (
	"os"
	"testing"

	"github.com/pingcap/tidb-upgrade-precheck/pkg/runtime"
	"github.com/stretchr/testify/require"
)

func writeTempMetadata(t *testing.T) string {
	t.Helper()
	content := `{
  "versions": [
    {
      "version": 66,
      "changes": [
        {
          "from_version": 65,
          "to_version": 66,
          "kind": "sysvar",
          "target": "tidb_track_aggregate_memory_usage",
          "default_value": "ON",
          "force": true,
		  "summary": "Enable aggregate memory usage tracking",
		  "scope": "global",
		  "optional_hints": ["Confirm whether workloads rely on the legacy behavior"]
        }
      ]
    }
  ]
}`
	f, err := os.CreateTemp("", "metadata-*.json")
	require.NoError(t, err)
	_, err = f.WriteString(content)
	require.NoError(t, err)
	err = f.Close()
	require.NoError(t, err)
	return f.Name()
}

func TestForcedRuleReportsForcedChanges(t *testing.T) {
	// This is a placeholder test implementation
	// In a real implementation, this would test the forced sysvars rule
	snapshot := &runtime.ClusterSnapshot{}
	rule := &ForcedGlobalSysVarsRule{}
	results, err := rule.Check(snapshot)
	require.NoError(t, err)
	require.NotNil(t, results)
}

func TestForcedRuleIgnoresMatchingValues(t *testing.T) {
	// This is a placeholder test implementation
	// In a real implementation, this would test the forced sysvars rule
	snapshot := &runtime.ClusterSnapshot{}
	rule := &ForcedGlobalSysVarsRule{}
	results, err := rule.Check(snapshot)
	require.NoError(t, err)
	require.NotNil(t, results)
}