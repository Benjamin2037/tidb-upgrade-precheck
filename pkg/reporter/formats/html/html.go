package html

import (
	"fmt"
	"strings"

	"github.com/pingcap/tidb-upgrade-precheck/pkg/analyzer"
	"github.com/pingcap/tidb-upgrade-precheck/pkg/reporter/formats"
	"github.com/pingcap/tidb-upgrade-precheck/pkg/reporter/sections"
)

// HTMLFormatter handles HTML format rendering
type HTMLFormatter struct {
	sections []formats.ReportSection
	header   formats.ReportHeader
	footer   formats.ReportFooter
}

// NewHTMLFormatter creates a new HTML formatter
func NewHTMLFormatter() *HTMLFormatter {
	return &HTMLFormatter{
		sections: []formats.ReportSection{
			sections.NewParameterCheckSection(),
			// Future: Add plan check section here
		},
		header: NewHTMLHeader(),
		footer: NewHTMLFooter(),
	}
}

// Generate generates a complete HTML format report
func (f *HTMLFormatter) Generate(result *analyzer.AnalysisResult, options *formats.Options) (string, error) {
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
			sectionContent, err := section.Render(formats.HTMLFormat, result)
			if err != nil {
				return "", fmt.Errorf("failed to render section %s: %w", section.Name(), err)
			}
			content.WriteString(sectionContent)
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
