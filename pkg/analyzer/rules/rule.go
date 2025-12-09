// Package rules provides standardized rule definitions for upgrade precheck
// Each rule is a minimal logical unit that performs a specific comparison
package rules

import (
	"context"
)

// DataSourceRequirement defines what data a rule needs from source cluster and knowledge base
// This allows the analyzer to load only the necessary data
type DataSourceRequirement struct {
	// SourceClusterRequirements defines what data is needed from the running cluster
	SourceClusterRequirements struct {
		// Components specifies which components' data is needed (tidb, pd, tikv, tiflash)
		Components []string `json:"components"`
		// NeedConfig indicates if configuration parameters are needed
		NeedConfig bool `json:"need_config"`
		// NeedSystemVariables indicates if system variables are needed (mainly for TiDB)
		NeedSystemVariables bool `json:"need_system_variables"`
		// NeedAllTikvNodes indicates if all TiKV nodes' data is needed (for consistency checks)
		NeedAllTikvNodes bool `json:"need_all_tikv_nodes"`
	} `json:"source_cluster_requirements"`

	// SourceKBRequirements defines what data is needed from source version knowledge base
	SourceKBRequirements struct {
		// Components specifies which components' knowledge base is needed
		Components []string `json:"components"`
		// NeedConfigDefaults indicates if config defaults are needed
		NeedConfigDefaults bool `json:"need_config_defaults"`
		// NeedSystemVariables indicates if system variable defaults are needed
		NeedSystemVariables bool `json:"need_system_variables"`
		// NeedUpgradeLogic indicates if upgrade logic is needed
		NeedUpgradeLogic bool `json:"need_upgrade_logic"`
	} `json:"source_kb_requirements"`

	// TargetKBRequirements defines what data is needed from target version knowledge base
	TargetKBRequirements struct {
		// Components specifies which components' knowledge base is needed
		Components []string `json:"components"`
		// NeedConfigDefaults indicates if config defaults are needed
		NeedConfigDefaults bool `json:"need_config_defaults"`
		// NeedSystemVariables indicates if system variable defaults are needed
		NeedSystemVariables bool `json:"need_system_variables"`
		// NeedUpgradeLogic indicates if upgrade logic is needed (for forced changes)
		NeedUpgradeLogic bool `json:"need_upgrade_logic"`
	} `json:"target_kb_requirements"`
}

// Rule defines the standard interface for upgrade precheck rules
// Each rule is a minimal logical unit that performs a specific comparison
type Rule interface {
	// Name returns a unique identifier for this rule
	Name() string

	// Description returns a human-readable description of what this rule checks
	Description() string

	// Category returns the category/group of this rule (e.g., "user_modified", "upgrade_difference", "consistency")
	Category() string

	// DataRequirements returns what data this rule needs from source cluster and knowledge base
	// This allows the analyzer to load only the necessary data
	DataRequirements() DataSourceRequirement

	// Evaluate performs the rule check by comparing source and target data
	// ruleCtx contains the loaded data based on DataRequirements
	// Returns a list of CheckResult items, each representing a finding from this rule
	Evaluate(ctx context.Context, ruleCtx *RuleContext) ([]CheckResult, error)
}

// BaseRule provides a base implementation for common rule functionality
type BaseRule struct {
	name        string
	description string
	category    string
}

// NewBaseRule creates a new base rule
func NewBaseRule(name, description, category string) *BaseRule {
	return &BaseRule{
		name:        name,
		description: description,
		category:    category,
	}
}

// Name returns the rule name
func (r *BaseRule) Name() string {
	return r.name
}

// Description returns the rule description
func (r *BaseRule) Description() string {
	return r.description
}

// Category returns the rule category
func (r *BaseRule) Category() string {
	return r.category
}
