// Copyright 2024 PingCAP, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// See the License for the specific language governing permissions and
// limitations under the License.

package reporter

import (
	"encoding/json"
	"fmt"
	"strings"
	"text/template"

	"github.com/pingcap/tidb-upgrade-precheck/pkg/analyzer"
)

// ReportFormat represents the format of the report
type ReportFormat string

const (
	// JSONFormat represents JSON format
	JSONFormat ReportFormat = "json"
	// TextFormat represents plain text format
	TextFormat = "text"
	// HTMLFormat represents HTML format
	HTMLFormat = "html"
)

// Reporter is responsible for generating reports
type Reporter struct {
	format ReportFormat
}

// NewReporter creates a new reporter
func NewReporter(format ReportFormat) *Reporter {
	return &Reporter{
		format: format,
	}
}

// GenerateUpgradeReport generates an upgrade report
func (r *Reporter) GenerateUpgradeReport(report *analyzer.AnalysisReport) ([]byte, error) {
	switch r.format {
	case JSONFormat:
		return r.generateJSONUpgradeReport(report)
	case TextFormat:
		return r.generateTextUpgradeReport(report)
	case HTMLFormat:
		return r.generateHTMLUpgradeReport(report)
	default:
		return nil, fmt.Errorf("unsupported report format: %s", r.format)
	}
}

// GenerateClusterReport generates a cluster configuration report
func (r *Reporter) GenerateClusterReport(report *analyzer.ClusterAnalysisReport) ([]byte, error) {
	switch r.format {
	case JSONFormat:
		return r.generateJSONClusterReport(report)
	case TextFormat:
		return r.generateTextClusterReport(report)
	case HTMLFormat:
		return r.generateHTMLClusterReport(report)
	default:
		return nil, fmt.Errorf("unsupported report format: %s", r.format)
	}
}

// generateJSONUpgradeReport generates a JSON upgrade report
func (r *Reporter) generateJSONUpgradeReport(report *analyzer.AnalysisReport) ([]byte, error) {
	return json.MarshalIndent(report, "", "  ")
}

// generateTextUpgradeReport generates a text upgrade report
func (r *Reporter) generateTextUpgradeReport(report *analyzer.AnalysisReport) ([]byte, error) {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("=== UPGRADE ANALYSIS REPORT ===\n"))
	sb.WriteString(fmt.Sprintf("Component: %s\n", report.Component))
	sb.WriteString(fmt.Sprintf("Version: %s -> %s\n", report.VersionFrom, report.VersionTo))
	sb.WriteString(fmt.Sprintf("\n"))

	sb.WriteString(fmt.Sprintf("SUMMARY:\n"))
	sb.WriteString(fmt.Sprintf("  Total Changes: %d\n", report.Summary.TotalChanges))
	sb.WriteString(fmt.Sprintf("  Added: %d\n", report.Summary.Added))
	sb.WriteString(fmt.Sprintf("  Removed: %d\n", report.Summary.Removed))
	sb.WriteString(fmt.Sprintf("  Modified: %d\n", report.Summary.Modified))
	sb.WriteString(fmt.Sprintf("  High Risk: %d\n", report.Summary.HighRisk))
	sb.WriteString(fmt.Sprintf("  Medium Risk: %d\n", report.Summary.MediumRisk))
	sb.WriteString(fmt.Sprintf("  Low Risk: %d\n", report.Summary.LowRisk))
	sb.WriteString(fmt.Sprintf("  Info Level: %d\n", report.Summary.InfoLevel))
	sb.WriteString(fmt.Sprintf("\n"))

	if len(report.Parameters) > 0 {
		sb.WriteString(fmt.Sprintf("PARAMETER CHANGES:\n"))
		for _, param := range report.Parameters {
			sb.WriteString(fmt.Sprintf("  [%s] %s\n", param.RiskLevel, param.Name))
			sb.WriteString(fmt.Sprintf("    Description: %s\n", param.Description))
			sb.WriteString(fmt.Sprintf("    From: %v\n", param.FromValue))
			sb.WriteString(fmt.Sprintf("    To: %v\n", param.ToValue))
			sb.WriteString(fmt.Sprintf("\n"))
		}
	} else {
		sb.WriteString(fmt.Sprintf("No parameter changes detected.\n"))
	}

	return []byte(sb.String()), nil
}

