package text

import (
	"fmt"
	"strings"

	"github.com/pingcap/tidb-upgrade-precheck/pkg/analyzer"
	"github.com/pingcap/tidb-upgrade-precheck/pkg/reporter/formats"
	"github.com/pingcap/tidb-upgrade-precheck/pkg/reporter/sections"
)

// TextFormatter handles text format rendering
type TextFormatter struct {
	sections []formats.ReportSection
	header   formats.ReportHeader
	footer   formats.ReportFooter
}

// NewTextFormatter creates a new text formatter
func NewTextFormatter() *TextFormatter {
	return &TextFormatter{
		sections: []formats.ReportSection{
			sections.NewParameterCheckSection(),
			// Future: Add plan check section here
		},
		header: NewTextHeader(),
		footer: NewTextFooter(),
	}
}

// Generate generates a complete text format report
func (f *TextFormatter) Generate(result *analyzer.AnalysisResult, options *formats.Options) (string, error) {
	var content strings.Builder

	// Render header
	headerContent, err := f.header.Render(result)
	if err != nil {
		return "", fmt.Errorf("failed to render header: %w", err)
	}
	content.WriteString(headerContent)

	// Render sections (middle content)
	for _, section := range f.sections {
		if section.HasContent(result) {
			sectionContent, err := section.Render(formats.TextFormat, result)
			if err != nil {
				return "", fmt.Errorf("failed to render section %s: %w", section.Name(), err)
			}
			content.WriteString(sectionContent)
			content.WriteString("\n")
		}
	}

	// Render footer
	footerContent, err := f.footer.Render(result)
	if err != nil {
		return "", fmt.Errorf("failed to render footer: %w", err)
	}
	content.WriteString(footerContent)

	return content.String(), nil
}
