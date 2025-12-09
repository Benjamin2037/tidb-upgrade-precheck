// Package reporter provides report generation for analyzer results
// This file defines the interfaces for report sections, headers, and footers.
// The actual implementations are in pkg/reporter/formats/ directory.
package reporter

import (
	"github.com/pingcap/tidb-upgrade-precheck/pkg/analyzer"
)

// ReportSection represents a section in the report (e.g., parameter check, plan check)
// Each section is independent and can be rendered in different formats
// Implementations should be in pkg/reporter/formats/<format>/sections/
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
// Implementations should be in pkg/reporter/formats/<format>/header.go
type ReportHeader interface {
	// Render renders the header content
	Render(result *analyzer.AnalysisResult) (string, error)
}

// ReportFooter renders the footer of the report
// Note: Format-specific footers don't need Format parameter as they're already format-specific
// Implementations should be in pkg/reporter/formats/<format>/footer.go
type ReportFooter interface {
	// Render renders the footer content
	Render(result *analyzer.AnalysisResult) (string, error)
}
