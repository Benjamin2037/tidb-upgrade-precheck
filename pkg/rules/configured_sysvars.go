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

// Evaluate compares the captured global sysvars with the expected defaults for the source/target bootstrap versions.
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

	targetVersion := strings.TrimSpace(snapshot.TargetVersion)
	if targetVersion == "" {
		return []precheck.ReportItem{newItem(configuredSysvarRuleName, precheck.SeverityInfo,
			"Target version is missing; unable to determine upgrade default changes",
			"Provide snapshot.TargetVersion so the checker can compare source and target defaults", nil)}, nil
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

	targetBootstrap, ok, err := bootstrapVersion(targetVersion)
	if err != nil {
		return nil, fmt.Errorf("load knowledge base: %w", err)
	}
	if !ok {
		return []precheck.ReportItem{newItem(configuredSysvarRuleName, precheck.SeverityInfo,
			fmt.Sprintf("No bootstrap version mapping found for target %s; skipping configured variable checks", targetVersion),
			"Update the knowledge base or specify a full version string", nil)}, nil
	}

	sourceBaseline := r.catalog.LatestGlobalSysvarValues(sourceBootstrap)
	targetBaseline := r.catalog.LatestGlobalSysvarValues(targetBootstrap)

	forcedChanges := r.catalog.ForcedSysvarChanges(sourceBootstrap, targetBootstrap)
	forcedLookup := make(map[string]metadata.Change, len(forcedChanges))
	for _, change := range forcedChanges {
		key := strings.ToLower(strings.TrimSpace(change.Target))
		if key == "" {
			continue
		}
		forcedLookup[key] = change
	}

	actuals := make(map[string]string, len(snapshot.GlobalSysVars))
	originalNames := make(map[string]string, len(snapshot.GlobalSysVars))
	for k, v := range snapshot.GlobalSysVars {
		key := strings.ToLower(strings.TrimSpace(k))
		if key == "" {
			continue
		}
		actuals[key] = strings.TrimSpace(v)
		if _, exists := originalNames[key]; !exists {
			originalNames[key] = strings.TrimSpace(k)
		}
	}

	if len(actuals) == 0 {
		return nil, nil
	}

	items := make([]precheck.ReportItem, 0)
	for key, actual := range actuals {
		sourceChange, hasSource := sourceBaseline[key]
		targetChange, hasTarget := targetBaseline[key]
		forcedChange, hasForced := forcedLookup[key]

		if !hasSource && !hasTarget && !hasForced {
			continue
		}

		displayName := selectFirstNonEmpty(
			originalNames[key],
			sourceChange.Target,
			targetChange.Target,
			forcedChange.Target,
		)
		if displayName == "" {
			displayName = key
		}

		sourceDefault := ""
		sourceVersionIntroduced := int64(0)
		if hasSource {
			sourceDefault = strings.TrimSpace(sourceChange.DefaultValue)
			sourceVersionIntroduced = sourceChange.ToVersion
		}

		targetDefault := ""
		targetVersionIntroduced := int64(0)
		if hasTarget {
			targetDefault = strings.TrimSpace(targetChange.DefaultValue)
			targetVersionIntroduced = targetChange.ToVersion
		}

		forcedValue := ""
		forcedVersion := int64(0)
		if hasForced {
			forcedValue = strings.TrimSpace(forcedChange.DefaultValue)
			forcedVersion = forcedChange.ToVersion
		}
		if forcedValue == "" && targetDefault != "" {
			forcedValue = targetDefault
		}

		userModified := false
		matchesSource := false
		if hasSource {
			if valuesEqual(sourceDefault, actual) {
				matchesSource = true
			} else {
				userModified = true
			}
		}

		changeWithinRange := false
		if hasTarget {
			changeWithinRange = targetVersionIntroduced > sourceBootstrap && (!hasSource || !valuesEqual(sourceDefault, targetDefault))
		}

		metadata := map[string]any{
			"target":           displayName,
			"current_value":    actual,
			"default_value":    sourceDefault,
			"source_version":   sourceVersion,
			"target_version":   targetVersion,
			"source_bootstrap": sourceBootstrap,
			"target_bootstrap": targetBootstrap,
		}
		if hasSource {
			metadata["baseline_version"] = sourceVersionIntroduced
			if scope := strings.TrimSpace(sourceChange.Scope); scope != "" {
				metadata["scope"] = scope
			}
		}
		if hasTarget {
			metadata["target_default_value"] = targetDefault
			metadata["target_change_version"] = targetVersionIntroduced
			if _, ok := metadata["scope"]; !ok {
				if scope := strings.TrimSpace(targetChange.Scope); scope != "" {
					metadata["scope"] = scope
				}
			}
		}
		if hasForced {
			metadata["forced_value"] = forcedValue
			metadata["forced_version"] = forcedVersion
			if _, ok := metadata["scope"]; !ok {
				if scope := strings.TrimSpace(forcedChange.Scope); scope != "" {
					metadata["scope"] = scope
				}
			}
		}

		switch {
		case hasSource && userModified && hasForced:
			message := fmt.Sprintf("Global variable %s is configured as %q, diverging from the source data version %d default %q. Upgrading to data version %d forces TiDB to apply %q at data version %d, overriding the customized setting.",
				displayName, actual, sourceBootstrap, sourceDefault, targetBootstrap, forcedValue, forcedVersion)
			details := forcedChange.Details
			if details == "" {
				details = forcedChange.Summary
			}
			suggestion := "Plan how to handle the forced value after upgrade if the customized setting is still required"
			item := precheck.ReportItem{
				Rule:        configuredSysvarRuleName,
				Severity:    precheck.SeverityWarning,
				Message:     message,
				Metadata:    metadata,
				Suggestions: []string{suggestion},
			}
			if details != "" {
				item.Details = []string{details}
			}
			items = append(items, item)
			continue

		case hasSource && userModified:
			message := fmt.Sprintf("Global variable %s is configured as %q, which differs from the source data version %d default %q.",
				displayName, actual, sourceBootstrap, sourceDefault)
			if changeWithinRange && hasTarget {
				message += fmt.Sprintf(" Target data version %d updates the default to %q (introduced at data version %d).", targetBootstrap, targetDefault, targetVersionIntroduced)
			} else if !changeWithinRange {
				message += fmt.Sprintf(" Target data version %d keeps the same default.", targetBootstrap)
			}
			suggestion := "Confirm whether this customized value still satisfies business requirements"
			item := precheck.ReportItem{
				Rule:        configuredSysvarRuleName,
				Severity:    precheck.SeverityInfo,
				Message:     message,
				Metadata:    metadata,
				Suggestions: []string{suggestion},
			}
			items = append(items, item)
			continue

		case hasSource && matchesSource && changeWithinRange && hasTarget:
			message := fmt.Sprintf("Global variable %s currently matches the source data version %d default %q, but upgrading to data version %d changes the default to %q (introduced at data version %d).",
				displayName, sourceBootstrap, sourceDefault, targetBootstrap, targetDefault, targetVersionIntroduced)
			details := targetChange.Details
			if details == "" {
				details = targetChange.Summary
			}
			suggestion := "Review the new default and pin a value before upgrading if the change is undesirable"
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

func selectFirstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}
