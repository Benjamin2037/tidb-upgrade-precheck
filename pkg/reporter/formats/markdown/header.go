package markdown

import (
	"fmt"
	"strings"
	"time"

	"github.com/pingcap/tidb-upgrade-precheck/pkg/analyzer"
)

// MarkdownHeader renders the header for markdown format
type MarkdownHeader struct{}

// NewMarkdownHeader creates a new markdown header renderer
func NewMarkdownHeader() *MarkdownHeader {
	return &MarkdownHeader{}
}

// Render renders the header content
func (h *MarkdownHeader) Render(result *analyzer.AnalysisResult) (string, error) {
	var content strings.Builder

	content.WriteString("# TiDB Upgrade Precheck Report\n\n")
	content.WriteString(fmt.Sprintf("**Source Version:** %s  \n", result.SourceVersion))
	content.WriteString(fmt.Sprintf("**Target Version:** %s  \n", result.TargetVersion))
	content.WriteString(fmt.Sprintf("**Generated At:** %s\n\n", time.Now().Format("2006-01-02 15:04:05")))

	// Summary
	content.WriteString("## Summary\n\n")
	content.WriteString(fmt.Sprintf("- Modified Parameters: %d\n", countModifiedParams(result.ModifiedParams)))
	content.WriteString(fmt.Sprintf("- TiKV Inconsistencies: %d\n", len(result.TikvInconsistencies)))
	content.WriteString(fmt.Sprintf("- Upgrade Differences: %d\n", countUpgradeDifferences(result.UpgradeDifferences)))
	content.WriteString(fmt.Sprintf("- Forced Changes: %d\n", countForcedChanges(result.ForcedChanges)))
	content.WriteString(fmt.Sprintf("- Focus Parameters: %d\n", countFocusParams(result.FocusParams)))
	content.WriteString(fmt.Sprintf("- Check Results: %d\n\n", len(result.CheckResults)))

	return content.String(), nil
}

// Helper functions
func countModifiedParams(modifiedParams map[string]map[string]analyzer.ModifiedParamInfo) int {
	count := 0
	for _, params := range modifiedParams {
		count += len(params)
	}
	return count
}

func countUpgradeDifferences(differences map[string]map[string]analyzer.UpgradeDifference) int {
	count := 0
	for _, params := range differences {
		count += len(params)
	}
	return count
}

func countForcedChanges(forcedChanges map[string]map[string]analyzer.ForcedChange) int {
	count := 0
	for _, params := range forcedChanges {
		count += len(params)
	}
	return count
}

func countFocusParams(focusParams map[string]map[string]analyzer.FocusParamInfo) int {
	count := 0
	for _, params := range focusParams {
		count += len(params)
	}
	return count
}

