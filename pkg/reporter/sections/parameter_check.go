package sections

import (
	"fmt"

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
func renderHTML(deprecatedResults, filteredResults []rules.CheckResult, resultsByRiskLevel map[formats.RiskLevel]map[string][]rules.CheckResult) (string, error) {
	// TODO: Implement HTML rendering with filtered parameters support
	// For now, return empty string to allow compilation
	return "", fmt.Errorf("HTML rendering not yet implemented in unified sections")
}

// renderMarkdown renders parameter check results in Markdown format
// Includes filtered parameters in a collapsible section
func renderMarkdown(deprecatedResults, filteredResults []rules.CheckResult, resultsByRiskLevel map[formats.RiskLevel]map[string][]rules.CheckResult) (string, error) {
	// TODO: Implement Markdown rendering with filtered parameters support
	// For now, return empty string to allow compilation
	return "", fmt.Errorf("Markdown rendering not yet implemented in unified sections")
}

// renderText renders parameter check results in Text format
// Includes filtered parameters in a collapsible section
func renderText(deprecatedResults, filteredResults []rules.CheckResult, resultsByRiskLevel map[formats.RiskLevel]map[string][]rules.CheckResult) (string, error) {
	// TODO: Implement Text rendering with filtered parameters support
	// For now, return empty string to allow compilation
	return "", fmt.Errorf("Text rendering not yet implemented in unified sections")
}
