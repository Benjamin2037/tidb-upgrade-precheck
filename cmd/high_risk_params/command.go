package main

import (
	"github.com/pingcap/tidb-upgrade-precheck/pkg/analyzer/rules"
	"github.com/pingcap/tidb-upgrade-precheck/pkg/analyzer/rules/high_risk_params"
	"github.com/spf13/cobra"
)

var highRiskParamsConfigFile string

// getConfigManager returns a configuration manager instance
func getConfigManager() *high_risk_params.Manager {
	configPath := highRiskParamsConfigFile
	if configPath == "" {
		configPath = high_risk_params.GetDefaultConfigPath()
	}
	return high_risk_params.NewManager(configPath)
}

// loadHighRiskParamsConfig loads the configuration using the manager
func loadHighRiskParamsConfig() (*rules.HighRiskParamsConfig, error) {
	manager := getConfigManager()
	return manager.LoadConfig()
}

// saveHighRiskParamsConfig saves the configuration using the manager
func saveHighRiskParamsConfig(config *rules.HighRiskParamsConfig) error {
	manager := getConfigManager()
	return manager.SaveConfig(config)
}

// newHighRiskParamsCmd creates the high-risk-params root command
func newHighRiskParamsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "high-risk-params",
		Short: "Manage high-risk parameters configuration",
		Long: `Manage high-risk parameters configuration for upgrade precheck.

This command allows you to add, view, edit, and remove high-risk parameters
for each component (TiDB, PD, TiKV, TiFlash). The configuration is saved to a
JSON file that can be used by the upgrade precheck tool.`,
	}

	// Add config file flag to all subcommands
	cmd.PersistentFlags().StringVar(&highRiskParamsConfigFile, "config", "", "Path to high-risk parameters configuration file (default: ~/.tiup/high_risk_params.json)")

	// Add subcommands
	cmd.AddCommand(newHighRiskParamsAddCmd())
	cmd.AddCommand(newHighRiskParamsListCmd())
	cmd.AddCommand(newHighRiskParamsRemoveCmd())
	cmd.AddCommand(newHighRiskParamsViewCmd())
	cmd.AddCommand(newHighRiskParamsEditCmd())

	return cmd
}
