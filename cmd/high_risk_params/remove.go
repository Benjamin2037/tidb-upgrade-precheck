package main

import (
	"fmt"
	"strings"

	"github.com/pingcap/tidb-upgrade-precheck/pkg/analyzer/rules/high_risk_params"
	"github.com/spf13/cobra"
)

func newHighRiskParamsRemoveCmd() *cobra.Command {
	var (
		component string
		paramType string
		paramName string
	)

	cmd := &cobra.Command{
		Use:   "remove",
		Short: "Remove a high-risk parameter",
		Long: `Remove a high-risk parameter from the configuration.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if component == "" || paramType == "" || paramName == "" {
				return fmt.Errorf("component, type, and name are required")
			}

			config, err := loadHighRiskParamsConfig()
			if err != nil {
				return err
			}

			component = strings.ToLower(component)
			paramType = strings.ToLower(paramType)

			var removed bool

			switch component {
			case "tidb":
				if paramType == "config" {
					if _, exists := config.TiDB.Config[paramName]; exists {
						delete(config.TiDB.Config, paramName)
						removed = true
					}
				} else if paramType == "system_variable" || paramType == "system-variable" || paramType == "sysvar" {
					if _, exists := config.TiDB.SystemVariables[paramName]; exists {
						delete(config.TiDB.SystemVariables, paramName)
						removed = true
					}
				} else {
					return fmt.Errorf("invalid type for TiDB: %s", paramType)
				}
			case "pd":
				if paramType == "config" {
					if _, exists := config.PD.Config[paramName]; exists {
						delete(config.PD.Config, paramName)
						removed = true
					}
				} else {
					return fmt.Errorf("PD only supports 'config' type")
				}
			case "tikv":
				if paramType == "config" {
					if _, exists := config.TiKV.Config[paramName]; exists {
						delete(config.TiKV.Config, paramName)
						removed = true
					}
				} else {
					return fmt.Errorf("TiKV only supports 'config' type")
				}
			case "tiflash":
				if paramType == "config" {
					if _, exists := config.TiFlash.Config[paramName]; exists {
						delete(config.TiFlash.Config, paramName)
						removed = true
					}
				} else {
					return fmt.Errorf("TiFlash only supports 'config' type")
				}
			default:
				return fmt.Errorf("invalid component: %s", component)
			}

			if !removed {
				return fmt.Errorf("parameter %s/%s/%s not found", component, paramType, paramName)
			}

			// Save config
			if err := saveHighRiskParamsConfig(config); err != nil {
				return err
			}

			configFile := highRiskParamsConfigFile
			if configFile == "" {
				configFile = high_risk_params.GetDefaultConfigPath()
			}

			fmt.Printf("Successfully removed high-risk parameter: %s/%s/%s\n", component, paramType, paramName)
			fmt.Printf("Configuration saved to: %s\n", configFile)

			return nil
		},
	}

	cmd.Flags().StringVar(&component, "component", "", "Component name (tidb, pd, tikv, tiflash)")
	cmd.Flags().StringVar(&paramType, "type", "", "Parameter type (config or system_variable)")
	cmd.Flags().StringVar(&paramName, "name", "", "Parameter name")
	cmd.MarkFlagRequired("component")
	cmd.MarkFlagRequired("type")
	cmd.MarkFlagRequired("name")

	return cmd
}

