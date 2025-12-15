package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/pingcap/tidb-upgrade-precheck/pkg/analyzer/rules"
	"github.com/spf13/cobra"
)

func newHighRiskParamsListCmd() *cobra.Command {
	var (
		component string
		format    string
	)

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all high-risk parameters",
		Long: `List all high-risk parameters in the configuration.

You can filter by component and choose output format (table or json).`,
		RunE: func(cmd *cobra.Command, args []string) error {
			config, err := loadHighRiskParamsConfig()
			if err != nil {
				return err
			}

			if format == "json" {
				// JSON output
				var output interface{} = config
				if component != "" {
					// Filter by component
					switch component {
					case "tidb":
						output = config.TiDB
					case "pd":
						output = config.PD
					case "tikv":
						output = config.TiKV
					case "tiflash":
						output = config.TiFlash
					default:
						return fmt.Errorf("invalid component: %s", component)
					}
				}

				data, err := json.MarshalIndent(output, "", "  ")
				if err != nil {
					return err
				}
				fmt.Println(string(data))
				return nil
			}

			// Table output
			printHighRiskParamsTable(config, component)

			return nil
		},
	}

	cmd.Flags().StringVar(&component, "component", "", "Filter by component (tidb, pd, tikv, tiflash)")
	cmd.Flags().StringVar(&format, "format", "table", "Output format (table or json)")

	return cmd
}

func printHighRiskParamsTable(config *rules.HighRiskParamsConfig, filterComponent string) {
	components := []string{"tidb", "pd", "tikv", "tiflash"}
	if filterComponent != "" {
		components = []string{filterComponent}
	}

	for _, comp := range components {
		var hasParams bool

		switch comp {
		case "tidb":
			if len(config.TiDB.Config) > 0 || len(config.TiDB.SystemVariables) > 0 {
				hasParams = true
				fmt.Printf("\n=== TiDB ===\n\n")
				if len(config.TiDB.Config) > 0 {
					fmt.Println("Config Parameters:")
					for name, param := range config.TiDB.Config {
						printHighRiskParamInfo("  ", name, "config", param)
					}
					fmt.Println()
				}
				if len(config.TiDB.SystemVariables) > 0 {
					fmt.Println("System Variables:")
					for name, param := range config.TiDB.SystemVariables {
						printHighRiskParamInfo("  ", name, "system_variable", param)
					}
					fmt.Println()
				}
			}
		case "pd":
			if len(config.PD.Config) > 0 {
				hasParams = true
				fmt.Printf("\n=== PD ===\n\n")
				for name, param := range config.PD.Config {
					printHighRiskParamInfo("  ", name, "config", param)
				}
				fmt.Println()
			}
		case "tikv":
			if len(config.TiKV.Config) > 0 {
				hasParams = true
				fmt.Printf("\n=== TiKV ===\n\n")
				for name, param := range config.TiKV.Config {
					printHighRiskParamInfo("  ", name, "config", param)
				}
				fmt.Println()
			}
		case "tiflash":
			if len(config.TiFlash.Config) > 0 {
				hasParams = true
				fmt.Printf("\n=== TiFlash ===\n\n")
				for name, param := range config.TiFlash.Config {
					printHighRiskParamInfo("  ", name, "config", param)
				}
				fmt.Println()
			}
		}

		if !hasParams && filterComponent == comp {
			fmt.Printf("\n=== %s ===\n\n", strings.ToUpper(comp))
			fmt.Println("  No high-risk parameters configured.")
			fmt.Println()
		}
	}
}

func printHighRiskParamInfo(indent, name, paramType string, param rules.HighRiskParamConfig) {
	fmt.Printf("%s%s (%s)\n", indent, name, paramType)
	fmt.Printf("%s  Severity: %s\n", indent, param.Severity)
	if param.Description != "" {
		fmt.Printf("%s  Description: %s\n", indent, param.Description)
	}
	if param.FromVersion != "" {
		fmt.Printf("%s  From Version: %s\n", indent, param.FromVersion)
	}
	if param.ToVersion != "" {
		fmt.Printf("%s  To Version: %s\n", indent, param.ToVersion)
	}
	if param.CheckModified {
		fmt.Printf("%s  Check Modified: true\n", indent)
	}
	if len(param.AllowedValues) > 0 {
		fmt.Printf("%s  Allowed Values: %v\n", indent, param.AllowedValues)
	}
	fmt.Println()
}
