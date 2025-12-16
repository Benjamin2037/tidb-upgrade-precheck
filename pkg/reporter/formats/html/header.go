package html

import (
	"html/template"
	"strings"
	"time"

	"github.com/pingcap/tidb-upgrade-precheck/pkg/analyzer"
)

// HTMLHeader renders the header for HTML format
type HTMLHeader struct{}

// NewHTMLHeader creates a new HTML header renderer
func NewHTMLHeader() *HTMLHeader {
	return &HTMLHeader{}
}

// Render renders the header content
func (h *HTMLHeader) Render(result *analyzer.AnalysisResult) (string, error) {
	const headerTemplate = `<!DOCTYPE html>
<html>
<head>
    <meta charset="utf-8">
    <title>TiDB Upgrade Precheck Report</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 40px; }
        h1, h2 { color: #333; }
        table { border-collapse: collapse; width: 100%; margin: 20px 0; }
        th, td { border: 1px solid #ddd; padding: 8px; text-align: left; }
        th { background-color: #f2f2f2; }
        code { background-color: #f8f8f8; padding: 2px 4px; }
        .warning { color: #f57c00; }
        .error { color: #d32f2f; }
        .info { color: #1976d2; }
    </style>
</head>
<body>
    <h1>TiDB Upgrade Precheck Report</h1>
    
    <p><strong>Source Version:</strong> {{.SourceVersion}}</p>
    <p><strong>Target Version:</strong> {{.TargetVersion}}</p>
    <p><strong>Generated At:</strong> {{.GeneratedAt}}</p>
    
    <h2>Summary</h2>
    <table>
        <tr><th>Category</th><th>Count</th></tr>
        <tr><td>Modified Parameters</td><td>{{.ModifiedCount}}</td></tr>
        <tr><td>TiKV Inconsistencies</td><td>{{.TikvInconsistencyCount}}</td></tr>
        <tr><td>Upgrade Differences</td><td>{{.UpgradeDiffCount}}</td></tr>
        <tr><td>Forced Changes</td><td>{{.ForcedChangeCount}}</td></tr>
        <tr><td>Focus Parameters</td><td>{{.FocusParamCount}}</td></tr>
        <tr><td>Check Results</td><td>{{.CheckResultCount}}</td></tr>
        {{if .TotalParametersCompared}}
        <tr><td>Parameters Compared</td><td>{{.TotalParametersCompared}}</td></tr>
        <tr><td>Parameters with Differences</td><td>{{.ParametersWithDifferences}}</td></tr>
        <tr><td>Parameters Skipped (source == target)</td><td>{{.ParametersSkipped}}</td></tr>
        <tr><td>Parameters Filtered (deployment-specific)</td><td>{{.ParametersFiltered}}</td></tr>
        {{end}}
    </table>`

	data := struct {
		SourceVersion             string
		TargetVersion             string
		GeneratedAt               string
		ModifiedCount             int
		TikvInconsistencyCount    int
		UpgradeDiffCount          int
		ForcedChangeCount         int
		FocusParamCount           int
		CheckResultCount          int
		TotalParametersCompared   int
		ParametersWithDifferences int
		ParametersSkipped         int
		ParametersFiltered        int
	}{
		SourceVersion:             result.SourceVersion,
		TargetVersion:             result.TargetVersion,
		GeneratedAt:               time.Now().Format("2006-01-02 15:04:05"),
		ModifiedCount:             countModifiedParams(result.ModifiedParams),
		TikvInconsistencyCount:    len(result.TikvInconsistencies),
		UpgradeDiffCount:          countUpgradeDifferences(result.UpgradeDifferences),
		ForcedChangeCount:         countForcedChanges(result.ForcedChanges),
		FocusParamCount:           countFocusParams(result.FocusParams),
		CheckResultCount:          len(result.CheckResults),
		TotalParametersCompared:   result.Statistics.TotalParametersCompared,
		ParametersWithDifferences: result.Statistics.ParametersWithDifferences,
		ParametersSkipped:         result.Statistics.ParametersSkipped,
		ParametersFiltered:        result.Statistics.ParametersFiltered,
	}

	tmpl, err := template.New("header").Parse(headerTemplate)
	if err != nil {
		return "", err
	}

	var buf strings.Builder
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}

	return buf.String(), nil
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
