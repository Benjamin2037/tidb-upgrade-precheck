package report

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"os"
	"strings"
	"time"

	"github.com/pingcap/tidb-upgrade-precheck/pkg/precheck"
)

// Format represents the output format of the report
type Format string

const (
	TextFormat     Format = "text"
	MarkdownFormat Format = "``"
	HTMLFormat     Format = "html"
	JSONFormat     Format = "json"
)

// Options defines options for report generation
type Options struct {
	Format    Format
	OutputDir string
	Filename  string
}

// GeneratedReport represents a generated report
type GeneratedReport struct {
	Path string
	Data string
}

// Generator generates reports in various formats
type Generator struct{}

// NewGenerator creates a new report generator
func NewGenerator() *Generator {
	return &Generator{}
}

// Generate generates a report based on the analysis results
func (g *Generator) Generate(report *precheck.Report, options *Options) (string, error) {
	// Create output directory if it doesn't exist
	if err := os.MkdirAll(options.OutputDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create output directory: %w", err)
	}

	// Generate filename if not provided
	filename := options.Filename
	if filename == "" {
		timestamp := time.Now().Format("20060102_150405")
		filename = fmt.Sprintf("upgrade_precheck_report_%s", timestamp)
	}

	var content string
	var err error

	switch options.Format {
	case TextFormat:
		content, err = g.generateTextReport(report, options)
	case MarkdownFormat:
		content, err = g.generateMarkdownReport(report, options)
	case HTMLFormat:
		content, err = g.generateHTMLReport(report, options)
	case JSONFormat:
		content, err = g.generateJSONReport(report, options)
	default:
		content, err = g.generateTextReport(report, options)
	}

	if err != nil {
		return "", fmt.Errorf("failed to generate report content: %w", err)
	}

	// Write to file
	filePath := fmt.Sprintf("%s/%s.%s", options.OutputDir, filename, getFileExtension(options.Format))
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		return "", fmt.Errorf("failed to write report to file: %w", err)
	}

	return filePath, nil
}

func getFileExtension(format Format) string {
	switch format {
	case TextFormat:
		return "txt"
	case MarkdownFormat:
		return "md"
	case HTMLFormat:
		return "html"
	case JSONFormat:
		return "json"
	default:
		return "txt"
	}
}

// generateTextReport generates a text format report
func (g *Generator) generateTextReport(report *precheck.Report, options *Options) (string, error) {
	var content strings.Builder
	
	content.WriteString("TiDB Upgrade Precheck Report\n")
	content.WriteString("============================\n\n")
	
	content.WriteString(fmt.Sprintf("Started At: %s\n", report.StartedAt.Format(time.RFC3339)))
	content.WriteString(fmt.Sprintf("Finished At: %s\n\n", report.FinishedAt.Format(time.RFC3339)))
	
	content.WriteString("Summary:\n")
	content.WriteString(fmt.Sprintf("  Total Items: %d\n", report.Summary.Total))
	content.WriteString(fmt.Sprintf("  Blocking Issues: %d\n", report.Summary.Blocking))
	content.WriteString(fmt.Sprintf("  Warnings: %d\n", report.Summary.Warnings))
	content.WriteString(fmt.Sprintf("  Info: %d\n\n", report.Summary.Infos))
	
	if len(report.Items) > 0 {
		content.WriteString("Issues:\n")
		for _, item := range report.Items {
			content.WriteString(fmt.Sprintf("  [%s] %s: %s\n", item.Severity, item.Rule, item.Message))
			if len(item.Suggestions) > 0 {
				content.WriteString(fmt.Sprintf("    Suggestion: %s\n", item.Suggestions[0]))
			}
		}
		content.WriteString("\n")
	}
	
	if len(report.Errors) > 0 {
		content.WriteString("Errors:\n")
		for _, err := range report.Errors {
			content.WriteString(fmt.Sprintf("  - %s\n", err))
		}
		content.WriteString("\n")
	}
	
	return content.String(), nil
}

