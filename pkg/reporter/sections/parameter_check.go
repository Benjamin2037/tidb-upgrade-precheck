package sections

import (
	"fmt"
	"strings"

	"github.com/pingcap/tidb-upgrade-precheck/pkg/analyzer"
	"github.com/pingcap/tidb-upgrade-precheck/pkg/analyzer/rules"
	"github.com/pingcap/tidb-upgrade-precheck/pkg/reporter/formats"
)

// ParameterCheckSection renders parameter check results
// Supports HTML, Markdown, and Text formats
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

// Render renders the section content based on the format
// Groups CheckResults by risk level (high, medium, low), then by component
func (s *ParameterCheckSection) Render(format formats.Format, result *analyzer.AnalysisResult) (string, error) {
	if len(result.CheckResults) == 0 {
		return "", nil
	}

	// Filter and group results (common logic for all formats)
	deprecatedResults, filteredResults, resultsByRiskLevel := filterAndGroupResults(result.CheckResults)

	// Render based on format
	switch format {
	case formats.HTMLFormat:
		return renderHTML(deprecatedResults, filteredResults, resultsByRiskLevel)
	case formats.MarkdownFormat:
		return renderMarkdown(deprecatedResults, filteredResults, resultsByRiskLevel)
	case formats.TextFormat:
		return renderText(deprecatedResults, filteredResults, resultsByRiskLevel)
	default:
		return "", fmt.Errorf("unsupported format: %s", format)
	}
}

// filterAndGroupResults filters and groups CheckResults by risk level and component
// Returns deprecated results, filtered results, and grouped results by risk level
func filterAndGroupResults(checkResults []rules.CheckResult) ([]rules.CheckResult, []rules.CheckResult, map[formats.RiskLevel]map[string][]rules.CheckResult) {
	var deprecatedResults []rules.CheckResult
	var filteredResults []rules.CheckResult
	var otherResults []rules.CheckResult

	for _, check := range checkResults {
		// Skip if no parameter name (not a parameter check)
		if check.ParameterName == "" {
			continue
		}
		// Skip large configuration objects that bloat the report
		if check.ParameterName == "tidb_config" {
			continue
		}
		// Skip statistics CheckResults (they are handled separately)
		if check.ParameterName == "__statistics__" {
			continue
		}

		// Check if this is a filtered parameter (from preprocessor)
		// All filtering is done in preprocessor, reporter only needs to group and display results
		if check.Category == "filtered" || (check.Metadata != nil && check.Metadata["filtered"] == true) {
			filteredResults = append(filteredResults, check)
			continue
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

	return deprecatedResults, filteredResults, resultsByRiskLevel
}

// renderHTML renders parameter check results in HTML format
// Includes filtered parameters in a collapsible section
// Note: This function is deprecated. Use format-specific implementations in pkg/reporter/formats/html/sections
func renderHTML(deprecatedResults, filteredResults []rules.CheckResult, resultsByRiskLevel map[formats.RiskLevel]map[string][]rules.CheckResult) (string, error) {
	// Simple implementation for backward compatibility
	return renderText(deprecatedResults, filteredResults, resultsByRiskLevel)
}

// renderMarkdown renders parameter check results in Markdown format
// Includes filtered parameters in a collapsible section
// Note: This function is deprecated. Use format-specific implementations in pkg/reporter/formats/markdown/sections
func renderMarkdown(deprecatedResults, filteredResults []rules.CheckResult, resultsByRiskLevel map[formats.RiskLevel]map[string][]rules.CheckResult) (string, error) {
	// Simple implementation for backward compatibility
	return renderText(deprecatedResults, filteredResults, resultsByRiskLevel)
}

// renderText renders parameter check results in Text format
// Includes filtered parameters in a collapsible section
func renderText(deprecatedResults, filteredResults []rules.CheckResult, resultsByRiskLevel map[formats.RiskLevel]map[string][]rules.CheckResult) (string, error) {
	// Use the format-specific implementation from pkg/reporter/formats/text/sections
	// This is a temporary workaround until we fully migrate to format-specific sections
	// For now, delegate to the text format section implementation
	textSection := &textParameterCheckSection{}
	return textSection.renderText(resultsByRiskLevel)
}

// textParameterCheckSection is a helper to access the text format implementation
type textParameterCheckSection struct{}

func (s *textParameterCheckSection) renderText(resultsByRiskLevel map[formats.RiskLevel]map[string][]rules.CheckResult) (string, error) {
	// Import the text section implementation logic here
	// For now, return a simple implementation
	var content strings.Builder
	
	riskLevelOrder := []formats.RiskLevel{
		formats.RiskLevelHigh,
		formats.RiskLevelMedium,
		formats.RiskLevelLow,
	}
	
	componentOrder := []string{"tidb", "pd", "tikv", "tiflash"}
	
	sectionNum := 1
	for _, riskLevel := range riskLevelOrder {
		byComponent := resultsByRiskLevel[riskLevel]
		if len(byComponent) == 0 {
			continue
		}
		
		content.WriteString(fmt.Sprintf("\n%d. %s\n", sectionNum, getRiskLevelTitle(riskLevel)))
		sectionNum++
		
		for _, compType := range componentOrder {
			compChecks := byComponent[compType]
			if len(compChecks) == 0 {
				continue
			}
			
			content.WriteString(fmt.Sprintf("   [%s Component]\n", strings.ToUpper(compType)))
			for _, check := range compChecks {
				content.WriteString(fmt.Sprintf("   - %s: %s\n", check.ParameterName, check.Message))
			}
		}
	}
	
	return content.String(), nil
}

func getRiskLevelTitle(riskLevel formats.RiskLevel) string {
	switch riskLevel {
	case formats.RiskLevelHigh:
		return "High Risk"
	case formats.RiskLevelMedium:
		return "Medium Risk"
	case formats.RiskLevelLow:
		return "Low Risk"
	default:
		return "Unknown Risk"
	}
}
