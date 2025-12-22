package sections

import (
	"fmt"
	"sort"
	"strings"

	"github.com/pingcap/tidb-upgrade-precheck/pkg/analyzer"
	"github.com/pingcap/tidb-upgrade-precheck/pkg/analyzer/rules"
	"github.com/pingcap/tidb-upgrade-precheck/pkg/reporter"
	"github.com/pingcap/tidb-upgrade-precheck/pkg/reporter/formats"
)

// ParameterCheckSection renders parameter check results
type ParameterCheckSection struct{}

// NewParameterCheckSection creates a new parameter check section
func NewParameterCheckSection() *ParameterCheckSection {
	return &ParameterCheckSection{}
}

// Name returns the section name
func (s *ParameterCheckSection) Name() string {
	return "Parameter Check"
}

// HasContent checks if this section has any content to render
func (s *ParameterCheckSection) HasContent(result *analyzer.AnalysisResult) bool {
	return len(result.CheckResults) > 0
}

// Render renders the section content
// Groups CheckResults by risk level (high, medium, low), then by component
func (s *ParameterCheckSection) Render(format formats.Format, result *analyzer.AnalysisResult) (string, error) {
	if len(result.CheckResults) == 0 {
		return "", nil
	}

	// Group CheckResults by risk level, then by component
	resultsByRiskLevel := make(map[formats.RiskLevel]map[string][]rules.CheckResult)
	for _, check := range result.CheckResults {
		// Skip if no parameter name (not a parameter check)
		if check.ParameterName == "" {
			continue
		}
		// Skip large configuration objects that bloat the report
		// These are internal config objects, not individual parameters users need to see
		if check.ParameterName == "tidb_config" {
			continue
		}
		// Skip statistics CheckResults (they are handled separately)
		if check.ParameterName == "__statistics__" {
			continue
		}

		// Filter path-related parameters at report generation time
		// This ensures all parameters are properly categorized before filtering
		if reporter.IsPathParameter(check.ParameterName) {
			continue
		}

		// Filter deployment-specific parameters (pd.endpoints, etc.)
		// These parameters vary by deployment environment and should not be reported
		// Check if parameter name matches any ignored parameter pattern
		if check.ParameterName == "pd.endpoints" || strings.HasSuffix(check.ParameterName, ".pd.endpoints") {
			continue
		}

		// Filter resource-dependent parameters at report generation time
		// These parameters are automatically adjusted by TiKV/TiFlash based on system resources
		// (CPU cores, memory, etc.) and should not be reported if source default == target default
		// but current differs (difference is due to deployment environment, not user modification)
		if reporter.IsResourceDependentParameter(check.ParameterName) {
			if check.SourceDefault != nil && check.TargetDefault != nil {
				sourceEqualsTarget := rules.CompareValues(check.SourceDefault, check.TargetDefault)
				if sourceEqualsTarget {
					// Source default == target default, but current differs
					// This is likely auto-tuned by TiKV/TiFlash based on system resources
					// Skip reporting as the difference is due to deployment environment
					continue
				}
			}
		}

		// Filter: If current value, source default, and target default are all the same, skip
		// No action is needed after upgrade, so no need to report
		if check.CurrentValue != nil && check.SourceDefault != nil && check.TargetDefault != nil {
			if rules.CompareValues(check.CurrentValue, check.SourceDefault) &&
				rules.CompareValues(check.CurrentValue, check.TargetDefault) {
				// All three values are the same, skip reporting
				continue
			}
		}

		// Filter: If source default is nil (N/A) but current value equals target default, skip
		// This is not a true "New" parameter that needs user action
		// The parameter already exists in cluster and matches target default, so no action needed
		if check.SourceDefault == nil && check.CurrentValue != nil && check.TargetDefault != nil {
			if rules.CompareValues(check.CurrentValue, check.TargetDefault) {
				// Current value equals target default, no action needed after upgrade
				continue
			}
		}

		riskLevel := check.RiskLevel
		if riskLevel == "" {
			// Fallback: determine from severity if risk level not set
			riskLevel = rules.GetRiskLevel(check.Severity)
		}
		component := check.Component
		if component == "" {
			component = "unknown"
		}

		if resultsByRiskLevel[riskLevel] == nil {
			resultsByRiskLevel[riskLevel] = make(map[string][]rules.CheckResult)
		}
		resultsByRiskLevel[riskLevel][component] = append(resultsByRiskLevel[riskLevel][component], check)
	}

	var content strings.Builder

	// Define order of risk levels
	riskLevelOrder := []formats.RiskLevel{
		formats.RiskLevelHigh,
		formats.RiskLevelMedium,
		formats.RiskLevelLow,
	}

	riskLevelTitles := map[formats.RiskLevel]string{
		formats.RiskLevelHigh:   "High Risk",
		formats.RiskLevelMedium: "Medium Risk",
		formats.RiskLevelLow:    "Low Risk",
	}

	riskLevelDescriptions := map[formats.RiskLevel]string{
		formats.RiskLevelHigh:   "⚠️  Critical issues that require immediate attention before upgrade.",
		formats.RiskLevelMedium: "⚠️  Warnings that should be reviewed before upgrade.",
		formats.RiskLevelLow:    "ℹ️  Informational items for awareness.",
	}

	// Define component order
	componentOrder := []string{"tidb", "pd", "tikv", "tiflash"}

	sectionNum := 1
	for _, riskLevel := range riskLevelOrder {
		byComponent := resultsByRiskLevel[riskLevel]
		if len(byComponent) == 0 {
			continue
		}

		content.WriteString(fmt.Sprintf("\n%d. %s\n", sectionNum, riskLevelTitles[riskLevel]))
		content.WriteString(fmt.Sprintf("   %s\n\n", riskLevelDescriptions[riskLevel]))
		sectionNum++

		// Display by component in order
		for _, compType := range componentOrder {
			compChecks := byComponent[compType]
			if len(compChecks) == 0 {
				continue
			}

			content.WriteString(fmt.Sprintf("   [%s Component]\n", strings.ToUpper(compType)))

			// Sort checks by parameter name
			sort.Slice(compChecks, func(i, j int) bool {
				return compChecks[i].ParameterName < compChecks[j].ParameterName
			})

			// Render each check result
			for _, check := range compChecks {
				paramType := check.ParamType
				if paramType == "" {
					paramType = "config"
				}

				// Determine report type for display
				reportType := formats.GetReportType(check)
				reportTypeLabel := ""
				switch reportType {
				case formats.ReportTypeForcedChange:
					reportTypeLabel = "[Forced]"
				case formats.ReportTypeUserModified:
					reportTypeLabel = "[Modified]"
				case formats.ReportTypeDefaultChanged:
					reportTypeLabel = "[Default Changed]"
				case formats.ReportTypeDeprecated:
					reportTypeLabel = "[Deprecated]"
				case formats.ReportTypeNewParameter:
					reportTypeLabel = "[New]"
				case formats.ReportTypeInconsistency:
					reportTypeLabel = "[Inconsistent]"
				case formats.ReportTypeHighRisk:
					reportTypeLabel = "[High Risk]"
				}

				// Format as checklist item
				content.WriteString(fmt.Sprintf("   - [%s] %s %s (%s)\n", check.Severity, reportTypeLabel, check.ParameterName, paramType))

				// If Details contains formatted content, use it; otherwise show individual values
				if check.Details != "" && strings.Contains(check.Details, "Current Value:") {
					// Details already contains formatted comparison, use it
					detailsLines := strings.Split(check.Details, "\n")
					for _, line := range detailsLines {
						if line != "" {
							content.WriteString(fmt.Sprintf("     %s\n", line))
						}
					}
				} else {
					// Fall back to individual value display
					if check.CurrentValue != nil {
						content.WriteString(fmt.Sprintf("     Current: %s\n", formatValueForDisplay(check.CurrentValue)))
					}
					if check.SourceDefault != nil {
						content.WriteString(fmt.Sprintf("     Source Default: %s\n", formatValueForDisplay(check.SourceDefault)))
					}
					if check.TargetDefault != nil {
						content.WriteString(fmt.Sprintf("     Target Default: %s\n", formatValueForDisplay(check.TargetDefault)))
					}
					if check.ForcedValue != nil {
						content.WriteString(fmt.Sprintf("     Forced To: %s\n", formatValueForDisplay(check.ForcedValue)))
					}
					if check.Details != "" {
						detailsLines := strings.Split(check.Details, "\n")
						for _, line := range detailsLines {
							if line != "" {
								content.WriteString(fmt.Sprintf("     %s\n", line))
							}
						}
					}
				}

				if check.Message != "" {
					content.WriteString(fmt.Sprintf("     Message: %s\n", check.Message))
				}
				if len(check.Suggestions) > 0 {
					content.WriteString("     Suggestions:\n")
					for _, suggestion := range check.Suggestions {
						content.WriteString(fmt.Sprintf("       - %s\n", suggestion))
					}
				}
				content.WriteString("\n")
			}
		}

		// Display unknown components if any
		for compType, compChecks := range byComponent {
			found := false
			for _, knownComp := range componentOrder {
				if compType == knownComp {
					found = true
					break
				}
			}
			if !found && len(compChecks) > 0 {
				content.WriteString(fmt.Sprintf("   [%s Component]\n", strings.ToUpper(compType)))
				for _, check := range compChecks {
					paramType := check.ParamType
					if paramType == "" {
						paramType = "config"
					}
					content.WriteString(fmt.Sprintf("   - [%s] %s (%s): %s\n", check.Severity, check.ParameterName, paramType, check.Message))
				}
			}
		}
	}

	return content.String(), nil
}

// formatValue formats a value for display
// Uses rules.FormatValue to properly handle scientific notation and numeric types
func formatValue(v interface{}) string {
	if v == nil {
		return "N/A"
	}
	return rules.FormatValue(v)
}

// formatValueForDisplay formats a value for clear display, handling complex types
func formatValueForDisplay(v interface{}) string {
	if v == nil {
		return "N/A"
	}

	// For complex types (maps, slices), try JSON formatting for readability
	valStr := fmt.Sprintf("%v", v)
	if len(valStr) > 200 {
		// For very long values, truncate and add ellipsis
		return valStr[:200] + "..."
	}
	return valStr
}