// generateHTMLUpgradeReport generates an HTML upgrade report
func (r *Reporter) generateHTMLUpgradeReport(report *analyzer.AnalysisReport) ([]byte, error) {
	const htmlTemplate = `
<!DOCTYPE html>
<html>
<head>
    <title>Upgrade Analysis Report</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 20px; }
        h1 { color: #333; }
        h2 { color: #666; }
        .summary { background-color: #f5f5f5; padding: 10px; border-radius: 5px; }
        .parameter-change { border: 1px solid #ddd; margin: 10px 0; padding: 10px; border-radius: 5px; }
        .high-risk { border-left: 5px solid #d9534f; }
        .medium-risk { border-left: 5px solid #f0ad4e; }
        .low-risk { border-left: 5px solid #5bc0de; }
        .info-level { border-left: 5px solid #5cb85c; }
    </style>
</head>
<body>
    <h1>UPGRADE ANALYSIS REPORT</h1>
    
    <p><strong>Component:</strong> {{.Component}}</p>
    <p><strong>Version:</strong> {{.VersionFrom}} -> {{.VersionTo}}</p>
    
    <div class="summary">
        <h2>SUMMARY</h2>
        <p>Total Changes: {{.Summary.TotalChanges}}</p>
        <p>Added: {{.Summary.Added}}</p>
        <p>Removed: {{.Summary.Removed}}</p>
        <p>Modified: {{.Summary.Modified}}</p>
        <p>High Risk: {{.Summary.HighRisk}}</p>
        <p>Medium Risk: {{.Summary.MediumRisk}}</p>
        <p>Low Risk: {{.Summary.LowRisk}}</p>
        <p>Info Level: {{.Summary.InfoLevel}}</p>
    </div>
    
    <h2>PARAMETER CHANGES</h2>
    {{range .Parameters}}
    <div class="parameter-change {{.RiskLevel}}-risk">
        <h3>[{{.RiskLevel}}] {{.Name}}</h3>
        <p><strong>Description:</strong> {{.Description}}</p>
        <p><strong>From:</strong> {{.FromValue}}</p>
        <p><strong>To:</strong> {{.ToValue}}</p>
    </div>
    {{else}}
    <p>No parameter changes detected.</p>
    {{end}}
</body>
</html>
`

	tmpl, err := template.New("upgrade-report").Parse(htmlTemplate)
	if err != nil {
		return nil, fmt.Errorf("failed to parse template: %v", err)
	}

	var sb strings.Builder
	if err := tmpl.Execute(&sb, report); err != nil {
		return nil, fmt.Errorf("failed to execute template: %v", err)
	}

	return []byte(sb.String()), nil
}

// generateJSONClusterReport generates a JSON cluster report
func (r *Reporter) generateJSONClusterReport(report *analyzer.ClusterAnalysisReport) ([]byte, error) {
	return json.MarshalIndent(report, "", "  ")
}

