package report
package report

// RiskLevel defines the severity of a risk.
package report

// RiskLevel defines the severity of a risk.
type RiskLevel string

const (
	RiskHigh   RiskLevel = "HIGH"
	RiskMedium RiskLevel = "MEDIUM"
	RiskInfo   RiskLevel = "INFO"
)

// RiskItem represents a single risk found during upgrade analysis.
type RiskItem struct {
	Component   string    // e.g. TiDB, TiKV, PD
	Parameter   string    // e.g. tidb_enable_new_storage
	Current     string    // current value
	Target      string    // target value or default
	Level       RiskLevel // HIGH, MEDIUM, INFO
	Impact      string    // summary of impact
	Suggestion  string    // recommended action
	RDComment   string    // R&D comments or PR/issue link
}

// AuditItem represents a configuration audit entry.
type AuditItem struct {
	Component string
	Parameter string
	Current   string
	Target    string
	Status    string // e.g. "User Custom", "Default"
}

// Report is the top-level structure for a full upgrade precheck report.
type Report struct {
	ClusterName   string
	UpgradePath   string // e.g. v7.5.1 (Bootstrap: 180) -> v8.5.3 (Bootstrap: 218)
	Summary       map[RiskLevel]int // risk level -> count
	Risks         []RiskItem
	Audits        []AuditItem
	GeneratedAt   string // timestamp
}