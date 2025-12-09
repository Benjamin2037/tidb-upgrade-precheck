package rules

import (
	"fmt"

	"github.com/pingcap/tidb-upgrade-precheck/pkg/runtime"
)

// SysVarCheckRule checks system variables for upgrade compatibility
type SysVarCheckRule struct{}

// NewSysVarCheckRule creates a new system variable check rule
func NewSysVarCheckRule() *SysVarCheckRule {
	return &SysVarCheckRule{}
}

// RuleID returns the rule ID
func (r *SysVarCheckRule) RuleID() string {
	return "SYSVAR_CHECK"
}

// Description returns the rule description
func (r *SysVarCheckRule) Description() string {
	return "Check system variables for upgrade compatibility"
}

// Check performs the system variable check
func (r *SysVarCheckRule) Check(snapshot *runtime.ClusterState) ([]CheckResult, error) {
	var results []CheckResult

	// Check each TiDB instance for system variables
	for _, instance := range snapshot.Instances {
		if instance.State.Type != runtime.TiDBComponent {
			continue
		}

		sysVarResults := r.checkSysVars(instance)
		results = append(results, sysVarResults...)
	}

	return results, nil
}

// checkSysVars checks system variables for a specific TiDB instance
func (r *SysVarCheckRule) checkSysVars(instance runtime.InstanceState) []CheckResult {
	var results []CheckResult

	// Check for deprecated system variables
	deprecatedVars := map[string]string{
		"tidb_enable_streaming": "This variable has been deprecated and will be removed in future versions",
	}

	for varName, warningMsg := range deprecatedVars {
		if _, exists := instance.State.Config[varName]; exists {
			results = append(results, CheckResult{
				RuleID:      r.RuleID(),
				Description: r.Description(),
				Severity:    "warning",
				Message:     fmt.Sprintf("Deprecated system variable found: %s", varName),
				Details:     warningMsg,
			})
		}
	}

	// Check for incompatible system variable values
	incompatibleVars := map[string]map[interface{}]string{
		"tidb_txn_mode": {
			"pessimistic": "Pessimistic transaction mode may cause performance issues in newer versions",
			"optimistic":  "Optimistic transaction mode may not support all features in newer versions",
		},
	}

	for varName, warnings := range incompatibleVars {
		if value, exists := instance.State.Config[varName]; exists {
			if warningMsg, hasWarning := warnings[value]; hasWarning {
				results = append(results, CheckResult{
					RuleID:      r.RuleID(),
					Description: r.Description(),
					Severity:    "warning",
					Message:     fmt.Sprintf("Potentially incompatible value for %s: %v", varName, value),
					Details:     warningMsg,
				})
			}
		}
	}

	return results
}