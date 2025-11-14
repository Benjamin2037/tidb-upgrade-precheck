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
			"目标版本缺失，无法评估强制全局变量变更",
			"为 snapshot.TargetVersion 提供正确的目标版本", nil)}, nil
	}

	targetBootstrap, ok, err := knowledge.BootstrapVersion(targetVersion)
	if err != nil {
		return nil, fmt.Errorf("加载知识库失败: %w", err)
	}
	if !ok {
		return []precheck.ReportItem{newItem(forcedSysvarRuleName, precheck.SeverityInfo,
			fmt.Sprintf("未找到目标版本 %s 对应的 bootstrap 版本，跳过全局变量检查", targetVersion),
			"更新知识库或指定完整的目标版本", nil)}, nil
	}

	var sourceBootstrap int64
	sourceVersion := strings.TrimSpace(snapshot.SourceVersion)
	if sourceVersion != "" {
		v, ok, err := knowledge.BootstrapVersion(sourceVersion)
		if err != nil {
			return nil, fmt.Errorf("加载知识库失败: %w", err)
		}
		if !ok {
			return []precheck.ReportItem{newItem(forcedSysvarRuleName, precheck.SeverityInfo,
				fmt.Sprintf("未找到当前版本 %s 对应的 bootstrap 版本，跳过全局变量检查", sourceVersion),
				"更新知识库或指定完整的当前版本", nil)}, nil
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
		message := fmt.Sprintf("升级到 bootstrap %d 时 TiDB 将强制把全局变量 %s 设置为 %q", change.ToVersion, change.Target, change.DefaultValue)
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
