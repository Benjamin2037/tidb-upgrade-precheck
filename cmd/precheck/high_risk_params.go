package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/pingcap/tidb-upgrade-precheck/pkg/analyzer/rules"
	"github.com/spf13/cobra"
)

var highRiskParamsConfigFile string

// getDefaultConfigPath returns the default path for high-risk params config
func getDefaultConfigPath() string {
	// Try to get from environment variable
	if path := os.Getenv("HIGH_RISK_PARAMS_CONFIG"); path != "" {
		return path
	}

	// Default locations (in order of preference)
	homeDir, err := os.UserHomeDir()
	if err == nil {
		// ~/.tiup/high_risk_params.json (for TiUP integration)
		tiupPath := filepath.Join(homeDir, ".tiup", "high_risk_params.json")
		if _, err := os.Stat(tiupPath); err == nil {
			return tiupPath
		}

		// ~/.tidb-upgrade-precheck/high_risk_params.json
		precheckPath := filepath.Join(homeDir, ".tidb-upgrade-precheck", "high_risk_params.json")
		if _, err := os.Stat(precheckPath); err == nil {
			return precheckPath
		}

		// Return TiUP path as default (will be created if doesn't exist)
		return tiupPath
	}

	// Fallback to current directory
	return "./high_risk_params.json"
}

// loadConfig loads the high-risk parameters configuration from file
func loadHighRiskParamsConfig() (*rules.HighRiskParamsConfig, error) {
	configFile := highRiskParamsConfigFile
	if configFile == "" {
		configFile = getDefaultConfigPath()
	}

	config := &rules.HighRiskParamsConfig{}

	// Check if file exists
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		// File doesn't exist, return empty config
		return config, nil
	}

	data, err := os.ReadFile(configFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	if len(data) == 0 {
		// Empty file, return empty config
		return config, nil
	}

	if err := json.Unmarshal(data, config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	return config, nil
}

// saveConfig saves the high-risk parameters configuration to file
func saveHighRiskParamsConfig(config *rules.HighRiskParamsConfig) error {
	configFile := highRiskParamsConfigFile
	if configFile == "" {
		configFile = getDefaultConfigPath()
	}

	// Create directory if it doesn't exist
	dir := filepath.Dir(configFile)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(configFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
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