// generateMarkdownReport generates a markdown format report
func (g *Generator) generateMarkdownReport(report *precheck.Report, options *Options) (string, error) {
	var content strings.Builder
	
	content.WriteString("# TiDB Upgrade Precheck Report\n\n")
	
	content.WriteString(fmt.Sprintf("**Started At:** %s  \n", report.StartedAt.Format(time.RFC3339)))
	content.WriteString(fmt.Sprintf("**Finished At:** %s\n\n", report.FinishedAt.Format(time.RFC3339)))
	
	content.WriteString("## Summary\n\n")
	content.WriteString("| Category | Count |\n")
	content.WriteString("|----------|-------|\n")
	content.WriteString(fmt.Sprintf("| Total Items | %d |\n", report.Summary.Total))
	content.WriteString(fmt.Sprintf("| Blocking Issues | %d |\n", report.Summary.Blocking))
	content.WriteString(fmt.Sprintf("| Warnings | %d |\n", report.Summary.Warnings))
	content.WriteString(fmt.Sprintf("| Info | %d |\n\n", report.Summary.Infos))
	
	if len(report.Items) > 0 {
		content.WriteString("## Issues\n\n")
		content.WriteString("| Severity | Rule | Message |\n")
		content.WriteString("|----------|------|---------|\n")
		for _, item := range report.Items {
			content.WriteString(fmt.Sprintf("| %s | %s | %s |\n", item.Severity, item.Rule, item.Message))
		}
		content.WriteString("\n")
	}
	
	if len(report.Errors) > 0 {
		content.WriteString("## Errors\n\n")
		content.WriteString("| Error |\n")
		content.WriteString("|-------|\n")
		for _, err := range report.Errors {
			content.WriteString(fmt.Sprintf("| %s |\n", err))
		}
		content.WriteString("\n")
	}
	
	return content.String(), nil
}

// generateHTMLReport generates an HTML format report
func (g *Generator) generateHTMLReport(report *precheck.Report, options *Options) (string, error) {
	const htmlTemplate = `<!DOCTYPE html>
<html>
<head>
    <meta charset="utf-8">
    <title>TiDB Upgrade Precheck Report</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 40px; }
        h1 { color: #333; }
        table { border-collapse: collapse; width: 100%; margin: 20px 0; }
        th, td { border: 1px solid #ddd; padding: 8px; text-align: left; }
        th { background-color: #f2f2f2; }
        code { background-color: #f8f8f8; padding: 2px 4px; }
    </style>
</head>
<body>
    <h1>TiDB Upgrade Precheck Report</h1>
    
    <p><strong>Started At:</strong> {{.StartedAt}}</p>
    <p><strong>Finished At:</strong> {{.FinishedAt}}</p>
    
    <h2>Summary</h2>
    <table>
        <tr><th>Category</th><th>Count</th></tr>
        <tr><td>Total Items</td><td>{{.Summary.Total}}</td></tr>
        <tr><td>Blocking Issues</td><td>{{.Summary.Blocking}}</td></tr>
        <tr><td>Warnings</td><td>{{.Summary.Warnings}}</td></tr>
        <tr><td>Info</td><td>{{.Summary.Infos}}</td></tr>
    </table>
    
    {{if .Items}}
    <h2>Issues</h2>
    <table>
        <tr><th>Severity</th><th>Rule</th><th>Message</th></tr>
        {{range .Items}}
        <tr>
            <td>{{.Severity}}</td>
            <td>{{.Rule}}</td>
            <td>{{.Message}}</td>
        </tr>
        {{end}}
    </table>
    {{end}}
    
    {{if .Errors}}
    <h2>Errors</h2>
    <ul>
        {{range .Errors}}
        <li>{{.}}</li>
        {{end}}
    </ul>
    {{end}}
</body>
</html>`

	tmpl, err := template.New("report").Parse(htmlTemplate)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, report); err != nil {
		return "", err
	}

	return buf.String(), nil
}

// generateJSONReport generates a JSON format report
func (g *Generator) generateJSONReport(report *precheck.Report, options *Options) (string, error) {
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}