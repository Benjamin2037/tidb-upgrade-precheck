package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

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

// promptInput prompts user for input and returns the result
func promptInput(prompt string, defaultValue string) string {
	reader := bufio.NewReader(os.Stdin)
	fmt.Print(prompt)
	if defaultValue != "" {
		fmt.Printf(" [%s]: ", defaultValue)
	} else {
		fmt.Print(": ")
	}

	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)

	if input == "" && defaultValue != "" {
		return defaultValue
	}

	return input
}

// promptYesNo prompts user for yes/no input
func promptYesNo(prompt string, defaultValue bool) bool {
	defaultStr := "n"
	if defaultValue {
		defaultStr = "y"
	}

	input := promptInput(prompt+" (y/n)", defaultStr)
	return strings.ToLower(input) == "y" || strings.ToLower(input) == "yes"
}

// promptSelect prompts user to select from options
func promptSelect(prompt string, options []string, defaultValue string) string {
	fmt.Println(prompt)
	for i, opt := range options {
		marker := " "
		if opt == defaultValue {
			marker = "*"
		}
		fmt.Printf("  %s [%d] %s\n", marker, i+1, opt)
	}

	input := promptInput("Select option", defaultValue)
	if idx, err := strconv.Atoi(input); err == nil && idx > 0 && idx <= len(options) {
		return options[idx-1]
	}

	// Try to match by name
	for _, opt := range options {
		if strings.EqualFold(opt, input) {
			return opt
		}
	}

	return defaultValue
}

// newHighRiskParamsCmd creates the high-risk-params subcommand group
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
