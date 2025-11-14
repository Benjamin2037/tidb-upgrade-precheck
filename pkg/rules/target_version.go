package rules

import (
	"context"
	"fmt"
	"strings"

	"github.com/pingcap/tidb-upgrade-precheck/pkg/precheck"
	"golang.org/x/mod/semver"
)

const targetVersionRuleName = "core.target-version-order"

// TargetVersionOrderRule 确保目标版本不低于当前版本。
type TargetVersionOrderRule struct{}

// NewTargetVersionOrderRule 创建规则实例。
func NewTargetVersionOrderRule() precheck.Rule {
	return &TargetVersionOrderRule{}
}

// Name 返回规则名称。
func (TargetVersionOrderRule) Name() string { return targetVersionRuleName }

// Evaluate 检查目标版本是否高于当前版本。
func (TargetVersionOrderRule) Evaluate(_ context.Context, snapshot precheck.Snapshot) ([]precheck.ReportItem, error) {
	source := strings.TrimSpace(snapshot.SourceVersion)
	target := strings.TrimSpace(snapshot.TargetVersion)
	if target == "" {
		return []precheck.ReportItem{newItem(targetVersionRuleName, precheck.SeverityWarning,
			"目标版本为空，无法评估升级路径",
			"为 snapshot.TargetVersion 提供正确的目标版本", nil)}, nil
	}

	if source == "" {
		return []precheck.ReportItem{newItem(targetVersionRuleName, precheck.SeverityInfo,
			"未提供当前版本，跳过版本顺序校验", "", nil)}, nil
	}

	normalizedSource := ensureSemverPrefix(source)
	normalizedTarget := ensureSemverPrefix(target)

	if !semver.IsValid(normalizedSource) || !semver.IsValid(normalizedTarget) {
		msg := fmt.Sprintf("无法解析版本号 source=%q target=%q", source, target)
		return []precheck.ReportItem{newItem(targetVersionRuleName, precheck.SeverityWarning,
			msg, "确保使用语义化版本格式，例如 v7.5.0", map[string]string{
				"source": source,
				"target": target,
			})}, nil
	}

	switch {
	case semver.Compare(normalizedTarget, normalizedSource) < 0:
		msg := fmt.Sprintf("目标版本 %s 低于当前版本 %s，这是不被支持的", target, source)
		return []precheck.ReportItem{newItem(targetVersionRuleName, precheck.SeverityBlocker,
			msg, "调整目标版本，使其不低于当前版本", map[string]string{
				"source": source,
				"target": target,
			})}, nil
	case semver.Compare(normalizedTarget, normalizedSource) == 0:
		msg := fmt.Sprintf("目标版本 %s 与当前版本一致，将按原版本重新部署", target)
		return []precheck.ReportItem{newItem(targetVersionRuleName, precheck.SeverityWarning,
			msg, "确认这是否符合预期，必要时指定更高版本", map[string]string{
				"source": source,
				"target": target,
			})}, nil
	default:
		msg := fmt.Sprintf("检测到从 %s 升级到 %s", source, target)
		return []precheck.ReportItem{newItem(targetVersionRuleName, precheck.SeverityInfo,
			msg, "", map[string]string{
				"source": source,
				"target": target,
			})}, nil
	}
}

func ensureSemverPrefix(v string) string {
	if v == "" {
		return v
	}
	if strings.HasPrefix(v, "v") || strings.HasPrefix(v, "V") {
		return "v" + strings.TrimPrefix(strings.TrimPrefix(v, "v"), "V")
	}
	return "v" + v
}

func newItem(rule string, severity precheck.Severity, message string, suggestion string, metadata any) precheck.ReportItem {
	item := precheck.ReportItem{
		Rule:     rule,
		Severity: severity,
		Message:  message,
		Metadata: metadata,
	}
	if suggestion != "" {
		item.Suggestions = []string{suggestion}
	}
	return item
}
