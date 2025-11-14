package rules

import (
	"context"
	"fmt"
	"strings"

	"github.com/pingcap/tidb-upgrade-precheck/pkg/precheck"
	"golang.org/x/mod/semver"
)

const targetVersionRuleName = "core.target-version-order"

// TargetVersionOrderRule ensures the target version is not lower than the source version.
type TargetVersionOrderRule struct{}

// NewTargetVersionOrderRule constructs the rule.
func NewTargetVersionOrderRule() precheck.Rule {
	return &TargetVersionOrderRule{}
}

// Name returns the rule name.
func (TargetVersionOrderRule) Name() string { return targetVersionRuleName }

// Evaluate checks whether the target version is ahead of the source version.
func (TargetVersionOrderRule) Evaluate(_ context.Context, snapshot precheck.Snapshot) ([]precheck.ReportItem, error) {
	source := strings.TrimSpace(snapshot.SourceVersion)
	target := strings.TrimSpace(snapshot.TargetVersion)
	if target == "" {
		return []precheck.ReportItem{newItem(targetVersionRuleName, precheck.SeverityWarning,
			"Target version is empty; unable to evaluate the upgrade path",
			"Provide a valid target version for snapshot.TargetVersion", nil)}, nil
	}

	if source == "" {
		return []precheck.ReportItem{newItem(targetVersionRuleName, precheck.SeverityInfo,
			"Source version is not provided; skipping version order validation", "", nil)}, nil
	}

	normalizedSource := ensureSemverPrefix(source)
	normalizedTarget := ensureSemverPrefix(target)

	if !semver.IsValid(normalizedSource) || !semver.IsValid(normalizedTarget) {
		msg := fmt.Sprintf("Unable to parse version numbers source=%q target=%q", source, target)
		return []precheck.ReportItem{newItem(targetVersionRuleName, precheck.SeverityWarning,
			msg, "Use semantic version strings such as v7.5.0", map[string]string{
				"source": source,
				"target": target,
			})}, nil
	}

	switch {
	case semver.Compare(normalizedTarget, normalizedSource) < 0:
		msg := fmt.Sprintf("Target version %s is lower than current version %s; this is unsupported", target, source)
		return []precheck.ReportItem{newItem(targetVersionRuleName, precheck.SeverityBlocker,
			msg, "Adjust the target version so it is not lower than the source", map[string]string{
				"source": source,
				"target": target,
			})}, nil
	case semver.Compare(normalizedTarget, normalizedSource) == 0:
		msg := fmt.Sprintf("Target version %s is identical to the current version; the upgrade would redeploy the same version", target)
		return []precheck.ReportItem{newItem(targetVersionRuleName, precheck.SeverityWarning,
			msg, "Confirm whether this is intended or choose a higher target version", map[string]string{
				"source": source,
				"target": target,
			})}, nil
	default:
		msg := fmt.Sprintf("Detected upgrade from %s to %s", source, target)
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
