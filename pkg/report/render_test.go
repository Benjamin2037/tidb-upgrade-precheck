package report

import (
	"os"
	"testing"
)

func TestRenderMarkdownReport(t *testing.T) {
	report := &Report{
		ClusterName: "test-cluster",
		UpgradePath: "v7.5.1 -> v8.5.3",
		Summary: map[RiskLevel]int{
			RiskHigh:   1,
			RiskMedium: 2,
			RiskInfo:   3,
		},
		Risks: []RiskItem{{
			Component:  "TiDB",
			Parameter:  "tidb_enable_new_storage",
			Current:    "OFF",
			Target:     "ON",
			Level:      RiskHigh,
			Impact:     "The v8.5.3 upgrade process will forcefully overwrite your setting ('OFF') to 'ON'.",
			Suggestion: "Check disk usage.",
			RDComment:  "Mandatory for v8.0+. This may increase disk usage by ~10%.",
		}},
		Audits: []AuditItem{{
			Component: "TiKV",
			Parameter: "raftstore.apply-pool-size",
			Current:   "3",
			Target:    "2",
			Status:    "User Custom",
		}},
		GeneratedAt: "2025-11-20T12:00:00Z",
	}
	md, err := RenderMarkdownReport(report)
	if err != nil {
		t.Fatalf("markdown render failed: %v", err)
	}
	if len(md) == 0 || md[0] != '#' {
		t.Errorf("unexpected markdown output: %s", md)
	}
}

func TestRenderHTMLReport(t *testing.T) {
	report := &Report{
		ClusterName: "test-cluster",
		UpgradePath: "v7.5.1 -> v8.5.3",
		Summary: map[RiskLevel]int{
			RiskHigh:   1,
			RiskMedium: 2,
			RiskInfo:   3,
		},
		Risks: []RiskItem{{
			Component:  "TiDB",
			Parameter:  "tidb_enable_new_storage",
			Current:    "OFF",
			Target:     "ON",
			Level:      RiskHigh,
			Impact:     "The v8.5.3 upgrade process will forcefully overwrite your setting ('OFF') to 'ON'.",
			Suggestion: "Check disk usage.",
			RDComment:  "Mandatory for v8.0+. This may increase disk usage by ~10%.",
		}},
		Audits: []AuditItem{{
			Component: "TiKV",
			Parameter: "raftstore.apply-pool-size",
			Current:   "3",
			Target:    "2",
			Status:    "User Custom",
		}},
		GeneratedAt: "2025-11-20T12:00:00Z",
	}
	html, err := RenderHTMLReport(report)
	if err != nil {
		t.Fatalf("html render failed: %v", err)
	}
	if len(html) == 0 || html[:6] != "<!DOCT" {
		t.Errorf("unexpected html output: %s", html)
	}
}

func TestWriteReportToFile(t *testing.T) {
	report := &Report{
		ClusterName: "test-cluster",
		UpgradePath: "v7.5.1 -> v8.5.3",
		Summary: map[RiskLevel]int{
			RiskHigh:   1,
			RiskMedium: 2,
			RiskInfo:   3,
		},
		Risks:       nil,
		Audits:      nil,
		GeneratedAt: "2025-11-20T12:00:00Z",
	}
	dir := t.TempDir()
	file, err := WriteReportToFile(report, "md", dir)
	if err != nil {
		t.Fatalf("write markdown failed: %v", err)
	}
	if _, err := os.Stat(file); err != nil {
		t.Errorf("markdown file not found: %v", err)
	}
	file, err = WriteReportToFile(report, "html", dir)
	if err != nil {
		t.Fatalf("write html failed: %v", err)
	}
	if _, err := os.Stat(file); err != nil {
		t.Errorf("html file not found: %v", err)
	}
}

// END: file end, close package scope
