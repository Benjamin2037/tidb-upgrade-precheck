package sections

import (
	"fmt"
	"sort"
	"strings"

	"github.com/pingcap/tidb-upgrade-precheck/pkg/analyzer"
	"github.com/pingcap/tidb-upgrade-precheck/pkg/analyzer/rules"
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

				content.WriteString(fmt.Sprintf("   - [%s] %s %s (%s):\n", check.Severity, reportTypeLabel, check.ParameterName, paramType))
				content.WriteString(fmt.Sprintf("     Current: %v\n", formatValue(check.CurrentValue)))
				if check.SourceDefault != nil {
					content.WriteString(fmt.Sprintf("     Source Default: %v\n", formatValue(check.SourceDefault)))
				}
				if check.TargetDefault != nil {
					content.WriteString(fmt.Sprintf("     Target Default: %v\n", formatValue(check.TargetDefault)))
				}
				if check.ForcedValue != nil {
					content.WriteString(fmt.Sprintf("     Forced To: %v\n", formatValue(check.ForcedValue)))
				}
				if check.Message != "" {
					content.WriteString(fmt.Sprintf("     Message: %s\n", check.Message))
				}
				if check.Details != "" {
					content.WriteString(fmt.Sprintf("     Details: %s\n", check.Details))
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
func formatValue(v interface{}) string {
	if v == nil {
		return "N/A"
	}
	return fmt.Sprintf("%v", v)
}
