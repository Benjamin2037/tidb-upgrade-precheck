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
	Check(snapshot *runtime.ClusterState) ([]CheckResult, error)
	
	// RuleID returns the unique identifier for this check rule
	RuleID() string
	
	// Description returns a brief description of what this rule checks
	Description() string
}

// CheckRunner orchestrates the execution of all checks
type CheckRunner struct {
	checkers []Checker
}

// NewCheckRunner creates a new check runner
func NewCheckRunner(checkers []Checker) *CheckRunner {
	return &CheckRunner{
		checkers: checkers,
	}
}

// Run executes all checks and returns the combined results
func (r *CheckRunner) Run(snapshot *runtime.ClusterState) ([]CheckResult, error) {
	var allResults []CheckResult
	
	for _, checker := range r.checkers {
		results, err := checker.Check(snapshot)
		if err != nil {
			// Log error but continue with other checkers
			continue
		}
		allResults = append(allResults, results...)
	}
	
	return allResults, nil
}