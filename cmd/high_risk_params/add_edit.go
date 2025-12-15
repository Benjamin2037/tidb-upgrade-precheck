package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/pingcap/tidb-upgrade-precheck/pkg/analyzer/rules"
	"github.com/pingcap/tidb-upgrade-precheck/pkg/analyzer/rules/high_risk_params"
	"github.com/spf13/cobra"
)

// addOrEditParameter adds or edits a high-risk parameter
// isAdd: true for add (must not exist), false for edit (must exist)
func addOrEditParameter(component, paramType, paramName string, paramConfig rules.HighRiskParamConfig, isAdd bool) error {
	manager := getConfigManager()

	// Normalize inputs
	component = strings.ToLower(component)
	paramType = strings.ToLower(paramType)

	// Check if parameter exists
	_, exists := manager.FindParameter(component, paramType, paramName)

	if isAdd {
		// Add: parameter must not exist
		if exists {
			return fmt.Errorf("parameter %s/%s/%s already exists. Use 'edit' command to modify it", component, paramType, paramName)
		}
	} else {
		// Edit: parameter must exist
		if !exists {
			return fmt.Errorf("parameter %s/%s/%s not found. Use 'add' command to create it", component, paramType, paramName)
		}
	}

	// Add or update the parameter
	return manager.AddParameter(component, paramType, paramName, paramConfig)
}

// parseAllowedValues parses string slice to interface slice
func parseAllowedValues(allowedValues []string) []interface{} {
	if len(allowedValues) == 0 {
		return nil
	}

	result := make([]interface{}, len(allowedValues))
	for i, v := range allowedValues {
		// Try to parse as number first
		if intVal, err := strconv.ParseInt(v, 10, 64); err == nil {
			result[i] = intVal
		} else if floatVal, err := strconv.ParseFloat(v, 64); err == nil {
			result[i] = floatVal
		} else if boolVal, err := strconv.ParseBool(v); err == nil {
			result[i] = boolVal
		} else {
			result[i] = v
		}
	}
	return result
}

// buildParamConfigFromFlags builds HighRiskParamConfig from command line flags
func buildParamConfigFromFlags(severity, description string, checkModified bool, fromVersion, toVersion string, allowedValues []string) rules.HighRiskParamConfig {
	return rules.HighRiskParamConfig{
		Severity:      severity,
		Description:   description,
		CheckModified: checkModified,
		FromVersion:   fromVersion,
		ToVersion:     toVersion,
		AllowedValues: parseAllowedValues(allowedValues),
	}
}

// validateRequiredFields validates required fields for add/edit
func validateRequiredFields(component, paramType, paramName, severity string) error {
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
	return nil
}

// ParameterInput represents the structure for file input
type ParameterInput struct {
	Component     string        `json:"component"`
	Type          string        `json:"type"`
	Name          string        `json:"name"`
	Severity      string        `json:"severity"`
	Description   string        `json:"description,omitempty"`
	CheckModified bool          `json:"check_modified,omitempty"`
	FromVersion   string        `json:"from_version,omitempty"`
	ToVersion     string        `json:"to_version,omitempty"`
	AllowedValues []interface{} `json:"allowed_values,omitempty"`
}

