package sections

import (
	"fmt"
	"sort"
	"strings"

	"github.com/pingcap/tidb-upgrade-precheck/pkg/analyzer"
	"github.com/pingcap/tidb-upgrade-precheck/pkg/analyzer/rules"
	"github.com/pingcap/tidb-upgrade-precheck/pkg/reporter/formats"
)

// ParameterCheckSection renders parameter check results in markdown format
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

// Render renders the section content in markdown format
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
		formats.RiskLevelHigh:   "⚠️  **Critical issues that require immediate attention before upgrade.**",
		formats.RiskLevelMedium: "⚠️  **Warnings that should be reviewed before upgrade.**",
		formats.RiskLevelLow:    "ℹ️  **Informational items for awareness.**",
	}

	// Define component order
	componentOrder := []string{"tidb", "pd", "tikv", "tiflash"}

	sectionNum := 1
	for _, riskLevel := range riskLevelOrder {
		byComponent := resultsByRiskLevel[riskLevel]
		if len(byComponent) == 0 {
			continue
		}

		content.WriteString(fmt.Sprintf("\n## %d. %s\n\n", sectionNum, riskLevelTitles[riskLevel]))
		content.WriteString(fmt.Sprintf("%s\n\n", riskLevelDescriptions[riskLevel]))
		sectionNum++

		// Display by component in order
		for _, compType := range componentOrder {
			compChecks := byComponent[compType]
			if len(compChecks) == 0 {
				continue
			}

			content.WriteString(fmt.Sprintf("### %s Component\n\n", strings.ToUpper(compType)))

			// Sort checks by parameter name
			sort.Slice(compChecks, func(i, j int) bool {
				return compChecks[i].ParameterName < compChecks[j].ParameterName
			})

			// Table header
			content.WriteString("| Parameter | Type | Current Value | Source Default | Target Default | Forced To | Severity | Message |\n")
			content.WriteString("|-----------|------|---------------|----------------|----------------|-----------|----------|----------|\n")

			// Render each check result as a table row
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
					reportTypeLabel = "🔴 Forced"
				case formats.ReportTypeUserModified:
					reportTypeLabel = "✏️ Modified"
				case formats.ReportTypeDefaultChanged:
					reportTypeLabel = "📝 Default Changed"
				case formats.ReportTypeDeprecated:
					reportTypeLabel = "🗑️ Deprecated"
				case formats.ReportTypeNewParameter:
					reportTypeLabel = "✨ New"
				case formats.ReportTypeInconsistency:
					reportTypeLabel = "⚠️ Inconsistent"
				case formats.ReportTypeHighRisk:
					reportTypeLabel = "🚨 High Risk"
				}

				// Format values with highlighting for differences
				currentFormatted := formatValueWithHighlight(check.CurrentValue, check.SourceDefault, check.TargetDefault, "current")
				sourceFormatted := formatValueWithHighlight(check.SourceDefault, check.SourceDefault, check.TargetDefault, "source")
				targetFormatted := formatValueWithHighlight(check.TargetDefault, check.SourceDefault, check.TargetDefault, "target")
				forcedFormatted := formatValue(check.ForcedValue)

				content.WriteString(fmt.Sprintf(
					"| `%s`<br/>%s | %s | %s | %s | %s | %s | %s | %s |\n",
					check.ParameterName, reportTypeLabel, paramType,
					currentFormatted, sourceFormatted, targetFormatted, forcedFormatted,
					check.Severity, check.Message))
			}

			content.WriteString("\n")
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
				content.WriteString(fmt.Sprintf("### %s Component\n\n", strings.ToUpper(compType)))
				content.WriteString("| Parameter | Type | Current Value | Severity | Message |\n")
				content.WriteString("|-----------|------|---------------|----------|----------|\n")
				for _, check := range compChecks {
					paramType := check.ParamType
					if paramType == "" {
						paramType = "config"
					}
					content.WriteString(fmt.Sprintf(
						"| `%s` | %s | %v | %s | %s |\n",
						check.ParameterName, paramType,
						formatValue(check.CurrentValue),
						check.Severity, check.Message))
				}
				content.WriteString("\n")
			}
		}
	}

	return content.String(), nil
}

// formatValue formats a value for display
// Large configuration objects (like tidb_config) are truncated to avoid bloating the report
func formatValue(v interface{}) string {
	if v == nil {
		return "N/A"
	}

	// Use rules.FormatValue to properly format values (handles scientific notation)
	// Then apply truncation logic for long values
	str := rules.FormatValue(v)

	// If the value is a large JSON object (like tidb_config), truncate it
	// Large objects typically start with "map[" or "{" and are very long
	if len(str) > 200 {
		// Check if it looks like a JSON object or map
		if (strings.HasPrefix(str, "map[") || strings.HasPrefix(str, "{")) && strings.Count(str, "\n") > 5 {
			// Truncate and add indicator
			lines := strings.Split(str, "\n")
			if len(lines) > 10 {
				// Show first 5 lines, then truncation indicator, then last 2 lines
				truncated := strings.Join(lines[:5], "\n")
				truncated += "\n... (truncated, " + fmt.Sprintf("%d", len(lines)-7) + " lines omitted) ...\n"
				truncated += strings.Join(lines[len(lines)-2:], "\n")
				return truncated
			}
		}
		// For other long strings, just truncate
		if len(str) > 500 {
			return str[:500] + "... (truncated)"
		}
	}

	return str
}

