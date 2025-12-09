// Package reporter provides report generation for analyzer results
package reporter

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/pingcap/tidb-upgrade-precheck/pkg/analyzer"
	"github.com/pingcap/tidb-upgrade-precheck/pkg/reporter/formats"
	"github.com/pingcap/tidb-upgrade-precheck/pkg/reporter/formats/html"
	jsonfmt "github.com/pingcap/tidb-upgrade-precheck/pkg/reporter/formats/json"
	"github.com/pingcap/tidb-upgrade-precheck/pkg/reporter/formats/markdown"
	"github.com/pingcap/tidb-upgrade-precheck/pkg/reporter/formats/text"
)

// Format represents the output format of the report
type Format string

const (
	TextFormat     Format = "text"
	MarkdownFormat Format = "markdown"
	HTMLFormat     Format = "html"
	JSONFormat     Format = "json"
)

// Options defines options for report generation
type Options struct {
	Format    Format
	OutputDir string
	Filename  string
}

// Generator generates reports in various formats
type Generator struct{}

// NewGenerator creates a new report generator
func NewGenerator() *Generator {
	return &Generator{}
}

// GenerateFromAnalysisResult generates a report from analyzer.AnalysisResult
// Uses modular formatters for different output formats
func (g *Generator) GenerateFromAnalysisResult(result *analyzer.AnalysisResult, options *Options) (string, error) {
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

	// Use format-specific formatters
	formatStr := string(options.Format)
	switch formatStr {
	case "text":
		formatter := text.NewTextFormatter()
		content, err = formatter.Generate(result, &formats.Options{
			Format:    formats.TextFormat,
			OutputDir: options.OutputDir,
			Filename:  options.Filename,
		})
	case "markdown":
		formatter := markdown.NewMarkdownFormatter()
		content, err = formatter.Generate(result, &formats.Options{
			Format:    formats.MarkdownFormat,
			OutputDir: options.OutputDir,
			Filename:  options.Filename,
		})
	case "html":
		formatter := html.NewHTMLFormatter()
		content, err = formatter.Generate(result, &formats.Options{
			Format:    formats.HTMLFormat,
			OutputDir: options.OutputDir,
			Filename:  options.Filename,
		})
	case "json":
		formatter := jsonfmt.NewJSONFormatter()
		content, err = formatter.Generate(result, &formats.Options{
			Format:    formats.JSONFormat,
			OutputDir: options.OutputDir,
			Filename:  options.Filename,
		})
	default:
		return "", fmt.Errorf("unsupported format: %s", formatStr)
	}

	if err != nil {
		return "", fmt.Errorf("failed to generate report content: %w", err)
	}

	// Write to file
	filePath := filepath.Join(options.OutputDir, fmt.Sprintf("%s.%s", filename, getFileExtension(options.Format)))
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		return "", fmt.Errorf("failed to write report to file: %w", err)
	}

	return filePath, nil
}

// getFileExtension returns the file extension for a given format
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
