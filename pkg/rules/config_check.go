package rules

import (
	"context"
	"fmt"

	"github.com/pingcap/tidb-upgrade-precheck/pkg/precheck"
)

// ConfigCheckRule checks for configuration-related issues
type ConfigCheckRule struct {
	sourceKB map[string]interface{}
	targetKB map[string]interface{}
}

// NewConfigCheckRule creates a new config check rule
func NewConfigCheckRule(sourceKB, targetKB map[string]interface{}) precheck.Rule {
	return &ConfigCheckRule{
		sourceKB: sourceKB,
		targetKB: targetKB,
	}
}

// Name returns the rule name
func (r *ConfigCheckRule) Name() string {
	return "config-check"
}

// Evaluate evaluates the rule against a snapshot
func (r *ConfigCheckRule) Evaluate(ctx context.Context, snapshot precheck.Snapshot) ([]precheck.ReportItem, error) {
	var items []precheck.ReportItem

	// Check TiDB config parameters
	if tidbSnapshot, exists := snapshot.Components["tidb"]; exists {
		items = append(items, r.checkTiDBConfig(tidbSnapshot)...)
	}

	return items, nil
}

func (r *ConfigCheckRule) checkTiDBConfig(snapshot precheck.ComponentSnapshot) []precheck.ReportItem {
	var items []precheck.ReportItem

	// Get source and target config defaults
	sourceConfigDefaults := make(map[string]interface{})
	targetConfigDefaults := make(map[string]interface{})

	if sourceConfig, ok := r.sourceKB["config_defaults"].(map[string]interface{}); ok {
		sourceConfigDefaults = sourceConfig
	}

	if targetConfig, ok := r.targetKB["config_defaults"].(map[string]interface{}); ok {
		targetConfigDefaults = targetConfig
	}

	// Check each config parameter
	for name, currentValue := range snapshot.Config {
		sourceDefault, sourceExists := sourceConfigDefaults[name]
		targetDefault, targetExists := targetConfigDefaults[name]

		// Check if parameter default changed
		if sourceExists && targetExists && fmt.Sprintf("%v", sourceDefault) != fmt.Sprintf("%v", targetDefault) {
			items = append(items, precheck.ReportItem{
				Rule:     "config-check",
				Severity: precheck.SeverityWarning,
				Message:  fmt.Sprintf("TiDB config parameter '%s' default value changed", name),
				Details: []string{
					fmt.Sprintf("Default value changed from '%v' to '%v'", sourceDefault, targetDefault),
				},
			})
		}

		// Check if user has customized the parameter
		if sourceExists && fmt.Sprintf("%v", currentValue) != fmt.Sprintf("%v", sourceDefault) {
			items = append(items, precheck.ReportItem{
				Rule:     "config-check",
				Severity: precheck.SeverityInfo,
				Message:  fmt.Sprintf("TiDB config parameter '%s' has custom value", name),
				Details: []string{
					fmt.Sprintf("Current value: '%v', default value: '%v'", currentValue, sourceDefault),
				},
			})
		}
	}

	return items
}

// ConfigChecker checks for configuration compatibility issues
type ConfigChecker struct {
	// Knowledge base data would be loaded here
	// For now, we'll use placeholder data
}

// NewConfigChecker creates a new configuration checker
func NewConfigChecker() Checker {
	return &ConfigChecker{}
}

// RuleID returns the unique identifier for this check rule
func (c *ConfigChecker) RuleID() string {
	return "CONFIG_CHECK"
}

// Description returns a brief description of what this rule checks
func (c *ConfigChecker) Description() string {
	return "Check for configuration compatibility issues"
}

// Check performs the configuration compatibility check
func (c *ConfigChecker) Check(snapshot *runtime.ClusterSnapshot) ([]CheckResult, error) {
	var results []CheckResult
	
	// Check TiDB configurations
	if tidbComponent, exists := snapshot.Components["tidb"]; exists {
		results = append(results, c.checkTiDBConfig(tidbComponent)...)
	}
	
	// Check TiKV configurations
	for name, component := range snapshot.Components {
		if component.Type == "tikv" {
			results = append(results, c.checkTiKVConfig(name, component)...)
		}
	}
	
	// Check PD configurations
	if pdComponent, exists := snapshot.Components["pd"]; exists {
		results = append(results, c.checkPDConfig(pdComponent)...)
	}
	
	return results, nil
}

func (c *ConfigChecker) checkTiDBConfig(component runtime.ComponentState) []CheckResult {
	var results []CheckResult
	
	// Example checks - in a real implementation, these would be based on knowledge base
	if val, exists := component.Config["performance.max-procs"]; exists {
		if valStr, ok := val.(string); ok && valStr == "0" {
			results = append(results, CheckResult{
				RuleID:      c.RuleID(),
				Description: c.Description(),
				Severity:    "warning",
				Message:     "performance.max-procs is set to 0",
				Details:     "Setting performance.max-procs to 0 will use all available CPU cores, which might not be optimal in containerized environments",
			})
		}
	}
	
	// Check for deprecated configurations
	if _, exists := component.Config["prepared-plan-cache.enabled"]; exists {
		results = append(results, CheckResult{
			RuleID:      c.RuleID(),
			Description: c.Description(),
			Severity:    "info",
			Message:     "prepared-plan-cache.enabled configuration detected",
			Details:     "prepared-plan-cache.enabled has been deprecated, use [performance] section instead",
		})
	}
	
	return results
}

func (c *ConfigChecker) checkTiKVConfig(name string, component runtime.ComponentState) []CheckResult {
	var results []CheckResult
	
	// Example checks - in a real implementation, these would be based on knowledge base
	if val, exists := component.Config["rocksdb.max-open-files"]; exists {
		if valFloat, ok := val.(float64); ok && valFloat < 1000 {
			results = append(results, CheckResult{
				RuleID:      c.RuleID(),
				Description: c.Description(),
				Severity:    "warning",
				Message:     fmt.Sprintf("rocksdb.max-open-files is set to %v", valFloat),
				Details:     "Setting rocksdb.max-open-files to a low value may impact performance",
			})
		}
	}
	
	return results
}

func (c *ConfigChecker) checkPDConfig(component runtime.ComponentState) []CheckResult {
	var results []CheckResult
	
	// Example checks - in a real implementation, these would be based on knowledge base
	if val, exists := component.Config["schedule.max-merge-region-size"]; exists {
		if valFloat, ok := val.(float64); ok && valFloat > 20 {
			results = append(results, CheckResult{
				RuleID:      c.RuleID(),
				Description: c.Description(),
				Severity:    "info",
				Message:     fmt.Sprintf("schedule.max-merge-region-size is set to %v", valFloat),
				Details:     "Large region merge size may impact online workload performance",
			})
		}
	}
	
	return results
}