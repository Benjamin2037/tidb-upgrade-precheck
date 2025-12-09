package text

import (
	"strings"

	"github.com/pingcap/tidb-upgrade-precheck/pkg/analyzer"
)

// TextFooter renders the footer for text format
type TextFooter struct{}

// NewTextFooter creates a new text footer renderer
func NewTextFooter() *TextFooter {
	return &TextFooter{}
}

// Render renders the footer content
func (f *TextFooter) Render(result *analyzer.AnalysisResult) (string, error) {
	var content strings.Builder

	content.WriteString("\n")
	content.WriteString("============================\n")
	content.WriteString("End of Report\n")
	content.WriteString("============================\n")

	return content.String(), nil
}
