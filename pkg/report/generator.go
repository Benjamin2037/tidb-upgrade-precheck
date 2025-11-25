package report

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/pingcap/tidb-upgrade-precheck/pkg/precheck"
)

// Generator defines the interface for generating reports
type Generator interface {
	Generate(report *Report, options *Options) (*GeneratedReport, error)
}

// Options defines options for report generation
type Options struct {
	Format    Format
	OutputDir string
}

// Format represents the output format of the report
type Format string

const (
	MarkdownFormat Format = "markdown"
	HTMLFormat     Format = "html"
	TextFormat     Format = "text"
)

// GeneratedReport represents a generated report
type GeneratedReport struct {
	Path string
	Data string
}

// generator implements the Generator interface
type generator struct{}

// NewGenerator creates a new report generator
func NewGenerator() Generator {
	return &generator{}
}

// Generate generates a report based on the provided data and options
func (g *generator) Generate(report *Report, options *Options) (*GeneratedReport, error) {
	// Set timestamp if not already set
	if report.GeneratedAt == "" {
		report.GeneratedAt = time.Now().Format(time.RFC3339)
	}

	var content string
	var extension string
	var err error

	switch options.Format {
	case MarkdownFormat:
		content, err = RenderMarkdownReport(report)
		extension = ".md"
	case HTMLFormat:
		content, err = RenderHTMLReport(report)
		extension = ".html"
	case TextFormat:
		content, err = renderTextReport(report)
		extension = ".txt"
	default:
		return nil, fmt.Errorf("unsupported format: %s", options.Format)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to render report: %w", err)
	}

	// If output directory is specified, write to file
	if options.OutputDir != "" {
		// Create output directory if it doesn't exist
		if err := os.MkdirAll(options.OutputDir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create output directory: %w", err)
		}

		// Generate filename
		filename := fmt.Sprintf("tidb_upgrade_precheck_report_%s%s", 
			time.Now().Format("20060102_150405"), extension)
		filePath := filepath.Join(options.OutputDir, filename)

		// Write to file
		if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
			return nil, fmt.Errorf("failed to write report to file: %w", err)
		}

		return &GeneratedReport{
			Path: filePath,
			Data: content,
		}, nil
	}

	// If no output directory specified, return content only
	return &GeneratedReport{
		Data: content,
	}, nil
}

// renderTextReport renders the report as plain text
func renderTextReport(report *Report) (string, error) {
	// For now, we'll just return a simple text representation
	// In a real implementation, this would be a more detailed text format
	content := fmt.Sprintf("TiDB Upgrade Precheck Report\n")
	content += fmt.Sprintf("Cluster: %s\n", report.ClusterName)
	content += fmt.Sprintf("Upgrade Path: %s\n", report.UpgradePath)
	content += fmt.Sprintf("Generated At: %s\n\n", report.GeneratedAt)
	
	content += fmt.Sprintf("Summary:\n")
	content += fmt.Sprintf("  HIGH:   %d\n", report.Summary[precheck.RiskHigh])
	content += fmt.Sprintf("  MEDIUM: %d\n", report.Summary[precheck.RiskMedium])
	content += fmt.Sprintf("  INFO:   %d\n\n", report.Summary[precheck.RiskInfo])
	
	if len(report.Risks) > 0 {
		content += fmt.Sprintf("Risks:\n")
		for _, risk := range report.Risks {
			content += fmt.Sprintf("  [%s] %s: %s\n", risk.Level, risk.Parameter, risk.Impact)
		}
		content += fmt.Sprintf("\n")
	}
	
	if len(report.Audits) > 0 {
		content += fmt.Sprintf("Configuration Audits:\n")
		for _, audit := range report.Audits {
			content += fmt.Sprintf("  %s.%s: %s (target: %s)\n", 
				audit.Component, audit.Parameter, audit.Current, audit.Target)
		}
		content += fmt.Sprintf("\n")
	}
	
	return content, nil
}