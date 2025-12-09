# Report Generator Design

This document describes the detailed design and implementation of the report generator module.

## Overview

The report generator produces unified, standardized precheck reports in various formats. It provides a flexible framework for generating reports in different formats while maintaining consistency in content structure.

## Architecture

The report generator uses a format-agnostic design where report content is structured as sections, and each format (text, markdown, HTML, JSON) renders these sections appropriately.

**Key Design Principles:**
- **Format-Specific Formatters**: Each format has its own formatter implementation
- **Modular Sections**: Report content is organized into independent sections
- **Consistent Structure**: All formats follow the same content structure
- **Extensible**: New formats can be added by implementing the formatter interface

## Supported Formats

### Text Format

**Location**: `pkg/reporter/formats/text/`

**Features:**
- Console-friendly output
- Simple text formatting
- Suitable for terminal output

**Implementation:**
- `text.go`: Main formatter
- `header.go`: Report header
- `footer.go`: Report footer
- `sections/parameter_check.go`: Parameter check section

### Markdown Format

**Location**: `pkg/reporter/formats/markdown/`

**Features:**
- Markdown-formatted output
- Suitable for documentation and version control
- Supports tables and code blocks

**Implementation:**
- `markdown.go`: Main formatter
- `header.go`: Report header
- `footer.go`: Report footer
- `sections/parameter_check.go`: Parameter check section

### HTML Format

**Location**: `pkg/reporter/formats/html/`

**Features:**
- Rich HTML formatting
- Interactive elements
- Suitable for web viewing
- Includes CSS styling

**Implementation:**
- `html.go`: Main formatter
- `header.go`: Report header with CSS
- `footer.go`: Report footer
- `sections/parameter_check.go`: Parameter check section

### JSON Format

**Location**: `pkg/reporter/formats/json/`

**Features:**
- Machine-readable format
- Suitable for programmatic processing
- Complete data structure preservation

**Implementation:**
- `json.go`: Main formatter (direct JSON marshalling of `AnalysisResult`)

## Report Structure

All formats follow the same content structure:

1. **Header**: Report metadata (version, timestamp, cluster info)
2. **Executive Summary**: High-level risk overview
3. **Risk Assessment by Severity**: Grouped by severity level
4. **Parameter Change Details**: Detailed parameter changes
5. **Upgrade Recommendations**: Actionable recommendations
6. **Component-Specific Analysis**: Per-component analysis
7. **Footer**: Additional information and references

## Implementation

### Generator Interface

**Location**: `pkg/reporter/reporter.go`

```go
type Generator struct{}

func (g *Generator) GenerateFromAnalysisResult(
    result *analyzer.AnalysisResult,
    options *Options,
) (string, error)
```

### Format-Specific Formatters

Each format implements the formatter interface:

```go
type Formatter interface {
    Generate(result *analyzer.AnalysisResult, options *formats.Options) (string, error)
}
```

**Current Implementations:**
- `text.NewTextFormatter()`
- `markdown.NewMarkdownFormatter()`
- `html.NewHTMLFormatter()`
- `json.NewJSONFormatter()`

### Section-Based Design

Reports are generated section by section, allowing each format to render sections appropriately:

- **Header/Footer**: Format-specific implementations in each format directory
- **Sections**: Format-specific implementations in `sections/` subdirectory

## Usage

```go
generator := reporter.NewGenerator()
options := &reporter.Options{
    Format:    reporter.HTMLFormat,
    OutputDir: "./reports",
    Filename:  "precheck_report",
}

filePath, err := generator.GenerateFromAnalysisResult(analysisResult, options)
```

## Customization

### Adding New Formats

1. Create format directory: `pkg/reporter/formats/<format_name>/`
2. Implement formatter interface:
   ```go
   type FormatNameFormatter struct{}
   
   func NewFormatNameFormatter() *FormatNameFormatter {
       return &FormatNameFormatter{}
   }
   
   func (f *FormatNameFormatter) Generate(result *analyzer.AnalysisResult, options *formats.Options) (string, error) {
       // Implementation
   }
   ```
3. Register format in `pkg/reporter/reporter.go` switch statement
4. Implement header, footer, and sections for the new format

### Customizing Report Content

Report content can be customized by:
- Modifying section renderers in `pkg/reporter/formats/<format>/sections/`
- Adding new sections
- Customizing formatting styles in format-specific files

## Related Documents

- [Analyzer Design](../analyzer/) - Risk analysis results that feed into reports
