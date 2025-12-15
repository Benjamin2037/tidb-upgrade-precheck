package formats

import (
	"github.com/pingcap/tidb-upgrade-precheck/pkg/analyzer"
	rules "github.com/pingcap/tidb-upgrade-precheck/pkg/analyzer/rules"
)

// Format represents the output format
type Format string

const (
	TextFormat     Format = "text"
	MarkdownFormat Format = "markdown"
	HTMLFormat     Format = "html"
	JSONFormat     Format = "json"
)

// ReportType represents the type of parameter change
type ReportType string

const (
	// ReportTypeUserModified - User modified parameter (current != source default)
	ReportTypeUserModified ReportType = "user_modified"
	// ReportTypeForcedChange - Forced change during upgrade (in upgrade_logic.json)
	ReportTypeForcedChange ReportType = "forced_change"
	// ReportTypeDefaultChanged - Default value changed (target != source, not forced)
	ReportTypeDefaultChanged ReportType = "default_changed"
	// ReportTypeDeprecated - Parameter deprecated (exists in source, not in target)
	ReportTypeDeprecated ReportType = "deprecated"
	// ReportTypeNewParameter - New parameter (not in source, exists in target)
	ReportTypeNewParameter ReportType = "new_parameter"
	// ReportTypeInconsistency - Parameter inconsistency across nodes (TiKV)
	ReportTypeInconsistency ReportType = "inconsistency"
	// ReportTypeHighRisk - High-risk parameter check
	ReportTypeHighRisk ReportType = "high_risk"
)

// RiskLevel is re-exported from rules package for convenience
type RiskLevel = rules.RiskLevel

const (
	RiskLevelHigh   = rules.RiskLevelHigh
	RiskLevelMedium = rules.RiskLevelMedium
	RiskLevelLow    = rules.RiskLevelLow
)

// GetReportType determines the report type from a CheckResult
func GetReportType(check rules.CheckResult) ReportType {
	// Check for forced changes (has ForcedValue)
	if check.ForcedValue != nil {
		return ReportTypeForcedChange
	}

	// Check category to determine type
	switch check.Category {
	case "user_modified":
		return ReportTypeUserModified
	case "consistency":
		return ReportTypeInconsistency
	case "high_risk":
		return ReportTypeHighRisk
	case "upgrade_difference":
		// For upgrade_difference, check if it's a default change or deprecated/new
		if check.SourceDefault != nil && check.TargetDefault == nil {
			return ReportTypeDeprecated
		}
		if check.SourceDefault == nil && check.TargetDefault != nil {
			return ReportTypeNewParameter
		}
		if check.SourceDefault != nil && check.TargetDefault != nil {
			return ReportTypeDefaultChanged
		}
		return ReportTypeDefaultChanged
	default:
		// Default to info type based on severity
		if check.Severity == "info" {
			return ReportTypeDefaultChanged
		}
		return ReportTypeUserModified
	}
}

// Options represents report options
type Options struct {
	Format    Format
	OutputDir string
	Filename  string
}

// ReportSection represents a section in the report (e.g., parameter check, plan check)
// Each section is independent and can be rendered in different formats
type ReportSection interface {
	// Name returns the section name (e.g., "Parameter Check", "Plan Change Check")
	Name() string

	// Render renders the section content in the specified format
	// Returns the rendered content as a string
	Render(format Format, result *analyzer.AnalysisResult) (string, error)

	// HasContent checks if this section has any content to render
	HasContent(result *analyzer.AnalysisResult) bool
}

// ReportHeader renders the header of the report
// Note: Format-specific headers don't need Format parameter as they're already format-specific
type ReportHeader interface {
	// Render renders the header content
	Render(result *analyzer.AnalysisResult) (string, error)
}

// ReportFooter renders the footer of the report
// Note: Format-specific footers don't need Format parameter as they're already format-specific
type ReportFooter interface {
	// Render renders the footer content
	Render(result *analyzer.AnalysisResult) (string, error)
}
