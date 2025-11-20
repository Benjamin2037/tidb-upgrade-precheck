package report

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/pingcap/tidb-upgrade-precheck/pkg/precheck"
)

// ConvertEngineReportToReport converts the precheck.Engine Report to the new Report format for rendering.
func ConvertEngineReportToReport(clusterName, upgradePath string, r *precheck.Report) *Report {
	summary := map[RiskLevel]int{
		RiskHigh:   0,
		RiskMedium: 0,
		RiskInfo:   0,
	}
	var risks []RiskItem
	var audits []AuditItem
	for _, item := range r.Items {
		level := RiskInfo
		if item.Severity == precheck.SeverityBlocker {
			level = RiskHigh
		} else if item.Severity == precheck.SeverityWarning {
			level = RiskMedium
		}
		summary[level]++
		// Simple mapping, can be extended to classify by item.Rule
		risk := RiskItem{
			Component:  fmt.Sprint(item.Metadata),
			Parameter:  item.Rule,
			Current:    "",
			Target:     "",
			Level:      level,
			Impact:     item.Message,
			Suggestion: "",
			RDComment:  "",
		}
		if len(item.Suggestions) > 0 {
			risk.Suggestion = item.Suggestions[0]
		}
		if len(item.Details) > 0 {
			risk.RDComment = item.Details[0]
		}
		risks = append(risks, risk)
	}
	// Audit items can be supplemented according to rules
	return &Report{
		ClusterName: clusterName,
		UpgradePath: upgradePath,
		Summary:     summary,
		Risks:       risks,
		Audits:      audits,
		GeneratedAt: time.Now().Format(time.RFC3339),
	}
}

// WriteReportToFile writes the rendered report to the given directory and format.
func WriteReportToFile(report *Report, format, dir string) (string, error) {
	var content string
	var err error
	var ext string
	switch format {
	case "md":
		content, err = RenderMarkdownReport(report)
		ext = ".md"
	case "html":
		content, err = RenderHTMLReport(report)
		ext = ".html"
	default:
		return "", fmt.Errorf("unsupported report format: %s", format)
	}
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	filename := filepath.Join(dir, "report"+ext)
	if err := os.WriteFile(filename, []byte(content), 0o644); err != nil {
		return "", err
	}
	return filename, nil
}
