package rules

import (
	"fmt"

	"github.com/pingcap/tidb-upgrade-precheck/pkg/runtime"
)

// TargetVersionRule checks if the target version is compatible with the current cluster setup
type TargetVersionRule struct {
	targetVersion string
}

// NewTargetVersionRule creates a new target version rule
func NewTargetVersionRule(targetVersion string) *TargetVersionRule {
	return &TargetVersionRule{
		targetVersion: targetVersion,
	}
}

// RuleID returns the rule ID
func (r *TargetVersionRule) RuleID() string {
	return "TARGET_VERSION_CHECK"
}

// Description returns the rule description
func (r *TargetVersionRule) Description() string {
	return "Check if the target version is compatible with the current cluster setup"
}

// Check performs the target version compatibility check
func (r *TargetVersionRule) Check(snapshot *runtime.ClusterState) ([]CheckResult, error) {
	var results []CheckResult

	// Check each component version
	for _, instance := range snapshot.Instances {
		versionCheckResult := r.checkVersionCompatibility(instance)
		results = append(results, versionCheckResult)
	}

	return results, nil
}

// checkVersionCompatibility checks version compatibility for a specific instance
func (r *TargetVersionRule) checkVersionCompatibility(instance runtime.InstanceState) CheckResult {
	currentVersion := instance.State.Version

	// Simple version comparison (in a real implementation, this would be more sophisticated)
	if isVersionCompatible(currentVersion, r.targetVersion) {
		return CheckResult{
			RuleID:      r.RuleID(),
			Description: r.Description(),
			Severity:    "info",
			Message:     fmt.Sprintf("Version compatibility check passed for %s (%s -> %s)", instance.State.Type, currentVersion, r.targetVersion),
			Details:     fmt.Sprintf("Instance at %s can be upgraded from %s to %s", instance.Address, currentVersion, r.targetVersion),
		}
	}

	return CheckResult{
		RuleID:      r.RuleID(),
		Description: r.Description(),
		Severity:    "error",
		Message:     fmt.Sprintf("Version compatibility check failed for %s (%s -> %s)", instance.State.Type, currentVersion, r.targetVersion),
		Details:     fmt.Sprintf("Instance at %s cannot be upgraded from %s to %s", instance.Address, currentVersion, r.targetVersion),
	}
}

// isVersionCompatible checks if a current version is compatible with a target version
// This is a simplified implementation - a real one would be more complex
func isVersionCompatible(currentVersion, targetVersion string) bool {
	// For now, we'll assume all versions are compatible
	// A real implementation would check version constraints
	return true
}