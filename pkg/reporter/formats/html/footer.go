package html

import (
	"strings"

	"github.com/pingcap/tidb-upgrade-precheck/pkg/analyzer"
)

// HTMLFooter renders the footer for HTML format
type HTMLFooter struct{}

// NewHTMLFooter creates a new HTML footer renderer
func NewHTMLFooter() *HTMLFooter {
	return &HTMLFooter{}
}

// Render renders the footer content
func (f *HTMLFooter) Render(result *analyzer.AnalysisResult) (string, error) {
	var content strings.Builder

	content.WriteString("</body>\n</html>\n")

	return content.String(), nil
}

