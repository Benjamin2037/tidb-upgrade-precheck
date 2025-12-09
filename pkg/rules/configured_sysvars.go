package rules

import (
	"fmt"

	"github.com/pingcap/tidb-upgrade-precheck/pkg/runtime"
)

// ConfiguredSysVarsRule checks for configured system variables that may affect upgrade
type ConfiguredSysVarsRule struct{}

// NewConfiguredSysVarsRule creates a new configured system variables rule
func NewConfiguredSysVarsRule() *ConfiguredSysVarsRule {
	return &ConfiguredSysVarsRule{}
}

// RuleID returns the rule ID
func (r *ConfiguredSysVarsRule) RuleID() string {
	return "CONFIGURED_SYSVARS_CHECK"
}

// Description returns the rule description
func (r *ConfiguredSysVarsRule) Description() string {
	return "Check for configured system variables that may affect upgrade"
}

// Check performs the configured system variables check
func (r *ConfiguredSysVarsRule) Check(snapshot *runtime.ClusterState) ([]CheckResult, error) {
	var results []CheckResult

	// Check each TiDB instance for configured system variables
	for _, instance := range snapshot.Instances {
		if instance.State.Type != runtime.TiDBComponent {
			continue
		}

		sysVarResults := r.checkConfiguredSysVars(instance)
		results = append(results, sysVarResults...)
	}

	return results, nil
}

// checkConfiguredSysVars checks configured system variables for a specific TiDB instance
func (r *ConfiguredSysVarsRule) checkConfiguredSysVars(instance runtime.InstanceState) []CheckResult {
	var results []CheckResult

	// Check for system variables that may cause issues during upgrade
	problematicVars := map[string]string{
		"tidb_config":        "Custom tidb_config may conflict with new version defaults",
		"tidb_slow_log_file": "Custom slow log file configuration may need adjustment",
	}

	for varName, warningMsg := range problematicVars {
		if _, exists := instance.State.Config[varName]; exists {
			results = append(results, CheckResult{
				RuleID:      r.RuleID(),
				Description: r.Description(),
				Severity:    "warning",
				Message:     fmt.Sprintf("Configured system variable may affect upgrade: %s", varName),
				Details:     warningMsg,
			})
		}
	}

	return results
}