// loadParameterFromFile loads parameter configuration from a JSON file
func loadParameterFromFile(filePath string) (rules.HighRiskParamConfig, string, string, string, error) {
	var input ParameterInput
	var config rules.HighRiskParamConfig

	data, err := os.ReadFile(filePath)
	if err != nil {
		return config, "", "", "", fmt.Errorf("failed to read file: %w", err)
	}

	if err := json.Unmarshal(data, &input); err != nil {
		return config, "", "", "", fmt.Errorf("failed to parse JSON: %w", err)
	}

	// Validate required fields
	if err := validateRequiredFields(input.Component, input.Type, input.Name, input.Severity); err != nil {
		return config, "", "", "", err
	}

	config = rules.HighRiskParamConfig{
		Severity:      input.Severity,
		Description:   input.Description,
		CheckModified: input.CheckModified,
		FromVersion:   input.FromVersion,
		ToVersion:     input.ToVersion,
		AllowedValues: input.AllowedValues,
	}

	return config, input.Component, input.Type, input.Name, nil
}

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
		inputFile     string
	)

	cmd := &cobra.Command{
		Use:   "add",
		Short: "Add a high-risk parameter",
		Long: `Add a high-risk parameter to the configuration.

You can provide parameters via command line flags or load from a JSON file using --file.
The parameter must not already exist.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			var paramConfig rules.HighRiskParamConfig
			var err error

			if inputFile != "" {
				// Load from file
				paramConfig, component, paramType, paramName, err = loadParameterFromFile(inputFile)
				if err != nil {
					return err
				}
			} else {
				// Command line mode
				if err := validateRequiredFields(component, paramType, paramName, severity); err != nil {
					return err
				}

				paramConfig = buildParamConfigFromFlags(severity, description, checkModified, fromVersion, toVersion, allowedValues)
			}

			// Add parameter (will check if exists)
			if err := addOrEditParameter(component, paramType, paramName, paramConfig, true); err != nil {
				return err
			}

			configFile := highRiskParamsConfigFile
			if configFile == "" {
				configFile = high_risk_params.GetDefaultConfigPath()
			}

			fmt.Printf("Successfully added high-risk parameter: %s/%s/%s\n", component, paramType, paramName)
			fmt.Printf("Configuration saved to: %s\n", configFile)

			return nil
		},
	}

	cmd.Flags().StringVar(&inputFile, "file", "", "Load parameter configuration from JSON file")
	cmd.Flags().StringVar(&component, "component", "", "Component name (tidb, pd, tikv, tiflash)")
	cmd.Flags().StringVar(&paramType, "type", "", "Parameter type (config or system_variable)")
	cmd.Flags().StringVar(&paramName, "name", "", "Parameter name")
	cmd.Flags().StringVar(&severity, "severity", "", "Severity level (error, warning, info)")
	cmd.Flags().StringVar(&description, "description", "", "Description of why this parameter is high-risk")
	cmd.Flags().BoolVar(&checkModified, "check-modified", false, "Only check if parameter is modified from default")
	cmd.Flags().StringVar(&fromVersion, "from-version", "", "From version (e.g., v7.0.0)")
	cmd.Flags().StringVar(&toVersion, "to-version", "", "To version (e.g., v7.5.0)")
	cmd.Flags().StringSliceVar(&allowedValues, "allowed-values", []string{}, "Allowed values (comma-separated)")

	return cmd
}

func newHighRiskParamsEditCmd() *cobra.Command {
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
		inputFile     string
	)

	cmd := &cobra.Command{
		Use:   "edit",
		Short: "Edit a high-risk parameter",
		Long: `Edit an existing high-risk parameter in the configuration.

You can provide parameters via command line flags or load from a JSON file using --file.
The parameter must already exist.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			var paramConfig rules.HighRiskParamConfig
			var err error

			if inputFile != "" {
				// Load from file
				paramConfig, component, paramType, paramName, err = loadParameterFromFile(inputFile)
				if err != nil {
					return err
				}
			} else {
				// Command line mode
				if err := validateRequiredFields(component, paramType, paramName, severity); err != nil {
					return err
				}

				paramConfig = buildParamConfigFromFlags(severity, description, checkModified, fromVersion, toVersion, allowedValues)
			}

			// Edit parameter (will check if exists)
			if err := addOrEditParameter(component, paramType, paramName, paramConfig, false); err != nil {
				return err
			}

			configFile := highRiskParamsConfigFile
			if configFile == "" {
				configFile = high_risk_params.GetDefaultConfigPath()
			}

			fmt.Printf("Successfully edited high-risk parameter: %s/%s/%s\n", component, paramType, paramName)
			fmt.Printf("Configuration saved to: %s\n", configFile)

			return nil
		},
	}

	cmd.Flags().StringVar(&inputFile, "file", "", "Load parameter configuration from JSON file")
	cmd.Flags().StringVar(&component, "component", "", "Component name (tidb, pd, tikv, tiflash)")
	cmd.Flags().StringVar(&paramType, "type", "", "Parameter type (config or system_variable)")
	cmd.Flags().StringVar(&paramName, "name", "", "Parameter name")
	cmd.Flags().StringVar(&severity, "severity", "", "Severity level (error, warning, info)")
	cmd.Flags().StringVar(&description, "description", "", "Description of why this parameter is high-risk")
	cmd.Flags().BoolVar(&checkModified, "check-modified", false, "Only check if parameter is modified from default")
	cmd.Flags().StringVar(&fromVersion, "from-version", "", "From version (e.g., v7.0.0)")
	cmd.Flags().StringVar(&toVersion, "to-version", "", "To version (e.g., v7.5.0)")
	cmd.Flags().StringSliceVar(&allowedValues, "allowed-values", []string{}, "Allowed values (comma-separated)")

	return cmd
}
