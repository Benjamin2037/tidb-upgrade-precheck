package formats

import (
	"github.com/pingcap/tidb-upgrade-precheck/pkg/analyzer"
)

// Format represents the output format
type Format string

const (
	TextFormat     Format = "text"
	MarkdownFormat Format = "markdown"
	HTMLFormat     Format = "html"
	JSONFormat     Format = "json"
)

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

