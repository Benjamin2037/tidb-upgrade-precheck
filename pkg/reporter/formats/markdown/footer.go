package markdown

import (
	"strings"

	"github.com/pingcap/tidb-upgrade-precheck/pkg/analyzer"
)

// MarkdownFooter renders the footer for markdown format
type MarkdownFooter struct{}

// NewMarkdownFooter creates a new markdown footer renderer
func NewMarkdownFooter() *MarkdownFooter {
	return &MarkdownFooter{}
}

// Render renders the footer content
func (f *MarkdownFooter) Render(result *analyzer.AnalysisResult) (string, error) {
	var content strings.Builder

	content.WriteString("\n---\n")
	content.WriteString("*End of Report*\n")

	return content.String(), nil
}