// generateTextClusterReport generates a text cluster report
func (r *Reporter) generateTextClusterReport(report *analyzer.ClusterAnalysisReport) ([]byte, error) {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("=== CLUSTER CONFIGURATION ANALYSIS REPORT ===\n"))
	sb.WriteString(fmt.Sprintf("\n"))

	sb.WriteString(fmt.Sprintf("INSTANCES:\n"))
	for _, instance := range report.Instances {
		sb.WriteString(fmt.Sprintf("  %s (%s): %s\n", instance.Address, instance.State.Type, instance.State.Version))
	}
	sb.WriteString(fmt.Sprintf("\n"))

	if len(report.InconsistentConfigs) > 0 {
		sb.WriteString(fmt.Sprintf("INCONSISTENT CONFIGURATIONS:\n"))
		for _, config := range report.InconsistentConfigs {
			sb.WriteString(fmt.Sprintf("  [%s] %s\n", config.RiskLevel, config.ParameterName))
			sb.WriteString(fmt.Sprintf("    Description: %s\n", config.Description))
			for _, value := range config.Values {
				sb.WriteString(fmt.Sprintf("    %s: %v\n", value.InstanceAddress, value.Value))
			}
			sb.WriteString(fmt.Sprintf("\n"))
		}
	} else {
		sb.WriteString(fmt.Sprintf("No inconsistent configurations detected.\n"))
		sb.WriteString(fmt.Sprintf("\n"))
	}

	if len(report.Recommendations) > 0 {
		sb.WriteString(fmt.Sprintf("RECOMMENDATIONS:\n"))
		for _, rec := range report.Recommendations {
			sb.WriteString(fmt.Sprintf("  [%s] %s\n", rec.RiskLevel, rec.ParameterName))
			sb.WriteString(fmt.Sprintf("    Description: %s\n", rec.Description))
			sb.WriteString(fmt.Sprintf("    Recommendation: %s\n", rec.Recommendation))
			sb.WriteString(fmt.Sprintf("\n"))
		}
	} else {
		sb.WriteString(fmt.Sprintf("No recommendations.\n"))
		sb.WriteString(fmt.Sprintf("\n"))
	}

	return []byte(sb.String()), nil
}

// generateHTMLClusterReport generates an HTML cluster report
func (r *Reporter) generateHTMLClusterReport(report *analyzer.ClusterAnalysisReport) ([]byte, error) {
	const htmlTemplate = `
<!DOCTYPE html>
<html>
<head>
    <title>Cluster Configuration Analysis Report</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 20px; }
        h1 { color: #333; }
        h2 { color: #666; }
        .instances { background-color: #f5f5f5; padding: 10px; border-radius: 5px; }
        .inconsistent-config { border: 1px solid #ddd; margin: 10px 0; padding: 10px; border-radius: 5px; }
        .recommendation { border: 1px solid #ddd; margin: 10px 0; padding: 10px; border-radius: 5px; }
        .high-risk { border-left: 5px solid #d9534f; }
        .medium-risk { border-left: 5px solid #f0ad4e; }
        .low-risk { border-left: 5px solid #5bc0de; }
    </style>
</head>
<body>
    <h1>CLUSTER CONFIGURATION ANALYSIS REPORT</h1>
    
    <div class="instances">
        <h2>INSTANCES</h2>
        <ul>
        {{range .Instances}}
            <li>{{.Address}} ({{.State.Type}}): {{.State.Version}}</li>
        {{end}}
        </ul>
    </div>
    
    <h2>INCONSISTENT CONFIGURATIONS</h2>
    {{range .InconsistentConfigs}}
    <div class="inconsistent-config {{.RiskLevel}}-risk">
        <h3>[{{.RiskLevel}}] {{.ParameterName}}</h3>
        <p><strong>Description:</strong> {{.Description}}</p>
        <ul>
        {{range .Values}}
            <li>{{.InstanceAddress}}: {{.Value}}</li>
        {{end}}
        </ul>
    </div>
    {{else}}
    <p>No inconsistent configurations detected.</p>
    {{end}}
    
    <h2>RECOMMENDATIONS</h2>
    {{range .Recommendations}}
    <div class="recommendation {{.RiskLevel}}-risk">
        <h3>[{{.RiskLevel}}] {{.ParameterName}}</h3>
        <p><strong>Description:</strong> {{.Description}}</p>
        <p><strong>Recommendation:</strong> {{.Recommendation}}</p>
    </div>
    {{else}}
    <p>No recommendations.</p>
    {{end}}
</body>
</html>
`

	tmpl, err := template.New("cluster-report").Parse(htmlTemplate)
	if err != nil {
		return nil, fmt.Errorf("failed to parse template: %v", err)
	}

	var sb strings.Builder
	if err := tmpl.Execute(&sb, report); err != nil {
		return nil, fmt.Errorf("failed to execute template: %v", err)
	}

	return []byte(sb.String()), nil
}