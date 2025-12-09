package rules

import (
	"fmt"

	"github.com/pingcap/tidb-upgrade-precheck/pkg/runtime"
)

// ConfigCheckRule checks for configuration inconsistencies across cluster nodes
type ConfigCheckRule struct{}

// NewConfigCheckRule creates a new config check rule
func NewConfigCheckRule() *ConfigCheckRule {
	return &ConfigCheckRule{}
}

// RuleID returns the rule ID
func (r *ConfigCheckRule) RuleID() string {
	return "CONFIG_CHECK"
}

// Description returns the rule description
func (r *ConfigCheckRule) Description() string {
	return "Check for configuration inconsistencies across cluster nodes"
}

// Check performs the configuration consistency check
func (r *ConfigCheckRule) Check(snapshot *runtime.ClusterState) ([]CheckResult, error) {
	var results []CheckResult

	// Group instances by component type
	tidbInstances := []runtime.InstanceState{}
	pdInstances := []runtime.InstanceState{}
	tikvInstances := []runtime.InstanceState{}

	for _, instance := range snapshot.Instances {
		switch instance.State.Type {
		case runtime.TiDBComponent:
			tidbInstances = append(tidbInstances, instance)
		case runtime.PDComponent:
			pdInstances = append(pdInstances, instance)
		case runtime.TiKVComponent:
			tikvInstances = append(tikvInstances, instance)
		}
	}

	// Check TiDB configuration consistency
	if len(tidbInstances) > 1 {
		tidbResults := r.checkComponentConsistency(tidbInstances, "TiDB")
		results = append(results, tidbResults...)
	}

	// Check PD configuration consistency
	if len(pdInstances) > 1 {
		pdResults := r.checkComponentConsistency(pdInstances, "PD")
		results = append(results, pdResults...)
	}

	// Check TiKV configuration consistency
	if len(tikvInstances) > 1 {
		tikvResults := r.checkComponentConsistency(tikvInstances, "TiKV")
		results = append(results, tikvResults...)
	}

	return results, nil
}

// checkComponentConsistency checks configuration consistency for a specific component type
func (r *ConfigCheckRule) checkComponentConsistency(instances []runtime.InstanceState, componentName string) []CheckResult {
	var results []CheckResult

	// Create a map of parameter name to values across instances
	paramValues := make(map[string][]interface{})

	for _, instance := range instances {
		for paramName, paramValue := range instance.State.Config {
			paramValues[paramName] = append(paramValues[paramName], paramValue)
		}
	}

	// Check each parameter for inconsistencies
	for paramName, values := range paramValues {
		if len(values) != len(instances) {
			// Parameter doesn't exist on all instances
			results = append(results, CheckResult{
				RuleID:      r.RuleID(),
				Description: r.Description(),
				Severity:    "warning",
				Message:     fmt.Sprintf("Parameter %s is not configured on all %s instances", paramName, componentName),
				Details:     fmt.Sprintf("Parameter %s missing on some instances", paramName),
			})
			continue
		}

		// Check if all values are the same
		firstValue := values[0]
		isConsistent := true
		for _, value := range values[1:] {
			if firstValue != value {
				isConsistent = false
				break
			}
		}

		if !isConsistent {
			results = append(results, CheckResult{
				RuleID:      r.RuleID(),
				Description: r.Description(),
				Severity:    "warning",
				Message:     fmt.Sprintf("Inconsistent values for parameter %s across %s instances", paramName, componentName),
				Details:     fmt.Sprintf("Parameter %s has different values on different instances", paramName),
			})
		}
	}

	return results
}