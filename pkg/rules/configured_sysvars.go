package rules

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/pingcap/tidb-upgrade-precheck/knowledge"
	"github.com/pingcap/tidb-upgrade-precheck/pkg/metadata"
	"github.com/pingcap/tidb-upgrade-precheck/pkg/precheck"
)

const configuredSysvarRuleName = "core.configured-global-sysvars"

// ConfiguredGlobalSysvarsRule reports global system variables whose current value diverges from the baseline value in the source version.
type ConfiguredGlobalSysvarsRule struct {
	catalog *metadata.Catalog
}

// NewConfiguredGlobalSysvarsRule constructs a rule that highlights user-configured global variables.
func NewConfiguredGlobalSysvarsRule(catalog *metadata.Catalog) precheck.Rule {
	if catalog == nil {
		return nil
	}
	return &ConfiguredGlobalSysvarsRule{catalog: catalog}
}

// Name returns the rule identifier.
func (r *ConfiguredGlobalSysvarsRule) Name() string { return configuredSysvarRuleName }

// Evaluate compares the captured global sysvars with the expected defaults for the source bootstrap version.
func (r *ConfiguredGlobalSysvarsRule) Evaluate(_ context.Context, snapshot precheck.Snapshot) ([]precheck.ReportItem, error) {
	if r.catalog == nil {
		return nil, nil
	}
	if len(snapshot.GlobalSysVars) == 0 {
		return nil, nil
	}

	sourceVersion := strings.TrimSpace(snapshot.SourceVersion)
	if sourceVersion == "" {
		return []precheck.ReportItem{newItem(configuredSysvarRuleName, precheck.SeverityInfo,
			"Source version is missing; unable to evaluate existing variable overrides",
			"Provide snapshot.SourceVersion to identify the baseline defaults", nil)}, nil
	}

	sourceBootstrap, ok, err := bootstrapVersion(sourceVersion)
	if err != nil {
		return nil, fmt.Errorf("load knowledge base: %w", err)
	}
	if !ok {
		return []precheck.ReportItem{newItem(configuredSysvarRuleName, precheck.SeverityInfo,
			fmt.Sprintf("No bootstrap version mapping found for source %s; skipping configured variable checks", sourceVersion),
			"Update the knowledge base or specify a full version string", nil)}, nil
	}

	baseline := r.catalog.LatestGlobalSysvarValues(sourceBootstrap)
	if len(baseline) == 0 {
		return nil, nil
	}

	actuals := make(map[string]string, len(snapshot.GlobalSysVars))
	for k, v := range snapshot.GlobalSysVars {
		actuals[strings.ToLower(strings.TrimSpace(k))] = strings.TrimSpace(v)
	}

	items := make([]precheck.ReportItem, 0)
	for key, change := range baseline {
		if change.Force {
			// Forced changes are handled by the dedicated forced rule which already reports the current value.
			continue
		}
		current, ok := actuals[key]
		if !ok {
			continue
		}
		expected := strings.TrimSpace(change.DefaultValue)
		if valuesEqual(expected, current) {
			continue
		}

		message := fmt.Sprintf("Global variable %s is configured as %q, which differs from the bootstrap %d default %q", change.Target, current, change.ToVersion, expected)
		details := change.Details
		if details == "" {
			details = change.Summary
		}

		metadata := map[string]any{
			"target":           change.Target,
			"current_value":    current,
			"default_value":    expected,
			"baseline_value":   expected,
			"baseline_version": change.ToVersion,
			"summary":          change.Summary,
			"details":          change.Details,
		}

		suggestion := "Confirm whether this customized value still satisfies business requirements"
		item := precheck.ReportItem{
			Rule:        configuredSysvarRuleName,
			Severity:    precheck.SeverityInfo,
			Message:     message,
			Metadata:    metadata,
			Suggestions: []string{suggestion},
		}
		if details != "" {
			item.Details = []string{details}
		}
		items = append(items, item)
	}

	sort.Slice(items, func(i, j int) bool {
		left := strings.ToLower(items[i].Metadata.(map[string]any)["target"].(string))
		right := strings.ToLower(items[j].Metadata.(map[string]any)["target"].(string))
		return left < right
	})

	return items, nil
}

func valuesEqual(expected, current string) bool {
	if expected == current {
		return true
	}
	return strings.EqualFold(expected, current)
}

var bootstrapLookup = knowledgeBootstrap

func bootstrapVersion(version string) (int64, bool, error) {
	return bootstrapLookup(version)
}

func knowledgeBootstrap(version string) (int64, bool, error) {
	return knowledge.BootstrapVersion(version)
}
