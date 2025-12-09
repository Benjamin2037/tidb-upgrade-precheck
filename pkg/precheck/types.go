package precheck

import "github.com/pingcap/tidb-upgrade-precheck/pkg/rules"

// RiskLevel represents the risk level of an upgrade issue
type RiskLevel string

const (
	RiskHigh   RiskLevel = "HIGH"
	RiskMedium RiskLevel = "MEDIUM"
	RiskLow    RiskLevel = "LOW"
	RiskInfo   RiskLevel = "INFO"
)

// RiskItem represents a potential risk identified during upgrade precheck
type RiskItem struct {
	Level     RiskLevel `json:"level"`
	Parameter string    `json:"parameter"`
	Component string    `json:"component"`
	Message   string    `json:"message"`
	Details   string    `json:"details"`
}

func ConvertToRiskItems(checkResults []rules.CheckResult) []RiskItem {
	var risks []RiskItem
	for _, result := range checkResults {
		risk := RiskItem{
			Parameter: result.RuleID,
			Component: "unknown", // This would need to be determined from the check result
			Message:   result.Message,
			Details:   result.Details,
		}
		
		// Map severity to risk level
		switch result.Severity {
		case "critical", "error":
			risk.Level = RiskHigh
		case "warning":
			risk.Level = RiskMedium
		case "info":
			risk.Level = RiskInfo
		default:
			risk.Level = RiskLow
		}
		
		risks = append(risks, risk)
	}
	return risks
}

