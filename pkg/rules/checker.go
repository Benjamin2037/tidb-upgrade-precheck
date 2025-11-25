package rules

import (
	"github.com/pingcap/tidb-upgrade-precheck/pkg/runtime"
)

// CheckResult represents the result of a single check
type CheckResult struct {
	RuleID      string `json:"rule_id"`
	Description string `json:"description"`
	Severity    string `json:"severity"` // "info", "warning", "error", "critical"
	Message     string `json:"message"`
	Details     string `json:"details,omitempty"`
}

// Checker defines the interface for checking upgrade compatibility
type Checker interface {
	// Check performs the compatibility check
	Check(snapshot *runtime.ClusterSnapshot) ([]CheckResult, error)
	
	// RuleID returns the unique identifier for this check rule
	RuleID() string
	
	// Description returns a brief description of what this rule checks
	Description() string
}

// CheckRunner orchestrates the execution of all checks
type CheckRunner struct {
	checkers []Checker
}

// NewCheckRunner creates a new check runner with the provided checkers
func NewCheckRunner(checkers []Checker) *CheckRunner {
	return &CheckRunner{
		checkers: checkers,
	}
}

// Run executes all checks against the provided cluster snapshot
func (cr *CheckRunner) Run(snapshot *runtime.ClusterSnapshot) ([]CheckResult, error) {
	var results []CheckResult
	
	for _, checker := range cr.checkers {
		checkResults, err := checker.Check(snapshot)
		if err != nil {
			// Log error but continue with other checks
			results = append(results, CheckResult{
				RuleID:      checker.RuleID(),
				Description: checker.Description(),
				Severity:    "error",
				Message:     "Check execution failed",
				Details:     err.Error(),
			})
			continue
		}
		
		results = append(results, checkResults...)
	}
	
	return results, nil
}