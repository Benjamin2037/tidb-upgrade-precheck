package precheck

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
	Impact    string    `json:"impact"`
	Detail    string    `json:"detail"`
}

// AuditItem represents a configuration audit item
type AuditItem struct {
	Component string `json:"component"`
	Parameter string `json:"parameter"`
	Current   string `json:"current"`
	Target    string `json:"target"`
}