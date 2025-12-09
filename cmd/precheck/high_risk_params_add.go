package main

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/pingcap/tidb-upgrade-precheck/pkg/analyzer/rules"
	"github.com/spf13/cobra"
)

func newHighRiskParamsAddCmd() *cobra.Command {
	var (
		component     string
		paramType     string
		paramName     string
		severity      string
		description   string
		checkModified bool
		fromVersion   string
		toVersion     string
		allowedValues []string
		interactive   bool
	)

	cmd := &cobra.Command{
		Use:   "add",
		Short: "Add a high-risk parameter",
		Long: `Add a high-risk parameter to the configuration.

You can either use interactive mode (default) or provide all parameters via command line flags.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			config, err := loadHighRiskParamsConfig()
			if err != nil {
				return err
			}

			var paramConfig rules.HighRiskParamConfig

			if interactive || (component == "" && paramType == "" && paramName == "") {
				// Interactive mode
				paramConfig, component, paramType, paramName, err = promptAddParameter()
				if err != nil {
					return err
				}
			} else {
				// Command line mode
				paramConfig = rules.HighRiskParamConfig{
					Severity:      severity,
					Description:   description,
					CheckModified: checkModified,
					FromVersion:   fromVersion,
					ToVersion:     toVersion,
				}

				// Parse allowed values
				if len(allowedValues) > 0 {
					paramConfig.AllowedValues = make([]interface{}, len(allowedValues))
					for i, v := range allowedValues {
						// Try to parse as number first
						if intVal, err := strconv.ParseInt(v, 10, 64); err == nil {
							paramConfig.AllowedValues[i] = intVal
						} else if floatVal, err := strconv.ParseFloat(v, 64); err == nil {
							paramConfig.AllowedValues[i] = floatVal
						} else if boolVal, err := strconv.ParseBool(v); err == nil {
							paramConfig.AllowedValues[i] = boolVal
						} else {
							paramConfig.AllowedValues[i] = v
						}
					}
				}

				// Validate required fields
				if component == "" {
					return fmt.Errorf("component is required")
				}
				if paramType == "" {
					return fmt.Errorf("type is required (config or system_variable)")
				}
				if paramName == "" {
					return fmt.Errorf("parameter name is required")
				}
				if severity == "" {
					return fmt.Errorf("severity is required (error, warning, or info)")
				}
			}

			// Add to config
			component = strings.ToLower(component)
			paramType = strings.ToLower(paramType)

			switch component {
			case "tidb":
				if paramType == "config" {
					if config.TiDB.Config == nil {
						config.TiDB.Config = make(map[string]rules.HighRiskParamConfig)
					}
					config.TiDB.Config[paramName] = paramConfig
				} else if paramType == "system_variable" || paramType == "system-variable" || paramType == "sysvar" {
					if config.TiDB.SystemVariables == nil {
						config.TiDB.SystemVariables = make(map[string]rules.HighRiskParamConfig)
					}
					config.TiDB.SystemVariables[paramName] = paramConfig
				} else {
					return fmt.Errorf("invalid type for TiDB: %s (must be 'config' or 'system_variable')", paramType)
				}
			case "pd":
				if paramType == "config" {
					if config.PD.Config == nil {
						config.PD.Config = make(map[string]rules.HighRiskParamConfig)
					}
					config.PD.Config[paramName] = paramConfig
				} else {
					return fmt.Errorf("PD only supports 'config' type")
				}
			case "tikv":
				if paramType == "config" {
					if config.TiKV.Config == nil {
						config.TiKV.Config = make(map[string]rules.HighRiskParamConfig)
					}
					config.TiKV.Config[paramName] = paramConfig
				} else {
					return fmt.Errorf("TiKV only supports 'config' type")
				}
			case "tiflash":
				if paramType == "config" {
					if config.TiFlash.Config == nil {
						config.TiFlash.Config = make(map[string]rules.HighRiskParamConfig)
					}
					config.TiFlash.Config[paramName] = paramConfig
				} else {
					return fmt.Errorf("TiFlash only supports 'config' type")
				}
			default:
				return fmt.Errorf("invalid component: %s (must be tidb, pd, tikv, or tiflash)", component)
			}

			// Save config
			if err := saveHighRiskParamsConfig(config); err != nil {
				return err
			}

			configFile := highRiskParamsConfigFile
			if configFile == "" {
				configFile = getDefaultConfigPath()
			}

			fmt.Printf("Successfully added high-risk parameter: %s/%s/%s\n", component, paramType, paramName)
			fmt.Printf("Configuration saved to: %s\n", configFile)

			return nil
		},
	}

	cmd.Flags().StringVar(&component, "component", "", "Component name (tidb, pd, tikv, tiflash)")
	cmd.Flags().StringVar(&paramType, "type", "", "Parameter type (config or system_variable)")
	cmd.Flags().StringVar(&paramName, "name", "", "Parameter name")
	cmd.Flags().StringVar(&severity, "severity", "", "Severity level (error, warning, info)")
	cmd.Flags().StringVar(&description, "description", "", "Description of why this parameter is high-risk")
	cmd.Flags().BoolVar(&checkModified, "check-modified", false, "Only check if parameter is modified from default")
	cmd.Flags().StringVar(&fromVersion, "from-version", "", "From version (e.g., v7.0.0)")
	cmd.Flags().StringVar(&toVersion, "to-version", "", "To version (e.g., v7.5.0)")
	cmd.Flags().StringSliceVar(&allowedValues, "allowed-values", []string{}, "Allowed values (comma-separated)")
	cmd.Flags().BoolVarP(&interactive, "interactive", "i", true, "Use interactive mode")

	return cmd
}

// promptAddParameter prompts user to add a parameter interactively
func promptAddParameter() (rules.HighRiskParamConfig, string, string, string, error) {
	var config rules.HighRiskParamConfig

	// Component
	component := promptSelect("Select component:", []string{"tidb", "pd", "tikv", "tiflash"}, "tidb")

	// Parameter type
	var paramTypeOptions []string
	if component == "tidb" {
		paramTypeOptions = []string{"config", "system_variable"}
	} else {
		paramTypeOptions = []string{"config"}
	}
	paramType := promptSelect("Select parameter type:", paramTypeOptions, "config")

	// Parameter name
	paramName := promptInput("Enter parameter name", "")
	if paramName == "" {
		return config, "", "", "", fmt.Errorf("parameter name is required")
	}

	// Severity
	severity := promptSelect("Select severity:", []string{"error", "warning", "info"}, "warning")

	// Description
	description := promptInput("Enter description (optional)", "")

	// Check modified
	checkModified := promptYesNo("Only check if parameter is modified from default?", false)

	// From version
	fromVersion := promptInput("From version (e.g., v7.0.0, optional)", "")

	// To version
	toVersion := promptInput("To version (e.g., v7.5.0, optional)", "")

	// Allowed values
	var allowedValues []interface{}
	if promptYesNo("Specify allowed values?", false) {
		for {
			value := promptInput("Enter allowed value (empty to finish)", "")
			if value == "" {
				break
			}
			// Try to parse as number first
			if intVal, err := strconv.ParseInt(value, 10, 64); err == nil {
				allowedValues = append(allowedValues, intVal)
			} else if floatVal, err := strconv.ParseFloat(value, 64); err == nil {
				allowedValues = append(allowedValues, floatVal)
			} else if boolVal, err := strconv.ParseBool(value); err == nil {
				allowedValues = append(allowedValues, boolVal)
			} else {
				allowedValues = append(allowedValues, value)
			}
		}
	}

	config = rules.HighRiskParamConfig{
		Severity:      severity,
		Description:   description,
		CheckModified: checkModified,
		FromVersion:   fromVersion,
		ToVersion:     toVersion,
		AllowedValues: allowedValues,
	}

	return config, component, paramType, paramName, nil
}