// formatValueWithHighlight formats a value with highlighting for differences
// role: "current", "source", or "target"
func formatValueWithHighlight(value, sourceDefault, targetDefault interface{}, role string) string {
	if value == nil {
		return "N/A"
	}

	// Use rules.FormatValue to properly format values (handles scientific notation)
	valueStr := rules.FormatValue(value)
	sourceStr := ""
	targetStr := ""

	if sourceDefault != nil {
		sourceStr = rules.FormatValue(sourceDefault)
	}
	if targetDefault != nil {
		targetStr = rules.FormatValue(targetDefault)
	}

	// If source and target are the same, no highlighting needed
	if sourceStr == targetStr {
		return formatValue(value)
	}

	// For comma-separated lists (like function lists), highlight differences
	if strings.Contains(valueStr, ",") && strings.Contains(sourceStr, ",") && strings.Contains(targetStr, ",") {
		return highlightCommaSeparatedList(valueStr, sourceStr, targetStr, role)
	}

	// For simple string differences, highlight the entire value if it differs
	if role == "current" {
		if valueStr != sourceStr && valueStr != targetStr {
			// Current differs from both, highlight in yellow
			return fmt.Sprintf("<span style=\"background-color: #fff3cd;\">%s</span>", escapeHTML(valueStr))
		}
	} else if role == "target" {
		if targetStr != sourceStr {
			// Target differs from source, highlight additions in green
			return fmt.Sprintf("<span style=\"background-color: #d4edda;\">%s</span>", escapeHTML(targetStr))
		}
	} else if role == "source" {
		if sourceStr != targetStr {
			// Source differs from target, highlight removals in red (lighter)
			return fmt.Sprintf("<span style=\"background-color: #f8d7da;\">%s</span>", escapeHTML(sourceStr))
		}
	}

	return formatValue(value)
}

// highlightCommaSeparatedList highlights differences in comma-separated lists
func highlightCommaSeparatedList(valueStr, sourceStr, targetStr, role string) string {
	valueItems := strings.Split(valueStr, ",")
	sourceItems := strings.Split(sourceStr, ",")
	targetItems := strings.Split(targetStr, ",")

	// Trim spaces
	for i := range valueItems {
		valueItems[i] = strings.TrimSpace(valueItems[i])
	}
	for i := range sourceItems {
		sourceItems[i] = strings.TrimSpace(sourceItems[i])
	}
	for i := range targetItems {
		targetItems[i] = strings.TrimSpace(targetItems[i])
	}

	// Create sets for comparison
	sourceSet := make(map[string]bool)
	for _, item := range sourceItems {
		sourceSet[item] = true
	}
	targetSet := make(map[string]bool)
	for _, item := range targetItems {
		targetSet[item] = true
	}

	var result []string
	for _, item := range valueItems {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}

		inSource := sourceSet[item]
		inTarget := targetSet[item]

		escapedItem := escapeHTML(item)

		if role == "current" {
			// For current value, show if it matches source or target
			if inSource && !inTarget {
				// Will be removed in target
				result = append(result, fmt.Sprintf("<span style=\"background-color: #f8d7da; text-decoration: line-through;\">%s</span>", escapedItem))
			} else if !inSource && inTarget {
				// Will be added in target
				result = append(result, fmt.Sprintf("<span style=\"background-color: #d4edda;\">%s</span>", escapedItem))
			} else {
				// Same in both
				result = append(result, escapedItem)
			}
		} else if role == "source" {
			// For source, highlight items that will be removed
			if inSource && !inTarget {
				result = append(result, fmt.Sprintf("<span style=\"background-color: #f8d7da; text-decoration: line-through;\">%s</span>", escapedItem))
			} else {
				result = append(result, escapedItem)
			}
		} else if role == "target" {
			// For target, highlight items that are new
			if !inSource && inTarget {
				result = append(result, fmt.Sprintf("<span style=\"background-color: #d4edda; font-weight: bold;\">%s</span>", escapedItem))
			} else {
				result = append(result, escapedItem)
			}
		}
	}

	return strings.Join(result, ", ")
}

// escapeHTML escapes HTML special characters
func escapeHTML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	s = strings.ReplaceAll(s, "'", "&#39;")
	return s
}
