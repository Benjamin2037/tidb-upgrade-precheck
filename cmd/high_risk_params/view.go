package main

import (
	"encoding/json"
	"fmt"

	"github.com/pingcap/tidb-upgrade-precheck/pkg/analyzer/rules"
	"github.com/pingcap/tidb-upgrade-precheck/pkg/analyzer/rules/high_risk_params"
	"github.com/spf13/cobra"
)

func newHighRiskParamsViewCmd() *cobra.Command {
	var (
		component string
		paramType string
		paramName string
	)

	cmd := &cobra.Command{
		Use:   "view",
		Short: "View high-risk parameters configuration",
		Long: `View the high-risk parameters configuration.

If component, type, and name are provided, view a specific parameter.
Otherwise, view the entire configuration in JSON format.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			config, err := loadHighRiskParamsConfig()
			if err != nil {
				return err
			}

			// If specific parameter requested
			if component != "" && paramType != "" && paramName != "" {
				param, found := findParameterInConfig(config, component, paramType, paramName)
				if !found {
					return fmt.Errorf("parameter %s/%s/%s not found", component, paramType, paramName)
				}

				// Display parameter details
				fmt.Printf("\nParameter Details:\n")
				fmt.Printf("==================\n")
				fmt.Printf("Parameter Name: %s\n", paramName)
				fmt.Printf("Component:      %s\n", component)
				fmt.Printf("Type:           %s\n", paramType)
				if param.Severity != "" {
					fmt.Printf("Severity:       %s\n", param.Severity)
				}
				if param.Description != "" {
					fmt.Printf("Description:    %s\n", param.Description)
				}
				if param.FromVersion != "" {
					fmt.Printf("From Version:   %s\n", param.FromVersion)
				}
				if param.ToVersion != "" {
					fmt.Printf("To Version:     %s\n", param.ToVersion)
				}
				fmt.Printf("Check Modified: %v\n", param.CheckModified)
				if len(param.AllowedValues) > 0 {
					fmt.Printf("Allowed Values: %v\n", param.AllowedValues)
				}
				fmt.Println()
				return nil
			}

			// Otherwise, display entire config
			data, err := json.MarshalIndent(config, "", "  ")
			if err != nil {
				return err
			}

			fmt.Println(string(data))
			return nil
		},
	}

	cmd.Flags().StringVar(&component, "component", "", "Component name (tidb, pd, tikv, tiflash)")
	cmd.Flags().StringVar(&paramType, "type", "", "Parameter type (config or system_variable)")
	cmd.Flags().StringVar(&paramName, "name", "", "Parameter name")

	return cmd
}

// findParameterInConfig finds a parameter in the config using the manager
func findParameterInConfig(config *rules.HighRiskParamsConfig, component, paramType, paramName string) (rules.HighRiskParamConfig, bool) {
	return high_risk_params.FindParameterInConfig(config, component, paramType, paramName)
}
