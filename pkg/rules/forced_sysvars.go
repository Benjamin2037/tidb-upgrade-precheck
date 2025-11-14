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

const forcedSysvarRuleName = "core.forced-global-sysvars"

// ForcedGlobalSysvarsRule reports forced global system variable changes between two bootstrap versions.
type ForcedGlobalSysvarsRule struct {
	catalog *metadata.Catalog
}

// NewForcedGlobalSysvarsRule constructs a rule instance. If catalog is nil the rule is disabled.
func NewForcedGlobalSysvarsRule(catalog *metadata.Catalog) precheck.Rule {
	if catalog == nil {
		return nil
	}
	return &ForcedGlobalSysvarsRule{catalog: catalog}
}

// Name returns the rule identifier.
func (r *ForcedGlobalSysvarsRule) Name() string { return forcedSysvarRuleName }

// Evaluate detects all forced global sysvar changes for the provided snapshot.
func (r *ForcedGlobalSysvarsRule) Evaluate(_ context.Context, snapshot precheck.Snapshot) ([]precheck.ReportItem, error) {
	if r.catalog == nil {
		return nil, nil
	}

	targetVersion := strings.TrimSpace(snapshot.TargetVersion)
	if targetVersion == "" {
		return []precheck.ReportItem{newItem(forcedSysvarRuleName, precheck.SeverityWarning,
			"Target version is missing; unable to evaluate forced global variable changes",
			"Provide a valid target version for snapshot.TargetVersion", nil)}, nil
	}

	targetBootstrap, ok, err := knowledge.BootstrapVersion(targetVersion)
	if err != nil {
		return nil, fmt.Errorf("failed to load knowledge base: %w", err)
	}
	if !ok {
		return []precheck.ReportItem{newItem(forcedSysvarRuleName, precheck.SeverityInfo,
			fmt.Sprintf("No bootstrap version found for target %s; skipping global variable checks", targetVersion),
			"Update the knowledge base or specify a fully qualified target version", nil)}, nil
	}

	var sourceBootstrap int64
	sourceVersion := strings.TrimSpace(snapshot.SourceVersion)
	if sourceVersion != "" {
		v, ok, err := knowledge.BootstrapVersion(sourceVersion)
		if err != nil {
			return nil, fmt.Errorf("failed to load knowledge base: %w", err)
		}
		if !ok {
			return []precheck.ReportItem{newItem(forcedSysvarRuleName, precheck.SeverityInfo,
				fmt.Sprintf("No bootstrap version found for source %s; skipping global variable checks", sourceVersion),
				"Update the knowledge base or specify a fully qualified source version", nil)}, nil
		}
		sourceBootstrap = v
	}

	if sourceBootstrap >= targetBootstrap {
		return nil, nil
	}

	changes := r.catalog.ForcedSysvarChanges(sourceBootstrap, targetBootstrap)
	if len(changes) == 0 {
		return nil, nil
	}

	collapsed := collapseChanges(changes)
	items := make([]precheck.ReportItem, 0, len(collapsed))
	for _, change := range collapsed {
		message := fmt.Sprintf("Upgrading to bootstrap %d forces TiDB to set global variable %s to %q", change.ToVersion, change.Target, change.DefaultValue)
		metadata := map[string]any{
			"target":        change.Target,
			"default_value": change.DefaultValue,
			"to_version":    change.ToVersion,
			"summary":       change.Summary,
			"details":       change.Details,
			"force":         change.Force,
		}
		details := change.Details
		if details == "" {
			details = change.Summary
		}

		item := precheck.ReportItem{
			Rule:     forcedSysvarRuleName,
			Severity: precheck.SeverityWarning,
			Message:  message,
			Metadata: metadata,
		}
		if details != "" {
			item.Details = []string{details}
		}
		if len(change.OptionalHints) > 0 {
			item.Suggestions = append(item.Suggestions, change.OptionalHints...)
		}
		items = append(items, item)
	}

	sort.Slice(items, func(i, j int) bool {
		left := items[i].Metadata.(map[string]any)["to_version"].(int64)
		right := items[j].Metadata.(map[string]any)["to_version"].(int64)
		if left == right {
			return items[i].Message < items[j].Message
		}
		return left < right
	})

	return items, nil
}

func collapseChanges(changes []metadata.Change) []metadata.Change {
	if len(changes) == 0 {
		return nil
	}
	dedup := make(map[string]metadata.Change)
	for _, change := range changes {
		if change.Target == "" {
			continue
		}
		existing, ok := dedup[change.Target]
		if !ok || change.ToVersion > existing.ToVersion {
			dedup[change.Target] = change
		}
	}
	result := make([]metadata.Change, 0, len(dedup))
	for _, change := range dedup {
		result = append(result, change)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].ToVersion == result[j].ToVersion {
			return result[i].Target < result[j].Target
		}
		return result[i].ToVersion < result[j].ToVersion
	})
	return result
}
