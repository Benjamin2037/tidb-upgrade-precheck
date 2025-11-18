package rules

import (
	"context"
	"testing"

	"github.com/pingcap/tidb-upgrade-precheck/pkg/metadata"
	"github.com/pingcap/tidb-upgrade-precheck/pkg/precheck"
	"github.com/stretchr/testify/require"
)

func TestConfiguredRuleReportsCustomizedValues(t *testing.T) {
	catalog := mustLoadCatalog(t, `{
        "versions": [
            {
                "version": 100,
                "changes": [
                    {
                        "kind": "sysvar",
                        "target": "tidb_allow_something",
                        "default_value": "ON",
                        "scope": "GLOBAL",
                        "summary": "summary",
                        "details": "details"
                    }
                ]
            }
        ]
    }`)

	bootstrapLookup = func(version string) (int64, bool, error) {
		return 100, true, nil
	}
	t.Cleanup(func() { bootstrapLookup = knowledgeBootstrap })

	rule := NewConfiguredGlobalSysvarsRule(catalog)
	snapshot := precheck.Snapshot{
		SourceVersion: "v6.5.0",
		GlobalSysVars: map[string]string{
			"tidb_allow_something": "OFF",
		},
	}

	items, err := rule.Evaluate(context.Background(), snapshot)
	require.NoError(t, err)
	require.Len(t, items, 1)

	item := items[0]
	require.Equal(t, configuredSysvarRuleName, item.Rule)
	require.Equal(t, precheck.SeverityInfo, item.Severity)
	require.Contains(t, item.Message, "tidb_allow_something")
	require.Equal(t, "OFF", item.Metadata.(map[string]any)["current_value"])
	require.Equal(t, "ON", item.Metadata.(map[string]any)["default_value"])
	require.NotEmpty(t, item.Suggestions)
}

func TestConfiguredRuleIgnoresMatchingValues(t *testing.T) {
	catalog := mustLoadCatalog(t, `{"versions":[{"version": 200, "changes":[{"kind":"sysvar","target":"tidb_case","default_value":"1","scope":"global"}]}]}`)

	bootstrapLookup = func(version string) (int64, bool, error) {
		return 200, true, nil
	}
	t.Cleanup(func() { bootstrapLookup = knowledgeBootstrap })

	rule := NewConfiguredGlobalSysvarsRule(catalog)
	snapshot := precheck.Snapshot{
		SourceVersion: "v7.0.0",
		GlobalSysVars: map[string]string{
			"tidb_case": "1",
		},
	}

	items, err := rule.Evaluate(context.Background(), snapshot)
	require.NoError(t, err)
	require.Len(t, items, 0)
}

func mustLoadCatalog(t *testing.T, payload string) *metadata.Catalog {
	t.Helper()
	catalog, err := metadata.LoadCatalogFromBytes([]byte(payload))
	require.NoError(t, err)
	return catalog
}
