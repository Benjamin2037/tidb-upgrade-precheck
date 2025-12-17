package sections

import (
	"fmt"
	"sort"
	"strings"

	"github.com/pingcap/tidb-upgrade-precheck/pkg/analyzer"
	"github.com/pingcap/tidb-upgrade-precheck/pkg/analyzer/rules"
	"github.com/pingcap/tidb-upgrade-precheck/pkg/reporter/formats"
)

// ParameterCheckSection renders parameter check results in HTML format
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

// Render renders the section content in HTML format
// Groups CheckResults by risk level (high, medium, low), then by component
func (s *ParameterCheckSection) Render(format formats.Format, result *analyzer.AnalysisResult) (string, error) {
	if len(result.CheckResults) == 0 {
		return "", nil
	}

	// Separate deprecated parameters from other results
	var deprecatedResults []rules.CheckResult
	var otherResults []rules.CheckResult

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
		if rules.IsPathParameter(check.ParameterName) {
			continue
		}

		// Filter deployment-specific parameters (pd.endpoints, etc.)
		// These parameters vary by deployment environment and should not be reported
		// Check if parameter name matches any ignored parameter pattern
		if check.ParameterName == "pd.endpoints" || strings.HasSuffix(check.ParameterName, ".pd.endpoints") {
			continue
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

		// Check if this is a deprecated parameter
		reportType := formats.GetReportType(check)
		if reportType == formats.ReportTypeDeprecated {
			deprecatedResults = append(deprecatedResults, check)
		} else {
			otherResults = append(otherResults, check)
		}
	}

	// Group non-deprecated CheckResults by risk level, then by component
	resultsByRiskLevel := make(map[formats.RiskLevel]map[string][]rules.CheckResult)
	for _, check := range otherResults {
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

	// Add JavaScript functions for collapsible sections
	content.WriteString(`
<script>
function toggleSection(sectionId, buttonId) {
    var section = document.getElementById(sectionId);
    var button = document.getElementById(buttonId);
    if (section.style.display === 'none') {
        section.style.display = 'block';
        button.textContent = button.textContent.replace('▶', '▼').replace('Show', 'Hide');
    } else {
        section.style.display = 'none';
        button.textContent = button.textContent.replace('▼', '▶').replace('Hide', 'Show');
    }
}
</script>
`)

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
		formats.RiskLevelHigh:   "⚠️ <strong>Critical issues that require immediate attention before upgrade.</strong>",
		formats.RiskLevelMedium: "⚠️ <strong>Warnings that should be reviewed before upgrade.</strong>",
		formats.RiskLevelLow:    "ℹ️ <strong>Informational items for awareness.</strong>",
	}

	// Define which risk levels should be collapsible (collapsed by default)
	collapsibleRiskLevels := map[formats.RiskLevel]bool{
		formats.RiskLevelMedium: true,
		formats.RiskLevelLow:    true,
	}

	// Define component order
	componentOrder := []string{"tidb", "pd", "tikv", "tiflash"}

	sectionNum := 1
	for _, riskLevel := range riskLevelOrder {
		byComponent := resultsByRiskLevel[riskLevel]
		if len(byComponent) == 0 {
			continue
		}

		// Count total items for this risk level
		totalItems := 0
		for _, compChecks := range byComponent {
			totalItems += len(compChecks)
		}

		content.WriteString(fmt.Sprintf("<h2>%d. %s</h2>\n", sectionNum, riskLevelTitles[riskLevel]))
		content.WriteString(fmt.Sprintf("<p>%s</p>\n", riskLevelDescriptions[riskLevel]))
		sectionNum++

		// Add toggle button for collapsible risk levels
		if collapsibleRiskLevels[riskLevel] {
			sectionId := fmt.Sprintf("risk-section-%s", riskLevel)
			buttonId := fmt.Sprintf("risk-toggle-%s", riskLevel)
			content.WriteString(fmt.Sprintf(`
<button id="%s" onclick="toggleSection('%s', '%s')" style="padding: 8px 16px; margin: 10px 0; cursor: pointer; background-color: #f0f0f0; border: 1px solid #ccc; border-radius: 4px;">
▶ Show %s (%d items)
</button>
<div id="%s" style="display: none;">
`, buttonId, sectionId, buttonId, riskLevelTitles[riskLevel], totalItems, sectionId))
		}

		// Display by component in order
		for _, compType := range componentOrder {
			compChecks := byComponent[compType]
			if len(compChecks) == 0 {
				continue
			}

			content.WriteString(fmt.Sprintf("<h3>%s Component</h3>\n", strings.ToUpper(compType)))

			// Sort checks by parameter name
			sort.Slice(compChecks, func(i, j int) bool {
				return compChecks[i].ParameterName < compChecks[j].ParameterName
			})

			// Table header
			content.WriteString("<table>\n")
			content.WriteString("<tr><th>Parameter</th><th>Type</th><th>Current Value</th><th>Source Default</th><th>Target Default</th><th>Forced To</th><th>Severity</th><th>Message</th><th>Details</th></tr>\n")

			// Render each check result as a table row
			for _, check := range compChecks {
				paramType := check.ParamType
				if paramType == "" {
					paramType = "config"
				}
				severityClass := ""
				switch check.Severity {
				case "error", "critical":
					severityClass = "error"
				case "warning":
					severityClass = "warning"
				case "info":
					severityClass = "info"
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
					"<tr class=\"%s\"><td><code>%s</code><br/><small>%s</small></td><td>%s</td><td>%s</td><td>%s</td><td>%s</td><td>%s</td><td class=\"%s\">%s</td><td>%s</td><td>%s</td></tr>\n",
					severityClass, check.ParameterName, reportTypeLabel, paramType,
					currentFormatted, sourceFormatted, targetFormatted, forcedFormatted,
					severityClass, check.Severity, check.Message, check.Details))
			}

			content.WriteString("</table>\n")
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
				content.WriteString(fmt.Sprintf("<h3>%s Component</h3>\n", strings.ToUpper(compType)))
				content.WriteString("<table>\n")
				content.WriteString("<tr><th>Parameter</th><th>Type</th><th>Current Value</th><th>Severity</th><th>Message</th></tr>\n")
				for _, check := range compChecks {
					paramType := check.ParamType
					if paramType == "" {
						paramType = "config"
					}
					severityClass := ""
					switch check.Severity {
					case "error", "critical":
						severityClass = "error"
					case "warning":
						severityClass = "warning"
					case "info":
						severityClass = "info"
					}
					content.WriteString(fmt.Sprintf(
						"<tr class=\"%s\"><td><code>%s</code></td><td>%s</td><td>%v</td><td class=\"%s\">%s</td><td>%s</td></tr>\n",
						severityClass, check.ParameterName, paramType,
						formatValue(check.CurrentValue),
						severityClass, check.Severity, check.Message))
				}
				content.WriteString("</table>\n")
			}
		}

		// Close collapsible section div if this risk level is collapsible
		if collapsibleRiskLevels[riskLevel] {
			content.WriteString("</div>\n")
		}
	}

	// Add deprecated parameters section at the bottom (collapsible)
	if len(deprecatedResults) > 0 {
		// Group deprecated results by component
		deprecatedByComponent := make(map[string][]rules.CheckResult)
		for _, check := range deprecatedResults {
			component := check.Component
			if component == "" {
				component = "unknown"
			}
			deprecatedByComponent[component] = append(deprecatedByComponent[component], check)
		}

		// Add collapsible section for deprecated parameters
		content.WriteString(`
<script>
function toggleDeprecated() {
    var section = document.getElementById('deprecated-section');
    var button = document.getElementById('deprecated-toggle');
    if (section.style.display === 'none') {
        section.style.display = 'block';
        button.textContent = '▼ Hide Deprecated Parameters';
    } else {
        section.style.display = 'none';
        button.textContent = '▶ Show Deprecated Parameters';
    }
}
</script>
`)

		content.WriteString(fmt.Sprintf(`
<h2>🗑️ Deprecated Parameters</h2>
<p><strong>Note:</strong> The following parameters exist in the source version but will be removed in the target version. These are typically low-priority informational items.</p>
<button id="deprecated-toggle" onclick="toggleDeprecated()" style="padding: 8px 16px; margin: 10px 0; cursor: pointer; background-color: #f0f0f0; border: 1px solid #ccc; border-radius: 4px;">
▶ Show Deprecated Parameters (%d)
</button>
<div id="deprecated-section" style="display: none;">
`, len(deprecatedResults)))

		// Display deprecated parameters by component
		componentOrder := []string{"tidb", "pd", "tikv", "tiflash"}
		for _, compType := range componentOrder {
			compChecks := deprecatedByComponent[compType]
			if len(compChecks) == 0 {
				continue
			}

			content.WriteString(fmt.Sprintf("<h3>%s Component</h3>\n", strings.ToUpper(compType)))

			// Sort checks by parameter name
			sort.Slice(compChecks, func(i, j int) bool {
				return compChecks[i].ParameterName < compChecks[j].ParameterName
			})

			// Table header
			content.WriteString("<table>\n")
			content.WriteString("<tr><th>Parameter</th><th>Type</th><th>Current Value</th><th>Source Default</th><th>Severity</th><th>Message</th><th>Details</th></tr>\n")

			// Render each deprecated check result as a table row
			for _, check := range compChecks {
				paramType := check.ParamType
				if paramType == "" {
					paramType = "config"
				}
				severityClass := ""
				switch check.Severity {
				case "error", "critical":
					severityClass = "error"
				case "warning":
					severityClass = "warning"
				case "info":
					severityClass = "info"
				}

				// Format values
				currentFormatted := formatValue(check.CurrentValue)
				sourceFormatted := formatValue(check.SourceDefault)

				content.WriteString(fmt.Sprintf(
					"<tr class=\"%s\"><td><code>%s</code><br/><small>🗑️ Deprecated</small></td><td>%s</td><td>%s</td><td>%s</td><td class=\"%s\">%s</td><td>%s</td><td>%s</td></tr>\n",
					severityClass, check.ParameterName, paramType,
					currentFormatted, sourceFormatted,
					severityClass, check.Severity, check.Message, check.Details))
			}

			content.WriteString("</table>\n")
		}

		// Display unknown components if any
		for compType, compChecks := range deprecatedByComponent {
			found := false
			for _, knownComp := range componentOrder {
				if compType == knownComp {
					found = true
					break
				}
			}
			if !found && len(compChecks) > 0 {
				content.WriteString(fmt.Sprintf("<h3>%s Component</h3>\n", strings.ToUpper(compType)))
				content.WriteString("<table>\n")
				content.WriteString("<tr><th>Parameter</th><th>Type</th><th>Current Value</th><th>Source Default</th><th>Severity</th><th>Message</th><th>Details</th></tr>\n")
				for _, check := range compChecks {
					paramType := check.ParamType
					if paramType == "" {
						paramType = "config"
					}
					severityClass := ""
					switch check.Severity {
					case "error", "critical":
						severityClass = "error"
					case "warning":
						severityClass = "warning"
					case "info":
						severityClass = "info"
					}
					currentFormatted := formatValue(check.CurrentValue)
					sourceFormatted := formatValue(check.SourceDefault)
					content.WriteString(fmt.Sprintf(
						"<tr class=\"%s\"><td><code>%s</code><br/><small>🗑️ Deprecated</small></td><td>%s</td><td>%s</td><td>%s</td><td class=\"%s\">%s</td><td>%s</td><td>%s</td></tr>\n",
						severityClass, check.ParameterName, paramType,
						currentFormatted, sourceFormatted,
						severityClass, check.Severity, check.Message, check.Details))
				}
				content.WriteString("</table>\n")
			}
		}

		content.WriteString("</div>\n")
	}

	return content.String(), nil
}

// formatValue formats a value for display
// Uses rules.FormatValue to properly handle scientific notation and numeric types
func formatValue(v interface{}) string {
	if v == nil {
		return "<em>N/A</em>"
	}
	return rules.FormatValue(v)
}

// formatValueWithHighlight formats a value with highlighting for differences
// role: "current", "source", or "target"
func formatValueWithHighlight(value, sourceDefault, targetDefault interface{}, role string) string {
	if value == nil {
		return "<em>N/A</em>"
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
		return highlightCommaSeparatedListHTML(valueStr, sourceStr, targetStr, role)
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

// highlightCommaSeparatedListHTML highlights differences in comma-separated lists for HTML
func highlightCommaSeparatedListHTML(valueStr, sourceStr, targetStr, role string) string {
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
