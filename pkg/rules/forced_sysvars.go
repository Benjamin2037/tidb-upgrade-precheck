package rules

import (
	"fmt"

	"github.com/pingcap/tidb-upgrade-precheck/pkg/runtime"
)

// ForcedSysVarsRule checks for system variables that will be forcibly changed during upgrade
type ForcedSysVarsRule struct{}

// NewForcedSysVarsRule creates a new forced system variables rule
func NewForcedSysVarsRule() *ForcedSysVarsRule {
	return &ForcedSysVarsRule{}
}

// RuleID returns the rule ID
func (r *ForcedSysVarsRule) RuleID() string {
	return "FORCED_SYSVARS_CHECK"
}

// Description returns the rule description
func (r *ForcedSysVarsRule) Description() string {
	return "Check for system variables that will be forcibly changed during upgrade"
}

// Check performs the forced system variables check
func (r *ForcedSysVarsRule) Check(snapshot *runtime.ClusterState) ([]CheckResult, error) {
	var results []CheckResult

	// Check each TiDB instance for system variables that may be forcibly changed
	for _, instance := range snapshot.Instances {
		if instance.State.Type != runtime.TiDBComponent {
			continue
		}

		sysVarResults := r.checkForcedSysVars(instance)
		results = append(results, sysVarResults...)
	}

	return results, nil
}

// checkForcedSysVars checks for system variables that may be forcibly changed during upgrade
func (r *ForcedSysVarsRule) checkForcedSysVars(instance runtime.InstanceState) []CheckResult {
	var results []CheckResult

	// Check for system variables that are commonly forcibly changed during upgrade
	forcedChangeVars := map[string]string{
		"tidb_enable_table_partition": "This variable may be forcibly set to 'on' during upgrade",
		"tidb_enable_vector":         "This variable may be forcibly changed during upgrade",
	}

	for varName, warningMsg := range forcedChangeVars {
		if _, exists := instance.State.Config[varName]; exists {
			results = append(results, CheckResult{
				RuleID:      r.RuleID(),
				Description: r.Description(),
				Severity:    "info",
				Message:     fmt.Sprintf("System variable may be forcibly changed during upgrade: %s", varName),
				Details:     warningMsg,
			})
		}
	}

	return results
}