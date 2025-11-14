package rules

import (
	"context"
	"os"
	"testing"

	"github.com/pingcap/tidb-upgrade-precheck/pkg/metadata"
	"github.com/pingcap/tidb-upgrade-precheck/pkg/precheck"
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
          "optional_hints": ["确认业务是否依赖旧行为"]
        }
      ]
    },
    {
      "version": 70,
      "changes": [
        {
          "from_version": 69,
          "to_version": 70,
          "kind": "sysvar",
          "target": "tidb_some_feature",
          "default_value": "OFF",
          "force": false,
          "summary": "Non forced change",
          "scope": "global"
        }
      ]
    }
  ]
}`
	file, err := os.CreateTemp(t.TempDir(), "metadata-*.json")
	require.NoError(t, err)
	_, err = file.WriteString(content)
	require.NoError(t, err)
	require.NoError(t, file.Close())
	return file.Name()
}

func TestForcedGlobalSysvarsRule(t *testing.T) {
	metadataPath := writeTempMetadata(t)
	catalog, err := metadata.LoadCatalog(metadataPath)
	require.NoError(t, err)

	rule := NewForcedGlobalSysvarsRule(catalog)
	require.NotNil(t, rule)

	snapshot := precheck.Snapshot{
		SourceVersion: "",
		TargetVersion: "v6.5.0",
	}

	items, err := rule.Evaluate(context.Background(), snapshot)
	require.NoError(t, err)
	require.Len(t, items, 1)
	item := items[0]
	require.Equal(t, forcedSysvarRuleName, item.Rule)
	require.Equal(t, precheck.SeverityWarning, item.Severity)
	require.Contains(t, item.Message, "tidb_track_aggregate_memory_usage")
	require.NotEmpty(t, item.Suggestions)
	require.NotNil(t, item.Metadata)
}

func TestForcedGlobalSysvarsRuleMissingTarget(t *testing.T) {
	rule := NewForcedGlobalSysvarsRule(&metadata.Catalog{})
	snapshot := precheck.Snapshot{}
	items, err := rule.Evaluate(context.Background(), snapshot)
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, precheck.SeverityWarning, items[0].Severity)
}
